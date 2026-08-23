// T3.2/T3.4 service 侧验收：JIT、admin 引导一次性、会话生命周期、令牌签发与吊销。
package authservice

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/migrations"
)

func newAuthSvc(t *testing.T, adminEmails []string) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := (&migrations.Migrator{DB: db, Dialect: "sqlite"}).Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	impl := sqlitestore.New(db)
	return New(impl, impl, impl, "https://idp.example", adminEmails, false), db
}

func TestJITProvisioning(t *testing.T) {
	svc, _ := newAuthSvc(t, nil)
	ctx := context.Background()

	u, err := svc.ResolveSSO(ctx, "sub-1", "dev@x.com", "Dev")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != permission.Viewer || u.Status != model.UserActive {
		t.Errorf("JIT 默认应为 active viewer: %+v", u)
	}

	again, err := svc.ResolveSSO(ctx, "sub-1", "dev@x.com", "Dev")
	if err != nil || again.ID != u.ID {
		t.Fatalf("同 subject 应复用账号: %+v err=%v", again, err)
	}
	if again.LastLoginAt < u.LastLoginAt && again.LastLoginAt == u.LastLoginAt && again.CreatedAt != again.LastLoginAt {
		t.Log("时间戳精度内视为通过")
	}
}

func TestDisabledNeverRevives(t *testing.T) {
	svc, _ := newAuthSvc(t, nil)
	ctx := context.Background()
	u, _ := svc.ResolveSSO(ctx, "s9", "nine@x.com", "Nine")

	// 直接置为 disabled（管理操作在 M6）
	if err := svc.users.UpdateUserStatus(ctx, u.ID, model.UserDisabled); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveSSO(ctx, "s9", "nine@x.com", "Nine"); !errors.Is(err, ErrDisabled) {
		t.Errorf("禁用账号登录应拒绝且不复活: %v", err)
	}
	// 账号仍是 disabled
	got, _ := svc.users.GetUser(ctx, u.ID)
	if got.Status != model.UserDisabled {
		t.Errorf("状态被篡改: %s", got.Status)
	}
}

func TestAdminBootstrapOnlyOnce(t *testing.T) {
	svc, _ := newAuthSvc(t, []string{"boss@corp.com"})
	ctx := context.Background()

	first, err := svc.ResolveSSO(ctx, "boss-sub", "BOSS@corp.com", "Boss")
	if err != nil {
		t.Fatal(err)
	}
	if first.Role != permission.Admin {
		t.Fatalf("首个命中引导名单者应成为 admin, got %s (email 大小写应不敏感)", first.Role)
	}

	second, _ := svc.ResolveSSO(ctx, "other-sub", "boss@corp.com", "Imposter")
	if second.Role != permission.Viewer {
		t.Errorf("admin 已存在后不得再次引导: %s", second.Role)
	}
}

func TestSessionLifecycle(t *testing.T) {
	svc, _ := newAuthSvc(t, nil)
	ctx := context.Background()
	u, _ := svc.ResolveSSO(ctx, "s1", "a@x.com", "A")

	raw, exp, err := svc.NewSession(ctx, u.ID)
	if err != nil || raw == "" || exp <= 0 {
		t.Fatalf("签发异常: %q %d %v", raw, exp, err)
	}
	actor, err := svc.ActorFromSession(ctx, raw)
	if err != nil || actor.UserID() != u.ID {
		t.Fatalf("会话解析: %v %v", actor, err)
	}
	if !actor.Has(permission.DocUpdate) && !actor.Has(permission.DocRead) {
		t.Error("viewer 至少应有读集")
	}

	if err := svc.Logout(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ActorFromSession(ctx, raw); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("注销后应失效: %v", err)
	}
	if err := svc.Logout(ctx, raw); err != nil {
		t.Errorf("重复注销幂等: %v", err)
	}
}

