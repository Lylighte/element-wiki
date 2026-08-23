// T4.3/T4.4 验收：搜索 API 权限过滤与二次校验；手动全量重建任务闭环。
package httpapi

import (
	"context"
	"element-wiki/internal/model"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	searchservice "element-wiki/internal/service/searchservice"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/internal/search"
)

func newSearchEnv(t *testing.T) (*authEnv, *search.Index, *search.RebuildDeps) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newAppliedMigrator(t, db)
	for _, u := range []struct{ id, role string }{
		{"ed", "editor"}, {"vw", "viewer"}, {"ad", "admin"},
	} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, '', '', ?, 'active', 1)`, u.id, u.id, u.role)
	}
	idx, err := search.Open(filepath.Join(t.TempDir(), "documents.bleve"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetSearchHooks(idx, impl)
	ssvc := searchservice.New(idx, impl, impl)
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true,
		Search: ssvc, Jobs: impl}
	return &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db, svc: svc}, idx, &search.RebuildDeps{Jobs: impl, Docs: impl, Coms: impl, Index: idx, Log: nopSlog()}
}

func TestManualFullRebuildJob(t *testing.T) {
	e, idx, worker := newSearchEnv(t)
	ctx := context.Background()
	d, cerr := editorCreate(e, ctx, "rb-doc", "重建目标", "rebuild marker")
	if cerr != nil {
		t.Fatal(cerr)
	}

	// 直接破坏索引一致性：删除该文档索引项
	idx.DeleteDoc(ctx, d.ID)

	// admin 发起全量重建 → 202 + job_id
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/admin/search/rebuild", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ad")})
	r1, _ := http.DefaultClient.Do(req)
	b1, _ := io.ReadAll(r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 202 {
		t.Fatalf("rebuild 应 202, got %d %s", r1.StatusCode, b1)
	}
	var job struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(b1, &job)
	if job.JobID == "" {
		t.Fatal("缺 job_id")
	}

	// 直接查库确认任务存在
	var n int
	e.db.QueryRow(`SELECT COUNT(*) FROM search_reindex_jobs`).Scan(&n)
	t.Logf("jobs in db=%d", n)

	// 手动驱动 worker 一轮 → done 且检索恢复
	for worker.ProcessOnce(ctx) {
	}
	var st2, d2 string
	e.db.QueryRow(`SELECT status, COALESCE(document_id,'') FROM search_reindex_jobs`).Scan(&st2, &d2)
	t.Logf("after worker: status=%s doc=%q", st2, d2)
	t.Logf("GET url will be: %q", e.srv.URL+"/v1/admin/search/rebuild/"+job.JobID)
	stReq, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/search/rebuild/"+job.JobID, nil)
	stReq.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ad")})
	stR, _ := http.DefaultClient.Do(stReq)
	rawStatus := ioReadAllBody(stR)
	t.Logf("raw status body: %q code=%d", rawStatus, stR.StatusCode)
	st := struct {
		Status   string  `json:"status"`
		Document *string `json:"document_id"`
	}{}
	_ = json.Unmarshal([]byte(rawStatus), &st)
	if st.Status != "done" || st.Document != nil {
		t.Fatalf("job 状态异常: %+v", st)
	}

	hits, _ := idx.Query(ctx, "marker", 5)
	found := false
	for _, h := range hits {
		if h.DocumentID == d.ID {
			found = true
		}
	}
	if !found {
		t.Error("全量重建后应恢复检索")
	}

	// 权限：editor 无 search.rebuild → 403
	reqE, _ := http.NewRequest("POST", e.srv.URL+"/v1/admin/search/rebuild", nil)
	reqE.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ed")})
	rE, _ := http.DefaultClient.Do(reqE)
	rE.Body.Close()
	if rE.StatusCode != 403 {
		t.Errorf("editor 触发重建应 403, got %d", rE.StatusCode)
	}
	// 状态查询不存在 job → 404
	reqG, _ := http.NewRequest("GET", e.srv.URL+"/v1/admin/search/rebuild/ghost", nil)
	reqG.AddCookie(&http.Cookie{Name: "access_token", Value: e.sessionFor("ad")})
	rG, _ := http.DefaultClient.Do(reqG)
	rG.Body.Close()
	if rG.StatusCode != 404 {
		t.Errorf("幽灵 job 应 404, got %d", rG.StatusCode)
	}
}

// T4.3 验收：搜索 API 权限过滤（viewer 不见 restricted）+ 高亮 + 短语。
func TestSearchAPIPermissionFiltering(t *testing.T) {
	e, _, _ := newSearchEnv(t)
	ctx := context.Background()
	editor := actorOf(t, "ed")

	pub, cerr := e.svc.CreateDocument(ctx, editor, nil, "pub-doc", "公开")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if _, cerr = e.svc.Commit(ctx, editor, pub.ID, "", "needle in public body", "m"); cerr != nil {
		t.Fatal(cerr)
	}
	sec, cerr := e.svc.CreateDocument(ctx, editor, nil, "sec-doc", "机密")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if _, cerr = e.svc.Commit(ctx, editor, sec.ID, "", "needle hidden secret", "m"); cerr != nil {
		t.Fatal(cerr)
	}
	if cerr = e.svc.SetVisibility(ctx, editor, sec.ID, model.VisibilityRestricted); cerr != nil {
		t.Fatal(cerr)
	}

	// viewer → 仅 standard
	resp, body := e.doWithCookieBody("GET", "/v1/search?q=needle&limit=10",
		e.sessionFor("vw"))
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["document_id"] != pub.ID {
		t.Fatalf("viewer 过滤失败: %v", items)
	}
	if !strings.Contains(items[0].(map[string]any)["snippet"].(string), "<mark>") {
		t.Errorf("snippet 缺高亮: %v", items[0])
	}

	// editor → 两篇
	resp, body = e.doWithCookieBody("GET", "/v1/search?q=needle", e.sessionFor("ed"))
	mustStatus(t, resp.StatusCode, 200, body)
	if len(body["items"].([]any)) != 2 {
		t.Fatalf("editor 应命中两篇: %v", body)
	}

	// 精确短语
	resp, body = e.doWithCookieBody("GET",
		"/v1/search?q="+url.QueryEscape(`"hidden secret"`), e.sessionFor("ed"))
	mustStatus(t, resp.StatusCode, 200, body)
	items = body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["document_id"] != sec.ID {
		t.Fatalf("短语查询应只命中 sec-doc: %v", items)
	}

	// 匿名关闭站点 → 401
	if r := e.doWithCookie("GET", "/v1/search?q=x", "", ""); r.StatusCode != 401 {
		t.Errorf("匿名搜索应 401, got %d", r.StatusCode)
	}
}
