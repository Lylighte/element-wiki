// 重建 worker：消费 search_reindex_jobs（T4.2 降级通道 + T4.4 手动全量）。
package search

import (
	"context"
	"log/slog"
	"time"

	"element-wiki/internal/model"
	"element-wiki/internal/store"
)

// RebuildDeps 汇集 worker 所需能力。
type RebuildDeps struct {
	Jobs  store.SearchJobStore
	Docs  store.DocumentStore
	Coms  store.CommitStore
	Index *Index
	Log   *slog.Logger
}

// LoadDoc 从事实来源装载单篇快照；文档缺失返回 ErrNotFound。
func (d *RebuildDeps) LoadDoc(ctx context.Context, docID string) (*Doc, error) {
	doc, err := d.Docs.Get(ctx, docID)
	if err != nil {
		return nil, err
	}
	content := ""
	if doc.HeadCommitID != "" {
		head, err := d.Coms.GetCommit(ctx, docID, doc.HeadCommitID)
		if err != nil {
			return nil, err
		}
		content, err = d.Coms.GetBlob(ctx, head.BlobHash)
		if err != nil {
			return nil, err
		}
	}
	return &Doc{DocumentID: docID, Title: doc.Title,
		Content: content, UpdatedAt: doc.UpdatedAt}, nil
}

// ProcessOnce 处理一个 pending 任务；无任务返回 false。
func (d *RebuildDeps) ProcessOnce(ctx context.Context) bool {
	job, err := d.Jobs.PopPending(ctx)
	if err != nil {
		return false // ErrNotFound 或瞬态错误：本轮结束
	}
	var runErr error
	if job.DocumentID == nil {
		runErr = d.rebuildAll(ctx)
	} else if _, gerr := d.Docs.Get(ctx, *job.DocumentID); gerr != nil {
		// 文档已不存在 → 同步删除索引
		runErr = d.Index.DeleteDoc(ctx, *job.DocumentID)
	} else {
		var snap *Doc
		snap, runErr = d.LoadDoc(ctx, *job.DocumentID)
		if runErr == nil {
			runErr = d.Index.IndexDoc(ctx, *snap)
		}
	}
	if failErr := d.Jobs.FinishReindexJob(ctx, job.ID, runErr != nil, errText(runErr)); failErr != nil {
		d.Log.Error("任务收尾失败", "job", job.ID, "err", failErr)
	}
	if runErr != nil {
		d.Log.Warn("重建任务失败", "job", job.ID, "err", runErr)
	}
	return true
}

func (d *RebuildDeps) rebuildAll(ctx context.Context) error {
	ids, err := d.Docs.ListAliveIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		snap, lerr := d.LoadDoc(ctx, id)
		if lerr != nil {
			continue // 单篇失败不阻断全量
		}
		if ierr := d.Index.IndexDoc(ctx, *snap); ierr != nil {
			return ierr
		}
	}
	return nil
}

// RunRebuildWorker 周期性消费队列，直到 ctx 取消。
func RunRebuildWorker(ctx context.Context, deps RebuildDeps, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for deps.ProcessOnce(ctx) { // 清空当前积压
			}
		}
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

var _ = model.JobPending
