// T1.8 验收：documents 域端点矩阵——method/path/status/body 精确断言，
// 含 409 冲突与 422 校验契约、权限拒绝、树构建与渲染。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	"element-wiki/internal/permission"
	"element-wiki/internal/render"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

type env struct {
	t   *testing.T
	srv *httptest.Server
}

// actorFor 按请求头注入角色：editor/viewer/admin/anon。
func actorFor(r *http.Request) permission.Actor {
	switch r.Header.Get("X-Test-Role") {
	case "editor":
		return permission.NewActor("u1", permission.CodesFor(permission.Editor))
	case "viewer":
		return permission.NewActor("u2", permission.CodesFor(permission.Viewer))
	case "admin":
		return permission.NewActor("u3", permission.CodesFor(permission.Admin))
	default:
		return permission.Anonymous(false)
	}
}

func newEnv(t *testing.T) *env {
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
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{Docs: svc, Trees: impl, ActorFor: actorFor, Auth: auth}
	srv := httptest.NewServer(NewRouter(deps))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv}
}

func (e *env) do(method, path, role string, body any) (*http.Response, map[string]any) {
	e.t.Helper()
	var rd *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	req.Header.Set("X-Test-Role", role)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func mustStatus(t *testing.T, got int, want int, body map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d, body=%v", got, want, body)
	}
}

func TestDocumentEndpointMatrix(t *testing.T) {
	e := newEnv(t)

	// 未认证创建 → 403（匿名模式关闭）
	resp, body := e.do("POST", "/v1/documents", "anon",
		map[string]any{"slug": "x", "title": "X"})
	mustStatus(t, resp.StatusCode, 403, body)

	// 创建根文档 → 201 + snake_case 字段
	resp, body = e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "hello", "title": "Hello"})
	mustStatus(t, resp.StatusCode, 201, body)
	docMap := body["document"].(map[string]any)
	id := docMap["id"].(string)
	if docMap["slug"] != "hello" || docMap["visibility"] != "standard" ||
		docMap["head_commit_id"] != "" || docMap["parent_id"] != nil {
		t.Fatalf("create 视图异常: %v", docMap)
	}

	// 重复 slug → 409 conflict
	resp, body = e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "hello", "title": "H2"})
	mustStatus(t, resp.StatusCode, 409, body)
	if body["detail"] != "conflict" {
		t.Errorf("conflict detail = %v", body["detail"])
	}

	// 非法 slug → 422 fields.slug
	resp, body = e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "BAD!", "title": "T"})
	mustStatus(t, resp.StatusCode, 422, body)
	fields := body["fields"].(map[string]any)
	if _, ok := fields["slug"]; !ok {
		t.Errorf("fields 缺少 slug: %v", fields)
	}

	// GET 单文档 + 生效可见性
	resp, body = e.do("GET", "/v1/documents/"+id, "viewer", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	dv := body["document"].(map[string]any)
	if dv["effective_visibility"] != "standard" {
		t.Errorf("effective_visibility = %v", dv["effective_visibility"])
	}

	// PATCH 改标题；parent_id=null 不触发移动错误
	resp, body = e.do("PATCH", "/v1/documents/"+id, "editor",
		map[string]any{"title": "Hello v2"})
	mustStatus(t, resp.StatusCode, 200, body)
	if body["document"].(map[string]any)["title"] != "Hello v2" {
		t.Errorf("PATCH 标题未生效")
	}
	// 非法 visibility → 422
	resp, body = e.do("PATCH", "/v1/documents/"+id, "editor",
		map[string]any{"visibility": "secret"})
	mustStatus(t, resp.StatusCode, 422, body)

	// viewer PATCH → 403
	resp, _ = e.do("PATCH", "/v1/documents/"+id, "viewer",
		map[string]any{"title": "nope"})
	mustStatus(t, resp.StatusCode, 403, map[string]any{})

	// ghost → 404
	resp, body = e.do("GET", "/v1/documents/nope", "editor", nil)
	mustStatus(t, resp.StatusCode, 404, body)
	if body["detail"] != "not found" {
		t.Errorf("404 detail = %v", body["detail"])
	}
}

