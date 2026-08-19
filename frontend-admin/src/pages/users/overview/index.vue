<template>
  <div class="user-overview-page">
    <header
      class="overview-header surface-card"
      :style="{ '--header-bg': `url('${headerBg}')` }"
    >
      <div class="overview-header__brand">
        <div class="overview-header__badge">
          <DashboardIcon size="22" aria-hidden="true" />
        </div>
        <div class="overview-header__text">
          <div class="overview-header__title-row">
            <h2 class="overview-header__title">用户总览</h2>
            <span
              v-if="!loading && !errorMessage"
              class="overview-header__sync"
            >
              <span class="sync-dot" aria-hidden="true"></span>
              实时同步
            </span>
          </div>
          <p class="overview-header__subtitle">
            汇总平台用户状态与地域分布 · 数据同步于 {{ lastSync }}
          </p>
        </div>
      </div>
      <t-button
        theme="primary"
        variant="outline"
        size="medium"
        :loading="loading"
        class="overview-header__refresh"
        aria-label="刷新统计"
        @click="loadAll"
      >
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新数据
      </t-button>
    </header>

    <section class="stat-grid" aria-label="用户统计指标">
      <article
        v-for="(stat, idx) in statCards"
        :key="stat.key"
        class="stat-card surface-card"
        :class="`stat-card--${stat.variant}`"
        :style="{ animationDelay: `${60 + idx * 45}ms`, '--card-bg': `url('${cardBg}')` }"
        tabindex="0"
        role="button"
        :aria-label="`${stat.title}：${stat.value}，点击查看明细`"
        @click="goStat(stat)"
        @keydown.enter="goStat(stat)"
      >
        <div class="stat-card__body">
          <div class="stat-card__head">
            <div class="stat-card__icon" :class="`stat-card__icon--${stat.variant}`">
              <component :is="stat.icon" size="20" aria-hidden="true" />
            </div>
            <span class="stat-card__title">{{ stat.title }}</span>
          </div>
          <div class="stat-card__main">
            <t-statistic
              :value="stat.value"
              :decimal-places="0"
              :precision="0"
              class="stat-card__value"
            />
            <span class="stat-card__hint" :class="`stat-card__hint--${stat.variant}`">{{ stat.hint }}</span>
          </div>
          <p class="stat-card__desc">{{ stat.desc }}</p>
          <div class="stat-card__foot">
            <span class="stat-card__link">查看明细</span>
            <ArrowRightIcon size="13" class="stat-card__arrow" aria-hidden="true" />
          </div>
        </div>
      </article>
    </section>

    <section class="chart-grid">
      <article class="panel-card surface-card" aria-labelledby="status-chart-title">
        <header class="panel-card__head">
          <div>
            <h3 id="status-chart-title" class="panel-card__title">用户状态分布</h3>
            <p class="panel-card__subtitle">点击扇区可跳转对应用户列表</p>
          </div>
          <t-tag theme="primary" variant="light" size="small" shape="round">扇形图</t-tag>
        </header>
        <t-loading v-if="loading" size="small" text="加载中..." class="chart-loading" />
        <EChart
          v-else-if="!errorMessage"
          :option="statusPieOption"
          :height="300"
          class="chart-canvas"
          @click="onStatusClick"
        />
        <div v-else class="chart-empty">
          <ErrorCircleIcon size="22" aria-hidden="true" />
          <span>暂无数据</span>
        </div>
      </article>

      <article class="panel-card surface-card quick-panel" aria-labelledby="quick-entry-title">
        <header class="panel-card__head">
          <div>
            <h3 id="quick-entry-title" class="panel-card__title">快捷入口</h3>
            <p class="panel-card__subtitle">点击即可跳转，可在右侧编辑自定义入口</p>
          </div>
          <t-button variant="outline" size="small" @click="openEntryEditor">
            <template #icon>
              <EditIcon aria-hidden="true" />
            </template>
            编辑
          </t-button>
        </header>
        <div class="quick-grid">
          <button
            v-for="entry in activeEntries"
            :key="entry.key"
            type="button"
            class="quick-btn"
            @click="navigate(entry.path)"
          >
            <span class="quick-btn__icon">
              <component :is="entry.icon" size="20" aria-hidden="true" />
            </span>
            <span class="quick-btn__label">{{ entry.label }}</span>
            <ArrowRightIcon size="13" class="quick-btn__arrow" aria-hidden="true" />
          </button>
          <div v-if="!activeEntries.length" class="quick-empty">
            暂未选择快捷入口，点击右上角"编辑"添加
          </div>
        </div>
      </article>
    </section>

    <article class="panel-card surface-card" aria-labelledby="region-chart-title">
      <header class="panel-card__head">
        <div>
          <h3 id="region-chart-title" class="panel-card__title">用户地域分布</h3>
          <p class="panel-card__subtitle">点击柱条可跳转对应用户列表</p>
        </div>
        <t-tag theme="success" variant="light" size="small" shape="round">统计图</t-tag>
      </header>
      <t-loading v-if="loading" size="small" text="加载中..." class="chart-loading" />
      <EChart
        v-else-if="!errorMessage && regionItems.length"
        :option="regionBarOption"
        :height="300"
        class="chart-canvas"
        @click="onRegionClick"
      />
      <div v-else class="chart-empty">
        <ErrorCircleIcon size="22" aria-hidden="true" />
        <span>暂无数据</span>
      </div>
    </article>

    <t-dialog
      v-model:visible="editorVisible"
      header="编辑快捷入口"
      width="520px"
      :confirm-btn="{ content: '保存', loading: false }"
      :on-confirm="saveEntries"
    >
      <p class="editor-tip">从下方候选中选择常用功能（最多 8 项），将展示在总览页快捷入口区。</p>
      <t-checkbox-group v-model="draftKeys" class="editor-group">
        <div v-for="c in entryCandidates" :key="c.key" class="editor-item">
          <t-checkbox :value="c.key" :disabled="false">
            <template #default>
              <span class="editor-item__inner">
                <component :is="c.icon" size="16" aria-hidden="true" />
                <span>{{ c.label }}</span>
              </span>
            </template>
          </t-checkbox>
        </div>
      </t-checkbox-group>
    </t-dialog>

    <div v-if="errorMessage" class="error-banner surface-card" role="alert">
      <ErrorCircleIcon size="16" aria-hidden="true" />
      <span class="error-banner__text">{{ errorMessage }}</span>
      <t-link theme="primary" size="small" @click="loadAll">重试</t-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { EChartsOption } from 'echarts'

