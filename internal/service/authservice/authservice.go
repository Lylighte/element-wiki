// Package authservice 承载认证域业务规则：SSO JIT 建号、admin 引导、
// 会话签发与解析、Bearer 令牌认证（T3.2~T3.4）。
package authservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
	"element-wiki/internal/util"
)

// 领域错误。
var (
	ErrDisabled = errors.New("authservice: 账号已禁用")
)

const sessionDays = 7

// Service 认证域服务。
type Service struct {
	users    store.UserStore
	sessions store.SessionStore
	tokens   store.APITokenStore
	issuer   string   // 配置的 OIDC issuer（用于 JIT 锚定）
	admins   []string // oidc.admin_emails 小写集合
	anonRead bool     // 匿名只读开关（PM-06）
	nowFn    func() int64
}

func New(users store.UserStore, sessions store.SessionStore,
	tokens store.APITokenStore, issuer string, adminEmails []string, anonRead bool) *Service {
	low := make([]string, 0, len(adminEmails))
	for _, e := range adminEmails {
		low = append(low, strings.ToLower(strings.TrimSpace(e)))
	}
	return &Service{users: users, sessions: sessions, tokens: tokens,
		issuer: issuer, admins: low, anonRead: anonRead, nowFn: defaultNow}
}

func defaultNow() int64 { return util_Millis() }

// ResolveSSO 处理一次 SSO 身份到达：JIT 建号 + admin 引导（仅首次）+ 登录时间戳。
// 返回的用户保证 status=active；disabled 直接拒绝不复活（AGENTS §5）。
func (s *Service) ResolveSSO(ctx context.Context, subject, email, displayName string) (*model.User, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, errors.New("authservice: 空 subject")
	}
	existing, err := s.users.FindUserByIssuerSubject(ctx, s.issuer, subject)
	switch {
	case err == nil:
		if existing.Status == model.UserDisabled {
			return nil, ErrDisabled
		}
		if err := s.users.TouchLogin(ctx, existing.ID, s.nowFn()); err != nil {
			return nil, err
		}
		return existing, nil
	case !store.IsNotFound(err):
		return nil, err
	}

	// 新用户：JIT viewer；若全站无 admin 且邮箱命中引导名单 → admin（仅此一次窗口）
	role := permission.Viewer
	emailNorm := strings.ToLower(strings.TrimSpace(email))
	if emailNorm != "" && slices_Contains(s.admins, emailNorm) {
		if n, cerr := s.users.CountAdmins(ctx); cerr == nil && n == 0 {
			role = permission.Admin
		}
	}
	u := &model.User{
		ID: util.NewID(), Issuer: s.issuer, Subject: subject,
		Email: email, DisplayName: displayName,
		Role: role, Status: model.UserActive,
		CreatedAt: s.nowFn(), LastLoginAt: s.nowFn(),
	}
	if err := s.users.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// NewSession 为用户签发会话，返回 (cookie 明文值, 过期毫秒)。
func (s *Service) NewSession(ctx context.Context, userID string) (string, int64, error) {
	raw := randomToken(32)
	hash := HashToken(raw)
	exp := s.nowFn() + int64(sessionDays)*86400_000
	if err := s.sessions.CreateSession(ctx, hash, userID, exp); err != nil {
		return "", 0, err
	}
	return raw, exp, nil
}

// Logout 注销会话；不存在也视为成功（幂等）。
func (s *Service) Logout(ctx context.Context, cookieVal string) error {
	err := s.sessions.DeleteSession(ctx, HashToken(cookieVal))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}

var ErrUnauthenticated = errors.New("authservice: unauthenticated")

// ActorFromSession 由 cookie 值解析 Actor。
func (s *Service) ActorFromSession(ctx context.Context, cookieVal string) (permission.Actor, error) {
	if cookieVal == "" {
		return nil, ErrUnauthenticated
	}
	userID, exp, err := s.sessions.GetSession(ctx, HashToken(cookieVal))
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if exp <= s.nowFn() {
		_ = s.sessions.DeleteSession(ctx, HashToken(cookieVal))
		return nil, ErrUnauthenticated
	}
	return s.actorOfActiveUser(ctx, userID)
}

// ActorFromBearer 解析个人访问令牌并刷新 last_used_at。
func (s *Service) ActorFromBearer(ctx context.Context, bearer string) (permission.Actor, error) {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return nil, ErrUnauthenticated
	}
	tk, err := s.tokens.GetTokenByHash(ctx, HashToken(bearer))
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if tk.RevokedAt != nil {
		return nil, ErrUnauthenticated
	}
	_ = s.tokens.TouchToken(ctx, tk.ID, s.nowFn())
	return s.actorOfActiveUser(ctx, tk.UserID)
}

func (s *Service) actorOfActiveUser(ctx context.Context, userID string) (permission.Actor, error) {
	u, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if u.Status != model.UserActive {
		return nil, fmt.Errorf("%w: %s", permission.ErrDenied, "disabled account")
	}
	return permission.NewActor(u.ID, permission.CodesFor(u.Role)), nil
}

// AnonymousActor 返回按站点开关配置的匿名身份。
func (s *Service) AnonymousActor() permission.Actor {
	return permission.Anonymous(s.anonRead)
}

// AnonymousEnabled 报告站点是否允许匿名只读（PM-06）。
func (s *Service) AnonymousEnabled() bool { return s.anonRead }

// ---- Token 签发 ----

type IssuedToken struct {
	TokenRecord *model.APIToken
	Plaintext   string // 仅此一次可见
}

// IssueToken 生成明文令牌（ew_ 前缀），库中只落哈希与前缀。
func (s *Service) IssueToken(ctx context.Context, userID, name string) (*IssuedToken, error) {
	raw := "ew_" + randomToken(24)
	tk := &model.APIToken{
		ID: util.NewID(), UserID: userID, Name: name,
		Prefix: raw[:8], TokenHash: HashToken(raw), CreatedAt: s.nowFn(),
	}
	if err := s.tokens.CreateToken(ctx, tk); err != nil {
		return nil, err
	}
	return &IssuedToken{TokenRecord: tk, Plaintext: raw}, nil
}

// ListTokens / RevokeToken 直通 store，权限（own）由 handler 校验 userID。
func (s *Service) ListTokens(ctx context.Context, userID string) ([]*model.APIToken, error) {
	return s.tokens.ListTokensByUser(ctx, userID)
}

func (s *Service) RevokeToken(ctx context.Context, id, userID string) error {
	return s.tokens.RevokeToken(ctx, id, userID, s.nowFn())
}

// ---- 工具 ----

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken 统一 SHA-256 十六进制存储约定。
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func slices_Contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Me 返回当前用户资料（/v1/users/me）。
func (s *Service) Me(ctx context.Context, userID string) (*model.User, error) {
	return s.users.GetUser(ctx, userID)
}

// SetDisabledForTest 仅供测试：直接禁用账号。
func (s *Service) SetDisabledForTest(userID string) {
	_ = s.users.UpdateUserStatus(context.Background(), userID, model.UserDisabled)
}
