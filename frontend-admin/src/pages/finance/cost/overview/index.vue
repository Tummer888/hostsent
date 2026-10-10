<template>
  <div class="page-body finance-module cost-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartBarIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">成本总览</h2>
          <p class="page-header__desc">
            {{ month }} 成本与利润核算 · 数据截至 {{ overview.as_of || '—' }}
            <template v-if="overview.current">（当月为「至今」视图）</template>
          </p>
        </div>
      </div>
      <t-space size="small" align="center">
        <t-date-picker
          v-model="monthValue"
          mode="month"
          format="YYYY-MM"
          :clearable="false"
          placeholder="选择月份"
          style="width: 150px"
          @change="handleMonthChange"
        />
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <t-alert
      v-if="overview.unconfigured_hint"
      theme="warning"
      :message="overview.unconfigured_hint"
      class="cost-hint"
    >
      <template #operation>
        <t-link theme="primary" @click="router.push('/finance/cost/items')">去配置成本项</t-link>
      </template>
    </t-alert>

    <t-alert
      v-for="alert in visibleAlerts"
      :key="alert.provider_id"
      :theme="alert.level === 'critical' ? 'error' : 'warning'"
      class="cost-hint"
      :message="`【${alert.provider_name}】${alert.message}`"
    />

    <section class="cost-kpi-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><ArrowRightUpIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(overview.service_revenue) }}</span>
          <span class="stat-card__label">服务收入（消费 − 退款）</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--red">
        <span class="stat-card__icon"><ArrowLeftDownIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(overview.cost_total) }}</span>
          <span class="stat-card__label">成本合计</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><CloudIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(overview.upstream_cost) }}</span>
          <span class="stat-card__label">上游余额消耗</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><ServerIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(overview.fixed_cost) }}</span>
          <span class="stat-card__label">成本项配置（母机/人力/机房）</span>
        </div>
      </div>
      <div class="stat-card surface-card" :class="overview.profit >= 0 ? 'stat-card--success' : 'stat-card--red'">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(overview.profit) }}</span>
          <span class="stat-card__label">
            利润
            <template v-if="overview.current">
              · 摊分口径 ¥{{ formatPrice(overview.profit_to_date) }}
            </template>
          </span>
        </div>
      </div>
      <div class="stat-card surface-card" :class="overview.profit >= 0 ? 'stat-card--success' : 'stat-card--red'">
        <span class="stat-card__icon"><ChartBarIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatRate(overview.profit_rate) }}</span>
          <span class="stat-card__label">
            利润率（利润 / 收入）
            <template v-if="overview.current">
              · 摊分口径 {{ formatRate(overview.profit_rate_to_date) }}
            </template>
          </span>
        </div>
      </div>
    </section>

    <section class="cost-detail-bar surface-card">
      <div class="cost-detail-item">
        <span class="cost-detail-item__label">期间消费</span>
        <span class="cost-detail-item__value">¥{{ formatPrice(overview.consume_total) }}</span>
      </div>
      <div class="cost-detail-item">
        <span class="cost-detail-item__label">期间退款</span>
        <span class="cost-detail-item__value">¥{{ formatPrice(overview.refund_total) }}</span>
      </div>
      <div class="cost-detail-item">
        <span class="cost-detail-item__label">用户佣金入账</span>
        <span class="cost-detail-item__value">¥{{ formatPrice(overview.commission_cost) }}</span>
      </div>
      <div class="cost-detail-item">
        <span class="cost-detail-item__label">推广返现计提</span>
        <span class="cost-detail-item__value">¥{{ formatPrice(overview.referral_cost) }}</span>
      </div>
      <div class="cost-detail-item" title="上游账本消费流水净额（余额支付的开通/续费 − 对应退款），逐笔可在台账页核对">
        <span class="cost-detail-item__label">
          上游消耗·账本流水
          <t-tag
            :theme="overview.upstream_cost_source === 'ledger' ? 'primary' : 'default'"
            variant="light"
            size="small"
            shape="round"
          >
            {{ overview.upstream_cost_source === 'ledger' ? '已同步' : '无账本数据' }}
          </t-tag>
        </span>
        <span class="cost-detail-item__value">¥{{ formatPrice(overview.upstream_cost) }}</span>
      </div>
      <div class="cost-detail-item">
        <span class="cost-detail-item__label">资金口径收入（参考）</span>
        <span class="cost-detail-item__value cell-muted">¥{{ formatPrice(overview.fund_income) }}</span>
      </div>
      <div v-if="overview.current" class="cost-detail-item">
        <span class="cost-detail-item__label">成本项摊到今日</span>
        <span class="cost-detail-item__value cell-muted">¥{{ formatPrice(overview.fixed_cost_to_date) }}</span>
      </div>
    </section>

    <section class="cost-grid-2">
      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">成本构成</h3>
          <span class="table-card__meta">{{ month }} · 合计 ¥{{ formatPrice(overview.cost_total) }}</span>
        </div>
        <EChart v-if="hasCost" :option="costOption" :height="280" />
        <div v-else class="chart-empty"><t-empty description="本月暂无成本数据（上游账本未同步、未配置成本项）" /></div>

        <ul class="cost-line-list">
          <li v-for="line in overview.cost_lines" :key="line.key" class="cost-line">
            <div class="cost-line__main">
              <span class="cost-line__label">
                {{ line.label }}
                <t-tag :theme="line.auto ? 'primary' : 'warning'" variant="light" size="small" shape="round">
                  {{ line.auto ? '自动计算' : '配置项' }}
                </t-tag>
              </span>
              <span class="cost-line__usage">{{ line.usage }}</span>
            </div>
            <span class="cost-line__amount">¥{{ formatPrice(line.amount) }}</span>
          </li>
        </ul>
      </div>

      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">近 12 月成本与利润</h3>
          <span class="table-card__meta">收入/成本为柱，利润率为线（右轴）</span>
        </div>
        <EChart v-if="overview.trend.length" :option="trendOption" :height="280" />
        <div v-else class="chart-empty"><t-empty description="暂无历史数据" /></div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">上游渠道余额消耗</h3>
        <t-link theme="primary" class="view-all-link" @click="router.push('/finance/cost/upstreams')">
          查看上游台账
          <template #suffix><ChevronRightIcon size="14" /></template>
        </t-link>
      </div>
      <t-table
        row-key="provider_id"
        :data="overview.upstream_rows"
        :columns="upstreamColumns"
        :loading="loading"
        size="small"
        hover
        cell-empty-content="—"
      >
        <template #provider_name="{ row }">
          <div class="provider-cell">
            <span class="cell-strong">{{ row.provider_name || `#${row.provider_id}` }}</span>
            <span class="cell-muted">{{ row.provider_type || '—' }}</span>
          </div>
        </template>
        <template #latest_balance="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ moneyText(row.latest_balance) }}</span>
            <span class="price-sub">{{ row.latest_date || '暂无快照' }}</span>
          </div>
        </template>
        <template #consumption="{ row }">
          <div class="price-cell">
            <span v-if="row.consumption !== null" class="cell-strong">¥{{ formatPrice(row.consumption) }}</span>
            <span v-else class="cell-muted">—</span>
            <span v-if="row.cost_source === 'ledger'" class="price-sub">{{ row.consumption_entries }} 笔流水</span>
            <span v-else class="price-sub">无账本数据</span>
          </div>
        </template>
        <template #topup_total="{ row }">
          <span v-if="row.topup_total !== null" class="amount-income">{{ formatAmount(row.topup_total) }}</span>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #cost_source="{ row }">
          <t-tag
            :theme="row.cost_source === 'ledger' ? 'primary' : 'default'"
            variant="light"
            size="small"
            shape="round"
          >
            {{ row.cost_source === 'ledger' ? '账本流水' : '未接入账本' }}
          </t-tag>
        </template>
        <template #empty><t-empty description="暂无上游渠道（上游转售渠道在「资源管理 → 渠道管理」维护）" /></template>
      </t-table>
    </section>

    <p v-if="overview.caliber" class="caliber-note">{{ overview.caliber }}</p>
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
  CloudIcon,
  MoneyIcon,
  RefreshIcon,
  ServerIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'
