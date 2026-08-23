// Package model 定义跨层共享的领域模型。
package model

// Visibility 文档可见性两档（PM-05）。
type Visibility string

const (
	VisibilityStandard   Visibility = "standard"
	VisibilityRestricted Visibility = "restricted"
)

func (v Visibility) Valid() bool {
	return v == VisibilityStandard || v == VisibilityRestricted
}

// Document 是文档树节点元数据；正文存于 document_blobs，经 commit 关联。
type Document struct {
	ID           string
	ParentID     *string // NULL = 根
	SpaceID      *string // v1 恒为 NULL（预留列）
	Slug         string
	Title        string
	SortKey      int64
	Visibility   Visibility
	HeadCommitID string // '' 表示尚无 commit；无外键，一致性由 service 事务保证
	CreatedBy    string
	UpdatedBy    string
	CreatedAt    int64
	UpdatedAt    int64
	DeletedAt    *int64 // NULL = 存活
	DeletedBy    *string
	PurgeAt      *int64
}

// Alive 报告文档是否未进回收站。
func (d *Document) Alive() bool { return d.DeletedAt == nil }

// DocumentMut 是 UpdateMeta 的可选字段集；nil 字段不修改。
type DocumentMut struct {
	Title      *string
	Slug       *string
	SortKey    *int64
	Visibility *Visibility
}

// Commit 是文档的一次不可变版本（线性历史，doc/01 §4.4）。
type Commit struct {
	ID             string  `json:"id"`
	DocumentID     string  `json:"document_id"`
	CommitNo       int64   `json:"commit_no"`
	ParentCommitID *string `json:"parent_commit_id"` // 首个 commit 为 NULL
	BlobHash       string  `json:"blob_hash"`
	AuthorID       string  `json:"author_id"`
	Message        string  `json:"message"`
	CreatedAt      int64   `json:"created_at"`
}

// Draft 是按用户隔离的未提交草稿。
type Draft struct {
	DocumentID   string `json:"document_id"`
	UserID       string `json:"user_id"`
	BaseCommitID string `json:"base_commit_id"`
	Content      string `json:"content"`
	UpdatedAt    int64  `json:"updated_at"`
}
