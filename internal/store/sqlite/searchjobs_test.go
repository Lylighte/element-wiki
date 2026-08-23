// T4.2/T4.4 存储层验收：重建任务队列 + 存活 ID 枚举。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

func TestSearchJobQueueLifecycle(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)

	docID := "doc-j1"
	if _, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
		VALUES ('` + docID + `',NULL,'jq','t','u1','u1',1,1)`); err != nil {
		t.Fatal(err)
	}

	id1, err := s.EnqueueReindex(ctx, &docID, "update")
	if err != nil || id1 == "" {
		t.Fatalf("入队: %v", err)
	}
	id2, err := s.EnqueueReindex(ctx, nil, "manual")
	if err != nil {
		t.Fatal(err)
	}

	// FIFO：先消费单篇任务
	j, err := s.PopPending(ctx)
	if err != nil || j.ID != id1 || j.DocumentID == nil || *j.DocumentID != docID {
		t.Fatalf("pop1 = %+v %v", j, err)
	}
	if j.Status != model.JobRunning {
		t.Errorf("pop 后应 running: %s", j.Status)
	}
	if err := s.FinishReindexJob(ctx, id1, false, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetReindexJob(ctx, id1)
	if got.Status != model.JobDone {
		t.Errorf("应为 done: %s", got.Status)
	}

	// 全量任务
	j2, err := s.PopPending(ctx)
	if err != nil || j2.ID != id2 || j2.DocumentID != nil {
		t.Fatalf("pop2 = %+v %v", j2, err)
	}
	if err := s.FinishReindexJob(ctx, id2, true, "boom"); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.GetReindexJob(ctx, id2)
	if got2.Status != model.JobFailed || got2.LastErr != "boom" {
		t.Errorf("失败任务记录异常: %+v", got2)
	}

	// 队列空
	if _, err := s.PopPending(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("空队列应 NotFound: %v", err)
	}
	// 幽灵状态更新
	if err := s.FinishReindexJob(ctx, "ghost", false, ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("幽灵收尾应 NotFound: %v", err)
	}
}

func TestListAliveIDsExcludesTrashed(t *testing.T) {
	s := New(openMigrated(t))
	ctx := context.Background()
	seedUserRow(t, s)
	mk := func(id, slug string) {
		if _, err := s.db.Exec(`INSERT INTO documents (id,parent_id,slug,title,created_by,updated_by,created_at,updated_at)
			VALUES ('` + id + `',NULL,'` + slug + `','t','u1','u1',1,1)`); err != nil {
			t.Fatal(err)
		}
	}
	mk("alive", "a1")
	mk("dead", "a2")
	if _, err := s.db.Exec(`UPDATE documents SET deleted_at=9 WHERE id='dead'`); err != nil {
		t.Fatal(err)
	}

	ids, err := s.ListAliveIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundAlive, foundDead := false, false
	for _, id := range ids {
		if id == "alive" {
			foundAlive = true
		}
		if id == "dead" {
			foundDead = true
		}
	}
	if !foundAlive || foundDead {
		t.Errorf("存活枚举异常: %v", ids)
	}
}
