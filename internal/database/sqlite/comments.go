package sqlite

import (
	"context"
	"errors"

	"element-wiki/internal/model"
)

func (s *DB) CreateComment(ctx context.Context, c *model.Comment, mentions []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO comments (id, document_id, author_id, content, created_at) VALUES (?,?,?,?,?)`,
		c.ID, c.DocumentID, c.AuthorID, c.Content, c.CreatedAt); err != nil {
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

const commentCols = `id, document_id, author_id, content, created_at`

func scanComment(row interface{ Scan(...any) error }) (*model.Comment, error) {
	var c model.Comment
	err := row.Scan(&c.ID, &c.DocumentID, &c.AuthorID, &c.Content, &c.CreatedAt)
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
		`SELECT `+commentCols+` FROM comments WHERE document_id=? ORDER BY created_at, id LIMIT ?`,
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
	res, err := s.db.ExecContext(ctx, `DELETE FROM comments WHERE id=?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return mapErr(errNoRowsWrap())
	}
	return nil
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
