// T3.3 全流程集成：login→(stub IdP)→callback→会话→me；篡改与一次性引导。
package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// loginFlow 走完整跳转链，返回 callback 响应与其 cookies。
func (e *authEnv) loginFlow(t *testing.T, idp *stubIDP, sub, email, name string) (*http.Response, []*http.Cookie) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // 不跟随 302
	}}

	// 1) /login → 捕获 Location 与流程 cookie
	req, _ := http.NewRequest("GET", e.srv.URL+"/v1/auth/oidc/login?redirect=/docs/x", nil)
	r1, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Body.Close()
	if r1.StatusCode != http.StatusSeeOther {
		t.Fatalf("login 应 303, got %d", r1.StatusCode)
	}
	authzURL := r1.Header.Get("Location")
	var state, nonce, verifier string
	for _, c := range r1.Cookies() {
		switch c.Name {
		case cOauthState:
			state = c.Value
		case cOauthNonce:
			nonce = c.Value
		case cOauthPKCE:
			verifier = c.Value
		}
	}
	if !strings.Contains(authzURL, "state="+state) ||
		!strings.Contains(authzURL, "code_challenge_method=S256") {
		t.Fatalf("authorize URL 缺参数: %s", authzURL)
	}
	// redirect 目标被安全记录（同源相对路径）
	u := url_Parse(authzURL)
	if u.Query().Get("redirect_uri") == "" {
		t.Fatal("缺 redirect_uri")
	}

	// 2) 登记 code（stub 充当 authorize 完成态），带 PKCE challenge
	sum := sha256Sum(verifier)
	code := idp.IssueCode(sub, email, name, nonce, sum)

	// 3) callback：携带流程 cookie + code/state
	parsed := mustParseURL(e.srv.URL + "/v1/auth/oidc/callback")
	q := parsed.Query()
	q.Set("code", code)
	q.Set("state", state)
	parsed.RawQuery = q.Encode()
	req2, _ := http.NewRequest("GET", parsed.String(), nil)
	for _, c := range r1.Cookies() {
		req2.AddCookie(c)
	}
	r2, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	return r2, r2.Cookies()
}

func TestOIDCFullFlowJITViewer(t *testing.T) {
	e, idp := newOIDCEnv(t, nil, "Corp SSO")

	resp, cookies := e.loginFlow(t, idp, "sub-abc", "dev@corp.com", "Dev")
	if resp.StatusCode != http.StatusSeeOther {
		body := readAllBody(resp)
		t.Fatalf("callback 应 303, got %d: %s", resp.StatusCode, body)
	}
	if loc := resp.Header.Get("Location"); loc != "/docs/x" {
		t.Errorf("应回跳原始目标, got %q", loc)
	}

	// 会话 cookie 属性：HttpOnly + Secure + 非空
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == "access_token" {
			session = c
		}
	}
	if session == nil || !session.HttpOnly || !session.Secure || session.Value == "" {
		t.Fatalf("会话 cookie 异常: %+v", session)
	}

	// me → JIT viewer
	me, body := e.doWithCookieBody("GET", "/v1/users/me", session.Value)
	if me.StatusCode != 200 {
		t.Fatalf("me 应 200: %d %s", me.StatusCode, body)
	}
	user := body["user"].(map[string]any)
	if user["role"] != "viewer" || user["email"] != "dev@corp.com" {
		t.Errorf("JIT 用户异常: %v", user)
	}
	perms := body["permissions"].([]any)
	if len(perms) == 0 {
		t.Error("权限列表为空")
	}

	// 二次登录同 subject 不产生重复账号
	resp2, _ := e.loginFlow(t, idp, "sub-abc", "dev@corp.com", "Dev")
	readAllBody(resp2)
	var n int
	e.db.QueryRow(`SELECT COUNT(*) FROM users WHERE subject='sub-abc'`).Scan(&n)
	if n != 1 {
		t.Errorf("subject 应唯一, 行数 %d", n)
	}
}

