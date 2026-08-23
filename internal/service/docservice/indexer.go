package docservice

import (
	"context"
	"log/slog"

	"element-wiki/internal/model"
	"element-wiki/internal/search"
)

// Indexer 是搜索索引的写入面（SE-02：派生数据，可失败降级）。
type Indexer interface {
	IndexDoc(ctx context.Context, d search.Doc) error
	DeleteDoc(ctx context.Context, documentID string) error
}

// JobSink 是索引重建任务队列。
type JobSink interface {
	EnqueueReindex(ctx context.Context, documentID *string, reason string) (string, error)
}

// SetSearchHooks 注入索引与任务队列；两者均可为 nil（关闭同步）。
func (s *Service) SetSearchHooks(idx Indexer, jobs JobSink) {
	s.indexer = idx
	s.jobs = jobs
}

// reindexSnapshot 用当前 HEAD 内容刷新索引；失败入重建任务并照常返回。
func (s *Service) reindexSnapshot(ctx context.Context, docID string) {
	if s.indexer == nil {
		return
	}
	d, err := s.docs.Get(ctx, docID)
	if err != nil || !d.Alive() {
		return
	}
	content := ""
	if d.HeadCommitID != "" {
		head, cerr := s.coms.GetCommit(ctx, docID, d.HeadCommitID)
		if cerr != nil {
			s.enqueueReindex(ctx, &docID, "corrupt")
			return
		}
		content, _ = s.coms.GetBlob(ctx, head.BlobHash)
	}
	if err := s.indexer.IndexDoc(ctx, search.Doc{
		DocumentID: docID, Title: d.Title, Content: content, UpdatedAt: d.UpdatedAt,
	}); err != nil {
		slog.Warn("索引更新失败，已入重建队列", "doc", docID, "err", err)
		s.enqueueReindex(ctx, &docID, "update")
	}
}

func (s *Service) enqueueReindex(ctx context.Context, docID *string, reason string) {
	if s.jobs == nil {
		return
	}
	if _, err := s.jobs.EnqueueReindex(ctx, docID, reason); err != nil {
		slog.Error("重建任务入队失败", "err", err)
	}
}

var _ = model.JobDone // 保持 model 引用稳定