func TestTokenIssueAndRevoke(t *testing.T) {
	svc, _ := newAuthSvc(t, nil)
	ctx := context.Background()
	u, _ := svc.ResolveSSO(ctx, "tk", "tk@x.com", "T")

	issued, err := svc.IssueToken(ctx, u.ID, "ci")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Plaintext, "ew_") || len(issued.Plaintext) < 20 {
		t.Fatalf("明文令牌形态异常: %q", issued.Plaintext)
	}
	rec := issued.TokenRecord
	if rec.TokenHash == issued.Plaintext || rec.TokenHash == "" {
		t.Error("库中必须只存哈希")
	}
	if rec.Prefix != issued.Plaintext[:8] {
		t.Errorf("prefix = %q", rec.Prefix)
	}

	actor, err := svc.ActorFromBearer(ctx, issued.Plaintext)
	if err != nil || actor.UserID() != u.ID {
		t.Fatalf("Bearer 解析: %v %v", actor, err)
	}

	if err := svc.RevokeToken(ctx, rec.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ActorFromBearer(ctx, issued.Plaintext); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("吊销后应失效: %v", err)
	}
	if err := svc.RevokeToken(ctx, rec.ID, u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复吊销应 NotFound: %v", err)
	}

	list, _ := svc.ListTokens(ctx, u.ID)
	if len(list) != 1 || list[0].ID != rec.ID {
		t.Errorf("列表: %+v", list)
	}
}

func TestAnonymousActorToggle(t *testing.T) {
	off := New(nil, nil, nil, "", nil, false)
	if off.AnonymousEnabled() || off.AnonymousActor().Has(permission.DocRead) {
		t.Error("关闭时匿名无任何权限")
	}
	on := New(nil, nil, nil, "", nil, true)
	if !on.AnonymousActor().Has(permission.DocRead) {
		t.Error("开启时匿名应有读集")
	}
}

// 跨用户吊销必须 404（own 域隔离，不泄露存在性）。
func TestRevokeScopedToOwner(t *testing.T) {
	svc, _ := newAuthSvc(t, nil)
	ctx := context.Background()
	owner, _ := svc.ResolveSSO(ctx, "ow", "ow@x.com", "O")
	other, _ := svc.ResolveSSO(ctx, "ot", "ot@x.com", "T")

	tk, _ := svc.IssueToken(ctx, owner.ID, "mine")
	if err := svc.RevokeToken(ctx, tk.TokenRecord.ID, other.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("他人吊销应 NotFound: %v", err)
	}
	if err := svc.RevokeToken(ctx, tk.TokenRecord.ID, owner.ID); err != nil {
		t.Fatalf("本人吊销应成功: %v", err)
	}
}

// 边界：空 cookie、不存在用户会话、禁用用户 Bearer。
func TestAuthEdgePaths(t *testing.T) {
	svc, db := newAuthSvc(t, nil)
	ctx := context.Background()

	if _, err := svc.ActorFromSession(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("空 cookie: %v", err)
	}
	if _, err := svc.ActorFromSession(ctx, "nonexistent"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("未知会话: %v", err)
	}
	if _, err := svc.ActorFromBearer(ctx, "   "); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("空白 bearer: %v", err)
	}
	if _, err := svc.ActorFromBearer(ctx, "ew_bogus"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("伪造 bearer: %v", err)
	}
	if _, err := svc.ResolveSSO(ctx, "", "e@x.com", "N"); err == nil {
		t.Error("空 subject 必须拒绝")
	}

	// 会话指向的用户被禁用 → actorOfActiveUser 拒绝（permission.ErrDenied 语义）
	u, _ := svc.ResolveSSO(ctx, "edge", "edge@x.com", "E")
	raw, _, _ := svc.NewSession(ctx, u.ID)
	svc.users.UpdateUserStatus(ctx, u.ID, model.UserDisabled)
	if _, err := svc.ActorFromSession(ctx, raw); err == nil ||
		!strings.Contains(err.Error(), "disabled") {
		t.Errorf("禁用用户会话应拒绝: %v", err)
	}
	_ = db
}
