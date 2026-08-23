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

func ioReadAll(r interface{ Read([]byte) (int, error) }) (string, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf), nil
}

func ioCopyDiscard(r interface{ Read([]byte) (int, error) }) {
	for {
		var tmp [1024]byte
		if _, err := r.Read(tmp[:]); err != nil {
			return
		}
	}
}

func jsonDecodeBody(r interface{ Read([]byte) (int, error) }) (string, error) {
	s, err := ioReadAll(r)
	return s, err
}
