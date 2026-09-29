package sqlite

import (
	"context"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
	"element-wiki/internal/util"
)

func (s *DB) CreateContentReport(ctx context.Context, r *model.ContentReport) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO content_reports(id,reporter_id,content_type,content_id,reason,status,created_at) VALUES(?,?,?,?,?,'pending',?)`, r.ID, r.ReporterID, r.ContentType, r.ContentID, r.Reason, r.CreatedAt)
	return mapErr(err)
}

func (s *DB) ListPendingContentReports(ctx context.Context, limit int) ([]*model.ContentReport, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,reporter_id,content_type,content_id,reason,status,created_at,resolution FROM content_reports WHERE status='pending' ORDER BY created_at,id LIMIT ?`, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := make([]*model.ContentReport, 0)
	for rows.Next() {
		var r model.ContentReport
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.ContentType, &r.ContentID, &r.Reason, &r.Status, &r.CreatedAt, &r.Resolution); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

func (s *DB) ReviewContentReport(ctx context.Context, id, reviewerID, action, resolution string, at int64) error {
	if action != "resolve" && action != "dismiss" {
		return store.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	var contentType, contentID string
	err = tx.QueryRowContext(ctx, `SELECT content_type,content_id FROM content_reports WHERE id=? AND status='pending'`, id).Scan(&contentType, &contentID)
	if err != nil {
		return mapErr(err)
	}
	status := map[bool]string{true: "resolved", false: "dismissed"}[action == "resolve"]
	res, err := tx.ExecContext(ctx, `UPDATE content_reports SET status=?,reviewed_by=?,reviewed_at=?,resolution=? WHERE id=? AND status='pending'`, status, reviewerID, at, resolution, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return store.ErrNotFound
	}
	auditAction := map[string]string{"resolve": "resolve_report", "dismiss": "dismiss_report"}[action]
	if _, err := tx.ExecContext(ctx, `INSERT INTO moderation_actions(id,content_type,content_id,revision_id,action,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,?)`, util.NewID(), "report", contentType+":"+contentID, id, auditAction, reviewerID, resolution, at); err != nil {
		return mapErr(err)
	}
	return mapErr(tx.Commit())
}

var _ store.ContentReportStore = (*DB)(nil)
