package model

import "element-wiki/internal/permission"

// User 是 SSO JIT 建号的账号（无密码体系，AGENTS §5）。
type User struct {
	ID          string          `json:"id"`
	Issuer      string          `json:"-"`
	Subject     string          `json:"-"`
	Email       string          `json:"email"`
	DisplayName string          `json:"display_name"`
	Role        permission.Role `json:"role"`
	Status      string          `json:"status"` // active | disabled
	CreatedAt   int64           `json:"created_at"`
	LastLoginAt int64           `json:"last_login_at"`
}

const (
	UserActive   = "active"
	UserDisabled = "disabled"
)

// APIToken 个人访问令牌；库中仅存哈希与前缀。
type APIToken struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	TokenHash  string `json:"-"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`
	RevokedAt  *int64 `json:"revoked_at"`
}
