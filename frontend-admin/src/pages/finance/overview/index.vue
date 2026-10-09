<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">财务总览</h2>
          <p class="page-header__desc">
            {{ rangeLabel }} · 口径内收入/支出/净额与待办（数据由后端全量聚合，非列表首屏累加）
          </p>
        </div>
      </div>
      <t-space size="small" align="center">
        <t-radio-group v-model="preset" size="small" class="range-group" @change="handlePresetChange">
          <t-radio-button v-for="item in periodPresetOptions" :key="item.value" :value="item.value">
            {{ item.label }}
          </t-radio-button>
        </t-radio-group>
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="wallet-stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><ArrowRightUpIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.income_total) }}</span>
          <span class="stat-card__label">期间收入</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--red">
        <span class="stat-card__icon"><ArrowLeftDownIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.expense_total) }}</span>
          <span class="stat-card__label">期间支出</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><ChartBarIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.net_total) }}</span>
          <span class="stat-card__label">净额（收入 − 支出）</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><FileIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.tx_count }}</span>
          <span class="stat-card__label">
            流水笔数<template v-if="summary.internal_count">（另有 {{ summary.internal_count }} 笔内部划转）</template>
          </span>
        </div>
      </div>
    </section>

    <section class="overview-grid">
      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">收支趋势</h3>
          <span class="table-card__meta">{{ rangeLabel }} · 收入 / 支出 / 净额</span>
        </div>
        <EChart v-if="trend.length" :option="trendOption" height="300" />
        <div v-else class="chart-empty"><t-empty description="所选期间暂无流水" /></div>
      </div>

      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">资金与待办</h3>
          <span class="table-card__meta">入口仅做跳转，不改变权限</span>
        </div>

        <div class="fund-block">
          <div class="fund-row">
            <span class="fund-row__label">钱包可用余额合计</span>
            <span class="fund-row__value">¥{{ formatPrice(wallet.balance_total) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">冻结金额合计</span>
            <span class="fund-row__value">¥{{ formatPrice(wallet.frozen_total) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">钱包账户数</span>
            <span class="fund-row__value">{{ wallet.count }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">低余额钱包（< ¥{{ formatPrice(wallet.low_balance_threshold) }}）</span>
            <span class="fund-row__value" :class="{ 'is-warning': wallet.low_balance_count > 0 }">
              {{ wallet.low_balance_count }} 个 / ¥{{ formatPrice(wallet.low_balance_amount) }}
            </span>
          </div>
        </div>

        <div class="todo-list">
          <button
            v-for="item in todoItems"
            :key="item.key"
            type="button"
            class="todo-item"
            :class="{ 'is-clear': item.count === 0 }"
            @click="router.push(item.to)"
          >
            <span class="todo-item__label">{{ item.label }}</span>
            <span class="todo-item__count">
              {{ item.count }}<template v-if="item.amount !== undefined"> · ¥{{ formatPrice(item.amount) }}</template>
            </span>
            <ChevronRightIcon size="16" aria-hidden="true" />
          </button>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近流水</h3>
        <t-link theme="primary" class="view-all-link" @click="router.push('/finance/transactions')">
          查看全部
          <template #suffix><ChevronRightIcon size="14" /></template>
        </t-link>
      </div>
      <t-table
        row-key="id"
        :data="recentItems"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        cell-empty-content="—"
      >
        <template #tx_no="{ row }">
          <span class="cell-strong">{{ row.tx_no }}</span>
        </template>
        <template #username="{ row }">
          <span>{{ row.username || `#${row.user_id}` }}</span>
        </template>
        <template #type="{ row }">
          <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ txTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #amount="{ row }">
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">
            {{ formatAmount(row.amount, row.direction) }}
          </span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty><t-empty description="暂无流水" /></template>
      </t-table>
    </section>

    <p v-if="caliber" class="caliber-note">{{ caliber }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import {
  ArrowLeftDownIcon,
  ArrowRightUpIcon,
  ChartBarIcon,
  ChevronRightIcon,
  FileIcon,
  MoneyIcon,
  RefreshIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'
import type { EChartsOption } from 'echarts'

import EChart from '@/components/EChart.vue'
import { getFinanceStats, getTransactionList } from '@/api/finance'
import {
  formatAmount,
  formatPrice,
  formatTime,
  periodPresetOptions,
  resolvePeriodRange,
  txTypeLabel,
  txTypeTheme,
  type PeriodPreset,
} from '@/pages/finance/constants'
import type { FinanceStatsResponse, TransactionInfo } from '@/types/interface'

defineOptions({ name: 'FinanceOverview' })

const router = useRouter()

const loading = ref(false)
const preset = ref<PeriodPreset>('last30')
const stats = ref<FinanceStatsResponse | null>(null)
const recentItems = ref<TransactionInfo[]>([])

const columns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', minWidth: 170 },
  { colKey: 'username', title: '用户', minWidth: 120 },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'balance_after', title: '变动后余额', width: 120 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

const summary = computed(() => stats.value?.summary ?? {
  income_total: 0,
  expense_total: 0,
  net_total: 0,
  tx_count: 0,
  avg_amount: 0,
  internal_count: 0,
})
const trend = computed(() => stats.value?.trend ?? [])
const wallet = computed(() => stats.value?.wallet ?? {
  balance_total: 0,
  frozen_total: 0,
  count: 0,
  low_balance_threshold: 0,
  low_balance_count: 0,
  low_balance_amount: 0,
})
const caliber = computed(() => stats.value?.caliber ?? '')

const rangeLabel = computed(() => {
  const range = stats.value?.range
  if (!range) return '近 30 天'
  return `${range.start_time} ~ ${range.end_time}`
})

/** 待办：点进去就是处理页（入口仅跳转，越权由目标页与后端权限码拦截）。 */
const todoItems = computed(() => {
  const pending = stats.value?.pending
  const bills = stats.value?.bills
  return [
    {
      key: 'recharge',
      label: '待确认充值',
      count: pending?.recharge_pending_count ?? 0,
      amount: pending?.recharge_pending_amount ?? 0,
      to: '/finance/recharges',
    },
    {
      key: 'withdraw',
      label: '待审核提现',
      count: pending?.withdraw_pending_count ?? 0,
      amount: pending?.withdraw_pending_amount ?? 0,
      to: '/finance/withdrawals',
    },
    {
      key: 'payout',
      label: '待打款提现',
      count: pending?.withdraw_paying_count ?? 0,
      amount: pending?.withdraw_paying_amount ?? 0,
      to: '/finance/withdrawals',
    },
    {
      key: 'invoice',
      label: '待开票申请',
      count: pending?.invoice_pending_count ?? 0,
      amount: undefined,
      to: '/finance/invoices',
    },
    {
      key: 'bill',
      label: '未结账单',
      count: bills?.unpaid_count ?? 0,
      amount: bills?.unpaid_amount ?? 0,
      to: '/finance/bills',
    },
  ]
})

const trendOption = computed<EChartsOption>(() => {
  const points = trend.value
  const labels = points.map((p) => (p.period.length === 7 ? p.period : p.period.slice(5)))
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: unknown) => `¥${Number(v ?? 0).toFixed(2)}` },
    legend: { top: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
    grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: '#e2e8f0' } },
      axisTick: { show: false },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f1f5f9' } },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
    },
    series: [
      { name: '收入', type: 'bar', stack: 'flow', barMaxWidth: 18, itemStyle: { color: '#16a34a', borderRadius: [0, 0, 0, 0] }, data: points.map((p) => p.income) },
      { name: '支出', type: 'bar', stack: 'flow', barMaxWidth: 18, itemStyle: { color: '#ef4444' }, data: points.map((p) => p.expense) },
      {
        name: '净额',
        type: 'line',
        smooth: true,
        symbolSize: 6,
        lineStyle: { width: 2, color: '#2563eb' },
        itemStyle: { color: '#2563eb' },
        data: points.map((p) => p.net),
      },
    ],
  }
})

async function load() {
  loading.value = true
  const range = resolvePeriodRange(preset.value)
  try {
    const [statsData, txData] = await Promise.all([
      getFinanceStats({ start_time: range.start, end_time: range.end }),
      getTransactionList({ page: 1, page_size: 10, start_time: `${range.start} 00:00:00`, end_time: `${range.end} 23:59:59` }),
    ])
    stats.value = statsData
    recentItems.value = txData.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载财务总览失败')
    recentItems.value = []
  } finally {
    loading.value = false
  }
}

function handlePresetChange() {
  void load()
}

onMounted(load)
</script>

<style lang="css">
@import '../shared.css';

.finance-module .range-group .t-radio-button {
  background: var(--hs-surface-2);
  border-color: var(--color-border);
}

.finance-module .overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
  gap: var(--space-lg);
}

@media (max-width: 1180px) {
  .finance-module .overview-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

.finance-module .chart-card {
  padding: var(--space-lg) var(--space-xl);
}

.finance-module .chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 300px;
}

.finance-module .fund-block {
  display: grid;
  gap: 8px;
  padding-bottom: var(--space-md);
  border-bottom: 1px dashed var(--color-border);
}

.finance-module .fund-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.finance-module .fund-row__label {
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.finance-module .fund-row__value {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.finance-module .fund-row__value.is-warning {
  color: #d97706;
}

.finance-module .todo-list {
  display: grid;
  gap: 6px;
  padding-top: var(--space-md);
}

.finance-module .todo-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md, 8px);
  background: var(--hs-surface-2);
  color: var(--td-text-color-primary);
  font-size: 13px;
  cursor: pointer;
  transition: border-color 160ms ease, background 160ms ease;
}

.finance-module .todo-item:hover {
  border-color: var(--finance-green-border);
  background: var(--finance-green-soft);
}

.finance-module .todo-item.is-clear {
  opacity: 0.62;
}

.finance-module .todo-item__label {
  flex: 1;
  text-align: left;
}

.finance-module .todo-item__count {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.finance-module .caliber-note {
  margin: 0;
  padding: 0 4px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}

.finance-module .view-all-link {
  cursor: pointer;
}
</style>
