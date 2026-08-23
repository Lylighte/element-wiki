package model

// Comment 是文档评论（CO-01，线性列表）。
type Comment struct {
	ID         string   `json:"id"`
	DocumentID string   `json:"document_id"`
	AuthorID   string   `json:"author_id"`
	Content    string   `json:"content"`
	CreatedAt  int64    `json:"created_at"`
	Mentions   []string `json:"mentions,omitempty"` // 仅响应装配用
}

// Attachment 元数据；文件本体在存储目录。
type Attachment struct {
	ID          string `json:"id"`
	DocumentID  string `json:"document_id"`
	Filename    string `json:"filename"`
	StoragePath string `json:"-"`
	MimeType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	UploadedBy  string `json:"uploaded_by"`
	CreatedAt   int64  `json:"created_at"`
}
