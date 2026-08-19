<template>
  <div class="dashboard-page">
    <section class="dashboard-hero surface-card" aria-labelledby="dashboard-hero-title">
      <div class="hero-content">
        <div class="hero-status">
          <span class="status-dot" aria-hidden="true"></span>
          <span class="hero-label">Dashboard · 实时控制台</span>
        </div>
        <h2 id="dashboard-hero-title" class="hero-title">
          {{ greeting }}，
          <span class="hero-title--accent">
            {{ userStore.userInfo.name || userStore.userInfo.username }}
          </span>
        </h2>
        <p class="hero-desc">
          今天是 {{ today }}。平台运行稳定，共监控 {{ totalResources }} 项活跃资源。
        </p>
        <div class="hero-actions">
          <t-button theme="primary" size="medium" class="hero-btn hero-btn--primary">
            <template #icon>
              <CloudIcon aria-hidden="true" />
            </template>
            新建云主机
          </t-button>
          <t-button variant="outline" size="medium" class="hero-btn hero-btn--outline">
            <template #icon>
              <FileExportIcon aria-hidden="true" />
            </template>
            导出周报
          </t-button>
        </div>
      </div>
      <div class="hero-visual">
        <img
          :src="heroImage"
          alt="云控平台监控示意图"
          class="hero-visual__img"
          loading="eager"
          width="420"
          height="220"
        />
      </div>
    </section>

    <section class="stat-grid" aria-label="关键指标">
      <article
        v-for="(stat, idx) in statCards"
        :key="stat.key"
        class="stat-card surface-card"
        :style="{ animationDelay: `${60 + idx * 50}ms` }"
      >
        <div class="stat-card__head">
          <span class="stat-card__title">{{ stat.title }}</span>
          <div class="stat-card__icon" :class="`stat-card__icon--${stat.variant}`">
            <component :is="stat.icon" size="18" aria-hidden="true" />
          </div>
        </div>
        <div class="stat-card__main">
          <t-statistic
            :value="stat.value"
            :decimal-places="0"
            :precision="0"
            class="stat-card__value"
          />
          <t-tag
            :theme="stat.trend === 'up' ? 'success' : stat.trend === 'down' ? 'warning' : 'default'"
            variant="light"
            shape="round"
            class="stat-card__trend"
          >
            <ArrowUpIcon v-if="stat.trend === 'up'" size="11" aria-hidden="true" />
            <ArrowDownIcon v-else-if="stat.trend === 'down'" size="11" aria-hidden="true" />
            <MinusIcon v-else size="11" aria-hidden="true" />
            {{ stat.change }}
          </t-tag>
        </div>
        <div class="stat-card__spark" aria-hidden="true">
          <svg viewBox="0 0 200 36" preserveAspectRatio="none" class="spark-svg">
            <defs>
              <linearGradient :id="`grad-${stat.key}`" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" :stop-color="stat.sparkColor" stop-opacity="0.28" />
                <stop offset="100%" :stop-color="stat.sparkColor" stop-opacity="0" />
              </linearGradient>
            </defs>
            <path
              :d="stat.sparkArea"
              :fill="`url(#grad-${stat.key})`"
              class="spark-area"
            />
            <path
              :d="stat.sparkLine"
              fill="none"
              :stroke="stat.sparkColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="spark-line"
            />
          </svg>
        </div>
      </article>
    </section>

    <section class="panel-grid">
      <article class="panel-card surface-card" aria-labelledby="overview-title">
        <header class="panel-card__head">
          <div>
            <h3 id="overview-title" class="panel-card__title">平台运行概览</h3>
            <p class="panel-card__subtitle">实时资源使用率 · 上次同步 {{ lastSync }}</p>
          </div>
          <t-radio-group v-model="range" size="small" class="range-group">
            <t-radio-button value="today">今日</t-radio-button>
            <t-radio-button value="7day">近7日</t-radio-button>
            <t-radio-button value="30day">近30日</t-radio-button>
          </t-radio-group>
        </header>
        <div class="overview-list">
          <div
            v-for="(item, idx) in overview"
            :key="item.label"
            class="overview-item"
            :style="{ animationDelay: `${80 + idx * 40}ms` }"
          >
            <div class="overview-item__head">
              <span class="overview-item__label">{{ item.label }}</span>
              <span class="overview-item__value" :data-theme="item.theme">{{ item.value }}</span>
            </div>
            <t-progress
              :percentage="item.percent"
              :theme="item.theme"
              :label="false"
              size="medium"
              class="overview-item__progress"
            />
          </div>
        </div>
      </article>

      <article class="panel-card surface-card" aria-labelledby="logs-title">
        <header class="panel-card__head">
          <div>
            <h3 id="logs-title" class="panel-card__title">最近操作日志</h3>
            <p class="panel-card__subtitle">审计追踪 · 共 {{ logList.length }} 条记录</p>
          </div>
          <t-link theme="primary" size="small" hover="color" class="view-all-link">
            查看全部
            <template #suffix>
              <ChevronRightIcon size="13" aria-hidden="true" />
            </template>
          </t-link>
        </header>
        <t-table
          :data="logList"
          :columns="logColumns"
          size="small"
          row-key="id"
          :bordered="false"
          :pagination="false"
          :row-class-name="resolveLogRowClass"
          class="log-table"
        >
          <template #result="{ row }">
            <t-tag
              :theme="row.result === '成功' ? 'success' : 'danger'"
              variant="light"
              shape="round"
            >
              {{ row.result }}
            </t-tag>
          </template>
        </t-table>
      </article>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, shallowRef } from 'vue'