func TestOIDCAdminBootstrapOnceAndDisabledRedirect(t *testing.T) {
	e, idp := newOIDCEnv(t, []string{"root@corp.com"}, "SSO")

	// 首个命中者 → admin
	resp, _ := e.loginFlow(t, idp, "boss-sub", "root@corp.com", "Boss")
	readAllBody(resp)
	me, body := e.doWithCookieBody("GET", "/v1/users/me",
		e.latestSessionOf(t, "boss-sub"))
	if me.StatusCode != 200 || body["user"].(map[string]any)["role"] != "admin" {
		t.Errorf("引导 admin 失败: %v", body)
	}

	// 第二个同邮箱用户不再提权
	resp2, _ := e.loginFlow(t, idp, "imposter-sub", "root@corp.com", "Imp")
	readAllBody(resp2)
	me2, b2 := e.doWithCookieBody("GET", "/v1/users/me",
		e.latestSessionOf(t, "imposter-sub"))
	if me2.StatusCode != 200 || b2["user"].(map[string]any)["role"] != "viewer" {
		t.Errorf("第二次引导未拦截: %v", b2)
	}

	// 管理员禁用该账号后，其既有会话登录 → 重定向 account_disabled
	subject := "victim-sub"
	victim, _ := e.auth.ResolveSSO(context_Background(), subject, "v@corp.com", "V")
	e.auth.SetDisabledForTest(victim.ID)
	r3, _ := e.loginFlow(t, idp, subject, "v@corp.com", "V")
	loc := r3.Header.Get("Location")
	readAllBody(r3)
	if !strings.Contains(loc, "account_disabled") {
		t.Errorf("禁用账号应重定向 account_disabled: %q", loc)
	}
}

func TestOIDCTamperRejections(t *testing.T) {
	e, idp := newOIDCEnv(t, nil, "SSO")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// state 篡改 → 303 至 login?error=state_mismatch
	r1, _ := client.Get(e.srv.URL + "/v1/auth/oidc/login")
	readAllBody(r1)
	badCB := e.srv.URL + "/v1/auth/oidc/callback?code=x&state=TAMPERED"
	req, _ := http.NewRequest("GET", badCB, nil)
	for _, c := range r1.Cookies() {
		req.AddCookie(c)
	}
	r2, _ := client.Do(req)
	b2 := readAllBody(r2)
	if !strings.Contains(r2.Header.Get("Location"), "state_mismatch") {
		t.Errorf("state 篡改应拒绝: %q %s", r2.Header.Get("Location"), b2)
	}

	// nonce 篡改：登记 code 但 stub 用错误 nonce 签发 → nonce_mismatch
	r3, _ := client.Get(e.srv.URL + "/v1/auth/oidc/login")
	var state, verifier string
	for _, c := range r3.Cookies() {
		switch c.Name {
		case cOauthState:
			state = c.Value
		case cOauthPKCE:
			verifier = c.Value
		}
	}
	code := idp.IssueCode("sub-n", "n@x.com", "N", "WRONG-NONCE", sha256Sum(verifier))
	cb := e.srv.URL + "/v1/auth/oidc/callback?code=" + code + "&state=" + state
	req4, _ := http.NewRequest("GET", cb, nil)
	for _, c := range r3.Cookies() {
		req4.AddCookie(c)
	}
	r4, _ := client.Do(req4)
	readAllBody(r4)
	if !strings.Contains(r4.Header.Get("Location"), "nonce_mismatch") &&
		!strings.Contains(r4.Header.Get("Location"), "token_invalid") {
		t.Errorf("nonce 篡改应拒绝: %q", r4.Header.Get("Location"))
	}

	// 未配置站点：status=false 且 login=503
	dbEnv := newAuthEnv(t, false)
	stResp, sb := dbEnv.doWithCookieBody("GET", "/v1/auth/oidc/status", "")
	mustStatus(t, stResp.StatusCode, 200, sb)
	if sb["enabled"] != false {
		t.Errorf("enabled 应 false: %v", sb)
	}
	lr, lb := dbEnv.doWithCookieBody("GET", "/v1/auth/oidc/login", "")
	if lr.StatusCode != 503 {
		t.Errorf("未配置 login 应 503, got %d %s", lr.StatusCode, lb)
	}
}

func TestLogoutIdempotentViaAPI(t *testing.T) {
	e, idp := newOIDCEnv(t, nil, "SSO")
	resp, cookies := e.loginFlow(t, idp, "lo-sub", "lo@x.com", "L")
	readAllBody(resp)
	var session string
	for _, c := range cookies {
		if c.Name == "access_token" {
			session = c.Value
		}
	}

	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: session})
	r1, _ := http.DefaultClient.Do(req)
	readAllBody(r1)
	if r1.StatusCode != 204 {
		t.Errorf("logout 应 204, got %d", r1.StatusCode)
	}
	// 幂等：无 cookie 再注销仍 204
	r2, _ := http.Post(e.srv.URL+"/v1/auth/logout", "", nil)
	readAllBody(r2)
	if r2.StatusCode != 204 {
		t.Errorf("幂等 logout 应 204, got %d", r2.StatusCode)
	}
	// 旧会话已失效
	if r := e.doWithCookie("GET", "/v1/users/me", session, ""); r.StatusCode != 401 {
		t.Errorf("注销后 me 应 401, got %d", r.StatusCode)
	}
}
