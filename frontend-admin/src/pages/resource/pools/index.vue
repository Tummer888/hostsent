<template>
  <div class="page-body resource-module">
    <header v-if="!isEmbedded" class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LayersIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">资源池管理</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadPools">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section v-if="!isEmbedded" class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">所属提供商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
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
        <h3 class="card-title">资源池列表</h3>
        <span class="table-card__meta">共 {{ total }} 个资源池</span>
      </div>
      <t-table
        row-key="id"
        :data="poolList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <span class="pool-name">{{ row.name }}</span>
        </template>

        <template #cpu="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ row.used_cpu }} / {{ row.total_cpu }}</span>
            <t-progress :percentage="usagePercent(row.used_cpu, row.total_cpu)" :color="usageColor(row.used_cpu, row.total_cpu)" :stroke-width="6" />
          </div>
        </template>

        <template #memory="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ formatMemory(row.used_memory) }} / {{ formatMemory(row.total_memory) }}</span>
            <t-progress :percentage="usagePercent(row.used_memory, row.total_memory)" :color="usageColor(row.used_memory, row.total_memory)" :stroke-width="6" />
          </div>
        </template>

        <template #disk="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ formatDisk(row.used_disk) }} / {{ formatDisk(row.total_disk) }}</span>
            <t-progress :percentage="usagePercent(row.used_disk, row.total_disk)" :color="usageColor(row.used_disk, row.total_disk)" :stroke-width="6" />
          </div>
        </template>

        <template #pool_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ row.pool_type || '—' }}</t-tag>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
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
          <t-empty description="暂无资源池数据" />
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
      :header="detailPool ? `资源池详情 · ${detailPool.name}` : '资源池详情'"
      width="560px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailPool" :column="2" bordered size="small">
        <t-descriptions-item label="池名称">{{ detailPool.name }}</t-descriptions-item>
        <t-descriptions-item label="上游 ID">{{ detailPool.upstream_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="类型">{{ detailPool.pool_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailPool.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ detailPool.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="CPU 配额">{{ detailPool.used_cpu }} / {{ detailPool.total_cpu }}</t-descriptions-item>
        <t-descriptions-item label="内存配额">{{ formatMemory(detailPool.used_memory) }} / {{ formatMemory(detailPool.total_memory) }}</t-descriptions-item>
        <t-descriptions-item label="磁盘配额">{{ formatDisk(detailPool.used_disk) }} / {{ formatDisk(detailPool.total_disk) }}</t-descriptions-item>
        <t-descriptions-item label="最后同步">{{ detailPool.last_sync_at ? formatTime(detailPool.last_sync_at) : '从未同步' }}</t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'

import { LayersIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getPoolList, getProviderList } from '@/api/admin'
import type { PoolInfo, ProviderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourcePools' })

// 作为详情页内嵌面板时传入 provider_id；独立使用时为 undefined（全部）
const props = defineProps<{
  providerId?: number
}>()

const poolList = ref<PoolInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])

const filters = reactive<{ provider_id: number | undefined }>({
  provider_id: undefined,
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
const effectiveProviderId = computed(() => {
  if (props.providerId !== undefined) return props.providerId
  return filters.provider_id
})

const isEmbedded = computed(() => props.providerId !== undefined)

function usagePercent(used: number, total: number): number {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

function usageColor(used: number, total: number): string {
  const pct = usagePercent(used, total)
  if (pct >= 80) return '#dc2626'
  if (pct >= 60) return '#d97706'
  return '#16a34a'
}

function formatMemory(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatDisk(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns: PrimaryTableCol<PoolInfo>[] = [
  { colKey: 'name', title: '池名称', minWidth: 160 },
  { colKey: 'pool_type', title: '类型', width: 110 },
  { colKey: 'cpu', title: 'CPU（已用/总）', minWidth: 160 },
  { colKey: 'memory', title: '内存（已用/总）', minWidth: 160 },
  { colKey: 'disk', title: '磁盘（已用/总）', minWidth: 160 },
  { colKey: 'status', title: '状态', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 90,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadPools() {
  loading.value = true
  try {
    const data = await getPoolList({
      provider_id: effectiveProviderId.value,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    poolList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载资源池列表失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  if (props.providerId !== undefined) return
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
  loadPools()
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
  loadPools()
}

function handleResetFilters() {
  filters.provider_id = undefined
  pagination.current = 1
  loadPools()
}

const detailVisible = ref(false)
const detailPool = ref<PoolInfo | null>(null)

function openDetailDialog(row: PoolInfo) {
  detailPool.value = row
  detailVisible.value = true
}

watch(effectiveProviderId, () => {
  pagination.current = 1
  loadPools()
})

onMounted(() => {
  loadProviders()
  loadPools()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: PoolInfo) {
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
  --chip-shadow: 0 2px 6px rgba(22, 163, 74, 0.16);
}

.filter-card__grid {
  grid-template-columns: minmax(220px, 1fr);
}

.pool-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.spec-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}
</style>
