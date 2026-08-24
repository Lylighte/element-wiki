// T6.1~T6.4 service 侧验收。
package adminservice

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	docservice "element-wiki/internal/service/docservice"
	"element-wiki/internal/store"
	sqlitestore "element-wiki/internal/store/sqlite"

	"element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/migrations"
)

func newSvc(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "adm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, role string }{
		{"ad", "admin"}, {"ed", "editor"},
	} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,role,status,created_at)
			VALUES (?, 'i', ?, ?, '', ?, 'active', 1)`, u.id, u.id, u.id+"@x.com", u.role); err != nil {
			t.Fatal(err)
		}
	}
	impl := sqlitestore.New(db)
	return New(impl, impl, impl), db
}

func admin() permission.Actor {
	return permission.NewActor("ad", permission.CodesFor(permission.Admin))
}
func viewerA() permission.Actor {
	return permission.NewActor("vw", permission.CodesFor(permission.Viewer))
}

func TestSettingsValidationMatrix(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()

	valid := map[string]string{
		"wiki_title": "T", "anonymous_read": "true", "max_versions": "5",
		"timezone": "Asia/Tokyo", "default_lang": "en",
	}
	if err := svc.UpdateSettings(ctx, admin(), valid); err != nil {
		t.Fatalf("合法批量更新失败: %v", err)
	}
	all, _ := svc.AllSettings(ctx, admin())
	for k, want := range valid {
		if all[k] != want {
			t.Errorf("%s = %q want %q", k, all[k], want)
		}
	}

	cases := []struct{ key, val string }{
		{"unknown_key", "1"}, {"wiki_title", " "},
		{"anonymous_read", "maybe"}, {"max_versions", "0"},
		{"timezone", "Nowhere"}, {"default_lang", "jp"},
	}
	for _, tc := range cases {
		err := svc.UpdateSettings(ctx, admin(), map[string]string{tc.key: tc.val})
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != tc.key {
			t.Errorf("(%s=%s) 应报字段 %s, got %v", tc.key, tc.val, tc.key, err)
		}
		// 零写入：其余键不受影响
		all2, _ := svc.AllSettings(ctx, admin())
		if all2["wiki_title"] != "T" {
			t.Fatalf("拒绝路径污染了其他键")
		}
	}

	if _, err := svc.AllSettings(ctx, viewerA()); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 读设置应拒绝: %v", err)
	}
}

func TestUserGovernance(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()

	list, err := svc.ListUsers(ctx, admin(), "", 100)
	if err != nil || len(list) < 2 {
		t.Fatalf("列表: %v %v", list, err)
	}
	filtered, _ := svc.ListUsers(ctx, admin(), "ed", 10)
	if len(filtered) != 1 || filtered[0].ID != "ed" {
		t.Errorf("过滤异常: %+v", filtered)
	}

	editorRole := permission.Editor
	got, err := svc.UpdateUser(ctx, admin(), "ed", &editorRole, nil)
	if err != nil || got.Role != permission.Editor {
		t.Fatalf("改角色: %v", err)
	}
	bad := permission.Role("root")
	if _, err := svc.UpdateUser(ctx, admin(), "ed", &bad, nil); !errors.Is(err, ErrValidation) {
		t.Errorf("非法角色应校验失败: %v", err)
	}
	self := "ad"
	if _, err := svc.UpdateUser(ctx, admin(), self, &editorRole, nil); !errors.Is(err, ErrValidation) &&
		!strings.Contains(err.Error(), "自己") {
		t.Logf("自操作防护返回: %v", err)
	}
	if _, err := svc.UpdateUser(ctx, admin(), "ghost", &editorRole, nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("幽灵用户应 NotFound: %v", err)
	}
	if err := svc.users.UpdateUserStatus(ctx, "ed", model.UserDisabled); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.users.GetUser(ctx, "ed"); err != nil {
		t.Fatal(err)
	}
	svc.users.UpdateUserStatus(ctx, "ed", model.UserActive)
}

func TestDashboardAggService(t *testing.T) {
	svc, db := newSvc(t)
	ctx := context.Background()
	impl := sqlitestore.New(db)
	dsvc := docservice.New(impl, impl, impl, impl, impl, 100)
	editor := permission.NewActor("ed", permission.CodesFor(permission.Editor))
	d1, _ := dsvc.CreateDocument(ctx, editor, nil, "dash-x", "X")
	dsvc.Commit(ctx, editor, d1.ID, "", "b", "m")

	st, err := svc.Dashboard(ctx, admin())
	if err != nil {
		t.Fatal(err)
	}
	if st.DocumentsTotal < 1 || len(st.RecentDocs) == 0 || st.RecentDocs[0].Slug != "dash-x" {
		t.Errorf("聚合异常: %+v", st)
	}
}
