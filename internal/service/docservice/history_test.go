// 历史版本增强验收：ListCommits 附带作者展示名；userLookup 缺失/用户不存在时回退 author_id。
package docservice

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
	"element-wiki/internal/store/sqlite"
)

func TestListCommitsAuthorName(t *testing.T) {
	svc, db := newSvc(t)
	ctx := context.Background()
	act := editor()

	// newSvc 已建 u1（display_name 为空），此处补展示名
	if _, err := db.Exec(`UPDATE users SET display_name = '张三' WHERE id = 'u1'`); err != nil {
		t.Fatal(err)
	}
	svc.SetCommentStore(nil, sqlite.New(db)) // 注入 userLookup 以解析展示名
	d, err := svc.CreateDocument(ctx, act, nil, "hist-doc", "H")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commit(ctx, act, d.ID, "", "v1", "first"); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListCommits(ctx, act, d.ID, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCommits = %d, %v", len(list), err)
	}
	if list[0].AuthorName != "张三" {
		t.Errorf("author_name = %q, want 张三", list[0].AuthorName)
	}
}

func TestListCommitsAuthorFallback(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "hist-fallback", "H")
	if _, err := svc.Commit(ctx, act, d.ID, "", "v1", "first"); err != nil {
		t.Fatal(err)
	}

	// userLookup 未注入（SetCommentStore 未调用）→ 回退 author_id
	list, err := svc.ListCommits(ctx, act, d.ID, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListCommits = %d, %v", len(list), err)
	}
	if list[0].AuthorName != "u1" {
		t.Errorf("无 userLookup 时 author_name 应回退 author_id, got %q", list[0].AuthorName)
	}

	// 注入 userLookup 但用户不存在 → 同样回退
	svc.SetCommentStore(nil, nopUserStore{})
	list2, _ := svc.ListCommits(ctx, act, d.ID, 10)
	if list2[0].AuthorName != "u1" {
		t.Errorf("用户不存在时 author_name 应回退, got %q", list2[0].AuthorName)
	}
}

// nopUserStore 仅满足接口（GetUser 恒 ErrNotFound 由 sqlite 实现之外的自定义替身承担）。
type nopUserStore struct {
	store.UserStore
}

func (nopUserStore) GetUser(ctx context.Context, id string) (*model.User, error) {
	return nil, errUserNotFoundForTest
}

var errUserNotFoundForTest = errors.New("user not found")
