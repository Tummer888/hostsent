<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">折扣活动</h2>
          <p class="page-header__desc">
            面向<strong>普通用户</strong>的限时折扣：到点自动生效、到期自动失效，用户端下单页直接可见。
          </p>
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
          新建活动
        </t-button>
      </t-space>
    </header>

    <!--
      如实告知边界：本页折扣与「代理拿货折扣」是两条互不叠加的线。
      代理身份按 users.agent_level_id 判定，代理账号一律只走代理折扣、不参与本页活动 ——
      否则大促时会出现「代理比普通用户还贵」，价格说不清。
    -->
    <t-alert
      theme="info"
      title="本页折扣面向普通用户"
      message="有代理身份的账号不参与活动折扣，它们在「用户组管理 → ③折扣组 / ④生效矩阵」里按代理分组拿价；两条折扣线互不叠加。"
      class="flash-notice"
    />

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="活动名称 / 编码" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">启停</span>
        <t-select v-model="filters.status" clearable placeholder="全部" :options="statusOptions" />
      </div>
      <div class="field">
        <span class="field__label">时间窗</span>
        <t-select v-model="filters.running" clearable placeholder="全部活动" :options="runningOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">活动列表</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 个活动</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        :pagination="isMobile ? undefined : pagination"
        table-layout="fixed"
        cell-empty-content="—"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="cell-stack">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="cell-sub">{{ row.code }}</span>
          </div>
        </template>

        <template #discount="{ row }">
          <div class="cell-stack">
            <span class="cell-strong">{{ discountLabel(row) }}</span>
            <span class="cell-sub">{{ row.discount_type === 'rate' ? '折扣率' : '直减金额' }}</span>
          </div>
        </template>

        <template #scope="{ row }">
          <div class="scope-cell">
            <t-tag variant="light" size="small" shape="round" :theme="(row.items || []).length ? 'warning' : 'primary'">
              {{ (row.items || []).length ? `${row.items.length} 个目标` : '全站商品' }}
            </t-tag>
            <span v-if="(row.items || []).length" class="cell-sub">{{ itemSummary(row) }}</span>
          </div>
        </template>

        <template #window="{ row }">
          <span class="time-text">{{ windowLabel(row) }}</span>
        </template>

        <template #runtime_status="{ row }">
          <t-tag :theme="runtimeTag(row.runtime_status).theme" variant="light" size="small" shape="round">
            {{ runtimeTag(row.runtime_status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '停用', value: 'toggle', hidden: () => row.status !== 'active', theme: 'warning' },
                { content: '启用', value: 'toggle', hidden: () => row.status === 'active', theme: 'success' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link v-if="row.status === 'active'" theme="warning" hover="color" @click="handleToggleStatus(row)">停用</t-link>
              <t-link v-else theme="primary" hover="color" @click="handleToggleStatus(row)">启用</t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="还没有折扣活动，点右上角「新建活动」" />
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
      :header="editing ? '编辑折扣活动' : '新建折扣活动'"
      width="680px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form ref="formRef" label-align="top" :data="form" :rules="rules">
        <div class="form-grid">
          <t-form-item label="活动名称" name="name">
            <t-input v-model="form.name" placeholder="如：双 11 云主机限时 8 折" maxlength="64" />
          </t-form-item>
          <t-form-item label="活动编码" name="code">
            <t-input v-model="form.code" placeholder="如：double11_vhost" maxlength="64" />
          </t-form-item>
        </div>

        <div class="form-grid">
          <t-form-item label="折扣方式" name="discount_type">
            <t-select v-model="form.discount_type" :options="discountTypeOptions" />
          </t-form-item>
          <t-form-item
            :label="form.discount_type === 'rate' ? '折扣率（0.85 = 八五折）' : '直减金额（元）'"
            name="discount_value"
          >
            <t-input-number
              v-model="form.discount_value"
              :min="0"
              :max="form.discount_type === 'rate' ? 1 : undefined"
              :step="form.discount_type === 'rate' ? 0.01 : 1"
              :precision="2"
              theme="normal"
              placeholder="越小越优惠"
              style="width: 100%"
            />
          </t-form-item>
        </div>

        <t-form-item label="作用范围" name="scope">
          <t-radio-group v-model="form.scope" variant="default-filled">
            <t-radio-button value="all">全站商品</t-radio-button>
            <t-radio-button value="category">按分类</t-radio-button>
            <t-radio-button value="product">按单个商品</t-radio-button>
          </t-radio-group>
          <span class="field-hint">
            「全站商品」= 所有商品打折；按分类/商品则只对下面勾选的目标生效（同一活动里商品条目优先于分类条目）。
          </span>
        </t-form-item>

        <t-form-item label="目标分类" name="categoryIds">
          <t-select
            v-model="form.categoryIds"
            :options="categoryOptions"
            multiple
            filterable
            clearable
            placeholder="可多选分类（仅在需要按分类命中时选）"
          />
        </t-form-item>

        <t-form-item label="目标商品" name="productIds">
          <t-select
            v-model="form.productIds"
            :options="productOptions"
            multiple
            filterable
            clearable
            placeholder="可多选单个商品（未上架的带标注）"
          />
        </t-form-item>

        <div class="form-grid">
          <t-form-item label="生效开始" name="start_at">
            <t-date-picker v-model="form.start_at" enable-time-picker clearable placeholder="留空 = 立即开始" />
          </t-form-item>
          <t-form-item label="生效结束" name="end_at">
            <t-date-picker v-model="form.end_at" enable-time-picker clearable placeholder="留空 = 不限结束" />
          </t-form-item>
        </div>

        <div class="form-grid">
          <t-form-item label="启停" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-input v-model="form.remark" placeholder="选填" maxlength="255" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import {
  DialogPlugin,
  MessagePlugin,
  type FormInstanceFunctions,
  type FormRule,
  type PrimaryTableCol,
} from 'tdesign-vue-next'

import {
  createFlashDiscount,
  deleteFlashDiscount,
  getFlashDiscountList,
  updateFlashDiscount,
  type FlashDiscountInfo,
  type FlashDiscountRequest,
} from '@/api/flash-discount'
import { getProductCategoryList, getProductList } from '@/api/product'
import FilterCard from '@/components/filter-card/index.vue'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

defineOptions({ name: 'ProductPromotionActivities' })

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

const runningOptions = [{ label: '只看进行中', value: 'running' }]

const discountTypeOptions = [
  { label: '折扣率（0.85 = 八五折）', value: 'rate' },
  { label: '直减固定金额', value: 'amount' },
]

const loading = ref(false)
const saving = ref(false)
const { isMobile } = useIsMobile()
const list = ref<FlashDiscountInfo[]>([])

const filters = reactive<{ keyword: string | undefined; status: string | undefined; running: string | undefined }>({
  keyword: undefined,
  status: undefined,
  running: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadData)

const columns: PrimaryTableCol<FlashDiscountInfo>[] = [
  { colKey: 'name', title: '活动', minWidth: 180 },
  { colKey: 'discount', title: '折扣', width: 140 },
  { colKey: 'scope', title: '作用范围', minWidth: 190 },
  { colKey: 'window', title: '生效时间', width: 210 },
  { colKey: 'runtime_status', title: '状态', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 170,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function discountLabel(row: FlashDiscountInfo): string {
  if (row.discount_type === 'rate') {
    const discount = (row.discount_value * 10).toFixed(1).replace(/\.0$/, '')
    return `${row.discount_value.toFixed(2)}（${discount}折）`
  }
  return `直减 ¥${row.discount_value.toFixed(2)}`
}

function itemSummary(row: FlashDiscountInfo): string {
  const names = (row.items || []).map((item) => item.target_name || `#${item.target_id}`)
  if (names.length <= 2) return names.join('、')
  return `${names.slice(0, 2).join('、')} 等 ${names.length} 项`
}

function windowLabel(row: FlashDiscountInfo): string {
  if (!row.start_at && !row.end_at) return '不限'
  return `${row.start_at || '立即'} ~ ${row.end_at || '不限'}`
}

function runtimeTag(status: string): { theme: 'success' | 'warning' | 'default' | 'danger'; text: string } {
  switch (status) {
    case 'running':
      return { theme: 'success', text: '进行中' }
    case 'pending':
      return { theme: 'warning', text: '待开始' }
    case 'expired':
      return { theme: 'default', text: '已结束' }
    default:
      return { theme: 'danger', text: '已停用' }
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await getFlashDiscountList({
      keyword: filters.keyword,
      status: filters.status,
      running: filters.running === 'running',
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items || []
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载折扣活动失败')
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
  filters.running = undefined
  resetPage()
}

// —— 分类 / 商品下拉（与代理折扣页同口径：只列启用分类，商品在售排前）——
const categoryOptions = ref<{ label: string; value: number }[]>([])
const productOptions = ref<{ label: string; value: number }[]>([])

interface CategoryNode {
  id: number
  name: string
  status?: number
  children?: unknown[]
}

async function loadOptions() {
  try {
    const data = await getProductCategoryList()
    const flat: { label: string; value: number }[] = []
    const walk = (items: CategoryNode[], depth: number) => {
      for (const item of items) {
        if (item.status !== undefined && item.status !== 1) continue
        const prefix = depth === 0 ? '' : `${'　'.repeat(depth - 1)}└ `
        flat.push({ label: `${prefix}${item.name}`, value: item.id })
        if (Array.isArray(item.children) && item.children.length) walk(item.children as CategoryNode[], depth + 1)
      }
    }
    walk((data.items || []) as unknown as CategoryNode[], 0)
    categoryOptions.value = flat
  } catch {
    categoryOptions.value = []
  }
  try {
    const data = await getProductList({ page: 1, page_size: 500 } as never)
    const items = (data.items || []) as Array<{ id: number; name: string; status?: number }>
    const onSale: { label: string; value: number }[] = []
    const offShelf: { label: string; value: number }[] = []
    for (const item of items) {
      const entry = { label: item.status === 1 ? item.name : `${item.name}（已下架）`, value: item.id }
      ;(item.status === 1 ? onSale : offShelf).push(entry)
    }
    productOptions.value = [...onSale, ...offShelf]
  } catch {
    productOptions.value = []
  }
}

// —— 新建 / 编辑 ——
const formRef = ref<FormInstanceFunctions>()
const formVisible = ref(false)
const editing = ref(false)
let editingId = 0

interface FormState {
  name: string
  code: string
  discount_type: 'rate' | 'amount'
  discount_value: number
  scope: 'all' | 'category' | 'product'
  categoryIds: number[]
  productIds: number[]
  start_at: string
  end_at: string
  status: string
  remark: string
}

function emptyForm(): FormState {
  return {
    name: '',
    code: '',
    discount_type: 'rate',
    discount_value: 0.9,
    scope: 'all',
    categoryIds: [],
    productIds: [],
    start_at: '',
    end_at: '',
    status: 'active',
    remark: '',
  }
}

const form = reactive<FormState>(emptyForm())

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请填写活动名称', type: 'error' }],
  code: [{ required: true, message: '请填写活动编码', type: 'error' }],
  discount_value: [{ required: true, message: '请填写折扣值', type: 'error' }],
}

function openCreate() {
  editing.value = false
  editingId = 0
  Object.assign(form, emptyForm())
  formVisible.value = true
}

function openEdit(row: FlashDiscountInfo) {
  editing.value = true
  editingId = row.id
  const categoryIds: number[] = []
  const productIds: number[] = []
  for (const item of row.items || []) {
    if (item.target_type === 'category') categoryIds.push(item.target_id)
    else productIds.push(item.target_id)
  }
  Object.assign(form, {
    name: row.name,
    code: row.code,
    discount_type: row.discount_type,
    discount_value: row.discount_value,
    scope: row.scope,
    categoryIds,
    productIds,
    // 后端下发 "2006-01-02 15:04:05"，日期选择器要 T 分隔的本地时间串。
    start_at: toPickerValue(row.start_at),
    end_at: toPickerValue(row.end_at),
    status: row.status,
    remark: row.remark || '',
  })
  formVisible.value = true
}

function toPickerValue(value: string): string {
  if (!value) return ''
  return value.replace(' ', 'T')
}

function toPayloadValue(value: string): string {
  if (!value) return ''
  return value.replace('T', ' ')
}

/** 选中了分类/商品即视为定向活动（后端口径一致：有条目就按条目命中）。 */
const hasItems = computed(() => form.categoryIds.length > 0 || form.productIds.length > 0)

async function handleSave() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  if (!hasItems.value && form.scope !== 'all') {
    MessagePlugin.warning('请选择作用分类或商品，否则活动不会命中任何商品')
    return
  }
  if (form.start_at && form.end_at && form.end_at < form.start_at) {
    MessagePlugin.error('结束时间不能早于开始时间')
    return
  }
  const payload: FlashDiscountRequest = {
    name: form.name,
    code: form.code,
    discount_type: form.discount_type,
    discount_value: Number(form.discount_value || 0),
    // 有定向条目时 scope 只作兜底；没有条目则必须是 all（上面已拦）。
    scope: hasItems.value ? form.scope : 'all',
    start_at: toPayloadValue(form.start_at) || undefined,
    end_at: toPayloadValue(form.end_at) || undefined,
    status: form.status,
    remark: form.remark,
    items: [
      ...form.categoryIds.map((id) => ({ target_type: 'category' as const, target_id: id })),
      ...form.productIds.map((id) => ({ target_type: 'product' as const, target_id: id })),
    ],
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateFlashDiscount(editingId, payload)
      MessagePlugin.success('活动已更新（按时间窗自动生效 / 失效）')
    } else {
      await createFlashDiscount(payload)
      MessagePlugin.success('活动已创建（按时间窗自动生效 / 失效）')
    }
    formVisible.value = false
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleToggleStatus(row: FlashDiscountInfo) {
  try {
    await updateFlashDiscount(row.id, {
      name: row.name,
      code: row.code,
      discount_type: row.discount_type,
      discount_value: row.discount_value,
      scope: row.scope,
      start_at: row.start_at || undefined,
      end_at: row.end_at || undefined,
      status: row.status === 'active' ? 'disabled' : 'active',
      remark: row.remark || '',
      // 整体覆盖语义：把现有条目原样回传，否则会被当成「清空条目」。
      items: (row.items || []).map((item) => ({ target_type: item.target_type, target_id: item.target_id })),
    })
    MessagePlugin.success('状态已更新')
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败')
  }
}

function handleDelete(row: FlashDiscountInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除折扣活动',
    body: `确认删除「${row.name}」？删除后该活动立即退出算价，不可恢复。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteFlashDiscount(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadData()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

function handleMobileAction(value: string | number | Record<string, any>, row: FlashDiscountInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
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

onMounted(() => {
  loadData()
  void loadOptions()
})
</script>

<style lang="css">
@import '../../shared.css';

.flash-notice {
  margin-bottom: 16px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.field-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.cell-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-strong {
  font-weight: 600;
}

.cell-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.scope-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.action-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.time-text {
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