import {
  ArrowRightIcon,
  DashboardIcon,
  EditIcon,
  ErrorCircleIcon,
  MenuIcon,
  RefreshIcon,
  UserAddIcon,
  UserArrowUpIcon,
  UserIcon,
  UserListIcon,
  UserLockedIcon,
  UserSafetyIcon,
  UserUnknownIcon,
} from 'tdesign-icons-vue-next'

import EChart from '@/components/EChart.vue'
import { getRegionStats, getUserStats, type RegionStatItem, type UserStatsResponse } from '@/api/user'

defineOptions({ name: 'UserOverview' })

const router = useRouter()

const headerBg =
  'https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=' +
  encodeURIComponent(
    'light minimal abstract SaaS dashboard banner, soft white and pale green gradient, subtle geometric grid lines, professional, very low contrast, no text, clean',
  ) +
  '&image_size=landscape_16_9'

const cardBg =
  'https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=' +
  encodeURIComponent(
    'realistic photograph of a modern data center server room, rows of server racks with soft blue and green indicator lights, shallow depth of field, bright clean professional lighting, no people, no text',
  ) +
  '&image_size=landscape_4_3'

type StatVariant = 'blue' | 'green' | 'cyan' | 'orange' | 'purple' | 'warning'

interface StatCardItem {
  key: keyof UserStatsResponse
  title: string
  value: number
  variant: StatVariant
  hint: string
  desc: string
  icon: unknown
  filter: Record<string, string>
}

const loading = ref(false)
const errorMessage = ref('')
const stats = ref<UserStatsResponse>({
  total: 0,
  today_new: 0,
  active: 0,
  disabled: 0,
  pending_real_name: 0,
  pending_review: 0,
})
const regionItems = ref<RegionStatItem[]>([])
const lastSync = ref('--:--:--')

const listPath = '/users/accounts/list'

