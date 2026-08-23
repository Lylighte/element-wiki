package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// RD-05：[[目标]] 与 [[目标|别名]] 内联语法。
// 以真正的 InlineParser 实现，天然不作用于代码块/行内代码。

var wikilinkKind = ast.NewNodeKind("Wikilink")

// Wikilink 节点：Target 为跳转目标，Label 为显示文本。
type Wikilink struct {
	ast.BaseInline
	Target string
	Label  string
}

func (n *Wikilink) Kind() ast.NodeKind { return wikilinkKind }
func (n *Wikilink) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type wikilinkParser struct{}

func (wikilinkParser) Trigger() []byte { return []byte{'['} }

func (wikilinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("[[")) {
		return nil
	}
	end := bytes.Index(line, []byte("]]"))
	if end < 2 {
		return nil
	}
	inner := string(line[2:end])
	if inner == "" || strings.ContainsAny(inner, "[\n\r]") {
		return nil
	}

	target, label := inner, inner
	if i := strings.IndexByte(inner, '|'); i >= 0 {
		target = strings.TrimSpace(inner[:i])
		label = strings.TrimSpace(inner[i+1:])
	}
	if target == "" || label == "" {
		return nil
	}

	block.Advance(end + 2)
	node := &Wikilink{Target: target, Label: label}
	node.AppendChild(node, ast.NewString([]byte(label)))
	return node
}

type wikilinkRenderer struct {
	html.Config
}

func newWikilinkRenderer() renderer.NodeRenderer {
	return &wikilinkRenderer{}
}

func (r *wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilinkKind, r.render)
}

func (r *wikilinkRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	wl := node.(*Wikilink)
	_, _ = fmt.Fprintf(w, `<a class="wikilink" data-target="%s">`,
		strings.ReplaceAll(strings.ReplaceAll(wl.Target, `"`, "&quot;"), "<", "&lt;"))
	return ast.WalkContinue, nil
}

type wikilinkExt struct{}

func (wikilinkExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(wikilinkParser{}, 60), // 早于默认 link 解析器(70)
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newWikilinkRenderer(), 60),
	))
}
