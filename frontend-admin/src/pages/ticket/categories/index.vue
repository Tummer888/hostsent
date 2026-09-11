<template>
  <div class="page-body ticket-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <TagIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">工单分类管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadCategories">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新增分类
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">分类列表</h3>
        <span class="table-card__meta">共 {{ categories.length }} 个分类</span>
      </div>
      <t-table
        row-key="id"
        :data="categories"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="null"
      >
        <template #status="{ row }">
          <t-tag :theme="categoryStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ categoryStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '禁用/启用', value: 'toggle', theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无分类数据" />
        </template>
      </t-table>
    </section>

    <!-- 新增/编辑分类对话框 -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editingId ? '编辑分类' : '新增分类'"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="分类名称" name="name">
          <t-input v-model="form.name" placeholder="如：技术支持" :maxlength="64" />
        </t-form-item>
        <t-form-item label="分类编码" name="code">
          <t-input v-model="form.code" placeholder="如：technical（唯一，保存后不可与现有重复）" :maxlength="64" :disabled="!!editingId" />
        </t-form-item>
        <t-form-item label="描述" name="description">
          <t-textarea v-model="form.description" placeholder="分类用途说明" :autosize="{ minRows: 2, maxRows: 4 }" :maxlength="255" />
        </t-form-item>
        <t-form-item label="排序值" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" theme="column" style="width: 160px" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-select v-model="form.status" :options="categoryStatusOptions" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { AddIcon, RefreshIcon, TagIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createTicketCategory, deleteTicketCategory, getTicketCategories, updateTicketCategory } from '@/api/ticket'
import { categoryStatusLabel, categoryStatusOptions, categoryStatusTheme, formatTime } from '@/pages/ticket/constants'
import type { TicketCategoryInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'TicketCategories' })

const categories = ref<TicketCategoryInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const saving = ref(false)
const formVisible = ref(false)
const editingId = ref<number | null>(null)

const form = reactive<{
  name: string
  code: string
  description: string
  sort_order: number
  status: string
}>({
  name: '',
  code: '',
  description: '',
  sort_order: 0,
  status: 'active',
})

const columns: PrimaryTableCol<TicketCategoryInfo>[] = [
  { colKey: 'name', title: '分类名称', minWidth: 140 },
  { colKey: 'code', title: '编码', minWidth: 140 },
  { colKey: 'description', title: '描述', minWidth: 220, ellipsis: true },
  { colKey: 'sort_order', title: '排序', width: 80, align: 'center' as const },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 170, fixed: 'right' as const, align: 'center' as const },
]

async function loadCategories() {
  loading.value = true
  try {
    categories.value = await getTicketCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载分类列表失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.name = ''
  form.code = ''
  form.description = ''
  form.sort_order = 0
  form.status = 'active'
  formVisible.value = true
}

function openEdit(row: TicketCategoryInfo) {
  editingId.value = row.id
  form.name = row.name
  form.code = row.code
  form.description = row.description
  form.sort_order = row.sort_order
  form.status = row.status
  formVisible.value = true
}

async function handleSave() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入分类名称')
    return
  }
  if (!form.code.trim()) {
    MessagePlugin.warning('请输入分类编码')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateTicketCategory(editingId.value, {
        name: form.name.trim(),
        code: form.code.trim(),
        description: form.description,
        sort_order: form.sort_order,
        status: form.status,
      })
      MessagePlugin.success('分类已更新')
    } else {
      await createTicketCategory({
        name: form.name.trim(),
        code: form.code.trim(),
        description: form.description,
        sort_order: form.sort_order,
        status: form.status,
      })
      MessagePlugin.success('分类已创建')
    }
    formVisible.value = false
    loadCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存分类失败')
  } finally {
    saving.value = false
  }
}

// 启用/禁用切换
async function handleToggleStatus(row: TicketCategoryInfo) {
  const target = row.status === 'active' ? 'disabled' : 'active'
  try {
    await updateTicketCategory(row.id, {
      name: row.name,
      code: row.code,
      description: row.description,
      sort_order: row.sort_order,
      status: target,
    })
    MessagePlugin.success(target === 'active' ? '分类已启用' : '分类已禁用')
    loadCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '状态更新失败')
  }
}

function handleDelete(row: TicketCategoryInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除分类',
    body: `确认删除分类「${row.name}」吗？删除后用户端不可再选择该分类。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteTicketCategory(row.id)
        MessagePlugin.success('分类已删除')
        dialog.destroy()
        loadCategories()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadCategories)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: TicketCategoryInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'toggle':
      void handleToggleStatus(row)
      break
    case 'delete':
      void handleDelete(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>