import type { EChartsOption } from 'echarts'

import EChart from '@/components/EChart.vue'
import { getCostOverview } from '@/api/cost'
import { formatAmount, formatPrice } from '@/pages/finance/constants'
import type { CostOverviewResponse, UpstreamLedgerRow } from '@/types/interface'

defineOptions({ name: 'FinanceCostOverview' })

const router = useRouter()

/** 空响应兜底：后端未返回时不至于整页报错。 */
function emptyOverview(): CostOverviewResponse {
  return {
    month: '',
    as_of: '',
    current: true,
    service_revenue: 0,
    consume_total: 0,
    refund_total: 0,
    fund_income: 0,
    cost_total: 0,
    cost_to_date: 0,
    upstream_cost: 0,
    fixed_cost: 0,
    fixed_cost_to_date: 0,
    commission_cost: 0,
    referral_cost: 0,
    profit: 0,
    profit_rate: 0,
    profit_to_date: 0,
    profit_rate_to_date: 0,
    cost_lines: [],
    upstream_rows: [],
    trend: [],
    caliber: '',
    unconfigured_hint: '',
    upstream_cost_source: 'none',
    ledger_synced_at: '',
    balance_alerts: [],
  }
}

const loading = ref(false)
const overview = ref<CostOverviewResponse>(emptyOverview())
/** 选中的统计月（YYYY-MM 字符串，空=当月）。 */
const monthValue = ref<string>(currentMonth())

