<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartLineBoardIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">上游资源大盘</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="handleRefresh">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <div
        v-for="stat in stats"
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

    <section class="mid-grid">
      <div class="surface-card mid-card">
        <div class="table-card__head">
          <h3 class="card-title">各上游资源分布</h3>
          <span class="table-card__meta">按提供商聚合云主机数量</span>
        </div>
        <div class="dist-list">
          <div v-for="dist in distributions" :key="dist.name" class="dist-row">
            <div class="dist-row__head">
              <span class="dist-row__name">{{ dist.name }}</span>
              <span class="dist-row__value">{{ dist.value }}</span>
            </div>
            <div class="dist-row__track">
              <div
                class="dist-row__bar"
                :style="{ width: dist.percent + '%', background: dist.color }"
              ></div>
            </div>
          </div>
        </div>
      </div>

      <div class="surface-card mid-card">
        <div class="table-card__head">
          <h3 class="card-title">最近同步记录</h3>
          <span class="table-card__meta">近 5 条</span>
        </div>
        <t-table
          row-key="id"
          :data="recentSyncs"
          :columns="recentColumns"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #status="{ row }">
            <t-tag :theme="syncStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ syncStatusLabel(row.status) }}
            </t-tag>
          </template>
        </t-table>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">云主机资源汇总</h3>
        <span class="table-card__meta">共 {{ hostSummary.length }} 条规格（实时数据）</span>
      </div>
      <t-table
        row-key="id"
        :data="hostSummary"
        :columns="summaryColumns"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #count="{ row }">
          <span class="count-badge">{{ row.count }}</span>
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { markRaw, onMounted, ref } from 'vue'

import {
  ChartLineBoardIcon,
  CloudIcon,
  DesktopIcon,
  ErrorCircleIcon,
  RefreshIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getInstanceList, getProviderList, getSyncTaskList } from '@/api/admin'
import type { InstanceInfo, ProviderInfo, SyncTaskInfo } from '@/types/interface'

defineOptions({ name: 'ResourceDashboard' })

const loading = ref(false)
const providers = ref<ProviderInfo[]>([])
const instances = ref<InstanceInfo[]>([])
const tasks = ref<SyncTaskInfo[]>([])

const stats = ref([
  { key: 'accounts', label: '上游账户数', value: 0, icon: markRaw(CloudIcon), variant: 'green' },
  { key: 'hosts', label: '云主机总数', value: 0, icon: markRaw(DesktopIcon), variant: 'success' },
  { key: 'added', label: '同步任务数', value: 0, icon: markRaw(RefreshIcon), variant: 'orange' },
  { key: 'errors', label: '异常数', value: 0, icon: markRaw(ErrorCircleIcon), variant: 'danger' },
])

const taskTypeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]
const syncStatusOptions = [
  { label: '待执行', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

function taskTypeLabel(type: string): string {
  return taskTypeOptions.find((item) => item.value === type)?.label ?? type
}

function syncStatusLabel(status: string): string {
  return syncStatusOptions.find((item) => item.value === status)?.label ?? status
}

function syncStatusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'default'
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const DIST_COLORS = ['#16a34a', '#16a34a', '#f59e0b', '#6366f1', '#ec4899', '#dc2626']
const abnormalStatuses = new Set(['error', 'failed', 'stopped', 'deleted'])

function loadStats() {
  const failedTasks = tasks.value.filter((t) => t.status === 'failed').length
  const abnormalInstances = instances.value.filter((i) => abnormalStatuses.has(i.status)).length
  stats.value[0].value = providers.value.length
  stats.value[1].value = instances.value.length
  stats.value[2].value = tasks.value.length
  stats.value[3].value = failedTasks + abnormalInstances
}

function loadDistributions() {
  const providerName = new Map(providers.value.map((p) => [p.id, p.name]))
  const buckets = new Map<number, number>()
  for (const inst of instances.value) {
    buckets.set(inst.provider_id, (buckets.get(inst.provider_id) ?? 0) + 1)
  }
  const max = Math.max(1, ...buckets.values())
  distributions.value = [...buckets.entries()]
    .map(([id, value], idx) => ({
      name: providerName.get(id) ?? `提供商 #${id}`,
      value,
      percent: Math.round((value / max) * 100),
      color: DIST_COLORS[idx % DIST_COLORS.length],
    }))
    .sort((a, b) => b.value - a.value)
}

type RecentSyncRow = {
  id: number
  task: string
  status: string
  time: string
}

const distributions = ref<{ name: string; value: number; percent: number; color: string }[]>([])
const recentSyncs = ref<RecentSyncRow[]>([])
const recentColumns: PrimaryTableCol<RecentSyncRow>[] = [
  { colKey: 'task', title: '同步任务', minWidth: 130 },
  { colKey: 'status', title: '结果', width: 90 },
  { colKey: 'time', title: '时间', minWidth: 170 },
]

function loadRecentSyncs() {
  recentSyncs.value = tasks.value.slice(0, 5).map((t) => ({
    id: t.id,
    task: taskTypeLabel(t.task_type),
    status: t.status,
    time: formatTime(t.completed_at || t.created_at),
  }))
}

type HostSummaryRow = {
  id: number
  region: string
  spec: string
  vcpu: number
  memory: string
  disk: string
  count: number
}

const hostSummary = ref<HostSummaryRow[]>([])
const summaryColumns: PrimaryTableCol<HostSummaryRow>[] = [
  { colKey: 'region', title: '区域', width: 130 },
  { colKey: 'spec', title: '规格', minWidth: 140 },
  { colKey: 'vcpu', title: 'vCPU', width: 90, align: 'center' as const },
  { colKey: 'memory', title: '内存', width: 110, align: 'center' as const },
  { colKey: 'disk', title: '磁盘', width: 110, align: 'center' as const },
  { colKey: 'count', title: '数量', width: 100, align: 'center' as const },
]

function loadHostSummary() {
  const buckets = new Map<
    string,
    { region: string; spec: string; vcpu: number; memory: string; disk: string; count: number }
  >()
  for (const inst of instances.value) {
    const key = `${inst.region}|${inst.cpu}|${inst.memory}|${inst.disk}`
    const spec = `${inst.cpu}核${inst.memory}G`
    const existing = buckets.get(key)
    if (existing) {
      existing.count += 1
    } else {
      buckets.set(key, {
        region: inst.region || '—',
        spec,
        vcpu: inst.cpu,
        memory: `${inst.memory}GB`,
        disk: `${inst.disk}GB`,
        count: 1,
      })
    }
  }
  hostSummary.value = [...buckets.values()].map((item, idx) => ({ id: idx + 1, ...item }))
}

const pagination = ref({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
})

async function loadAll() {
  loading.value = true
  try {
    const [providerRes, instanceRes, taskRes] = await Promise.all([
      getProviderList({ page: 1, page_size: 100 }),
      getInstanceList({ page: 1, page_size: 100 }),
      getSyncTaskList({ page: 1, page_size: 100 }),
    ])
    providers.value = providerRes.items
    instances.value = instanceRes.items
    tasks.value = taskRes.items
    loadStats()
    loadDistributions()
    loadRecentSyncs()
    loadHostSummary()
    pagination.value.total = hostSummary.value.length
  } catch (error) {
    MessagePlugin.error('加载仪表盘数据失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

function handleRefresh() {
  loadAll()
}

onMounted(loadAll)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">

.mid-grid {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 16px;
  align-items: stretch;
}

.mid-card {
  padding: var(--space-lg) 20px;
}

.dist-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.dist-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dist-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
}

.dist-row__name {
  font-size: 13px;
  color: var(--color-foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dist-row__value {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.dist-row__track {
  height: 8px;
  border-radius: var(--hs-radius-xl);
  background: var(--hs-surface-3);
  overflow: hidden;
}

.dist-row__bar {
  height: 100%;
  border-radius: var(--hs-radius-xl);
  transition: width var(--hs-duration-base) var(--hs-ease-out);
}

.count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  padding: 1px 10px;
  border-radius: var(--hs-radius-xl);
  background: var(--hs-surface-3);
  color: var(--color-foreground);
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 1200px) {
  .mid-grid {
    grid-template-columns: 1fr;
  }
}

/* 移动端：汇总表列多（区域/规格/vCPU/内存/磁盘/数量），窄屏收进横向滚动，
   不把整页撑出视口；分布条与统计卡铺满可用宽度。 */
@media (max-width: 768px) {
  .stat-grid,
  .mid-grid,
  .table-card {
    max-width: 100%;
  }

  .page-body :deep(.t-table__content) {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
}
</style>
