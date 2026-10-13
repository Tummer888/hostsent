<template>
  <section class="surface-card metrics-card">
    <header class="card-head">
      <div class="card-head__main">
        <span class="card-title">性能监控</span>
        <span class="card-head__meta">
          <template v-if="catalog?.supported">共 {{ catalog.charts.length }} 项指标</template>
          <template v-else>{{ catalog?.reason || '正在加载监控数据…' }}</template>
        </span>
      </div>
      <div class="card-head__actions">
        <t-radio-group v-model="range" variant="default-filled" size="small" @change="reloadAll">
          <t-radio-button v-for="opt in rangeOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </t-radio-button>
        </t-radio-group>
        <t-button variant="outline" size="small" :loading="loading" @click="reloadAll">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </div>
    </header>

    <!-- 平台不支持监控：给说明而不是画空图（README 里的"按钮能点但没反应"同类问题） -->
    <t-empty v-if="catalog && !catalog.supported" :description="catalog.reason || '当前实例不提供性能监控'" />

    <div v-else-if="catalog?.supported" class="metric-grid">
      <article
        v-for="chart in charts"
        :key="chartKey(chart)"
        class="metric-panel"
        :class="`metric-panel--${kindOf(chart)}`"
      >
        <div class="metric-panel__head">
          <span class="metric-title">{{ chart.title }}</span>
          <div class="metric-panel__ops">
            <t-select
              v-if="chart.select && chart.select.length"
              v-model="selected[chartKey(chart)]"
              size="small"
              class="metric-select"
              :options="chart.select.map((o) => ({ label: o.label, value: o.value }))"
              @change="() => loadSeries(chart)"
            />
            <span v-if="unitOf(chart)" class="metric-unit">{{ unitOf(chart) }}</span>
          </div>
        </div>
        <t-loading :loading="seriesLoading[chartKey(chart)]" show-overlay>
          <EChart v-if="hasPoints(chart)" :option="optionFor(chart)" :height="240" />
          <div v-else-if="seriesError[chartKey(chart)]" class="metric-empty metric-empty--error">
            <ErrorCircleIcon size="22" aria-hidden="true" />
            <span>{{ seriesError[chartKey(chart)] }}</span>
            <t-link theme="primary" hover="color" @click="loadSeries(chart)">重试</t-link>
          </div>
          <div v-else class="metric-empty">
            <ChartLineIcon size="22" aria-hidden="true" />
            <span>该时段暂无采样数据</span>
          </div>
        </t-loading>
      </article>
    </div>

    <!--
      首屏骨架：先摆出与真实卡片同高同列的占位块。
      原来是加载前只有一句文字、数据到位后卡片"从无到有"地冒出来，
      整块区域高度突增，页面明显抖一下；占位块把这个跳变抹平。
    -->
    <div v-else class="metric-grid" aria-busy="true">
      <article v-for="n in skeletonPanels" :key="n" class="metric-panel metric-panel--skeleton">
        <div class="metric-panel__head">
          <span class="metric-skeleton metric-skeleton--title" />
        </div>
        <div class="metric-skeleton metric-skeleton--chart" />
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ChartLineIcon, ErrorCircleIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import type { EChartsOption } from 'echarts'

import EChart from '@/components/EChart.vue'
import {
  getInstanceMetricsCatalog,
  getInstanceMetricsSeries,
  type MetricsCatalog,
  type MetricsChartInfo,
  type MetricsRange,
  type MetricsSeries,
} from '@/api/cloud'

defineOptions({ name: 'InstanceMetrics' })

const props = defineProps<{
  instanceId: number
}>()

const catalog = ref<MetricsCatalog | null>(null)
const loading = ref(false)
const seriesLoading = reactive<Record<string, boolean>>({})
// 单图失败与「无采样」是两回事：前者要能重试，后者是正常的空时段。
// 混在一起会让上游故障看起来像"这台机器没数据"。
const seriesError = reactive<Record<string, string>>({})
const series = reactive<Record<string, MetricsSeries>>({})
const selected = reactive<Record<string, string>>({})
const range = ref<MetricsRange>('6h')

/** 骨架占位卡数量：与常见指标数（4）对齐，网格布局不塌陷。 */
const skeletonPanels = 4