import {
  ArrowDownIcon,
  ArrowUpIcon,
  ChevronRightIcon,
  CloudIcon,
  FileCopyIcon,
  FileExportIcon,
  LayersIcon,
  MinusIcon,
  ServiceIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { useUserStore } from '@/store'

defineOptions({ name: 'AdminDashboardBase' })

const userStore = useUserStore()
const range = ref('7day')

const heroImage =
  'https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=' +
  encodeURIComponent(
    'light minimal cloud infrastructure monitoring illustration with servers and dashboard charts, white background, green accent color, flat vector style, professional SaaS hero image, 16:9'
  ) +
  '&image_size=landscape_16_9'

const today = new Date().toLocaleDateString('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  weekday: 'long',
})

const lastSync = computed(() => {
  const d = new Date()
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
})

const hour = new Date().getHours()
const greeting = computed(() => {
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

function wrap(iconComp: unknown) {
  return shallowRef({ render: () => h(iconComp as never) })
}

function buildSpark(points: number[], height = 36, width = 200) {
  const stepX = width / (points.length - 1)
  const min = Math.min(...points)
  const max = Math.max(...points)
  const rangeVal = max - min || 1
  const pts = points.map((v, i) => {
    const x = i * stepX
    const y = height - ((v - min) / rangeVal) * (height - 6) - 3
    return [x, y] as const
  })
  let line = `M ${pts[0][0]} ${pts[0][1]}`
  for (let i = 1; i < pts.length; i++) {
    const [x0, y0] = pts[i - 1]
    const [x1, y1] = pts[i]
    const cx = (x0 + x1) / 2
    line += ` C ${cx} ${y0}, ${cx} ${y1}, ${x1} ${y1}`
  }
  const area = `${line} L ${width} ${height} L 0 ${height} Z`
  return { line, area }
}

type Trend = 'up' | 'down' | 'flat'
type StatVariant = 'blue' | 'cyan' | 'green' | 'orange'

type StatCardItem = {
  key: string
  title: string
  value: number
  variant: StatVariant
  trend: Trend
  change: string
  sparkColor: string
  sparkLine: string
  sparkArea: string
  icon: ReturnType<typeof wrap>
}

const totalResources = computed(() => 1792)

const statCards = computed<StatCardItem[]>(() => {
  const data: Array<Omit<StatCardItem, 'sparkLine' | 'sparkArea'>> = [
    { key: 'users', title: '当前用户总数', value: 1284, variant: 'blue', trend: 'up', change: '+12%', sparkColor: '#2563EB', icon: wrap(UserIcon) },
    { key: 'hosts', title: '在役云主机数', value: 326, variant: 'cyan', trend: 'up', change: '+6%', sparkColor: '#0891B2', icon: wrap(LayersIcon) },
    { key: 'orders', title: '本月订单数', value: 182, variant: 'green', trend: 'up', change: '+18%', sparkColor: '#16A34A', icon: wrap(FileCopyIcon) },
    { key: 'tickets', title: '待处理工单', value: 9, variant: 'orange', trend: 'down', change: '-2%', sparkColor: '#D97706', icon: wrap(ServiceIcon) },
  ]
  return data.map((item) => {
    const seq = Array.from({ length: 14 }, (_, i) => 30 + Math.sin(i * 0.9 + item.key.length) * 18 + (i % 3) * 6)
    const { line, area } = buildSpark(seq)
    return { ...item, sparkLine: line, sparkArea: area }
  })
})

type OverviewTheme = 'primary' | 'success' | 'warning' | 'danger'

const overview = ref<Array<{ label: string; value: string; percent: number; theme: OverviewTheme }>>([
  { label: 'CPU 平均使用率', value: '58%', percent: 58, theme: 'primary' },
  { label: '内存平均使用率', value: '64%', percent: 64, theme: 'success' },
  { label: '网络入流量', value: '42%', percent: 42, theme: 'warning' },
  { label: '存储使用率', value: '77%', percent: 77, theme: 'danger' },
])

type LogRow = {
  id: number
  operator: string
  action: string
  target: string
  time: string
  result: string
}

const logList: LogRow[] = [
  { id: 1, operator: 'admin', action: '重置用户密码', target: 'u_1023', time: '2026-08-19 10:06', result: '成功' },
  { id: 2, operator: 'admin', action: '启动云主机', target: 'host-0082', time: '2026-08-19 09:41', result: '成功' },
  { id: 3, operator: 'admin', action: '审核订单支付', target: 'P202608190013', time: '2026-08-19 09:18', result: '成功' },
  { id: 4, operator: 'admin', action: '创建产品规格', target: '通用型-2C4G', time: '2026-08-18 21:09', result: '成功' },
  { id: 5, operator: 'admin', action: '关闭实例', target: 'host-0075', time: '2026-08-18 19:55', result: '失败' },
]

const logColumns: PrimaryTableCol<LogRow>[] = [
  { colKey: 'id', title: 'ID', width: 60 },
  { colKey: 'operator', title: '操作人', width: 110 },
  { colKey: 'action', title: '操作', ellipsis: true },
  { colKey: 'target', title: '对象', ellipsis: true },
  { colKey: 'time', title: '时间', width: 170 },
  { colKey: 'result', title: '结果', width: 90 },
]

type RowClassNameCtx = { row: LogRow }

function resolveLogRowClass({ row }: RowClassNameCtx) {
  return (row as LogRow).result === '失败' ? 'log-row log-row--danger' : 'log-row'
}

onMounted(() => {
  // Reserved: attach live refresh ticker once backed by realtime source.
})
</script>

<style scoped lang="css">
.dashboard-page {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  isolation: isolate;
  padding: 2px 2px 16px;
}

/* Shared surface card — flat, no glass */
.surface-card {
  position: relative;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-xs);
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.surface-card:hover {
  box-shadow: var(--hs-shadow-sm);
}

/* Hero section */
.dashboard-hero {
  padding: 18px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  overflow: hidden;
}

.hero-content {
  flex: 1 1 380px;
  z-index: 1;
}

.hero-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: var(--hs-radius-xl);
  background: #ecfdf5;
  border: 1px solid #bbf7d0;
  margin-bottom: 10px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #16a34a;
  box-shadow: 0 0 0 0 rgba(22, 163, 74, 0.5);
  animation: statusPulse 2.4s ease-in-out infinite;
}

@keyframes statusPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(22, 163, 74, 0.5); }
  70% { box-shadow: 0 0 0 6px rgba(22, 163, 74, 0); }
}

