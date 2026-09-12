<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SwapRightIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">任务队列</h2>
          <p class="page-header__desc">
            平台发往上游的开通 / 暂停 / 续费 / 重启 / 同步等动作，是否经接口正确到达上游；按类别成队列查看
          </p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="reload">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="stat-grid">
      <article v-for="card in statCards" :key="card.key" class="stat-card surface-card" :class="`stat-card--${card.theme}`">
        <div class="stat-card__icon">
          <component :is="card.icon" size="20" aria-hidden="true" />
        </div>
        <div class="stat-card__text">
          <span class="stat-card__value">{{ card.value }}</span>
          <span class="stat-card__label">{{ card.label }}</span>
          <span v-if="card.hint" class="stat-card__hint">{{ card.hint }}</span>
        </div>
      </article>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">任务类别</h3>
        <span class="filter-card__meta">未到达上游的任务会标红，便于逐类排障</span>
      </div>
      <div class="category-bar">
        <t-radio-group v-model="filters.category" variant="default-filled" @change="handleCategoryChange">
          <t-radio-button value="">全部（{{ allTotal }}）</t-radio-button>
          <t-radio-button v-for="cat in categories" :key="cat.category" :value="cat.category">
            {{ cat.category_name }}（{{ cat.total }}）<template v-if="cat.not_reached">· 未到达 {{ cat.not_reached }}</template>
          </t-radio-button>
        </t-radio-group>
      </div>

      <div class="filter-card__head filter-card__head--sub">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="主体 / 单号 / 实例标识" @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">所属渠道</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部渠道" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">上游到达状态</span>
          <t-select v-model="filters.upstream_state" clearable placeholder="全部状态" :options="upstreamStateOptions" />
        </div>
        <div class="field">
          <span class="field__label">任务状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
        <div class="field field--wide">
          <span class="field__label">创建时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
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
        <h3 class="card-title">任务列表</h3>
        <span class="table-card__meta">共 {{ total }} 条任务</span>
      </div>
      <t-alert
        theme="info"
        message="当前「是否到达上游」依据本地任务执行结果判定；后续上游接口提供回执后，将改为按上游返回值核对。"
        class="queue-hint"
      />
      <t-table
        row-key="id"
        :data="taskList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #subject="{ row }">
          <div class="subject-cell">
            <span class="subject-name">{{ row.subject || '—' }}</span>
            <span class="subject-meta">
              <t-tag :theme="categoryTheme(row.category)" variant="light" size="small" shape="round">
                {{ row.category_name }}
              </t-tag>
              <span v-if="row.ref_no" class="subject-ref">{{ row.ref_no }}</span>
              <span v-if="row.instance_ref" class="subject-ref">{{ row.instance_ref }}</span>
            </span>
          </div>
        </template>

        <template #op="{ row }">
          <t-tag variant="outline" size="small" shape="round">{{ row.action_name || row.action }}</t-tag>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ row.status_name || row.status }}
          </t-tag>
        </template>

        <template #upstream="{ row }">
          <div class="upstream-cell">
            <t-tag :theme="upstreamTheme(row.upstream_state)" variant="light" size="small" shape="round">
              {{ row.upstream_state_name }}
            </t-tag>
            <t-tooltip v-if="row.upstream_detail" :content="row.upstream_detail" placement="top-left">
              <span class="upstream-detail">{{ row.upstream_detail }}</span>
            </t-tooltip>
          </div>
        </template>

        <template #provider="{ row }">
          <span v-if="row.provider_name" class="cell-strong">{{ row.provider_name }}</span>
          <span v-else-if="row.provider_id" class="cell-muted">渠道 #{{ row.provider_id }}</span>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #operator="{ row }">
          <span v-if="row.username">{{ row.username }}</span>
          <span v-else-if="row.user_id" class="cell-muted">用户 #{{ row.user_id }}</span>
          <span v-else class="cell-muted">系统</span>
        </template>

        <template #attempts="{ row }">
          <span v-if="row.max_attempts > 0" class="attempts-text">{{ row.attempts }} / {{ row.max_attempts }}</span>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #duration="{ row }">
          <span v-if="row.duration_ms > 0" class="time-text">{{ formatDuration(row.duration_ms) }}</span>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #row_action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([{ content: '详情', value: 'detail', theme: 'default' }])"
              @select="() => openDetailDialog(row)"
            />
            <t-link v-else theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无任务数据" />
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
      v-model:visible="detailVisible"
      header="任务详情"
      width="620px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailTask" :column="2" bordered size="small">
        <t-descriptions-item label="类别">{{ detailTask.category_name }}</t-descriptions-item>
        <t-descriptions-item label="动作">{{ detailTask.action_name || detailTask.action }}</t-descriptions-item>
        <t-descriptions-item label="任务状态">
          <t-tag :theme="statusTheme(detailTask.status)" variant="light" size="small" shape="round">
            {{ detailTask.status_name || detailTask.status }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="上游到达">
          <t-tag :theme="upstreamTheme(detailTask.upstream_state)" variant="light" size="small" shape="round">
            {{ detailTask.upstream_state_name }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="任务主体">{{ detailTask.subject || '—' }}</t-descriptions-item>
        <t-descriptions-item label="业务单号">{{ detailTask.ref_no || '—' }}</t-descriptions-item>
        <t-descriptions-item label="实例标识">{{ detailTask.instance_ref || '—' }}</t-descriptions-item>
        <t-descriptions-item label="所属渠道">
          {{ detailTask.provider_name || (detailTask.provider_id ? `渠道 #${detailTask.provider_id}` : '—') }}
        </t-descriptions-item>
        <t-descriptions-item label="操作人">{{ detailTask.username || '系统' }}</t-descriptions-item>
        <t-descriptions-item label="重试次数">
          {{ detailTask.max_attempts > 0 ? `${detailTask.attempts} / ${detailTask.max_attempts}` : '—' }}
        </t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatTime(detailTask.created_at) }}</t-descriptions-item>
        <t-descriptions-item label="完成时间">
          {{ detailTask.finished_at ? formatTime(detailTask.finished_at) : '—' }}
        </t-descriptions-item>
        <t-descriptions-item v-if="detailTask.amount > 0" label="涉及金额">￥{{ detailTask.amount.toFixed(2) }}</t-descriptions-item>
        <t-descriptions-item label="耗时">
          {{ detailTask.duration_ms > 0 ? formatDuration(detailTask.duration_ms) : '—' }}
        </t-descriptions-item>
        <t-descriptions-item label="上游细节" :span="2">
          <span v-if="detailTask.upstream_detail" class="upstream-detail upstream-detail--full">{{ detailTask.upstream_detail }}</span>
          <span v-else class="cell-muted">无</span>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import {
  CheckCircleIcon,
  ErrorCircleIcon,
  RefreshIcon,
  SearchIcon,
  SwapRightIcon,
  TimeIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderList, getTaskQueueList } from '@/api/admin'
