// T9.3：enhanceMarkdownExtras 仅在命中公式/mermaid 时动态加载（普通内容零依赖）。
import { describe, expect, it, vi, beforeEach } from 'vitest'

const katexRender = vi.fn()
const mermaidRun = vi.fn().mockResolvedValue(undefined)

vi.mock('katex', () => ({ default: {} }))
vi.mock('katex/dist/katex.min.css', () => ({}))
vi.mock('katex/contrib/auto-render', () => ({ default: katexRender }))
vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    run: mermaidRun,
  },
}))

import { enhanceMarkdownExtras } from './enhance'

function el(html: string): HTMLElement {
  const div = document.createElement('div')
  div.innerHTML = html
  document.body.appendChild(div)
  return div
}

describe('enhanceMarkdownExtras', () => {
  beforeEach(() => {
    katexRender.mockClear()
    mermaidRun.mockClear()
  })

  it('普通文本不触发任何动态加载', async () => {
    await enhanceMarkdownExtras(el('<p>hello world</p>'))
    expect(katexRender).not.toHaveBeenCalled()
    expect(mermaidRun).not.toHaveBeenCalled()
  })

  it('命中行内公式 → 调用 KaTeX auto-render，忽略 pre/code 内部', async () => {
    await enhanceMarkdownExtras(el('<p>能量 $E=mc^2$ 守恒</p><pre><code>$not math$</code></pre>'))
    expect(katexRender).toHaveBeenCalledTimes(1)
    expect(mermaidRun).not.toHaveBeenCalled()
  })

  it('mermaid 代码块被替换为 .mermaid 容器并渲染', async () => {
    const root = el('<pre><code class="language-mermaid">graph TD; A-->B</code></pre>')
    await enhanceMarkdownExtras(root)
    expect(root.querySelector('.mermaid')?.textContent).toContain('graph TD')
    expect(mermaidRun).toHaveBeenCalledTimes(1)
  })
})
