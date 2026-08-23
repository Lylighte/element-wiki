package docservice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

// 补充边界路径，保证分支覆盖。
func TestHeadContentEmptyAndMissing(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "empty-doc", "E")
	body, head, err := svc.HeadContent(ctx, act, d.ID)
	if err != nil || body != "" || head != nil {
		t.Fatalf("无版本文档 HEAD 应为空三元组: %q %+v %v", body, head, err)
	}
	if _, _, err := svc.HeadContent(ctx, act, "ghost"); !IsNotFound(err) {
		t.Errorf("缺失文档应 NotFound: %v", err)
	}
}

func TestReadPermissionMatrix(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "perm-doc", "P")
	svc.Commit(ctx, act, d.ID, "", "c", "m")

	// viewer 持有 version.read，可读历史
	if _, err := svc.ListCommits(ctx, viewer(), d.ID, 10); err != nil {
		t.Errorf("viewer 应可读历史: %v", err)
	}
	// 未认证匿名 actor（匿名模式关闭）被拒
	if _, err := svc.ListCommits(ctx, permission.Anonymous(false), d.ID, 10); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("匿名读历史应被拒: %v", err)
	}
	_ = model.VisibilityStandard
}

func TestRenameOnlyTitleWhenSlugEmpty(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "rt", "旧")

	nt := ptr("仅新标题")
	empty := ""
	if err := svc.RenameDocument(ctx, act, d.ID, &empty, nt); err != nil {
		t.Fatalf("空 slug 视为不改: %v", err)
	}
	got, _ := svc.Get(ctx, act, d.ID)
	if got.Slug != "rt" || got.Title != "仅新标题" {
		t.Errorf("slug=%q title=%q", got.Slug, got.Title)
	}

	long := strings.Repeat("长", 201)
	if err := svc.RenameDocument(ctx, act, d.ID, nil, &long); !errors.Is(err, ErrValidation) {
		t.Errorf("超长标题应校验失败: %v", err)
	}
}

func TestCommitOnTrashedDocInvisible(t *testing.T) {
	svc, db := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "trashed-target", "T")
	if _, err := db.Exec(`UPDATE documents SET deleted_at=1 WHERE id=?`, d.ID); err != nil {
		t.Fatal(err)
	}

	err := func() error {
		_, e := svc.Commit(ctx, act, d.ID, "", "x", "m")
		return e
	}()
	if !IsNotFound(err) {
		t.Errorf("回收站文档对外不可见(404 掩护): %v", err)
	}
	if _, err := svc.Get(ctx, act, d.ID); !IsNotFound(err) {
		t.Errorf("Get 回收站文档应 NotFound: %v", err)
	}
	_ = store.ErrConflict
}

func TestDeadLinksDedup(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "dedup", "D")
	res, _ := svc.Commit(ctx, act, d.ID, "", "[[nope]] [[nope]] [[alsonope]]", "m")
	if len(res.DeadLinks) != 2 {
		t.Errorf("死链去重失败: %v", res.DeadLinks)
	}
}

// 幽灵 ID 全方法矩阵：一律 404 语义。
func TestGhostIDMatrix(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	ghost := "ghost-id"

	if _, err := svc.GetDraft(ctx, act, ghost); !IsNotFound(err) {
		t.Errorf("GetDraft: %v", err)
	}
	if err := svc.DeleteDraft(ctx, act, ghost); !IsNotFound(err) {
		t.Errorf("DeleteDraft: %v", err)
	}
	if err := svc.SaveDraft(ctx, act, ghost, "b", "c"); !IsNotFound(err) {
		t.Errorf("SaveDraft: %v", err)
	}
	if err := svc.SetVisibility(ctx, act, ghost, model.VisibilityStandard); !IsNotFound(err) {
		t.Errorf("SetVisibility: %v", err)
	}
	if err := svc.MoveDocument(ctx, act, ghost, nil); !IsNotFound(err) {
		t.Errorf("Move: %v", err)
	}
	ns := "ns"
	if err := svc.RenameDocument(ctx, act, ghost, &ns, nil); !IsNotFound(err) {
		t.Errorf("Rename: %v", err)
	}
	if _, err := svc.Revert(ctx, act, ghost, "any"); !IsNotFound(err) {
		t.Errorf("Revert: %v", err)
	}
	if _, err := svc.Commit(ctx, act, ghost, "", "c", "m"); !IsNotFound(err) {
		t.Errorf("Commit: %v", err)
	}
}

func TestErrorTypesMessage(t *testing.T) {
	vc := &VersionConflictError{HeadCommitID: "h1"}
	if vc.Error() == "" || !strings.Contains(vc.Error(), "h1") {
		t.Errorf("冲突错误信息应含 HEAD: %q", vc.Error())
	}
	ve := &ValidationError{Field: "slug", Reason: "bad"}
	if !strings.Contains(ve.Error(), "slug") || !strings.Contains(ve.Error(), "bad") {
		t.Errorf("校验错误信息异常: %q", ve.Error())
	}
}

// viewer 删除草稿走权限拒绝分支。
func TestDeleteDraftDeniedForViewer(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "dd-v", "D")
	svc.SaveDraft(ctx, act, d.ID, "b", "c")
	if err := svc.DeleteDraft(ctx, viewer(), d.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 删草稿应拒绝: %v", err)
	}
}

func containsStr(h, n string) bool {
	return len(n) == 0 || (len(h) >= len(n) && (func() bool {
		for i := 0; i+len(n) <= len(h); i++ {
			if h[i:i+len(n)] == n {
				return true
			}
		}
		return false
	})())
}

// ListChildrenForTree 对无 restricted 读权限者过滤受限节点。
func TestTreeFilteringForViewer(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	pub, _ := svc.CreateDocument(ctx, act, nil, "tree-pub", "P")
	sec, _ := svc.CreateDocument(ctx, act, nil, "tree-sec", "S")
	svc.SetVisibility(ctx, act, sec.ID, model.VisibilityRestricted)

	kids, err := svc.ListChildrenForTree(ctx, viewer(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 1 || kids[0].ID != pub.ID {
		t.Errorf("viewer 树应过滤受限节点: %+v", kids)
	}
	kids2, err := svc.ListChildrenForTree(ctx, editor(), nil)
	if err != nil || len(kids2) != 2 {
		t.Errorf("editor 树应全量: %+v %v", kids2, err)
	}
}
