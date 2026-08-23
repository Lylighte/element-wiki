// T3.4 存储层验收。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func TestTokenStoreLifecycle(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)

	tk := &model.APIToken{ID: "t1", UserID: "u1", Name: "ci",
		Prefix: "ew_abc12", TokenHash: "hash-1", CreatedAt: 10}
	if err := s.CreateToken(ctx, tk); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTokenByHash(ctx, "hash-1")
	if err != nil || got.ID != "t1" || got.RevokedAt != nil {
		t.Fatalf("读回: %+v %v", got, err)
	}
	if _, err := s.GetTokenByHash(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失应 NotFound: %v", err)
	}
	if err := s.TouchToken(ctx, "t1", 99); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTokenByHash(ctx, "hash-1")
	if got.LastUsedAt != 99 {
		t.Errorf("touch 未生效: %d", got.LastUsedAt)
	}
	if err := s.RevokeToken(ctx, "t1", "u1", 100); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTokenByHash(ctx, "hash-1")
	if got.RevokedAt == nil || *got.RevokedAt != 100 {
		t.Errorf("吊销未生效: %+v", got.RevokedAt)
	}
	if err := s.RevokeToken(ctx, "t1", "u1", 101); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复吊销应 NotFound: %v", err)
	}
	list, _ := s.ListTokensByUser(ctx, "u1")
	if len(list) != 1 || list[0].ID != "t1" {
		t.Errorf("列表: %+v", list)
	}
	// 他人令牌不可见
	empty, _ := s.ListTokensByUser(ctx, "ghost")
	if len(empty) != 0 {
		t.Errorf("他人列表应为空: %+v", empty)
	}
	_ = context.Background
}
