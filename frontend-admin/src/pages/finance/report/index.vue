<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartBarIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">财务报表</h2>
          <p class="page-header__desc">按期间的收支汇总、类型分布与账单口径（同一口径可导出 CSV）</p>
        </div>
      </div>
      <t-space size="small" align="center">
        <t-radio-group v-model="preset" size="small" class="range-group" @change="handlePresetChange">
          <t-radio-button v-for="item in periodPresetOptions" :key="item.value" :value="item.value">
            {{ item.label }}
          </t-radio-button>
        </t-radio-group>
        <t-select v-model="granularity" :options="granularityOptions" size="small" style="width: 110px" @change="load" />
        <t-date-range-picker v-model="customRange" clearable size="small" style="width: 240px" @change="handleCustomRange" />
        <t-button variant="outline" :loading="exporting" @click="handleExport">
          <template #icon><DownloadIcon aria-hidden="true" /></template>
          导出报表
        </t-button>
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="wallet-stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.income_total) }}</span>
          <span class="stat-card__label">收入合计</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--red">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.expense_total) }}</span>
          <span class="stat-card__label">支出合计</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><ChartBarIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(summary.net_total) }}</span>
          <span class="stat-card__label">净额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><FileIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ summary.tx_count }}</span>
          <span class="stat-card__label">笔数 · 笔均 ¥{{ formatPrice(summary.avg_amount) }}</span>
        </div>
      </div>
    </section>

    <section class="surface-card chart-card">
      <div class="table-card__head">
        <h3 class="card-title">收支趋势</h3>
        <span class="table-card__meta">{{ rangeLabel }} · 粒度{{ granularityLabel }}</span>
      </div>
      <EChart v-if="trend.length" :option="trendOption" :height="320" />
      <div v-else class="chart-empty"><t-empty description="所选期间暂无流水" /></div>
    </section>

    <section class="overview-grid">
      <div class="surface-card table-card">
        <div class="table-card__head">
          <h3 class="card-title">类型分布</h3>
          <span class="table-card__meta">按流水类型 · 内部划转不计入收支</span>
        </div>
        <t-table
          row-key="type"
          :data="typeBreakdown"
          :columns="typeColumns"
          :loading="loading"
          size="small"
          hover
          cell-empty-content="—"
        >
          <template #type="{ row }">
            <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
              {{ txTypeLabel(row.type) }}
            </t-tag>
            <span v-if="row.internal" class="cell-muted">内部划转</span>
          </template>
          <template #income="{ row }">
            <span class="amount-income">{{ row.income ? `+${formatPrice(row.income)}` : '—' }}</span>
          </template>
          <template #expense="{ row }">
            <span class="amount-expense">{{ row.expense ? `-${formatPrice(row.expense)}` : '—' }}</span>
          </template>
          <template #net="{ row }">
            <span :class="row.net >= 0 ? 'amount-income' : 'amount-expense'">{{ formatPrice(row.net) }}</span>
          </template>
          <template #empty><t-empty description="所选期间暂无流水" /></template>
        </t-table>
      </div>

      <div class="surface-card table-card">
        <div class="table-card__head">
          <h3 class="card-title">账单口径</h3>
          <span class="table-card__meta">按生成时间 · 未结为全量</span>
        </div>
        <div class="bill-block">
          <div class="fund-row">
            <span class="fund-row__label">期间账单数</span>
            <span class="fund-row__value">{{ bills.count }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">期间应收（按期应结）</span>
            <span class="fund-row__value">¥{{ formatPrice(bills.total_amount) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">期间退款</span>
            <span class="fund-row__value">¥{{ formatPrice(bills.refund_amount) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">期间净应收</span>
            <span class="fund-row__value">¥{{ formatPrice(bills.net_amount) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">期间充值凭证（逐笔）</span>
            <span class="fund-row__value">{{ bills.recharge_count }} 笔 / ¥{{ formatPrice(bills.recharge_amount) }}</span>
          </div>
          <div class="fund-row">
            <span class="fund-row__label">未结账单（全量）</span>
            <span class="fund-row__value" :class="{ 'is-warning': bills.unpaid_count > 0 }">
              {{ bills.unpaid_count }} 笔 / ¥{{ formatPrice(bills.unpaid_amount) }}
            </span>
          </div>
        </div>

        <div class="status-block">
          <div v-for="row in bills.by_status" :key="row.status" class="status-row">
            <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
              {{ statusLabel(row.status) }}
            </t-tag>
            <span class="status-row__count">{{ row.count }} 笔</span>
            <span class="status-row__amount">¥{{ formatPrice(row.amount) }}</span>
          </div>
          <t-empty v-if="!bills.by_status.length" description="所选期间暂无账单" />
        </div>

        <div class="table-card__foot">
          <t-link theme="primary" @click="router.push('/finance/bills')">查看账单明细 →</t-link>
        </div>
      </div>
    </section>

    <p v-if="caliber" class="caliber-note">{{ caliber }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ChartBarIcon, DownloadIcon, FileIcon, MoneyIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'
import type { EChartsOption } from 'echarts'

import EChart from '@/components/EChart.vue'
import { getFinanceStats } from '@/api/finance'
import {
  billStatusLabel,
  billStatusTheme,
  downloadCsv,
  formatPrice,
  periodPresetOptions,
  resolvePeriodRange,
  toDateString,
  txTypeLabel,
  txTypeTheme,
  type PeriodPreset,
} from '@/pages/finance/constants'
import type { FinanceStatsResponse, FinanceStatsTypeRow } from '@/types/interface'

defineOptions({ name: 'FinanceReport' })

const router = useRouter()

const loading = ref(false)
const exporting = ref(false)
const preset = ref<PeriodPreset>('last30')
const granularity = ref<'day' | 'month'>('day')
const customRange = ref<(string | Date)[] | undefined>(undefined)
const stats = ref<FinanceStatsResponse | null>(null)

const granularityOptions = [
  { label: '按日', value: 'day' },
  { label: '按月', value: 'month' },
]

const typeColumns: PrimaryTableCol<FinanceStatsTypeRow>[] = [
  { colKey: 'type', title: '类型', minWidth: 140 },
  { colKey: 'income', title: '收入', width: 130, align: 'right' as const },
  { colKey: 'expense', title: '支出', width: 130, align: 'right' as const },
  { colKey: 'net', title: '净额', width: 130, align: 'right' as const },
  { colKey: 'count', title: '笔数', width: 80, align: 'right' as const },
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
const typeBreakdown = computed(() => stats.value?.type_breakdown ?? [])
const bills = computed(() => stats.value?.bills ?? {
  count: 0,
  total_amount: 0,
  refund_amount: 0,
  net_amount: 0,
  recharge_count: 0,
  recharge_amount: 0,
  by_status: [],
  unpaid_count: 0,
  unpaid_amount: 0,
})
const caliber = computed(() => stats.value?.caliber ?? '')

const rangeLabel = computed(() => {
  const range = stats.value?.range
  return range ? `${range.start_time} ~ ${range.end_time}` : '近 30 天'
})
// 后端在跨度 > 366 天时会自动升为按月，以实际生效粒度为准。
const granularityLabel = computed(() => (stats.value?.range.granularity === 'month' ? '按月' : '按日'))

function statusLabel(status: string): string {
  return billStatusLabel(status)
}
function statusTheme(status: string): string {
  return billStatusTheme(status)
}

const trendOption = computed<EChartsOption>(() => {
  const points = trend.value
  const labels = points.map((p) => (p.period.length === 7 ? p.period : p.period.slice(5)))
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: unknown) => `¥${Number(v ?? 0).toFixed(2)}` },
    legend: { top: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
    // echarts 6：containLabel 已废弃，等价写法是 outerBoundsMode/outerBoundsContain。
    grid: { left: 8, right: 8, top: 36, bottom: 8, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
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
      { name: '收入', type: 'bar', stack: 'flow', barMaxWidth: 20, itemStyle: { color: '#16a34a' }, data: points.map((p) => p.income) },
      { name: '支出', type: 'bar', stack: 'flow', barMaxWidth: 20, itemStyle: { color: '#ef4444' }, data: points.map((p) => p.expense) },
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
    stats.value = await getFinanceStats({
      start_time: range.start,
      end_time: range.end,
      granularity: granularity.value,
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载财务报表失败')
  } finally {
    loading.value = false
  }
}

function handlePresetChange() {
  void load()
}

/** 自定义区间：置为 custom 并立即按所选日期查询（清空则回默认近 30 天）。 */
function handleCustomRange(value: unknown) {
  const [start, end] = (value as (string | Date)[] | undefined) ?? []
  const startText = toDateString(start)
  const endText = toDateString(end)
  if (!startText || !endText) {
    preset.value = 'last30'
    void load()
    return
  }
  preset.value = 'custom'
  loading.value = true
  getFinanceStats({ start_time: startText, end_time: endText, granularity: granularity.value })
    .then((data) => {
      stats.value = data
    })
    .catch((error: Error) => {
      MessagePlugin.error(error.message || '加载财务报表失败')
    })
    .finally(() => {
      loading.value = false
    })
}

/** 导出报表：期间汇总 + 趋势 + 类型分布 + 账单口径，均为当前页面所见数据。 */
function handleExport() {
  if (!stats.value) {
    MessagePlugin.warning('暂无可导出的报表数据')
    return
  }
  exporting.value = true
  try {
    const rows: (string | number)[][] = []
    const range = stats.value.range
    rows.push(['期间', `${range.start_time} ~ ${range.end_time}`])
    rows.push(['粒度', range.granularity === 'month' ? '按月' : '按日'])
    rows.push([])
    rows.push(['汇总（资金口径，不含冻结/解冻内部划转）'])
    rows.push(['收入合计', summary.value.income_total])
    rows.push(['支出合计', summary.value.expense_total])
    rows.push(['净额', summary.value.net_total])
    rows.push(['笔数', summary.value.tx_count])
    rows.push(['笔均金额', summary.value.avg_amount])
    rows.push(['内部划转笔数', summary.value.internal_count])
    rows.push([])
    rows.push(['趋势', '收入', '支出', '净额', '笔数'])
    for (const point of trend.value) {
      rows.push([point.period, point.income, point.expense, point.net, point.count])
    }
    rows.push([])
    rows.push(['类型', '收入', '支出', '净额', '笔数', '是否内部划转'])
    for (const row of typeBreakdown.value) {
      rows.push([txTypeLabel(row.type), row.income, row.expense, row.net, row.count, row.internal ? '是' : '否'])
    }
    rows.push([])
    rows.push(['账单口径', '金额/数量'])
    rows.push(['期间账单数', bills.value.count])
    rows.push(['期间应收', bills.value.total_amount])
    rows.push(['期间退款', bills.value.refund_amount])
    rows.push(['期间净应收', bills.value.net_amount])
    rows.push(['期间充值凭证笔数', bills.value.recharge_count])
    rows.push(['期间充值凭证金额', bills.value.recharge_amount])
    rows.push(['未结账单数（全量）', bills.value.unpaid_count])
    rows.push(['未结应收（全量）', bills.value.unpaid_amount])
    rows.push([])
    rows.push(['口径说明', stats.value.caliber])

    downloadCsv(
      `finance-report-${range.start_time}_${range.end_time}.csv`,
      ['项目', '数值', '数值2', '数值3', '数值4', '备注'],
      rows,
    )
    MessagePlugin.success('报表已导出')
  } finally {
    exporting.value = false
  }
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
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
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
  min-height: 320px;
}

.finance-module .bill-block {
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
  font-variant-numeric: tabular-nums;
}

.finance-module .fund-row__value.is-warning {
  color: #d97706;
}

.finance-module .status-block {
  display: grid;
  gap: 8px;
  padding-top: var(--space-md);
}

.finance-module .status-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.finance-module .status-row__count {
  color: var(--color-muted-foreground);
}

.finance-module .status-row__amount {
  margin-left: auto;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.finance-module .table-card__foot {
  padding-top: var(--space-md);
}

.finance-module .caliber-note {
  margin: 0;
  padding: 0 4px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}
</style>
