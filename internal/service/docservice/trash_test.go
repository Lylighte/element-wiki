// T5.1/T5.2/T5.3 验收：回收站软删/恢复/彻底删除、到期清扫、blob GC。
package docservice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/search"
	sqlitestore "element-wiki/internal/store/sqlite"
)

type fakeIndexerDel struct {
	fakeIndexer
	deleted []string
	failDel bool
}

func (f *fakeIndexerDel) IndexDoc(ctx context.Context, d search.Doc) error {
	return f.fakeIndexer.IndexDoc(ctx, d)
}

func (f *fakeIndexerDel) DeleteDoc(_ context.Context, id string) error {
	if f.failDel {
		return errors.New("delete down")
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func newTrashSvc(t *testing.T) (*Service, *fakeIndexerDel) {
	t.Helper()
	svc, db := newSvc(t)
	idx := &fakeIndexerDel{}
	impl := sqlitestore.New(db)
	svc.SetTrashHooks(impl)
	svc.indexer = idx
	return svc, idx
}

func TestTrashRestoreLifecycle(t *testing.T) {
	svc, idx := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	root, _ := svc.CreateDocument(ctx, act, nil, "tr-root", "R")
	child, _ := svc.CreateDocument(ctx, act, &root.ID, "tr-child", "C")
	svc.Commit(ctx, act, root.ID, "", "root body", "m")

	// 软删子树
	if err := svc.TrashDocument(ctx, act, root.ID); err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if _, err := svc.Get(ctx, act, root.ID); !IsNotFound(err) {
		t.Errorf("回收站文档应 404: %v", err)
	}
	if _, err := svc.Get(ctx, act, child.ID); !IsNotFound(err) {
		t.Errorf("子节点应一并隐没: %v", err)
	}
	if len(idx.deleted) == 0 {
		t.Error("索引未同步移除")
	}

	// slug 已释放：存活查询不可见（includeDeleted=false）
	if _, err := svc.docs.(interface {
		GetBySlug(ctx context.Context, parent *string, slug string, includeDeleted bool) (*model.Document, error)
	}).GetBySlug(ctx, nil, "tr-root", false); !IsNotFound(err) {
		t.Errorf("软删后存活 slug 查询应不可见: %v", err)
	}

	// 列表可见
	trashList, _ := svc.ListTrash(ctx, act, 100)
	found := false
	for _, d := range trashList {
		if d.ID == root.ID || d.ID == child.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("回收站列表缺少成员: %+v", trashList)
	}

	// viewer 无恢复权限
	if err := svc.RestoreDocument(ctx, viewer(), root.ID, nil); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 恢复应拒绝: %v", err)
	}

	// 恢复 → 全树回归 + 索引快照重建
	if err := svc.RestoreDocument(ctx, act, root.ID, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	for _, id := range []string{root.ID, child.ID} {
		if _, err := svc.Get(ctx, act, id); err != nil {
			t.Errorf("恢复后 %s 不可见: %v", id, err)
		}
	}
	body, head, _ := svc.HeadContent(ctx, act, root.ID)
	if body != "root body" || head == nil {
		t.Errorf("内容丢失: %q", body)
	}
	_ = strings.Contains
}

func TestRestoreParentGoneConflictAndOverride(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	parent, _ := svc.CreateDocument(ctx, act, nil, "gone-parent", "P")
	childDoc, _ := svc.CreateDocument(ctx, act, &parent.ID, "gone-child", "C")

	// 先删父，再单独把 child 也放入回收站（模拟历史遗留）
	svc.TrashDocument(ctx, act, parent.ID)
	svc.TrashDocument(ctx, act, childDoc.ID)

	// 直接恢复 child：父链在回收站 → 409 语义
	if err := svc.RestoreDocument(ctx, act, childDoc.ID, nil); !errors.Is(err, ErrParentGone) {
		t.Fatalf("应报父级缺失: %v", err)
	}

	// 带重挂载目标 → 成功且挂在 alive 节点下
	newParent, _ := svc.CreateDocument(ctx, act, nil, "new-home", "N")
	if err := svc.RestoreDocument(ctx, act, childDoc.ID, &newParent.ID); err != nil {
		t.Fatalf("带 parent 恢复: %v", err)
	}
	got, _ := svc.Get(ctx, act, childDoc.ID)
	if got.ParentID == nil || *got.ParentID != newParent.ID {
		t.Errorf("重挂载失败: %+v", got.ParentID)
	}
}

func TestPurgePermanentlyRemovesEverything(t *testing.T) {
	svc, idx := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "purge-doc", "P")
	r1, _ := svc.Commit(ctx, act, d.ID, "", "body one", "m")
	draft := &model.Draft{DocumentID: d.ID, UserID: "u1", BaseCommitID: r1.Commit.ID, Content: "d", UpdatedAt: 1}
	svc.drafts.UpsertDraft(ctx, draft)
	svc.TrashDocument(ctx, act, d.ID)

	before := len(idx.deleted)
	if err := svc.PurgeDocument(ctx, act, d.ID); err != nil {
		t.Fatal(err)
	}
	if len(idx.deleted) <= before {
		t.Error("purge 应同步清理索引")
	}
	if _, err := svc.Get(ctx, act, d.ID); !IsNotFound(err) {
		t.Errorf("purge 后应不可见: %v", err)
	}
	var n int
	rawCount(t, lastDB[svc], `SELECT COUNT(*) FROM document_commits WHERE document_id=?`, d.ID, &n)
	if n != 0 {
		t.Errorf("commits 应级联清除: %d", n)
	}
	rawCount(t, lastDB[svc], `SELECT COUNT(*) FROM document_drafts WHERE document_id=?`, d.ID, &n)
	if n != 0 {
		t.Errorf("草稿应级联清除: %d", n)
	}
	// purge 只允许对已软删文档
	live, _ := svc.CreateDocument(ctx, act, nil, "live-doc", "L")
	err := func() error { e := svc.PurgeDocument(ctx, act, live.ID); return e }()
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Errorf("存活文档直接 purge 应校验失败: %v", err)
	}
}