const rangeOptions: { label: string; value: MetricsRange }[] = [
  { label: '1 小时', value: '1h' },
  { label: '6 小时', value: '6h' },
  { label: '24 小时', value: '24h' },
  { label: '7 天', value: '7d' },
]

// 面板可能返回自定义 type（不是 cpu/memory/disk/network），因此 key 用原生 type。
function chartKey(chart: MetricsChartInfo): string {
  return `${chart.type}|${chart.select?.[0]?.value || ''}`
}

/** 归一化语义（用于配色/图标）；未知 kind 退化为原生 type。 */
function kindOf(chart: MetricsChartInfo): string {
  return chart.kind || chart.type
}

const charts = computed(() => catalog.value?.charts || [])

function unitOf(chart: MetricsChartInfo): string {
  return series[chartKey(chart)]?.unit || ''
}

function hasPoints(chart: MetricsChartInfo): boolean {
  const s = series[chartKey(chart)]
  return !!s && s.list.some((points) => points.length > 0)
}

async function loadCatalog() {
  loading.value = true
  try {
    const { data } = await getInstanceMetricsCatalog(props.instanceId)
    catalog.value = data
    for (const chart of data.charts || []) {
      const key = chartKey(chart)
      if (chart.select?.length && !selected[key]) selected[key] = chart.select[0].value
    }
  } catch {
    catalog.value = { supported: false, reason: '监控目录加载失败，请稍后重试', charts: [] }
  } finally {
    loading.value = false
  }
}

async function loadSeries(chart: MetricsChartInfo) {
  const key = chartKey(chart)
  seriesLoading[key] = true
  delete seriesError[key]
  try {
    const { data } = await getInstanceMetricsSeries(props.instanceId, {
      type: chart.type,
      select: selected[key],
      range: range.value,
    })
    series[key] = data
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    seriesError[key] = e?.response?.data?.message || e?.message || '监控数据获取失败'
    series[key] = { type: chart.type, unit: '', chart_type: 'line', label: [], list: [] }
  } finally {
    seriesLoading[key] = false
  }
}

/**
 * 刷新目录与全部曲线。
 *
 * 曲线**并行**拉取：原来是 for..await 串行，四张卡片按顺序一张张加载完才出现，
 * 用户看到的是"卡片从无到有蹦出来"，页面还伴随高度跳动。
 * 并行后总耗时约等于最慢的一张，卡片几乎同时就位。
 */
async function reloadAll() {
  await loadCatalog()
  await Promise.all(charts.value.map((chart) => loadSeries(chart)))
}

/**
 * 把后端归一化后的序列转成 ECharts option。
 *
 * x 轴用**第一条曲线**的采样时间：面板所有曲线同频采样，时间轴一致；
 * 若上游某条曲线缺采样，用 category 轴会比时间轴更宽容（不会出现空洞）。
 */
function optionFor(chart: MetricsChartInfo): EChartsOption {
  const s = series[chartKey(chart)]
  if (!s) return {}
  const axis = s.list[0]?.map((p) => shortTime(p.time)) || []
  // 柱状只用于单曲线。多曲线（如内存的"已用/缓存"两条）用柱状会变成
  // 一排排并排细柱，密到读不出趋势 —— 改折线/面积，趋势一眼可见。
  const isBar = s.chart_type === 'bar' && s.label.length <= 1
  return {
    grid: { top: 28, left: 8, right: 16, bottom: 4, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: unknown) => `${v}${s.unit && s.unit !== '%' ? ' ' + s.unit : s.unit === '%' ? '%' : ''}`,
    },
    legend: s.label.length > 1 ? { top: 0, right: 0, icon: 'roundRect', itemWidth: 10, itemHeight: 10 } : undefined,
    xAxis: {
      type: 'category',
      boundaryGap: isBar,
      data: axis,
      axisLine: { lineStyle: { color: '#e2e8f0' } },
      axisLabel: { color: '#94a3b8', fontSize: 11 },
    },
    yAxis: yAxisFor(s),
    series: s.label.map((name, idx) => ({
      name,
      type: isBar ? 'bar' : 'line',
      smooth: !isBar,
      symbol: 'none',
      areaStyle: s.chart_type === 'area' ? { opacity: 0.18 } : undefined,
      barMaxWidth: 22,
      data: (s.list[idx] || []).map((p) => p.value),
    })),
  }
}

