// T1.2 验收：子树收集 + 生效可见性沿祖先链继承。
package sqlite

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func TestEffectiveVisibilityInheritsFromAnyAncestor(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ctx := context.Background()

	cases := []struct {
		name         string
		path         []string // 链上各节点 slug
		restrictedAt int      // 从 1 数；0 = 全 standard
		want         model.Visibility
	}{
		{"全 standard 链", []string{"a", "b", "c"}, 0, model.VisibilityStandard},
		{"根 restricted 向下传染", []string{"r", "b", "c"}, 1, model.VisibilityRestricted},
		{"中层 restricted 传染叶与更深层", []string{"a", "mid", "leaf", "bottom"}, 2, model.VisibilityRestricted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := make(map[string]*model.Document)
			var parent *string
			for i, raw := range tc.path {
				slug := strconv.Itoa(tc.restrictedAt) + raw + strconv.Itoa(i)
				d := doc(t, util_NewID(), deref(parent), slug)
				if tc.restrictedAt == i+1 {
					d.Visibility = model.VisibilityRestricted
				}
				mustCreate(t, s, ctx, d)
				nodes[raw] = d
				pid := d.ID
				parent = &pid
			}
			leaf := nodes[tc.path[len(tc.path)-1]]
			got, err := s.(store.TreeStore).EffectiveVisibility(ctx, leaf.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("叶节点生效可见性 = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSubtreeIDsExcludesTrashedBranches(t *testing.T) {
	s := newDocStore(t)
	seedUserRow(t, s)
	ts := s.(store.TreeStore)
	ctx := context.Background()

	rootD := doc(t, util_NewID(), "", "root")
	live := doc(t, util_NewID(), rootD.ID, "live")
	dead := doc(t, util_NewID(), rootD.ID, "dead")
	deadChild := doc(t, util_NewID(), dead.ID, "deadchild")

	mustCreate(t, s, ctx, rootD)
	mustCreate(t, s, ctx, live)
	mustCreate(t, s, ctx, dead)
	mustCreate(t, s, ctx, deadChild)

	mustTrashRaw(t, s, dead.ID)

	ids, err := ts.SubtreeIDs(ctx, rootD.ID)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	if len(ids) != 2 || !slices.Contains(ids, rootD.ID) || !slices.Contains(ids, live.ID) {
		t.Errorf("子树应只含存活分支: %v", ids)
	}

	if _, err := ts.SubtreeIDs(ctx, "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("不存在根应 ErrNotFound, got %v", err)
	}
}

func TestEffectiveVisibilityMissingDoc(t *testing.T) {
	s := newDocStore(t)
	ts := s.(store.TreeStore)
	if _, err := ts.EffectiveVisibility(context.Background(), "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("缺失文档应 ErrNotFound, got %v", err)
	}
}
