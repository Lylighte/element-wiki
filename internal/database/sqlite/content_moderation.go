package sqlite

import (
	"context"
	"database/sql"
	"errors"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/util"
)

func (s *DB) SetContentPublished(ctx context.Context, contentType, contentID string, published bool, actorID, reason string, at int64) error {
	if contentType != "document" && contentType != "comment" && contentType != "user_page" {
		return store.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	var current string
	switch contentType {
	case "document":
		err = tx.QueryRowContext(ctx, `SELECT visibility FROM documents WHERE id=? AND deleted_at IS NULL`, contentID).Scan(&current)
	case "comment":
		err = tx.QueryRowContext(ctx, `SELECT status FROM comments WHERE id=?`, contentID).Scan(&current)
	case "user_page":
		err = tx.QueryRowContext(ctx, `SELECT published_revision_id FROM user_pages WHERE user_id=?`, contentID).Scan(&current)
	}
	if err != nil {
		return mapErr(err)
	}
	var previous string
	stateErr := tx.QueryRowContext(ctx, `SELECT previous_state FROM content_moderation_state WHERE content_type=? AND content_id=?`, contentType, contentID).Scan(&previous)
	if published {
		if stateErr != nil {
			return mapErr(stateErr)
		}
		if previous == "" {
			return store.ErrNotFound
		}
		switch contentType {
		case "document":
			_, err = tx.ExecContext(ctx, `UPDATE documents SET visibility=?,updated_by=?,updated_at=? WHERE id=?`, previous, actorID, at, contentID)
		case "comment":
			_, err = tx.ExecContext(ctx, `UPDATE comments SET status=? WHERE id=?`, previous, contentID)
		case "user_page":
			_, err = tx.ExecContext(ctx, `UPDATE user_pages SET published_revision_id=?,updated_at=? WHERE user_id=?`, previous, at, contentID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM content_moderation_state WHERE content_type=? AND content_id=?`, contentType, contentID)
		}
	} else {
		if stateErr == nil {
			return store.ErrConflict
		}
		if !errors.Is(stateErr, sql.ErrNoRows) {
			return mapErr(stateErr)
		}
		if current == "" || current == "restricted" || current == "rejected" {
			return store.ErrConflict
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO content_moderation_state(content_type,content_id,previous_state,hidden_by,hidden_at) VALUES(?,?,?,?,?)`, contentType, contentID, current, actorID, at); err != nil {
			return mapErr(err)
		}
		switch contentType {
		case "document":
			_, err = tx.ExecContext(ctx, `UPDATE documents SET visibility='restricted',updated_by=?,updated_at=? WHERE id=?`, actorID, at, contentID)
		case "comment":
			_, err = tx.ExecContext(ctx, `UPDATE comments SET status='rejected',reviewed_by=?,reviewed_at=?,review_reason='unpublished by moderator' WHERE id=?`, actorID, at, contentID)
		case "user_page":
			_, err = tx.ExecContext(ctx, `UPDATE user_pages SET published_revision_id='',updated_at=? WHERE user_id=?`, at, contentID)
		}
	}
	if err != nil {
		return mapErr(err)
	}
	action := "restore"
	if !published {
		action = "unpublish"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO moderation_actions(id,content_type,content_id,revision_id,action,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, util.NewID(), contentType, contentID, "", action, actorID, reason, at); err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

func (s *DB) ListHiddenContent(ctx context.Context, limit int) ([]*model.HiddenContent, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT content_type,content_id,hidden_by,hidden_at FROM content_moderation_state ORDER BY hidden_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.HiddenContent, 0)
	for rows.Next() {
		var v model.HiddenContent
		if err := rows.Scan(&v.ContentType, &v.ContentID, &v.HiddenBy, &v.HiddenAt); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, &v)
	}
	return out, rows.Err()
}

var _ store.ContentModerationStore = (*DB)(nil)
