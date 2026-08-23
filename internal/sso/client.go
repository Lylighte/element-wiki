// Package sso 是仅覆盖本项目所需的极简 OIDC 客户端：
// 发现文档、授权码 + PKCE 换取令牌、HS256/RS256 ID Token 验签。
package sso

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Discovery 是 openid-configuration 的必需子集。
type Discovery struct {
	Issuer        string `json:"issuer"`
	AuthEndpoint  string `json:"authorization_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
	JWKSURI       string `json:"jwks_uri"`
}

// Claims 是 ID Token 中本项目消费的字段。
type Claims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Nonce             string `json:"nonce"`
	Audience          any    `json:"aud"`
	Issuer            string `json:"iss"`
	Expiry            int64  `json:"exp"`
}

var ErrInvalid = errors.New("sso: id_token 校验失败")

// Client 按 issuer 惰性缓存发现文档与 JWKS。
type Client struct {
	Issuer     string
	ClientID   string
	Secret     string
	HTTPClient *http.Client

	mu      sync.Mutex
	disc    *Discovery
	rsaKeys map[string]*rsa.PublicKey // kid -> key（JWKS 缓存）
}

func NewClient(issuer, clientID, secret string) *Client {
	return &Client{Issuer: issuer, ClientID: clientID, Secret: secret,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		rsaKeys:    map[string]*rsa.PublicKey{}}
}

// Discover 获取并缓存发现文档。
func (c *Client) Discover(ctx context.Context) (*Discovery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disc != nil {
		return c.disc, nil
	}
	wellKnown := strings.TrimSuffix(c.Issuer, "/") + "/.well-known/openid-configuration"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sso: 发现请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sso: 发现返回 %d", resp.StatusCode)
	}
	var d Discovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("sso: 发现解析失败: %w", err)
	}
	if d.AuthEndpoint == "" || d.TokenEndpoint == "" {
		return nil, errors.New("sso: 发现文档缺端点")
	}
	c.disc = &d
	return c.disc, nil
}

// RandomB64 生成 url-safe 随机串。
func RandomB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// PKCEPair 生成 verifier 与 S256 challenge。
func PKCEPair() (verifier, challenge string) {
	v := RandomB64(32)
	sum := sha256.Sum256([]byte(v))
	return v, base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthURL 构造授权跳转地址。
func (c *Client) AuthURL(d *Discovery, redirectURI, state, nonce, challenge string, scopes []string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(scopes) == 0 {
		scopes = []string{"openid"}
	}
	q.Set("scope", strings.Join(scopes, " "))
	sep := "?"
	if strings.Contains(d.AuthEndpoint, "?") {
		sep = "&"
	}
	return d.AuthEndpoint + sep + q.Encode()
}

// Exchange 用授权码换取 id_token（POST 表单 + client_secret_post）。
func (c *Client) Exchange(ctx context.Context, d *Discovery, redirectURI, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.Secret)
	if verifier != "" {
		form.Set("code_verifier", verifier)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint,
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sso: 令牌交换失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("sso: 令牌交换返回 %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.IDToken == "" {
		return "", errors.New("sso: 响应缺少 id_token")
	}
	return tok.IDToken, nil
}

// VerifyIDToken 验签并校验 iss/aud/exp/nonce，返回 Claims。
func (c *Client) VerifyIDToken(ctx context.Context, raw, nonce string) (*Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrInvalid
	}
	headerJSON, err := b64JSON(parts[0])
	if err != nil {
		return nil, ErrInvalid
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, ErrInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalid
	}
	signed := []byte(parts[0] + "." + parts[1])

	switch header.Alg {
	case "HS256":
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write(signed)
		if !hmac.Equal(sig, mac.Sum(nil)) {
			return nil, ErrInvalid
		}
	case "RS256":
		key, kerr := c.rsaKey(ctx, header.Kid)
		if kerr != nil {
			return nil, kerr
		}
		digest := sha256.Sum256(signed)
		if err := rsa.VerifyPKCS1v15(key, cryptoSHA256, digest[:], sig); err != nil {
			return nil, ErrInvalid
		}
	default:
		return nil, fmt.Errorf("%w: 不支持的算法 %s", ErrInvalid, header.Alg)
	}

	claimsJSON, err := b64JSON(parts[1])
	if err != nil {
		return nil, ErrInvalid
	}
	var cl Claims
	if err := json.Unmarshal(claimsJSON, &cl); err != nil {
		return nil, ErrInvalid
	}
	now := time.Now().Unix()
	if cl.Expiry > 0 && cl.Expiry < now-60 {
		return nil, fmt.Errorf("%w: token 已过期", ErrInvalid)
	}
	if normIssuer(cl.Issuer) != normIssuer(c.Issuer) {
		return nil, fmt.Errorf("%w: issuer 不匹配", ErrInvalid)
	}
	if !audContains(cl.Audience, c.ClientID) {
		return nil, fmt.Errorf("%w: audience 不匹配", ErrInvalid)
	}
	if nonce != "" && cl.Nonce != nonce {
		return nil, fmt.Errorf("%w: nonce 不匹配", ErrInvalid)
	}
	return &cl, nil
}
