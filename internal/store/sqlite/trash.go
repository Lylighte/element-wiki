package sqlite

import (
	"context"

	"element-wiki/internal/model"
)

// SoftDeleteSubtree 标记整个存活子树进入回收站。
func (s *DB) SoftDeleteSubtree(ctx context.Context, rootID, by string, deletedAt, purgeAt int64) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE documents SET deleted_at=?, deleted_by=?, purge_at=?
WHERE id IN (
    WITH RECURSIVE sub(id) AS (
        SELECT id FROM documents WHERE id=? AND deleted_at IS NULL
        UNION ALL
        SELECT d.id FROM documents d JOIN sub ON d.parent_id=sub.id
         WHERE d.deleted_at IS NULL
    )
    SELECT id FROM sub
)`, deletedAt, by, purgeAt, rootID); err != nil {
		return mapErr(err)
	}
	return nil
}

const trashCols = `id, parent_id, space_id, slug, title, sort_key, visibility,
head_commit_id, created_by, updated_by, created_at, updated_at,
deleted_at, deleted_by, purge_at`

func (s *DB) ListTrash(ctx context.Context, limit int) ([]*model.Document, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+trashCols+` FROM documents
		 WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.Document{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RestoreSubtree 清除子树的删除标记。
func (s *DB) RestoreSubtree(ctx context.Context, rootID, by string, at int64) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE documents SET deleted_at=NULL, deleted_by=NULL, purge_at=NULL, updated_by=?, updated_at=?
WHERE id IN (
    WITH RECURSIVE sub(id) AS (
        SELECT id FROM documents WHERE id=? AND deleted_at IS NOT NULL
        UNION ALL
        SELECT d.id FROM documents d JOIN sub ON d.parent_id=sub.id
         WHERE d.deleted_at IS NOT NULL
    )
    SELECT id FROM sub
)`, by, at, rootID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return mapErr(errNoRowsWrap())
	}
	return nil
}

// HasDeletedAncestor 沿父链向上检查是否穿过回收站（不含自身）。
func (s *DB) HasDeletedAncestor(ctx context.Context, id string) (bool, error) {
	var cnt int
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE anc(id, parent_id, deleted_at) AS (
    SELECT id, parent_id, deleted_at FROM documents WHERE id=?
    UNION ALL
    SELECT d.id, d.parent_id, d.deleted_at
      FROM documents d JOIN anc a ON d.id=a.parent_id
)
SELECT COUNT(*) FROM anc WHERE id <> ? AND deleted_at IS NOT NULL`, id, id).Scan(&cnt)
	if err != nil {
		return false, mapErr(err)
	}
	return cnt > 0, nil
}

// PurgeSubtree 物理删除子树行；commits/drafts/comments/attachments 级联。
func (s *DB) PurgeSubtree(ctx context.Context, rootID string) error {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM documents WHERE id IN (
    WITH RECURSIVE sub(id) AS (
        SELECT id FROM documents WHERE id=? AND deleted_at IS NOT NULL
        UNION ALL
        SELECT d.id FROM documents d JOIN sub ON d.parent_id=sub.id
         WHERE d.deleted_at IS NOT NULL
    )
    SELECT id FROM sub
)`, rootID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return mapErr(errNoRowsWrap())
	}
	return nil
}

// DuePurgeIDs 到期待彻底清除的根级条目。
func (s *DB) DuePurgeIDs(ctx context.Context, now int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id FROM documents WHERE deleted_at IS NOT NULL AND purge_at <= ?
  AND NOT EXISTS (
      SELECT 1 FROM documents p WHERE p.id = documents.parent_id
        AND p.deleted_at IS NOT NULL
  )`, now)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GCDereferencedBlobs 清理孤儿 blob（T5.3）。
func (s *DB) GCDereferencedBlobs(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM document_blobs
WHERE hash NOT IN (SELECT DISTINCT blob_hash FROM document_commits)`)
	if err != nil {
		return 0, mapErr(err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *DB) SubtreeIDsOfTrashed(ctx context.Context, rootID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
WITH RECURSIVE sub(id) AS (
    SELECT id FROM documents WHERE id=? AND deleted_at IS NOT NULL
    UNION ALL
    SELECT d.id FROM documents d JOIN sub ON d.parent_id=sub.id
     WHERE d.deleted_at IS NOT NULL
)
SELECT id FROM sub`, rootID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	if len(out) == 0 {
		return nil, mapErr(errNoRowsWrap())
	}
	return out, nil
}

var (
	_ interface {
		ListChildren(context.Context, *string) ([]*model.Document, error)
	} = (*DB)(nil)
	_ TrashMarker = (*DB)(nil)
)

type TrashMarker = interface{} // 占位保持结构清晰
