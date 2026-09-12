// Package backup 承载备份/恢复专用的 SQLite 存储操作（08 计划阶段 2）。
// 方言约束：VACUUM INTO 与跨库 ATTACH 拷贝为 SQLite 专用能力；
// PostgreSQL 部署下备份导出/导入在 httpapi 层显式 501 降级（C6），不经此包。
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Store 持主库连接；仅 backupservice 编排层调用。
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

// tableColumns 是一张表的固定列清单（顺序与 doc/01 DDL 一致）。
type tableColumns struct {
	name string
	cols []string
}

// 表/列清单为代码内常量：上传 zip 内 db.sqlite3 的实际 schema 一律不信任，
// INSERT/SELECT 标识符只取自本文件常量；rows.Columns() 仅用于导入前一致性校验。
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
	operationalTables = []tableColumns{
		{"sessions", []string{"token_hash", "user_id", "expires_at", "created_at"}},
		{"api_tokens", []string{"id", "user_id", "name", "prefix", "token_hash", "created_at", "last_used_at", "revoked_at"}},
		{"search_reindex_jobs", []string{"id", "document_id", "reason", "status", "attempts", "last_error", "created_at", "finished_at"}},
		{"backup_jobs", []string{"id", "kind", "filename", "status", "requested_by", "last_error", "created_at", "started_at", "finished_at"}},
		{"import_jobs", []string{"id", "status", "total_files", "imported_files", "failed_files", "requested_by", "last_error", "created_at", "started_at", "finished_at"}},
	}
)

// ErrSchemaMismatch 上传库 schema 与固定清单不一致（整体失败语义）。
type ErrSchemaMismatch struct{ Msg string }

func (e *ErrSchemaMismatch) Error() string { return e.Msg }

// Snapshot 将主库一致性快照写入 dstPath（VACUUM INTO，SQLite 专用）。
func (s *Store) Snapshot(ctx context.Context, dstPath string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, dstPath)
	return err
}

// CountAliveDocuments 返回存活文档总数（导出 manifest 统计）。
func (s *Store) CountAliveDocuments(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// ReplaceAll 打开 srcPath 的 staged 库，在单事务内把主库整表替换为源内容。
// selfJobID 为本次导入 job 行 id，backup_jobs/import_jobs 保留该行。
// 上传库 schema 先经 verifySchema 校验，不一致即整体失败（零写入）。
func (s *Store) ReplaceAll(ctx context.Context, srcPath, selfJobID string) error {
	src, err := sql.Open("sqlite", srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := verifySchema(ctx, src); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先清操作型子表，避免残留行悬挂引用新用户集；job 两表保留自身行（参数化）。
	for _, tc := range operationalTables {
		if tc.name == "backup_jobs" || tc.name == "import_jobs" {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM `+tc.name+` WHERE id <> ?`, selfJobID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, `DELETE FROM `+tc.name); err != nil {
			return err
		}
	}
	for i := len(dataTables) - 1; i >= 0; i-- {
		tbl := dataTables[i].name
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+tbl); err != nil {
			return fmt.Errorf("clear table %s: %w", tbl, err)
		}
	}
	for _, tc := range dataTables {
		if err := copyTable(ctx, src, tx, tc); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// verifySchema 校验上传库：数据表必须存在且列集合与固定清单完全一致；
// 操作型表允许缺失（旧备份兼容），存在时同样校验。
func verifySchema(ctx context.Context, src *sql.DB) error {
	check := func(tc tableColumns, required bool) error {
		actual, err := actualColumns(ctx, src, tc.name)
		if err != nil {
			if required {
				return &ErrSchemaMismatch{Msg: fmt.Sprintf(
					"backup schema invalid: table %s: %v", tc.name, err)}
			}
			return nil // 可选表缺失：跳过
		}
		if !equalSets(actual, tc.cols) {
			return &ErrSchemaMismatch{Msg: fmt.Sprintf(
				"backup schema invalid: table %s schema mismatch: got %v, want %v",
				tc.name, actual, tc.cols)}
		}
		return nil
	}
	for _, tc := range dataTables {
		if err := check(tc, true); err != nil {
			return err
		}
	}
	for _, tc := range operationalTables {
		if err := check(tc, false); err != nil {
			return err
		}
	}
	return nil
}

// actualColumns 读取上传库中某表的实际列名（仅用于与固定清单比对）。
func actualColumns(ctx context.Context, src *sql.DB, table string) ([]string, error) {
	rows, err := src.QueryContext(ctx, `SELECT * FROM `+table+` LIMIT 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}

// equalSets 比较两个字符串集合是否完全一致（忽略顺序与重复）。
func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(b))
	for _, s := range b {
		set[s]++
	}
	for _, s := range a {
		if set[s] <= 0 {
			return false
		}
		set[s]--
	}
	return true
}

// copyTable 按固定列清单整表拷贝：SELECT/INSERT 标识符全部取自代码内常量，
// 值走 ? 占位参数化。调用前须已通过 verifySchema。
func copyTable(ctx context.Context, src *sql.DB, dst *sql.Tx, tc tableColumns) error {
	colList := strings.Join(tc.cols, ",")
	rows, err := src.QueryContext(ctx, `SELECT `+colList+` FROM `+tc.name)
	if err != nil {
		return err
	}

	// 先全量读入内存，关闭游标后再写入目标，避免跨库游标交错
	var batch [][]any
	vals := make([]any, len(tc.cols))
	ptrs := make([]any, len(tc.cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close()
			return err
		}
		cp := make([]any, len(tc.cols))
		copy(cp, vals)
		batch = append(batch, cp)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	placeholders := strings.Repeat("?,", len(tc.cols))[:2*len(tc.cols)-1]
	stmt, err := dst.PrepareContext(ctx,
		`INSERT INTO `+tc.name+` (`+colList+`) VALUES (`+placeholders+`)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range batch {
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return err
		}
	}
	return nil
}
