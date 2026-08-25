// T1.3 验收：blob 去重、commit 序列、原子 AppendCommit、草稿 UPSERT。
// T1.4 验收：版本上限裁剪（maxVersions）。
package sqlite

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

type stores struct {
	docs  store.DocumentStore
	coms  store.CommitStore
	app   store.AppendCommitter
	draft store.DraftStore
}

func allStores(t *testing.T) stores {
	t.Helper()
	db := openMigrated(t)
	impl := New(db)
	return stores{docs: impl, coms: impl, app: impl, draft: impl}
}

func seedDocWithUser(t *testing.T, s stores, slug string) *model.Document {
	t.Helper()
	ctx := context.Background()
	seedUserRow(t, s.docs)
	d := doc(t, util_NewID(), "", slug)
	mustCreate(t, s.docs, ctx, d)
	return d
}

func mkCommit(docID string, no int64, blob string) *model.Commit {
	var p *string
	if no > 1 {
		p = ptr("prev")
	}
	return &model.Commit{
		ID: util_NewID(), DocumentID: docID, CommitNo: no,
		ParentCommitID: p, BlobHash: blob, AuthorID: "u1",
		Message: "msg", CreatedAt: 1000 + no,
	}
}

// appendN 顺序写入 n 个版本（blob 复用同一个 hash）。
func appendN(t *testing.T, s stores, ctx context.Context, docID string, n, maxVersions int64) {
	t.Helper()
	if err := s.coms.PutBlob(ctx, "shared-hash", "body"); err != nil {
		t.Fatal(err)
	}
	for no := int64(1); no <= n; no++ {
		c := mkCommit(docID, no, "shared-hash")
		if _, err := s.app.AppendCommit(ctx, c, maxVersions, nil); err != nil {
			t.Fatalf("append #%d: %v", no, err)
		}
	}
}

func TestBlobDedupAndRoundtrip(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	if err := s.coms.PutBlob(ctx, "h1", "content-v1"); err != nil {
		t.Fatal(err)
	}
	if err := s.coms.PutBlob(ctx, "h1", "ignored-dup"); err != nil {
		t.Fatal(err) // 幂等不报错
	}
	got, err := s.coms.GetBlob(ctx, "h1")
	if err != nil || got != "content-v1" {
		t.Fatalf("blob 读回 = %q,%v（首次写入必须不被覆盖）", got, err)
	}
	if _, err := s.coms.GetBlob(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("缺失 blob 应 ErrNotFound, got %v", err)
	}
}

func TestGetCommitScopedByDocumentAndListOrder(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "c-list")
	appendN(t, s, ctx, d.ID, 3, 0)

	list, err := s.coms.ListCommits(ctx, d.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].CommitNo != 3 || list[2].CommitNo != 1 {
		t.Fatalf("降序排列异常: %+v", list)
	}
	if n, _ := s.coms.CountCommits(ctx, d.ID); n != 3 {
		t.Fatalf("count = %d", n)
	}

	if _, err := s.coms.GetCommit(ctx, "other-doc", list[0].ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("跨文档查询应 ErrNotFound, got %v", err)
	}
	if _, err := s.coms.ListCommits(ctx, d.ID, 0); err == nil {
		t.Error("limit<1 应报错")
	}
}

func TestAppendCommitUpdatesHeadAtomically(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "head-test")

	if err := s.coms.PutBlob(ctx, "b1", "one"); err != nil {
		t.Fatal(err)
	}
	c1 := mkCommit(d.ID, 1, "b1")
	if _, err := s.app.AppendCommit(ctx, c1, 0, nil); err != nil {
		t.Fatal(err)
	}

	got, _ := s.docs.Get(ctx, d.ID)
	if got.HeadCommitID != c1.ID {
		t.Fatalf("HEAD 未推进到 %s: %q", c1.ID, got.HeadCommitID)
	}
	back, err := s.coms.GetCommit(ctx, d.ID, got.HeadCommitID)
	if err != nil || back.CommitNo != 1 {
		t.Fatalf("HEAD 指针不可解析: %v,%v", back, err)
	}
}

