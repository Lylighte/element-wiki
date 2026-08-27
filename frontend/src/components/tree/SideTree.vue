<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import treeStore from '@/stores/tree'
import treeMenu from '@/stores/treeMenu'
import { docApi } from '@/api'
import { can, CODES } from '@/permissions'

const props = defineProps<{ activeId?: string }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const activeId = computed(() => {
  const id = route.params.id
  return typeof id === 'string' ? id : props.activeId
})

onMounted(() => treeStore.load())

function open(id: string) {
  emit('select', id)
  router.push(`/docs/${id}`)
}

const menu = computed(() => treeMenu.state)
const showMenu = computed(() => !!menu.value.node)
const canUpdate = computed(() => can(CODES.document_update))
const canCreate = computed(() => can(CODES.document_create))
const canDelete = computed(() => can(CODES.document_delete))

async function menuTrash() {
  const node = menu.value.node
  if (!node) return
  try {
    await docApi.remove(node.id)
  } catch {
    ElMessage.error(t('tree.moveFailed'))
    treeMenu.close()
    return
  }
  treeMenu.close()
  await treeStore.load(true)
  if (route.params.id === node.id) router.push('/')
}

function menuRename() {
  if (menu.value.node) treeMenu.startRename(menu.value.node.id)
}

function menuCreateChild() {
  const node = menu.value.node
  if (!node) return
  treeMenu.requestCreateChild(node.id)
}

// 冒泡阶段监听：菜单项自身的 click 处理器先行执行，随后任意点击关闭菜单
document.addEventListener('click', () => treeMenu.close())
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
    <div
      v-if="showMenu"
      class="fixed z-50 bg-white border rounded shadow py-1 text-sm w-40"
      :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      data-test="tree-menu"
    >
      <button
        v-if="canUpdate"
        class="block w-full text-left px-3 py-1.5 hover:bg-gray-100"
        data-test="menu-rename"
        @click="menuRename"
      >
        {{ t('tree.rename') }}
      </button>
      <button
        v-if="canCreate"
        class="block w-full text-left px-3 py-1.5 hover:bg-gray-100"
        data-test="menu-new-child"
        @click="menuCreateChild"
      >
        {{ t('tree.newChild') }}
      </button>
      <button
        v-if="canDelete"
        class="block w-full text-left px-3 py-1.5 hover:bg-gray-100 text-red-600"
        data-test="menu-trash"
        @click="menuTrash"
      >
        {{ t('tree.toTrash') }}
      </button>
    </div>
  </aside>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
