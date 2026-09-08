<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ErrorCircleIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">异常处理</h2>
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
          <t-table row-key="id" :data="taskList" :columns="taskColumns" :loading="taskLoading" size="small" hover table-layout="fixed" cell-empty-content="—">
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
          <t-table row-key="id" :data="logList" :columns="logColumns" :loading="logLoading" size="small" hover table-layout="fixed" cell-empty-content="—">
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
          <t-table row-key="id" :data="instanceList" :columns="instanceColumns" :loading="instanceLoading" size="small" hover table-layout="fixed" cell-empty-content="—">
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

defineOptions({ name: 'ResourceAnomalies' })

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

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #dc2626, #b91c1c);
  --chip-shadow: 0 4px 10px rgba(220, 38, 38, 0.25);
}
</style>
