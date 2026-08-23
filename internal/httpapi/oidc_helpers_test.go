package httpapi

import (
	"context"
	"crypto"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"element-wiki/migrations"
)

func newAppliedMigrator(t *testing.T, db *sql.DB) *migrations.Migrator {
	t.Helper()
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m
}

func time_Now() int64 { return time.Now().Unix() }

const crypto_SHA256 = crypto.SHA256

func b64url(s string) string      { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
func b64urlBytes(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sha256Sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func mustParseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

type bodyReader = interface{ ReadAll() }

func url_Parse(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func context_Background() context.Context { return context.Background() }

func os_MkdirAll(path string) {
	_ = path
	// 目录由 SetAttachmentStore 调用方负责；这里仅占位保持测试可读
}
