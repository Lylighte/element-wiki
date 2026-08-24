package sqlite

import (
	"context"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func (s *DB) EnqueueBackup(ctx context.Context, kind, requestedBy string) (string, error) {
	id := newIDForJobs()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO backup_jobs (id, kind, status, requested_by, created_at) VALUES (?,?, 'pending', ?, ?)`,
		id, kind, requestedBy, nowMs()); err != nil {
		return "", mapErr(err)
	}
	return id, nil
}

func (s *DB) SetBackupFilename(ctx context.Context, id, filename string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backup_jobs SET filename=?, started_at=? WHERE id=?`, filename, nowMs(), id)
	return mapErr(err)
}

func (s *DB) FinishBackup(ctx context.Context, id string, failed bool, lastErr string) error {
	status := model.JobDone
	if failed {
		status = model.JobFailed
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE backup_jobs SET status=?, last_error=NULLIF(?,''), finished_at=? WHERE id=?`,
		status, lastErr, nowMs(), id)
	return mapErr(err)
}

func (s *DB) GetBackupJob(ctx context.Context, id string) (*model.BackupJob, error) {
	var j model.BackupJob
	var fname, lastErr *string
	var start, fin *int64
	err := s.db.QueryRowContext(ctx, `
SELECT id, kind, COALESCE(filename,''), status, requested_by,
       COALESCE(last_error,''), created_at, COALESCE(started_at,0), COALESCE(finished_at,0)
FROM backup_jobs WHERE id=?`, id).
		Scan(&j.ID, &j.Kind, &fname, &j.Status, &j.RequestedBy,
			&lastErr, &j.CreatedAt, &start, &fin)
	if err != nil {
		return nil, mapErr(err)
	}
	j.Filename, j.StartedAt, j.FinishedAt = *fname, derefInt64(start), derefInt64(fin)
	if lastErr != nil {
		j.LastErr = *lastErr
	}
	return &j, nil
}

func (s *DB) EnqueueImport(ctx context.Context, requestedBy string) (string, error) {
	id := newIDForJobs()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO import_jobs (id, status, requested_by, created_at) VALUES (?, 'pending', ?, ?)`,
		id, requestedBy, nowMs()); err != nil {
		return "", mapErr(err)
	}
	return id, nil
}

func (s *DB) UpdateImportProgress(ctx context.Context, id string, total, imported, failed int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE import_jobs SET total_files=?, imported_files=?, failed_files=?, started_at=? WHERE id=?`,
		total, imported, failed, nowMs(), id)
	return mapErr(err)
}

func (s *DB) FinishImport(ctx context.Context, id string, failed bool, lastErr string) error {
	status := model.JobDone
	if failed {
		status = model.JobFailed
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE import_jobs SET status=?, last_error=NULLIF(?,''), finished_at=? WHERE id=?`,
		status, lastErr, nowMs(), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *DB) GetImportJob(ctx context.Context, id string) (*model.ImportJob, error) {
	var j model.ImportJob
	var lastErr *string
	var start, fin *int64
	err := s.db.QueryRowContext(ctx, `
SELECT id, status, total_files, imported_files, failed_files, requested_by,
       COALESCE(last_error,''), created_at, COALESCE(started_at,0), COALESCE(finished_at,0)
FROM import_jobs WHERE id=?`, id).
		Scan(&j.ID, &j.Status, &j.TotalFiles, &j.ImportedFiles, &j.FailedFiles,
			&j.RequestedBy, &lastErr, &j.CreatedAt, &start, &fin)
	if err != nil {
		return nil, mapErr(err)
	}
	j.StartedAt, j.FinishedAt = derefInt64(start), derefInt64(fin)
	if lastErr != nil {
		j.LastErr = *lastErr
	}
	return &j, nil
}
