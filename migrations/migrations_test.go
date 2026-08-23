package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestApplyFreshDBReachesLatest(t *testing.T) {
	db := newTestDB(t)
	m := &Migrator{DB: db, Dialect: "sqlite"}

	if err := m.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	current, err := m.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	latest, err := Latest("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if current != latest {
		t.Fatalf("当前版本 %d != 最新 %d", current, latest)
	}
}

func TestAppliedMigrationIsUsable(t *testing.T) {
	db := newTestDB(t)
	m := &Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	// settings 表可写可读，主键约束生效
	if _, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES ('k','v',1)`); err != nil {
		t.Fatalf("写入 settings: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES ('k','v2',2)`); err == nil {
		t.Fatal("重复主键必须被拒绝")
	}
	var got string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key='k'`).Scan(&got); err != nil || got != "v" {
		t.Fatalf("读回 = %q, err=%v", got, err)
	}
}

func TestApplyIdempotent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	m := &Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Apply(ctx); err != nil {
		t.Fatalf("重复 Apply 必须幂等: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	latest, _ := Latest("sqlite")
	if n != latest {
		t.Fatalf("版本记录数 %d != %d", n, latest)
	}
}

func TestVerifyUpToDateRejectsFutureDB(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	m := &Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.VerifyUpToDate(ctx); err != nil {
		t.Fatalf("刚迁移完应视为最新: %v", err)
	}

	// 模拟“新库旧二进制”
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (9999, '9999_future.sql', 1)`); err != nil {
		t.Fatal(err)
	}
	err := m.VerifyUpToDate(ctx)
	if err == nil || !strings.Contains(err.Error(), "禁止用旧程序启动新库") {
		t.Fatalf("应拒绝更高版本的库, got %v", err)
	}
}

func TestFSDialectValidation(t *testing.T) {
	if _, err := FS("mysql"); err == nil {
		t.Fatal("未知方言必须报错")
	}
	if _, err := FS("postgres"); err != nil {
		t.Fatalf("postgres 目录应存在: %v", err)
	}
}

func TestVersionOfInvalidNames(t *testing.T) {
	for _, name := range []string{"noext", "abcd.sql", "xx01_name.sql"} {
		if _, err := versionOf(name); err == nil {
			t.Errorf("%q 应报错", name)
		}
	}
	v, err := versionOf("0007_do_thing.sql")
	if err != nil || v != 7 {
		t.Fatalf("0007 解析 = %d, %v", v, err)
	}
}
