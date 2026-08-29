<template>
  <div class="page-body">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CloudDownloadIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">商品同步</h2>
          <p class="page-header__desc">按提供商手动触发商品同步，拉取上游商品到本地商品库。</p>
        </div>
      </div>
      <t-button variant="outline" :loading="providersLoading || tasksLoading" @click="loadAll">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">提供商列表</h3>
        <span class="table-card__meta">共 {{ providers.length }} 个提供商</span>
      </div>
      <t-table row-key="id" :data="providers" :columns="providerColumns" :loading="providersLoading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
        <template #name="{ row }">
          <span class="cell-strong">{{ row.name }}</span>
        </template>
        <template #sync_enabled="{ row }">
          <t-tag :theme="row.sync_enabled ? 'success' : 'default'" variant="light" size="small" shape="round">{{ row.sync_enabled ? '已启用' : '未启用' }}</t-tag>
        </template>
        <template #last_sync_at="{ row }">
          <span class="cell-muted">{{ row.last_sync_at ? formatTime(row.last_sync_at) : '从未同步' }}</span>
        </template>
        <template #action="{ row }">
          <t-button size="small" theme="primary" variant="outline" :loading="syncingId === row.id" @click="triggerSync(row)">
            <template #icon>
              <CloudDownloadIcon aria-hidden="true" />
            </template>
            同步商品
          </t-button>
        </template>
        <template #empty>
          <t-empty description="暂无提供商，请先添加上游提供商" />
        </template>
      </t-table>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近商品同步任务</h3>
        <span class="table-card__meta">共 {{ tasks.length }} 条</span>
      </div>
      <t-table row-key="id" :data="tasks" :columns="taskColumns" :loading="tasksLoading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
        <template #result="{ row }">
          <span class="cell-muted">{{ row.success_count }} / {{ row.total_count }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : row.status === 'running' ? 'warning' : 'default'" variant="light" size="small" shape="round">
            {{ taskStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无商品同步任务" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { CloudDownloadIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createSyncTask, getProviderList, getSyncTaskList } from '@/api/admin'
import type { ProviderInfo, SyncTaskInfo } from '@/types/interface'

defineOptions({ name: 'UpstreamProductSync' })

const providers = ref<ProviderInfo[]>([])
const providersLoading = ref(false)
const tasks = ref<SyncTaskInfo[]>([])
const tasksLoading = ref(false)
const syncingId = ref<number | null>(null)

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function taskStatusLabel(status: string): string {
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'running') return '执行中'
  if (status === 'pending') return '待执行'
  return status
}

const providerColumns: PrimaryTableCol<ProviderInfo>[] = [
  { colKey: 'name', title: '提供商', minWidth: 160 },
  { colKey: 'provider_type', title: '类型', width: 120 },
  { colKey: 'api_endpoint', title: 'API 地址', minWidth: 180 },
  { colKey: 'sync_enabled', title: '同步开关', width: 110 },
  { colKey: 'last_sync_at', title: '上次同步', minWidth: 170 },
  { colKey: 'action', title: '操作', width: 140, fixed: 'right' as const, align: 'center' as const },
]

const taskColumns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', width: 100 },
  { colKey: 'result', title: '成功/总数', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

async function loadProviders() {
  providersLoading.value = true
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providers.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商失败')
  } finally {
    providersLoading.value = false
  }
}

async function loadTasks() {
  tasksLoading.value = true
  try {
    const data = await getSyncTaskList({ task_type: 'product', page: 1, page_size: 10 })
    tasks.value = data.items
  } catch {
    /* 任务加载失败不阻塞 */
  } finally {
    tasksLoading.value = false
  }
}

async function triggerSync(row: ProviderInfo) {
  if (syncingId.value === row.id) return
  syncingId.value = row.id
  try {
    await createSyncTask({ provider_id: row.id, task_type: 'product' })
    MessagePlugin.success(`已触发「${row.name}」商品同步`)
    loadTasks()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '触发商品同步失败')
  } finally {
    syncingId.value = null
  }
}

async function loadAll() {
  await Promise.all([loadProviders(), loadTasks()])
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
  background: linear-gradient(135deg, #7c3aed, #6d28d9);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(124, 58, 237, 0.25);
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

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
