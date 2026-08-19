<template>
  <div class="menu-page">
    <div class="menu-page__header">
      <div>
        <h2 class="menu-page__title">菜单管理</h2>
        <p class="menu-page__subtitle">
          统一维护管理员后台与用户中心菜单树，支持目录与菜单层级编辑。
        </p>
      </div>
      <div class="menu-page__actions">
        <t-radio-group v-model="platform" variant="default-filled" size="small" @change="loadTree">
          <t-radio-button value="admin">管理员后台</t-radio-button>
          <t-radio-button value="user">用户中心</t-radio-button>
        </t-radio-group>
        <t-button theme="primary" @click="onCreateRoot">
          <template #icon>
            <AddIcon />
          </template>
          新增顶级菜单
        </t-button>
      </div>
    </div>

    <t-table
      row-key="id"
      :data="treeData"
      :columns="columns"
      :loading="loading"
      :tree="{ childrenKey: 'children', treeNodeColumnIndex: 0 }"
      size="small"
      hover
      vertical-align="middle"
    >
      <template #icon="{ row }">
        <span class="menu-icon">{{ row.icon || '—' }}</span>
      </template>
      <template #type="{ row }">
        <t-tag theme="primary" variant="light" size="small" shape="round">
          {{ typeLabel(row.type) }}
        </t-tag>
      </template>
      <template #platform="{ row }">
        <t-tag :theme="row.platform === 'admin' ? 'success' : 'warning'" variant="light" size="small" shape="round">
          {{ row.platform === 'admin' ? '管理员' : '用户中心' }}
        </t-tag>
      </template>
      <template #status="{ row }">
        <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light" size="small" shape="round">
          {{ row.status === 'active' ? '启用' : '禁用' }}
        </t-tag>
      </template>
      <template #operation="{ row }">
        <t-space size="small">
          <t-link theme="primary" size="small" @click="onAddChild(row)">新增子级</t-link>
          <t-link theme="primary" size="small" @click="onEdit(row)">编辑</t-link>
          <t-popconfirm content="确认删除该菜单及其全部子节点？" @confirm="onDelete(row)">
            <t-link theme="danger" size="small">删除</t-link>
          </t-popconfirm>
        </t-space>
      </template>
    </t-table>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="520px"
      :confirm-btn="{ content: '保存', loading: submitting }"
      :on-confirm="onSubmit"
      @close="onDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <t-form-item label="所属平台" name="platform">
          <t-radio-group v-model="formData.platform">
            <t-radio value="admin">管理员后台</t-radio>
            <t-radio value="user">用户中心</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="上级菜单" name="parent_id">
          <t-select
            v-model="formData.parent_id"
            :options="parentOptions"
            :keys="{ label: 'name', value: 'id' }"
            placeholder="顶级菜单（不选）"
            clearable
            filterable
          />
        </t-form-item>
        <t-form-item label="菜单名称" name="name">
          <t-input v-model="formData.name" placeholder="请输入菜单名称" maxlength="64" />
        </t-form-item>
        <t-form-item label="节点类型" name="type">
          <t-radio-group v-model="formData.type">
            <t-radio value="directory">目录（含子级）</t-radio>
            <t-radio value="menu">菜单（可访问页面）</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="formData.type === 'menu'" label="路由路径" name="path">
          <t-input v-model="formData.path" placeholder="如 /system/menus" />
        </t-form-item>
        <t-form-item v-if="formData.type === 'menu'" label="前端组件" name="component">
          <t-input v-model="formData.component" placeholder="如 system/menus/index" />
        </t-form-item>
        <t-form-item label="图标名称" name="icon">
          <t-input v-model="formData.icon" placeholder="tdesign 图标名，如 dashboard" />
        </t-form-item>
        <t-form-item label="排序" name="sort_order">
          <t-input-number v-model="formData.sort_order" :min="0" :max="9999" theme="normal" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="formData.status">
            <t-radio value="active">启用</t-radio>
            <t-radio value="disabled">禁用</t-radio>
          </t-radio-group>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next'

