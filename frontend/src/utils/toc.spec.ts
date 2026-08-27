// TOC 目录树构建算法单测：嵌套、平铺、跳级、h4/h5/h6 深嵌套。
import { describe, expect, it } from 'vitest'
import { buildTocTree } from './toc'

describe('buildTocTree', () => {
  it('h1>h2>h3 逐级嵌套', () => {
    const tree = buildTocTree([
      { level: 1, text: 'A', id: 'a' },
      { level: 2, text: 'B', id: 'b' },
      { level: 3, text: 'C', id: 'c' },
    ])
    expect(tree.length).toBe(1)
    expect(tree[0].text).toBe('A')
    expect(tree[0].children.map((n) => n.text)).toEqual(['B'])
    expect(tree[0].children[0].children.map((n) => n.text)).toEqual(['C'])
  })

  it('同级标题平铺', () => {
    const tree = buildTocTree([
      { level: 1, text: 'A', id: 'a' },
      { level: 1, text: 'B', id: 'b' },
    ])
    expect(tree.length).toBe(2)
    expect(tree[0].children).toEqual([])
    expect(tree[1].children).toEqual([])
  })

  it('跳级 h2 直接到 h4 → h4 挂在最近 h2 下', () => {
    const tree = buildTocTree([
      { level: 2, text: 'A', id: 'a' },
      { level: 4, text: 'D', id: 'd' },
    ])
    expect(tree.length).toBe(1)
    expect(tree[0].children.length).toBe(1)
    expect(tree[0].children[0].level).toBe(4)
  })

  it('h4/h5/h6 逐级嵌套且不被拍平', () => {
    const tree = buildTocTree([
      { level: 1, text: 'A', id: 'a' },
      { level: 4, text: 'D', id: 'd' },
      { level: 5, text: 'E', id: 'e' },
      { level: 6, text: 'F', id: 'f' },
    ])
    const d = tree[0].children[0]
    expect(d.level).toBe(4)
    const e = d.children[0]
    expect(e.level).toBe(5)
    const f = e.children[0]
    expect(f.level).toBe(6)
    expect(f.children).toEqual([])
  })

  it('空列表返回空树', () => {
    expect(buildTocTree([])).toEqual([])
  })
})
