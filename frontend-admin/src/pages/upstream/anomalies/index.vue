<template>
  <div class="page-body">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WarningIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">异常处理</h2>
          <p class="page-header__desc">聚合上游连接、同步与异常实例事件，集中排查并修复问题。</p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadAll">
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
        </div>
      </article>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">异常明细</h3>
        <span class="table-card__meta">共 {{ totalCount }} 条异常记录</span>
      </div>
      <t-tabs v-model="activeTab" :theme="'card'">
        <t-tab-panel value="tasks" label="同步失败任务">
          <t-table row-key="id" :data="taskList" :columns="taskColumns" :loading="taskLoading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
            <template #provider_id="{ row }">
              <span class="cell-strong">{{ providerName(row.provider_id) }}</span>
            </template>
            <template #task_type="{ row }">
              <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.task_type) }}</t-tag>
            </template>
            <template #status="{ row }">
              <t-tag theme="danger" variant="light" size="small" shape="round">失败</t-tag>
            </template>
            <template #error_message="{ row }">
              <span class="cell-error">{{ row.error_message || '—' }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无失败任务" />
            </template>
          </t-table>
        </t-tab-panel>

        <t-tab-panel value="logs" label="同步失败日志">
          <t-table row-key="id" :data="logList" :columns="logColumns" :loading="logLoading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
            <template #provider_id="{ row }">
              <span class="cell-strong">{{ providerName(row.provider_id) }}</span>
            </template>
            <template #sync_type="{ row }">
              <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.sync_type) }}</t-tag>
            </template>
            <template #status="{ row }">
              <t-tag theme="danger" variant="light" size="small" shape="round">失败</t-tag>
            </template>
            <template #error_message="{ row }">
              <span class="cell-error">{{ row.error_message || '—' }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无失败日志" />
            </template>
          </t-table>
        </t-tab-panel>

        <t-tab-panel value="instances" label="异常实例">
          <t-table row-key="id" :data="instanceList" :columns="instanceColumns" :loading="instanceLoading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
            <template #name="{ row }">
              <span class="cell-strong">{{ row.name }}</span>
            </template>
            <template #provider_id="{ row }">
              <span class="cell-muted">{{ providerName(row.provider_id) }}</span>
            </template>
            <template #region="{ row }">
              <span class="cell-muted">{{ row.region }}{{ row.zone ? ' / ' + row.zone : '' }}</span>
            </template>
            <template #status="{ row }">
              <t-tag theme="danger" variant="light" size="small" shape="round">{{ statusLabel(row.status) }}</t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无异常实例" />
            </template>
          </t-table>
        </t-tab-panel>
      </t-tabs>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { ErrorCircleIcon, ErrorIcon, InfoCircleIcon, RefreshIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getInstanceList, getProviderList, getSyncLogList, getSyncTaskList } from '@/api/admin'
import type { InstanceInfo, ProviderInfo, SyncLogInfo, SyncTaskInfo } from '@/types/interface'

defineOptions({ name: 'UpstreamAnomalies' })

const activeTab = ref('tasks')

const taskList = ref<SyncTaskInfo[]>([])
const logList = ref<SyncLogInfo[]>([])
const instanceList = ref<InstanceInfo[]>([])

const taskLoading = ref(false)
const logLoading = ref(false)
const instanceLoading = ref(false)
const loading = ref(false)

const providers = ref<ProviderInfo[]>([])

const typeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]

const abnormalStatuses = new Set(['error', 'stopped', 'deleted'])

const statCards = computed(() => [
  { key: 'tasks', label: '失败任务', value: taskList.value.length, theme: 'danger' as const, icon: ErrorCircleIcon },
  { key: 'logs', label: '失败日志', value: logList.value.length, theme: 'warning' as const, icon: ErrorIcon },
  { key: 'instances', label: '异常实例', value: instanceList.value.length, theme: 'danger' as const, icon: ServerIcon },
  { key: 'total', label: '异常总数', value: totalCount.value, theme: 'info' as const, icon: InfoCircleIcon },
])

const totalCount = computed(() => taskList.value.length + logList.value.length + instanceList.value.length)

function providerName(id: number): string {
  if (!id) return '—'
  const found = providers.value.find((item) => item.id === id)
  return found ? found.name : `提供商 #${id}`
}

function typeLabel(type: string): string {
  const found = typeOptions.find((item) => item.value === type)
  return found ? found.label : type
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    error: '异常',
    stopped: '已停止',
    deleted: '已删除',
  }
  return map[status] || status
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const taskColumns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', minWidth: 160 },
  { colKey: 'task_type', title: '类型', width: 110 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'error_message', title: '错误信息', minWidth: 240 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

const logColumns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'id', title: '日志 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', minWidth: 160 },
  { colKey: 'sync_type', title: '类型', width: 110 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'error_message', title: '错误信息', minWidth: 240 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

const instanceColumns: PrimaryTableCol<InstanceInfo>[] = [
  { colKey: 'id', title: '实例 ID', width: 90 },
  { colKey: 'name', title: '实例名称', minWidth: 160 },
  { colKey: 'provider_id', title: '提供商', minWidth: 160 },
  { colKey: 'region', title: '地域', width: 160 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 200 })
    providers.value = data.items
  } catch {
    /* 提供商名称映射失败不阻塞 */
  }
}

async function loadTasks() {
  taskLoading.value = true
  try {
    const data = await getSyncTaskList({ status: 'failed', page: 1, page_size: 200 })
    taskList.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载失败任务失败')
  } finally {
    taskLoading.value = false
  }
}

async function loadLogs() {
  logLoading.value = true
  try {
    const data = await getSyncLogList({ status: 'failed', page: 1, page_size: 200 })
    logList.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载失败日志失败')
  } finally {
    logLoading.value = false
  }
}

async function loadInstances() {
  instanceLoading.value = true
  try {
    const data = await getInstanceList({ page: 1, page_size: 200 })
    instanceList.value = data.items.filter((item) => abnormalStatuses.has(item.status))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载异常实例失败')
  } finally {
    instanceLoading.value = false
  }
}

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([loadProviders(), loadTasks(), loadLogs(), loadInstances()])
  } finally {
    loading.value = false
  }
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
  background: linear-gradient(135deg, #dc2626, #b91c1c);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(220, 38, 38, 0.25);
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
  gap: var(--space-md);
}

.stat-card {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-lg);
}

.stat-card__icon {
  width: 42px;
  height: 42px;
  border-radius: var(--hs-radius-lg);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  flex-shrink: 0;
}

.stat-card--danger .stat-card__icon {
  background: linear-gradient(135deg, #ef4444, #b91c1c);
  box-shadow: 0 4px 10px rgba(239, 68, 68, 0.25);
}

.stat-card--warning .stat-card__icon {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  box-shadow: 0 4px 10px rgba(245, 158, 11, 0.25);
}

.stat-card--info .stat-card__icon {
  background: linear-gradient(135deg, #0284c7, #0369a1);
  box-shadow: 0 4px 10px rgba(2, 132, 199, 0.25);
}

.stat-card__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.stat-card__value {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.stat-card__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
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

.cell-strong {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.cell-muted {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.cell-error {
  font-size: 12px;
  color: var(--color-error);
  word-break: break-all;
}

@media (max-width: 1100px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