const statCards = computed<StatCardItem[]>(() => {
  const items: StatCardItem[] = [
    {
      key: 'total',
      title: '总用户数',
      value: stats.value.total,
      variant: 'blue',
      hint: '累计',
      desc: '平台全部注册用户总数',
      icon: UserIcon,
      filter: {},
    },
    {
      key: 'today_new',
      title: '今日新增',
      value: stats.value.today_new,
      variant: 'green',
      hint: '今日',
      desc: '当日新增注册用户数量',
      icon: UserAddIcon,
      filter: { filter: 'today' },
    },
    {
      key: 'active',
      title: '活跃用户',
      value: stats.value.active,
      variant: 'cyan',
      hint: '正常',
      desc: '状态为正常可登录的账号',
      icon: UserArrowUpIcon,
      filter: { status: 'active' },
    },
    {
      key: 'disabled',
      title: '冻结用户',
      value: stats.value.disabled,
      variant: 'orange',
      hint: '冻结',
      desc: '因风控或违规被冻结的账号',
      icon: UserLockedIcon,
      filter: { status: 'disabled' },
    },
    {
      key: 'pending_real_name',
      title: '待实名',
      value: stats.value.pending_real_name,
      variant: 'warning',
      hint: '待认证',
      desc: '正常账号中尚未实名认证',
      icon: UserSafetyIcon,
      filter: { filter: 'pending_real_name' },
    },
    {
      key: 'pending_review',
      title: '待审核',
      value: stats.value.pending_review,
      variant: 'purple',
      hint: '待审核',
      desc: '等待管理员审核的新注册账号',
      icon: UserUnknownIcon,
      filter: { status: 'pending' },
    },
  ]
  return items
})

const statusPieData = computed(() => [
  { name: '正常账号', value: stats.value.active, itemStyle: { color: '#16a34a' } },
  { name: '冻结账号', value: stats.value.disabled, itemStyle: { color: '#d97706' } },
  { name: '待审核', value: stats.value.pending_review, itemStyle: { color: '#dc2626' } },
])

const statusPieOption = computed<EChartsOption>(() => ({
  tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
  legend: { bottom: 0, icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 } },
  series: [
    {
      name: '用户状态',
      type: 'pie',
      radius: ['46%', '72%'],
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

const regionBarOption = computed<EChartsOption>(() => {
  const items = regionItems.value
  const regions = items.map((i) => i.region)
  const counts = items.map((i) => i.count)
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 8, right: 24, top: 10, bottom: 8, containLabel: true },
    xAxis: { type: 'value', axisLabel: { color: '#94a3b8' }, splitLine: { lineStyle: { color: '#eef2f7' } } },
    yAxis: {
      type: 'category',
      data: regions.slice().reverse(),
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: '#475569', fontSize: 12 },
    },
    series: [
      {
        type: 'bar',
        data: counts.slice().reverse(),
        barWidth: 14,
        itemStyle: {
          borderRadius: [0, 6, 6, 0],
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 1, y2: 0,
            colorStops: [
              { offset: 0, color: '#86efac' },
              { offset: 1, color: '#16a34a' },
            ],
          },
        },
        emphasis: { itemStyle: { color: '#15803d' } },
        label: { show: true, position: 'right', color: '#475569', fontSize: 12 },
      },
    ],
  }
})

interface QuickCandidate {
  key: string
  label: string
  path: string
  icon: unknown
}

const entryCandidates: QuickCandidate[] = [
  { key: 'users-overview', label: '用户总览', path: '/users/overview', icon: DashboardIcon },
  { key: 'users-list', label: '用户列表', path: listPath, icon: UserListIcon },
  { key: 'real-name', label: '待实名', path: `${listPath}?filter=pending_real_name`, icon: UserSafetyIcon },
  { key: 'pending', label: '待审核', path: `${listPath}?status=pending`, icon: UserUnknownIcon },
  { key: 'disabled', label: '冻结用户', path: `${listPath}?status=disabled`, icon: UserLockedIcon },
  { key: 'today-new', label: '今日新增', path: `${listPath}?filter=today`, icon: UserAddIcon },
  { key: 'dashboard', label: '仪表盘', path: '/dashboard/base', icon: DashboardIcon },
  { key: 'menus', label: '菜单管理', path: '/system/menus', icon: MenuIcon },
]

const QUICK_STORAGE_KEY = 'hostsent_admin_quick_entries'
const DEFAULT_QUICK_KEYS = ['users-list', 'real-name', 'pending', 'dashboard']

function readStoredKeys(): string[] {
  try {
    const raw = localStorage.getItem(QUICK_STORAGE_KEY)
    if (!raw) return DEFAULT_QUICK_KEYS.slice()
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return DEFAULT_QUICK_KEYS.slice()
    return parsed.filter((k): k is string => typeof k === 'string')
  } catch {
    return DEFAULT_QUICK_KEYS.slice()
  }
}

const selectedKeys = ref<string[]>(readStoredKeys())
const editorVisible = ref(false)
const draftKeys = ref<string[]>([])

const activeEntries = computed<QuickCandidate[]>(() =>
  selectedKeys.value
    .map((k) => entryCandidates.find((c) => c.key === k))
    .filter((c): c is QuickCandidate => !!c),
)

function openEntryEditor() {
  draftKeys.value = selectedKeys.value.slice()
  editorVisible.value = true
}

