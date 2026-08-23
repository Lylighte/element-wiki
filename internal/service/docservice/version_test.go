// T1.6/T1.7 验收：提交冲突零污染、死链报告、revert 不改写历史。
package docservice

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"element-wiki/internal/permission"
	"element-wiki/internal/store"
)

func TestCommitHappyPathWithDeadLinks(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	target, _ := svc.CreateDocument(ctx, act, nil, "target-doc", "T")
	_ = target
	d, _ := svc.CreateDocument(ctx, act, nil, "main", "M")

	content := "# M\n\n见 [[target-doc]] 与 [[ghost-link]]\n"
	res, err := svc.Commit(ctx, act, d.ID, "", content, "init")
	if err != nil {
		t.Fatalf("首次提交: %v", err)
	}
	if res.Commit.CommitNo != 1 || res.Commit.ParentCommitID != nil {
		t.Errorf("首版元数据异常: %+v", res.Commit)
	}
	if !slices.Contains(res.DeadLinks, "ghost-link") || slices.Contains(res.DeadLinks, "target-doc") {
		t.Errorf("死链报告错误: %v", res.DeadLinks)
	}

	got, _ := svc.Get(ctx, act, d.ID)
	if got.HeadCommitID != res.Commit.ID {
		t.Error("HEAD 未推进")
	}
	body, head, err := svc.HeadContent(ctx, act, d.ID)
	if err != nil || body != content || head.CommitNo != 1 {
		t.Fatalf("HEAD 正文读回 = %q,%+v,%v", body, head, err)
	}

	// 第二版 parent 链正确
	res2, err := svc.Commit(ctx, act, d.ID, res.Commit.ID, "# v2\n", "second")
	if err != nil {
		t.Fatal(err)
	}
	if *res2.Commit.ParentCommitID != res.Commit.ID || res2.Commit.CommitNo != 2 {
		t.Errorf("父链/序号异常: %+v", res2.Commit)
	}
}

func TestCommitConflictZeroPollution(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "conflict-doc", "C")
	r1, _ := svc.Commit(ctx, act, d.ID, "", "v1", "one")

	// 用过期 base 提交 → 409 语义，且事实来源零污染
	stale := strings.Replace(r1.Commit.ID, "0", "Z", 1) // 必然不等于 HEAD
	_, err := svc.Commit(ctx, act, d.ID, stale, "v-bad", "bad")
	vc, ok := AsVersionConflict(err)
	if !ok {
		t.Fatalf("过期 base 应返回版本冲突, got %v", err)
	}
	if vc.HeadCommitID != r1.Commit.ID {
		t.Errorf("冲突应携带当前 HEAD: %+v", vc)
	}

	got, _ := svc.Get(ctx, act, d.ID)
	if got.HeadCommitID != r1.Commit.ID {
		t.Errorf("HEAD 被污染: %s", got.HeadCommitID)
	}
	list, _ := svc.ListCommits(ctx, act, d.ID, 10)
	if len(list) != 1 {
		t.Fatalf("版本数被污染: %d", len(list))
	}
	draft, _ := svc.GetDraft(ctx, act, d.ID)
	_ = draft // 无草稿时 NotFound 属正常；关键是无新写入
	if _, err := svc.GetDraft(ctx, act, d.ID); err != nil && !IsNotFound(err) {
		t.Errorf("草稿读取异常: %v", err)
	}
}

func TestDraftFlowThroughService(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()
	d, _ := svc.CreateDocument(ctx, act, nil, "draft-flow", "D")

	if _, err := svc.GetDraft(ctx, act, d.ID); !IsNotFound(err) {
		t.Fatalf("无草稿应 ErrNotFound, got %v", err)
	}
	if err := svc.SaveDraft(ctx, act, d.ID, "base-x", "wip"); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.GetDraft(ctx, act, d.ID)
	if err != nil || draft.Content != "wip" || draft.BaseCommitID != "base-x" {
		t.Fatalf("草稿 = %+v,%v", draft, err)
	}
	// viewer 无 DocUpdate
	if err := svc.SaveDraft(ctx, viewer(), d.ID, "b", "c"); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 存草稿应拒绝: %v", err)
	}
	if err := svc.DeleteDraft(ctx, act, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteDraft(ctx, act, d.ID); !IsNotFound(err) {
		t.Errorf("二次删除应 ErrNotFound: %v", err)
	}
}

func TestRevertCreatesNewVersionWithoutRewritingHistory(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "revert-doc", "R")
	r1, _ := svc.Commit(ctx, act, d.ID, "", "# version one\n", "first")
	r2, _ := svc.Commit(ctx, act, d.ID, r1.Commit.ID, "# version two\n", "second")

	rv, err := svc.Revert(ctx, act, d.ID, r1.Commit.ID)
	if err != nil {
		t.Fatalf("回滚: %v", err)
	}
	if rv.Commit.CommitNo != 3 {
		t.Errorf("回滚应是第 3 版: %d", rv.Commit.CommitNo)
	}
	body, head, _ := svc.HeadContent(ctx, act, d.ID)
	if body != "# version one\n" || head.ID != rv.Commit.ID {
		t.Errorf("回滚后 HEAD 内容/指针异常: %q %+v", body, head)
	}

	// 历史不可变：r2 的 blob 内容仍可读且未变化
	oldBlob, err := svc.coms.GetBlob(ctx, r2.Commit.BlobHash)
	if err != nil || oldBlob != "# version two\n" {
		t.Errorf("历史版本被改写: %q,%v", oldBlob, err)
	}

	// viewer 无 revert 权限
	if _, err := svc.Revert(ctx, viewer(), d.ID, r1.Commit.ID); !errors.Is(err, permission.ErrDenied) {
		t.Errorf("viewer 回滚应拒绝: %v", err)
	}
}

// T1.4 服务侧联动：maxVersions=100 默认下连续 105 次提交只留 100 版。
func TestServiceVersionCapTrimsOldest(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	act := editor()

	d, _ := svc.CreateDocument(ctx, act, nil, "cap-doc", "C")
	base := ""
	var lastID string
	for i := 1; i <= 105; i++ {
		r, err := svc.Commit(ctx, act, d.ID, base, "body "+strings.Repeat("x", i), "n")
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		base = r.Commit.ID
		lastID = r.Commit.ID
	}
	list, _ := svc.ListCommits(ctx, act, d.ID, 500)
	if len(list) != 100 {
		t.Fatalf("裁剪后版本数 = %d, want 100", len(list))
	}
	if list[0].ID != lastID || list[0].CommitNo != 105 {
		t.Errorf("HEAD 应为 #105: %+v", list[0])
	}
	if list[len(list)-1].CommitNo != 6 {
		t.Errorf("最旧保留应为 #6, got #%d", list[len(list)-1].CommitNo)
	}
	_ = store.ErrNotFound
}
