package sqlite

import (
	"context"

	"element-wiki/internal/store"
)

func (s *DB) DashboardStats(ctx context.Context) (*store.DashboardStatsView, error) {
	out := &store.DashboardStatsView{}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documents WHERE deleted_at IS NULL`).Scan(&out.DocumentsTotal); err != nil {
		return nil, mapErr(err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM comments`).Scan(&out.CommentsTotal); err != nil {
		return nil, mapErr(err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attachments`).Scan(&out.AttachmentsTotal); err != nil {
		return nil, mapErr(err)
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, slug, updated_at FROM documents
WHERE deleted_at IS NULL ORDER BY updated_at DESC LIMIT 5`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r store.RecentDocView
		if err := rows.Scan(&r.ID, &r.Title, &r.Slug, &r.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		out.RecentDocs = append(out.RecentDocs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err)
	}

	crows, err := s.db.QueryContext(ctx, `
SELECT u.id, COALESCE(NULLIF(u.display_name,''), u.email, u.id), COUNT(*)
FROM documents d JOIN users u ON u.id = d.updated_by
WHERE d.deleted_at IS NULL
GROUP BY u.id ORDER BY COUNT(*) DESC LIMIT 5`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer crows.Close()
	for crows.Next() {
		var c store.ContributorView
		if err := crows.Scan(&c.UserID, &c.Name, &c.Count); err != nil {
			return nil, mapErr(err)
		}
		out.Contributors = append(out.Contributors, c)
	}
	return out, crows.Err()
}
