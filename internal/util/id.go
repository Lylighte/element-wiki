// Package util 提供通用工具。
package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	lastT   time.Time
	entropy = rand.Reader
)

// NewID 生成时间有序的 ULID（TEXT 主键约定，doc/01 §1）。
func NewID() string {
	mu.Lock()
	defer mu.Unlock()
	t := time.Now()
	if !t.After(lastT) {
		t = lastT.Add(1) // 同毫秒内保证单调
	}
	lastT = t
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// SHA256Hex 返回内容的十六进制 sha256（blob 内容寻址）。
func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
