package sqlite

import (
	"context"
	"errors"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/util"
)

func (s *DB) CreateComment(ctx context.Context, c *model.Comment, mentions []string) error {
	status := c.Status
	if status == "" {
		// Preserve the store contract for direct callers and legacy integrations.
		status = "published"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO comments (id, document_id, author_id, content, created_at, status) VALUES (?,?,?,?,?,?)`,
		c.ID, c.DocumentID, c.AuthorID, c.Content, c.CreatedAt, status); err != nil {
		return mapErr(err)
	}
	for _, uid := range mentions {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO comment_mentions (comment_id, user_id) VALUES (?,?)`,
			c.ID, uid); err != nil {
			return mapErr(err)
		}
	}
	return tx.Commit()
}

const commentCols = `id, document_id, author_id, content, created_at, status, review_reason`

func scanComment(row interface{ Scan(...any) error }) (*model.Comment, error) {
	var c model.Comment
	err := row.Scan(&c.ID, &c.DocumentID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.Status, &c.ReviewReason)
	if err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (s *DB) ListComments(ctx context.Context, docID string, limit int) ([]*model.Comment, error) {
	if limit < 1 {
		return nil, errors.New("sqlite: limit 必须 >= 1")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+commentCols+` FROM comments WHERE document_id=? AND status='published' ORDER BY created_at, id LIMIT ?`,
		docID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *DB) ListPendingComments(ctx context.Context, limit int) ([]*model.Comment, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+commentCols+` FROM comments WHERE status='pending' ORDER BY created_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *DB) ReviewComment(ctx context.Context, id, reviewerID, action, reason string, at int64) error {
	if action != "approve" && action != "reject" {
		return store.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	var authorID string
	if err := tx.QueryRowContext(ctx, `SELECT author_id FROM comments WHERE id=? AND status='pending'`, id).Scan(&authorID); err != nil {
		return mapErr(err)
	}
	if authorID == reviewerID {
		return store.ErrInvalid
	}
	status := map[bool]string{true: "published", false: "rejected"}[action == "approve"]
	result, err := tx.ExecContext(ctx, `UPDATE comments SET status=?,reviewed_by=?,reviewed_at=?,review_reason=? WHERE id=? AND status='pending'`, status, reviewerID, at, reason, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return store.ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO moderation_actions(id,content_type,content_id,revision_id,action,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, util.NewID(), "comment", id, "", action, reviewerID, reason, at); err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

func (s *DB) GetComment(ctx context.Context, id string) (*model.Comment, error) {
	c, err := scanComment(s.db.QueryRowContext(ctx,
		`SELECT `+commentCols+` FROM comments WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	mids, _ := s.MentionIDsOf(ctx, id)
	c.Mentions = mids
	return c, nil
}

func (s *DB) DeleteComment(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM content_moderation_state WHERE content_type='comment' AND content_id=?`, id); err != nil {
		return mapErr(err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE id=?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return mapErr(errNoRowsWrap())
	}
	return mapErr(tx.Commit())
}

func (s *DB) MentionIDsOf(ctx context.Context, commentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM comment_mentions WHERE comment_id=?`, commentID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}
