// 08 计划阶段 1：备份导入的固定表/列清单（与 doc/01 DDL 一一对应）。
// 上传 zip 内 db.sqlite3 的实际 schema 一律不信任：INSERT/SELECT 标识符
// 只取自本文件常量；rows.Columns() 仅用于导入前的一致性校验。
package backupservice

// tableColumns 是一张表的固定列清单（顺序与 doc/01 DDL 一致）。
type tableColumns struct {
	name string
	cols []string
}

var (
	tblSettings = tableColumns{"settings", []string{
		"key", "value", "updated_at", "updated_by"}}
	tblUsers = tableColumns{"users", []string{
		"id", "issuer", "subject", "email", "display_name", "role",
		"status", "created_at", "last_login_at"}}
	tblDocumentBlobs = tableColumns{"document_blobs", []string{
		"hash", "content", "size", "created_at"}}
	tblDocuments = tableColumns{"documents", []string{
		"id", "parent_id", "space_id", "slug", "title", "sort_key",
		"visibility", "head_commit_id", "created_by", "updated_by",
		"created_at", "updated_at", "deleted_at", "deleted_by", "purge_at"}}
	tblDocumentCommits = tableColumns{"document_commits", []string{
		"id", "document_id", "commit_no", "parent_commit_id", "blob_hash",
		"author_id", "message", "created_at"}}
	tblDocumentDrafts = tableColumns{"document_drafts", []string{
		"document_id", "user_id", "base_commit_id", "content", "updated_at"}}
	tblComments = tableColumns{"comments", []string{
		"id", "document_id", "author_id", "content", "created_at"}}
	tblCommentMentions = tableColumns{"comment_mentions", []string{
		"comment_id", "user_id"}}
	tblAttachments = tableColumns{"attachments", []string{
		"id", "document_id", "filename", "storage_path", "mime_type",
		"size", "sha256", "uploaded_by", "created_at"}}

	// dataTables 是整表替换的数据表（拷贝顺序即外键安全顺序）。
	dataTables = []tableColumns{
		tblSettings, tblUsers, tblDocumentBlobs, tblDocuments,
		tblDocumentCommits, tblDocumentDrafts, tblComments,
		tblCommentMentions, tblAttachments,
	}

	// operationalTables 是导入时清空（不拷贝）的操作型表。
	// selfPreserved 中的表导入时保留自身 job 行（id <> ?）。
	operationalTables = []tableColumns{
		{"sessions", []string{"token_hash", "user_id", "expires_at", "created_at"}},
		{"api_tokens", []string{"id", "user_id", "name", "prefix", "token_hash", "created_at", "last_used_at", "revoked_at"}},
		{"search_reindex_jobs", []string{"id", "document_id", "reason", "status", "attempts", "last_error", "created_at", "finished_at"}},
		{"backup_jobs", []string{"id", "kind", "filename", "status", "requested_by", "last_error", "created_at", "started_at", "finished_at"}},
		{"import_jobs", []string{"id", "status", "total_files", "imported_files", "failed_files", "requested_by", "last_error", "created_at", "started_at", "finished_at"}},
	}
)
