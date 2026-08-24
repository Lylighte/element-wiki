<script setup lang="ts">
import { onMounted } from 'vue'
import treeStore from '@/stores/tree'

const props = defineProps<{ activeId?: string }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()

onMounted(() => treeStore.load())
</script>

<template>
  <aside class="w-60 border-r bg-white overflow-auto" data-test="side-tree">
    <TreeItem
      v-for="n in treeStore.state.nodes"
      :key="n.id"
      :node="n"
      :active-id="props.activeId"
      @select="(id: string) => emit('select', id)"
    />
  </aside>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