/**
 * Y 轴配置。
 *
 * 百分比类指标（内存/CPU 使用率）固定 0~100：不固定时 ECharts 会按数据范围
 * 自适应（比如都挤在 20% 附近时把轴放成 18~22），曲线看着剧烈波动但实际很平稳，
 * 反而误导用户。带单位的绝对量不固定量程，交给自适应。
 */
function yAxisFor(s: MetricsSeries): Record<string, unknown> {
  const base = {
    type: 'value',
    axisLabel: { color: '#94a3b8', fontSize: 11 },
    splitLine: { lineStyle: { color: '#f1f5f9' } },
  }
  if (s.unit === '%') {
    return { ...base, min: 0, max: 100 }
  }
  return base
}

/** "2026-01-01 00:05:00" → "01-01 00:05"（横轴只留必要信息，避免拥挤）。 */
function shortTime(t: string): string {
  if (!t) return ''
  return t.length >= 16 ? `${t.slice(5, 10)} ${t.slice(11, 16)}` : t
}

function resetAll() {
  for (const key of Object.keys(series)) delete series[key]
  for (const key of Object.keys(seriesError)) delete seriesError[key]
  for (const key of Object.keys(selected)) delete selected[key]
  range.value = '6h'
}

onMounted(() => {
  void reloadAll()
})

// 实例切换（详情页单实例场景不会发生，但保持组件可复用）：重载目录与数据。
watch(
  () => props.instanceId,
  () => {
    resetAll()
    void reloadAll()
  },
)
</script>

<style scoped>
.metrics-card {
  padding: var(--space-lg) var(--space-xl);
}
.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  margin-bottom: var(--space-md);
}
.card-head__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.card-head__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
.card-head__actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.metrics-loading {
  display: block;
  padding: 40px 0;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

/* —— 首屏骨架：与真实卡片同高，加载完成时就地替换，不产生高度跳变 —— */
.metric-panel--skeleton {
  pointer-events: none;
}
.metric-skeleton {
  display: block;
  border-radius: 6px;
  background: linear-gradient(90deg, rgba(148, 163, 184, 0.14) 25%, rgba(148, 163, 184, 0.22) 37%, rgba(148, 163, 184, 0.14) 63%);
  background-size: 400% 100%;
  animation: metric-skeleton-shimmer 1.4s ease infinite;
}
.metric-skeleton--title {
  width: 30%;
  height: 16px;
}
.metric-skeleton--chart {
  height: 240px;
  margin-top: var(--space-sm);
}
@keyframes metric-skeleton-shimmer {
  0% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0 50%;
  }
}
@media (prefers-reduced-motion: reduce) {
  .metric-skeleton {
    animation: none;
  }
}
.metric-panel {
  --metric-accent: 37, 99, 235;
  position: relative;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-lg);
  padding: var(--space-md);
  background: var(--hs-surface-1);
}

/* 左上角淡色光晕按指标着色，四张图一眼能区分（与管理端统计卡片同一手法）。 */
.metric-panel::before {
  content: '';
  position: absolute;
  width: 140px;
  height: 140px;
  left: -40px;
  top: -40px;
  border-radius: 999px;
  pointer-events: none;
  background: radial-gradient(
    circle at 45% 45%,
    rgba(var(--metric-accent), 0.14) 0%,
    rgba(var(--metric-accent), 0) 72%
  );
}

.metric-panel--cpu { --metric-accent: 37, 99, 235; }
.metric-panel--memory { --metric-accent: 124, 58, 237; }
.metric-panel--disk { --metric-accent: 8, 145, 178; }
.metric-panel--network { --metric-accent: 234, 88, 12; }

.metric-panel__head,
.metric-panel :deep(.t-loading__parent),
.metric-panel :deep(.echart) {
  position: relative;
  z-index: 1;
}
.metric-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-bottom: var(--space-sm);
}
.metric-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}
.metric-panel__ops {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.metric-select {
  width: 132px;
}
.metric-unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
.metric-empty--error {
  color: var(--color-error, #d54941);
}
.metric-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 240px;
  color: var(--color-muted-foreground);
  font-size: 12px;
}
@media (max-width: 900px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }
}
</style>
