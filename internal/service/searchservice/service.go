// Package searchservice 执行权限感知的全文检索（SE-03/04）。
package searchservice

import (
	"context"

	"element-wiki/internal/permission"
	"element-wiki/internal/search"
	"element-wiki/internal/store"
)

// Queryer 是索引查询面（便于测试替身）。
type Queryer interface {
	Query(ctx context.Context, q string, limit int) ([]search.Hit, error)
}

// Service 在索引之上叠加可见性过滤与二次校验。
type Service struct {
	index Queryer
	docs  store.DocumentStore
	trees store.TreeStore
}

func New(index Queryer, docs store.DocumentStore, trees store.TreeStore) *Service {
	return &Service{index: index, docs: docs, trees: trees}
}

// Hit 是对外契约条目（doc/02 §7）。
type Hit struct {
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score"`
}

// Search：先按候选集查询，逐条做存活与生效可见性校验（AGENTS §9 禁忌的合规实现）。
func (s *Service) Search(ctx context.Context, actor permission.Actor,
	q string, limit int) ([]Hit, error) {
	if err := actor.Require(permission.DocRead); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	candidates, err := s.index.Query(ctx, q, limit*3+10)
	if err != nil {
		return nil, err
	}
	restrictedOK := actor.Has(permission.DocReadRestricted)
	out := make([]Hit, 0, limit)
	for _, h := range candidates {
		d, derr := s.docs.Get(ctx, h.DocumentID)
		if derr != nil || !d.Alive() {
			continue // 不存在或回收站：静默跳过，不泄露存在性
		}
		if !restrictedOK {
			vis, verr := s.trees.EffectiveVisibility(ctx, h.DocumentID)
			if verr != nil || vis == "restricted" {
				continue
			}
		}
		out = append(out, Hit{
			DocumentID: h.DocumentID, Title: d.Title,
			Snippet: h.Snippet, Score: h.Score,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}
