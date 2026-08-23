package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"element-wiki/internal/permission"
	authservice "element-wiki/internal/service/authservice"
	"element-wiki/internal/sso"
)

// OIDCDeps 承载 SSO 配置；Enabled=false 时相关端点返回 503。
type OIDCDeps struct {
	Enabled      bool
	ProviderName string
	RedirectURI  string // 回调完整 URL（含 base_path）
	Scopes       []string
	Client       *sso.Client
}

// oidcStateCookies 三个短效流程 cookie 名。
const (
	cOauthState   = "ew_oidc_state"
	cOauthNonce   = "ew_oidc_nonce"
	cOauthPKCE    = "ew_oidc_pkce"
	cOauthRedir   = "ew_oidc_redir"
	sessionMaxAge = 7 * 24 * 3600
)

func (d *Deps) handleOIDCStatus(w http.ResponseWriter, r *http.Request) {
	if d.OIDC == nil || !d.OIDC.Enabled {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": true, "provider_name": d.OIDC.ProviderName,
	})
}

func (d *Deps) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !d.oidcReady(w) {
		return
	}
	ctx := r.Context()
	disc, err := d.OIDC.Client.Discover(ctx)
	if err != nil {
		redirectLoginErr(w, r, d.CookieCfg(), "provider_unavailable")
		return
	}
	state := sso.RandomB64(16)
	nonce := sso.RandomB64(16)
	verifier, challenge := sso.PKCEPair()

	setShortCookie(w, d.CookieCfg(), cOauthState, state, 600)
	setShortCookie(w, d.CookieCfg(), cOauthNonce, nonce, 600)
	setShortCookie(w, d.CookieCfg(), cOauthPKCE, verifier, 600)

	target := sanitizeRedirect(r.URL.Query().Get("redirect"))
	setShortCookie(w, d.CookieCfg(), cOauthRedir, target, 600)

	http.Redirect(w, r, d.OIDC.Client.AuthURL(disc,
		d.OIDC.RedirectURI, state, nonce, challenge, d.OIDC.Scopes),
		http.StatusSeeOther)
}

func (d *Deps) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !d.oidcReady(w) {
		return
	}
	cfgC := d.CookieCfg()
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		clearOIDCCookies(w, cfgC)
		redirectLoginErr(w, r, cfgC, e)
		return
	}
	stateCookie, err := r.Cookie(cOauthState)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != q.Get("state") {
		clearOIDCCookies(w, cfgC)
		redirectLoginErr(w, r, cfgC, "state_mismatch")
		return
	}
	nonceCookie, _ := r.Cookie(cOauthNonce)
	verifierCookie, _ := r.Cookie(cOauthPKCE)
	code := q.Get("code")
	if code == "" || verifierCookie == nil {
		clearOIDCCookies(w, cfgC)
		redirectLoginErr(w, r, cfgC, "missing_code")
		return
	}

	ctx := r.Context()
	disc, err := d.OIDC.Client.Discover(ctx)
	if err != nil {
		redirectLoginErr(w, r, cfgC, "provider_unavailable")
		return
	}
	rawToken, err := d.OIDC.Client.Exchange(ctx, disc, d.OIDC.RedirectURI, code, verifierCookie.Value)
	if err != nil {
		clearOIDCCookies(w, cfgC)
		redirectLoginErr(w, r, cfgC, "exchange_failed")
		return
	}
	var nonce string
	if nonceCookie != nil {
		nonce = nonceCookie.Value
	}
	claims, verr := d.OIDC.Client.VerifyIDToken(ctx, rawToken, nonce)
	if verr != nil {
		clearOIDCCookies(w, cfgC)
		reason := "token_invalid"
		if errors.Is(verr, sso.ErrInvalid) && strings.Contains(verr.Error(), "nonce") {
			reason = "nonce_mismatch"
		}
		redirectLoginErr(w, r, cfgC, reason)
		return
	}

	user, serr := d.Auth.ResolveSSO(ctx, claims.Subject, claims.Email,
		firstNonEmpty(claims.PreferredUsername, claims.Name, claims.Email))
	if serr != nil {
		clearOIDCCookies(w, cfgC)
		if errors.Is(serr, authservice.ErrDisabled) {
			redirectLoginErr(w, r, cfgC, "account_disabled")
			return
		}
		redirectLoginErr(w, r, cfgC, "provision_failed")
		return
	}

	raw, exp, err := d.Auth.NewSession(ctx, user.ID)
	if err != nil {
		redirectLoginErr(w, r, cfgC, "session_failed")
		return
	}
	maxAge := int((exp - nowMillisHTTP()) / 1000)
	setSessionCookie(w, cfgC, raw, exp, maxAge)
	clearOIDCCookies(w, cfgC)

	target := "/"
	if rc, rcerr := r.Cookie(cOauthRedir); rcerr == nil && rc.Value != "" {
		if t := sanitizeRedirect(rc.Value); t != "" {
			target = t
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (d *Deps) handleLogout(w http.ResponseWriter, r *http.Request) {
	val := ""
	if c, cerr := r.Cookie(sessionCookie); cerr == nil {
		val = c.Value
	}
	if err := d.Auth.Logout(r.Context(), val); mapServiceErr(w, err) {
		return
	}
	clearSessionCookie(w, d.CookieCfg())
	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) handleMe(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r)
	if actor == nil || actor.UserID() == "" {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	u, err := d.Auth.Me(r.Context(), actor.UserID())
	if mapServiceErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":        u,
		"permissions": permission.CodesFor(u.Role),
	})
}

func (d *Deps) oidcReady(w http.ResponseWriter) bool {
	if d.OIDC == nil || !d.OIDC.Enabled {
		writeErr(w, http.StatusServiceUnavailable, "OIDC is not configured")
		return false
	}
	return true
}

func setShortCookie(w http.ResponseWriter, cfg cookieCfg, name, val string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: val, Path: "/", HttpOnly: true,
		Secure: cfg.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	})
}

func clearOIDCCookies(w http.ResponseWriter, cfg cookieCfg) {
	for _, n := range []string{cOauthState, cOauthNonce, cOauthPKCE, cOauthRedir} {
		setShortCookie(w, cfg, n, "", -1)
	}
}

// sanitizeRedirect 仅接受同源相对路径，防开放重定向。
func sanitizeRedirect(target string) string {
	if target == "" {
		return ""
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	return target
}

func redirectLoginErr(w http.ResponseWriter, r *http.Request, cfg cookieCfg, reason string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(reason), http.StatusSeeOther)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
