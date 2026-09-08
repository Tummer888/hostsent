<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartLineBoardIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">订单统计</h2>
          <p class="page-header__desc">掌握平台订单的销售额、订单量、客单价与退款情况。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadStats">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <div
        v-for="stat in metricCards"
        :key="stat.key"
        class="stat-card surface-card"
        :class="`stat-card--${stat.variant}`"
      >
        <span class="stat-card__icon">
          <component :is="stat.icon" size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ stat.value }}</span>
          <span class="stat-card__label">{{ stat.label }}</span>
        </div>
      </div>
    </section>

    <section class="chart-grid">
      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">近 7 日趋势</h3>
          <span class="table-card__meta">销售额与订单量</span>
        </div>
        <EChart :option="trendOption" height="300" />
      </div>

      <div class="surface-card chart-card">
        <div class="table-card__head">
          <h3 class="card-title">状态分布</h3>
          <span class="table-card__meta">订单状态占比</span>
        </div>
        <EChart v-if="hasStatusData" :option="statusPieOption" height="300" />
        <div v-else class="chart-empty">
          <t-empty description="暂无订单数据" />
        </div>
      </div>
    </section>

    <section class="surface-card chart-card">
      <div class="table-card__head">
        <h3 class="card-title">状态明细</h3>
        <span class="table-card__meta">按订单状态统计</span>
      </div>
      <div class="dist-list">
        <div v-for="dist in distributions" :key="dist.status" class="dist-row">
          <div class="dist-row__head">
            <span class="dist-row__name">{{ orderStatusLabel(dist.status) }}</span>
            <span class="dist-row__value">{{ dist.count }}</span>
          </div>
          <div class="dist-row__track">
            <div class="dist-row__bar" :style="{ width: dist.percent + '%', background: dist.color }"></div>
          </div>
        </div>
        <t-empty v-if="distributions.length === 0" description="暂无订单数据" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, markRaw, onMounted, ref } from 'vue'

import { CartIcon, ChartLineBoardIcon, ChartPieIcon, MoneyIcon, PercentIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { EChartsOption } from 'echarts'

import EChart from '@/components/EChart.vue'
import { getOrderStats } from '@/api/order'
import { formatPrice, orderStatusLabel } from '@/pages/order/constants'
import type { OrderStatsResponse } from '@/types/interface'

defineOptions({ name: 'OrderStats' })

const loading = ref(false)
const stats = ref<OrderStatsResponse | null>(null)

const STAT_COLORS = ['#16a34a', '#3b82f6', '#f59e0b', '#6366f1', '#ec4899', '#dc2626', '#0891b2', '#64748b', '#10b981']

const metricCards = ref([
  { key: 'today_sales', label: '今日销售额', value: '¥0.00', icon: markRaw(MoneyIcon), variant: 'success' },
  { key: 'today_orders', label: '今日订单量', value: '0', icon: markRaw(CartIcon), variant: 'blue' },
  { key: 'avg_order_value', label: '客单价', value: '¥0.00', icon: markRaw(ChartPieIcon), variant: 'orange' },
  { key: 'refund_rate', label: '退款率', value: '0%', icon: markRaw(PercentIcon), variant: 'red' },
])

const trendOption = computed<EChartsOption>(() => {
  const trend = stats.value?.trend ?? []
  const dates = trend.map((t) => t.date.slice(5))
  const sales = trend.map((t) => t.sales)
  const orders = trend.map((t) => t.orders)
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
    grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: { lineStyle: { color: '#e2e8f0' } },
      axisTick: { show: false },
      axisLabel: { color: '#475569', fontSize: 12 },
    },
    yAxis: [
      {
        type: 'value',
        name: '销售额',
        axisLabel: { color: '#94a3b8' },
        splitLine: { lineStyle: { color: '#eef2f7' } },
        nameTextStyle: { color: '#94a3b8' },
      },
      {
        type: 'value',
        name: '订单量',
        axisLabel: { color: '#94a3b8' },
        splitLine: { show: false },
        nameTextStyle: { color: '#94a3b8' },
      },
    ],
    series: [
      {
        name: '销售额',
        type: 'bar',
        yAxisIndex: 0,
        data: sales,
        barMaxWidth: 18,
        itemStyle: {
          borderRadius: [4, 4, 0, 0],
          color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: '#60a5fa' }, { offset: 1, color: '#2563eb' }] },
        },
      },
      {
        name: '订单量',
        type: 'bar',
        yAxisIndex: 1,
        data: orders,
        barMaxWidth: 18,
        itemStyle: {
          borderRadius: [4, 4, 0, 0],
          color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: '#a78bfa' }, { offset: 1, color: '#4f46e5' }] },
        },
      },
    ],
  }
})

const statusPieData = computed(() => {
  const distribution = stats.value?.status_distribution ?? []
  return distribution.map((item) => ({
    name: orderStatusLabel(item.status),
    value: item.count,
  }))
})

const hasStatusData = computed(() => statusPieData.value.some((item) => item.value > 0))

const statusPieOption = computed<EChartsOption>(() => ({
  tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
  legend: { bottom: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
  series: [
    {
      name: '订单状态',
      type: 'pie',
      radius: ['46%', '70%'],
      center: ['50%', '44%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
      label: { show: true, formatter: '{b}\n{c}', fontSize: 12, color: '#334155' },
      emphasis: {
        label: { show: true, fontSize: 14, fontWeight: 'bold' },
        itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.12)' },
      },
      data: statusPieData.value,
    },
  ],
}))

const distributions = computed(() => {
  const distribution = stats.value?.status_distribution ?? []
  const total = distribution.reduce((sum, item) => sum + item.count, 0)
  return distribution.map((item, index) => ({
    status: item.status,
    count: item.count,
    percent: total > 0 ? Math.round((item.count / total) * 100) : 0,
    color: STAT_COLORS[index % STAT_COLORS.length],
  }))
})

function loadMetricCards() {
  const data = stats.value
  if (!data) return
  metricCards.value[0].value = `¥${formatPrice(data.today_sales)}`
  metricCards.value[1].value = String(data.today_orders)
  metricCards.value[2].value = `¥${formatPrice(data.avg_order_value)}`
  metricCards.value[3].value = `${data.refund_rate}%`
}

async function loadStats() {
  loading.value = true
  try {
    stats.value = await getOrderStats()
    loadMetricCards()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载订单统计失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadStats)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 300px;
}
</style>
