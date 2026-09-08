<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步任务</h2>
          <p class="page-header__desc">查看上游资源同步任务，可手动发起商品 / 实例同步。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadTasks">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          创建同步任务
        </t-button>
      </t-space>
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
          <span class="field__label">任务类型</span>
          <t-select v-model="filters.task_type" clearable placeholder="全部类型" :options="taskTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">任务列表</h3>
        <span class="table-card__meta">共 {{ total }} 个任务</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
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
        <template #provider="{ row }">
          <span>{{ providerName(row.provider_id) }}</span>
        </template>

        <template #task_type="{ row }">
          <span>{{ taskTypeLabel(row.task_type) }}</span>
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

        <template #time="{ row }">
          <div class="cell-muted">{{ formatTime(row.started_at) }}<br />至 {{ formatTime(row.completed_at) }}</div>
        </template>

        <template #error="{ row }">
          <span class="cell-muted">{{ row.error_message || '—' }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无同步任务" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="createVisible"
      header="创建同步任务"
      width="460px"
      :confirm-btn="{ content: '发起同步', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreateTask"
      @close="createVisible = false"
    >
      <t-form label-align="top" :data="createForm" @submit.prevent>
        <t-form-item label="供应商" name="provider_id">
          <t-select v-model="createForm.provider_id" :options="providerOptions" placeholder="请选择供应商" filterable />
        </t-form-item>
        <t-form-item label="任务类型" name="task_type">
          <t-select v-model="createForm.task_type" :options="taskTypeOptions" placeholder="请选择任务类型" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getProviderList, getSyncTaskList, createSyncTask } from '@/api/admin'
import { formatTime } from '@/pages/product/constants'
import type { ProviderInfo, SyncTaskInfo } from '@/types/interface'

defineOptions({ name: 'ProductSyncTasks' })

const taskTypeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '实例同步', value: 'instance' },
]
const statusOptions = [
  { label: '进行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

const loading = ref(false)
const list = ref<SyncTaskInfo[]>([])
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])

const filters = reactive<{ provider_id: number | undefined; task_type: string | undefined; status: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'id', title: '任务 ID', width: 70 },
  { colKey: 'provider', title: '供应商', width: 140 },
  { colKey: 'task_type', title: '类型', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'progress', title: '进度', width: 100 },
  { colKey: 'time', title: '执行时间', width: 170 },
  { colKey: 'error', title: '错误信息', minWidth: 160 },
]

function providerName(id: number): string {
  return providerOptions.value.find((item) => item.value === id)?.label || `#${id}` || '—'
}

function taskTypeLabel(type: string): string {
  return taskTypeOptions.find((item) => item.value === type)?.label || type || '—'
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

async function loadTasks() {
  loading.value = true
  try {
    const data = await getSyncTaskList({
      provider_id: filters.provider_id,
      task_type: filters.task_type,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载任务列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTasks()
}

function handleSearch() {
  pagination.current = 1
  loadTasks()
}

function handleResetFilters() {
  filters.provider_id = undefined
  filters.task_type = undefined
  filters.status = undefined
  pagination.current = 1
  loadTasks()
}

// ---- 创建任务 ----
const createVisible = ref(false)
const createForm = reactive<{ provider_id: number | undefined; task_type: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
})

function openCreate() {
  createForm.provider_id = undefined
  createForm.task_type = 'product'
  createVisible.value = true
}

async function handleCreateTask() {
  if (!createForm.provider_id || !createForm.task_type) {
    MessagePlugin.warning('请选择供应商与任务类型')
    return
  }
  try {
    await createSyncTask({ provider_id: createForm.provider_id, task_type: createForm.task_type })
    MessagePlugin.success('同步任务已创建')
    createVisible.value = false
    loadTasks()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建同步任务失败')
  }
}

onMounted(() => {
  loadProviders()
  loadTasks()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