func TestAppendCommitErrorBranches(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "errs")
	mustExecRaw(t, s, `INSERT INTO document_blobs (hash,content,size,created_at) VALUES ('bh','x',1,1)`)

	first := mkCommit(d.ID, 1, "bh")
	if _, err := s.app.AppendCommit(ctx, first, 0, nil); err != nil {
		t.Fatal(err)
	}

	// commit_no 重复 → ErrConflict
	err := func() error {
		_, e := s.app.AppendCommit(ctx, mkCommit(d.ID, 1, "bh"), 0, nil)
		return e
	}()
	if !errors.Is(err, store.ErrConflict) {
		t.Errorf("重复版本号应 ErrConflict, got %v", err)
	}

	// 幽灵文档（FK 缺失 users/documents 链）→ ErrInvalid 或 ErrNotFound 均为拒绝
	ghost := mkCommit("ghost-doc", 1, "bh")
	ghost.AuthorID = "ghost-user"
	_, err = s.app.AppendCommit(ctx, ghost, 0, nil)
	if err == nil {
		t.Fatal("幽灵文档提交必须失败")
	}

	// HEAD 不因失败而改变
	got, _ := s.docs.Get(ctx, d.ID)
	if got.HeadCommitID != first.ID {
		t.Errorf("失败路径污染了 HEAD: %q", got.HeadCommitID)
	}
}

func mustExecRaw(t *testing.T, s stores, q string) {
	t.Helper()
	if _, err := rawOf(s.docs).Exec(q); err != nil {
		t.Fatal(err)
	}
}

// T1.4：上限裁剪——超出 keep 的最旧版本同事务删除，HEAD 与剩余版本不受影响。
func TestAppendCommitTrimsBeyondCap(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "trim")
	appendN(t, s, ctx, d.ID, 5, 3)

	n, _ := s.coms.CountCommits(ctx, d.ID)
	if n != 3 {
		t.Fatalf("裁剪后应剩 3 版, got %d", n)
	}
	list, _ := s.coms.ListCommits(ctx, d.ID, 10)
	if list[0].CommitNo != 5 || list[len(list)-1].CommitNo != 3 {
		t.Fatalf("应保留 #3~#5: %+v", list)
	}
	head, _ := s.docs.Get(ctx, d.ID)
	if head.HeadCommitID != list[0].ID {
		t.Error("HEAD 必须指向最新版")
	}
	// blob 不裁剪（GC 任务负责）
	var blobs int
	mustCount(t, s, `SELECT COUNT(*) FROM document_blobs`, &blobs)
	if blobs != 1 {
		t.Errorf("blob 应保留待 GC, got %d", blobs)
	}
}

func mustCount(t *testing.T, s stores, q string, into *int) {
	t.Helper()
	if err := rawOf(s.docs).QueryRow(q).Scan(into); err != nil {
		t.Fatal(err)
	}
}

func TestDraftUpsertGetDelete(t *testing.T) {
	s := allStores(t)
	ctx := context.Background()
	d := seedDocWithUser(t, s, "draft-doc")

	up := &model.Draft{DocumentID: d.ID, UserID: "u1", BaseCommitID: "base1", Content: "v1", UpdatedAt: 10}
	if err := s.draft.UpsertDraft(ctx, up); err != nil {
		t.Fatal(err)
	}
	up.Content = "v2"
	up.UpdatedAt = 20
	if err := s.draft.UpsertDraft(ctx, up); err != nil {
		t.Fatalf("重复 UPSERT 应为覆盖: %v", err)
	}
	got, err := s.draft.GetDraft(ctx, d.ID, "u1")
	if err != nil || got.Content != "v2" || got.BaseCommitID != "base1" {
		t.Fatalf("草稿读回 = %+v,%v", got, err)
	}

	if err := s.draft.DeleteDraft(ctx, d.ID, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.draft.GetDraft(ctx, d.ID, "u1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("删除后应 ErrNotFound, got %v", err)
	}
	if err := s.draft.DeleteDraft(ctx, d.ID, "u1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复删除应 ErrNotFound, got %v", err)
	}
}
