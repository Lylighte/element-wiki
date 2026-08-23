// T3.5/T3.6 验收：匿名只读门闩 + restricted 全链路 404 掩护 + 角色矩阵。
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/migrations"
)

// anonOnEnv：开启匿名只读的真实中间件环境（无 ActorFor 钩子）。
func newAnonEnv(t *testing.T, anonRead bool) (*authEnv, *docservice.Service) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "anon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, role string }{
		{"ed", "editor"}, {"vw", "viewer"}, {"ad", "admin"},
	} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, '', '', ?, 'active', 1)`, u.id, u.id, u.role)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, anonRead)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true}
	return &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db}, svc
}

// seedRestrictedTree 经 service 以 editor 建立标准/受限两棵子树。
func seedAnonFixtures(t *testing.T, e *authEnv, svc *docservice.Service) (stdID, resID string) {
	t.Helper()
	ctx := context.Background()
	editor := actorOf(t, "ed")
	pub, _ := svc.CreateDocument(ctx, editor, nil, "public-root", "公开")
	sec, _ := svc.CreateDocument(ctx, editor, nil, "secret-root", "机密")
	svc.SetVisibility(ctx, editor, sec.ID, docVisibilityRestricted())
	_, _ = svc.Commit(ctx, editor, pub.ID, "", "public body", "p")
	_, _ = svc.Commit(ctx, editor, sec.ID, "", "secret body", "s")
	return pub.ID, sec.ID
}

func TestAnonymousGateAndRestrictedMasking(t *testing.T) {
	e, svc := newAnonEnv(t, true)
	stdID, resID := seedAnonFixtures(t, e, svc)

	// 匿名树只含 standard 分支
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/documents/tree", nil)
	r1, _ := http.DefaultClient.Do(req)
	b1, _ := ioReadAll(r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 200 {
		t.Fatalf("匿名模式树应 200: %d %s", r1.StatusCode, b1)
	}
	if strings.Contains(b1, "secret-root") {
		t.Errorf("匿名树泄漏 restricted 节点: %s", b1)
	}

	// 匿名读 restricted 文档 → 404 掩护（而非 403）
	req, _ = http.NewRequest("GET", e.srv.URL+"/v1/documents/"+resID, nil)
	r2, _ := http.DefaultClient.Do(req)
	ioCopyDiscard(r2.Body)
	if r2.StatusCode != 404 {
		t.Errorf("匿名读 restricted 应 404 掩护, got %d", r2.StatusCode)
	}

	// 匿名读 standard → 200
	req, _ = http.NewRequest("GET", e.srv.URL+"/v1/documents/"+stdID, nil)
	r3, _ := http.DefaultClient.Do(req)
	ioCopyDiscard(r3.Body)
	if r3.StatusCode != 200 {
		t.Errorf("匿名读 standard 应 200, got %d", r3.StatusCode)
	}

	// restricted 的 render/history 同样 404
	req, _ = http.NewRequest("GET", e.srv.URL+"/v1/documents/"+resID+"/render", nil)
	r4, _ := http.DefaultClient.Do(req)
	ioCopyDiscard(r4.Body)
	if r4.StatusCode != 404 {
		t.Errorf("restricted render 应 404, got %d", r4.StatusCode)
	}

	// 匿名写 → 权限拒绝（403，非 401——已认证语义之外的匿名）
	req, _ = http.NewRequest("POST", e.srv.URL+"/v1/documents",
		strings.NewReader(`{"slug":"w","title":"W"}`))
	r5, _ := http.DefaultClient.Do(req)
	ioCopyDiscard(r5.Body)
	if r5.StatusCode != 403 {
		t.Errorf("匿名写应 403, got %d", r5.StatusCode)
	}

	// viewer 对 restricted 也只能得到 404
	if r := e.doWithCookie("GET", "/v1/documents/"+resID,
		e.sessionFor("vw"), ""); r.StatusCode != 404 {
		t.Errorf("viewer 读 restricted 应 404, got %d", r.StatusCode)
	}

	// editor/admin 正常可见
	editorCookie := e.sessionFor("ed")
	if r := e.doWithCookie("GET", "/v1/documents/"+resID,
		editorCookie, ""); r.StatusCode != 200 {
		t.Errorf("editor 读 restricted 应 200, got %d", r.StatusCode)
	}
	adminCookie := e.sessionFor("ad")
	reqA, _ := http.NewRequest("GET", e.srv.URL+"/v1/documents/tree", nil)
	reqA.AddCookie(&http.Cookie{Name: "access_token", Value: adminCookie})
	rA, _ := http.DefaultClient.Do(reqA)
	rawA := readAllBody(rA)
	if !strings.Contains(rawA, "secret-root") {
		t.Errorf("admin 树应含 restricted: %s", rawA)
	}

	// 匿名关闭后同一站点行为切换为整站 401
	e2, svc2 := newAnonEnv(t, false)
	stdID2, resID2 := seedAnonFixtures(t, e2, svc2)
	for _, id := range []string{stdID2, resID2} {
		reqB, _ := http.NewRequest("GET", e2.srv.URL+"/v1/documents/"+id, nil)
		rb, _ := http.DefaultClient.Do(reqB)
		ioCopyDiscard(rb.Body)
		if rb.StatusCode != 401 {
			t.Errorf("匿名关闭时读文档应 401, got %d", rb.StatusCode)
		}
	}
}
