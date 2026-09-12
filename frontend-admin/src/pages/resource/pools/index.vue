<template>
  <div class="page-body resource-module">
    <header v-if="!isEmbedded" class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LayersIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">容量与位置监测</h2>
          <p class="page-header__desc">只看资源池的容量与位置：CPU / 内存 / 磁盘用量对比配额，池所属地域与可用区</p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadPools">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section v-if="!isEmbedded" class="stat-grid stat-grid--pools">
      <article v-for="card in statCards" :key="card.key" class="stat-card surface-card" :class="`stat-card--${card.theme}`">
        <div class="stat-card__icon">
          <component :is="card.icon" size="20" aria-hidden="true" />
        </div>
        <div class="stat-card__text">
          <span class="stat-card__value">{{ card.value }}</span>
          <span class="stat-card__label">{{ card.label }}</span>
          <span v-if="card.hint" class="stat-card__hint">{{ card.hint }}</span>
        </div>
      </article>
    </section>

    <section v-if="!isEmbedded" class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="池名 / 上游 ID" @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">所属提供商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">位置（地域）</span>
          <t-select v-model="filters.region" clearable placeholder="全部地域" :options="regionOptions" />
        </div>
        <div class="field">
          <span class="field__label">容量告警</span>
          <t-select v-model="filters.alert" clearable placeholder="全部资源池" :options="alertOptions" />
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
          <div class="pool-cell">
            <span class="pool-name">{{ row.name }}</span>
            <span v-if="!isEmbedded" class="pool-provider">{{ row.provider_name || `渠道 #${row.provider_id}` }}</span>
          </div>
        </template>

        <template #location="{ row }">
          <div class="loc-cell">
            <span v-if="row.region" class="loc-region">{{ row.region }}</span>
            <t-tag v-else theme="warning" variant="light" size="small" shape="round">未标注位置</t-tag>
            <span v-if="row.zone" class="loc-zone">{{ row.zone }}</span>
          </div>
        </template>

        <template #cpu="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ row.used_cpu }} / {{ row.total_cpu }}</span>
            <t-progress :percentage="row.cpu_usage_percent" :color="usageColor(row.cpu_usage_percent)" :stroke-width="6" />
          </div>
        </template>

        <template #memory="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ formatMemory(row.used_memory) }} / {{ formatMemory(row.total_memory) }}</span>
            <t-progress :percentage="row.memory_usage_percent" :color="usageColor(row.memory_usage_percent)" :stroke-width="6" />
          </div>
        </template>

        <template #disk="{ row }">
          <div class="spec-cell">
            <span class="spec-text">{{ formatDisk(row.used_disk) }} / {{ formatDisk(row.total_disk) }}</span>
            <t-progress :percentage="row.disk_usage_percent" :color="usageColor(row.disk_usage_percent)" :stroke-width="6" />
          </div>
        </template>

        <template #pool_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ row.pool_type || '—' }}</t-tag>
        </template>

        <template #probe="{ row }">
          <t-tag v-if="!row.probe_status" theme="default" variant="light" size="small" shape="round">未接入</t-tag>
          <t-tooltip v-else :content="row.probe_message || '探针正常'" placement="top">
            <t-tag :theme="probeTheme(row.probe_status)" variant="light" size="small" shape="round">
              {{ probeLabel(row.probe_status) }}
            </t-tag>
          </t-tooltip>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
        </template>

        <template #ops_url="{ row }">
          <t-link
            v-if="row.provider_ops_url"
            theme="primary"
            hover="color"
            :href="row.provider_ops_url"
            target="_blank"
          >
            运维平台
          </t-link>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
                { content: '运维平台', value: 'ops', hidden: () => !row.provider_ops_url, theme: 'default' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
              <span v-if="row.provider_ops_url" class="action-gap" />
              <t-link
                v-if="row.provider_ops_url"
                theme="primary"
                hover="color"
                :href="row.provider_ops_url"
                target="_blank"
              >
                运维平台
              </t-link>
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
        <t-descriptions-item label="所属渠道">{{ detailPool.provider_name || `渠道 #${detailPool.provider_id}` }}</t-descriptions-item>
        <t-descriptions-item label="类型">{{ detailPool.pool_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="位置">
          <span v-if="detailPool.region">
            {{ detailPool.region }}<template v-if="detailPool.zone"> / {{ detailPool.zone }}</template>
          </span>
          <t-tag v-else theme="warning" variant="light" size="small" shape="round">未标注位置</t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailPool.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ detailPool.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="CPU 配额">
          {{ detailPool.used_cpu }} / {{ detailPool.total_cpu }}（{{ detailPool.cpu_usage_percent }}%）
        </t-descriptions-item>
        <t-descriptions-item label="内存配额">
          {{ formatMemory(detailPool.used_memory) }} / {{ formatMemory(detailPool.total_memory) }}（{{ detailPool.memory_usage_percent }}%）
        </t-descriptions-item>
        <t-descriptions-item label="磁盘配额">
          {{ formatDisk(detailPool.used_disk) }} / {{ formatDisk(detailPool.total_disk) }}（{{ detailPool.disk_usage_percent }}%）
        </t-descriptions-item>
        <t-descriptions-item label="最后同步">{{ detailPool.last_sync_at ? formatTime(detailPool.last_sync_at) : '从未同步' }}</t-descriptions-item>
        <t-descriptions-item label="探针状态">
          <t-tag v-if="!detailPool.probe_status" theme="default" variant="light" size="small" shape="round">未接入</t-tag>
          <span v-else>{{ probeLabel(detailPool.probe_status) }} · {{ formatTime(detailPool.probe_at || '') }}</span>
        </t-descriptions-item>
        <t-descriptions-item label="运维平台">
          <t-link
            v-if="detailPool.provider_ops_url"
            theme="primary"
            hover="color"
            :href="detailPool.provider_ops_url"
            target="_blank"
          >
            打开运维平台
          </t-link>
          <span v-else class="cell-muted">未配置</span>
        </t-descriptions-item>
      </t-descriptions>
      <t-alert
        v-if="detailPool && !detailPool.probe_status"
        theme="info"
        message="该资源池尚未接入探针流：当前用量来自上游同步周期快照，接入探针后可展示实时状态 / 内存 / 硬盘用量。"
        class="probe-hint"
      />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'

import {
  CloudDownloadIcon,
  ErrorCircleIcon,
  LayersIcon,
  RefreshIcon,
  SearchIcon,
  ServerIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getPoolList, getProviderList } from '@/api/admin'
import type { PoolCapacitySummary, PoolInfo, ProviderInfo } from '@/types/interface'
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
const regionOptions = ref<{ label: string; value: string }[]>([])

const emptySummary: PoolCapacitySummary = {
  total_pools: 0,
  online_pools: 0,
  total_cpu: 0,
  used_cpu: 0,
  total_memory: 0,
  used_memory: 0,
  total_disk: 0,
  used_disk: 0,
  warning_pools: 0,
  danger_pools: 0,
  unlocated_pools: 0,
}
const summary = ref<PoolCapacitySummary>({ ...emptySummary })

const alertOptions = [
  { label: '告警池（用量≥80%）', value: 'danger' },
  { label: '预警池（用量≥60%）', value: 'warn' },
]

const filters = reactive<{ keyword: string; provider_id: number | undefined; region: string; alert: string }>({
  keyword: '',
  provider_id: undefined,
  region: '',
  alert: '',
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

// 容量汇总卡片：总体用量 + 告警池 + 未定位池（位置检测的覆盖率提示）。
const statCards = computed(() => {
  const s = summary.value
  return [
    {
      key: 'pools',
      label: '资源池总数',
      value: `${s.online_pools} / ${s.total_pools}`,
      hint: '在线 / 全部',
      icon: LayersIcon,
      theme: 'info' as const,
    },
    {
      key: 'cpu',
      label: 'CPU 用量',
      value: `${percent(s.used_cpu, s.total_cpu)}%`,
      hint: `${s.used_cpu} / ${s.total_cpu} 核`,
      icon: ServerIcon,
      theme: 'success' as const,
    },
    {
      key: 'memory',
      label: '内存用量',
      value: `${percent(s.used_memory, s.total_memory)}%`,
      hint: `${formatMemory(s.used_memory)} / ${formatMemory(s.total_memory)}`,
      icon: ServerIcon,
      theme: 'success' as const,
    },
    {
      key: 'disk',
      label: '磁盘用量',
      value: `${percent(s.used_disk, s.total_disk)}%`,
      hint: `${formatDisk(s.used_disk)} / ${formatDisk(s.total_disk)}`,
      icon: CloudDownloadIcon,
      theme: 'success' as const,
    },
    {
      key: 'alert',
      label: '容量告警池',
      value: `${s.danger_pools}`,
      hint: `预警池 ${s.warning_pools} 个`,
      icon: ErrorCircleIcon,
      theme: s.danger_pools > 0 ? ('danger' as const) : ('info' as const),
    },
    {
      key: 'location',
      label: '未标注位置',
      value: `${s.unlocated_pools}`,
      hint: '需在上游或渠道补充地域',
      icon: LayersIcon,
      theme: s.unlocated_pools > 0 ? ('warning' as const) : ('info' as const),
    },
  ]
})

function percent(used: number, total: number): number {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

function usageColor(pct: number): string {
  if (pct >= 80) return '#dc2626'
  if (pct >= 60) return '#d97706'
  return '#16a34a'
}

function probeLabel(status: string): string {
  switch (status) {
    case 'healthy':
      return '探针正常'
    case 'warning':
      return '探针异常'
    case 'down':
      return '探针离线'
    default:
      return status
  }
}

function probeTheme(status: string) {
  if (status === 'healthy') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'down') return 'danger'
  return 'default'
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
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns = computed<PrimaryTableCol<PoolInfo>[]>(() => {
  const base: PrimaryTableCol<PoolInfo>[] = [
    { colKey: 'name', title: '池名称', minWidth: 170 },
    { colKey: 'location', title: '位置（地域 / 可用区）', width: 170 },
    { colKey: 'pool_type', title: '类型', width: 100 },
    { colKey: 'cpu', title: 'CPU（已用/总）', minWidth: 150 },
    { colKey: 'memory', title: '内存（已用/总）', minWidth: 150 },
    { colKey: 'disk', title: '磁盘（已用/总）', minWidth: 150 },
  ]
  if (!isEmbedded.value) {
    base.push({ colKey: 'probe', title: '探针', width: 100 })
  }
  base.push(
    { colKey: 'status', title: '状态', width: 90 },
    {
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : isEmbedded.value ? 90 : 160,
      fixed: 'right' as const,
      align: 'center' as const,
    },
  )
  return base
})

async function loadPools() {
  loading.value = true
  try {
    const data = await getPoolList({
      provider_id: effectiveProviderId.value,
      keyword: filters.keyword || undefined,
      region: filters.region || undefined,
      alert: filters.alert || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    poolList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    summary.value = data.summary || { ...emptySummary }
    regionOptions.value = (data.regions || []).map((region) => ({ label: region, value: region }))
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
  filters.keyword = ''
  filters.provider_id = undefined
  filters.region = ''
  filters.alert = ''
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
    case 'ops':
      if (row.provider_ops_url) window.open(row.provider_ops_url, '_blank', 'noopener')
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

/* 6 张容量/位置卡片：3 列两行比默认 4 列更均衡 */
.resource-module .stat-grid--pools {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.resource-module .stat-card__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.filter-card__grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

@media (max-width: 1200px) and (min-width: 769px) {
  .filter-card__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .resource-module .stat-grid--pools {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.pool-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pool-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.pool-provider {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.loc-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.loc-region {
  font-size: 12px;
  color: var(--color-foreground);
}

.loc-zone {
  font-size: 11px;
  color: var(--color-muted-foreground);
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

.action-gap {
  display: inline-block;
  width: 8px;
}

.probe-hint {
  margin-top: var(--space-md);
}
</style>