.hero-label {
  font-family: var(--hs-font-mono);
  font-size: 11px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: #15803d;
}

.hero-title {
  margin: 0 0 6px;
  font-family: var(--hs-font-heading);
  font-size: clamp(20px, 2.2vw, 26px);
  font-weight: 700;
  line-height: 1.2;
  color: var(--color-foreground);
}

.hero-title--accent {
  color: #16a34a;
}

.hero-desc {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.65;
  color: var(--color-muted-foreground);
  max-width: 60ch;
}

.hero-actions {
  display: flex;
  gap: 10px;
  z-index: 1;
  flex-wrap: wrap;
}

.hero-btn {
  height: 36px;
  font-weight: 600;
  border-radius: var(--hs-radius-md);
}

.hero-btn--primary {
  background: #16a34a;
  border: 0;
  color: #ffffff;
  box-shadow: 0 3px 10px rgba(22, 163, 74, 0.22);
  transition:
    background-color var(--hs-duration-fast),
    transform var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.hero-btn--primary:hover {
  background: #15803d;
  transform: translateY(-1px);
  box-shadow: 0 5px 14px rgba(22, 163, 74, 0.3);
}

.hero-btn--outline {
  color: var(--color-foreground);
  border-color: var(--color-border);
  background: var(--hs-surface-1);
  transition:
    background-color var(--hs-duration-fast),
    border-color var(--hs-duration-fast),
    transform var(--hs-duration-fast);
}

.hero-btn--outline:hover {
  background: var(--hs-surface-3);
  border-color: #86efac;
  transform: translateY(-1px);
}

.hero-visual {
  flex: 0 0 360px;
  max-width: 420px;
}

.hero-visual__img {
  width: 100%;
  height: auto;
  display: block;
  border-radius: var(--hs-radius-md);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-sm);
  aspect-ratio: 16 / 9;
  object-fit: cover;
}

