package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

const userCols = `id, issuer, subject, email, display_name, role, status, created_at, last_login_at`

func scanUser(row interface{ Scan(...any) error }) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Issuer, &u.Subject, &u.Email, &u.DisplayName,
		&u.Role, &u.Status, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (s *DB) CreateUser(ctx context.Context, u *model.User) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO users (id, issuer, subject, email, display_name, role, status, created_at, last_login_at)
VALUES (?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Issuer, u.Subject, u.Email, u.DisplayName,
		string(u.Role), u.Status, u.CreatedAt, u.LastLoginAt)
	return mapErr(err)
}

func (s *DB) GetUser(ctx context.Context, id string) (*model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *DB) FindUserByIssuerSubject(ctx context.Context, issuer, subject string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE issuer = ? AND subject = ?`, issuer, subject))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	return u, err
}

func (s *DB) FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE lower(email) = lower(?)`, email))
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *DB) UpdateUserRole(ctx context.Context, id string, role permission.Role) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *DB) UpdateUserStatus(ctx context.Context, id string, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *DB) TouchLogin(ctx context.Context, id string, at int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = ? WHERE id = ?`, at, id)
	return mapErr(err)
}

func (s *DB) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, mapErr(err)
}

func (s *DB) ListUsers(ctx context.Context, q string, limit int) ([]*model.User, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	base := `SELECT ` + userCols + ` FROM users`
	var args []any
	if qq := "%" + q + "%"; q != "" {
		base += ` WHERE email LIKE ? OR display_name LIKE ?`
		args = append(args, qq, qq)
	}
	base += ` ORDER BY created_at, id LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
