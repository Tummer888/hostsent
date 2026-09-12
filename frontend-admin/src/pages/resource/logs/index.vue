<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <HistoryIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步日志</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadLogs">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">所属提供商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">同步类型</span>
          <t-select v-model="filters.sync_type" clearable placeholder="全部类型" :options="typeOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
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
        <h3 class="card-title">同步日志列表</h3>
        <span class="table-card__meta">共 {{ total }} 条日志</span>
      </div>
      <t-table
        row-key="id"
        :data="logList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #sync_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.sync_type) }}</t-tag>
        </template>

        <template #result="{ row }">
          <span class="spec-text">{{ row.success_count }} / {{ row.total_count }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'default'" variant="light" size="small" shape="round">
            {{ statusLabel(row.status) }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="spec-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无同步日志" />
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
      :header="detailLog ? `同步日志 · #${detailLog.id}` : '同步日志详情'"
      width="520px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailLog" :column="2" bordered size="small">
        <t-descriptions-item label="日志 ID">#{{ detailLog.id }}</t-descriptions-item>
        <t-descriptions-item label="任务 ID">#{{ detailLog.task_id }}</t-descriptions-item>
        <t-descriptions-item label="提供商 ID">{{ detailLog.provider_id }}</t-descriptions-item>
        <t-descriptions-item label="同步类型">{{ typeLabel(detailLog.sync_type) }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailLog.status === 'success' ? 'success' : detailLog.status === 'failed' ? 'danger' : 'default'" variant="light" size="small" shape="round">
            {{ statusLabel(detailLog.status) }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="成功/总数">{{ detailLog.success_count }} / {{ detailLog.total_count }}</t-descriptions-item>
        <t-descriptions-item label="错误信息">{{ detailLog.error_message || '—' }}</t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatTime(detailLog.created_at) }}</t-descriptions-item>
      </t-descriptions>
      <div v-if="detailLog?.details" class="details-block">
        <h4 class="details-title">同步明细</h4>
        <pre class="details-pre">{{ detailLog.details }}</pre>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { HistoryIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderList, getSyncLogList } from '@/api/admin'
import type { ProviderInfo, SyncLogInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceLogs' })

const logList = ref<SyncLogInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const typeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]
const statusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

const filters = reactive<{ provider_id: number | undefined; sync_type: string | undefined; status: string | undefined }>({
  provider_id: undefined,
  sync_type: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
function typeLabel(type: string): string {
  const found = typeOptions.find((item) => item.value === type)
  return found ? found.label : type
}

function statusLabel(status: string): string {
  if (status === 'success') return '成功'
  if (status === 'failed') return '失败'
  return status
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const columns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'id', title: '日志 ID', width: 90 },
  { colKey: 'task_id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', width: 100 },
  { colKey: 'sync_type', title: '同步类型', width: 120 },
  { colKey: 'result', title: '成功/总数', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 90,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadLogs() {
  loading.value = true
  try {
    const data = await getSyncLogList({
      provider_id: filters.provider_id,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    logList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步日志失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 提供商下拉加载失败不阻塞列表 */
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadLogs()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
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
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}


function handleSearch() {
  pagination.current = 1
  loadLogs()
}

function handleResetFilters() {
  filters.provider_id = undefined
  filters.sync_type = undefined
  filters.status = undefined
  pagination.current = 1
  loadLogs()
}

const detailVisible = ref(false)
const detailLog = ref<SyncLogInfo | null>(null)

function openDetailDialog(row: SyncLogInfo) {
  detailLog.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadProviders()
  loadLogs()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: SyncLogInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetailDialog(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

/* 桌面端固定列数；窄屏回落到 shared.css 的单列（否则 3×200px 在移动端横向溢出） */
@media (min-width: 769px) {
  .filter-card__grid {
    grid-template-columns: repeat(3, minmax(200px, 1fr));
  }
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.details-block {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}

.details-title {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.details-pre {
  margin: 0;
  padding: 12px;
  max-height: 260px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  color: #334155;
  background: var(--hs-surface-2);
  border-radius: var(--hs-radius-md);
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
