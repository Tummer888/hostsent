<template>
  <div class="user-overview-page">
    <!-- 面包屑：与参考稿一致，标出当前页在「用户管理」下的位置 -->
    <nav class="overview-breadcrumb" aria-label="面包屑">
      <span class="overview-breadcrumb__item">首页</span>
      <span class="overview-breadcrumb__sep">/</span>
      <span class="overview-breadcrumb__item">用户管理</span>
      <span class="overview-breadcrumb__sep">/</span>
      <span class="overview-breadcrumb__item overview-breadcrumb__item--current">用户总览</span>
    </nav>

    <header class="overview-header surface-card">
      <div class="overview-header__brand">
        <div class="overview-header__badge">
          <UserIcon size="22" aria-hidden="true" />
        </div>
        <div class="overview-header__text">
          <h2 class="overview-header__title">用户总览</h2>
        </div>
      </div>
      <t-button
        theme="default"
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
        刷新
      </t-button>
    </header>

    <section class="stat-grid" aria-label="用户统计指标">
      <article
        v-for="(stat, idx) in statCards"
        :key="stat.key"
        class="stat-card surface-card"
        :class="[`stat-card--${stat.variant}`, { 'stat-card--clickable': hasFilter(stat) }]"
        :style="{ animationDelay: `${60 + idx * 45}ms` }"
        :title="stat.desc"
        :role="hasFilter(stat) ? 'button' : undefined"
        :tabindex="hasFilter(stat) ? 0 : undefined"
        @click="goStat(stat)"
        @keydown.enter="goStat(stat)"
        @keydown.space.prevent="goStat(stat)"
      >
        <span class="stat-card__icon">
          <component :is="stat.icon" size="20" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatStatValue(stat) }}</span>
          <span class="stat-card__label">{{ stat.title }}</span>
        </div>
      </article>
    </section>

    <section class="chart-grid">
      <article class="panel-card surface-card" aria-labelledby="status-chart-title">
        <header class="panel-card__head">
          <div>
            <h3 id="status-chart-title" class="panel-card__title">用户状态分布</h3>
          </div>
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
            :style="{ '--entry-accent': `var(--uo-${entry.accent})` }"
            @click="navigate(entry.path)"
          >
            <span class="quick-btn__icon">
              <component :is="entry.icon" size="18" aria-hidden="true" />
            </span>
            <span class="quick-btn__label">{{ entry.label }}</span>
            <ArrowRightIcon size="14" class="quick-btn__arrow" aria-hidden="true" />
          </button>
          <div v-if="!activeEntries.length" class="quick-empty">暂未选择快捷入口，点击右上角"编辑"添加</div>
        </div>
      </article>
    </section>

    <!-- 在线用户 / 最近登录用户（替代原「登录 IP 归属地分布」）。
         归属地依赖外部 IP 反查服务，同一 IP 在不同时间可能得到不同结果，
         作为统计维度不可复现；在线态与最近登录都是平台自己写下的事实。 -->
    <section class="activity-grid" aria-label="用户活动统计">
      <article class="panel-card surface-card" aria-labelledby="online-title">
        <header class="panel-card__head">
          <div>
            <h3 id="online-title" class="panel-card__title">在线用户</h3>
            <p class="panel-card__sub">当前有效登录会话，按最近活跃排序</p>
          </div>
          <span class="panel-card__badge">{{ activity.online_total }} 个会话</span>
        </header>
        <t-loading v-if="loading" size="small" text="加载中..." class="chart-loading" />
        <div v-else-if="activity.online_users.length" class="activity-list">
          <div
            v-for="row in activity.online_users"
            :key="row.session_id"
            class="activity-row activity-row--clickable"
            role="button"
            tabindex="0"
            :title="`查看 ${row.username} 的详情`"
            @click="goUser(row.user_id)"
            @keydown.enter="goUser(row.user_id)"
          >
            <div class="activity-row__main">
              <span class="activity-row__name">{{ row.username }}</span>
              <span class="activity-row__meta">
                {{ platformLabel(row.platform) }} · {{ formatDateTime(row.last_active_at) }}
              </span>
            </div>
            <span class="activity-row__ip" :title="row.ip || '未记录'">{{ row.ip || '未记录' }}</span>
          </div>
        </div>
        <div v-else class="chart-empty">
          <ErrorCircleIcon size="22" aria-hidden="true" />
          <span>当前没有在线会话</span>
        </div>
      </article>

      <article class="panel-card surface-card" aria-labelledby="recent-login-title">
        <header class="panel-card__head">
          <div>
            <h3 id="recent-login-title" class="panel-card__title">最近登录用户</h3>
            <p class="panel-card__sub">最近 {{ activity.recent_window_hours }} 小时内有成功登录</p>
          </div>
          <span class="panel-card__badge">{{ activity.recent_total }} 人</span>
        </header>
        <t-loading v-if="loading" size="small" text="加载中..." class="chart-loading" />
        <div v-else-if="activity.recent_users.length" class="activity-list">
          <div
            v-for="row in activity.recent_users"
            :key="row.user_id"
            class="activity-row activity-row--clickable"
            role="button"
            tabindex="0"
            :title="`查看 ${row.username} 的详情`"
            @click="goUser(row.user_id)"
            @keydown.enter="goUser(row.user_id)"
          >
            <div class="activity-row__main">
              <span class="activity-row__name">{{ row.username }}</span>
              <span class="activity-row__meta">{{ formatDateTime(row.last_login_at) }}</span>
            </div>
            <span class="activity-row__ip" :title="row.ip || '未记录'">{{ row.ip || '未记录' }}</span>
          </div>
        </div>
        <div v-else class="chart-empty">
          <ErrorCircleIcon size="22" aria-hidden="true" />
          <span>该时段内没有登录记录</span>
        </div>
      </article>
    </section>

    <t-dialog
      v-model:visible="editorVisible"
      header="编辑快捷入口"
      width="520px"
      :confirm-btn="{ content: '保存', loading: false }"
      :on-confirm="saveEntries"
    >
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
  BillIcon,
  CloseCircleIcon,
  DashboardIcon,
  DeleteIcon,
  EditIcon,
  ErrorCircleIcon,
  RefreshIcon,
  SnowflakeIcon,
  UserAddIcon,
  UserArrowUpIcon,
  UserIcon,
  UserListIcon,
  UserSafetyIcon,
  UserUnknownIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'

