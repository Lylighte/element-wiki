package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"element-wiki/internal/database"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
	"element-wiki/migrations"
)

// ---- 测试基础设施（包内共享） ----

func openMigrated(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newDocStore(t *testing.T) store.DocumentStore {
	t.Helper()
	return New(openMigrated(t))
}

func rawOf(s store.DocumentStore) *sql.DB { return s.(*DB).db }

func ptr[T any](v T) *T { return &v }

const seedUserSQL = `INSERT INTO users (id,issuer,subject,email,display_name,created_at)
	VALUES ('u1','i','s','','seed',1)`

func seedUserRow(t *testing.T, s store.DocumentStore) {
	t.Helper()
	if _, err := rawOf(s).Exec(seedUserSQL); err != nil {
		t.Fatal(err)
	}
}

func mustCreate(t *testing.T, s store.DocumentStore, ctx context.Context, d *model.Document) {
	t.Helper()
	if err := s.Create(ctx, d); err != nil {
		t.Fatalf("Create %s: %v", d.Slug, err)
	}
}

func mustTrashRaw(t *testing.T, s store.DocumentStore, id string) {
	t.Helper()
	if _, err := rawOf(s).Exec(`UPDATE documents SET deleted_at=100 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
}

func trashDoc(t *testing.T, s store.DocumentStore, id string) error {
	t.Helper()
	return func() error {
		_, err := rawOf(s).Exec(`UPDATE documents SET deleted_at=100 WHERE id=?`, id)
		return err
	}()
}

// 测试别名，避免各文件重复导入 util。
var (
	util_NewID = func() string { return newID() }
)

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
