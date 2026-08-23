package sqlite

import (
	"context"
	"database/sql"

	"element-wiki/internal/store"
)

func (s *DB) CreateSession(ctx context.Context, tokenHash, userID string, expiresAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)`,
		tokenHash, userID, expiresAt, nowMs())
	return mapErr(err)
}

var errNoSession = sql.ErrNoRows

func (s *DB) GetSession(ctx context.Context, tokenHash string) (string, int64, error) {
	var userID string
	var exp int64
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&userID, &exp)
	if err != nil {
		if err == errNoSession {
			return "", 0, store.ErrNotFound
		}
		return "", 0, mapErr(err)
	}
	return userID, exp, nil
}

func (s *DB) DeleteSession(ctx context.Context, tokenHash string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}
