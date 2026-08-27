// TOC 目录树：把渲染接口返回的扁平标题列表构建成嵌套大纲（支持 h1~h6 与跳级）。

export interface TocNode {
  level: number
  text: string
  id: string
  children: TocNode[]
}

export interface TocItem {
  level: number
  text: string
  id: string
}

// heading outline 算法：一个标题挂在它前面最近的、级别更浅的标题下；
// 级别相同或更深的用栈回溯，天然处理 h2→h4 跳级。
export function buildTocTree(items: TocItem[]): TocNode[] {
  const root: TocNode[] = []
  const stack: TocNode[] = []
  for (const it of items) {
    const node: TocNode = { level: it.level, text: it.text, id: it.id, children: [] }
    while (stack.length && stack[stack.length - 1].level >= node.level) stack.pop()
    if (stack.length) stack[stack.length - 1].children.push(node)
    else root.push(node)
    stack.push(node)
  }
  return root
}
