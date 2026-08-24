// 回归：回收站存储未注入时不得 panic，必须返回结构化错误。
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	docservice "element-wiki/internal/service/docservice"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/internal/permission"

	"element-wiki/migrations"
)

func TestTrashWithoutWiringReturnsJSONNotPanic(t *testing.T) {
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
		VALUES ('ed','i','ed','','E','editor','active',1)`)

	impl := sqlitestore.New(db)
	// 关键：不调用 SetTrashHooks —— 复现生产接线遗漏
	svc := docservice.New(impl, impl, impl, impl, impl, 100)
	deps := Deps{
		Docs: svc, Trees: impl,
		ActorFor: func(*http.Request) permission.Actor {
			return permission.NewActor("ed", permission.CodesFor(permission.Editor))
		},
	}

	srv := httptest.NewServer(NewRouter(deps))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/v1/trash", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw := readAllBody(resp)
	if resp.StatusCode != 500 {
		t.Fatalf("未接线应 500, got %d %s", resp.StatusCode, raw)
	}
	if !strings.Contains(raw, `"detail"`) {
		t.Errorf("响应应为 JSON 错误结构: %s", raw)
	}
}
