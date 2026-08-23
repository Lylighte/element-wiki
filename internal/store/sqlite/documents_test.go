// T1.1 验收：documents store CRUD + slug 部分唯一索引行为。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

func doc(t *testing.T, id, parent, slug string) *model.Document {
	t.Helper()
	p := ptr(parent)
	if parent == "" {
		p = nil
	}
	now := nowMs()
	return &model.Document{
		ID: id, ParentID: p, Slug: slug, Title: "T-" + slug,
		SortKey: 100, Visibility: model.VisibilityStandard,
		CreatedBy: "u1", UpdatedBy: "u1", CreatedAt: now, UpdatedAt: now,
	}
}

func TestCreateAndGetRoundtrip(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	in := doc(t, util.NewID(), "", "root-doc")
	if err := s.Create(ctx, in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(ctx, in.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Slug != "root-doc" || got.Title != "T-root-doc" || got.SortKey != 100 {
		t.Errorf("roundtrip 字段不符: %+v", got)
	}
	if got.ParentID != nil || got.DeletedAt != nil || got.SpaceID != nil {
		t.Errorf("NULL 列应保持 NULL: %+v", got)
	}
	if !got.Alive() {
		t.Error("新文档必须存活")
	}

	if _, err := s.Get(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失应 ErrNotFound, got %v", err)
	}
}

func TestGetBySlugRespectsTrashAndParentScope(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	root := doc(t, util.NewID(), "", "guide")
	child := doc(t, util.NewID(), root.ID, "guide") // 不同父级同 slug 合法
	for _, d := range []*model.Document{root, child} {
		if err := s.Create(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.GetBySlug(ctx, nil, "guide", false)
	if err != nil || got.ID != root.ID {
		t.Fatalf("根级查找 = %v,%v", got, err)
	}
	got2, err := s.GetBySlug(ctx, ptr(root.ID), "guide", false)
	if err != nil || got2.ID != child.ID {
		t.Fatalf("子级查找 = %v,%v", got2, err)
	}

	// 进回收站后 includeDeleted=false 不可见，true 可见
	if err := trashDoc(t, s, root.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBySlug(ctx, nil, "guide", false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("回收站文档对存活查询应不可见, got %v", err)
	}
	got3, err := s.GetBySlug(ctx, nil, "guide", true)
	if err != nil || got3.ID != root.ID {
		t.Errorf("includeDeleted 应命中回收站文档: %v,%v", got3, err)
	}
}

func TestCreateDuplicateSlugReturnsConflict(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	a := doc(t, util.NewID(), "", "same")
	b := doc(t, util.NewID(), "", "same")
	if err := s.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	err := s.Create(ctx, b)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("重复 slug 应映射 ErrConflict, got %v", err)
	}
}

func TestListChildrenOrderAndFiltering(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	parent := doc(t, util.NewID(), "", "p")
	mustCreate(t, s, ctx, parent)

	c2 := doc(t, util.NewID(), parent.ID, "bbb")
	c2.SortKey = 200
	c1 := doc(t, util.NewID(), parent.ID, "zzz")
	c1.SortKey = 100
	trashed := doc(t, util.NewID(), parent.ID, "aaa")

	for _, d := range []*model.Document{c2, c1} {
		mustCreate(t, s, ctx, d)
	}
	mustCreate(t, s, ctx, trashed)
	mustTrashRaw(t, s, trashed.ID)

	kids, err := s.ListChildren(ctx, ptr(parent.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 2 {
		t.Fatalf("子节点数 = %d, 回收站项不应出现", len(kids))
	}
	if kids[0].Slug != "zzz" || kids[1].Slug != "bbb" {
		t.Errorf("应按 sort_key 排序: %s,%s", kids[0].Slug, kids[1].Slug)
	}

	roots, err := s.ListChildren(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].ID != parent.ID {
		t.Errorf("根列表 = %+v", roots)
	}
}

func TestUpdateMetaPartialAndNotFound(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	if _, err := rawOf(s).Exec(`INSERT INTO users (id,issuer,subject,email,display_name,created_at)
		VALUES ('u9','i','s9','','updater',1)`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	d := doc(t, util.NewID(), "", "m1")
	mustCreate(t, s, ctx, d)

	newTitle := "Renamed"
	newVis := model.VisibilityRestricted
	newSort := int64(7)
	if err := s.UpdateMeta(ctx, d.ID, model.DocumentMut{Title: &newTitle, SortKey: &newSort, Visibility: &newVis}, "u9", 999); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}
	got, _ := s.Get(ctx, d.ID)
	if got.Title != "Renamed" || got.SortKey != 7 || got.Visibility != model.VisibilityRestricted {
		t.Errorf("部分更新未生效: %+v", got)
	}
	if got.UpdatedBy != "u9" || got.UpdatedAt != 999 {
		t.Errorf("updated_by/at 未刷新: %+v", got)
	}
	if got.Slug != "m1" {
		t.Errorf("nil 字段不得被修改: %q", got.Slug)
	}

	err := s.UpdateMeta(ctx, "ghost", model.DocumentMut{}, "u1", 1)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("更新不存在文档应 ErrNotFound, got %v", err)
	}
}

func TestMoveSetAndClearParent(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	root := doc(t, util.NewID(), "", "r")
	target := doc(t, util.NewID(), "", "t2")
	mustCreate(t, s, ctx, root)
	mustCreate(t, s, ctx, target)

	if err := s.Move(ctx, target.ID, ptr(root.ID), "u1", 10); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, target.ID)
	if got.ParentID == nil || *got.ParentID != root.ID {
		t.Fatalf("移动到父级失败: %+v", got.ParentID)
	}

	if err := s.Move(ctx, target.ID, nil, "u1", 11); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, target.ID)
	if got.ParentID != nil {
		t.Fatalf("移回根应清除父级: %+v", got.ParentID)
	}
	if err := s.Move(ctx, "ghost", nil, "u1", 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("移动不存在应 ErrNotFound, got %v", err)
	}
}
