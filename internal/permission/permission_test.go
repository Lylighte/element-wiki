package permission

import (
	"errors"
	"slices"
	"testing"
)

// catalog 与角色映射的一致性守卫（AGENTS §4：新增权限必须同步）。
func TestCatalogIntegrity(t *testing.T) {
	if len(AllCodes) != 23 {
		t.Fatalf("目录数量 %d != 契约 23，若为有意扩展请同步 doc/02 §13 与前端 permissions", len(AllCodes))
	}
	sorted := slices.Clone(AllCodes)
	if !slices.IsSorted(sorted) {
		t.Error("AllCodes 应保持排序以便 diff 审查")
	}
}

func TestRoleMappingCompleteness(t *testing.T) {
	viewer := CodesFor(Viewer)
	editor := CodesFor(Editor)
	admin := CodesFor(Admin)

	// admin 必须拥有全目录
	for _, c := range AllCodes {
		if !slices.Contains(admin, c) {
			t.Errorf("admin 缺少 %s", c)
		}
	}
	// editor ⊃ viewer；admin ⊃ editor
	for _, c := range viewer {
		if !slices.Contains(editor, c) {
			t.Errorf("editor 缺少 viewer 权限 %s", c)
		}
	}
	for _, c := range editor {
		if !slices.Contains(admin, c) {
			t.Errorf("admin 缺少 editor 权限 %s", c)
		}
	}
	// 关键差异点
	for _, denied := range []string{DocReadRestricted, DocUpdate, DocDeleteAny()} {
		if slices.Contains(viewer, denied) {
			t.Errorf("viewer 不应持有 %s", denied)
		}
	}
	if !slices.Contains(editor, DocReadRestricted) || !slices.Contains(editor, DocUpdate) {
		t.Error("editor 应持有 restricted 读与更新")
	}
	if slices.Contains(admin, "nonexistent.code") {
		t.Error("不可能分支")
	}
	// viewer 持有评论创建与自己删除
	if !slices.Contains(viewer, CommentCreate) || !slices.Contains(viewer, CommentDeleteOwn) {
		t.Error("viewer 评论权限缺失")
	}
	// CommentDeleteAny 仅 admin
	if slices.Contains(editor, CommentDeleteAny) {
		t.Error("CommentDeleteAny 不应给 editor")
	}
}

func DocDeleteAny() string { return CommentDeleteAny }

func TestActorBehavior(t *testing.T) {
	a := NewActor("u1", CodesFor(Editor))
	if a.UserID() != "u1" {
		t.Fatal("UserID")
	}
	if err := a.Require(DocRead, DocUpdate); err != nil {
		t.Errorf("editor 应通过双码校验: %v", err)
	}
	if err := a.Require(UserManage); !errors.Is(err, ErrDenied) {
		t.Errorf("越权应返回 ErrDenied, got %v", err)
	}
	if a.Has(UserManage) || a.HasAny(UserManage, SettingsManage) {
		t.Error("Has/HasAny 越权误报")
	}
	if !a.HasAny(UserManage, DocUpdate) {
		t.Error("HasAny 任一命中应 true")
	}
}

func TestAnonymousActor(t *testing.T) {
	off := Anonymous(false)
	if off.Has(DocRead) {
		t.Error("匿名关闭时不应有任何读权限")
	}
	on := Anonymous(true)
	for _, c := range []string{DocRead, VersionRead, AttachmentRead, CommentRead} {
		if !on.Has(c) {
			t.Errorf("匿名只读缺少 %s", c)
		}
	}
	for _, c := range []string{DocCreate, DocReadRestricted, UserManage} {
		if on.Has(c) {
			t.Errorf("匿名不应持有 %s", c)
		}
	}
	if on.UserID() != "" {
		t.Error("匿名 UserID 必须为空串")
	}
}

func TestRoleValid(t *testing.T) {
	for r, want := range map[Role]bool{
		Viewer: true, Editor: true, Admin: true,
		Role("root"): false, Role(""): false,
	} {
		if r.Valid() != want {
			t.Errorf("Role(%q).Valid() = %v", r, want)
		}
	}
}
