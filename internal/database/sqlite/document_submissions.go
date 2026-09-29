package sqlite

import (
	"context"
	"database/sql"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/util"
)

func scanDocumentSubmission(row interface{ Scan(...any) error }) (*model.DocumentSubmission, error) {
	var s model.DocumentSubmission
	var title sql.NullString
	err := row.Scan(&s.ID, &s.DocumentID, &title, &s.BaseCommitID, &s.Content, &s.Message, &s.AuthorID, &s.CreatedAt, &s.Reason)
	if err != nil {
		return nil, mapErr(err)
	}
	if title.Valid {
		s.Title = &title.String
	}
	return &s, nil
}

const submissionCols = `id,document_id,title,base_commit_id,content,message,author_id,created_at,reason`

func (s *DB) CreateDocumentSubmission(ctx context.Context, sub *model.DocumentSubmission) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO document_submissions(id,document_id,title,base_commit_id,content,message,author_id,created_at,status) VALUES(?,?,?,?,?,?,?,?, 'pending')`, sub.ID, sub.DocumentID, sub.Title, sub.BaseCommitID, sub.Content, sub.Message, sub.AuthorID, sub.CreatedAt)
	return mapErr(err)
}

func (s *DB) GetDocumentSubmission(ctx context.Context, id string) (*model.DocumentSubmission, error) {
	return scanDocumentSubmission(s.db.QueryRowContext(ctx, `SELECT `+submissionCols+` FROM document_submissions WHERE id=? AND status='pending'`, id))
}

func (s *DB) ListPendingDocumentSubmissions(ctx context.Context, limit int) ([]*model.DocumentSubmission, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+submissionCols+` FROM document_submissions WHERE status='pending' ORDER BY created_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.DocumentSubmission, 0)
	for rows.Next() {
		sub, err := scanDocumentSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *DB) FinishDocumentSubmission(ctx context.Context, id, reviewerID, action, reason string, at int64) error {
	if action != "approve" && action != "reject" {
		return store.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	var authorID, documentID string
	if err := tx.QueryRowContext(ctx, `SELECT author_id,document_id FROM document_submissions WHERE id=? AND status='pending'`, id).Scan(&authorID, &documentID); err != nil {
		return mapErr(err)
	}
	if authorID == reviewerID {
		return store.ErrInvalid
	}
	status := map[bool]string{true: "approved", false: "rejected"}[action == "approve"]
	res, err := tx.ExecContext(ctx, `UPDATE document_submissions SET status=?,reviewed_by=?,reviewed_at=?,reason=? WHERE id=? AND status='pending'`, status, reviewerID, at, reason, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return store.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO moderation_actions(id,content_type,content_id,revision_id,action,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, util.NewID(), "document", documentID, id, action, reviewerID, reason, at); err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

var _ store.DocumentSubmissionStore = (*DB)(nil)