import EChart from '@/components/EChart.vue'
import { useSettingsStore } from '@/store/modules/settings'
import { formatDateTime } from '../constants'
import {
  getActivityOverview,
  getUserStats,
  type UserActivityOverviewResponse,
  type UserStatsResponse,
} from '@/api/user'

defineOptions({ name: 'UserOverview' })

const router = useRouter()
const settings = useSettingsStore()

type StatVariant = 'blue' | 'violet' | 'green' | 'cyan' | 'orange' | 'red' | 'slate'

interface StatCardItem {
  key: keyof UserStatsResponse
  title: string
  variant: StatVariant
  // desc 作为卡片 title 提示；filter 是点击卡片后跳转用户列表的查询参数。
  desc: string
  icon: unknown
  filter: Record<string, string>
  displayValue: number
  decimalPlaces: number
}

function formatStatValue(stat: StatCardItem) {
  const n = Number(stat.displayValue ?? 0)
  return n.toLocaleString('zh-CN', {
    minimumFractionDigits: stat.decimalPlaces,
    maximumFractionDigits: stat.decimalPlaces,
  })
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
  total_balance: 0,
  purchased_count: 0,
  deleted: 0,
})
const activity = ref<UserActivityOverviewResponse>({
  online_total: 0,
  online_users: [],
  recent_total: 0,
  recent_users: [],
  recent_window_hours: 24,
})

const listPath = '/users/accounts/list'

