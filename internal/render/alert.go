package render

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// GFM Alert（RD-06）：`> [!NOTE]` 风格引用块转换为带语义类别的提示块。
// 类名沿用 GitHub 约定，便于前端复用样式。

var alertRe = regexp.MustCompile(`^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)

var alertTitles = map[string]string{
	"NOTE":      "Note",
	"TIP":       "Tip",
	"IMPORTANT": "Important",
	"WARNING":   "Warning",
	"CAUTION":   "Caution",
}

type alertTransformer struct{}

func (a *alertTransformer) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		bq, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue, nil
		}
		p, ok := bq.FirstChild().(*ast.Paragraph)
		if !ok {
			return ast.WalkContinue, nil
		}
		first := strings.TrimRight(firstLineText(p, reader), "\r")
		m := alertRe.FindStringSubmatch(first)
		if m == nil {
			return ast.WalkContinue, nil
		}
		kind := strings.ToLower(m[1])
		bq.SetAttributeString("class", []byte("markdown-alert markdown-alert-"+kind))

		// 标题段落：<p class="markdown-alert-title"><strong>Note</strong></p>
		title := ast.NewParagraph()
		title.SetAttributeString("class", []byte("markdown-alert-title"))
		em := ast.NewEmphasis(2)
		em.AppendChild(em, ast.NewString([]byte(alertTitles[m[1]])))
		title.AppendChild(title, em)

		if p.Lines().Len() <= 1 && p.ChildCount() <= 1 {
			// 整段就是标记行：替换为标题段
			bq.ReplaceChild(bq, p, title)
			return ast.WalkSkipChildren, nil
		}
		// 标记行后还有内容：插入标题，并从原段删除首行
		bq.InsertBefore(bq, p, title)
		removeFirstLine(p)
		return ast.WalkSkipChildren, nil
	})
}

// removeFirstLine 删除段落的首个行片段。
func removeFirstLine(p *ast.Paragraph) {
	lines := p.Lines()
	if lines.Len() == 0 {
		return
	}
	lines.SetSliced(1, lines.Len()) // 保留第 2 行起的切片
}
