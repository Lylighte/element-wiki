// T2.1~T2.5 验收：各扩展黄金断言 + XSS 向量中和。
package render

import (
	"strings"
	"testing"
)

func renderHTML(t *testing.T, src string) *Result {
	t.Helper()
	res, err := Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res
}

func containsAll(hay string, subs ...string) bool {
	for _, s := range subs {
		if !strings.Contains(hay, s) {
			return false
		}
	}
	return true
}

func TestGFMTable(t *testing.T) {
	out := renderHTML(t, "| a | b |\n|---|---|\n| 1 | 2 |\n")
	if !containsAll(out.HTML, "<table>", "<th>a</th>", "<td>1</td>") {
		t.Errorf("表格渲染异常:\n%s", out.HTML)
	}
}

func TestStrikethroughAndTaskList(t *testing.T) {
	out := renderHTML(t, "~~gone~~\n\n- [x] done\n- [ ] todo\n")
	if !containsAll(out.HTML, "<del>gone</del>",
		`type="checkbox"`, "checked", `disabled`) {
		t.Errorf("删除线/任务清单异常:\n%s", out.HTML)
	}
}

func TestGFMAAlerts(t *testing.T) {
	out := renderHTML(t, "> [!WARNING]\n> careful here\n")
	if !containsAll(out.HTML,
		`class="markdown-alert markdown-alert-warning"`,
		`class="markdown-alert-title"`,
		"<strong>Warning</strong>",
		"careful here") {
		t.Errorf("Alert 结构异常:\n%s", out.HTML)
	}

	out = renderHTML(t, "> [!NOTE]\n> note body\n>\n> second para\n")
	if !containsAll(out.HTML, "markdown-alert-note", "<strong>Note</strong>", "note body", "second para") {
		t.Errorf("NOTE 多段 Alert 异常:\n%s", out.HTML)
	}

	// 普通引用不受影响
	out = renderHTML(t, "> plain quote\n")
	if strings.Contains(out.HTML, "markdown-alert") {
		t.Errorf("普通引用被误判为 Alert:\n%s", out.HTML)
	}
}

func TestMermaidAndMathConventions(t *testing.T) {
	// T2.2 约定：mermaid 围栏保留语言类；$ 定界符不被吞掉
	out := renderHTML(t, "```mermaid\ngraph TD; A-->B;\n```\n")
	if !strings.Contains(out.HTML, `class="language-mermaid"`) {
		t.Errorf("mermaid 语言类丢失:\n%s", out.HTML)
	}
	out = renderHTML(t, "质能方程 $E=mc^2$ 保持原样\n")
	if !strings.Contains(out.HTML, "$E=mc^2$") {
		t.Errorf("$ 定界符被破坏:\n%s", out.HTML)
	}
}

func TestTOCAndAnchors(t *testing.T) {
	out := renderHTML(t, "# Top\n## 安装指南\n### Advanced 配置\n## 安装指南\n")
	wantIDs := []string{"top", "安装指南", "advanced-配置", "安装指南-2"}
	for _, id := range wantIDs {
		if !strings.Contains(out.HTML, `id="`+id+`"`) {
			t.Errorf("缺少锚点 id=%q:\n%s", id, out.HTML)
		}
	}
	if len(out.TOC) != 4 || out.TOC[1].Text != "安装指南" || out.TOC[1].ID != "安装指南" {
		t.Errorf("TOC 内容异常: %+v", out.TOC)
	}
	if out.TOC[0].Level != 1 || out.TOC[1].Level != 2 {
		t.Errorf("TOC 层级异常: %+v", out.TOC)
	}
}

func TestWikilinkPreprocess(t *testing.T) {
	out := renderHTML(t, "参见 [[install-guide]] 与 [[alias|显示名]]\n")
	if !containsAll(out.HTML,
		`class="wikilink"`, `data-target="install-guide"`) {
		t.Errorf("wikilink 渲染异常:\n%s", out.HTML)
	}
	if !strings.Contains(out.HTML, "[[") == true && strings.Contains(renderHTML(t, "`[[code]]`").HTML, "data-target=") {
		t.Errorf("代码内的 wikilink 不应转换")
	}
}

// T2.5：XSS 向量全部中和。
func TestXSST_vectorsNeutralized(t *testing.T) {
	vectors := map[string]string{
		"raw script":        "<script>alert(1)</script>",
		"inline onerror":    `<img src=x onerror="alert(1)">`,
		"javascript href":   "[click](javascript:alert(1))",
		"data href":         "[d](data:text/html;base64,xxx)",
		"vbscript autolink": "<vbscript:msgbox>",
	}
	for name, src := range vectors {
		out := renderHTML(t, src)
		h := out.HTML
		for bad := range map[string]bool{
			"<script>": true, "onerror=\"": true,
			`href="javascript:`: true, `href="data:`: true, "vbscript:": true,
		} {
			if strings.Contains(h, bad) {
				t.Errorf("[%s] 未中和 %q:\n%s", name, bad, h)
			}
		}
	}
	// javascript 链接降级为纯文本但保留可读信息
	out := renderHTML(t, "[click me](javascript:alert(1))")
	if !strings.Contains(out.HTML, "click me") || strings.Contains(out.HTML, "<a ") {
		t.Errorf("危险链接应降级为纯文本:\n%s", out.HTML)
	}
	// 安全链接放行
	out = renderHTML(t, "[ok](https://example.com) [rel](docs/x.md)")
	if !containsAll(out.HTML, `href="https://example.com"`, `href="docs/x.md"`) {
		t.Errorf("安全链接被误杀:\n%s", out.HTML)
	}
}
