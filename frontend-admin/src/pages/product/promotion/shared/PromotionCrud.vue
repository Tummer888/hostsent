<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ labels.pageTitle }}</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          {{ labels.createText }}
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" :placeholder="labels.namePlaceholder" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">{{ labels.tableTitle }}</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 个{{ labels.unit }}</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">#{{ row.id }} · 排序 {{ row.sort_order }}</span>
          </div>
        </template>

        <template #rule="{ row }">
          <span class="cell-muted">{{ row.rule || '—' }}</span>
        </template>

        <template #time="{ row }">
          <div class="cell-muted">{{ formatTime(row.start_time) }}<br />至 {{ formatTime(row.end_time) }}</div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '停用', value: 'toggle', hidden: () => row.status !== 1, theme: 'warning' },
                { content: '启用', value: 'toggle', hidden: () => row.status === 1, theme: 'success' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link
                v-if="row.status === 1"
                theme="warning"
                hover="color"
                @click="handleToggleStatus(row)"
              >停用</t-link>
              <t-link
                v-else
                theme="primary"
                hover="color"
                @click="handleToggleStatus(row)"
              >启用</t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty :description="labels.emptyText" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <t-dialog
      v-model:visible="formVisible"
      :header="editing ? `编辑${labels.unit}` : labels.createText"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item :label="labels.nameLabel" name="name" :rules="[{ required: true, message: `请填写${labels.nameLabel}` }]">
          <t-input v-model="form.name" :placeholder="labels.namePlaceholder" />
        </t-form-item>
        <t-form-item :label="labels.ruleLabel" name="rule">
          <t-textarea
            v-if="labels.ruleEditor === 'textarea'"
            v-model="form.rule"
            :autosize="{ minRows: 3, maxRows: 6 }"
            :placeholder="labels.rulePlaceholder"
          />
          <t-input v-else v-model="form.rule" :placeholder="labels.rulePlaceholder" />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="开始时间" name="start_time">
            <t-date-picker v-model="form.start_time" placeholder="开始时间" clearable />
          </t-form-item>
          <t-form-item label="结束时间" name="end_time">
            <t-date-picker v-model="form.end_time" placeholder="结束时间" clearable />
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="排序权重" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { createPromotion, deletePromotion, getPromotionList, updatePromotion } from '@/api/product'
import { formatTime } from '@/pages/product/constants'
import type { PromotionInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

import type { PromotionLabels } from './types'

/**
 * 促销活动通用 CRUD：「折扣活动」(promotion_type=discount) 与「套餐组合」(bundle)
 * 此前是两个逐行重复的 406 行页面，差异仅在文案与规则控件，收敛为一个参数化组件。
 */
const props = defineProps<{
  promotionType: 'discount' | 'bundle'
  labels: PromotionLabels
}>()

const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const loading = ref(false)
const saving = ref(false)
const { isMobile } = useIsMobile()
const list = ref<PromotionInfo[]>([])

const filters = reactive<{ keyword: string | undefined; status: number | undefined }>({
  keyword: undefined,
  status: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadData)

const columns: PrimaryTableCol<PromotionInfo>[] = [
  { colKey: 'name', title: props.labels.unit, minWidth: 180 },
  { colKey: 'rule', title: props.labels.ruleLabel, minWidth: 200 },
  { colKey: 'time', title: '有效期', width: 190 },
  { colKey: 'sort_order', title: '排序', width: 70 },
  { colKey: 'status', title: '状态', width: 80 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 170,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function statusTag(status: number): { theme: 'success' | 'danger' | 'default'; text: string } {
  return status === 1 ? { theme: 'success', text: '启用' } : { theme: 'default', text: '停用' }
}

async function loadData() {
  loading.value = true
  try {
    const data = await getPromotionList({
      promotion_type: props.promotionType,
      keyword: filters.keyword,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || `加载${props.labels.unit}失败`)
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  resetPage()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.status = undefined
  resetPage()
}

// ---- 新建 / 编辑 ----
const formVisible = ref(false)
const editing = ref(false)
let editingId = 0
const form = reactive<Record<string, any>>({
  name: '',
  rule: '',
  start_time: undefined,
  end_time: undefined,
  sort_order: 0,
})

function openCreate() {
  editing.value = false
  editingId = 0
  Object.assign(form, { name: '', rule: '', start_time: undefined, end_time: undefined, sort_order: 0 })
  formVisible.value = true
}

function openEdit(row: PromotionInfo) {
  editing.value = true
  editingId = row.id
  Object.assign(form, {
    name: row.name,
    rule: row.rule,
    start_time: row.start_time,
    end_time: row.end_time,
    sort_order: row.sort_order,
  })
  formVisible.value = true
}

async function handleSave() {
  if (!form.name) {
    MessagePlugin.warning(`请填写${props.labels.nameLabel}`)
    return
  }
  const payload = {
    name: form.name,
    promotion_type: props.promotionType,
    rule: form.rule || undefined,
    start_time: form.start_time || undefined,
    end_time: form.end_time || undefined,
    sort_order: form.sort_order ?? undefined,
    // 新建默认「启用」：列表只有启用/停用两个状态，若落成 0（草稿）则新建后立即显示为停用，
    // 与优惠券、规格模板的默认启用口径一致。编辑时不改状态（沿用停用/启用操作单独切换）。
    ...(editing.value ? {} : { status: 1 }),
  }
  saving.value = true
  try {
    if (editing.value) {
      await updatePromotion(editingId, payload)
    } else {
      await createPromotion(payload)
    }
    MessagePlugin.success('已保存')
    formVisible.value = false
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleToggleStatus(row: PromotionInfo) {
  try {
    await updatePromotion(row.id, {
      name: row.name,
      promotion_type: props.promotionType,
      rule: row.rule || undefined,
      status: row.status === 1 ? 0 : 1,
    })
    MessagePlugin.success('状态已更新')
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败')
  }
}

function handleDelete(row: PromotionInfo) {
  const dialog = DialogPlugin.confirm({
    header: `删除${props.labels.unit}`,
    body: `确认删除「${row.name}」？删除后不可恢复。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deletePromotion(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadData()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

onMounted(() => {
  loadData()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: PromotionInfo) {
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
@import '../../shared.css';
</style>
