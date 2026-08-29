<template>
  <div class="page-body">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChartLineBoardIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">上游资源大盘</h2>
          <p class="page-header__desc">聚合各上游提供商的账户、云主机与同步概况，快速掌握整体资源水位。</p>
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
      <div v-for="stat in stats" :key="stat.key" class="stat-card surface-card">
        <span class="stat-card__icon" :style="{ background: stat.bg, color: stat.color }">
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
        bordered
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

defineOptions({ name: 'UpstreamDashboard' })

const loading = ref(false)
const providers = ref<ProviderInfo[]>([])
const instances = ref<InstanceInfo[]>([])
const tasks = ref<SyncTaskInfo[]>([])

const stats = ref([
  { key: 'accounts', label: '上游账户数', value: 0, icon: markRaw(CloudIcon), bg: '#e0f2fe', color: '#0284c7' },
  { key: 'hosts', label: '云主机总数', value: 0, icon: markRaw(DesktopIcon), bg: '#dcfce7', color: '#16a34a' },
  { key: 'added', label: '同步任务数', value: 0, icon: markRaw(RefreshIcon), bg: '#fef3c7', color: '#d97706' },
  { key: 'errors', label: '异常数', value: 0, icon: markRaw(ErrorCircleIcon), bg: '#fee2e2', color: '#dc2626' },
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

const DIST_COLORS = ['#16a34a', '#0891b2', '#f59e0b', '#6366f1', '#ec4899', '#dc2626']
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

<style scoped lang="css">
.page-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}

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

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  padding: var(--space-lg) var(--space-xl);
  flex-wrap: wrap;
}

.page-header__main {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  min-width: 0;
}

.page-header__chip {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, #0891b2, #0e7490);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(8, 145, 178, 0.25);
}

.page-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.page-header__desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-lg) 20px;
}

.stat-card__icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.stat-card__value {
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.stat-card__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

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

.table-card {
  padding: var(--space-lg) 20px;
}

.table-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
  flex-wrap: wrap;
}

.card-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.table-card__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
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

@media (max-width: 900px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>
