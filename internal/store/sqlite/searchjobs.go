package sqlite

import (
	"context"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func (s *DB) EnqueueReindex(ctx context.Context, documentID *string, reason string) (string, error) {
	id := newIDForJobs()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO search_reindex_jobs (id, document_id, reason, status, created_at)
		 VALUES (?,?,?, 'pending', ?)`, id, documentID, reason, nowMs()); err != nil {
		return "", mapErr(err)
	}
	return id, nil
}

func (s *DB) GetReindexJob(ctx context.Context, id string) (*model.SearchJob, error) {
	var j model.SearchJob
	var docID *string
	var fin *int64
	var lastErr *string
	err := s.db.QueryRowContext(ctx, `
SELECT id, document_id, reason, status, attempts, last_error, created_at, finished_at
FROM search_reindex_jobs WHERE id = ?`, id).
		Scan(&j.ID, &docID, &j.Reason, &j.Status, &j.Attempts, &lastErr, &j.CreatedAt, &fin)
	if err != nil {
		return nil, mapErr(err)
	}
	j.DocumentID, j.FinishedAt = docID, derefInt64(fin)
	if lastErr != nil {
		j.LastErr = *lastErr
	}
	return &j, nil
}

func (s *DB) PopPending(ctx context.Context) (*model.SearchJob, error) {
	var j model.SearchJob
	var docID *string
	err := s.db.QueryRowContext(ctx, `
SELECT id, document_id, reason FROM search_reindex_jobs
WHERE status = 'pending' ORDER BY created_at LIMIT 1`).
		Scan(&j.ID, &docID, &j.Reason)
	if err != nil {
		return nil, mapErr(err)
	}
	j.DocumentID = docID
	if _, err := s.db.ExecContext(ctx,
		`UPDATE search_reindex_jobs SET status='running', attempts=attempts+1 WHERE id=?`,
		j.ID); err != nil {
		return nil, mapErr(err)
	}
	j.Status = model.JobRunning
	return &j, nil
}

func (s *DB) FinishReindexJob(ctx context.Context, id string, failed bool, lastErr string) error {
	status := model.JobDone
	if failed {
		status = model.JobFailed
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE search_reindex_jobs SET status=?, last_error=?, finished_at=? WHERE id=?`,
		status, nullStr(lastErr), nowMs(), id)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}