function saveEntries() {
  const trimmed = draftKeys.value.slice(0, 8)
  selectedKeys.value = trimmed
  try {
    localStorage.setItem(QUICK_STORAGE_KEY, JSON.stringify(trimmed))
  } catch {
    /* ignore storage errors */
  }
  editorVisible.value = false
}

function navigate(path: string) {
  const [pathname, search] = path.split('?')
  const query: Record<string, string> = {}
  if (search) {
    new URLSearchParams(search).forEach((v, k) => {
      query[k] = v
    })
  }
  navigateRaw(pathname, query)
}

function goStat(stat: StatCardItem) {
  navigateRaw(listPath, stat.filter)
}

function navigateRaw(pathname: string, query: Record<string, string>) {
  router.push({ path: pathname, query })
}

function onStatusClick(payload: { name: string; seriesType?: string }) {
  if (payload.seriesType !== 'pie') return
  const map: Record<string, Record<string, string>> = {
    '正常账号': { status: 'active' },
    '冻结账号': { status: 'disabled' },
    '待审核': { status: 'pending' },
  }
  const filter = map[payload.name]
  if (filter) void navigateRaw(listPath, filter)
}

function onRegionClick(payload: { name: string; seriesType?: string }) {
  if (payload.seriesType !== 'bar') return
  if (payload.name) void navigateRaw(listPath, { region: payload.name })
}

function markSync() {
  const d = new Date()
  lastSync.value = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

async function loadAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [s, r] = await Promise.all([getUserStats(), getRegionStats()])
    stats.value = {
      total: s.total ?? 0,
      today_new: s.today_new ?? 0,
      active: s.active ?? 0,
      disabled: s.disabled ?? 0,
      pending_real_name: s.pending_real_name ?? 0,
      pending_review: s.pending_review ?? 0,
    }
    regionItems.value = r.items ?? []
    markSync()
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : '获取用户统计失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadAll()
})
</script>

<style scoped lang="css">
.user-overview-page {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  isolation: isolate;
  padding: 2px 2px 16px;
}

/* Shared surface card — flat, no shadow, no glass */
.surface-card {
  position: relative;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: none;
  transition:
    border-color var(--hs-duration-fast),
    transform var(--hs-duration-fast);
}

.surface-card:hover {
  box-shadow: none;
}

/* Page header — banner with faint background image */
.overview-header {
  position: relative;
  padding: 20px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  overflow: hidden;
  border-color: #d1fae5;
}

.overview-header::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: var(--header-bg);
  background-size: cover;
  background-position: center;
  opacity: 0.22;
  z-index: 0;
  pointer-events: none;
}

.overview-header::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, rgba(255, 255, 255, 0.82) 0%, rgba(255, 255, 255, 0.6) 60%, rgba(255, 255, 255, 0.7) 100%);
  z-index: 0;
  pointer-events: none;
}

.overview-header > * {
  position: relative;
  z-index: 1;
}

.overview-header__brand {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.overview-header__badge {
  width: 46px;
  height: 46px;
  border-radius: var(--hs-radius-md);
  background: linear-gradient(135deg, #22c55e, #16a34a);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: none;
}

.overview-header__text {
  min-width: 0;
}

.overview-header__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.overview-header__title {
  margin: 0 0 2px;
  font-family: var(--hs-font-heading);
  font-size: 22px;
  font-weight: 800;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
}

.overview-header__sync {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: var(--hs-radius-xl);
  background: #ecfdf5;
  border: 1px solid #bbf7d0;
  color: #15803d;
  font-size: 11px;
  font-weight: 600;
}

.overview-header__subtitle {
  margin: 0;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.overview-header__refresh {
  border-radius: var(--hs-radius-md);
  flex-shrink: 0;
}

.sync-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #16a34a;
  display: inline-block;
  margin-right: 4px;
  animation: syncPulse 2.4s ease-in-out infinite;
}

@keyframes syncPulse {
  0%, 100% { opacity: 1; }
  70% { opacity: 0.4; }
}

/* Stat cards grid */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.stat-card {
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  opacity: 0;
  transform: translateY(8px);
  animation: cardIn 360ms var(--hs-ease-out) forwards;
  outline: none;
}

.stat-card:focus-visible {
  border-color: #16a34a;
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.18);
}

.stat-card:hover {
  border-color: #bbf7d0;
  box-shadow: none;
}

@keyframes cardIn {
  to { opacity: 1; transform: translateY(0); }
}

.stat-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: var(--card-bg);
  background-size: cover;
  background-position: center;
  opacity: 0.2;
  z-index: 0;
  pointer-events: none;
  transition: opacity var(--hs-duration-fast);
}

