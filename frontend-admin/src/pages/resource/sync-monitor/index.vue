<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <DataCheckedIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步监控</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadMonitor">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="stat-grid">
      <div
        v-for="card in statCards"
        :key="card.key"
        class="stat-card surface-card"
        :class="`stat-card--${card.variant}`"
      >
        <span class="stat-card__icon">
          <component :is="card.icon" size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ card.value }}</span>
          <span class="stat-card__label">{{ card.label }}</span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近同步任务</h3>
        <span class="table-card__meta">共 {{ total }} 个任务</span>
      </div>
      <t-table
        row-key="id"
        :data="taskList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #task_type="{ row }">
          <t-tag :theme="typeTheme(row.task_type)" variant="light" size="small" shape="round">{{ typeLabel(row.task_type) }}</t-tag>
        </template>
        <template #result="{ row }">
          <span class="spec-text">{{ row.success_count }} / {{ row.total_count }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">{{ statusLabel(row.status) }}</t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="spec-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无同步任务" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { CheckCircleIcon, DataCheckedIcon, ErrorCircleIcon, RefreshIcon, TimeIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getSyncTaskList } from '@/api/admin'
import type { SyncTaskInfo } from '@/types/interface'

defineOptions({ name: 'ResourceSyncMonitor' })

const taskList = ref<SyncTaskInfo[]>([])
const loading = ref(false)
const total = ref(0)
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
})

const stat = reactive({
  total: 0,
  running: 0,
  success: 0,
  failed: 0,
})

const statCards = computed(() => [
  { key: 'total', label: '任务总数', value: stat.total, icon: TimeIcon, variant: 'indigo' },
  { key: 'running', label: '执行中', value: stat.running, icon: RefreshIcon, variant: 'green' },
  { key: 'success', label: '成功', value: stat.success, icon: CheckCircleIcon, variant: 'success' },
  { key: 'failed', label: '失败', value: stat.failed, icon: ErrorCircleIcon, variant: 'danger' },
])

const typeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]

function typeLabel(type: string): string {
  const found = typeOptions.find((item) => item.value === type)
  return found ? found.label : type
}

function typeTheme(type: string): string {
  if (type === 'product') return 'primary'
  if (type === 'pool') return 'success'
  if (type === 'instance') return 'warning'
  return 'default'
}

function statusLabel(status: string): string {
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'running') return '执行中'
  if (status === 'pending') return '待执行'
  return status
}

function statusTheme(status: string): string {
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

const columns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'id', title: '任务 ID', width: 80 },
  { colKey: 'provider_id', title: '提供商', width: 90 },
  { colKey: 'task_type', title: '类型', width: 110 },
  { colKey: 'result', title: '成功/总数', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

async function loadStat() {
  try {
    const data = await getSyncTaskList({ page: 1, page_size: 200 })
    const items = data.items
    stat.total = data.meta.total
    stat.running = items.filter((item) => item.status === 'running').length
    stat.success = items.filter((item) => item.status === 'success').length
    stat.failed = items.filter((item) => item.status === 'failed').length
  } catch {
    /* 统计失败不阻塞 */
  }
}

async function loadTasks() {
  loading.value = true
  try {
    const data = await getSyncTaskList({ page: pagination.current, page_size: pagination.pageSize })
    taskList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步任务失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTasks()
}

async function loadMonitor() {
  await Promise.all([loadStat(), loadTasks()])
}

onMounted(loadMonitor)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #0891b2, #0e7490);
  --chip-shadow: 0 4px 10px rgba(8, 145, 178, 0.25);
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}
</style>
