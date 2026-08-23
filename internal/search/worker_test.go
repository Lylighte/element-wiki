// T4.2/T4.4 worker 验收：单篇恢复、全量重建、缺失文档清理、周期运行。
package search

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"element-wiki/internal/database"
	sqlitestore "element-wiki/internal/store/sqlite"
	"element-wiki/internal/util"

	"element-wiki/migrations"
)

type workerEnv struct {
	t    *testing.T
	deps *RebuildDeps
	idx  *Index
	db   *sql.DB
}

func newWorkerEnv(t *testing.T) (*workerEnv, func(slug, marker string) string) {
	t.Helper()
	db, err := database.Open("sqlite", filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := &migrations.Migrator{DB: db, Dialect: "sqlite"}
	if err := m.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO users (id,issuer,subject,email,display_name,created_at)
		VALUES ('u1','i','u1','','U',1)`); err != nil {
		t.Fatal(err)
	}
	idx, err := Open(filepath.Join(t.TempDir(), "documents.bleve"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	impl := sqlitestore.New(db)
	deps := &RebuildDeps{Jobs: impl, Docs: impl, Coms: impl, Index: idx,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	makeDoc := func(slug, marker string) string {
		docID := util.NewID()
		blob := "blob-" + slug
		commit := "c-" + slug
		if _, err := db.Exec(`INSERT INTO documents (id,slug,title,head_commit_id,created_by,updated_by,created_at,updated_at)
			VALUES (?,?,'标题','','u1','u1',1,1)`, docID, slug); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO document_blobs (hash,content,size,created_at)
			VALUES (?, ?, 10, 1)`, blob, marker); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO document_commits (id,document_id,commit_no,blob_hash,author_id,created_at)
			VALUES (?, ?, 1, ?, 'u1', 1)`, commit, docID, blob); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE documents SET head_commit_id=? WHERE id=?`, commit, docID); err != nil {
			t.Fatal(err)
		}
		return docID
	}
	return &workerEnv{t: t, deps: deps, idx: idx, db: db}, makeDoc
}

func findHitW(t *testing.T, idx *Index, term, docID string) bool {
	t.Helper()
	hits, err := idx.Query(context.Background(), term, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.DocumentID == docID {
			return true
		}
	}
	return false
}

func TestWorkerRestoresSingleDoc(t *testing.T) {
	e, makeDoc := newWorkerEnv(t)
	ctx := context.Background()
	id := makeDoc("restore-me", "unique restore marker")

	if err := e.deps.Index.DeleteDoc(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.deps.Jobs.EnqueueReindex(ctx, &id, "update"); err != nil {
		t.Fatal(err)
	}
	if !e.deps.ProcessOnce(ctx) {
		t.Fatal("应有任务可消费")
	}
	if !findHitW(t, e.deps.Index, "restore", id) {
		t.Error("worker 应恢复索引项")
	}
	if e.deps.ProcessOnce(ctx) {
		t.Error("队列应已空")
	}
}

func TestWorkerFullRebuild(t *testing.T) {
	e, makeDoc := newWorkerEnv(t)
	ctx := context.Background()
	idA := makeDoc("full-a", "alpha rebuild content")
	idB := makeDoc("full-b", "beta rebuild content")

	// 清空索引制造不一致
	e.deps.Index.DeleteDoc(ctx, idA)
	e.deps.Index.DeleteDoc(ctx, idB)

	if _, err := e.deps.Jobs.EnqueueReindex(ctx, nil, "manual"); err != nil {
		t.Fatal(err)
	}
	if !e.deps.ProcessOnce(ctx) {
		t.Fatal("应消费全量任务")
	}
	if !findHitW(t, e.deps.Index, "alpha", idA) || !findHitW(t, e.deps.Index, "beta", idB) {
		t.Error("全量重建后两篇均应可检索")
	}
}

func TestWorkerRemovesIndexForMissingDoc(t *testing.T) {
	e, _ := newWorkerEnv(t)
	ctx := context.Background()
	ghost := util.NewID()
	if err := e.deps.Index.IndexDoc(ctx, Doc{DocumentID: ghost,
		Title: "ghost", Content: "ghostbody"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.deps.Jobs.EnqueueReindex(ctx, &ghost, "delete"); err != nil {
		t.Fatal(err)
	}
	e.deps.ProcessOnce(ctx)
	if findHitW(t, e.deps.Index, "ghostbody", ghost) {
		t.Error("缺失文档的索引项应被清除")
	}
}

func TestRunRebuildWorkerPeriodic(t *testing.T) {
	e, makeDoc := newWorkerEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id := makeDoc("periodic", "periodic marker body")
	e.deps.Index.DeleteDoc(ctx, id)
	if _, err := e.deps.Jobs.EnqueueReindex(ctx, &id, "update"); err != nil {
		t.Fatal(err)
	}

	go RunRebuildWorker(ctx, *e.deps, 5*time.Millisecond)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if findHitW(t, e.deps.Index, "marker", id) {
			cancel()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("周期 worker 未在超时内恢复索引")
}
