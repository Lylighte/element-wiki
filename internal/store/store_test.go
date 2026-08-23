package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteAndPing(t *testing.T) {
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenUnknownDialect(t *testing.T) {
	if _, err := Open("mysql", "x"); err == nil {
		t.Fatal("未知方言必须报错")
	}
}

func TestOpenPostgresNotImplementedYet(t *testing.T) {
	if _, err := Open("postgres", "postgres://localhost/x"); err == nil {
		t.Fatal("未实现的适配器必须显式报错而不是静默返回")
	}
}

func TestWithSQLitePragmas(t *testing.T) {
	cases := map[string]string{
		"data/x.db":        "data/x.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)",
		"file:x.db?mode=r": "file:x.db?mode=r&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)",
	}
	for in, want := range cases {
		if got := withSQLitePragmas(in); got != want {
			t.Errorf("withSQLitePragmas(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOpenSQLiteRejectsGarbageDSN(t *testing.T) {
	// modernc/sqlite 对非法 DSN 在首次使用时报错，Open 本身应不 panic
	db, err := Open("sqlite", "data/\x00bad\x00.db")
	if err == nil {
		db.Close()
	}
}
