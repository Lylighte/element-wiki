package sqlite

import (
	"context"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

// UpsertDraft 幂等覆盖（ON CONFLICT 语法双方言通用）。
func (s *DB) UpsertDraft(ctx context.Context, d *model.Draft) error {
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO document_drafts (document_id, user_id, base_commit_id, content, updated_at)
VALUES (?,?,?,?,?)
ON CONFLICT(document_id, user_id) DO UPDATE SET
	base_commit_id = excluded.base_commit_id,
	content = excluded.content,
	updated_at = excluded.updated_at`,
		d.DocumentID, d.UserID, d.BaseCommitID, d.Content, d.UpdatedAt); err != nil {
		return mapErr(err)
	}
	return nil
}

func (s *DB) GetDraft(ctx context.Context, docID, userID string) (*model.Draft, error) {
	var d model.Draft
	err := s.db.QueryRowContext(ctx, `
SELECT document_id, user_id, base_commit_id, content, updated_at
FROM document_drafts WHERE document_id = ? AND user_id = ?`,
		docID, userID).Scan(&d.DocumentID, &d.UserID, &d.BaseCommitID, &d.Content, &d.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &d, nil
}

func (s *DB) DeleteDraft(ctx context.Context, docID, userID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM document_drafts WHERE document_id = ? AND user_id = ?`, docID, userID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

var _ store.DraftStore = (*DB)(nil)
var _ store.AppendCommitter = (*DB)(nil)
var _ store.CommitStore = (*DB)(nil)