// 卡片图标配色对齐参考稿：每个指标一个高饱和实色块，
// 同族指标（今日新增/活跃用户=绿，待实名=橙，待审核=紫）复用同一色，
// 「已注销」用中性灰表示已归档的终态。
const statCards = computed<StatCardItem[]>(() => {
  const items: StatCardItem[] = [
    // Row 1
    {
      key: 'total_balance',
      title: '用户总余额',
      variant: 'blue',
      desc: '平台全部用户账户余额总和',
      icon: WalletIcon,
      filter: {},
      displayValue: stats.value.total_balance,
      decimalPlaces: 2,
    },
    {
      key: 'total',
      title: '总用户数',
      variant: 'violet',
      desc: '平台全部注册用户总数',
      icon: UserIcon,
      filter: {},
      displayValue: stats.value.total,
      decimalPlaces: 0,
    },
    {
      key: 'today_new',
      title: '今日新增',
      variant: 'green',
      desc: '当日新增注册用户数量',
      icon: UserAddIcon,
      filter: { filter: 'today' },
      displayValue: stats.value.today_new,
      decimalPlaces: 0,
    },
    {
      key: 'purchased_count',
      title: '已购用户',
      variant: 'cyan',
      desc: '至少有一条订单的用户数量',
      icon: BillIcon,
      filter: {},
      displayValue: stats.value.purchased_count,
      decimalPlaces: 0,
    },
    // Row 2
    {
      key: 'pending_real_name',
      title: '待实名',
      variant: 'orange',
      desc: '尚未实名认证的账号（real_name_verified_at 为空）',
      icon: UserSafetyIcon,
      filter: { filter: 'pending_real_name' },
      displayValue: stats.value.pending_real_name,
      decimalPlaces: 0,
    },
    {
      key: 'pending_review',
      title: '待审核',
      variant: 'violet',
      desc: '等待管理员审核的新注册账号',
      icon: UserUnknownIcon,
      filter: { status: 'pending' },
      displayValue: stats.value.pending_review,
      decimalPlaces: 0,
    },
    {
      key: 'disabled',
      title: '冻结用户',
      variant: 'red',
      desc: '因风控或违规被冻结的账号',
      icon: SnowflakeIcon,
      filter: { status: 'disabled' },
      displayValue: stats.value.disabled,
      decimalPlaces: 0,
    },
    {
      key: 'deleted',
      title: '已注销',
      variant: 'slate',
      desc: '留存期内可从回收站恢复的账号',
      icon: CloseCircleIcon,
      filter: { filter: 'deleted' },
      displayValue: stats.value.deleted,
      decimalPlaces: 0,
    },
    {
      key: 'active',
      title: '活跃用户',
      variant: 'green',
      desc: '状态为正常可登录的账号',
      icon: UserArrowUpIcon,
      filter: { status: 'active' },
      displayValue: stats.value.active,
      decimalPlaces: 0,
    },
  ]
  return items
})

// 环形图配色与统计卡同族：正常=绿、冻结=红、待审核=蓝，
// 点图例能到对应卡片找到同一个颜色。ECharts 走 canvas 渲染，
// 不解析 CSS 变量，因此这里按深浅色给出两套实色。
const chartPalette = computed(() =>
  settings.isDark
    ? { normal: '#4ade80', frozen: '#f87171', pending: '#60a5fa', label: '#cbd5e1', legend: '#94a3b8', gap: '#0a0a0a' }
    : { normal: '#22c55e', frozen: '#ef4444', pending: '#3b82f6', label: '#334155', legend: '#64748b', gap: '#ffffff' },
)

const statusTotal = computed(() => stats.value.active + stats.value.disabled + stats.value.pending_review)

const statusPieData = computed(() => {
  const c = chartPalette.value
  return [
    { name: '正常账号', value: stats.value.active, itemStyle: { color: c.normal } },
    { name: '冻结账号', value: stats.value.disabled, itemStyle: { color: c.frozen } },
    { name: '待审核', value: stats.value.pending_review, itemStyle: { color: c.pending } },
  ]
})

const statusPieOption = computed<EChartsOption>(() => {
  const c = chartPalette.value
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, icon: 'circle', textStyle: { color: c.legend, fontSize: 12 } },
    series: [
      {
        name: '用户状态',
        type: 'pie',
        radius: ['52%', '74%'],
        center: ['50%', '44%'],
        avoidLabelOverlap: true,
        itemStyle: { borderRadius: 6, borderColor: c.gap, borderWidth: 2 },
        label: { show: true, formatter: '{b}\n{c}', fontSize: 12, color: c.label },
        emphasis: {
          label: { show: true, fontSize: 14, fontWeight: 'bold' },
          itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.12)' },
        },
        // 环心留白放「总计」，参考稿把总数放在这里，比只在图例里体现更醒目。
        data: statusPieData.value,
      },
    ],
    // graphic 而不是 title：title 会参与布局并挤压画布，graphic 直接定位在环心。
    graphic: [
      {
        type: 'text',
        left: 'center',
        top: '38%',
        silent: true,
        style: { text: '总计', fill: c.legend, fontSize: 12, textAlign: 'center' },
      },
      {
        type: 'text',
        left: 'center',
        top: '44%',
        silent: true,
        style: {
          text: String(statusTotal.value),
          fill: c.label,
          fontSize: 22,
          fontWeight: 700,
          textAlign: 'center',
        },
      },
    ],
  }
})

