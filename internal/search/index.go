// Package search 封装 Bleve 全文索引（AR-03）：索引为派生数据，
// 事实来源是数据库；支持 CJK 中文与精确短语查询。
package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	_ "github.com/blevesearch/bleve/v2/analysis/lang/cjk" // 注册 cjk 分析器
)

// Doc 是入索引的文档快照。
type Doc struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	UpdatedAt  int64  `json:"updated_at"`
}

// Hit 是单条检索结果；Snippet 为高亮 HTML 片段（<mark> 包裹）。
type Hit struct {
	DocumentID string
	Score      float64
	Snippet    string
}

// Index 线程安全的 Bleve 封装。
type Index struct {
	mu  sync.Mutex
	idx bleve.Index
}

// Open 打开或创建索引目录（T4.1）。
func Open(path string) (*Index, error) {
	i := &Index{}
	idx, err := bleve.Open(path)
	if err != nil {
		// 目录不存在 → 创建；映射默认分析器 cjk 覆盖中英混排
		mapping := bleve.NewIndexMapping()
		mapping.DefaultAnalyzer = "cjk"
		idx, err = bleve.New(path, mapping)
		if err != nil {
			return nil, fmt.Errorf("search: 创建索引失败: %w", err)
		}
	}
	i.idx = idx
	return i, nil
}

// Close 关闭底层索引。
func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return nil
	}
	err := i.idx.Close()
	i.idx = nil
	return err
}

// IndexDoc 以 document_id 为主键写入/覆盖。
func (i *Index) IndexDoc(_ context.Context, d Doc) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return fmt.Errorf("search: index closed")
	}
	if err := i.idx.Index(d.DocumentID, d); err != nil {
		return fmt.Errorf("search: 索引文档 %s 失败: %w", d.DocumentID, err)
	}
	return nil
}

// DeleteDoc 从索引移除文档。
func (i *Index) DeleteDoc(_ context.Context, documentID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return fmt.Errorf("search: index closed")
	}
	if err := i.idx.Delete(documentID); err != nil {
		return fmt.Errorf("search: 删除索引 %s 失败: %w", documentID, err)
	}
	return nil
}

// Query 执行检索：关键词 + `"精确短语"`（SE-05），返回带高亮片段的结果。
func (i *Index) Query(ctx context.Context, q string, limit int) ([]Hit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Hit{}, nil
	}
	if limit < 1 || limit > 200 {
		limit = 20
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return nil, fmt.Errorf("search: index closed")
	}

	req := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(q), limit, 0, false)
	req.Fields = []string{"content"}
	req.Highlight = bleve.NewHighlightWithStyle("html")
	res, err := i.idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search: 查询失败: %w", err)
	}
	out := make([]Hit, 0, len(res.Hits))
	for _, hit := range res.Hits {
		snippet := ""
		if frags, ok := hit.Fragments["content"]; ok && len(frags) > 0 {
			snippet = strings.Join(frags, " … ")
		}
		out = append(out, Hit{DocumentID: hit.ID, Score: hit.Score, Snippet: snippet})
	}
	return out, nil
}

// DocCount 返回索引内文档数（诊断用）。
func (i *Index) DocCount(ctx context.Context) (uint64, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return 0, fmt.Errorf("search: index closed")
	}
	return i.idx.DocCount()
}
