<script setup lang="ts">
// TOC 递归渲染：每级嵌套 ul 提供缩进，按 level 分级字重/颜色。
import type { TocNode } from '@/utils/toc'

defineOptions({ name: 'TocTree' })

defineProps<{ nodes: TocNode[]; activeId?: string }>()
defineEmits<{ (e: 'jump', id: string): void }>()

function linkClass(level: number): string {
  const base = 'hover:text-[var(--color-primary)] hover:underline truncate block rounded px-1 py-0.5'
  if (level <= 1) return `font-medium text-[var(--color-heading)] ${base}`
  if (level === 2) return `text-[var(--color-heading)] ${base}`
  return `text-[var(--color-text)] ${base}`
}
</script>

<template>
  <ul v-if="nodes.length" class="space-y-0.5" data-test="toc-list">
    <li v-for="n in nodes" :key="n.id + n.text" data-test="toc-item">
      <a
        href="#"
        :class="linkClass(n.level)"
        :aria-current="n.id === activeId ? 'location' : undefined"
        :style="n.id === activeId ? { color: 'var(--color-primary)', background: 'var(--color-accent)' } : undefined"
        data-test="toc-link"
        @click.prevent="$emit('jump', n.id)"
      >{{ n.text }}</a>
      <TocTree
        v-if="n.children.length"
        :nodes="n.children"
        :active-id="activeId"
        class="pl-2 border-l border-[var(--color-border)]"
        @jump="$emit('jump', $event)"
      />
    </li>
  </ul>
</template>
