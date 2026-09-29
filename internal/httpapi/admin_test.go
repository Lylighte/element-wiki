// T6.1~T6.4 验收：设置校验/部分更新、用户治理、仪表盘聚合。
package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	store "element-wiki/internal/database"
	sqlitestore "element-wiki/internal/database/sqlite"
	adminservice "element-wiki/internal/service/adminservice"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"

	"element-wiki/internal/model"
)

func newAdminEnv(t *testing.T) (*authEnv, *adminservice.Service) {
	t.Helper()
	db, err := store.Open("sqlite", filepath.Join(t.TempDir(), "adm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newAppliedMigrator(t, db)
	for _, u := range []struct{ id, role string }{
		{"ad", "admin"}, {"ed", "editor"},
	} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, ?, '', ?, 'active', 1)`, u.id, u.id, u.id+"@x.com", u.role); err != nil {
			t.Fatal(err)
		}
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetCommentStore(impl, impl)
	svc.SetDocumentSubmissionStore(impl)
	svc.SetTrashHooks(impl)
	svc.SetAttachmentStore(impl, filepath.Join(t.TempDir(), "att"),
		"png,txt", 5)
	os_MkdirAll(svc.AttachDir())
	auth := authservice.New(impl, impl, impl, "https://idp.test", []string{"ad@x.com"}, false)
	admin := adminservice.New(impl, impl, impl)
	svc.SetSettingsSource(admin)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true,
		Admin: admin, Reports: impl, ContentModeration: impl, CommentsEnabled: true, AttachmentsOn: true,
		AttachDir: svc.AttachDir(), UploadMaxBytes: 5 << 20}
	e := &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db, svc: svc}

	// 种子两篇文档（一篇 restricted）供 dashboard
	editor := actorOf(t, "ed")
	d1, _ := svc.CreateDocument(context.Background(), editor, nil, "dash-a", "公开A")
	svc.Commit(context.Background(), editor, d1.ID, "", "body a", "m")
	d2, _ := svc.CreateDocument(context.Background(), editor, nil, "dash-b", "机密B")
	svc.SetVisibility(context.Background(), editor, d2.ID, model.VisibilityRestricted)
	time.Sleep(2 * time.Millisecond)
	svc.Commit(context.Background(), editor, d2.ID, "", "body b", "m")
	return e, admin
}

func TestSettingsEndpoints(t *testing.T) {
	e, _ := newAdminEnv(t)

	// viewer/editor → 403
	if r := e.doWithCookie("GET", "/v1/admin/settings", e.sessionFor("ed"), ""); r.StatusCode != 403 {
		t.Errorf("非 admin 读设置应 403, got %d", r.StatusCode)
	}

	// GET 全量含种子键
	resp, body := e.doWithCookieBody("GET", "/v1/admin/settings", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	if body["comments_enabled"] != "false" || body["max_versions"] != "100" {
		t.Errorf("种子缺失: %v", body)
	}

	// PATCH 合法部分更新
	resp, body = e.doJSON("PATCH", "/v1/admin/settings", e.sessionFor("ad"),
		map[string]any{"wiki_title": "New Title", "max_versions": "50"})
	mustStatus(t, resp.StatusCode, 200, body)

	resp, body = e.doWithCookieBody("GET", "/v1/admin/settings", e.sessionFor("ad"))
	if body["wiki_title"] != "New Title" || body["max_versions"] != "50" ||
		body["comments_enabled"] != "false" {
		t.Errorf("更新后状态异常: %v", body)
	}

	// 未知键 → 422 fields
	resp, body = e.doJSON("PATCH", "/v1/admin/settings", e.sessionFor("ad"),
		map[string]any{"hacker_key": "1"})
	mustStatus(t, resp.StatusCode, 422, body)
	if _, ok := body["fields"].(map[string]any)["hacker_key"]; !ok {
		t.Errorf("fields 缺 hacker_key: %v", body)
	}

	// 类型错误 → 422
	for key, val := range map[string]any{
		"max_versions":     "-5",
		"timezone":         "Mars/Olympus",
		"default_lang":     "fr",
		"comments_enabled": "yes-please",
	} {
		resp, body = e.doJSON("PATCH", "/v1/admin/settings", e.sessionFor("ad"),
			map[string]any{key: val})
		mustStatus(t, resp.StatusCode, 422, body)
	}

	// editor 写 → 403
	resp, _ = e.doJSON("PATCH", "/v1/admin/settings", e.sessionFor("ed"),
		map[string]any{"wiki_title": "hack"})
	mustStatus(t, resp.StatusCode, 403, nil)
}

func TestDocumentSubmissionReviewEndpoints(t *testing.T) {
	e, admin := newAdminEnv(t)
	ctx := context.Background()
	editor := actorOf(t, "ed")
	doc, err := e.svc.CreateDocument(ctx, editor, nil, "review-http", "Review HTTP")
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.UpdateSettings(ctx, permission_AdminActor(), map[string]string{"document_review_required": "true"}); err != nil {
		t.Fatal(err)
	}
	resp, body := e.doJSON("POST", "/v1/documents/"+doc.ID+"/commits", e.sessionFor("ed"), map[string]any{"base_commit_id": "", "content": "submitted body", "message": "proposed"})
	mustStatus(t, resp.StatusCode, 202, body)
	if body["pending"] != true {
		t.Fatalf("submission not marked pending: %v", body)
	}
	resp, queue := e.doWithCookieBody("GET", "/v1/admin/reviews/documents", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, queue)
	items := queue["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("pending queue=%v", items)
	}
	id := items[0].(map[string]any)["id"].(string)
	if r := e.doWithCookie("POST", "/v1/admin/reviews/documents/"+id+"/approve", e.sessionFor("ed"), `{}`); r.StatusCode != 403 {
		t.Fatalf("non-admin approved submission: %d", r.StatusCode)
	}
	if r := e.doWithCookie("POST", "/v1/admin/reviews/documents/"+id+"/approve", e.sessionFor("ad"), `{}`); r.StatusCode != 204 {
		t.Fatalf("admin approve: %d", r.StatusCode)
	}
	bodyText, _, err := e.svc.HeadContent(ctx, editor, doc.ID)
	if err != nil || bodyText != "submitted body" {
		t.Fatalf("approved body=%q err=%v", bodyText, err)
	}
}

func TestContentReportEndpoints(t *testing.T) {
	e, _ := newAdminEnv(t)
	doc, err := e.svc.CreateDocument(context.Background(), actorOf(t, "ed"), nil, "reported-doc", "Reported document")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.svc.Commit(context.Background(), actorOf(t, "ed"), doc.ID, "", "# Reported title\n\nunsafe excerpt", "seed report target"); err != nil {
		t.Fatal(err)
	}
	resp, body := e.doJSON("POST", "/v1/reports", e.sessionFor("ed"), map[string]any{"content_type": "document", "content_id": doc.ID, "reason": "contains unsafe instructions"})
	mustStatus(t, resp.StatusCode, 201, body)
	duplicate, _ := e.doJSON("POST", "/v1/reports", e.sessionFor("ed"), map[string]any{"content_type": "document", "content_id": doc.ID, "reason": "duplicate report"})
	if duplicate.StatusCode != 409 {
		t.Fatalf("pending duplicate report status=%d", duplicate.StatusCode)
	}
	resp, body = e.doWithCookieBody("GET", "/v1/admin/reviews/reports", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("report queue=%v", items)
	}
	if items[0].(map[string]any)["title"] != "Reported document" || !strings.Contains(items[0].(map[string]any)["preview_html"].(string), "unsafe excerpt") {
		t.Fatalf("report queue has no safe target preview: %v", items[0])
	}
	id := items[0].(map[string]any)["id"].(string)
	if r := e.doWithCookie("POST", "/v1/admin/reviews/reports/"+id+"/resolve", e.sessionFor("ad"), `{"resolution":"document restricted"}`); r.StatusCode != 204 {
		t.Fatalf("resolve report: %d", r.StatusCode)
	}
	resp, body = e.doWithCookieBody("GET", "/v1/admin/reviews/reports", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	if len(body["items"].([]any)) != 0 {
		t.Fatalf("resolved report still pending: %v", body)
	}
}

func TestAdminUnpublishRestoreEndpoints(t *testing.T) {
	e, _ := newAdminEnv(t)
	ctx := context.Background()
	doc, err := e.svc.CreateDocument(ctx, actorOf(t, "ed"), nil, "moderate-me", "Moderate me")
	if err != nil {
		t.Fatal(err)
	}
	url := "/v1/admin/moderation/document/" + doc.ID
	if r := e.doWithCookie("POST", url+"/unpublish", e.sessionFor("ad"), `{"reason":"policy violation"}`); r.StatusCode != 204 {
		t.Fatalf("unpublish status=%d", r.StatusCode)
	}
	resp, body := e.doWithCookieBody("GET", "/v1/admin/moderation/hidden", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	if len(body["items"].([]any)) != 1 {
		t.Fatalf("hidden list=%v", body)
	}
	if r := e.doWithCookie("POST", url+"/restore", e.sessionFor("ad"), `{"reason":"review complete"}`); r.StatusCode != 204 {
		t.Fatalf("restore status=%d", r.StatusCode)
	}
}

func TestUserManagementEndpoints(t *testing.T) {
	e, _ := newAdminEnv(t)

	// 列表 + q 过滤
	resp, body := e.doWithCookieBody("GET", "/v1/admin/users?q=ed", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != "ed" {
		t.Fatalf("q 过滤异常: %v", items)
	}

	// editor 访问 → 403
	if r := e.doWithCookie("GET", "/v1/admin/users", e.sessionFor("ed"), ""); r.StatusCode != 403 {
		t.Errorf("editor 列用户应 403, got %d", r.StatusCode)
	}

	// 角色调整
	resp, body = e.doJSON("PATCH", "/v1/admin/users/ed", e.sessionFor("ad"),
		map[string]any{"role": "viewer"})
	mustStatus(t, resp.StatusCode, 200, body)
	if body["user"].(map[string]any)["role"] != "viewer" {
		t.Errorf("角色未生效: %v", body)
	}
	// 恢复
	e.doJSON("PATCH", "/v1/admin/users/ed", e.sessionFor("ad"), map[string]any{"role": "editor"})

	// 操作自己 → 422
	resp, body = e.doJSON("PATCH", "/v1/admin/users/ad", e.sessionFor("ad"),
		map[string]any{"role": "viewer"})
	mustStatus(t, resp.StatusCode, 422, body)

	// 非法 role/status → 422
	for _, patch := range []map[string]any{
		{"role": "root"},
		{"status": "zombie"},
	} {
		resp, body = e.doJSON("PATCH", "/v1/admin/users/ed", e.sessionFor("ad"), patch)
		mustStatus(t, resp.StatusCode, 422, body)
	}

	// 禁用 → 该用户既有会话立即 403
	e.doJSON("PATCH", "/v1/admin/users/ed", e.sessionFor("ad"),
		map[string]any{"status": "disabled"})
	if r := e.doWithCookie("GET", "/v1/documents/tree", e.sessionFor("ed"), ""); r.StatusCode != 403 {
		t.Errorf("禁用后会话应 403, got %d", r.StatusCode)
	}
	// 启用恢复
	e.doJSON("PATCH", "/v1/admin/users/ed", e.sessionFor("ad"),
		map[string]any{"status": "active"})
	if r := e.doWithCookie("GET", "/v1/documents/tree", e.sessionFor("ed"), ""); r.StatusCode != 200 {
		t.Errorf("启用后应恢复 200, got %d", r.StatusCode)
	}
}

func TestDashboardAggregation(t *testing.T) {
	e, _ := newAdminEnv(t)
	resp, body := e.doWithCookieBody("GET", "/v1/admin/dashboard", e.sessionFor("ad"))
	mustStatus(t, resp.StatusCode, 200, body)
	if body["documents_total"].(float64) != 2 {
		t.Errorf("documents_total = %v", body["documents_total"])
	}
	recents := body["recent_docs"].([]any)
	if len(recents) != 2 {
		t.Fatalf("recent_docs 数量 = %d", len(recents))
	}
	first := recents[0].(map[string]any)
	// 最近更新应为 dash-b（后提交）
	if first["slug"] != "dash-b" {
		t.Errorf("排序异常: %v", first)
	}
	contribs := body["contributors"].([]any)
	if len(contribs) == 0 || contribs[0].(map[string]any)["count"].(float64) < 1 {
		t.Errorf("贡献者统计异常: %v", contribs)
	}

	// editor 无权限 → 403
	if r := e.doWithCookie("GET", "/v1/admin/dashboard", e.sessionFor("ed"), ""); r.StatusCode != 403 {
		t.Errorf("editor 仪表盘应 403, got %d", r.StatusCode)
	}
	_ = strings.Contains
	_ = json.Marshal
}
