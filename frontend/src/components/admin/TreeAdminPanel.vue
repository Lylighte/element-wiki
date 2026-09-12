<script setup lang="ts">
// M16/T16.2 后台「文档树」面板：结构编辑唯一入口（拖拽移动/排序、重命名、
// 新建子文档、移入回收站）；浏览侧栏不承载任何编辑能力（T16.3）。
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { docApi, type TreeNode } from '@/api'
import treeStore from '@/stores/tree'
import { findNode } from '@/composables/treeDnd'
import TreeAdminItem from './TreeAdminItem.vue'

const { t } = useI18n()

onMounted(() => treeStore.load())

// 新建子文档对话框（slug 可选，T14.4：留空由后端生成并回显）
const createOpen = ref(false)
const createParentId = ref('')
const form = reactive({ slug: '', title: '' })
const creating = ref(false)

function openCreate(parentId: string) {
  createParentId.value = parentId
  form.slug = ''
  form.title = ''
  createOpen.value = true
}

async function submitCreate() {
  if (!form.title.trim()) return
  creating.value = true
  try {
    await docApi.create({
      parent_id: createParentId.value,
      slug: form.slug || undefined,
      title: form.title.trim(),
    })
    createOpen.value = false
    await treeStore.load(true)
  } catch {
    ElMessage.error(t('tree.moveFailed'))
  } finally {
    creating.value = false
  }
}

// —— 移动到…对话框（T16.4）：跨父移动，候选父级排除自身及子孙（防环）——
interface ParentOpt { id: string; label: string }
function flattenParents(nodes: TreeNode[], excludeId: string, prefix = ''): ParentOpt[] {
  return nodes.flatMap((n) => {
    if (n.id === excludeId) return [] // 自身及其整个子树不可作为父级
    return [
      { id: n.id, label: prefix + n.title },
      ...flattenParents(n.children, excludeId, prefix + n.title + ' / '),
    ]
  })
}

const moveOpen = ref(false)
const moveId = ref('')
const moveParentId = ref('')
const moving = ref(false)
const moveOptions = computed(() => flattenParents(treeStore.state.nodes, moveId.value))

function openMove(nodeId: string) {
  const node = findNode(treeStore.state.nodes, nodeId)
  if (!node) return
  moveId.value = nodeId
  moveParentId.value = node.parent_id ?? ''
  moveOpen.value = true
}

async function submitMove() {
  const id = moveId.value
  const node = findNode(treeStore.state.nodes, id)
  if (!node) {
    moveOpen.value = false
    return
  }
  const target = moveParentId.value || null
  if ((node.parent_id ?? null) === target) {
    moveOpen.value = false // 父级未变化：无操作
    return
  }
  moving.value = true
  try {
    let ok: boolean
    if (target === null) {
      // 移回根级：以首个非自身根节点为锚，插到其之前（planMove 生成完整根层顺序）
      const anchor = treeStore.state.nodes.find((r) => r.id !== id)
      ok = anchor ? await treeStore.moveNode(id, anchor.id, 'before') : true
    } else {
      // 移入目标节点子级末尾（planMove 追加并生成完整兄弟列表）
      ok = await treeStore.moveNode(id, target, 'inside')
    }
    if (!ok) ElMessage.error(t('tree.moveFailed'))
    moveOpen.value = false
  } finally {
    moving.value = false
  }
}
</script>

<template>
  <div data-test="admin-tree">
    <p class="text-xs text-[var(--color-text)] mb-2" data-test="admin-tree-hint">{{ t('tree.dndHint') }}</p>
    <div class="max-w-xl">
      <TreeAdminItem
        v-for="n in treeStore.state.nodes"
        :key="n.id"
        :node="n"
        @create="openCreate"
        @move="openMove"
      />
    </div>

    <el-dialog v-model="createOpen" :title="t('doc.create')" width="420px">
      <form class="space-y-3" @submit.prevent="submitCreate">
        <input v-model="form.slug" placeholder="slug (可选，留空自动生成)" data-test="admin-tree-create-slug" class="w-full border rounded px-2 py-1" />
        <input v-model="form.title" :placeholder="t('doc.titlePlaceholder')" data-test="admin-tree-create-title" class="w-full border rounded px-2 py-1" />
      </form>
      <template #footer>
        <button class="px-3 py-1 rounded border" @click="createOpen = false">{{ t('common.cancel') }}</button>
        <button class="px-3 py-1 bg-blue-600 text-white rounded ml-2" :disabled="creating" data-test="admin-tree-create-submit" @click="submitCreate">
          {{ t('doc.createAndEdit') }}
        </button>
      </template>
    </el-dialog>

    <el-dialog v-model="moveOpen" :title="t('tree.moveTo')" width="420px">
      <form class="space-y-3" @submit.prevent="submitMove">
        <select v-model="moveParentId" data-test="admin-tree-move-parent" class="w-full border rounded px-2 py-1">
          <option value="">/（{{ t('tree.root') }}）</option>
          <option v-for="o in moveOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
        </select>
      </form>
      <template #footer>
        <button class="px-3 py-1 rounded border" @click="moveOpen = false">{{ t('common.cancel') }}</button>
        <button class="px-3 py-1 bg-blue-600 text-white rounded ml-2" :disabled="moving" data-test="admin-tree-move-submit" @click="submitMove">
          {{ t('tree.moveTo') }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>
