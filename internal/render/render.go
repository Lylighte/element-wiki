// Package render 实现 Markdown 渲染管线（RD-01~RD-08）。
package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// TOCItem 是目录条目（RD-04）。
type TOCItem struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
}

// Result 是一次渲染的产物。
type Result struct {
	HTML string
	TOC  []TOCItem
}

// NewEngine 构建全站统一渲染引擎。
//
// 安全基线（T2.5）：不启用 Unsafe，原生 HTML 一律转义；
// URL 策略见 urlguard.go；数学公式保持 $ 定界符原样交给前端 KaTeX（T2.2 约定），
// ```mermaid 围栏透传语言类给前端 Mermaid。
func NewEngine() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Footnote,
			wikilinkExt{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
			parser.WithASTTransformers(
				util.Prioritized(&alertTransformer{}, 100),
				util.Prioritized(&headingAnchorTOCTransformer{}, 200),
				util.Prioritized(&urlGuardTransformer{}, 300),
			),
		),
	)
}

var engine = NewEngine()

// Render 渲染 Markdown 并产出 HTML 与目录。
func Render(src string) (*Result, error) {
	ctx := parser.NewContext()
	var buf bytes.Buffer
	if err := engine.Convert([]byte(src), &buf, parser.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	res := &Result{HTML: buf.String()}
	if items := ctx.Get(tocContextKey); items != nil {
		res.TOC = items.([]TOCItem)
	}
	return res, nil
}

// firstLineText 提取段落首个文本行的原始内容。
func firstLineText(p *ast.Paragraph, reader text.Reader) string {
	lines := p.Lines()
	if lines.Len() == 0 {
		return ""
	}
	seg := lines.At(0)
	return string(seg.Value(reader.Source()))
}

var _ = ast.NewString
var _ = text.NewReader
