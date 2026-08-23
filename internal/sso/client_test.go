// T3.3 sso 包单元验收：发现/授权 URL/交换/HS256+RS256 验签与全部拒绝分支。
package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeIdP struct {
	t           *testing.T
	srv         *httptest.Server
	key         *rsa.PrivateKey
	alg         string // HS256 | RS256
	claims      map[string]any
	tokenStatus int
}

func newFake(t *testing.T, alg string) *fakeIdP {
	t.Helper()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	f := &fakeIdP{t: t, key: key, alg: alg, claims: map[string]any{
		"iss": "", "aud": "cid", "sub": "sub-x", "email": "x@y.z",
		"nonce": "n1", "exp": time.Now().Unix() + 300,
	}, tokenStatus: 200}
	f.srv = httptest.NewServer(http.HandlerFunc(f.route))
	t.Cleanup(f.srv.Close)
	f.claims["iss"] = f.srv.URL
	return f
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (f *fakeIdP) sign(claims map[string]any) string {
	header := b64([]byte(`{"alg":"` + f.alg + `","kid":"k1"}`))
	payload := b64(mustJSON(claims))
	sum := sha256.Sum256([]byte(header + "." + payload))
	var sig []byte
	if f.alg == "HS256" {
		mac := hmac_sha256([]byte(header+"."+payload), []byte("sec"))
		sig = mac
	} else {
		s, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto_SHA256, sum[:])
		if err != nil {
			f.t.Fatal(err)
		}
		sig = s
	}
	return header + "." + payload + "." + b64(sig)
}

func (f *fakeIdP) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		base := f.srv.URL
		json.NewEncoder(w).Encode(map[string]string{
			"issuer": base, "authorization_endpoint": base + "/auth",
			"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
		})
	case "/jwks":
		n := b64(f.key.PublicKey.N.Bytes())
		e := b64(bigIntBytes(int64(f.key.E)))
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{"kid": "k1", "kty": "RSA", "n": n, "e": e}},
		})
	case "/token":
		if f.tokenStatus != 200 {
			w.WriteHeader(f.tokenStatus)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"id_token": f.sign(f.claims)})
	default:
		http.NotFound(w, r)
	}
}

func TestDiscoverAuthURLAndExchangeHS256(t *testing.T) {
	f := newFake(t, "HS256")
	c := NewClient(f.srv.URL, "cid", "sec")

	d, err := c.Discover(context.Background())
	if err != nil || d.TokenEndpoint == "" {
		t.Fatalf("discover: %v", err)
	}
	d2, _ := c.Discover(context.Background())
	if d2 != d {
		t.Error("发现文档应缓存复用")
	}

	v, chal := PKCEPair()
	if len(v) < 40 || len(chal) < 20 {
		t.Fatalf("PKCE 长度异常")
	}
	sum := sha256.Sum256([]byte(v))
	if chal != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatal("challenge 非 S256")
	}
	authURL := c.AuthURL(d, "http://cb", "st1", "n1", chal, []string{"openid"})
	for _, part := range []string{"state=st1", "code_challenge_method=S256", "scope=openid"} {
		if !strings.Contains(authURL, part) {
			t.Errorf("authorize 缺 %s: %s", part, authURL)
		}
	}

	raw, err := c.Exchange(context.Background(), d, "http://cb", "code1", v)
	if err != nil || raw == "" {
		t.Fatalf("exchange: %v", err)
	}
	cl, err := c.VerifyIDToken(context.Background(), raw, "n1")
	if err != nil || cl.Subject != "sub-x" || cl.Email != "x@y.z" {
		t.Fatalf("HS256 验签: %+v %v", cl, err)
	}

	// 拒绝矩阵
	if _, err := c.VerifyIDToken(context.Background(), raw, "wrong"); !errors.Is(err, ErrInvalid) {
		t.Errorf("nonce 错误应拒绝: %v", err)
	}
	badAud := map[string]any{"iss": f.srv.URL, "aud": "other", "sub": "s", "exp": time.Now().Unix() + 100}
	if _, err := c.VerifyIDToken(context.Background(), f.sign(badAud), ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("aud 错误应拒绝: %v", err)
	}
	expired := map[string]any{"iss": f.srv.URL, "aud": "cid", "sub": "s", "exp": time.Now().Unix() - 500}
	if _, err := c.VerifyIDToken(context.Background(), f.sign(expired), ""); err == nil ||
		!strings.Contains(err.Error(), "过期") {
		t.Errorf("过期应拒绝: %v", err)
	}
	wrongIss := map[string]any{"iss": "https://elsewhere", "aud": "cid", "sub": "s", "exp": time.Now().Unix() + 100}
	if _, err := c.VerifyIDToken(context.Background(), f.sign(wrongIss), ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("issuer 错误应拒绝: %v", err)
	}
	if _, err := c.VerifyIDToken(context.Background(), "not.a.jwt", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("畸形 token 应拒绝: %v", err)
	}
	// 篡改签名
	tampered := raw[:len(raw)-2] + "xx"
	if _, err := c.VerifyIDToken(context.Background(), tampered, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("篡改签名应拒绝: %v", err)
	}
	// 交换失败分支
	f.tokenStatus = 500
	if _, err := c.Exchange(context.Background(), d, "http://cb", "code1", v); err == nil ||
		!strings.Contains(err.Error(), "500") {
		t.Errorf("5xx 交换应报错: %v", err)
	}
}

func TestRS256ViaJWKS(t *testing.T) {
	f := newFake(t, "RS256")
	c := NewClient(f.srv.URL, "cid", "sec")
	ctx := context.Background()
	d, _ := c.Discover(ctx)

	raw, err := c.Exchange(ctx, d, "http://cb", "c", "")
	if err != nil {
		t.Fatal(err)
	}
	cl, err := c.VerifyIDToken(ctx, raw, "n1")
	if err != nil || cl.Subject != "sub-x" {
		t.Fatalf("RS256 验签失败: %v", err)
	}
	// 二次走缓存路径
	if _, err := c.VerifyIDToken(ctx, raw, "n1"); err != nil {
		t.Errorf("缓存 JWKS 后验签失败: %v", err)
	}
	// 篡改签名末字节 → RS256 验签失败
	parts := strings.Split(raw, ".")
	sig := []byte(parts[2])
	// 首字符的 6 位全部有效（末字符含填充位，翻转可能解码不变）
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	tampered := parts[0] + "." + parts[1] + "." + string(sig)
	if _, err := c.VerifyIDToken(ctx, tampered, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("RS256 篡改应拒绝: %v", err)
	}
}

func TestUnsupportedAlgRejected(t *testing.T) {
	f := newFake(t, "none-alg")
	c := NewClient(f.srv.URL, "cid", "sec")
	f.alg = "PLAINDANGEROUS"
	raw := f.sign(map[string]any{"iss": f.srv.URL, "aud": "cid", "sub": "s", "exp": time.Now().Unix() + 100})
	if _, err := c.VerifyIDToken(context.Background(), raw, ""); !strings.Contains(err.Error(), "不支持") {
		t.Errorf("未知算法应拒绝: %v", err)
	}
}