.stat-card:hover::before {
  opacity: 0.28;
}

.stat-card__body {
  position: relative;
  z-index: 1;
  padding: 18px 20px 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.stat-card__head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-card__icon {
  width: 42px;
  height: 42px;
  border-radius: var(--hs-radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  flex-shrink: 0;
  box-shadow: none;
}

.stat-card__icon--blue { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.stat-card__icon--green { background: linear-gradient(135deg, #22c55e, #16a34a); }
.stat-card__icon--cyan { background: linear-gradient(135deg, #06b6d4, #0891b2); }
.stat-card__icon--orange { background: linear-gradient(135deg, #f59e0b, #d97706); }
.stat-card__icon--warning { background: linear-gradient(135deg, #f59e0b, #d97706); }
.stat-card__icon--purple { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }

.stat-card__title {
  color: var(--color-foreground);
  font-size: 14.5px;
  font-weight: 600;
}

.stat-card__main {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-top: 2px;
}

.stat-card__value :deep(.t-statistic__content) {
  font-family: var(--hs-font-heading);
  font-size: 34px;
  font-weight: 800;
  color: var(--color-foreground);
  line-height: 1;
  letter-spacing: -0.02em;
}

.stat-card__hint {
  font-family: var(--hs-font-mono);
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: var(--hs-radius-sm);
  background: var(--hs-surface-3);
  color: var(--color-muted-foreground);
}

.stat-card__hint--blue { color: #2563eb; background: #eff6ff; }
.stat-card__hint--green { color: #16a34a; background: #ecfdf5; }
.stat-card__hint--cyan { color: #0891b2; background: #ecfeff; }
.stat-card__hint--orange { color: #d97706; background: #fffbeb; }
.stat-card__hint--warning { color: #d97706; background: #fffbeb; }
.stat-card__hint--purple { color: #7c3aed; background: #f5f3ff; }

.stat-card__desc {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
  line-height: 1.5;
}

.stat-card__foot {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #16a34a;
  font-size: 12px;
  font-weight: 600;
}

.stat-card__arrow {
  transition: transform var(--hs-duration-fast);
}

.stat-card:hover .stat-card__arrow {
  transform: translateX(3px);
}

/* Chart grid — 两列：用户状态扇形图（左）+ 快捷入口（右），地域分布下移满宽 */
.chart-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 12px;
}

.panel-card {
  padding: 16px 16px 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 360px;
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

.chart-loading {
  display: flex;
  justify-content: center;
  padding: 60px 0;
}

.chart-canvas {
  flex: 1;
}

.chart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex: 1;
  color: var(--color-muted-foreground);
  font-size: 13px;
  padding: 40px 0;
}

/* Quick entries */
.quick-panel {
  padding: 16px 16px 16px;
}

.quick-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.quick-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  cursor: pointer;
  font: inherit;
  color: var(--color-foreground);
  transition:
    background-color var(--hs-duration-fast),
    border-color var(--hs-duration-fast),
    transform var(--hs-duration-fast);
  text-align: left;
}

.quick-btn:hover {
  background: #ecfdf5;
  border-color: #86efac;
  transform: translateY(-1px);
}

.quick-btn__icon {
  width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-sm);
  background: #ffffff;
  border: 1px solid var(--color-border);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #16a34a;
  flex-shrink: 0;
}

.quick-btn__label {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
}

.quick-btn__arrow {
  color: var(--color-muted-foreground);
  transition: transform var(--hs-duration-fast);
}

.quick-btn:hover .quick-btn__arrow {
  transform: translateX(3px);
  color: #16a34a;
}

.quick-empty {
  grid-column: 1 / -1;
  text-align: center;
  font-size: 13px;
  color: var(--color-muted-foreground);
  padding: 18px 0;
}

/* Editor dialog */
.editor-tip {
  margin: 0 0 12px;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.editor-group {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 16px;
}

.editor-item {
  padding: 6px 8px;
  border-radius: var(--hs-radius-sm);
}

.editor-item:hover {
  background: var(--hs-surface-3);
}

.editor-item__inner {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

/* Error banner */
.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  color: #b91c1c;
  font-size: 12.5px;
}

.error-banner__text {
  flex: 1;
}

/* Responsive */
@media (max-width: 1100px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .chart-grid {
    grid-template-columns: 1fr;
  }
  .quick-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
  .quick-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .editor-group {
    grid-template-columns: 1fr;
  }
  .overview-header,
  .panel-card {
    padding: 14px 12px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .stat-card,
  .sync-dot {
    animation: none !important;
    opacity: 1 !important;
    transform: none !important;
  }
}
</style>
