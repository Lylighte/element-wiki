// T3.2/T3.4 用户与会话存储层验收。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

func TestUserStoreLifecycle(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()

	u := &model.User{ID: "u-a", Issuer: "iss", Subject: "sub-a",
		Email: "a@x.com", DisplayName: "A",
		Role: permission.Viewer, Status: model.UserActive,
		CreatedAt: 1, LastLoginAt: 1}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	got, err := s.FindUserByIssuerSubject(ctx, "iss", "sub-a")
	if err != nil || got.ID != "u-a" || got.Role != permission.Viewer {
		t.Fatalf("按身份查找: %+v %v", got, err)
	}
	if _, err := s.FindUserByIssuerSubject(ctx, "iss", "none"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失应 NotFound: %v", err)
	}
	if _, err := s.GetUser(ctx, "u-a"); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateUserRole(ctx, "u-a", permission.Admin); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUserStatus(ctx, "u-a", model.UserDisabled); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetUser(ctx, "u-a")
	if got.Role != permission.Admin || got.Status != model.UserDisabled {
		t.Errorf("角色/状态更新未生效: %+v", got)
	}
	for _, tc := range []struct {
		call string
		err  error
	}{
		{"role", s.UpdateUserRole(ctx, "ghost", permission.Editor)},
		{"status", s.UpdateUserStatus(ctx, "ghost", model.UserActive)},
	} {
		if !errors.Is(tc.err, store.ErrNotFound) {
			t.Errorf("%s 幽灵更新应 NotFound: %v", tc.call, tc.err)
		}
	}
	if err := s.TouchLogin(ctx, "u-a", 555); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetUser(ctx, "u-a")
	if got.LastLoginAt != 555 {
		t.Errorf("TouchLogin 未生效: %d", got.LastLoginAt)
	}
	n, _ := s.CountAdmins(ctx)
	if n != 1 {
		t.Fatalf("admins = %d", n)
	}
}

func TestSessionStoreCRUD(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	mustCreateUserForSession(t, s)

	if err := s.CreateSession(ctx, "h1", "u-s", 100); err != nil {
		t.Fatal(err)
	}
	uid, exp, err := s.GetSession(ctx, "h1")
	if err != nil || uid != "u-s" || exp != 100 {
		t.Fatalf("读回 = %s,%d,%v", uid, exp, err)
	}
	if _, _, err := s.GetSession(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失会话应 NotFound: %v", err)
	}
	if err := s.DeleteSession(ctx, "h1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, "h1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复删除应 NotFound: %v", err)
	}
}

func mustCreateUserForSession(t *testing.T, s *DB) {
	t.Helper()
	if err := s.CreateUser(context.Background(), &model.User{
		ID: "u-s", Issuer: "i", Subject: "s", DisplayName: "S",
		Role: permission.Viewer, Status: model.UserActive,
		CreatedAt: 1, LastLoginAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
}
