<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">折扣策略</h2>
          <p class="page-header__desc">绑定到用户组/代理后生效；基础价来自商品计费模板，本页只配置「按客户打几折」。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadPolicies">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建策略
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
          <t-input v-model="filters.keyword" placeholder="策略名称 / 编码" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">作用域</span>
          <t-select v-model="filters.scope" clearable placeholder="全部作用域" :options="scopeFilterOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusFilterOptions" />
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
        <h3 class="card-title">策略列表</h3>
        <span class="table-card__meta">共 {{ total }} 条策略</span>
      </div>
      <t-table
        row-key="id"
        :data="policies"
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
          <div class="policy-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="policy-sub">编码：{{ row.code }}</span>
          </div>
        </template>

        <template #discount="{ row }">
          <span class="discount-cell">{{ discountLabel(row) }}</span>
        </template>

        <template #scope="{ row }">
          <span>{{ scopeLabel(row.scope) }}</span>
          <span v-if="row.items?.length" class="cell-muted">（{{ row.items.length }} 条覆盖）</span>
        </template>

        <template #effective="{ row }">
          <span class="cell-muted">{{ effectiveLabel(row) }}</span>
        </template>

        <template #priority="{ row }">
          <span class="cell-muted">{{ row.priority }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </t-tag>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无折扣策略" />
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
      v-model:visible="dialogVisible"
      :header="editingId ? '编辑折扣策略' : '新建折扣策略'"
      width="720px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="策略名称" name="name">
            <t-input v-model="form.name" placeholder="如：金牌代理价" />
          </t-form-item>
          <t-form-item label="策略编码" name="code">
            <t-input v-model="form.code" placeholder="唯一编码，如 gold_agent" />
          </t-form-item>
          <t-form-item label="折扣类型" name="discount_type">
            <t-radio-group v-model="form.discount_type" variant="default-filled">
              <t-radio-button value="rate">折扣率</t-radio-button>
              <t-radio-button value="amount">直减</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item :label="form.discount_type === 'rate' ? '折扣率（0.85 = 85 折）' : '直减金额（元）'" name="discount_value">
            <t-input-number v-model="form.discount_value" :min="0" :precision="4" theme="column" />
          </t-form-item>
          <t-form-item label="作用域" name="scope">
            <t-select v-model="form.scope" :options="scopeOptions" />
          </t-form-item>
          <t-form-item label="优先级" name="priority">
            <t-input-number v-model="form.priority" :min="0" theme="column" placeholder="数值越大越优先" />
          </t-form-item>
          <t-form-item label="生效开始" name="effective_from">
            <t-date-picker
              v-model="form.effective_from"
              enable-time-picker
              value-type="YYYY-MM-DD HH:mm:ss"
              placeholder="留空表示不限"
              clearable
            />
          </t-form-item>
          <t-form-item label="生效结束" name="effective_to">
            <t-date-picker
              v-model="form.effective_to"
              enable-time-picker
              value-type="YYYY-MM-DD HH:mm:ss"
              placeholder="留空表示不限"
              clearable
            />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="form.status" variant="default-filled">
              <t-radio-button value="active">启用</t-radio-button>
              <t-radio-button value="disabled">停用</t-radio-button>
            </t-radio-group>
          </t-form-item>
        </div>

        <t-form-item label="备注" name="remark">
          <t-textarea v-model="form.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，说明该策略的适用对象" />
        </t-form-item>

        <t-form-item v-if="form.scope !== 'all'" label="条目覆盖（针对具体分类/商品的折扣，优先于策略级折扣）">
          <div class="items-editor">
            <div v-for="(item, index) in form.items" :key="index" class="items-editor__row">
              <t-select v-model="item.target_type" :options="targetTypeOptions" class="items-editor__type" />
              <t-select
                v-model="item.target_id"
                :options="item.target_type === 'category' ? categoryOptions : productOptions"
                filterable
                :placeholder="item.target_type === 'category' ? '选择分类' : '选择商品'"
                class="items-editor__target"
              />
              <t-select v-model="item.discount_type" :options="discountTypeOptions" class="items-editor__dtype" />
              <t-input-number v-model="item.discount_value" :min="0" :precision="4" theme="column" class="items-editor__value" />
              <t-link theme="danger" hover="color" @click="removeItem(index)">移除</t-link>
            </div>
            <t-button variant="dashed" size="small" @click="addItem">添加条目</t-button>
          </div>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createPricePolicy,
  deletePricePolicy,
  getPricePolicyList,
  getProductCategoryList,
  getProductList,
  updatePricePolicy,
} from '@/api/product'
import type {
  PricePolicyItemRequest,
  PricePolicyInfo,
  PricePolicyRequest,
  SaleProductCategoryInfo,
} from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ProductPricingPolicies' })

interface PolicyForm {
  name: string
  code: string
  discount_type: string
  discount_value: number
  scope: string
  priority: number
  effective_from: string
  effective_to: string
  status: string
  remark: string
  items: PricePolicyItemRequest[]
}

const policies = ref<PricePolicyInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const saving = ref(false)
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const filters = reactive<{ keyword: string | undefined; scope: string | undefined; status: string | undefined }>({
  keyword: undefined,
  scope: undefined,
  status: undefined,
})

const scopeFilterOptions = [
  { label: '全站', value: 'all' },
  { label: '指定分类', value: 'category' },
  { label: '指定商品', value: 'product' },
]
const statusFilterOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]
const scopeOptions = scopeFilterOptions
const targetTypeOptions = [
  { label: '分类', value: 'category' },
  { label: '商品', value: 'product' },
]
const discountTypeOptions = [
  { label: '折扣率', value: 'rate' },
  { label: '直减', value: 'amount' },
]