const month = computed(() => overview.value.month || monthValue.value || currentMonth())
/** 只显示非 ok 的余额水位告警。 */
const visibleAlerts = computed(() => (overview.value.balance_alerts || []).filter((a) => a.level !== 'ok'))
const hasCost = computed(() => overview.value.cost_lines.some((line) => line.amount !== 0))

const upstreamColumns: PrimaryTableCol<UpstreamLedgerRow>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 170 },
  { colKey: 'consumption', title: '期间消耗（流水）', width: 170 },
  { colKey: 'topup_total', title: '期间充值', width: 130 },
  { colKey: 'cost_source', title: '取数来源', width: 120 },
  { colKey: 'latest_balance', title: '最新余额', width: 150 },
]

/** 成本构成：横条更利于比较金额量级（自动项与配置项同图不同色）。 */
const costOption = computed<EChartsOption>(() => {
  const lines = overview.value.cost_lines
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: (v: unknown) => `¥${Number(v ?? 0).toFixed(2)}` },
    grid: { left: 8, right: 60, top: 12, bottom: 8, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
    xAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f1f5f9' } },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
    },
    yAxis: {
      type: 'category',
      data: lines.map((line) => line.label),
      axisLine: { lineStyle: { color: '#e2e8f0' } },
      axisTick: { show: false },
      axisLabel: { color: '#64748b', fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        barMaxWidth: 18,
        data: lines.map((line) => ({
          value: line.amount,
          itemStyle: { color: line.auto ? '#2563eb' : '#f59e0b', borderRadius: [0, 4, 4, 0] },
        })),
        label: {
          show: true,
          position: 'right',
          fontSize: 11,
          color: '#475569',
          formatter: (params: { value?: unknown }) => `¥${Number(params.value ?? 0).toFixed(2)}`,
        },
      },
    ],
  }
})