interface QuickCandidate {
  key: string
  label: string
  path: string
  icon: unknown
  accent: string
}

// accent 取色板名（blue/green/...），与统计卡同族：同一个业务对象
// （如「冻结用户」）在卡片和入口里用同一种颜色，扫视时能对上号。
const entryCandidates: QuickCandidate[] = [
  // dashboard 在 DEFAULT_QUICK_KEYS 里，此前却不在候选表中 —— 默认快捷入口
  // 里那一个永远被 activeEntries 过滤掉，用户看到的是三个而不是四个（doc104 §3.3，F23）。
  { key: 'dashboard', label: '仪表盘', path: '/dashboard/base', icon: DashboardIcon, accent: 'blue' },
  { key: 'users-list', label: '用户列表', path: listPath, icon: UserListIcon, accent: 'violet' },
  {
    key: 'real-name',
    label: '待实名',
    path: `${listPath}?filter=pending_real_name`,
    icon: UserSafetyIcon,
    accent: 'orange',
  },
  { key: 'pending', label: '待审核', path: `${listPath}?status=pending`, icon: UserUnknownIcon, accent: 'violet' },
  { key: 'today-new', label: '今日新增', path: `${listPath}?filter=today`, icon: UserAddIcon, accent: 'green' },
  { key: 'purchased', label: '已购用户', path: `${listPath}?filter=purchased`, icon: BillIcon, accent: 'cyan' },
  {
    key: 'disabled',
    label: '冻结用户',
    path: `${listPath}?status=disabled`,
    icon: SnowflakeIcon,
    accent: 'red',
  },
  { key: 'recycle', label: '回收站', path: `${listPath}?filter=deleted`, icon: DeleteIcon, accent: 'slate' },
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
  selectedKeys.value.map((k) => entryCandidates.find((c) => c.key === k)).filter((c): c is QuickCandidate => !!c),
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

function navigateRaw(pathname: string, query: Record<string, string>) {
  router.push({ path: pathname, query })
}

// 统计卡点击 → 带筛选跳用户列表。此前卡片没有点击事件（goStat 是死代码），
// 而 filter 字段已经在数据里定义好了（doc104 §3.3，F23）。
function hasFilter(stat: StatCardItem) {
  return Object.keys(stat.filter).length > 0
}

function goStat(stat: StatCardItem) {
  if (!hasFilter(stat)) return
  navigateRaw(listPath, stat.filter)
}

function onStatusClick(payload: { name: string; seriesType?: string }) {
  if (payload.seriesType !== 'pie') return
  const map: Record<string, Record<string, string>> = {
    正常账号: { status: 'active' },
    冻结账号: { status: 'disabled' },
    待审核: { status: 'pending' },
  }
  const filter = map[payload.name]
  if (filter) void navigateRaw(listPath, filter)
}

// 活动卡片点击 → 该用户详情页。此前归属地柱状图点击是跳列表并按归属地筛选，
// 现在没有归属地维度了，逐行点击直接进详情更符合「看到异常就要查这个人」的意图。
function goUser(userId: number) {
  if (!userId) return
  void router.push({ path: '/users/accounts/detail', query: { id: String(userId) } })
}

// 会话平台取值到中文标签；未在表内的原样显示（后端已出现过 admin/oauth provider 名）。
const platformLabels: Record<string, string> = {
  web: '网页',
  mobile: '手机',
  desktop: '桌面端',
  admin: '管理端代登录',
}

function platformLabel(platform: string) {
  if (!platform) return '未知来源'
  return platformLabels[platform] || platform
}

async function loadAll() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [s, a] = await Promise.all([getUserStats(), getActivityOverview()])
    stats.value = {
      total: s.total ?? 0,
      today_new: s.today_new ?? 0,
      active: s.active ?? 0,
      disabled: s.disabled ?? 0,
      pending_real_name: s.pending_real_name ?? 0,
      pending_review: s.pending_review ?? 0,
      total_balance: s.total_balance ?? 0,
      purchased_count: s.purchased_count ?? 0,
      deleted: s.deleted ?? 0,
    }
    activity.value = {
      online_total: a.online_total ?? 0,
      online_users: a.online_users ?? [],
      recent_total: a.recent_total ?? 0,
      recent_users: a.recent_users ?? [],
      recent_window_hours: a.recent_window_hours ?? 24,
    }
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
/* ============================================================
   用户总览 · 设计语言（本页为试点，后续按此推广到其他页面）
   - 主色：石墨灰（黑）——品牌、标题、导航
   - 图标：高饱和实色圆角方块 + 白色图形（对齐参考稿）
   - 卡面圆角 14px、图标底 10px
   - 数字与描述压在 2 行以内，避免「数字+描述」比图标高出一截
   ============================================================ */
.user-overview-page {
  --uo-blue: 59, 130, 246;
  --uo-violet: 124, 58, 237;
  --uo-green: 34, 197, 94;
  --uo-cyan: 6, 182, 212;
  --uo-orange: 249, 115, 22;
  --uo-red: 239, 68, 68;
  --uo-slate: 100, 116, 139;
  --uo-radius-card: 14px;
  --uo-radius-tile: 10px;
  --uo-tile-size: 38px;

  position: relative;
  display: flex;
  flex-direction: column;
  gap: 14px;
  isolation: isolate;
  padding: 2px 2px 16px;
}

html.dark .user-overview-page {
  /* 深色下整体提亮一档：原色在纯黑底上偏暗，白图形对比度不足。 */
  --uo-blue: 96, 165, 250;
  --uo-violet: 167, 139, 250;
  --uo-green: 74, 222, 128;
  --uo-cyan: 34, 211, 238;
  --uo-orange: 251, 146, 60;
  --uo-red: 248, 113, 113;
  --uo-slate: 148, 163, 184;
}

.surface-card {
  position: relative;
  border-radius: var(--uo-radius-card);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: none;
  transition: border-color var(--hs-duration-fast);
}

/* ---------- 面包屑 ---------- */
.overview-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.overview-breadcrumb__sep {
  color: var(--color-border);
}

.overview-breadcrumb__item--current {
  color: var(--color-foreground);
  font-weight: 600;
}

/* ---------- 页头 ---------- */
.overview-header {
  position: relative;
  padding: 18px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.overview-header__brand {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

/* 页头徽标用项目主色（石墨灰）实色块：与统计卡的「实色块 + 白图形」
   统一，但保持中性——它是页面标题图标，不承担指标区分职责。 */
.overview-header__badge {
  width: 44px;
  height: 44px;
  border-radius: var(--uo-radius-tile);
  background: rgb(var(--color-primary-rgb));
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.overview-header__text {
  min-width: 0;
}

.overview-header__title {
  margin: 0;
  font-family: var(--hs-font-heading);
  font-size: 21px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
}

.overview-header__refresh {
  border-radius: var(--hs-radius-md);
  flex-shrink: 0;
}

/* ---------- 统计卡 ---------- */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stat-card {
  --stat-accent: var(--uo-slate);
  display: flex;
  align-items: center;
  gap: 13px;
  min-height: 84px;
  padding: 16px 18px;
  cursor: default;
  opacity: 0;
  transform: translateY(8px);
  animation: hs-overview-card-in 360ms var(--hs-ease-out) forwards;
}

.stat-card--blue {
  --stat-accent: var(--uo-blue);
}

.stat-card--violet {
  --stat-accent: var(--uo-violet);
}

.stat-card--green {
  --stat-accent: var(--uo-green);
}

.stat-card--cyan {
  --stat-accent: var(--uo-cyan);
}

.stat-card--orange {
  --stat-accent: var(--uo-orange);
}

.stat-card--red {
  --stat-accent: var(--uo-red);
}

.stat-card--slate {
  --stat-accent: var(--uo-slate);
}

/* 实色块 + 白色图形：高饱和底色保证 9 个指标一眼可分，
   白图形在任一底色上的对比度都够（深色模式下 ---accent 已提亮）。 */
.stat-card__icon {
  width: var(--uo-tile-size);
  height: var(--uo-tile-size);
  border-radius: var(--uo-radius-tile);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #ffffff;
  background: rgb(var(--stat-accent));
}

.stat-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* 数字与描述合起来的行高控制在图标高度以内：字号 21/12、行高 1.15/1.25
   → 约 24 + 15 = 39px，与 38px 图标齐平，不再出现文本块高于图标的下坠感。 */
.stat-card__value {
  font-size: 21px;
  font-weight: 700;
  line-height: 1.15;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.stat-card__label {
  color: var(--color-muted-foreground);
  font-size: 12px;
  line-height: 1.25;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

/* 有筛选条件的卡片可点击跳列表（无 filter 的余额/总数/已购卡片不加手型与高亮）。 */
.stat-card--clickable {
  cursor: pointer;
  transition:
    border-color 160ms var(--hs-ease-out),
    box-shadow 160ms var(--hs-ease-out),
    transform 160ms var(--hs-ease-out);
}

.stat-card--clickable:hover {
  border-color: rgba(var(--stat-accent), 0.55);
  box-shadow: var(--hs-shadow-md);
  transform: translateY(-2px);
}

.stat-card--clickable:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--color-primary),
    var(--hs-shadow-md);
}

@keyframes hs-overview-card-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ---------- 面板卡 ---------- */
.chart-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 12px;
}

.panel-card {
  padding: 18px 20px 16px;
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
  font-size: 15.5px;
  font-weight: 650;
  color: var(--color-foreground);
}

.panel-card__sub {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.panel-card__badge {
  flex-shrink: 0;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--hs-surface-3);
  color: var(--color-muted-foreground);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
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

/* ---------- 快捷入口 ---------- */
.quick-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  align-content: start;
}

.quick-btn {
  --entry-accent: var(--uo-slate);
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--uo-radius-tile);
  background: var(--hs-surface-1);
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
  background: var(--hs-surface-2);
  border-color: rgba(var(--entry-accent), 0.45);
  transform: translateY(-1px);
}

/* 与统计卡同一套「实色块 + 白图形」，但尺寸小一档（30px）。 */
.quick-btn__icon {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: rgb(var(--entry-accent));
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
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
  color: rgb(var(--entry-accent));
}

.quick-empty {
  grid-column: 1 / -1;
  text-align: center;
  font-size: 13px;
  color: var(--color-muted-foreground);
  padding: 18px 0;
}

/* ---------- 在线用户 / 最近登录用户 两张并排卡片 ---------- */
.activity-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.activity-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-height: 0;
}

.activity-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 10px;
  border-radius: var(--hs-radius-sm);
  transition: background-color var(--hs-duration-fast);
}

.activity-row--clickable {
  cursor: pointer;
}

.activity-row--clickable:hover,
.activity-row--clickable:focus-visible {
  background: var(--hs-surface-2);
  outline: none;
}

.activity-row__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.activity-row__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.activity-row__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* IP 用等宽字体：这是运营要逐个字符核对的字段，比例字体下 1/l、0/O 极易看错。 */
.activity-row__ip {
  flex-shrink: 0;
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: var(--color-muted-foreground);
  max-width: 45%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------- 快捷入口编辑弹窗 ---------- */
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
  background: var(--hs-surface-2);
}

.editor-item__inner {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

/* ---------- 错误条 ---------- */
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

@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .chart-grid,
  .activity-grid {
    grid-template-columns: 1fr;
  }
  .quick-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .chart-grid,
  .activity-grid {
    grid-template-columns: 1fr;
  }
  .quick-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .stat-card {
    min-height: 76px;
    padding: 14px 16px;
    gap: 12px;
  }

  .stat-card__icon {
    --uo-tile-size: 34px;
  }

  .stat-card__value {
    font-size: 19px;
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
  .stat-card {
    animation: none !important;
    opacity: 1 !important;
    transform: none !important;
  }
}
</style>
