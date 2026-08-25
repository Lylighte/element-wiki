// Package adminservice 承载管理域业务：设置校验、用户治理、仪表盘（T6.1~T6.4）。
package adminservice

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

// ErrValidation 校验失败；errors.As 取 *ValidationError。
var ErrValidation = errors.New("adminservice: validation failed")

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

func invalid(field, reason string) error {
	return fmt.Errorf("%w: %w", ErrValidation, &ValidationError{Field: field, Reason: reason})
}

// KnownSettings 键 → 校验函数（nil = 任意非空字符串）。
func KnownSettings() map[string]func(string) error {
	return map[string]func(string) error{
		"wiki_title":           nonEmpty,
		"anonymous_read":       parseBoolSetting,
		"comments_enabled":     parseBoolSetting,
		"max_versions":         intMin(1),
		"upload_max_mb":        intMin(1),
		"allowed_extensions":   nonEmpty,
		"timezone":             validTimezone,
		"default_lang":         oneOf("zh-CN", "en"),
		"trash_retention_days": intMin(1),
	}
}

func nonEmpty(v string) error {
	if strings.TrimSpace(v) == "" {
		return errors.New("must not be empty")
	}
	return nil
}

func parseBoolSetting(v string) error {
	if _, err := strconv.ParseBool(v); err != nil {
		return errors.New("must be a boolean")
	}
	return nil
}

func intMin(min int64) func(string) error {
	return func(v string) error {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < min {
			return fmt.Errorf("must be an integer >= %d", min)
		}
		return nil
	}
}

func oneOf(allowed ...string) func(string) error {
	return func(v string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return fmt.Errorf("allowed values: %s", strings.Join(allowed, "/"))
	}
}

func validTimezone(v string) error {
	if _, err := time.LoadLocation(v); err != nil {
		return errors.New("not a valid IANA timezone")
	}
	return nil
}

// StatsStore 聚合查询面。
type StatsStore interface {
	DashboardStats(ctx context.Context) (*store.DashboardStatsView, error)
}

// Service 管理域服务。
type Service struct {
	settings store.SettingsStore
	users    store.UserStore
	stats    store.StatsStore
	nowFn    func() int64
}

func New(settings store.SettingsStore, users store.UserStore, stats store.StatsStore) *Service {
	return &Service{settings: settings, users: users, stats: stats, nowFn: time.Now().UnixMilli}
}

// AllSettings 返回全量键值（admin）。
func (s *Service) AllSettings(ctx context.Context, actor permission.Actor) (map[string]string, error) {
	if err := actor.Require(permission.SettingsManage); err != nil {
		return nil, err
	}
	return s.settings.GetAllSettings(ctx)
}

// PublicSiteValues 读取站点公开字段的在线值（无权限语义：仅暴露公开键）。
func (s *Service) PublicSiteValues(ctx context.Context) map[string]string {
	m, err := s.settings.GetAllSettings(ctx)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, k := range []string{"wiki_title", "default_lang", "anonymous_read", "comments_enabled"} {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

// UpdateSettings 部分更新：未知键或类型错误整体拒绝（零写入）。
func (s *Service) UpdateSettings(ctx context.Context, actor permission.Actor,
	patch map[string]string) error {
	if err := actor.Require(permission.SettingsManage); err != nil {
		return err
	}
	known := KnownSettings()
	for k, v := range patch {
		check, ok := known[k]
		if !ok {
			return invalid(k, "unknown setting key")
		}
		if check != nil {
			if rerr := check(v); rerr != nil {
				return invalid(k, rerr.Error())
			}
		}
	}
	return s.settings.SetSettings(ctx, patch, actor.UserID(), s.nowFn())
}

// ListUsers 管理列表（q 过滤）。
func (s *Service) ListUsers(ctx context.Context, actor permission.Actor,
	q string, limit int) ([]*model.User, error) {
	if err := actor.Require(permission.UserList); err != nil {
		return nil, err
	}
	return s.users.ListUsers(ctx, q, limit)
}

// UpdateUser 角色/状态调整；禁止操作自己（防自锁）。
func (s *Service) UpdateUser(ctx context.Context, actor permission.Actor,
	id string, role *permission.Role, status *string) (*model.User, error) {
	if err := actor.Require(permission.UserManage); err != nil {
		return nil, err
	}
	if id == actor.UserID() {
		return nil, invalid("user_id", "cannot operate on your own account")
	}
	target, err := s.users.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if role != nil {
		if !role.Valid() {
			return nil, invalid("role", "invalid value")
		}
		if err := s.users.UpdateUserRole(ctx, id, *role); err != nil {
			return nil, err
		}
		target.Role = *role
	}
	if status != nil {
		if *status != model.UserActive && *status != model.UserDisabled {
			return nil, invalid("status", "invalid value")
		}
		if err := s.users.UpdateUserStatus(ctx, id, *status); err != nil {
			return nil, err
		}
		target.Status = *status
	}
	return target, nil
}

// CommentsEnabled 实时读取评论开关（CO-00 门闩数据源）。
func (s *Service) CommentsEnabled(ctx context.Context) bool {
	v, err := s.settings.GetAllSettings(ctx)
	if err != nil {
		return false
	}
	b, perr := strconv.ParseBool(v["comments_enabled"])
	return perr == nil && b
}

// Dashboard 聚合统计。
func (s *Service) Dashboard(ctx context.Context, actor permission.Actor) (*store.DashboardStatsView, error) {
	if err := actor.Require(permission.DashboardRead); err != nil {
		return nil, err
	}
	return s.stats.DashboardStats(ctx)
}

// ErrParentGoneLike 占位：admin 域暂无 409 场景，保持 respond.go 分支完整。
var ErrParentGoneLike = errors.New("adminservice: parent gone")