/* Stat cards grid */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stat-card {
  padding: 14px 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  opacity: 0;
  transform: translateY(8px);
  animation: cardIn 360ms var(--hs-ease-out) forwards;
}

@keyframes cardIn {
  to { opacity: 1; transform: translateY(0); }
}

.stat-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-card__title {
  color: var(--color-muted-foreground);
  font-size: 12.5px;
  font-weight: 500;
}

.stat-card__icon {
  width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  flex-shrink: 0;
  box-shadow: var(--hs-shadow-xs);
}

.stat-card__icon--blue { background: #2563eb; }
.stat-card__icon--cyan { background: #0891b2; }
.stat-card__icon--green { background: #16a34a; }
.stat-card__icon--orange { background: #d97706; }

.stat-card__main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.stat-card__value :deep(.t-statistic__content) {
  font-family: var(--hs-font-heading);
  font-size: 24px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.1;
  letter-spacing: -0.01em;
}

.stat-card__trend {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 1px 8px !important;
  font-weight: 600;
  font-size: 11px;
}

.stat-card__spark {
  margin-top: 0;
  opacity: 0.95;
}

.spark-svg {
  width: 100%;
  height: 36px;
  display: block;
}

.spark-line {
  stroke-dasharray: 600;
  stroke-dashoffset: 600;
  animation: sparkDraw 1.2s var(--hs-ease-out) 0.2s forwards;
}

.spark-area {
  opacity: 0;
  animation: sparkFade 600ms ease-out 0.4s forwards;
}

@keyframes sparkDraw { to { stroke-dashoffset: 0; } }
@keyframes sparkFade { to { opacity: 1; } }

/* Panel grid */
.panel-grid {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 12px;
}

.panel-card {
  padding: 16px 16px 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.panel-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.panel-card__title {
  margin: 0 0 2px;
  font-family: var(--hs-font-heading);
  font-size: 15px;
  font-weight: 600;
  color: var(--color-foreground);
}

.panel-card__subtitle {
  margin: 0;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.range-group :deep(.t-radio-button) {
  background: var(--hs-surface-2);
  border-color: var(--color-border);
}

.view-all-link {
  display: inline-flex;
  align-items: center;
}

/* Overview list */
.overview-list {
  display: grid;
  gap: 12px;
}

.overview-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
  opacity: 0;
  transform: translateY(6px);
  animation: cardIn 360ms var(--hs-ease-out) forwards;
}

.overview-item__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.overview-item__label {
  color: #334155;
  font-size: 12.5px;
  font-weight: 500;
}

.overview-item__value {
  font-family: var(--hs-font-mono);
  font-size: 13px;
  font-weight: 700;
  color: var(--color-foreground);
}

.overview-item__value[data-theme='primary'] { color: #2563eb; }
.overview-item__value[data-theme='success'] { color: #16a34a; }
.overview-item__value[data-theme='warning'] { color: #d97706; }
.overview-item__value[data-theme='danger'] { color: #dc2626; }

.overview-item__progress :deep(.t-progress__bar--outer) {
  background: var(--hs-surface-3);
  border-radius: var(--hs-radius-xl);
}

/* Log table */
.log-table :deep(.t-table) {
  background: transparent;
}

.log-table :deep(.t-table__body tr:hover > .t-table__td) {
  background: var(--hs-surface-2) !important;
}

.log-table :deep(.t-table__th) {
  background: var(--hs-surface-2);
  color: var(--color-muted-foreground);
  font-weight: 600;
  font-size: 12px;
  border-bottom: 1px solid var(--color-border);
}

.log-table :deep(.t-table__td) {
  color: #334155;
  border-bottom: 1px solid var(--color-border);
  font-size: 12.5px;
}

.log-row--danger :deep(.t-table__td) {
  background: #fef2f2 !important;
}

/* Responsive */
@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .panel-grid {
    grid-template-columns: 1fr;
  }
  .hero-visual {
    display: none;
  }
}

@media (max-width: 640px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
  .dashboard-hero {
    padding: 14px 12px;
  }
  .panel-card {
    padding: 14px 12px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .status-dot,
  .spark-line,
  .spark-area {
    animation: none !important;
  }
  .stat-card,
  .overview-item {
    opacity: 1 !important;
    transform: none !important;
  }
  .spark-line { stroke-dashoffset: 0 !important; }
  .spark-area { opacity: 1 !important; }
}
</style>
