<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">优惠券管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadCoupons">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建优惠券
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
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
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="券码 / 名称" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">优惠券类型</span>
          <t-select v-model="filters.coupon_type" clearable placeholder="全部类型" :options="couponTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">优惠券列表</h3>
        <span class="table-card__meta">共 {{ total }} 张优惠券</span>
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
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">券码：{{ row.coupon_code }}</span>
          </div>
        </template>

        <template #coupon_type="{ row }">
          <span>{{ couponTypeLabel(row.coupon_type) }}</span>
        </template>

        <template #benefit="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ benefitText(row) }}</span>
            <span class="price-sub">满 {{ formatPrice(row.min_amount) }} 可用</span>
          </div>
        </template>

        <template #stock="{ row }">
          <span>{{ row.claimed_count }} / {{ row.total_stock }}</span>
        </template>

        <template #valid="{ row }">
          <div class="cell-muted">{{ formatTime(row.valid_from) }}<br />至 {{ formatTime(row.valid_to) }}</div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" hover="color" @click="openGrant(row)">批量发放</t-link>
            <t-link theme="primary" hover="color" @click="openGrants(row)">发放记录</t-link>
            <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无优惠券，请先新建" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="formVisible"
      :header="editing ? '编辑优惠券' : '新建优惠券'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="名称" name="name">
          <t-input v-model="form.name" placeholder="优惠券名称" />
        </t-form-item>
        <t-form-item label="优惠券类型" name="coupon_type">
          <t-select v-model="form.coupon_type" :options="couponTypeOptions" placeholder="请选择类型" @change="resetBenefit" />
        </t-form-item>
        <t-form-item v-if="form.coupon_type === 'fixed'" label="减免金额（元）" name="amount">
          <t-input-number v-model="form.amount" :min="0" :precision="2" theme="column" placeholder="满减金额" />
        </t-form-item>
        <t-form-item v-else label="折扣（如 0.8 表示 8 折）" name="discount">
          <t-input-number v-model="form.discount" :min="0.1" :max="1" :step="0.05" :precision="2" theme="column" placeholder="折扣系数" />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="使用门槛（元）" name="min_amount">
            <t-input-number v-model="form.min_amount" :min="0" :precision="2" theme="column" placeholder="最低消费金额" />
          </t-form-item>
          <t-form-item label="发行总量" name="total_stock">
            <t-input-number v-model="form.total_stock" :min="0" theme="column" placeholder="发行数量" />
          </t-form-item>
          <t-form-item label="每人限领" name="per_user_limit">
            <t-input-number v-model="form.per_user_limit" :min="1" theme="column" placeholder="每人限领张数" />
          </t-form-item>
          <t-form-item label="适用范围" name="scope">
            <t-input v-model="form.scope" placeholder="如：全场 / 指定分类" />
          </t-form-item>
          <t-form-item label="生效时间" name="valid_from">
            <t-date-picker v-model="form.valid_from" placeholder="生效时间" clearable />
          </t-form-item>
          <t-form-item label="失效时间" name="valid_to">
            <t-date-picker v-model="form.valid_to" placeholder="失效时间" clearable />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="grantVisible"
      header="批量发放优惠券"
      width="460px"
      :confirm-btn="{ content: '确认发放', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreateGrants"
      @close="grantVisible = false"
    >
      <p style="margin-bottom: var(--space-md); font-size: 13px; color: var(--color-muted-foreground);">
        向「{{ grantForm.couponName }}」发放 {{ grantInputIds.length }} 位用户。
      </p>
      <t-form label-align="top" @submit.prevent>
        <t-form-item label="用户 ID（逗号分隔）" name="user_ids">
          <t-textarea v-model="grantForm.userIdsText" :autosize="{ minRows: 3, maxRows: 5 }" placeholder="如：1001,1002,1003" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="grantsVisible"
      :header="`发放记录 · ${grantsCouponName}`"
      width="560px"
      :footer="false"
      @close="grantsVisible = false"
    >
      <t-table
        row-key="id"
        :data="grantList"
        :columns="grantColumns"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #status="{ row }">
          <t-tag variant="light" size="small" shape="round" :theme="row.status === 'used' ? 'success' : row.status === 'pending' ? 'warning' : 'default'">
            {{ grantStatusText(row.status) }}
          </t-tag>
        </template>
      </t-table>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import {
  createCoupon,
  createCouponGrants,
  deleteCoupon,
  getCouponGrantList,
  getCouponList,
  updateCoupon,
} from '@/api/product'
import { formatPrice, formatTime } from '@/pages/product/constants'
import type { CouponGrantInfo, CouponInfo } from '@/types/interface'

defineOptions({ name: 'ProductPromotionCoupons' })

const couponTypeOptions = [
  { label: '满减券', value: 'fixed' },
  { label: '折扣券', value: 'discount' },
]
const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const loading = ref(false)
const list = ref<CouponInfo[]>([])
const total = ref(0)

