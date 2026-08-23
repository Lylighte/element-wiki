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
	ID           string     `json:"id"`
	ParentID     *string    `json:"parent_id"`          // NULL = 根
	SpaceID      *string    `json:"space_id,omitempty"` // v1 恒为 NULL（预留列）
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	SortKey      int64      `json:"sort_key"`
	Visibility   Visibility `json:"visibility"`
	HeadCommitID string     `json:"head_commit_id"` // '' 表示尚无 commit
	CreatedBy    string     `json:"-"`
	UpdatedBy    string     `json:"-"`
	CreatedAt    int64      `json:"created_at"`
	UpdatedAt    int64      `json:"updated_at"`
	DeletedAt    *int64     `json:"deleted_at,omitempty"` // NULL = 存活
	DeletedBy    *string    `json:"-"`
	PurgeAt      *int64     `json:"purge_at,omitempty"`
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

// SearchJob 是索引重建任务行。
type SearchJob struct {
	ID         string  `json:"job_id"`
	DocumentID *string `json:"document_id"` // NULL = 全量
	Reason     string  `json:"reason"`
	Status     string  `json:"status"` // pending|running|done|failed
	Attempts   int64   `json:"attempts"`
	LastErr    string  `json:"last_error,omitempty"`
	CreatedAt  int64   `json:"created_at"`
	FinishedAt int64   `json:"finished_at,omitempty"`
}

const (
	JobPending = "pending"
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)
