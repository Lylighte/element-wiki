// T3.3 验收：stub IdP 全流程——PKCE/nonce/state、JIT、一次性 admin 引导、
// cookie 属性、禁用重定向、篡改拒绝。
package httpapi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"element-wiki/internal/database"
	authservice "element-wiki/internal/service/authservice"
	docservice "element-wiki/internal/service/docservice"
	"element-wiki/internal/sso"
	sqlitestore "element-wiki/internal/store/sqlite"
)

// stubIDP 是进程内 OpenID Provider。
type stubIDP struct {
	t        *testing.T
	srv      *httptest.Server
	key      *rsa.PrivateKey
	codes    map[string]stubCode
	clientID string
	secret   string
}

type stubCode struct {
	sub, email, name, nonce string
	pkceChallenge           string
}

func newStubIDP(t *testing.T) *stubIDP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &stubIDP{t: t, key: key,
		codes: map[string]stubCode{}, clientID: "elem-client", secret: "elem-secret"}
	p.srv = httptest.NewServer(http.HandlerFunc(p.route))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *stubIDP) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		base := p.srv.URL
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 base,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"jwks_uri":               base + "/jwks.json",
		})
	case "/jwks.json":
		n := base64.RawURLEncoding.EncodeToString(p.key.PublicKey.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes())
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA", "kid": "stub-1", "alg": "RS256", "use": "sig",
				"n": n, "e": e,
			}},
		})
	case "/token":
		p.handleToken(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *stubIDP) handleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	code := r.Form.Get("code")
	params, ok := p.codes[code]
	if !ok {
		http.Error(w, "unknown code", http.StatusBadRequest)
		return
	}
	if v := r.Form.Get("code_verifier"); v != "" && params.pkceChallenge != "" {
		sum := sha256.Sum256([]byte(v))
		chal := base64.RawURLEncoding.EncodeToString(sum[:])
		if chal != params.pkceChallenge {
			http.Error(w, "pkce mismatch", http.StatusBadRequest)
			return
		}
	}
	now := time.Now().Unix()
	header := b64url(`{"alg":"RS256","kid":"stub-1","typ":"JWT"}`)
	payload := b64url(mustJSON(map[string]any{
		"iss": p.srv.URL, "aud": p.clientID, "sub": params.sub,
		"email": params.email, "preferred_username": params.name,
		"nonce": params.nonce, "iat": now, "exp": now + 300,
	}))
	sum := sha256.Sum256([]byte(header + "." + payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, sum[:])
	if err != nil {
		p.t.Fatal(err)
	}
	idToken := header + "." + payload + "." + b64urlBytes(sig)
	json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
}

// IssueCode 由测试侧登记一次授权码。
func (p *stubIDP) IssueCode(sub, email, name, nonce, challenge string) string {
	code := sso.RandomB64(12)
	p.codes[code] = stubCode{sub: sub, email: email, name: name,
		nonce: nonce, pkceChallenge: challenge}
	return code
}

func newOIDCEnv(t *testing.T, adminEmails []string, providerName string) (*authEnv, *stubIDP) {
	t.Helper()
	idp := newStubIDP(t)
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "oidc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newAppliedMigrator(t, db)

	impl := sqlitestore.New(db)
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	auth := authservice.New(impl, impl, impl, idp.srv.URL, adminEmails, false)
	deps := Deps{Docs: svc, Trees: impl, Auth: auth, SecureCookies: true,
		OIDC: &OIDCDeps{Enabled: true, ProviderName: providerName,
			RedirectURI: idp.srv.URL + "/callback", Scopes: []string{"openid", "email"},
			Client: sso.NewClient(idp.srv.URL, idp.clientID, idp.secret)}}
	return &authEnv{t: t, srv: httptest.NewServer(NewRouter(deps)), auth: auth,
		db: db, svc: svc}, idp
}
