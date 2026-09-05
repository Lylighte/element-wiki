// 05 计划提交 4 验收：GET /v1/documents/resolve 路径解析 + 创建端点 slug 自动生成。
package httpapi

import (
	"context"
	"net/http"
	"testing"
)

func TestResolveEndpointByPath(t *testing.T) {
	e, svc := newAnonEnv(t, false)
	ctx := context.Background()
	ed := actorOf(t, "ed")

	root, _ := svc.CreateDocument(ctx, ed, nil, "guide", "Guide")
	child, _ := svc.CreateDocument(ctx, ed, &root.ID, "setup", "Setup")
	if _, err := svc.Commit(ctx, ed, child.ID, "", "# Setup\n", "init"); err != nil {
		t.Fatal(err)
	}

	r, body := e.doJSON("GET", "/v1/documents/resolve?path=guide/setup", e.sessionFor("ed"), nil)
	if r.StatusCode != 200 {
		t.Fatalf("解析嵌套路径 = %d %v", r.StatusCode, body)
	}
	doc, ok := body["document"].(map[string]any)
	if !ok || doc["slug"] != "setup" || doc["id"] != child.ID {
		t.Errorf("resolve document 异常: %v", body)
	}
	if _, ok := body["render"]; !ok {
		t.Errorf("resolve 应含 render: %v", body)
	}

	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=guide", e.sessionFor("ed"), nil); r.StatusCode != 200 {
		t.Errorf("根路径解析 = %d", r.StatusCode)
	}
	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=guide/nope", e.sessionFor("ed"), nil); r.StatusCode != 404 {
		t.Errorf("不存在路径应 404, got %d", r.StatusCode)
	}
	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=", e.sessionFor("ed"), nil); r.StatusCode != 404 {
		t.Errorf("空 path 应 404, got %d", r.StatusCode)
	}
	if r := e.doWithCookie("GET", "/v1/documents/resolve?path=guide", "", ""); r.StatusCode != 401 {
		t.Errorf("未认证应 401, got %d", r.StatusCode)
	}
}

func TestResolveEndpointRestrictedMasking(t *testing.T) {
	e, svc := newAnonEnv(t, false)
	ctx := context.Background()
	ed := actorOf(t, "ed")

	sec, _ := svc.CreateDocument(ctx, ed, nil, "secret", "Secret")
	if err := svc.SetVisibility(ctx, ed, sec.ID, docVisibilityRestricted()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commit(ctx, ed, sec.ID, "", "body", "s"); err != nil {
		t.Fatal(err)
	}

	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=secret", e.sessionFor("ed"), nil); r.StatusCode != 200 {
		t.Errorf("editor 应解析 restricted, got %d", r.StatusCode)
	}
	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=secret", e.sessionFor("vw"), nil); r.StatusCode != 404 {
		t.Errorf("viewer 访问 restricted 应 404 掩护, got %d", r.StatusCode)
	}
}

func TestResolveEndpointAnonymousMasking(t *testing.T) {
	e, svc := newAnonEnv(t, true)
	ctx := context.Background()
	ed := actorOf(t, "ed")

	pub, _ := svc.CreateDocument(ctx, ed, nil, "pub", "P")
	if _, err := svc.Commit(ctx, ed, pub.ID, "", "b", "p"); err != nil {
		t.Fatal(err)
	}
	sec, _ := svc.CreateDocument(ctx, ed, nil, "sec", "S")
	if err := svc.SetVisibility(ctx, ed, sec.ID, docVisibilityRestricted()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commit(ctx, ed, sec.ID, "", "b", "s"); err != nil {
		t.Fatal(err)
	}

	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=pub", "", nil); r.StatusCode != 200 {
		t.Errorf("匿名解析 standard 应 200, got %d", r.StatusCode)
	}
	if r, _ := e.doJSON("GET", "/v1/documents/resolve?path=sec", "", nil); r.StatusCode != 404 {
		t.Errorf("匿名解析 restricted 应 404 掩护, got %d", r.StatusCode)
	}
}

func TestCreateEndpointAutoSlug(t *testing.T) {
	e := newAuthEnv(t, false)

	r, body := e.doJSON("POST", "/v1/documents", e.sessionFor("u1"),
		map[string]any{"title": "Auto Slug Title"})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("自动 slug 创建 = %d %v", r.StatusCode, body)
	}
	doc, ok := body["document"].(map[string]any)
	if !ok || doc["slug"] != "auto-slug-title" {
		t.Errorf("自动生成 slug = %v, want auto-slug-title", body)
	}
}
