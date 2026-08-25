// T10.1 验收：GET /v1/site 公开端点——config 默认值 + 在线设置覆盖，匿名可访问。
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"element-wiki/internal/database"
	adminservice "element-wiki/internal/service/adminservice"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func newSiteEnv(t *testing.T, wireAdmin bool) (*env, *httptest.Server) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	auth := authservice.New(impl, impl, impl, "https://idp.test", nil, false)
	deps := Deps{
		Docs: svc, Trees: impl, ActorFor: actorFor, Auth: auth,
		SiteDefaults: SiteInfo{Title: "Cfg Title", DefaultLang: "zh-CN", AnonymousRead: true, CommentsEnabled: true},
	}
	if wireAdmin {
		deps.Admin = adminservice.New(impl, impl, impl)
	}
	srv := httptest.NewServer(NewRouter(deps))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv}, srv
}

func TestSitePublicDefaults(t *testing.T) {
	e, _ := newSiteEnv(t, false)

	// 匿名可访问；返回 config 默认值
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/site", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out struct {
		Title           string `json:"title"`
		DefaultLang     string `json:"default_lang"`
		AnonymousRead   bool   `json:"anonymous_read"`
		CommentsEnabled bool   `json:"comments_enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Title != "Cfg Title" || out.DefaultLang != "zh-CN" || !out.AnonymousRead || !out.CommentsEnabled {
		t.Fatalf("默认值不符: %+v", out)
	}
}

func TestSiteOverridesFromSettings(t *testing.T) {
	e, srv := newSiteEnv(t, true)

	// admin 修改在线设置 → site 反映覆盖值
	setResp, body := e.do("PATCH", "/v1/admin/settings", "admin",
		map[string]any{"wiki_title": "Renamed Wiki", "default_lang": "en", "comments_enabled": "false"})
	mustStatus(t, setResp.StatusCode, 200, body)

	req, _ := http.NewRequest("GET", srv.URL+"/v1/site", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Title           string `json:"title"`
		DefaultLang     string `json:"default_lang"`
		AnonymousRead   bool   `json:"anonymous_read"`
		CommentsEnabled bool   `json:"comments_enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Title != "Renamed Wiki" || out.DefaultLang != "en" || out.CommentsEnabled {
		t.Fatalf("在线覆盖未生效: %+v", out)
	}
	// anonymous_read 种子为 false：DB 值存在即覆盖 config 的 true
	if out.AnonymousRead {
		t.Fatalf("DB 种子值应覆盖默认: %+v", out)
	}
}
