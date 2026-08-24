// T3.2 验收：会话中间件矩阵——有效会话/过期/禁用/垃圾 cookie/匿名 401。
package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"element-wiki/internal/database"
	authsvc "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

type authEnv struct {
	t    *testing.T
	srv  *httptest.Server
	auth *authsvc.Service
	db   *sql.DB
	svc  *docservice.Service
}

func newAuthEnv(t *testing.T, anonRead bool) *authEnv {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, role, status string }{
		{"u1", "editor", "active"},
		{"u9", "viewer", "disabled"},
	} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, '', '', ?, ?, 1)`, u.id, u.id, u.role, u.status); err != nil {
			t.Fatal(err)
		}
	}
	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	auth := authsvc.New(impl, impl, impl, "https://idp.example", []string{"boss@x.com"}, anonRead)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true}
	return &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth, db: db, svc: svc}
}

func (e *authEnv) sessionFor(userID string) string {
	e.t.Helper()
	raw, _, err := e.auth.NewSession(context.Background(), userID)
	if err != nil {
		e.t.Fatal(err)
	}
	return raw
}

func (e *authEnv) doWithCookie(method, path, cookie, bodyJSON string) *http.Response {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp
}

func TestSessionMiddlewareMatrix(t *testing.T) {
	e := newAuthEnv(t, false)

	// 无凭据 → /v1 一律 401（契约 §14），healthz 不受影响
	if r := e.doWithCookie("POST", "/v1/documents", "", `{}`); r.StatusCode != 401 {
		t.Errorf("无凭据创建应 401, got %d", r.StatusCode)
	}
	hr, _ := http.Get(e.srv.URL + "/healthz")
	hr.Body.Close()
	if hr.StatusCode != 200 {
		t.Errorf("healthz 应公开: %d", hr.StatusCode)
	}

	// 有效 editor 会话 → 创建成功
	if r := e.doWithCookie("POST", "/v1/documents", e.sessionFor("u1"),
		`{"slug":"made","title":"Made"}`); r.StatusCode != 201 {
		t.Errorf("editor 会话创建应 201, got %d", r.StatusCode)
	}

	// 垃圾 cookie → 401
	if r := e.doWithCookie("POST", "/v1/documents", "garbage", `{}`); r.StatusCode != 401 {
		t.Errorf("垃圾 cookie 应 401, got %d", r.StatusCode)
	}

	// 禁用账号既有会话 → 403 且带明确 detail（绝不降级为匿名）
	dc := e.sessionFor("u9")
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/documents/tree", nil)
	req.AddCookie(&http.Cookie{Name: "ew_session", Value: dc})
	r2, _ := http.DefaultClient.Do(req)
	b2, _ := io.ReadAll(r2.Body)
	r2.Body.Close()
	if r2.StatusCode != 403 || !strings.Contains(string(b2), "account disabled") {
		t.Errorf("禁用账号应 403+detail, got %d %s", r2.StatusCode, b2)
	}
}

func TestSessionExpiryCleansUp(t *testing.T) {
	e := newAuthEnv(t, false)
	cookie := e.sessionFor("u1")
	if _, err := e.db.Exec(`UPDATE sessions SET expires_at = ?`,
		time.Now().UnixMilli()-1); err != nil {
		t.Fatal(err)
	}
	if r := e.doWithCookie("GET", "/v1/documents/tree", cookie, ""); r.StatusCode != 401 {
		t.Errorf("过期会话应 401, got %d", r.StatusCode)
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("过期会话应被顺手删除, 剩 %d", n)
	}
}

// T3.4 端点验收：签发一次性明文、列表无哈希、吊销后 Bearer 401。
func TestTokenEndpoints(t *testing.T) {
	e := newAuthEnv(t, false)
	cookie := e.sessionFor("u1")

	// 无凭据（newAuthEnv 无 ActorFor → 真实中间件生效）→ 401
	if r := e.doWithCookie("POST", "/v1/tokens", "", `{"name":"x"}`); r.StatusCode != 401 {
		t.Errorf("匿名签发应 401, got %d", r.StatusCode)
	}

	// 签发
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/tokens",
		strings.NewReader(`{"name":"ci"}`))
	req.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	r1, _ := http.DefaultClient.Do(req)
	b1, _ := io.ReadAll(r1.Body)
	r1.Body.Close()
	if r1.StatusCode != 201 {
		t.Fatalf("签发应 201, got %d %s", r1.StatusCode, b1)
	}
	var created struct {
		ID, Prefix, Token string
	}
	json.Unmarshal(b1, &struct {
		ID     *string `json:"id"`
		Prefix *string `json:"prefix"`
		Token  *string `json:"token"`
	}{&created.ID, &created.Prefix, &created.Token})
	if !strings.HasPrefix(created.Token, "ew_") || created.Prefix != created.Token[:8] {
		t.Fatalf("令牌契约异常: %+v", created)
	}

	// Bearer 访问受保护端点成功
	req2, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents",
		strings.NewReader(`{"slug":"via-token","title":"VT"}`))
	req2.Header.Set("Authorization", "Bearer "+created.Token)
	r2, _ := http.DefaultClient.Do(req2)
	r2.Body.Close()
	if r2.StatusCode != 201 {
		t.Errorf("Bearer 创建应 201, got %d", r2.StatusCode)
	}

	// 列表不含哈希/明文
	req3, _ := http.NewRequest("GET", e.srv.URL+"/v1/tokens", nil)
	req3.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	r3, _ := http.DefaultClient.Do(req3)
	b3, _ := io.ReadAll(r3.Body)
	r3.Body.Close()
	if strings.Contains(string(b3), created.Token) || strings.Contains(string(b3), `"token_hash"`) {
		t.Errorf("列表泄漏敏感值: %s", b3)
	}

	// 吊销 → 204 → Bearer 失效 401
	req4, _ := http.NewRequest("DELETE", e.srv.URL+"/v1/tokens/"+created.ID, nil)
	req4.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	r4, _ := http.DefaultClient.Do(req4)
	r4.Body.Close()
	if r4.StatusCode != 204 {
		t.Fatalf("吊销应 204, got %d", r4.StatusCode)
	}
	req5, _ := http.NewRequest("POST", e.srv.URL+"/v1/documents",
		strings.NewReader(`{"slug":"after","title":"A"}`))
	req5.Header.Set("Authorization", "Bearer "+created.Token)
	r5, _ := http.DefaultClient.Do(req5)
	r5.Body.Close()
	if r5.StatusCode != 401 {
		t.Errorf("吊销后 Bearer 应 401, got %d", r5.StatusCode)
	}

}

func (e *authEnv) doWithCookieBody(method, path, cookie string) (*http.Response, map[string]any) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "ew_session", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func (e *authEnv) latestSessionOf(t *testing.T, subject string) string {
	t.Helper()
	var userID string
	if err := e.db.QueryRow(`SELECT id FROM users WHERE subject = ?`, subject).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	return e.sessionFor(userID)
}

func readAllBody(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	return string(b)
}

func (e *authEnv) authSvcPtr() *authsvc.Service { return e.auth }
