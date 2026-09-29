package sqlite

import (
	"context"
	"database/sql"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/util"
)

func scanPageRevision(scanner interface{ Scan(...any) error }) (*model.UserPageRevision, error) {
	r := &model.UserPageRevision{}
	var id, content, status, createdBy sql.NullString
	var createdAt sql.NullInt64
	var reviewedBy sql.NullString
	var reviewedAt sql.NullInt64
	var reason sql.NullString
	err := scanner.Scan(&id, &content, &status, &createdBy, &createdAt, &reviewedBy, &reviewedAt, &reason)
	if err != nil || !id.Valid {
		return nil, err
	}
	r.ID, r.Content, r.Status, r.CreatedBy, r.CreatedAt = id.String, content.String, status.String, createdBy.String, createdAt.Int64
	if reviewedBy.Valid {
		r.ReviewedBy = reviewedBy.String
	}
	if reviewedAt.Valid {
		r.ReviewedAt = &reviewedAt.Int64
	}
	r.Reason = reason.String
	return r, nil
}

func (s *DB) GetUserPage(ctx context.Context, userID string) (*model.UserPage, error) {
	var p model.UserPage
	err := s.db.QueryRowContext(ctx, `SELECT p.user_id, u.display_name, p.updated_at FROM user_pages p JOIN users u ON u.id=p.user_id WHERE p.user_id=?`, userID).
		Scan(&p.UserID, &p.DisplayName, &p.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	var publishedID, pendingID string
	if err := s.db.QueryRowContext(ctx, `SELECT published_revision_id,pending_revision_id FROM user_pages WHERE user_id=?`, userID).Scan(&publishedID, &pendingID); err != nil {
		return nil, mapErr(err)
	}
	const cols = `id, content, status, created_by, created_at, reviewed_by, reviewed_at, reason`
	if publishedID != "" {
		r, err := scanPageRevision(s.db.QueryRowContext(ctx, `SELECT `+cols+` FROM user_page_revisions WHERE id=? AND user_id=?`, publishedID, userID))
		if err != nil {
			return nil, mapErr(err)
		}
		p.Published = r
	}
	if pendingID != "" {
		r, err := scanPageRevision(s.db.QueryRowContext(ctx, `SELECT `+cols+` FROM user_page_revisions WHERE id=? AND user_id=?`, pendingID, userID))
		if err != nil {
			return nil, mapErr(err)
		}
		p.Pending = r
	}
	return &p, nil
}

func (s *DB) SubmitUserPageRevision(ctx context.Context, r *model.UserPageRevision, requireReview bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_pages(user_id,updated_at) VALUES(?,?) ON CONFLICT(user_id) DO NOTHING`, r.UserID, r.CreatedAt); err != nil {
		return mapErr(err)
	}
	var previousPending string
	if err = tx.QueryRowContext(ctx, `SELECT pending_revision_id FROM user_pages WHERE user_id=?`, r.UserID).Scan(&previousPending); err != nil {
		return mapErr(err)
	}
	if previousPending != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE user_page_revisions SET status='rejected', reviewed_by=?, reviewed_at=?, reason='superseded by a newer submission' WHERE id=? AND status='pending'`, r.CreatedBy, r.CreatedAt, previousPending); err != nil {
			return mapErr(err)
		}
	}
	status := "published"
	if requireReview {
		status = "pending"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_page_revisions(id,user_id,content,status,created_by,created_at) VALUES(?,?,?,?,?,?)`, r.ID, r.UserID, r.Content, status, r.CreatedBy, r.CreatedAt); err != nil {
		return mapErr(err)
	}
	if requireReview {
		_, err = tx.ExecContext(ctx, `UPDATE user_pages SET pending_revision_id=?, updated_at=? WHERE user_id=?`, r.ID, r.CreatedAt, r.UserID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE user_pages SET published_revision_id=?, pending_revision_id='', updated_at=? WHERE user_id=?`, r.ID, r.CreatedAt, r.UserID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE content_moderation_state SET previous_state=? WHERE content_type='user_page' AND content_id=?`, r.ID, r.UserID)
		}
	}
	if err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

func (s *DB) ReviewUserPageRevision(ctx context.Context, userID, revisionID, reviewerID, action, reason string, at int64) error {
	if action != "approve" && action != "reject" {
		return store.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	var createdBy string
	err = tx.QueryRowContext(ctx, `SELECT created_by FROM user_page_revisions WHERE id=? AND user_id=? AND status='pending'`, revisionID, userID).Scan(&createdBy)
	if err != nil {
		return mapErr(err)
	}
	if createdBy == reviewerID {
		return store.ErrInvalid
	}
	result, err := tx.ExecContext(ctx, `UPDATE user_page_revisions SET status=?,reviewed_by=?,reviewed_at=?,reason=? WHERE id=? AND user_id=? AND status='pending'`, map[bool]string{true: "published", false: "rejected"}[action == "approve"], reviewerID, at, reason, revisionID, userID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return store.ErrNotFound
	}
	if action == "approve" {
		_, err = tx.ExecContext(ctx, `UPDATE user_pages SET published_revision_id=?,pending_revision_id='',updated_at=? WHERE user_id=? AND pending_revision_id=?`, revisionID, at, userID, revisionID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE content_moderation_state SET previous_state=? WHERE content_type='user_page' AND content_id=?`, revisionID, userID)
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE user_pages SET pending_revision_id='',updated_at=? WHERE user_id=? AND pending_revision_id=?`, at, userID, revisionID)
	}
	if err != nil {
		return mapErr(err)
	}
	actionName := action
	_, err = tx.ExecContext(ctx, `INSERT INTO moderation_actions(id,content_type,content_id,revision_id,action,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, util.NewID(), "user_page", userID, revisionID, actionName, reviewerID, reason, at)
	if err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

