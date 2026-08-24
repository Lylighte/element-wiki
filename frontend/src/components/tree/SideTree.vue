<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import treeStore from '@/stores/tree'

const props = defineProps<{ activeId?: string }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()
const router = useRouter()

onMounted(() => treeStore.load())

function open(id: string) {
  emit('select', id)
  router.push(`/docs/${id}`)
}
</script>

<template>
  <aside class="w-60 border-r bg-white overflow-auto" data-test="side-tree">
    <TreeItem
      v-for="n in treeStore.state.nodes"
      :key="n.id"
      :node="n"
      :active-id="props.activeId"
      @select="open"
    />
  </aside>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
