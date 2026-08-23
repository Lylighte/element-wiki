package sso

import (
	"crypto"
	"encoding/base64"
	"strings"
)

const cryptoSHA256 = crypto.SHA256

func b64JSON(seg string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(seg)
}

func normIssuer(s string) string { return strings.TrimSuffix(s, "/") }

func audContains(aud any, clientID string) bool {
	switch v := aud.(type) {
	case string:
		return v == clientID
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == clientID {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == clientID {
				return true
			}
		}
	}
	return false
}
