// T8.1 验收：DELETE /v1/documents/{id} 进回收站路由 + PATCH sort_key 透传。
package httpapi

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"element-wiki/internal/database"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

// newTrashEnv 在标准 env 之上接好回收站存储（生产 main 的真实接线形态）。
func newTrashEnv(t *testing.T) *env {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"u1", "u2", "u3"} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
			VALUES (?, 'i', ?, '', '', 1)`, u, u)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	svc.SetTrashHooks(impl)
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{Docs: svc, Trees: impl, ActorFor: actorFor, Auth: auth}
	srv := httptest.NewServer(NewRouter(deps))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv}
}

func createDoc(t *testing.T, e *env, slug, title string, parent *string) string {
	t.Helper()
	payload := map[string]any{"slug": slug, "title": title}
	if parent != nil {
		payload["parent_id"] = *parent
	}
	resp, body := e.do("POST", "/v1/documents", "editor", payload)
	mustStatus(t, resp.StatusCode, 201, body)
	return body["document"].(map[string]any)["id"].(string)
}

func TestDeleteDocumentMovesSubtreeToTrash(t *testing.T) {
	e := newTrashEnv(t)

	parent := createDoc(t, e, "del-parent", "Del Parent", nil)
	child := createDoc(t, e, "del-child", "Del Child", &parent)

	// viewer 删除 → 403
	resp, body := e.do("DELETE", "/v1/documents/"+parent, "viewer", nil)
	mustStatus(t, resp.StatusCode, 403, body)

	// editor 删除子树 → 204 无内容
	resp, body = e.do("DELETE", "/v1/documents/"+parent, "editor", nil)
	mustStatus(t, resp.StatusCode, 204, body)
	if len(body) != 0 {
		t.Fatalf("204 应无响应体, got %v", body)
	}

	// 主树不可见：父与子均消失，且不泄漏 restricted 式存在性
	resp, body = e.do("GET", "/v1/documents/"+parent, "admin", nil)
	mustStatus(t, resp.StatusCode, 404, body)
	resp, body = e.do("GET", "/v1/documents/"+child, "admin", nil)
	mustStatus(t, resp.StatusCode, 404, body)

	// 回收站可见
	resp, body = e.do("GET", "/v1/trash", "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	found := false
	for _, it := range items {
		if it.(map[string]any)["id"] == parent {
			found = true
		}
	}
	if !found {
		t.Fatalf("回收站缺少已删父文档: %v", body)
	}

	// 再次删除同一 id → 404（已不在主树）
	resp, body = e.do("DELETE", "/v1/documents/"+parent, "editor", nil)
	mustStatus(t, resp.StatusCode, 404, body)

	// 匿名删除 → 403
	resp, body = e.do("DELETE", "/v1/documents/"+parent, "anon", nil)
	mustStatus(t, resp.StatusCode, 403, body)
}

func TestPatchSortKeyOrdersTree(t *testing.T) {
	e := newEnv(t)

	a := createDoc(t, e, "order-a", "A", nil)
	b := createDoc(t, e, "order-b", "B", nil)

	treeOrder := func() []string {
		resp, body := e.do("GET", "/v1/documents/tree", "editor", nil)
		mustStatus(t, resp.StatusCode, 200, body)
		var ids []string
		for _, n := range body["nodes"].([]any) {
			ids = append(ids, n.(map[string]any)["id"].(string))
		}
		return ids
	}

	// 默认同为 100 → slug 字典序 a < b
	got := treeOrder()
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("默认顺序异常: %v (a=%s b=%s)", got, a, b)
	}

	// PATCH sort_key：b=100 提到最前，a=300 沉底
	resp, body := e.do("PATCH", "/v1/documents/"+b, "editor", map[string]any{"sort_key": 100})
	mustStatus(t, resp.StatusCode, 200, body)
	if bv := body["document"].(map[string]any)["sort_key"]; bv != float64(100) {
		t.Fatalf("PATCH 响应 sort_key = %v", bv)
	}
	resp, body = e.do("PATCH", "/v1/documents/"+a, "editor", map[string]any{"sort_key": 300})
	mustStatus(t, resp.StatusCode, 200, body)

	got = treeOrder()
	if len(got) != 2 || got[0] != b || got[1] != a {
		t.Fatalf("sort_key 未生效于树排序: %v", got)
	}

	// viewer PATCH sort_key → 403；非法类型静默忽略（与既有字段解析一致）
	resp, body = e.do("PATCH", "/v1/documents/"+a, "viewer", map[string]any{"sort_key": 1})
	mustStatus(t, resp.StatusCode, 403, body)
}