const columns: PrimaryTableCol<PricePolicyInfo>[] = [
  { colKey: 'name', title: '策略', minWidth: 180 },
  { colKey: 'discount', title: '默认折扣', width: 140 },
  { colKey: 'scope', title: '作用域', width: 160 },
  { colKey: 'effective', title: '生效时间', width: 200 },
  { colKey: 'priority', title: '优先级', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'action', title: '操作', width: 130, fixed: 'right' as const, align: 'center' as const },
]

const categoryOptions = ref<{ label: string; value: number }[]>([])
const productOptions = ref<{ label: string; value: number }[]>([])

function discountLabel(row: PricePolicyInfo): string {
  return row.discount_type === 'rate' ? `${row.discount_value} 折率` : `减 ¥${row.discount_value}`
}
function scopeLabel(scope: string): string {
  return { all: '全站', category: '指定分类', product: '指定商品' }[scope] || scope
}
function effectiveLabel(row: PricePolicyInfo): string {
  if (!row.effective_from && !row.effective_to) return '不限'
  return `${row.effective_from || '不限'} ~ ${row.effective_to || '不限'}`
}

async function loadOptions() {
  try {
    const data = await getProductCategoryList()
    const options: { label: string; value: number }[] = []
    const flatten = (nodes: SaleProductCategoryInfo[]) => {
      for (const node of nodes) {
        options.push({ label: node.name, value: node.id })
        if (node.children?.length) flatten(node.children)
      }
    }
    flatten(data.items)
    categoryOptions.value = options
  } catch {
    /* 分类加载失败不阻塞策略列表 */
  }
  try {
    const data = await getProductList({ page: 1, page_size: 200 })
    productOptions.value = data.items.map((item) => ({ label: item.name, value: item.id }))
  } catch {
    /* 商品加载失败不阻塞策略列表 */
  }
}

async function loadPolicies() {
  loading.value = true
  try {
    const data = await getPricePolicyList({
      keyword: filters.keyword,
      scope: filters.scope,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    policies.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载折扣策略失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadPolicies()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

function handleSearch() {
  pagination.current = 1
  loadPolicies()
}
function handleResetFilters() {
  filters.keyword = undefined
  filters.scope = undefined
  filters.status = undefined
  pagination.current = 1
  loadPolicies()
}

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<PolicyForm>(emptyForm())

function emptyForm(): PolicyForm {
  return {
    name: '',
    code: '',
    discount_type: 'rate',
    discount_value: 1,
    scope: 'all',
    priority: 0,
    effective_from: '',
    effective_to: '',
    status: 'active',
    remark: '',
    items: [],
  }
}

function openCreate() {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function openEdit(row: PricePolicyInfo) {
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    code: row.code,
    discount_type: row.discount_type,
    discount_value: row.discount_value,
    scope: row.scope || 'all',
    priority: row.priority ?? 0,
    effective_from: row.effective_from || '',
    effective_to: row.effective_to || '',
    status: row.status || 'active',
    remark: row.remark || '',
    items: (row.items || []).map((item) => ({
      target_type: item.target_type,
      target_id: item.target_id,
      discount_type: item.discount_type,
      discount_value: item.discount_value,
    })),
  })
  dialogVisible.value = true
}

function addItem() {
  form.items.push({ target_type: form.scope === 'product' ? 'product' : 'category', target_id: 0, discount_type: 'rate', discount_value: 1 })
}
function removeItem(index: number) {
  form.items.splice(index, 1)
}

async function handleSave() {
  if (!form.name || !form.code) {
    MessagePlugin.warning('请填写策略名称与编码')
    return
  }
  if (form.scope !== 'all' && form.items.some((item) => !item.target_id)) {
    MessagePlugin.warning('请为每条条目选择目标分类或商品')
    return
  }
  const payload: PricePolicyRequest = {
    name: form.name,
    code: form.code,
    discount_type: form.discount_type,
    discount_value: form.discount_value,
    scope: form.scope,
    priority: form.priority,
    effective_from: form.effective_from || undefined,
    effective_to: form.effective_to || undefined,
    status: form.status,
    remark: form.remark || undefined,
    items: form.scope === 'all' ? [] : form.items,
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updatePricePolicy(editingId.value, payload)
      MessagePlugin.success('策略已更新')
    } else {
      await createPricePolicy(payload)
      MessagePlugin.success('策略已创建')
    }
    dialogVisible.value = false
    loadPolicies()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleDelete(row: PricePolicyInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除折扣策略',
    body: `确定删除「${row.name}」？已绑定该策略的用户组/代理将恢复原价。`,
    theme: 'warning',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deletePricePolicy(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadPolicies()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

onMounted(() => {
  loadOptions()
  loadPolicies()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: PricePolicyInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
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

<style lang="css" scoped>
.policy-cell {
  display: flex;
  flex-direction: column;
}
.policy-sub {
  font-size: 12px;
  color: var(--td-text-color-placeholder, #999);
}
.discount-cell {
  font-weight: 600;
  color: var(--color-primary);
  font-variant-numeric: tabular-nums;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}
.items-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.items-editor__row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.items-editor__type {
  width: 90px;
}
.items-editor__target {
  flex: 1;
}
.items-editor__dtype {
  width: 100px;
}
.items-editor__value {
  width: 120px;
}
</style>