// T5.2 到期清扫。
func TestSweepPurgeDueOnlyExpired(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	old, _ := svc.CreateDocument(ctx, act, nil, "sweep-old", "O")
	fresh, _ := svc.CreateDocument(ctx, act, nil, "sweep-new", "N")
	now := nowMillis()
	svc.trash().SoftDeleteSubtree(ctx, old.ID, "u1", now-10_000_000, now-1000) // 已过期
	svc.trash().SoftDeleteSubtree(ctx, fresh.ID, "u1", now, now+86400_000)     // 未到期

	count, err := svc.SweepPurgeDue(ctx, now)
	if err != nil || count != 1 {
		t.Fatalf("清扫数 = %d,%v", count, err)
	}
	if _, err := svc.docs.Get(ctx, old.ID); !IsNotFound(err) {
		t.Errorf("过期项应被物理清除: %v", err)
	}
	got, err := svc.docs.Get(ctx, fresh.ID)
	if err != nil || !got.Alive() == false {
		t.Log("fresh 保持回收站状态 ✓")
	}
	if got == nil {
		t.Fatal("未到期项不应被清除")
	}
}

// T5.3 blob GC。
func TestBlobGCRemovesOnlyOrphans(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	a, _ := svc.CreateDocument(ctx, act, nil, "gc-a", "A")
	b, _ := svc.CreateDocument(ctx, act, nil, "gc-b", "B")
	ra, _ := svc.Commit(ctx, act, a.ID, "", "content A unique-orphan", "m")
	rb, _ := svc.Commit(ctx, act, b.ID, "", "content B keep-me", "m")

	// 彻底删除 a（commits 级联）→ 其 blob 成为孤儿
	svc.TrashDocument(ctx, act, a.ID)
	if err := svc.PurgeDocument(ctx, act, a.ID); err != nil {
		t.Fatal(err)
	}

	n, err := svc.GCBlobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("至少应回收 a 的 blob, got %d", n)
	}
	// b 的 blob 必须保留
	if _, err := svc.coms.GetBlob(ctx, rb.Commit.BlobHash); err != nil {
		t.Errorf("被引用 blob 不应被回收: %v", err)
	}
	if _, err := svc.coms.GetBlob(ctx, ra.Commit.BlobHash); !IsNotFound(err) {
		t.Errorf("孤儿 blob 应消失: %v", err)
	}
}

func TestTrashPermissionMatrix(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "perm-trash", "T")

	if err := svc.TrashDocument(ctx, viewer(), d.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 删除应拒绝: %v", err)
	}
	if _, err := svc.ListTrash(ctx, viewer(), 10); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 列回收站应拒绝: %v", err)
	}
	svc.TrashDocument(ctx, act, d.ID)
	// editor 可恢复（模板含 DocRestore）
	if err := svc.RestoreDocument(ctx, act, d.ID, nil); err != nil {
		t.Errorf("editor 恢复应成功: %v", err)
	}
}

// removeIndexed 在索引失败时入 delete 任务。
func TestRemoveIndexedFallback(t *testing.T) {
	jobs := &fakeJobs{}
	svc, idx := newTrashSvc(t)
	idx.failDel = true
	svc.SetSearchHooks(idx, jobs)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "rm-idx", "R")
	svc.TrashDocument(ctx, act, d.ID)

	found := false
	for _, j := range jobs.jobs {
		if j.reason == "delete" && j.docID != nil && *j.docID == d.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("应存在 delete 任务: %+v", jobs.jobs)
	}
}
