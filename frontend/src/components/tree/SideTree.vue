<script setup lang="ts">
// 侧栏文档树（T16.3 净化）：仅导航（点击跳转）+ 当前高亮；结构编辑全部在管理后台。
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import treeStore from '@/stores/tree'
import { findNodeByPath } from '@/composables/treeDnd'

const props = defineProps<{ activeId?: string }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()
const router = useRouter()
const route = useRoute()

// 05 计划提交 4：路由参数为 slug 路径；activeId 由路径在可见树内反解 id。
const activeId = computed(() => {
  const pm = route.params.pathMatch
  const path = Array.isArray(pm) ? pm.join('/') : ((pm as string) ?? '')
  if (path) return findNodeByPath(treeStore.state.nodes, path)?.id ?? ''
  return props.activeId ?? ''
})

onMounted(() => treeStore.load())

function open(id: string) {
  emit('select', id)
  const path = treeStore.pathSlugOf(treeStore.state.nodes, id)
  router.push(`/docs/${path}`)
}
</script>

<template>
  <aside class="w-60 border-r bg-white overflow-auto relative" data-test="side-tree">
    <TreeItem
      v-for="n in treeStore.state.nodes"
      :key="n.id"
      :node="n"
       :active-id="activeId"
      @select="open"
    />
  </aside>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
