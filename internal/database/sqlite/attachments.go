package sqlite

import (
	"context"

	"element-wiki/internal/model"
)

const attachCols = `id, document_id, filename, storage_path, mime_type, size, sha256, uploaded_by, created_at`

func (s *DB) CreateAttachment(ctx context.Context, a *model.Attachment) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO attachments (`+attachCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.DocumentID, a.Filename, a.StoragePath, a.MimeType,
		a.Size, a.SHA256, a.UploadedBy, a.CreatedAt)
	return mapErr(err)
}

func scanAttach(row interface{ Scan(...any) error }) (*model.Attachment, error) {
	var a model.Attachment
	err := row.Scan(&a.ID, &a.DocumentID, &a.Filename, &a.StoragePath,
		&a.MimeType, &a.Size, &a.SHA256, &a.UploadedBy, &a.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (s *DB) GetAttachment(ctx context.Context, id string) (*model.Attachment, error) {
	a, err := scanAttach(s.db.QueryRowContext(ctx,
		`SELECT `+attachCols+` FROM attachments WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *DB) ListAttachments(ctx context.Context, docID string) ([]*model.Attachment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+attachCols+` FROM attachments WHERE document_id=? ORDER BY created_at, id`, docID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []*model.Attachment{}
	for rows.Next() {
		a, err := scanAttach(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *DB) DeleteAttachment(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM attachments WHERE id=?`, id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return mapErr(errNoRowsWrap())
	}
	return nil
}
