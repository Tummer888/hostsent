<template>
  <div class="page-body sales-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">提成台账</h2>
          <p class="page-header__desc">首单/后续/续费计提、解冻转可用、退款冲减与提现冻结的完整流水</p>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" :disabled="!canApply" @click="openApplyDialog">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          申请提现
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <article class="stat-card" :class="summary.debt ? 'stat-card--red' : 'stat-card--green'">
        <span class="stat-card__icon">
          <MoneyIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.balance) }}</span>
          <span class="stat-card__label">{{ summary.debt ? '可用余额（欠款）' : '可用余额' }}</span>
        </div>
      </article>
      <article class="stat-card stat-card--warning">
        <span class="stat-card__icon">
          <TimeIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.pending_release) }}</span>
          <span class="stat-card__label">解冻中（{{ summary.release_days }} 天）</span>
        </div>
      </article>
      <article class="stat-card stat-card--blue">
        <span class="stat-card__icon">
          <LockOnIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.frozen) }}</span>
          <span class="stat-card__label">提现冻结</span>
        </div>
      </article>
      <article class="stat-card stat-card--indigo">
        <span class="stat-card__icon">
          <ChartBarIcon size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.total_income) }}</span>
          <span class="stat-card__label">累计提成</span>
        </div>
      </article>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="txTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">关联订单号</span>
          <t-input v-model="filters.order_no" placeholder="订单号模糊匹配" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">时间范围</span>
          <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleDateChange" />
        </div>
        <div class="field" v-if="canViewOthers">
          <span class="field__label">销售</span>
          <t-select
            v-model="filters.admin_id"
            clearable
            filterable
            placeholder="全部销售"
            :options="salesOptions"
          />
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
        <h3 class="card-title">流水明细</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="transactions"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #tx_no="{ row }">
          <span class="cell-muted">{{ row.tx_no }}</span>
        </template>

        <template #admin="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.admin_real_name || row.admin_name || '—' }}</span>
            <span class="price-sub">{{ row.department_name || '未分部门' }}</span>
          </div>
        </template>

        <template #type="{ row }">
          <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ row.type_label || row.type }}
          </t-tag>
        </template>

        <template #amount="{ row }">
          <span :class="row.direction >= 0 ? 'amount-income' : 'amount-expense'">
            {{ row.direction >= 0 ? '+' : '-' }}¥{{ formatPrice(row.amount) }}
          </span>
        </template>

        <template #balance_after="{ row }">
          <span class="price-main">¥{{ formatPrice(row.balance_after) }}</span>
        </template>

        <template #order_no="{ row }">
          <span class="cell-muted">{{ row.order_no || '—' }}</span>
        </template>

        <template #customer_name="{ row }">
          <span class="cell-strong">{{ row.customer_name || '—' }}</span>
        </template>

        <template #release_at="{ row }">
          <span class="time-text">{{ row.release_at ? formatTime(row.release_at) : '已解冻' }}</span>
        </template>

        <template #remark="{ row }">
          <span class="cell-muted">{{ row.remark || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无提成流水" />
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
      v-model:visible="applyVisible"
      header="申请提现"
      width="520px"
      :confirm-btn="{ content: '提交申请', theme: 'primary', loading: applySubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleApplyConfirm"
      @close="applyVisible = false"
    >
      <t-form label-align="top" :data="applyForm" @submit.prevent>
        <t-alert
          theme="info"
          :message="`可提现 ¥${formatPrice(summary.withdrawable)}，最低提现 ¥${formatPrice(summary.min_withdraw)}`"
          style="margin-bottom: 12px"
        />
        <t-form-item
          label="提现金额（元）"
          name="amount"
          :rules="[{ required: true, message: '请输入提现金额' }]"
        >
          <t-input-number
            v-model="applyForm.amount"
            :min="summary.min_withdraw"
            :max="summary.withdrawable"
            :precision="2"
            theme="column"
            placeholder="请输入提现金额"
          />
        </t-form-item>
        <t-form-item label="收款渠道" name="channel">
          <t-select v-model="applyForm.channel" :options="channelOptions" placeholder="请选择收款渠道" />
        </t-form-item>
        <t-form-item label="收款账号" name="account">
          <t-input v-model="applyForm.account" placeholder="支付宝账号 / 银行卡号" />
        </t-form-item>
        <t-form-item label="账户名" name="account_name">
          <t-input v-model="applyForm.account_name" placeholder="收款人姓名（选填）" />
        </t-form-item>
        <t-form-item v-if="applyForm.channel === 'bank'" label="开户行" name="bank_name">
          <t-input v-model="applyForm.bank_name" placeholder="如：招商银行深圳分行" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="applyForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { AddIcon, ChartBarIcon, LockOnIcon, MoneyIcon, RefreshIcon, SearchIcon, TimeIcon } from 'tdesign-icons-vue-next'

import { applySalesWithdrawal, getCommissionSummary, getCommissions, getSalesCandidates } from '@/api/sales'
import { useUserStore } from '@/store'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import type { SalesCommissionInfo, SalesCommissionSummary } from '@/types/interface'

defineOptions({ name: 'SalesCommissions' })

const { isMobile } = useIsMobile()
const userStore = useUserStore()

// 主管/超管视角：拥有审核或结算权限即可代查其他销售（与后端 scope 推导一致）
const canViewOthers = computed(() => userStore.hasPermission('sales:commission:audit', 'sales:commission:settle'))

const loading = ref(false)
const total = ref(0)
const transactions = ref<SalesCommissionInfo[]>([])

const summary = ref<SalesCommissionSummary>({
  admin_id: 0,
  admin_name: '',
  admin_real_name: '',
  department_name: '',
  enabled: true,
  balance: 0,
  pending_release: 0,
  frozen: 0,
  total_income: 0,
  total_out: 0,
  pending_withdraw: 0,
  withdrawable: 0,
  debt: false,
  min_withdraw: 100,
  release_days: 30,
  first_order_rate: 0.08,
  subsequent_rate: 0.05,
  renewal_rate: 0.03,
  active_customers: 0,
})

const filters = reactive<{
  type: string | undefined
  order_no: string
  admin_id: number | undefined
  start_at: string | undefined
  end_at: string | undefined
}>({
  type: undefined,
  order_no: '',
  admin_id: undefined,
  start_at: undefined,
  end_at: undefined,
})

const dateRange = ref<[string, string] | undefined>(undefined)

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

// 销售本人自助申请：仅当开启提成且可提现金额不低于门槛
const canApply = computed(() => summary.value.enabled && summary.value.withdrawable >= summary.value.min_withdraw)

const salesOptions = ref<Array<{ label: string; value: number }>>([])

const txTypeOptions = [
  { label: '首单提成', value: 'accrue_first' },
  { label: '后续单提成', value: 'accrue_subsequent' },
  { label: '续费提成', value: 'accrue_renewal' },
  { label: '解冻转可用', value: 'release' },
  { label: '退款冲减', value: 'refund_clawback' },
  { label: '提现冻结', value: 'withdraw_freeze' },
  { label: '提现退回', value: 'withdraw_return' },
  { label: '提现已打款', value: 'withdraw_paid' },
]

const channelOptions = [
  { label: '支付宝', value: 'alipay' },
  { label: '银行卡', value: 'bank' },
]

function txTypeTheme(type: string): 'success' | 'danger' | 'warning' | 'primary' | 'default' {
  switch (type) {
    case 'accrue_first':
    case 'accrue_subsequent':
    case 'accrue_renewal':
    case 'release':
      return 'success'
    case 'refund_clawback':
      return 'danger'
    case 'withdraw_freeze':
      return 'warning'
    case 'withdraw_return':
    case 'withdraw_paid':
      return 'primary'
    default:
      return 'default'
  }
}

const columns: PrimaryTableCol<SalesCommissionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', minWidth: 190 },
  { colKey: 'admin', title: '销售', minWidth: 150 },
  { colKey: 'type', title: '类型', width: 130 },
  { colKey: 'amount', title: '金额', width: 130, align: 'right' },
  { colKey: 'balance_after', title: '变动后可用', width: 130, align: 'right' },
  { colKey: 'order_no', title: '关联订单', minWidth: 180 },
  { colKey: 'customer_name', title: '客户', minWidth: 140 },
  { colKey: 'release_at', title: '解冻时间', width: 170 },
  { colKey: 'remark', title: '备注', minWidth: 160 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

async function loadSummary() {
  try {
    summary.value = await getCommissionSummary({})
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提成概览失败')
  }
}

async function loadTransactions() {
  loading.value = true
  try {
    const data = await getCommissions({
      type: filters.type,
      order_no: filters.order_no || undefined,
      admin_id: filters.admin_id,
      start_at: filters.start_at,
      end_at: filters.end_at,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    transactions.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    transactions.value = []
    MessagePlugin.error((error as Error).message || '加载提成流水失败')
  } finally {
    loading.value = false
  }
}

async function loadSalesOptions() {
  if (!canViewOthers.value) return
  try {
    const data = await getSalesCandidates({})
    salesOptions.value = (data.items || []).map((item) => ({
      label: `${item.real_name || item.username}${item.department_name ? `（${item.department_name}）` : ''}`,
      value: item.admin_id,
    }))
  } catch {
    salesOptions.value = []
  }
}

async function loadAll() {
  await Promise.all([loadSummary(), loadSalesOptions(), loadTransactions()])
}

function handleDateChange(value: unknown) {
  // t-date-range-picker 值为 ['YYYY-MM-DD','YYYY-MM-DD']，直接落到查询参数；
  // 清空时值为 undefined/null，需同步清掉筛选条件。
  const range = Array.isArray(value) ? (value as string[]) : undefined
  if (!range || range.length !== 2 || !range[0] || !range[1]) {
    filters.start_at = undefined
    filters.end_at = undefined
    return
  }
  filters.start_at = range[0]
  filters.end_at = range[1]
}

function handleSearch() {
  pagination.current = 1
  loadTransactions()
}

function handleResetFilters() {
  filters.type = undefined
  filters.order_no = ''
  filters.admin_id = undefined
  filters.start_at = undefined
  filters.end_at = undefined
  dateRange.value = undefined
  pagination.current = 1
  loadTransactions()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadTransactions()
}

function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  pagination.current = clamped
  mobilePage.current = clamped
  loadTransactions()
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  pagination.pageSize = pageSize
  pagination.current = 1
  mobilePage.current = 1
  loadTransactions()
}

// —— 申请提现 ——
const applyVisible = ref(false)
const applySubmitting = ref(false)
const applyForm = reactive<{
  amount: number | undefined
  channel: string
  account: string
  account_name: string
  bank_name: string
  remark: string
}>({
  amount: undefined,
  channel: 'alipay',
  account: '',
  account_name: '',
  bank_name: '',
  remark: '',
})

function openApplyDialog() {
  if (summary.value.debt) {
    MessagePlugin.warning('当前为欠款状态，暂不可提现')
    return
  }
  if (summary.value.withdrawable < summary.value.min_withdraw) {
    MessagePlugin.warning(`可提现金额不足最低提现门槛 ¥${formatPrice(summary.value.min_withdraw)}`)
    return
  }
  applyForm.amount = undefined
  applyForm.channel = 'alipay'
  applyForm.account = ''
  applyForm.account_name = ''
  applyForm.bank_name = ''
  applyForm.remark = ''
  applyVisible.value = true
}

async function handleApplyConfirm() {
  if (!applyForm.amount || applyForm.amount <= 0) {
    MessagePlugin.warning('请输入提现金额')
    return
  }
  if (applyForm.amount < summary.value.min_withdraw) {
    MessagePlugin.warning(`提现金额不得低于 ¥${formatPrice(summary.value.min_withdraw)}`)
    return
  }
  applySubmitting.value = true
  try {
    await applySalesWithdrawal({
      amount: applyForm.amount,
      channel: applyForm.channel,
      account: applyForm.account || undefined,
      account_name: applyForm.account_name || undefined,
      bank_name: applyForm.bank_name || undefined,
      remark: applyForm.remark || undefined,
    })
    MessagePlugin.success('提现申请已提交，等待审核')
    applyVisible.value = false
    await loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '提交提现申请失败')
  } finally {
    applySubmitting.value = false
  }
}

onMounted(loadAll)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.sales-module .stat-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

@media (max-width: 1200px) {
  .sales-module .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