/** 近 12 月：柱=收入/成本，线=利润率（右轴 %）。 */
const trendOption = computed<EChartsOption>(() => {
  const points = overview.value.trend
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
    grid: { left: 8, right: 8, top: 36, bottom: 8, outerBoundsMode: 'same', outerBoundsContain: 'axisLabel' },
    xAxis: {
      type: 'category',
      data: points.map((p) => p.month.slice(5)),
      axisLine: { lineStyle: { color: '#e2e8f0' } },
      axisTick: { show: false },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
    },
    yAxis: [
      {
        type: 'value',
        name: '金额',
        nameTextStyle: { color: '#94a3b8', fontSize: 11 },
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { lineStyle: { color: '#f1f5f9' } },
        axisLabel: { color: '#94a3b8', fontSize: 11 },
      },
      {
        type: 'value',
        name: '利润率',
        nameTextStyle: { color: '#94a3b8', fontSize: 11 },
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { color: '#94a3b8', fontSize: 11, formatter: '{value}%' },
      },
    ],
    series: [
      { name: '服务收入', type: 'bar', barMaxWidth: 16, itemStyle: { color: '#16a34a' }, data: points.map((p) => p.service_revenue) },
      { name: '上游消耗', type: 'bar', stack: 'cost', barMaxWidth: 16, itemStyle: { color: '#f97316' }, data: points.map((p) => p.upstream_cost) },
      { name: '成本项', type: 'bar', stack: 'cost', barMaxWidth: 16, itemStyle: { color: '#f59e0b' }, data: points.map((p) => p.fixed_cost) },
      { name: '佣金/返现', type: 'bar', stack: 'cost', barMaxWidth: 16, itemStyle: { color: '#a855f7' }, data: points.map((p) => p.other_cost) },
      {
        name: '利润率',
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbolSize: 6,
        lineStyle: { width: 2, color: '#2563eb' },
        itemStyle: { color: '#2563eb' },
        data: points.map((p) => p.profit_rate),
      },
    ],
  }
})

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function currentMonth(): string {
  const now = new Date()
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`
}

/** 月份选择器取值归一（组件可能给字符串或 Date）。 */
function normalizeMonth(value: unknown): string {
  if (!value) return ''
  if (value instanceof Date) return `${value.getFullYear()}-${pad(value.getMonth() + 1)}`
  const matched = String(value).match(/^(\d{4})-(\d{2})/)
  return matched ? `${matched[1]}-${matched[2]}` : ''
}

function formatRate(value: number): string {
  return `${Number(value || 0).toFixed(2)}%`
}

function moneyText(value: number | null): string {
  return value === null || value === undefined ? '—' : `¥${formatPrice(value)}`
}

async function load() {
  loading.value = true
  try {
    overview.value = await getCostOverview({ month: monthValue.value })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载成本总览失败')
    overview.value = emptyOverview()
  } finally {
    loading.value = false
  }
}

function handleMonthChange(value: unknown) {
  const next = normalizeMonth(value)
  monthValue.value = next
  void load()
}

onMounted(load)
</script>

<style lang="css">
@import '../../shared.css';

.finance-module.cost-module .cost-hint {
  border-radius: var(--hs-radius-lg);
}

.finance-module.cost-module .cost-kpi-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.finance-module.cost-module .cost-detail-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-lg) var(--space-xl);
  padding: var(--space-md) var(--space-xl);
}

.finance-module.cost-module .cost-detail-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.finance-module.cost-module .cost-detail-item__label {
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.finance-module.cost-module .cost-detail-item__value {
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-primary);
}

.finance-module.cost-module .cost-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-lg);
}

.finance-module.cost-module .chart-card {
  padding: var(--space-lg) var(--space-xl);
}

.finance-module.cost-module .chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 280px;
}

.finance-module.cost-module .cost-line-list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: var(--space-md) 0 0;
  list-style: none;
  border-top: 1px dashed var(--color-border);
}

.finance-module.cost-module .cost-line {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-md);
}

.finance-module.cost-module .cost-line__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.finance-module.cost-module .cost-line__label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.finance-module.cost-module .cost-line__usage {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.finance-module.cost-module .cost-line__amount {
  font-size: 14px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-primary);
  white-space: nowrap;
}

.finance-module.cost-module .provider-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.finance-module.cost-module .caliber-note {
  margin: 0;
  padding: 0 4px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}

.finance-module.cost-module .view-all-link {
  cursor: pointer;
}

@media (max-width: 1180px) {
  .finance-module.cost-module .cost-grid-2,
  .finance-module.cost-module .cost-kpi-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