func (s *DB) DeleteUserPage(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM content_moderation_state WHERE content_type='user_page' AND content_id=?`, userID); err != nil {
		return mapErr(err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM user_page_revisions WHERE user_id=?`, userID); err != nil {
		return mapErr(err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM user_pages WHERE user_id=?`, userID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return mapErr(tx.Commit())
}

func (s *DB) ListPendingUserPageRevisions(ctx context.Context, limit int) ([]*model.UserPageRevision, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,content,status,created_by,created_at,reviewed_by,reviewed_at,reason FROM user_page_revisions WHERE status='pending' ORDER BY created_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.UserPageRevision, 0)
	for rows.Next() {
		r := &model.UserPageRevision{}
		var reviewedBy sql.NullString
		var reviewedAt sql.NullInt64
		if err := rows.Scan(&r.ID, &r.UserID, &r.Content, &r.Status, &r.CreatedBy, &r.CreatedAt, &reviewedBy, &reviewedAt, &r.Reason); err != nil {
			return nil, mapErr(err)
		}
		if reviewedBy.Valid {
			r.ReviewedBy = reviewedBy.String
		}
		if reviewedAt.Valid {
			r.ReviewedAt = &reviewedAt.Int64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *DB) ListUserPageRevisions(ctx context.Context, userID string, limit int) ([]*model.UserPageRevision, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,content,status,created_by,created_at,reviewed_by,reviewed_at,reason FROM user_page_revisions WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.UserPageRevision, 0)
	for rows.Next() {
		r := &model.UserPageRevision{}
		var reviewer sql.NullString
		var reviewedAt sql.NullInt64
		if err := rows.Scan(&r.ID, &r.UserID, &r.Content, &r.Status, &r.CreatedBy, &r.CreatedAt, &reviewer, &reviewedAt, &r.Reason); err != nil {
			return nil, mapErr(err)
		}
		if reviewer.Valid {
			r.ReviewedBy = reviewer.String
		}
		if reviewedAt.Valid {
			r.ReviewedAt = &reviewedAt.Int64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var _ store.UserPageStore = (*DB)(nil)
