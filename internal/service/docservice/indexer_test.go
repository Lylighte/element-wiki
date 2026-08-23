// T4.2 验收：commit 后同步索引；索引故障 → 入重建任务且提交仍 201；
// rename 刷新标题快照。
package docservice

import (
	"context"
	"errors"
	"testing"

	"element-wiki/internal/search"
)

type fakeIndexer struct {
	docs    []search.Doc
	fail    bool
	indexed int
}

func (f *fakeIndexer) IndexDoc(_ context.Context, d search.Doc) error {
	f.indexed++
	if f.fail {
		return errors.New("indexer down")
	}
	f.docs = append(f.docs, d)
	return nil
}
func (f *fakeIndexer) DeleteDoc(_ context.Context, id string) error { return nil }

func (f *fakeIndexer) latestFor(id string) *search.Doc {
	for i := len(f.docs) - 1; i >= 0; i-- {
		if f.docs[i].DocumentID == id {
			return &f.docs[i]
		}
	}
	return nil
}

type fakeJobs struct {
	jobs []struct {
		docID  *string
		reason string
	}
}

func (f *fakeJobs) EnqueueReindex(_ context.Context, docID *string, reason string) (string, error) {
	f.jobs = append(f.jobs, struct {
		docID  *string
		reason string
	}{docID, reason})
	return "job-1", nil
}

func TestCommitSyncsIndex(t *testing.T) {
	svc := newSvcWithHooks(t, &fakeIndexer{}, nil)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "idx-doc", "索引文档")
	idx := svc.indexer.(*fakeIndexer)
	if _, err := svc.Commit(ctx, act, d.ID, "", "正文内容", "m"); err != nil {
		t.Fatal(err)
	}
	snap := idx.latestFor(d.ID)
	if snap == nil || snap.Title != "索引文档" || snap.Content == "" || !containsStr(snap.Content, "正文内容") {
		t.Fatalf("索引快照异常: %+v", snap)
	}

	// rename 触发标题刷新
	ns := ptr("renamed-doc")
	nt := ptr("改名后")
	if err := svc.RenameDocument(ctx, act, d.ID, ns, nt); err != nil {
		t.Fatal(err)
	}
	snap = idx.latestFor(d.ID)
	if snap == nil || snap.Title != "改名后" || snap.Content == "" {
		t.Fatalf("rename 后索引未刷新: %+v", snap)
	}
}

func TestIndexerFailureEnqueuesJobAndCommitSucceeds(t *testing.T) {
	idx := &fakeIndexer{fail: true}
	jobs := &fakeJobs{}
	svc := newSvcWithHooks(t, idx, jobs)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "degraded", "D")
	res, err := svc.Commit(ctx, act, d.ID, "", "body", "m")
	if err != nil {
		t.Fatalf("索引故障不得阻断提交: %v", err)
	}
	if res.Commit.CommitNo != 1 {
		t.Errorf("提交结果异常: %+v", res.Commit)
	}
	if len(jobs.jobs) != 1 || jobs.jobs[0].reason != "update" ||
		jobs.jobs[0].docID == nil || *jobs.jobs[0].docID != d.ID {
		t.Fatalf("应恰好入队一个 update 任务: %+v", jobs.jobs)
	}
	if idx.indexed != 1 {
		t.Errorf("应尝试过一次写入: %d", idx.indexed)
	}
}

func TestNilHooksLegacySafe(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "legacy", "L")
	if _, err := svc.Commit(ctx, act, d.ID, "", "c", "m"); err != nil {
		t.Fatalf("nil hooks 不应 panic 或报错: %v", err)
	}
}

func newSvcWithHooks(t *testing.T, idx Indexer, jobs JobSink) *Service {
	t.Helper()
	svc, _ := newSvc(t)
	svc.SetSearchHooks(idx, jobs)
	return svc
}
