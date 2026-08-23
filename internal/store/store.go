// Package store 定义存储层接口；sqlite 与 postgres 实现分属子包。
package store

import (
	"context"
	"errors"

	"element-wiki/internal/model"
)

// ErrNotFound 资源不存在或对调用方不可见（上层一律映射 404）。
var ErrNotFound = errors.New("store: not found")

// ErrConflict 唯一约束冲突（如 slug 重复、版本冲突）。
var ErrConflict = errors.New("store: conflict")

// ErrInvalid 违反外键或 CHECK 约束（数据非法）。
var ErrInvalid = errors.New("store: invalid")

// DocumentStore 是文档树元数据的持久化契约。
type DocumentStore interface {
	Create(ctx context.Context, d *model.Document) error
	Get(ctx context.Context, id string) (*model.Document, error)
	// GetBySlug 按父级+slug 精确查找；includeDeleted 控制是否命中回收站文档。
	GetBySlug(ctx context.Context, parentID *string, slug string, includeDeleted bool) (*model.Document, error)
	// ListChildren 返回直接子节点，按 sort_key, slug, id 排序；仅存活文档。
	ListChildren(ctx context.Context, parentID *string) ([]*model.Document, error)
	// UpdateMeta 应用非 nil 字段修改并刷新 updated_by/updated_at。
	UpdateMeta(ctx context.Context, id string, mut model.DocumentMut, updatedBy string, updatedAt int64) error
	// Move 变更父级（nil = 移到根）。
	Move(ctx context.Context, id string, parentID *string, updatedBy string, updatedAt int64) error
}