import type { ProviderInfo, TaskQueueCategoryCount, TaskQueueItem, TaskQueueSummary } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceTaskQueue' })

const taskList = ref<TaskQueueItem[]>([])
const loading = ref(false)
const total = ref(0)
const categories = ref<TaskQueueCategoryCount[]>([])
const providerOptions = ref<{ label: string; value: number }[]>([])
const { isMobile } = useIsMobile()

const emptySummary: TaskQueueSummary = {
  total: 0,
  pending: 0,
  running: 0,
  success: 0,
  failed: 0,
  manual: 0,
  reached: 0,
  not_reached: 0,
  reached_rate: 0,
  categories: [],
}
const summary = ref<TaskQueueSummary>({ ...emptySummary })

const upstreamStateOptions = [
  { label: '已到达上游', value: 'reached' },
  { label: '未到达上游', value: 'not_reached' },
  { label: '待执行', value: 'pending' },
  { label: '无需上游', value: 'not_applicable' },
  { label: '已跳过（渠道不支持）', value: 'skipped' },
]

const statusOptions = [
  { label: '排队中', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '待人工', value: 'manual' },
  { label: '已跳过', value: 'skipped' },
  { label: '已取消', value: 'cancelled' },
]

const filters = reactive<{
  category: string
  status: string
  upstream_state: string
  provider_id: number | undefined
  keyword: string
  dateRange: (string | Date | undefined)[] | undefined
}>({
  category: '',
  status: '',
  upstream_state: '',
  provider_id: undefined,
  keyword: '',
  dateRange: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})

// 「全部」角标取各类别计数之和：summary.total 会随筛选变化，与类别页签的固定全量口径不一致。
const allTotal = computed(() => categories.value.reduce((sum, cat) => sum + (cat.total || 0), 0))

// 概览卡片：总量 / 到达率 / 未到达 / 进行中，回答「有没有正确到达上游」。
const statCards = computed(() => {
  const s = summary.value
  return [
    {
      key: 'total',
      label: '任务总数',
      value: `${s.total}`,
      hint: '当前筛选范围内',
      icon: SwapRightIcon,
      theme: 'info' as const,
    },
    {
      key: 'rate',
      label: '上游到达率',
      value: `${s.reached_rate}%`,
      hint: `已到达 ${s.reached} · 未到达 ${s.not_reached}`,
      icon: CheckCircleIcon,
      theme: s.reached_rate >= 95 ? ('success' as const) : ('warning' as const),
    },
    {
      key: 'not_reached',
      label: '未到达上游',
      value: `${s.not_reached}`,
      hint: s.manual > 0 ? `其中待人工 ${s.manual}` : '需排障核对上游接口',
      icon: ErrorCircleIcon,
      theme: s.not_reached > 0 ? ('danger' as const) : ('info' as const),
    },
    {
      key: 'inflight',
      label: '进行中',
      value: `${s.pending + s.running}`,
      hint: `排队 ${s.pending} · 执行 ${s.running}`,
      icon: TimeIcon,
      theme: s.pending + s.running > 0 ? ('warning' as const) : ('info' as const),
    },
  ]
})

