<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RefreshIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">同步任务</h2>
        </div>
      </div>
      <t-space>
        <t-button variant="outline" :loading="loading" @click="loadTasks">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreateDialog">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建同步任务
        </t-button>
      </t-space>
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
          <span class="field__label">任务类型</span>
          <t-select v-model="filters.task_type" clearable placeholder="全部类型" :options="taskTypeOptions" />
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
        <h3 class="card-title">同步任务列表</h3>
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
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #task_type="{ row }">
          <t-tag :theme="taskTypeTheme(row.task_type)" variant="light" size="small" shape="round">{{ taskTypeLabel(row.task_type) }}</t-tag>
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
          <t-empty description="暂无同步任务，请新建同步任务" />
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
      :header="detailTask ? `同步任务详情 · #${detailTask.id}` : '同步任务详情'"
      width="520px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailTask" :column="2" bordered size="small">
        <t-descriptions-item label="任务 ID">#{{ detailTask.id }}</t-descriptions-item>
        <t-descriptions-item label="提供商 ID">{{ detailTask.provider_id }}</t-descriptions-item>
        <t-descriptions-item label="任务类型">{{ taskTypeLabel(detailTask.task_type) }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="statusTheme(detailTask.status)" variant="light" size="small" shape="round">{{ statusLabel(detailTask.status) }}</t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="成功/总数">{{ detailTask.success_count }} / {{ detailTask.total_count }}</t-descriptions-item>
        <t-descriptions-item label="错误信息">{{ detailTask.error_message || '—' }}</t-descriptions-item>
        <t-descriptions-item label="开始时间">{{ detailTask.started_at ? formatTime(detailTask.started_at) : '—' }}</t-descriptions-item>
        <t-descriptions-item label="完成时间">{{ detailTask.completed_at ? formatTime(detailTask.completed_at) : '—' }}</t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatTime(detailTask.created_at) }}</t-descriptions-item>
      </t-descriptions>
    </t-dialog>

    <t-dialog
      v-model:visible="createVisible"
      header="新建同步任务"
      width="460px"
      :confirm-btn="{ content: '提交', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreateTask"
      @close="createVisible = false"
    >
      <t-form label-align="top" :data="createForm" @submit.prevent>
        <t-form-item label="所属提供商" name="provider_id">
          <t-select v-model="createForm.provider_id" :options="providerOptions" placeholder="请选择提供商" />
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

import { AddIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { createSyncTask, getProviderList, getSyncTaskList } from '@/api/admin'
import type { ProviderInfo, SyncTaskInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceSync' })

const taskList = ref<SyncTaskInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const taskTypeOptions = [
  { label: '商品同步', value: 'product' },
  { label: '资源池同步', value: 'pool' },
  { label: '实例同步', value: 'instance' },
]
const statusOptions = [
  { label: '待执行', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

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

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
function taskTypeLabel(type: string): string {
  const found = taskTypeOptions.find((item) => item.value === type)
  return found ? found.label : type
}

function taskTypeTheme(type: string): string {
  if (type === 'product') return 'primary'
  if (type === 'pool') return 'success'
  if (type === 'instance') return 'warning'
  return 'default'
}

function statusLabel(status: string): string {
  const found = statusOptions.find((item) => item.value === status)
  return found ? found.label : status
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
  { colKey: 'id', title: '任务 ID', width: 90 },
  { colKey: 'provider_id', title: '提供商', width: 100 },
  { colKey: 'task_type', title: '任务类型', width: 120 },
  { colKey: 'result', title: '成功/总数', width: 120 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', minWidth: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: 90,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

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
    taskList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步任务失败')
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
  loadTasks()
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
  loadTasks()
}

function handleResetFilters() {
  filters.provider_id = undefined
  filters.task_type = undefined
  filters.status = undefined
  pagination.current = 1
  loadTasks()
}

const detailVisible = ref(false)
const detailTask = ref<SyncTaskInfo | null>(null)

function openDetailDialog(row: SyncTaskInfo) {
  detailTask.value = row
  detailVisible.value = true
}

const createVisible = ref(false)
const createForm = reactive<{ provider_id: number | undefined; task_type: string | undefined }>({
  provider_id: undefined,
  task_type: undefined,
})

function openCreateDialog() {
  createForm.provider_id = undefined
  createForm.task_type = undefined
  createVisible.value = true
}

async function handleCreateTask() {
  if (!createForm.provider_id || !createForm.task_type) {
    MessagePlugin.warning('请选择提供商与任务类型')
    return
  }
  try {
    await createSyncTask({
      provider_id: createForm.provider_id,
      task_type: createForm.task_type,
    })
    MessagePlugin.success('同步任务已创建')
    createVisible.value = false
    pagination.current = 1
    loadTasks()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建同步任务失败')
  }
}

onMounted(() => {
  loadProviders()
  loadTasks()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: SyncTaskInfo) {
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
  --chip-bg: linear-gradient(135deg, #0284c7, #0369a1);
  --chip-shadow: 0 4px 10px rgba(2, 132, 199, 0.25);
}

.filter-card__grid {
  grid-template-columns: repeat(3, minmax(200px, 1fr));
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}
</style>
