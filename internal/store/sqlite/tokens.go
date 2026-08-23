package sqlite

import (
	"context"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func (s *DB) CreateToken(ctx context.Context, tk *model.APIToken) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO api_tokens (id, user_id, name, prefix, token_hash, created_at)
VALUES (?,?,?,?,?,?)`,
		tk.ID, tk.UserID, tk.Name, tk.Prefix, tk.TokenHash, tk.CreatedAt)
	return mapErr(err)
}

func (s *DB) GetTokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	var tk model.APIToken
	var revoked *int64
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, name, prefix, token_hash, created_at, last_used_at, revoked_at
FROM api_tokens WHERE token_hash = ?`,
		hash).Scan(&tk.ID, &tk.UserID, &tk.Name, &tk.Prefix, &tk.TokenHash,
		&tk.CreatedAt, &tk.LastUsedAt, &revoked)
	if err != nil {
		return nil, mapErr(err)
	}
	tk.RevokedAt = revoked
	return &tk, nil
}

func (s *DB) ListTokensByUser(ctx context.Context, userID string) ([]*model.APIToken, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, name, prefix, created_at, last_used_at, revoked_at
FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.APIToken{}
	for rows.Next() {
		var tk model.APIToken
		var revoked *int64
		if err := rows.Scan(&tk.ID, &tk.UserID, &tk.Name, &tk.Prefix,
			&tk.CreatedAt, &tk.LastUsedAt, &revoked); err != nil {
			return nil, mapErr(err)
		}
		tk.RevokedAt = revoked
		out = append(out, &tk)
	}
	return out, rows.Err()
}

func (s *DB) RevokeToken(ctx context.Context, id, userID string, at int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		at, id, userID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *DB) TouchToken(ctx context.Context, id string, at int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, at, id)
	return mapErr(err)
}
