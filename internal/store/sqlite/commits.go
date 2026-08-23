package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func (s *DB) PutBlob(ctx context.Context, hash, content string) error {
	// INSERT OR IGNORE 为 sqlite 方言特性（AGENTS §3：方言差异在实现内隔离）。
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO document_blobs (hash, content, size, created_at) VALUES (?,?,?,?)`,
		hash, content, len(content), nowMs()); err != nil {
		return mapErr(err)
	}
	return nil
}

func (s *DB) GetBlob(ctx context.Context, hash string) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM document_blobs WHERE hash = ?`, hash).Scan(&content)
	if err != nil {
		return "", mapErr(err)
	}
	return content, nil
}

const commitCols = `id, document_id, commit_no, parent_commit_id, blob_hash, author_id, message, created_at`

func scanCommit(row interface{ Scan(...any) error }) (*model.Commit, error) {
	var c model.Commit
	err := row.Scan(&c.ID, &c.DocumentID, &c.CommitNo, &c.ParentCommitID,
		&c.BlobHash, &c.AuthorID, &c.Message, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *DB) GetCommit(ctx context.Context, docID, commitID string) (*model.Commit, error) {
	c, err := scanCommit(s.db.QueryRowContext(ctx,
		`SELECT `+commitCols+` FROM document_commits WHERE document_id = ? AND id = ?`,
		docID, commitID))
	return c, mapErr(err)
}

func (s *DB) ListCommits(ctx context.Context, docID string, limit int) ([]*model.Commit, error) {
	if limit < 1 {
		return nil, fmt.Errorf("sqlite: limit 必须 >= 1, got %d", limit)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+commitCols+` FROM document_commits WHERE document_id = ?
		 ORDER BY commit_no DESC LIMIT ?`, docID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.Commit{}
	for rows.Next() {
		c, err := scanCommit(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *DB) CountCommits(ctx context.Context, docID string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM document_commits WHERE document_id = ?`, docID).Scan(&n)
	return n, mapErr(err)
}

func (s *DB) MaxCommitNo(ctx context.Context, docID string) (int64, error) {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(commit_no) FROM document_commits WHERE document_id = ?`, docID).Scan(&v)
	if err != nil {
		return 0, mapErr(err)
	}
	return v.Int64, nil
}

// AppendCommit 原子完成：插入版本 → 推进 HEAD → 按上限裁剪最旧版本。
func (s *DB) AppendCommit(ctx context.Context, c *model.Commit, maxVersions int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: 开启提交事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO document_commits (id, document_id, commit_no, parent_commit_id, blob_hash, author_id, message, created_at)
VALUES (?,?,?,?,?,?,?,?)`,
		c.ID, c.DocumentID, c.CommitNo, c.ParentCommitID, c.BlobHash, c.AuthorID, c.Message, c.CreatedAt); err != nil {
		return 0, mapErr(err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE documents SET head_commit_id = ?, updated_at = ? WHERE id = ?`,
		c.ID, c.CreatedAt, c.DocumentID)
	if err != nil {
		return 0, mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, store.ErrNotFound // 目标文档不存在
	}

	var trimmed int64
	if maxVersions >= 1 {
		res2, err := tx.ExecContext(ctx, `
DELETE FROM document_commits WHERE document_id = ? AND id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (ORDER BY commit_no DESC) AS rn
        FROM document_commits WHERE document_id = ?
    ) AS t WHERE rn > ?
)`, c.DocumentID, c.DocumentID, maxVersions)
		if err != nil {
			return 0, fmt.Errorf("sqlite: 版本裁剪失败: %w", err)
		}
		trimmed, _ = res2.RowsAffected()
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: 提交事务失败: %w", err)
	}
	return trimmed, nil
}