import {
  createMenu,
  deleteMenu,
  getMenuTree,
  updateMenu,
  type MenuNode,
} from '@/api/menu'

defineOptions({ name: 'SystemMenus' })

const loading = ref(false)
const submitting = ref(false)
const platform = ref('admin')
const treeData = ref<MenuNode[]>([])

const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref<FormInstanceFunctions | null>(null)

const initForm = () => ({
  id: 0,
  parent_id: 0,
  platform: platform.value,
  name: '',
  type: 'menu' as 'directory' | 'menu',
  path: '',
  component: '',
  icon: '',
  sort_order: 0,
  status: 'active' as 'active' | 'disabled',
})

const formData = reactive(initForm())

const rules: Record<string, FormRule[]> = {
  platform: [{ required: true, message: '请选择所属平台', type: 'error', trigger: 'change' }],
  name: [{ required: true, message: '请输入菜单名称', type: 'error', trigger: 'blur' }],
  type: [{ required: true, message: '请选择节点类型', type: 'error', trigger: 'change' }],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const columns: PrimaryTableCol<MenuNode>[] = [
  { colKey: 'name', title: '菜单名称', minWidth: 200, ellipsis: true },
  { colKey: 'icon', title: '图标', width: 120 },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'path', title: '路由路径', minWidth: 180, ellipsis: true },
  { colKey: 'platform', title: '平台', width: 110 },
  { colKey: 'sort_order', title: '排序', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'operation', title: '操作', width: 200, fixed: 'right' },
]

const dialogTitle = computed(() => (isEdit.value ? '编辑菜单' : '新增菜单'))

const parentOptions = computed(() => {
  const options: MenuNode[] = []
  const walk = (nodes: MenuNode[]) => {
    for (const node of nodes) {
      if (node.type === 'directory') {
        options.push(node)
      }
      if (node.children?.length) {
        walk(node.children)
      }
    }
  }
  walk(treeData.value)
  return options
})

function typeLabel(type: string) {
  return type === 'directory' ? '目录' : '菜单'
}

function resetForm() {
  Object.assign(formData, initForm())
}

async function loadTree() {
  loading.value = true
  try {
    treeData.value = await getMenuTree(platform.value)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载菜单树失败')
  } finally {
    loading.value = false
  }
}

function openCreate(parentId = 0) {
  isEdit.value = false
  resetForm()
  formData.parent_id = parentId
  formData.platform = platform.value
  dialogVisible.value = true
}

function onCreateRoot() {
  openCreate(0)
}

function onAddChild(row: MenuNode) {
  openCreate(row.id)
}

function onEdit(row: MenuNode) {
  isEdit.value = true
  resetForm()
  Object.assign(formData, {
    id: row.id,
    parent_id: row.parent_id,
    platform: row.platform,
    name: row.name,
    type: row.type,
    path: row.path || '',
    component: row.component || '',
    icon: row.icon || '',
    sort_order: row.sort_order,
    status: row.status,
  })
  dialogVisible.value = true
}

async function onSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) {
    return
  }
  submitting.value = true
  try {
    const payload = {
      parent_id: formData.parent_id || 0,
      platform: formData.platform,
      name: formData.name.trim(),
      type: formData.type,
      path: formData.path,
      component: formData.component,
      icon: formData.icon,
      sort_order: formData.sort_order || 0,
      status: formData.status,
    }
    if (isEdit.value) {
      await updateMenu(formData.id, payload)
      MessagePlugin.success('菜单已更新')
    } else {
      await createMenu(payload)
      MessagePlugin.success('菜单已创建')
    }
    dialogVisible.value = false
    await loadTree()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function onDelete(row: MenuNode) {
  try {
    await deleteMenu(row.id)
    MessagePlugin.success('菜单已删除')
    await loadTree()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除失败')
  }
}

function onDialogClose() {
  resetForm()
  formRef.value?.clearValidate?.()
}

onMounted(loadTree)
</script>

<style scoped lang="css">
.menu-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.menu-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.menu-page__title {
  margin: 0 0 4px;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-foreground);
}

.menu-page__subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.menu-page__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.menu-icon {
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
