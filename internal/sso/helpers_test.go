package sso

import (
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
)

const crypto_SHA256 = crypto.SHA256

func hmac_sha256(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func bigIntBytes(v int64) []byte {
	if v == 0 {
		return []byte{0}
	}
	out := make([]byte, 0, 8)
	for v > 0 {
		out = append([]byte{byte(v % 256)}, out...)
		v /= 256
	}
	return out
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
