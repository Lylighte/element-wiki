<script setup lang="ts">
// 侧栏文档树（T16.3 净化）：仅导航（点击跳转）+ 当前高亮；结构编辑全部在管理后台。
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import treeStore from '@/stores/tree'
import siteStore from '@/stores/site'
import { findNodeByPath } from '@/composables/treeDnd'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ activeId?: string }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

// 05 计划提交 4：路由参数为 slug 路径；activeId 由路径在可见树内反解 id。
const activeId = computed(() => {
  const pm = route.params.pathMatch
  const path = Array.isArray(pm) ? pm.join('/') : ((pm as string) ?? '')
  if (path) return findNodeByPath(treeStore.state.nodes, path)?.id ?? ''
  return props.activeId ?? ''
})

onMounted(() => {
  // 匿名关闭时 tree 请求 401：静默降级为空树（侧栏仅导航，无错误 UI）
  void treeStore.load().catch(() => {})
})

function open(id: string) {
  emit('select', id)
  const path = treeStore.pathSlugOf(treeStore.state.nodes, id)
  router.push(`/docs/${path}`)
}
</script>

<template>
  <aside class="side-tree w-full md:w-60 h-full min-h-0 shrink-0 border-r border-[var(--color-border)] bg-[var(--color-card-background)] flex flex-col transition-colors" data-test="side-tree">
    <nav class="min-h-0 flex-1 overflow-y-auto px-2 py-3" :aria-label="t('nav.tree')">
      <TreeItem
        v-for="n in treeStore.state.nodes"
        :key="n.id"
        :node="n"
        :active-id="activeId"
        @select="open"
      />
    </nav>
    <footer v-if="siteStore.state.sidebarFooterHTML" class="side-tree-footer shrink-0 border-t border-[var(--color-border)] px-3 py-3" data-test="site-sidebar-footer">
      <div class="site-footer-markdown prose prose-sm max-h-[35vh] max-w-none overflow-y-auto" data-test="site-sidebar-markdown" v-html="siteStore.state.sidebarFooterHTML" />
    </footer>
  </aside>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
