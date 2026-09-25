package sqlite

import (
	"context"

	"element-wiki/internal/model"
)

func (s *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, mapErr(err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SetUnmodifiedTimezoneDefault keeps the file-config default effective until an
// administrator saves a timezone through the settings API (which sets updated_at).
func (s *DB) SetUnmodifiedTimezoneDefault(ctx context.Context, timezone string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE settings SET value=? WHERE key='timezone' AND updated_at=0`, timezone)
	return mapErr(err)
}

func (s *DB) SetSettings(ctx context.Context, patch map[string]string, by string, at int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	for k, v := range patch {
		if _, err := tx.ExecContext(ctx, `
UPDATE settings SET value=?, updated_at=?, updated_by=? WHERE key=?`, v, at, by, k); err != nil {
			return mapErr(err)
		}
	}
	return tx.Commit()
}

var _ = model.JobDone
