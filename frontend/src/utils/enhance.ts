// T9.3：KaTeX/Mermaid 前端增强——依赖按需动态加载，普通文档零额外 chunk。
interface AutoRenderOptions {
  delimiters?: { left: string; right: string; display: boolean }[]
  ignoredTags?: string[]
}
type AutoRender = (elem: HTMLElement, options?: AutoRenderOptions) => void
let cachedAutoRender: AutoRender | null = null

async function ensureAutoRender(): Promise<AutoRender> {
  if (cachedAutoRender) return cachedAutoRender
  await Promise.all([import('katex'), import('katex/dist/katex.min.css')])
  const mod = (await import('katex/contrib/auto-render')) as unknown as { default: AutoRender }
  cachedAutoRender = mod.default
  return mod.default
}

async function renderMermaidBlocks(blocks: Element[]) {
  const mermaid = (await import('mermaid')).default
  mermaid.initialize({ startOnLoad: false })
  const holders: HTMLElement[] = []
  for (const code of blocks) {
    const pre = code.parentElement
    if (!pre) continue
    const div = document.createElement('div')
    div.className = 'mermaid'
    div.textContent = code.textContent ?? ''
    pre.replaceWith(div)
    holders.push(div)
  }
  if (holders.length) await mermaid.run({ nodes: holders })
}

/** 扫描容器：命中公式/mermaid 才加载对应依赖并渲染；否则立即返回。 */
export async function enhanceMarkdownExtras(root: HTMLElement): Promise<void> {
  const hasMath = /\$[^$\n]+\$/.test(root.textContent ?? '')
  const mermaidCodes = Array.from(root.querySelectorAll('pre code.language-mermaid'))
  if (!hasMath && mermaidCodes.length === 0) return

  const tasks: Promise<void>[] = []
  if (hasMath) {
    const render = await ensureAutoRender()
    render(root, {
      delimiters: [
        { left: '$$', right: '$$', display: true },
        { left: '$', right: '$', display: false },
      ],
      ignoredTags: ['script', 'noscript', 'style', 'textarea', 'pre', 'code'],
    })
  }
  if (mermaidCodes.length) tasks.push(renderMermaidBlocks(mermaidCodes))
  await Promise.all(tasks)
}
