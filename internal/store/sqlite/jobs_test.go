// T6.5/T6.7 任务存储层验收。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func TestBackupJobStoreLifecycle(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)

	id, err := s.EnqueueBackup(ctx, "export", "u1")
	if err != nil || id == "" {
		t.Fatalf("enqueue: %v", err)
	}
	j, _ := s.GetBackupJob(ctx, id)
	if j.Status != model.JobPending || j.Kind != "export" {
		t.Fatalf("初始任务: %+v", j)
	}
	if err := s.SetBackupFilename(ctx, id, "b.zip"); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishBackup(ctx, id, false, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetBackupJob(ctx, id)
	if got.Status != model.JobDone || got.Filename != "b.zip" {
		t.Errorf("完成态异常: %+v", got)
	}
	if _, err := s.GetBackupJob(ctx, "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("幽灵应 NotFound: %v", err)
	}
}

func TestImportJobStoreProgress(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)

	id, err := s.EnqueueImport(ctx, "u1")
	if err != nil || id == "" {
		t.Fatalf("enqueue: %v", err)
	}
	if err := s.UpdateImportProgress(ctx, id, 10, 4, 1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetImportJob(ctx, id)
	if got.TotalFiles != 10 || got.ImportedFiles != 4 || got.FailedFiles != 1 {
		t.Errorf("进度异常: %+v", got)
	}
	if err := s.FinishImport(ctx, id, true, "mid-fail"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetImportJob(ctx, id)
	if got.Status != model.JobFailed || got.LastErr != "mid-fail" {
		t.Errorf("失败态异常: %+v", got)
	}
	if err := s.FinishImport(ctx, "ghost", false, ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("幽灵收尾应 NotFound: %v", err)
	}
}
