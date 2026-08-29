<template>
  <div class="page-body">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">对账报告</h2>
          <p class="page-header__desc">基于同步日志生成提供商维度的对账概览，核对成功与异常。</p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadAll">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">提供商对账汇总</h3>
        <span class="table-card__meta">共 {{ summaryList.length }} 个提供商</span>
      </div>
      <t-table row-key="provider_id" :data="summaryList" :columns="summaryColumns" :loading="loading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
        <template #provider_id="{ row }">
          <span class="cell-strong">提供商 #{{ row.provider_id }}</span>
        </template>
        <template #diff="{ row }">
          <t-tag :theme="row.failed === 0 ? 'success' : 'danger'" variant="light" size="small" shape="round">{{ row.failed === 0 ? '核对通过' : '存在异常' }}</t-tag>
        </template>
        <template #empty>
          <t-empty description="暂无对账数据，请先生成同步日志" />
        </template>
      </t-table>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">同步日志明细</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="logList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #sync_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.sync_type) }}</t-tag>
        </template>
        <template #result="{ row }">
          <span class="cell-muted">{{ row.success_count }} / {{ row.total_count }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'success' ? '成功' : row.status === 'failed' ? '失败' : row.status }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无同步日志" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon, VerifyIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getSyncLogList } from '@/api/admin'
import type { SyncLogInfo } from '@/types/interface'

defineOptions({ name: 'UpstreamReconciliation' })

const logList = ref<SyncLogInfo[]>([])
const loading = ref(false)
const total = ref(0)
const summaryList = ref<{ provider_id: number; total: number; success: number; failed: number }[]>([])

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const typeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]

function typeLabel(type: string): string {
  const found = typeOptions.find((item) => item.value === type)
  return found ? found.label : type
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const summaryColumns: PrimaryTableCol<{ provider_id: number; total: number; success: number; failed: number }>[] = [
  { colKey: 'provider_id', title: '提供商', minWidth: 140 },
  { colKey: 'total', title: '同步次数', width: 110 },
  { colKey: 'success', title: '成功', width: 100 },
  { colKey: 'failed', title: '失败', width: 100 },
  { colKey: 'diff', title: '对账结果', width: 120 },
]

const columns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'id', title: '日志 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', width: 100 },
  { colKey: 'task_id', title: '任务 ID', width: 90 },
  { colKey: 'sync_type', title: '类型', width: 110 },
  { colKey: 'result', title: '成功/总数', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
]

function buildSummary(items: SyncLogInfo[]) {
  const map = new Map<number, { provider_id: number; total: number; success: number; failed: number }>()
  for (const item of items) {
    const entry = map.get(item.provider_id) || { provider_id: item.provider_id, total: 0, success: 0, failed: 0 }
    entry.total += 1
    if (item.status === 'success') entry.success += 1
    if (item.status === 'failed') entry.failed += 1
    map.set(item.provider_id, entry)
  }
  summaryList.value = Array.from(map.values())
}

async function loadSummary() {
  try {
    const data = await getSyncLogList({ page: 1, page_size: 200 })
    buildSummary(data.items)
  } catch {
    /* 汇总失败不阻塞 */
  }
}

async function loadLogs() {
  loading.value = true
  try {
    const data = await getSyncLogList({ page: pagination.current, page_size: pagination.pageSize })
    logList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步日志失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadLogs()
}

async function loadAll() {
  await Promise.all([loadSummary(), loadLogs()])
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
