package render

import (
	"net/url"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// T2.5：URL 策略——仅放行 http/https/mailto/锚点/相对地址；
// 其余（javascript:/data:/vbscript: 等）降级为纯文本节点。

type urlGuardTransformer struct{}

func (u *urlGuardTransformer) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch link := n.(type) {
		case *ast.Link:
			if !safeURL(string(link.Destination)) {
				replaceWithText(link, reader)
				return ast.WalkSkipChildren, nil // 已替换，跳过子树
			}
		case *ast.AutoLink:
			if !safeURL(string(link.URL(reader.Source()))) {
				replaceWithText(link, reader)
				return ast.WalkSkipChildren, nil
			}
		case *ast.Image:
			if !safeURL(string(link.Destination)) {
				parent := n.Parent()
				if parent != nil {
					parent.ReplaceChild(parent, n, ast.NewString([]byte("[图片已屏蔽]")))
					return ast.WalkSkipChildren, nil
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

func safeURL(dest string) bool {
	dest = strings.TrimSpace(dest)
	low := strings.ToLower(dest)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") ||
		strings.HasPrefix(low, "mailto:") {
		return true
	}
	if strings.HasPrefix(dest, "#") { // 页内锚点
		return true
	}
	if dest == "" {
		return true
	}
	if u, err := url.Parse(dest); err == nil && u.Scheme == "" {
		return true // 相对路径
	}
	return false
}

// replaceWithText 将链接节点替换为其可见文本（保留信息，去除可点击性）。
func replaceWithText(node ast.Node, reader text.Reader) {
	parent := node.Parent()
	if parent == nil {
		return
	}
	label := nodeText(node, reader)
	parent.InsertBefore(parent, node, ast.NewString([]byte(label)))
	parent.RemoveChild(parent, node)
}

func nodeText(n ast.Node, reader text.Reader) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(reader.Source()))
		} else {
			sb.WriteString(nodeText(c, reader))
		}
	}
	if sb.Len() == 0 {
		sb.WriteString("(link)")
	}
	return sb.String()
}
