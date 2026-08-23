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

// TreeStore 是文档树的结构查询契约（T1.2）。
type TreeStore interface {
	// SubtreeIDs 返回以 rootID 为根的存活子树（含自身）；回收站分支被剪除。
	SubtreeIDs(ctx context.Context, rootID string) ([]string, error)
	// EffectiveVisibility 沿祖先链解析生效可见性（PM-05 继承规则）。
	EffectiveVisibility(ctx context.Context, docID string) (model.Visibility, error)
}

// CommitStore 是不可变版本历史的持久化契约。
type CommitStore interface {
	// PutBlob 内容寻址写入，hash 已存在时幂等。
	PutBlob(ctx context.Context, hash, content string) error
	GetBlob(ctx context.Context, hash string) (string, error)
	GetCommit(ctx context.Context, docID, commitID string) (*model.Commit, error)
	// ListCommits 按 commit_no 降序返回；limit 必须 >= 1。
	ListCommits(ctx context.Context, docID string, limit int) ([]*model.Commit, error)
	CountCommits(ctx context.Context, docID string) (int64, error)
	// MaxCommitNo 返回该文档当前最大 commit_no；无版本时返回 0。
	MaxCommitNo(ctx context.Context, docID string) (int64, error)
}

// AppendCommitter 是带事务保证的提交入口：插入 commit、推进 HEAD、按上限裁剪，
// 三者原子完成（doc/01 §4.4 设计决策）。
type AppendCommitter interface {
	// AppendCommit 写入新版本并把 documents.head_commit_id 指向它。
	// maxVersions >= 1 时同事务裁剪超出上限的最旧版本并返回裁剪数；
	// maxVersions = 0 表示不限制。
	AppendCommit(ctx context.Context, c *model.Commit, maxVersions int64) (trimmed int64, err error)
}

// DraftStore 是按用户隔离的草稿契约。
type DraftStore interface {
	UpsertDraft(ctx context.Context, d *model.Draft) error
	GetDraft(ctx context.Context, docID, userID string) (*model.Draft, error)
	// DeleteDraft 删除不存在的草稿返回 ErrNotFound。
	DeleteDraft(ctx context.Context, docID, userID string) error
}

// CommitStore 追加：MaxCommitNo 支持裁剪后存在缺口场景（MAX+1 才是下一个序号）。
