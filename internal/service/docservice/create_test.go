// T1.5 验收：创建/改名/可见性/移动的业务规则与权限。
package docservice

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"database/sql"
	"element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"

	"element-wiki/internal/store"
	"element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func newSvc(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	impl := sqlite.New(db)
	svc := New(impl, impl, impl, impl, impl, 100)

	for _, u := range []string{"u1", "u2"} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
			VALUES (?, 'i', ?, '', '', 1)`, u, u); err != nil {
			t.Fatal(err)
		}
	}
	lastDB[svc] = db
	return svc, db
}

func editor() permission.Actor {
	return permission.NewActor("u1", permission.CodesFor(permission.Editor))
}
func viewer() permission.Actor {
	return permission.NewActor("u2", permission.CodesFor(permission.Viewer))
}

func ptr[T any](v T) *T { return &v }

var lastDB = map[*Service]*sql.DB{}

func isConflictErr(err error) bool { return errors.Is(err, store.ErrConflict) }

func TestCreateDocumentRules(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, err := svc.CreateDocument(ctx, act, nil, "getting-started", "入门指南")
	if err != nil {
		t.Fatalf("根文档创建: %v", err)
	}
	if d.ID == "" || d.Visibility != model.VisibilityStandard || d.HeadCommitID != "" {
		t.Errorf("默认字段异常: %+v", d)
	}

	if _, err := svc.CreateDocument(ctx, viewer(), nil, "x", "x"); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 创建应被拒: %v", err)
	}
	ghost := "no-parent"
	if _, err := svc.CreateDocument(ctx, act, &ghost, "x", "x"); !IsNotFound(err) {
		t.Errorf("幽灵父级应 NotFound: %v", err)
	}
	for _, tc := range []struct{ slug, title, field string }{
		{"Bad_Slug", "t", "slug"},
		{"ok-slug", "", "title"},
	} {
		_, err := svc.CreateDocument(ctx, act, nil, tc.slug, tc.title)
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != tc.field {
			t.Errorf("(%s,%s) 应报字段 %s, got %v", tc.slug, tc.title, tc.field, err)
		}
	}
	if _, err := svc.CreateDocument(ctx, act, nil, "getting-started", "另一个"); !isConflictErr(err) {
		t.Errorf("重复 slug 应冲突: %v", err)
	}
	child, err := svc.CreateDocument(ctx, act, &d.ID, "sub", "子页")
	if err != nil {
		t.Fatalf("子页创建: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != d.ID {
		t.Errorf("父子关系未建立: %+v", child.ParentID)
	}
}

func TestRenameAndVisibility(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "old", "旧标题")

	ns, nt := "new-slug", ptr("新标题")
	if err := svc.RenameDocument(ctx, act, d.ID, &ns, nt); err != nil {
		t.Fatalf("改名: %v", err)
	}
	got, _ := svc.Get(ctx, act, d.ID)
	if got.Slug != "new-slug" || got.Title != "新标题" {
		t.Errorf("改名未生效: %+v", got)
	}

	d2, _ := svc.CreateDocument(ctx, act, nil, "taken", "T")
	clash := "new-slug"
	if err := svc.RenameDocument(ctx, act, d2.ID, &clash, nil); !isConflictErr(err) {
		t.Errorf("改名撞 slug 应冲突: %v", err)
	}

	if err := svc.SetVisibility(ctx, act, d.ID, model.VisibilityRestricted); err != nil {
		t.Fatalf("设可见性: %v", err)
	}
	got, _ = svc.Get(ctx, act, d.ID)
	if got.Visibility != model.VisibilityRestricted {
		t.Errorf("可见性未生效: %s", got.Visibility)
	}
	if err := svc.SetVisibility(ctx, act, d.ID, model.Visibility("secret")); !errors.Is(err, ErrValidation) {
		t.Errorf("非法档位应校验失败: %v", err)
	}
}

func TestMoveRejectsSelfSubtree(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	root, _ := svc.CreateDocument(ctx, act, nil, "m-root", "R")
	mid, _ := svc.CreateDocument(ctx, act, &root.ID, "m-mid", "M")
	leaf, _ := svc.CreateDocument(ctx, act, &mid.ID, "m-leaf", "L")
	other, _ := svc.CreateDocument(ctx, act, nil, "m-other", "O")

	self := root.ID
	if err := svc.MoveDocument(ctx, act, root.ID, &self); !errors.Is(err, ErrSelfChild) {
		t.Errorf("移到自身应拒绝: %v", err)
	}
	if err := svc.MoveDocument(ctx, act, root.ID, &mid.ID); !errors.Is(err, ErrSelfChild) {
		t.Errorf("移入自身子树应拒绝: %v", err)
	}
	if err := svc.MoveDocument(ctx, act, root.ID, &leaf.ID); !errors.Is(err, ErrSelfChild) {
		t.Errorf("移入深层子孙应拒绝: %v", err)
	}

	if err := svc.MoveDocument(ctx, act, mid.ID, &other.ID); err != nil {
		t.Fatalf("合法移动失败: %v", err)
	}
	got, _ := svc.Get(ctx, act, mid.ID)
	if got.ParentID == nil || *got.ParentID != other.ID {
		t.Errorf("移动未生效: %+v", got.ParentID)
	}

	if err := svc.MoveDocument(ctx, act, mid.ID, nil); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, act, mid.ID)
	if got.ParentID != nil {
		t.Errorf("移回根应清除父级: %+v", got.ParentID)
	}

	// 移动后目标父级下同 slug 冲突（other 下已存在 dup 子节点）
	dupChild, _ := svc.CreateDocument(ctx, act, &other.ID, "dup", "D")
	dup, _ := svc.CreateDocument(ctx, act, nil, "dup", "D2")
	if err := svc.MoveDocument(ctx, act, dup.ID, &other.ID); !isConflictErr(err) {
		t.Errorf("移动撞同父 slug 应冲突: %v", err)
	}
	_ = dupChild
}
