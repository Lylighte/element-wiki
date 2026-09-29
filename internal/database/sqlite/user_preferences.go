package sqlite

import (
	"context"

	store "element-wiki/internal/database"
	"element-wiki/internal/model"
)

func (s *DB) GetUserPreferences(ctx context.Context, userID string) (*model.UserPreferences, error) {
	p := &model.UserPreferences{}
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, language, theme, updated_at FROM user_preferences WHERE user_id = ?`, userID,
	).Scan(&p.UserID, &p.Language, &p.Theme, &p.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return p, nil
}

func (s *DB) SetUserPreferences(ctx context.Context, p *model.UserPreferences) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_preferences (user_id, language, theme, updated_at) VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET language=excluded.language, theme=excluded.theme, updated_at=excluded.updated_at`,
		p.UserID, p.Language, p.Theme, p.UpdatedAt)
	return mapErr(err)
}

var _ store.UserPreferencesStore = (*DB)(nil)
