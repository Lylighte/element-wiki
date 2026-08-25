// Package sqlite 实现 store 接口的 SQLite 方言。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

type DB struct {
	db *sql.DB
}

func New(db *sql.DB) *DB { return &DB{db: db} }

// mapErr 把驱动约束错误归一为 store 语义错误。
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint") {
		return fmt.Errorf("%w: %v", store.ErrConflict, err)
	}
	if strings.Contains(msg, "FOREIGN KEY constraint") || strings.Contains(msg, "CHECK constraint") {
		return fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	return err
}

const docCols = `id, parent_id, space_id, slug, title, sort_key, visibility,
head_commit_id, created_by, updated_by, created_at, updated_at,
deleted_at, deleted_by, purge_at`

func scanDoc(row interface{ Scan(...any) error }) (*model.Document, error) {
	var d model.Document
	var vis string
	err := row.Scan(&d.ID, &d.ParentID, &d.SpaceID, &d.Slug, &d.Title, &d.SortKey, &vis,
		&d.HeadCommitID, &d.CreatedBy, &d.UpdatedBy, &d.CreatedAt, &d.UpdatedAt,
		&d.DeletedAt, &d.DeletedBy, &d.PurgeAt)
	if err != nil {
		return nil, err
	}
	d.Visibility = model.Visibility(vis)
	return &d, nil
}

func (s *DB) Create(ctx context.Context, d *model.Document) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO documents (id, parent_id, space_id, slug, title, sort_key, visibility,
	head_commit_id, created_by, updated_by, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.ParentID, d.SpaceID, d.Slug, d.Title, d.SortKey, string(d.Visibility),
		d.HeadCommitID, d.CreatedBy, d.UpdatedBy, d.CreatedAt, d.UpdatedAt)
	return mapErr(err)
}

func (s *DB) Get(ctx context.Context, id string) (*model.Document, error) {
	d, err := scanDoc(s.db.QueryRowContext(ctx,
		`SELECT `+docCols+` FROM documents WHERE id = ?`, id))
	return d, mapErr(err)
}

func (s *DB) GetBySlug(ctx context.Context, parentID *string, slug string, includeDeleted bool) (*model.Document, error) {
	q := `SELECT ` + docCols + ` FROM documents WHERE slug = ? AND parent_id `
	args := []any{slug}
	if parentID == nil {
		q += "IS NULL"
	} else {
		q += "= ?"
		args = append(args, *parentID)
	}
	if !includeDeleted {
		q += " AND deleted_at IS NULL"
	}
	d, err := scanDoc(s.db.QueryRowContext(ctx, q, args...))
	return d, mapErr(err)
}

func (s *DB) ListChildren(ctx context.Context, parentID *string) ([]*model.Document, error) {
	q := `SELECT ` + docCols + ` FROM documents WHERE deleted_at IS NULL AND parent_id `
	var args []any
	if parentID == nil {
		q += "IS NULL"
	} else {
		q += "= ?"
		args = append(args, *parentID)
	}
	q += ` ORDER BY sort_key, slug, id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.Document{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *DB) UpdateMeta(ctx context.Context, id string, mut model.DocumentMut, updatedBy string, updatedAt int64) error {
	sets := []string{"updated_by = ?", "updated_at = ?"}
	args := []any{updatedBy, updatedAt}
	if mut.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *mut.Title)
	}
	if mut.Slug != nil {
		sets = append(sets, "slug = ?")
		args = append(args, *mut.Slug)
	}
	if mut.SortKey != nil {
		sets = append(sets, "sort_key = ?")
		args = append(args, *mut.SortKey)
	}
	if mut.Visibility != nil {
		sets = append(sets, "visibility = ?")
		args = append(args, string(*mut.Visibility))
	}
	args = append(args, id)
	res, err := s.db.ExecContext(ctx,
		`UPDATE documents SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *DB) Move(ctx context.Context, id string, parentID *string, updatedBy string, updatedAt int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE documents SET parent_id = ?, updated_by = ?, updated_at = ? WHERE id = ?`,
		parentID, updatedBy, updatedAt, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ReorderChildren 单事务批量写 sort_key=(i+1)*100（C1）；命中 0 行视为文档缺失。
func (s *DB) ReorderChildren(ctx context.Context, parentID *string, orderedIDs []string,
	updatedBy string, updatedAt int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapErr(err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`UPDATE documents SET sort_key = ?, updated_by = ?, updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL`)
	if err != nil {
		return mapErr(err)
	}
	defer stmt.Close()
	for i, id := range orderedIDs {
		res, err := stmt.ExecContext(ctx, int64(i+1)*100, updatedBy, updatedAt, id)
		if err != nil {
			return mapErr(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return store.ErrNotFound
		}
	}
	return mapErr(tx.Commit())
}

func (s *DB) ListAliveIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM documents WHERE deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