const filters = reactive<{ keyword: string | undefined; coupon_type: string | undefined; status: number | undefined }>({
  keyword: undefined,
  coupon_type: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<CouponInfo>[] = [
  { colKey: 'name', title: '优惠券', minWidth: 180 },
  { colKey: 'coupon_type', title: '类型', width: 100 },
  { colKey: 'benefit', title: '优惠', width: 150 },
  { colKey: 'stock', title: '已领/总量', width: 100 },
  { colKey: 'per_user_limit', title: '限领', width: 70 },
  { colKey: 'valid', title: '有效期', width: 170 },
  { colKey: 'status', title: '状态', width: 80 },
  {
    colKey: 'action',
    title: '操作',
    width: 200,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function couponTypeLabel(type: string): string {
  return couponTypeOptions.find((item) => item.value === type)?.label || type || '—'
}

function benefitText(row: CouponInfo): string {
  return row.coupon_type === 'discount'
    ? `${Number(row.discount).toFixed(2)} 折`
    : `减 ¥${formatPrice(row.amount)}`
}

function statusTag(status: number): { theme: 'success' | 'danger' | 'default'; text: string } {
  return status === 1 ? { theme: 'success', text: '启用' } : { theme: 'default', text: '停用' }
}

async function loadCoupons() {
  loading.value = true
  try {
    const data = await getCouponList({
      keyword: filters.keyword,
      coupon_type: filters.coupon_type,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载优惠券失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadCoupons()
}

function handleSearch() {
  pagination.current = 1
  loadCoupons()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.coupon_type = undefined
  filters.status = undefined
  pagination.current = 1
  loadCoupons()
}

// ---- 新建 / 编辑 ----
const formVisible = ref(false)
const editing = ref(false)
let editingId = 0
const form = reactive<Record<string, any>>({
  name: '',
  coupon_code: '',
  coupon_type: 'fixed',
  amount: 0,
  discount: 1,
  min_amount: 0,
  total_stock: 100,
  per_user_limit: 1,
  scope: '',
  valid_from: undefined,
  valid_to: undefined,
})

function openCreate() {
  editing.value = false
  editingId = 0
  Object.assign(form, {
    name: '',
    coupon_code: '',
    coupon_type: 'fixed',
    amount: 0,
    discount: 1,
    min_amount: 0,
    total_stock: 100,
    per_user_limit: 1,
    scope: '',
    valid_from: undefined,
    valid_to: undefined,
  })
  formVisible.value = true
}

function openEdit(row: CouponInfo) {
  editing.value = true
  editingId = row.id
  Object.assign(form, {
    name: row.name,
    coupon_code: row.coupon_code,
    coupon_type: row.coupon_type,
    amount: row.amount,
    discount: row.discount,
    min_amount: row.min_amount,
    total_stock: row.total_stock,
    per_user_limit: row.per_user_limit,
    scope: row.scope,
    valid_from: row.valid_from,
    valid_to: row.valid_to,
  })
  formVisible.value = true
}

function resetBenefit() {
  if (form.coupon_type === 'fixed') form.discount = 1
  else form.amount = 0
}

async function handleSave() {
  if (!form.name || !form.coupon_type) {
    MessagePlugin.warning('请填写名称与类型')
    return
  }
  const payload = {
    coupon_code: form.coupon_code || undefined,
    name: form.name,
    coupon_type: form.coupon_type,
    amount: form.coupon_type === 'fixed' ? form.amount : undefined,
    min_amount: form.min_amount || undefined,
    discount: form.coupon_type === 'discount' ? form.discount : undefined,
    total_stock: form.total_stock ?? undefined,
    per_user_limit: form.per_user_limit ?? undefined,
    scope: form.scope || undefined,
    valid_from: form.valid_from || undefined,
    valid_to: form.valid_to || undefined,
  }
  try {
    if (editing.value) {
      await updateCoupon(editingId, payload)
    } else {
      await createCoupon(payload)
    }
    MessagePlugin.success('已保存')
    formVisible.value = false
    loadCoupons()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  }
}

async function handleDelete(row: CouponInfo) {
  try {
    await deleteCoupon(row.id)
    MessagePlugin.success('已删除')
    loadCoupons()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除失败')
  }
}

// ---- 批量发放 ----
const grantVisible = ref(false)
let grantCouponId = 0
const grantForm = reactive<{ couponName: string; userIdsText: string }>({ couponName: '', userIdsText: '' })

const grantInputIds = computed(() =>
  grantForm.userIdsText
    .split(/[,\s，]+/)
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0),
)

function openGrant(row: CouponInfo) {
  grantCouponId = row.id
  grantForm.couponName = row.name
  grantForm.userIdsText = ''
  grantVisible.value = true
}

async function handleCreateGrants() {
  if (grantInputIds.value.length === 0) {
    MessagePlugin.warning('请输入至少一位用户 ID')
    return
  }
  try {
    const data = await createCouponGrants({ coupon_id: grantCouponId, user_ids: grantInputIds.value })
    MessagePlugin.success(`已成功发放 ${data.issued} 张`)
    grantVisible.value = false
  } catch (error) {
    MessagePlugin.error((error as Error).message || '发放失败')
  }
}

// ---- 发放记录 ----
const grantsVisible = ref(false)
const grantsCouponName = ref('')
const grantList = ref<CouponGrantInfo[]>([])
const grantColumns: PrimaryTableCol<CouponGrantInfo>[] = [
  { colKey: 'user_id', title: '用户 ID', width: 90 },
  { colKey: 'user_name', title: '用户', minWidth: 120 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'valid_to', title: '有效期至', width: 160 },
]

function grantStatusText(status: string): string {
  const map: Record<string, string> = { used: '已使用', pending: '待使用', expired: '已过期' }
  return map[status] || status || '—'
}

async function openGrants(row: CouponInfo) {
  grantsCouponName.value = row.name
  try {
    const data = await getCouponGrantList({ coupon_id: row.id, page: 1, page_size: 100 })
    grantList.value = data.items
    grantsVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载发放记录失败')
  }
}

onMounted(() => {
  loadCoupons()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
