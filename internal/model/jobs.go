package model

// BackupJob 导出/导入备份的异步任务。
type BackupJob struct {
	ID          string `json:"job_id"`
	Kind        string `json:"kind"` // export|import
	Filename    string `json:"filename,omitempty"`
	Status      string `json:"status"`
	RequestedBy string `json:"requested_by"`
	LastErr     string `json:"last_error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	StartedAt   int64  `json:"started_at,omitempty"`
	FinishedAt  int64  `json:"finished_at,omitempty"`
}

// ImportJob Markdown zip 导入任务。
type ImportJob struct {
	ID            string `json:"job_id"`
	Status        string `json:"status"`
	TotalFiles    int64  `json:"total_files"`
	ImportedFiles int64  `json:"imported_files"`
	FailedFiles   int64  `json:"failed_files"`
	RequestedBy   string `json:"requested_by"`
	LastErr       string `json:"last_error,omitempty"`
	CreatedAt     int64  `json:"created_at"`
	StartedAt     int64  `json:"started_at,omitempty"`
	FinishedAt    int64  `json:"finished_at,omitempty"`
}
