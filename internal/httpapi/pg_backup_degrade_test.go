// T12.4 验收：driver=postgres 时备份导出与两种导入返回 501；其他驱动不被拦截。
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"element-wiki/internal/database"
	docservice "element-wiki/internal/service/docservice"
	backupservice "element-wiki/internal/service/backupservice"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func TestPostgresBackupDegradation(t *testing.T) {
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "pg-flag.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	impl := sqlitestore.New(db)
	docsSvc := docservice.New(impl, impl, impl, impl, impl, 100)
	bs := backupservice.New(impl, impl, db, filepath.Join(t.TempDir(), "l.db"),
		filepath.Join(t.TempDir(), "att"), filepath.Join(t.TempDir(), "bk"), 1)

	deps := Deps{
		Docs:             docsSvc,
		Trees:            impl,
		ActorFor:         actorFor,
		DBDriver:         "postgres",
		Backups:          bs,
		Jobs:             impl,
		Imports:          impl,
		MarkdownImports:  backupservice.NewMarkdownImporter(impl, nil, nil),
	}
	srv := httptest.NewServer(NewRouter(deps))
	defer srv.Close()

	post := func(path string) int {
		resp, err := http.Post(srv.URL+path, "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if got := post("/v1/admin/backups"); got != 501 {
		t.Fatalf("备份导出应 501, got %d", got)
	}
	if got := post("/v1/admin/imports"); got != 501 {
		t.Fatalf("备份导入应 501, got %d", got)
	}
	if got := post("/v1/admin/markdown-import"); got != 501 {
		t.Fatalf("Markdown 导入应 501, got %d", got)
	}
}

func TestPGGuardOnlyAffectsPostgres(t *testing.T) {
	pg := Deps{DBDriver: "postgres"}
	if !pg.pgBackupUnsupported(httptest.NewRecorder()) {
		t.Fatal("postgres 应被守卫拦截")
	}
	sqliteDeps := Deps{DBDriver: "sqlite"}
	if (&Deps{}).pgBackupUnsupported(httptest.NewRecorder()) ||
		sqliteDeps.pgBackupUnsupported(httptest.NewRecorder()) {
		t.Fatal("sqlite/空驱动不应被拦截")
	}
}
