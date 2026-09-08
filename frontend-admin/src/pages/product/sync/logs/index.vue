<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
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
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">供应商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部供应商" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">任务 ID</span>
          <t-input-number v-model="filters.task_id" :min="0" clearable placeholder="按任务 ID 筛选" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">日志列表</h3>
        <span class="table-card__meta">共 {{ total }} 条日志</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #provider="{ row }">
          <span>{{ providerName(row.provider_id) }}</span>
        </template>

        <template #sync_type="{ row }">
          <span>{{ syncTypeLabel(row.sync_type) }}</span>
        </template>

        <template #progress="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ row.success_count }} / {{ row.total_count }}</span>
            <span class="price-sub">成功 / 总量</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #details="{ row }">
          <span class="cell-muted">{{ row.details || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
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
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getProviderList, getSyncLogList } from '@/api/admin'
import { formatTime } from '@/pages/product/constants'
import type { ProviderInfo, SyncLogInfo } from '@/types/interface'

defineOptions({ name: 'ProductSyncLogs' })

const statusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '进行中', value: 'running' },
]

const loading = ref(false)
const list = ref<SyncLogInfo[]>([])
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])

const filters = reactive<{ provider_id: number | undefined; task_id: number | undefined; status: string | undefined }>({
  provider_id: undefined,
  task_id: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<SyncLogInfo>[] = [
  { colKey: 'id', title: '日志 ID', width: 70 },
  { colKey: 'task_id', title: '任务 ID', width: 80 },
  { colKey: 'provider', title: '供应商', width: 140 },
  { colKey: 'sync_type', title: '同步类型', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'progress', title: '进度', width: 100 },
  { colKey: 'details', title: '详情', minWidth: 180 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
]

function providerName(id: number): string {
  return providerOptions.value.find((item) => item.value === id)?.label || `#${id}` || '—'
}

function syncTypeLabel(type: string): string {
  const map: Record<string, string> = { product: '商品同步', instance: '实例同步' }
  return map[type] || type || '—'
}

function statusTag(status: string): { theme: 'success' | 'danger' | 'warning' | 'default'; text: string } {
  if (status === 'success') return { theme: 'success', text: '成功' }
  if (status === 'failed') return { theme: 'danger', text: '失败' }
  if (status === 'running') return { theme: 'warning', text: '进行中' }
  return { theme: 'default', text: status || '—' }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 200 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 供应商加载失败不阻塞 */
  }
}

async function loadLogs() {
  loading.value = true
  try {
    const data = await getSyncLogList({
      provider_id: filters.provider_id,
      task_id: filters.task_id,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
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

function handleSearch() {
  pagination.current = 1
  loadLogs()
}

function handleResetFilters() {
  filters.provider_id = undefined
  filters.task_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadLogs()
}

onMounted(() => {
  loadProviders()
  loadLogs()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
