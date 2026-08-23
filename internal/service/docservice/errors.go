package docservice

import (
	"errors"
	"fmt"

	"element-wiki/internal/store"
)

// 领域错误：httpapi 层负责映射为 HTTP 状态码。
var (
	// ErrSelfChild 不允许把文档移动进自身子树（映射 422）。
	ErrSelfChild = errors.New("docservice: 不能移动进自身子树")
	// ErrValidation 入参非法；用 %w 包裹 *ValidationError 获取字段明细。
	ErrValidation = errors.New("docservice: 校验失败")
)

// ValidationError 携带字段级明细（API 契约的 fields 结构）。
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func invalid(field, reason string) error {
	return fmt.Errorf("%w: %w", ErrValidation, &ValidationError{Field: field, Reason: reason})
}

// VersionConflictError 并发保存冲突（映射 409，携带当前 HEAD）。
type VersionConflictError struct {
	HeadCommitID string
}

func (e *VersionConflictError) Error() string {
	return fmt.Sprintf("docservice: 版本冲突, 当前 HEAD=%s", e.HeadCommitID)
}

// 便于上层 errors.As 的哨兵包装。
var errAsConflict = &VersionConflictError{}

// AsVersionConflict 提取冲突详情。
func AsVersionConflict(err error) (*VersionConflictError, bool) {
	var vc *VersionConflictError
	if errors.As(err, &vc) {
		return vc, true
	}
	return nil, false
}

// IsNotFound 统一透传 store.ErrNotFound 判断。
func IsNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
