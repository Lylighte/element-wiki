package render

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// RD-04：标题锚点 + TOC 提取。一次遍历同时完成。

var tocContextKey = parser.NewContextKey()

type headingAnchorTOCTransformer struct{}

func (h *headingAnchorTOCTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	var toc []TOCItem
	used := map[string]bool{}

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		head, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		textContent := string(head.Text(reader.Source()))
		if textContent == "" {
			return ast.WalkContinue, nil
		}
		id := slugifyHeading(textContent)
		for base, i := id, 2; used[id]; i++ {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		used[id] = true
		head.SetAttributeString("id", []byte(id))
		toc = append(toc, TOCItem{Level: head.Level, Text: textContent, ID: id})
		return ast.WalkContinue, nil
	})

	if len(toc) > 0 {
		pc.Set(tocContextKey, toc)
	}
}

var (
	spaceRe  = regexp.MustCompile(`\s+`)
	unsafeRe = regexp.MustCompile(`[^\p{L}\p{N}\-_]`)
)

// slugifyHeading 生成稳定锚点：小写、空白折叠为 -、剔除危险字符；保留 CJK。
func slugifyHeading(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = spaceRe.ReplaceAllString(s, "-")
	s = unsafeRe.ReplaceAllString(s, "")
	if s == "" {
		s = "section"
	}
	return s
}
