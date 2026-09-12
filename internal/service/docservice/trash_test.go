// T5.1/T5.2/T5.3 验收：回收站软删/恢复/彻底删除、到期清扫、blob GC。
package docservice

import (
	"context"
	"errors"
	"fmt"
	"testing"

	store "element-wiki/internal/database"
	sqlitestore "element-wiki/internal/database/sqlite"
	"element-wiki/internal/model"
	"element-wiki/internal/permission"
	"element-wiki/internal/search"
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

// —— M18 恢复落位测试辅助：直接读取/操作底层行 ——
func rawQueryInt(t *testing.T, svc *Service, q string, into *int) {
	t.Helper()
	if err := lastDB[svc].QueryRow(q).Scan(into); err != nil {
		t.Fatal(err)
	}
}

func rawExec(t *testing.T, svc *Service, q string, args ...any) {
	t.Helper()
	if _, err := lastDB[svc].Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

func rawScanString(t *testing.T, svc *Service, q string, arg any, into *string) {
	t.Helper()
	if err := lastDB[svc].QueryRow(q, arg).Scan(into); err != nil {
		t.Fatal(err)
	}
}

func rawScanContainer(t *testing.T, svc *Service, id string) (slug, visibility string) {
	t.Helper()
	if err := lastDB[svc].QueryRow(
		`SELECT slug, visibility FROM documents WHERE id=?`, id).Scan(&slug, &visibility); err != nil {
		t.Fatal(err)
	}
	return
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
	if err := svc.RestoreDocument(ctx, viewer(), root.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 恢复应拒绝: %v", err)
	}

	// 恢复 → 落「已恢复」容器 + 子树结构保留 + 内容完整 + 索引快照重建
	if err := svc.RestoreDocument(ctx, act, root.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got, err := svc.Get(ctx, act, root.ID)
	if err != nil {
		t.Fatalf("恢复后根不可见: %v", err)
	}
	if got.ParentID == nil || *got.ParentID == "" {
		t.Fatalf("恢复根应挂在容器下: %+v", got.ParentID)
	}
	slug, vis := rawScanContainer(t, svc, *got.ParentID)
	if slug != "restored" || vis != string(model.VisibilityRestricted) {
		t.Errorf("容器 = %s/%s", slug, vis)
	}
	if _, err := svc.Get(ctx, act, child.ID); err != nil {
		t.Errorf("子节点未随子树恢复: %v", err)
	}
	body, head, _ := svc.HeadContent(ctx, act, root.ID)
	if body != "root body" || head == nil {
		t.Errorf("内容丢失: %q", body)
	}

	// 容器 restricted：恢复文档生效可见性 restricted（viewer/匿名 404 掩护）
	if vis, verr := svc.trees.EffectiveVisibility(ctx, root.ID); verr != nil || vis != model.VisibilityRestricted {
		t.Errorf("容器内生效可见性 = %s,%v", vis, verr)
	}
	if _, err := svc.Get(ctx, viewer(), root.ID); !IsNotFound(err) {
		t.Errorf("viewer 读容器内文档应 404 掩护: %v", err)
	}

	// 恢复后 purge_at 已清空
	trashList, _ = svc.ListTrash(ctx, act, 100)
	for _, d := range trashList {
		if d.ID == root.ID || d.ID == child.ID {
			t.Errorf("恢复后仍在回收站: %s", d.ID)
		}
	}
}

// 洞1（M18 修复）：父链被彻底删除（purge，行已不存在）后恢复 → 落容器成功且树可见。
func TestRestoreAfterParentPurged(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	parent, _ := svc.CreateDocument(ctx, act, nil, "dead-parent", "P")
	childDoc, _ := svc.CreateDocument(ctx, act, &parent.ID, "orphan", "O")

	// 直接构造悬挂行：软删 child 后物理删除其父行（模拟历史 purge 断链；需临时关闭 FK）
	svc.TrashDocument(ctx, act, childDoc.ID)
	rawExec(t, svc, `PRAGMA foreign_keys=OFF`)
	rawExec(t, svc, `DELETE FROM documents WHERE id=?`, parent.ID)
	rawExec(t, svc, `PRAGMA foreign_keys=ON`)

	if err := svc.RestoreDocument(ctx, act, childDoc.ID); err != nil {
		t.Fatalf("父行缺失时恢复应成功: %v", err)
	}
	got, _ := svc.Get(ctx, act, childDoc.ID)
	if got.ParentID == nil {
		t.Fatal("恢复后应有容器父级")
	}
	// 树可见：容器子节点含恢复文档
	kids, _ := svc.ListChildrenForTree(ctx, act, got.ParentID)
	seen := false
	for _, k := range kids {
		if k.ID == childDoc.ID {
			seen = true
		}
	}
	if !seen {
		t.Error("恢复文档应出现在容器子节点（树可见）")
	}
}

// 洞2（M18 修复）：容器内 slug 冲突 → 自增 -2；恢复永不失败于此。
func TestRestoreSlugConflictAutoIncrement(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	a, _ := svc.CreateDocument(ctx, act, nil, "dup", "A")
	p, _ := svc.CreateDocument(ctx, act, nil, "dup-parent", "P")
	b, _ := svc.CreateDocument(ctx, act, &p.ID, "dup", "B")
	svc.TrashDocument(ctx, act, a.ID)
	svc.TrashDocument(ctx, act, b.ID)

	if err := svc.RestoreDocument(ctx, act, a.ID); err != nil {
		t.Fatalf("首次恢复: %v", err)
	}
	if err := svc.RestoreDocument(ctx, act, b.ID); err != nil {
		t.Fatalf("二次恢复应自增 slug: %v", err)
	}
	got, _ := svc.Get(ctx, act, b.ID)
	if got.Slug != "dup-2" {
		t.Errorf("slug 应自增为 dup-2, got %q", got.Slug)
	}
}

// 自增上限 20 后仍冲突 → ErrConflict（零覆盖既有内容）。
func TestRestoreSlugConflictExhausted(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	// 预置容器与 install、install-2..install-21 全部占用
	container, _ := svc.CreateDocument(ctx, act, nil, "restored", "已恢复")
	svc.SetVisibility(ctx, act, container.ID, model.VisibilityRestricted)
	for n := 1; n <= 21; n++ {
		slug := "install"
		if n > 1 {
			slug = fmt.Sprintf("install-%d", n)
		}
		if _, err := svc.CreateDocument(ctx, act, &container.ID, slug, slug); err != nil {
			t.Fatalf("预置 %s: %v", slug, err)
		}
	}
	trashRoot, _ := svc.CreateDocument(ctx, act, nil, "install", "V")
	svc.TrashDocument(ctx, act, trashRoot.ID)
	err := svc.RestoreDocument(ctx, act, trashRoot.ID)
	if !errors.Is(err, store.ErrConflict) {
		t.Errorf("20 次自增耗尽应 ErrConflict: %v", err)
	}
	// 既有内容零覆盖
	var n int
	rawQueryInt(t, svc, `SELECT COUNT(*) FROM documents WHERE slug LIKE 'install%' AND deleted_at IS NULL`, &n)
	if n != 21 {
		t.Errorf("既有 install* 不应增减: %d", n)
	}
}

// 容器被回收/改名后，下次恢复惰性重建。
func TestRestoredRootRecreated(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "rc-doc", "D")
	svc.TrashDocument(ctx, act, d.ID)
	if err := svc.RestoreDocument(ctx, act, d.ID); err != nil {
		t.Fatal(err)
	}
	first, _ := svc.Get(ctx, act, d.ID)

	// 回收容器本身（普通文档语义）→ 再次恢复时重建新容器
	container := first.ParentID
	svc.TrashDocument(ctx, act, *container)
	other, _ := svc.CreateDocument(ctx, act, nil, "rc-doc-2", "D2")
	svc.TrashDocument(ctx, act, other.ID)
	if err := svc.RestoreDocument(ctx, act, other.ID); err != nil {
		t.Fatalf("容器缺失时应重建: %v", err)
	}
	got, _ := svc.Get(ctx, act, other.ID)
	if got.ParentID == nil || *got.ParentID == *container {
		t.Errorf("应挂在新容器下: %+v", got.ParentID)
	}
	var vis string
	rawScanString(t, svc, `SELECT visibility FROM documents WHERE id=?`, got.ParentID, &vis)
	if vis != string(model.VisibilityRestricted) {
		t.Errorf("重建容器可见性 = %s", vis)
	}
}

// 恢复与 purge 计时交互：恢复后 purge_at 清空，不再被到期清扫。
func TestRestoreClearsPurgeSchedule(t *testing.T) {
	svc, _ := newTrashSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "purge-sched", "P")
	svc.TrashDocument(ctx, act, d.ID)
	if err := svc.RestoreDocument(ctx, act, d.ID); err != nil {
		t.Fatal(err)
	}
	// 推进到期时间后清扫：不应清掉已恢复文档
	count, err := svc.SweepPurgeDue(ctx, nowMillis()+86400_000*400)
	if err != nil {
		t.Fatal(err)
	}
	_ = count
	if _, err := svc.Get(ctx, act, d.ID); err != nil {
		t.Errorf("恢复后不应被清扫: %v", err)
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
	if err := svc.RestoreDocument(ctx, act, d.ID); err != nil {
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
