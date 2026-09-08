<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">云主机实例</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadInstances">
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
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="实例名称 / IP" clearable />
        </div>
        <div class="field">
          <span class="field__label">所属提供商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">实例列表</h3>
        <span class="table-card__meta">共 {{ total }} 个实例</span>
      </div>
      <t-table
        row-key="id"
        :data="instanceList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="instance-cell">
            <span class="instance-name">{{ row.name }}</span>
            <span class="instance-sub">{{ row.instance_id }}</span>
          </div>
        </template>

        <template #specs="{ row }">
          <span class="spec-text">{{ row.cpu }}核 / {{ formatMemory(row.memory) }} / {{ formatDisk(row.disk) }} {{ row.bandwidth }}Mbps</span>
        </template>

        <template #ip="{ row }">
          <div class="ip-cell">
            <span class="ip-public">{{ row.public_ip || '—' }}</span>
            <span class="ip-private">内网 {{ row.private_ip || '—' }}</span>
          </div>
        </template>

        <template #region="{ row }">
          <span>{{ row.region }}{{ row.zone ? ' / ' + row.zone : '' }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">{{ statusLabel(row.status) }}</t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无实例，请先执行实例同步" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="detailVisible"
      :header="detailInstance ? `实例详情 · ${detailInstance.name}` : '实例详情'"
      width="620px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailInstance" :column="2" bordered size="small">
        <t-descriptions-item label="实例名称">{{ detailInstance.name }}</t-descriptions-item>
        <t-descriptions-item label="实例 ID">{{ detailInstance.instance_id }}</t-descriptions-item>
        <t-descriptions-item label="提供商 ID">{{ detailInstance.provider_id }}</t-descriptions-item>
        <t-descriptions-item label="商品 ID">{{ detailInstance.product_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="规格">{{ detailInstance.cpu }}核 / {{ formatMemory(detailInstance.memory) }} / {{ formatDisk(detailInstance.disk) }} {{ detailInstance.bandwidth }}Mbps</t-descriptions-item>
        <t-descriptions-item label="磁盘类型">{{ detailInstance.disk_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="操作系统">{{ detailInstance.os || '—' }}</t-descriptions-item>
        <t-descriptions-item label="区域/可用区">{{ detailInstance.region }}{{ detailInstance.zone ? ' / ' + detailInstance.zone : '' }}</t-descriptions-item>
        <t-descriptions-item label="公网 IP">{{ detailInstance.public_ip || '—' }}</t-descriptions-item>
        <t-descriptions-item label="内网 IP">{{ detailInstance.private_ip || '—' }}</t-descriptions-item>
        <t-descriptions-item label="计费模式">{{ billingLabel(detailInstance.billing_mode) }}</t-descriptions-item>
        <t-descriptions-item label="到期时间">{{ detailInstance.expire_at ? formatTime(detailInstance.expire_at) : '—' }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="statusTheme(detailInstance.status)" variant="light" size="small" shape="round">{{ statusLabel(detailInstance.status) }}</t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatTime(detailInstance.created_at) }}</t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon, SearchIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getInstanceList, getProviderList } from '@/api/admin'
import type { InstanceInfo, ProviderInfo } from '@/types/interface'

defineOptions({ name: 'ResourceInstances' })

const instanceList = ref<InstanceInfo[]>([])
const loading = ref(false)
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const statusOptions = [
  { label: '运行中', value: 'running' },
  { label: '已停止', value: 'stopped' },
  { label: '被禁用', value: 'disabled' },
  { label: '异常', value: 'error' },
]

const filters = reactive<{ keyword: string | undefined; provider_id: number | undefined; status: string | undefined }>({
  keyword: undefined,
  provider_id: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

function formatMemory(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatDisk(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function statusLabel(status: string): string {
  const found = statusOptions.find((item) => item.value === status)
  return found ? found.label : status
}

function statusTheme(status: string): string {
  if (status === 'running') return 'success'
  if (status === 'stopped') return 'default'
  if (status === 'error') return 'danger'
  return 'warning'
}

function billingLabel(mode: string): string {
  if (mode === 'monthly') return '包年包月'
  if (mode === 'hourly') return '按量计费'
  return mode || '—'
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns: PrimaryTableCol<InstanceInfo>[] = [
  { colKey: 'name', title: '实例', minWidth: 170 },
  { colKey: 'specs', title: '规格', minWidth: 190 },
  { colKey: 'os', title: '系统', width: 100 },
  { colKey: 'ip', title: 'IP', minWidth: 150 },
  { colKey: 'region', title: '区域/可用区', width: 130 },
  { colKey: 'status', title: '状态', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: 90,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadInstances() {
  loading.value = true
  try {
    const data = await getInstanceList({
      keyword: filters.keyword,
      provider_id: filters.provider_id,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    instanceList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例列表失败')
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
  loadInstances()
}

function handleSearch() {
  pagination.current = 1
  loadInstances()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.provider_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadInstances()
}

const detailVisible = ref(false)
const detailInstance = ref<InstanceInfo | null>(null)

function openDetailDialog(row: InstanceInfo) {
  detailInstance.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadProviders()
  loadInstances()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #059669, #047857);
  --chip-shadow: 0 4px 10px rgba(5, 150, 105, 0.25);
}

.filter-card__grid {
  grid-template-columns: repeat(3, minmax(200px, 1fr));
}

.instance-cell,
.ip-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.instance-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.instance-sub,
.ip-private {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.ip-public {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}
</style>