func TestTreeNestedAndRestrictedFlag(t *testing.T) {
	e := newEnv(t)
	// 摆两层树：mid(restricted) > leaf
	_, b2 := e.do("POST", "/v1/documents", "editor", map[string]any{"slug": "mid", "title": "M"})
	midID := ""
	if m, ok := b2["document"].(map[string]any); ok {
		midID = m["id"].(string)
	}
	e.do("PATCH", "/v1/documents/"+midID, "editor", map[string]any{"visibility": "restricted"})
	_, b3 := e.do("POST", "/v1/documents", "editor", map[string]any{"parent_id": midID, "slug": "leaf", "title": "L"})
	leafID := ""
	if m, ok := b3["document"].(map[string]any); ok {
		leafID = m["id"].(string)
	}

	// 叶子生效可见性应为 restricted（继承自 mid）
	resp, body := e.do("GET", "/v1/documents/"+leafID, "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	if body["document"].(map[string]any)["effective_visibility"] != "restricted" {
		t.Errorf("叶子继承失败: %v", body)
	}

	// 树接口：嵌套结构 + restricted 传播
	resp, body = e.do("GET", "/v1/documents/tree", "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	nodes := body["nodes"].([]any)
	var walk func(list []any) (foundRestrictedLeaf bool)
	walk = func(list []any) bool {
		for _, n := range list {
			node := n.(map[string]any)
			if node["slug"] == "leaf" {
				return node["restricted"] == true
			}
			if kids, ok := node["children"].([]any); ok && walk(kids) {
				return true
			}
		}
		return false
	}
	if !walk(nodes) {
		t.Errorf("树中未找到带 restricted 标记的 leaf: %v", nodes)
	}

	// 匿名（关闭）访问树 → 403
	resp, _ = e.do("GET", "/v1/documents/tree", "anon", nil)
	mustStatus(t, resp.StatusCode, 403, map[string]any{})
}

func TestDraftAndCommitAndRevertFlow(t *testing.T) {
	e := newEnv(t)
	_, b := e.do("POST", "/v1/documents", "editor", map[string]any{"slug": "flow", "title": "F"})
	id := b["document"].(map[string]any)["id"].(string)

	// 草稿：PUT 204 → GET 内容 → DELETE 204 → 二次 DELETE 404
	resp, _ := e.do("PUT", "/v1/documents/"+id+"/draft", "editor",
		map[string]any{"base_commit_id": "", "content": "wip"})
	mustStatus(t, resp.StatusCode, 204, nil)
	resp, body := e.do("GET", "/v1/documents/"+id+"/draft", "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	if body["draft"].(map[string]any)["content"] != "wip" {
		t.Errorf("草稿读回异常: %v", body)
	}
	resp, _ = e.do("DELETE", "/v1/documents/"+id+"/draft", "editor", nil)
	mustStatus(t, resp.StatusCode, 204, nil)
	resp, _ = e.do("DELETE", "/v1/documents/"+id+"/draft", "editor", nil)
	mustStatus(t, resp.StatusCode, 404, nil)

	// 无草稿 GET → {"draft": null}
	resp, body = e.do("GET", "/v1/documents/"+id+"/draft", "editor", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	if body["draft"] != nil {
		t.Errorf("应返回 null 草稿: %v", body)
	}

	// 首次提交 → 201，含死链数组
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": "", "content": "# v1\n见 [[missing]]", "message": "init"})
	mustStatus(t, resp.StatusCode, 201, body)
	cm := body["commit"].(map[string]any)
	if cm["commit_no"].(float64) != 1 {
		t.Errorf("commit_no = %v", cm["commit_no"])
	}
	if dl := body["dead_links"].([]any); len(dl) != 1 || dl[0] != "missing" {
		t.Errorf("dead_links = %v", dl)
	}
	head := cm["id"].(string)

	// 过期 base → 409 精确字段
	resp, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": "stale", "content": "bad"})
	mustStatus(t, resp.StatusCode, 409, body)
	if body["detail"] != "version conflict" || body["head_commit_id"] != head || body["base_commit_id"] != "stale" {
		t.Errorf("409 契约不符: %v", body)
	}

	// 第二版
	_, body = e.do("POST", "/v1/documents/"+id+"/commits", "editor",
		map[string]any{"base_commit_id": head, "content": "# v2", "message": "two"})
	head2 := body["commit"].(map[string]any)["id"].(string)

	// 版本列表降序
	resp, body = e.do("GET", "/v1/documents/"+id+"/commits?limit=10", "viewer", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	items := body["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["commit_no"].(float64) != 2 {
		t.Errorf("历史列表异常: %v", items)
	}

	// revert 到第 1 版 → 新 HEAD 内容为 v1，历史不动
	resp, body = e.do("POST", "/v1/documents/"+id+"/revert", "editor",
		map[string]any{"commit_id": head})
	mustStatus(t, resp.StatusCode, 201, body)
	rv := body["commit"].(map[string]any)
	if rv["commit_no"].(float64) != 3 || rv["id"] == head2 {
		t.Errorf("revert 结果异常: %v", rv)
	}

	// render 输出包含 v1 标题 HTML
	resp, body = e.do("GET", "/v1/documents/"+id+"/render", "viewer", nil)
	mustStatus(t, resp.StatusCode, 200, body)
	if !strings.Contains(body["html"].(string), "<h1") {
		t.Errorf("渲染输出缺少 h1: %v", body["html"])
	}

	// 预览渲染：editor 可用 / viewer 403
	resp, body = e.do("POST", "/v1/render-preview", "editor", map[string]any{"markdown": "*em*"})
	mustStatus(t, resp.StatusCode, 200, body)
	if !strings.Contains(body["html"].(string), "<em>") {
		t.Errorf("预览渲染异常: %v", body)
	}
	resp, _ = e.do("POST", "/v1/render-preview", "viewer", map[string]any{"markdown": "x"})
	mustStatus(t, resp.StatusCode, 403, nil)

	// 非法 JSON → 400
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents", strings.NewReader("{oops"))
	req.Header.Set("X-Test-Role", "editor")
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != 400 {
		t.Errorf("非法 JSON 应 400, got %d", r2.StatusCode)
	}
}

// 补边角：limit 非法、parent_id=null 移回根、PATCH 幽灵 404、revert ghost、draft on ghost。
func TestEdgeCasesForContract(t *testing.T) {
	e := newEnv(t)
	_, b := e.do("POST", "/v1/documents", "editor", map[string]any{"slug": "edge", "title": "E"})
	id := b["document"].(map[string]any)["id"].(string)
	_, b2 := e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "kid", "title": "K", "parent_id": id})
	kid := b2["document"].(map[string]any)["id"].(string)

	// limit=0 → 400
	resp, body := e.do("GET", "/v1/documents/"+id+"/commits?limit=0", "editor", nil)
	mustStatus(t, resp.StatusCode, 400, body)
	if body["detail"] != "limit 非法" {
		t.Errorf("limit detail = %v", body["detail"])
	}

	// parent_id: null → 移回根（本已在根，验证 null 不炸且仍 200）
	resp, body = e.do("PATCH", "/v1/documents/"+kid, "editor",
		map[string]any{"parent_id": nil})
	mustStatus(t, resp.StatusCode, 200, body)

	// PATCH 幽灵 → 404
	resp, body = e.do("PATCH", "/v1/documents/ghost", "editor",
		map[string]any{"title": "x"})
	mustStatus(t, resp.StatusCode, 404, body)

	// 草稿写入幽灵 → 404
	resp, _ = e.do("PUT", "/v1/documents/ghost/draft", "editor",
		map[string]any{"base_commit_id": "", "content": "c"})
	if resp.StatusCode != 404 {
		t.Errorf("幽灵草稿应 404, got %d", resp.StatusCode)
	}

	// revert 幽灵 commit → 404；revert 幽灵文档 → 404
	resp, _ = e.do("POST", "/v1/documents/ghost/revert", "editor",
		map[string]any{"commit_id": "any"})
	if resp.StatusCode != 404 {
		t.Errorf("幽灵 revert 文档应 404, got %d", resp.StatusCode)
	}
	resp, _ = e.do("POST", "/v1/documents/"+id+"/revert", "editor",
		map[string]any{"commit_id": "no-such"})
	if resp.StatusCode != 404 {
		t.Errorf("幽灵 commit 应 404, got %d", resp.StatusCode)
	}

	// viewer 读 render 可用（version.read），匿名被拒
	resp, _ = e.do("GET", "/v1/documents/"+id+"/render", "anon", nil)
	if resp.StatusCode != 403 {
		t.Errorf("匿名渲染应 403, got %d", resp.StatusCode)
	}
}

// 默认 ActorFor（nil）→ 全拒兜底；注入渲染故障 → 500。
func TestDefaultActorAndRenderFailure(t *testing.T) {
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "d2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"u1"} {
		db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
			VALUES (?, 'i', ?, '', '', 1)`, u, u)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)

	srv := httptest.NewServer(NewRouter(Deps{Docs: svc, Trees: impl}))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/v1/documents", "application/json", strings.NewReader(`{}`))
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("默认无认证应 403, got %d", resp.StatusCode)
	}

	boom := false
	srv2 := httptest.NewServer(NewRouter(Deps{
		Docs: svc, Trees: impl,
		ActorFor: func(*http.Request) permission.Actor {
			return permission.NewActor("u1", permission.CodesFor(permission.Editor))
		},
		Render: func(string) (*render.Result, error) {
			if boom {
				return nil, errors.New("renderer down")
			}
			return &render.Result{HTML: "ok"}, nil
		},
	}))
	defer srv2.Close()
	_, b := doJSON(t, srv2.URL, "POST", "/v1/documents", map[string]any{"slug": "rf", "title": "R"})
	id := b["document"].(map[string]any)["id"].(string)
	doJSON(t, srv2.URL, "POST", "/v1/documents/"+id+"/commits",
		map[string]any{"base_commit_id": "", "content": "c"})

	boom = true
	req, _ := http.NewRequest("GET", srv2.URL+"/v1/documents/"+id+"/render", nil)
	req.Header.Set("X-Test-Role", "editor")
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	if r2.StatusCode != 500 {
		t.Errorf("渲染故障应 500, got %d", r2.StatusCode)
	}
	req3, _ := http.NewRequest("POST", srv2.URL+"/v1/render-preview", strings.NewReader(`{"markdown":"x"}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Test-Role", "editor")
	r3, _ := http.DefaultClient.Do(req3)
	r3.Body.Close()
	if r3.StatusCode != 500 {
		t.Errorf("预览渲染故障应 500, got %d", r3.StatusCode)
	}
}

func doJSON(t *testing.T, url, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url+path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "editor")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// PATCH 全字段组合 + tokens 校验分支补齐。
func TestPatchAndTokenValidationBranches(t *testing.T) {
	e := newEnv(t)
	_, b := e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "combo", "title": "C"})
	id := b["document"].(map[string]any)["id"].(string)

	// title+slug 同时改
	resp, body := e.do("PATCH", "/v1/documents/"+id, "editor",
		map[string]any{"title": "T2", "slug": "combo-2"})
	mustStatus(t, resp.StatusCode, 200, body)

	// parent_id 非法 JSON 值（数字）→ 400
	req, _ := http.NewRequest("PATCH", e.srv.URL+"/v1/documents/"+id,
		strings.NewReader(`{"parent_id": 3}`))
	req.Header.Set("X-Test-Role", "editor")
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	if r2.StatusCode != 400 {
		t.Errorf("非法 parent_id 应 400, got %d", r2.StatusCode)
	}

	// token name 空白 → 422 fields.name
	resp, body = e.do("POST", "/v1/tokens", "editor", map[string]any{"name": "  "})
	mustStatus(t, resp.StatusCode, 422, body)
	if _, ok := body["fields"].(map[string]any)["name"]; !ok {
		t.Errorf("fields 缺 name: %v", body)
	}

	// 匿名（ActorFor 注入）无 token.manage.own → 403（harness 无 401 中间件）
	resp, _ = e.do("GET", "/v1/tokens", "anon", nil)
	if resp.StatusCode != 403 {
		t.Errorf("匿名列令牌应 403, got %d", resp.StatusCode)
	}

	// DELETE 不存在 token → 404
	resp, _ = e.do("DELETE", "/v1/tokens/nope", "editor", nil)
	mustStatus(t, resp.StatusCode, 404, nil)
}

func TestContractBranchesTopUp(t *testing.T) {
	e := newEnv(t)
	_, b := e.do("POST", "/v1/documents", "editor",
		map[string]any{"slug": "self-move", "title": "S"})
	id := b["document"].(map[string]any)["id"].(string)

	// 移到自身 → 422 fields.parent_id
	resp, body := e.do("PATCH", "/v1/documents/"+id, "editor",
		map[string]any{"parent_id": id})
	mustStatus(t, resp.StatusCode, 422, body)
	if _, ok := body["fields"].(map[string]any)["parent_id"]; !ok {
		t.Errorf("fields 缺 parent_id: %v", body)
	}

	// 匿名读草稿 → 403
	resp, _ = e.do("GET", "/v1/documents/"+id+"/draft", "anon", nil)
	mustStatus(t, resp.StatusCode, 403, nil)

	// 非法 JSON 提交 → 400
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents/"+id+"/commits",
		strings.NewReader("{bad"))
	req.Header.Set("X-Test-Role", "editor")
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	mustStatus(t, r2.StatusCode, 400, nil)
}