const columns = computed<PrimaryTableCol<TaskQueueItem>[]>(() => {
  const base: PrimaryTableCol<TaskQueueItem>[] = [
    { colKey: 'subject', title: '任务主体', minWidth: 200 },
    { colKey: 'op', title: '动作', width: 110 },
    { colKey: 'status', title: '状态', width: 100 },
    { colKey: 'upstream', title: '上游到达', minWidth: 170 },
    { colKey: 'provider', title: '渠道', width: 130 },
    { colKey: 'operator', title: '操作人', width: 110 },
    { colKey: 'attempts', title: '重试', width: 80 },
    { colKey: 'created_at', title: '创建时间', width: 150 },
    { colKey: 'duration', title: '耗时', width: 90 },
  ]
  base.push({
    colKey: 'row_action',
    title: '操作',
    width: isMobile.value ? 70 : 80,
    fixed: 'right' as const,
    align: 'center' as const,
  })
  return base
})

function categoryTheme(category: string): 'primary' | 'warning' | 'success' | 'danger' | 'default' {
  switch (category) {
    case 'provision':
      return 'primary'
    case 'instance_action':
      return 'warning'
    case 'renewal':
      return 'success'
    case 'sync':
      return 'default'
    default:
      return 'default'
  }
}

function statusTheme(status: string): 'primary' | 'warning' | 'success' | 'danger' | 'default' {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'primary'
    case 'pending':
    case 'manual':
      return 'warning'
    default:
      return 'default'
  }
}

function upstreamTheme(state: string): 'primary' | 'warning' | 'success' | 'danger' | 'default' {
  switch (state) {
    case 'reached':
      return 'success'
    case 'not_reached':
      return 'danger'
    case 'pending':
      return 'warning'
    case 'not_applicable':
    case 'skipped':
      return 'default'
    default:
      return 'default'
  }
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}min`
}

// 日期选择器可能返回 Date 或字符串，统一为 YYYY-MM-DD 交给后端解析。
function toDateString(value: string | Date | undefined): string {
  if (!value) return ''
  if (value instanceof Date) {
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
  }
  return String(value).slice(0, 10)
}

async function loadTasks() {
  loading.value = true
  try {
    const [from, to] = filters.dateRange ?? []
    const data = await getTaskQueueList({
      category: filters.category || undefined,
      status: filters.status || undefined,
      upstream_state: filters.upstream_state || undefined,
      provider_id: filters.provider_id,
      keyword: filters.keyword || undefined,
      created_from: toDateString(from) || undefined,
      created_to: toDateString(to) || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    taskList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    summary.value = data.summary || { ...emptySummary }
    categories.value = data.summary?.categories || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载任务队列失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 渠道下拉加载失败不阻塞列表 */
  }
}

function reload() {
  pagination.current = 1
  loadTasks()
}

function handleCategoryChange() {
  pagination.current = 1
  loadTasks()
}

function handleSearch() {
  pagination.current = 1
  loadTasks()
}

function handleResetFilters() {
  filters.category = ''
  filters.status = ''
  filters.upstream_state = ''
  filters.provider_id = undefined
  filters.keyword = ''
  filters.dateRange = undefined
  pagination.current = 1
  loadTasks()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTasks()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await loadTasks()
  mobilePage.total = pagination.total
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

const detailVisible = ref(false)
const detailTask = ref<TaskQueueItem | null>(null)

function openDetailDialog(row: TaskQueueItem) {
  detailTask.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadProviders()
  loadTasks()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

.resource-module .stat-card__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.resource-module .filter-card__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.filter-card__grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.filter-card__grid .field--wide {
  grid-column: span 2;
}

@media (max-width: 1200px) and (min-width: 769px) {
  .filter-card__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-card__grid {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }

  .filter-card__grid .field--wide {
    grid-column: span 1;
  }
}

.filter-card__head--sub {
  border-top: 1px solid var(--color-border);
  margin-top: 4px;
  padding-top: 16px;
}

.category-bar {
  padding: 0 20px 4px;
}

.category-bar :deep(.t-radio-group) {
  flex-wrap: wrap;
  gap: 8px;
}

.queue-hint {
  margin: 0 20px 12px;
}

.subject-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.subject-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.subject-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.subject-ref {
  font-size: 11px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.upstream-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
  align-items: flex-start;
}

.upstream-detail {
  display: block;
  max-width: 100%;
  font-size: 11px;
  color: var(--color-muted-foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.upstream-detail--full {
  white-space: normal;
  word-break: break-all;
}

.attempts-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}
</style>
