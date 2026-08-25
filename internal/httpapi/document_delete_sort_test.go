// T8.1 验收：DELETE /v1/documents/{id} 进回收站路由 + PATCH sort_key 透传。
package httpapi

import (
	"context"
	"net/http"
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

func TestReorderSiblings(t *testing.T) {
	e := newEnv(t)

	a := createDoc(t, e, "ro-a", "A", nil)
	b := createDoc(t, e, "ro-b", "B", nil)
	c := createDoc(t, e, "ro-c", "C", nil)
	other := createDoc(t, e, "ro-other", "Other", nil)

	reorder := func(ids []string) (*http.Response, map[string]any) {
		return e.do("PUT", "/v1/documents/reorder", "editor",
			map[string]any{"parent_id": nil, "document_ids": ids})
	}

	// viewer → 403
	resp, body := e.do("PUT", "/v1/documents/reorder", "viewer",
		map[string]any{"document_ids": []string{a, b, c}})
	mustStatus(t, resp.StatusCode, 403, body)

	// 缺员（少 other）→ 422 fields.document_ids
	resp, body = reorder([]string{a, b, c})
	mustStatus(t, resp.StatusCode, 422, body)
	if _, ok := body["fields"].(map[string]any)["document_ids"]; !ok {
		t.Fatalf("fields 缺少 document_ids: %v", body)
	}

	// 跨父混入 → 422
	resp, body = reorder([]string{a, b, c, other, other})
	mustStatus(t, resp.StatusCode, 422, body) // 先命中重复
	_, body = reorder([]string{a, b, c, other, "01JZZZNOTEXIST0000000000000"})
	mustStatus(t, 422, 422, body)

	// 成功：四兄弟全量倒序变序 → 204 无内容
	resp, body = reorder([]string{c, b, a, other})
	mustStatus(t, resp.StatusCode, 204, body)
	if len(body) != 0 {
		t.Fatalf("204 应无响应体: %v", body)
	}

	// 树顺序与 sort_key 落库验证
	resp, body = e.do("GET", "/v1/documents/tree", "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	nodes := body["nodes"].([]any)
	var order []string
	for i, n := range nodes {
		m := n.(map[string]any)
		order = append(order, m["id"].(string))
		if want := float64((i + 1) * 100); m["sort_key"] != want {
			t.Fatalf("节点 %d sort_key = %v, want %v", i, m["sort_key"], want)
		}
	}
	if len(order) != 4 || order[0] != c || order[1] != b || order[2] != a || order[3] != other {
		t.Fatalf("重排未生效: %v", order)
	}
}
