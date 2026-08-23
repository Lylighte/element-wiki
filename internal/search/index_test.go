// T4.1 验收：索引生命周期（建/开/写/查/删/关）+ CJK 与短语查询 + 高亮。
package search

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func openTemp(t *testing.T) *Index {
	t.Helper()
	idx, err := Open(filepath.Join(t.TempDir(), "documents.bleve"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

func TestLifecycleAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idx.bleve")
	ctx := context.Background()

	i1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := i1.IndexDoc(ctx, Doc{DocumentID: "d1", Title: "安装指南", Content: "运行 install 脚本", UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	n, _ := i1.DocCount(ctx)
	if n != 1 {
		t.Fatalf("count = %d", n)
	}
	if err := i1.Close(); err != nil {
		t.Fatal(err)
	}

	// 重新打开已有目录：数据仍在
	i2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer i2.Close()
	hits, err := i2.Query(ctx, "install", 10)
	if err != nil || len(hits) != 1 || hits[0].DocumentID != "d1" {
		t.Fatalf("重开后检索: %+v %v", hits, err)
	}
}

func TestCJKQueryMatches(t *testing.T) {
	idx := openTemp(t)
	ctx := context.Background()
	for _, d := range []Doc{
		{DocumentID: "a", Title: "部署手册", Content: "使用 Docker Compose 部署服务"},
		{DocumentID: "b", Title: "渲染管线", Content: "goldmark 扩展与中文分词说明"},
	} {
		if err := idx.IndexDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := idx.Query(ctx, "部署", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hits {
		if h.DocumentID == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("中文词 '部署' 应命中文档 a: %+v", hits)
	}
}

func TestExactPhraseQuery(t *testing.T) {
	idx := openTemp(t)
	ctx := context.Background()
	idx.IndexDoc(ctx, Doc{DocumentID: "p1", Content: "alpha beta gamma"})
	idx.IndexDoc(ctx, Doc{DocumentID: "p2", Content: "gamma beta alpha"})

	// 短语顺序敏感
	hits, err := idx.Query(ctx, `"alpha beta"`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "p1" {
		t.Errorf("精确短语应只命中 p1: %+v", hits)
	}
}

func TestHighlightSnippet(t *testing.T) {
	idx := openTemp(t)
	ctx := context.Background()
	idx.IndexDoc(ctx, Doc{DocumentID: "h", Content: "the quick brown fox jumps"})
	hits, err := idx.Query(ctx, "fox", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits = %+v %v", hits, err)
	}
	if !strings.Contains(hits[0].Snippet, "<mark>fox</mark>") {
		t.Errorf("高亮缺失: %q", hits[0].Snippet)
	}
}

func TestDeleteRemovesFromResults(t *testing.T) {
	idx := openTemp(t)
	ctx := context.Background()
	idx.IndexDoc(ctx, Doc{DocumentID: "gone", Content: "vanish me"})
	idx.DeleteDoc(ctx, "gone")
	hits, _ := idx.Query(ctx, "vanish", 5)
	if len(hits) != 0 {
		t.Errorf("删除后不应命中: %+v", hits)
	}
}

func TestEmptyQueryReturnsEmpty(t *testing.T) {
	idx := openTemp(t)
	hits, err := idx.Query(context.Background(), "   ", 5)
	if err != nil || len(hits) != 0 {
		t.Errorf("空查询应空结果: %+v %v", hits, err)
	}
}
