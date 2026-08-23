package sso

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
)

// rsaKey 取（并缓存）kid 对应的 RSA 公钥。
// 锁序：本函数不持锁调用 Discover（其内部自加锁）；写回缓存时短暂加锁。
func (c *Client) rsaKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	if k, ok := c.rsaKeys[kid]; ok {
		c.mu.Unlock()
		return k, nil
	}
	disc := c.disc
	c.mu.Unlock()

	if disc == nil {
		d, err := c.Discover(ctx)
		if err != nil {
			return nil, err
		}
		disc = d
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, disc.JWKSURI, nil)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sso: jwks 请求失败: %w", err)
	}
	defer resp.Body.Close()
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("sso: jwks 解析失败: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range jwks.Keys {
		nb, errN := base64.RawURLEncoding.DecodeString(k.N)
		eb, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(nb) == 0 || len(eb) == 0 {
			continue
		}
		pub := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}
		c.rsaKeys[k.Kid] = pub
	}
	if k, ok := c.rsaKeys[kid]; ok {
		return k, nil
	}
	if kid == "" && len(c.rsaKeys) > 0 {
		for _, k := range c.rsaKeys {
			return k, nil // 单钥匙场景允许空 kid
		}
	}
	return nil, fmt.Errorf("sso: jwks 无匹配 kid %q", kid)
}
