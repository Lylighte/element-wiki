// T7.4 验收：面包屑链（根→自身）与 restricted 透传无关。
import { describe, expect, it } from 'vitest'
import type { TreeNode } from '@/api'
import { crumbsFor } from './breadcrumbs'

const tree: TreeNode[] = [
  {
    id: 'r', parent_id: null, title: 'Root', slug: 'root', sort_key: 1,
    restricted: false, children: [
      {
        id: 'm', parent_id: 'r', title: 'Mid', slug: 'mid', sort_key: 1,
        restricted: true, children: [
          { id: 'leaf', parent_id: 'm', title: 'Leaf', slug: 'leaf', sort_key: 1, restricted: false, children: [] },
        ],
      },
    ],
  },
]

describe('crumbsFor', () => {
  it('返回根→自身完整链', () => {
    expect(crumbsFor(tree, 'leaf').map((c) => c.title)).toEqual(['Root', 'Mid', 'Leaf'])
  })
  it('未知 id 返回空链', () => {
    expect(crumbsFor(tree, 'ghost')).toEqual([])
  })
})
