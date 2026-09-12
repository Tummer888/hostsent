<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">实例对账</h2>
          <p class="page-header__desc">
            已开通并对接上游的实例：本地售价 vs 上游成本价、本地到期 vs 上游到期日期，逐台检测定价与账期异常
          </p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="reload">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="stat-grid">
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

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <span class="filter-card__meta">
          到期容差 {{ summary.tolerance_days }} 天 · 毛利率预警线 {{ summary.thin_margin_rate }}%
        </span>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="实例名 / 实例号 / 商品名" @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">所属渠道</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部渠道" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">异常类型</span>
          <t-select v-model="filters.anomaly" clearable placeholder="全部实例" :options="anomalyOptions" />
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
        <h3 class="card-title">对账结果</h3>
        <span class="table-card__meta">共 {{ total }} 台上游实例</span>
      </div>
      <t-table
        row-key="id"
        :data="rows"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #instance="{ row }">
          <div class="subject-cell">
            <span class="subject-name">{{ row.name || row.instance_id }}</span>
            <span class="subject-meta">
              <span class="subject-ref">{{ row.instance_id }}</span>
              <t-tag :theme="row.status === 'running' ? 'success' : 'default'" variant="light" size="small" shape="round">
                {{ row.status_name }}
              </t-tag>
            </span>
            <span class="subject-meta">
              <span class="subject-ref">{{ row.provider_name || `渠道 #${row.provider_id}` }}</span>
              <span v-if="row.username" class="subject-ref">· {{ row.username }}</span>
            </span>
          </div>
        </template>

        <template #price="{ row }">
          <div class="pair-cell">
            <div class="pair-row">
              <span class="pair-label">售价</span>
              <span class="pair-value">{{ money(row.local_price) }}</span>
            </div>
            <div class="pair-row">
              <span class="pair-label">成本</span>
              <span class="pair-value">{{ money(row.upstream_cost) }}</span>
            </div>
            <div class="pair-row pair-row--margin">
              <span class="pair-label">毛利</span>
              <span class="pair-value" :class="marginClass(row.margin_amount)">
                {{ money(row.margin_amount) }}<template v-if="row.margin_rate !== null">（{{ row.margin_rate.toFixed(1) }}%）</template>
              </span>
            </div>
          </div>
        </template>

        <template #expire="{ row }">
          <div class="pair-cell">
            <div class="pair-row">
              <span class="pair-label">本地</span>
              <span class="pair-value">{{ formatDate(row.local_expire_at) }}</span>
            </div>
            <div class="pair-row">
              <span class="pair-label">上游</span>
              <span class="pair-value">{{ formatDate(row.upstream_expire_at) }}</span>
            </div>
            <div v-if="row.expire_diff_days !== null" class="pair-row pair-row--margin">
              <span class="pair-label">偏差</span>
              <span class="pair-value" :class="row.expire_anomaly === 'ok' ? '' : 'value-warn'">
                {{ row.expire_diff_days > 0 ? '早' : row.expire_diff_days < 0 ? '晚' : '一致' }}
                <template v-if="row.expire_diff_days !== 0">{{ Math.abs(row.expire_diff_days).toFixed(1) }} 天</template>
              </span>
            </div>
          </div>
        </template>

        <template #anomaly="{ row }">
          <div class="anomaly-cell">
            <template v-if="row.anomalies.length">
              <t-tag
                v-for="item in row.anomalies"
                :key="item.code"
                :theme="item.severity === 'danger' ? 'danger' : 'warning'"
                variant="light"
                size="small"
                shape="round"
              >
                {{ item.name }}
              </t-tag>
            </template>
            <t-tag v-else theme="success" variant="light" size="small" shape="round">一致</t-tag>
          </div>
        </template>

        <template #severity="{ row }">
          <t-tag :theme="severityTheme(row.severity)" variant="light" size="small" shape="round">
            {{ row.severity_name }}
          </t-tag>
        </template>

        <template #row_action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([{ content: '详情', value: 'detail', theme: 'default' }])"
              @select="() => openDetailDialog(row)"
            />
            <t-link v-else theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无上游链路实例可对账" />
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
      header="对账详情"
      width="640px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-alert
        v-if="detailRow"
        :theme="detailRow.severity === 'danger' ? 'error' : detailRow.severity === 'warning' ? 'warning' : 'success'"
        :message="detailRow.detail"
        class="detail-alert"
      />
      <t-descriptions v-if="detailRow" :column="2" bordered size="small">
        <t-descriptions-item label="实例">{{ detailRow.name || detailRow.instance_id }}</t-descriptions-item>
        <t-descriptions-item label="实例号">{{ detailRow.instance_id }}</t-descriptions-item>
        <t-descriptions-item label="服务状态">{{ detailRow.status_name }}</t-descriptions-item>
        <t-descriptions-item label="所属渠道">
          {{ detailRow.provider_name || `渠道 #${detailRow.provider_id}` }}
        </t-descriptions-item>
        <t-descriptions-item label="归属用户">{{ detailRow.username || `用户 #${detailRow.user_id}` }}</t-descriptions-item>
        <t-descriptions-item label="异常等级">
          <t-tag :theme="severityTheme(detailRow.severity)" variant="light" size="small" shape="round">
            {{ detailRow.severity_name }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="本地售出商品">
          {{ detailRow.sell_product_name || (detailRow.sell_product_id ? `商品 #${detailRow.sell_product_id}` : '未绑定') }}
        </t-descriptions-item>
        <t-descriptions-item label="上游资源商品">
          {{ detailRow.upstream_product_name || (detailRow.upstream_product_id ? `商品 #${detailRow.upstream_product_id}` : '未绑定') }}
        </t-descriptions-item>
        <t-descriptions-item label="上游 SKU">{{ detailRow.upstream_sku || '—' }}</t-descriptions-item>
        <t-descriptions-item label="毛利">
          {{ money(detailRow.margin_amount) }}
          <template v-if="detailRow.margin_rate !== null">（{{ detailRow.margin_rate.toFixed(1) }}%）</template>
        </t-descriptions-item>
        <t-descriptions-item label="本地售价">{{ money(detailRow.local_price) }}</t-descriptions-item>
        <t-descriptions-item label="上游成本">{{ money(detailRow.upstream_cost) }}</t-descriptions-item>
        <t-descriptions-item label="本地到期">{{ formatDate(detailRow.local_expire_at) }}</t-descriptions-item>
        <t-descriptions-item label="上游到期">{{ formatDate(detailRow.upstream_expire_at) }}</t-descriptions-item>
        <t-descriptions-item label="到期偏差" :span="2">
          <template v-if="detailRow.expire_diff_days !== null">
            上游到期 {{ detailRow.expire_diff_days > 0 ? '晚于' : detailRow.expire_diff_days < 0 ? '早于' : '等于' }} 本地
            <template v-if="detailRow.expire_diff_days !== 0">{{ Math.abs(detailRow.expire_diff_days).toFixed(1) }} 天</template>
          </template>
          <template v-else>无法比对（缺一侧到期时间）</template>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { ErrorCircleIcon, RefreshIcon, SearchIcon, VerifyIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderList, getReconcileList } from '@/api/admin'
import type { ProviderInfo, ReconcileItem, ReconcileSummary } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ResourceReconcile' })

const rows = ref<ReconcileItem[]>([])
const loading = ref(false)
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const { isMobile } = useIsMobile()

const emptySummary: ReconcileSummary = {
  total: 0,
  danger: 0,
  warning: 0,
  ok: 0,
  below_cost: 0,
  thin_margin: 0,
  missing_price: 0,
  expire_early: 0,
  expire_late: 0,
  missing_expire: 0,
  negative_margin: 0,
  total_margin: 0,
  avg_margin_rate: 0,
  tolerance_days: 1,
  thin_margin_rate: 5,
}
const summary = ref<ReconcileSummary>({ ...emptySummary })

const anomalyOptions = [
  { label: '严重（严重亏损 / 账期错位）', value: 'danger' },
  { label: '预警（薄利 / 数据缺失）', value: 'warning' },
  { label: '仅售价低于成本', value: 'below_cost' },
  { label: '仅毛利过薄', value: 'thin_margin' },
  { label: '仅本地到期过早', value: 'expire_early' },
  { label: '仅本地到期过晚', value: 'expire_late' },
  { label: '仅数据缺失', value: 'missing' },
  { label: '无异常', value: 'ok' },
]

const filters = reactive<{ keyword: string; provider_id: number | undefined; anomaly: string }>({
  keyword: '',
  provider_id: undefined,
  anomaly: '',
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const statCards = computed(() => {
  const s = summary.value
  return [
    {
      key: 'total',
      label: '对账实例数',
      value: `${s.total}`,
      hint: '已开通的上游链路实例',
      icon: VerifyIcon,
      theme: 'info' as const,
    },
    {
      key: 'danger',
      label: '严重异常',
      value: `${s.danger}`,
      hint: `低于成本 ${s.below_cost} · 到期错位 ${s.expire_early + s.expire_late}`,
      icon: ErrorCircleIcon,
      theme: s.danger > 0 ? ('danger' as const) : ('info' as const),
    },
    {
      key: 'margin',
      label: '平均毛利率',
      value: `${s.avg_margin_rate.toFixed(1)}%`,
      hint: `毛利合计 ￥${s.total_margin.toFixed(2)}`,
      icon: VerifyIcon,
      theme: s.avg_margin_rate < 0 ? ('danger' as const) : s.avg_margin_rate < s.thin_margin_rate ? ('warning' as const) : ('success' as const),
    },
    {
      key: 'warning',
      label: '预警',
      value: `${s.warning}`,
      hint: `薄利 ${s.thin_margin} · 缺数据 ${s.missing_price + s.missing_expire}`,
      icon: ErrorCircleIcon,
      theme: s.warning > 0 ? ('warning' as const) : ('info' as const),
    },
  ]
})

const columns = computed<PrimaryTableCol<ReconcileItem>[]>(() => {
  const base: PrimaryTableCol<ReconcileItem>[] = [
    { colKey: 'instance', title: '实例', minWidth: 200 },
    { colKey: 'price', title: '售价 / 成本 / 毛利', minWidth: 190 },
    { colKey: 'expire', title: '到期（本地 / 上游）', minWidth: 190 },
    { colKey: 'anomaly', title: '异常项', minWidth: 180 },
    { colKey: 'severity', title: '等级', width: 90 },
  ]
  base.push({
    colKey: 'row_action',
    title: '操作',
    width: isMobile.value ? 70 : 80,
    fixed: 'right' as const,
    align: 'center' as const,
  })
  return base
})

function money(value: number | null): string {
  if (value === null || value === undefined) return '—'
  return `￥${value.toFixed(2)}`
}

function marginClass(value: number | null): string {
  if (value === null || value === undefined) return ''
  if (value < 0) return 'value-danger'
  if (value === 0) return 'value-warn'
  return 'value-ok'
}

function formatDate(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function severityTheme(severity: string): 'danger' | 'warning' | 'success' | 'default' {
  if (severity === 'danger') return 'danger'
  if (severity === 'warning') return 'warning'
  if (severity === 'ok') return 'success'
  return 'default'
}

async function loadRows() {
  loading.value = true
  try {
    const data = await getReconcileList({
      keyword: filters.keyword || undefined,
      provider_id: filters.provider_id,
      anomaly: filters.anomaly || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    rows.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    summary.value = data.summary || { ...emptySummary }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例对账失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 渠道下拉加载失败不阻塞列表 */
  }
}

function reload() {
  pagination.current = 1
  loadRows()
}

function handleSearch() {
  pagination.current = 1
  loadRows()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.provider_id = undefined
  filters.anomaly = ''
  pagination.current = 1
  loadRows()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadRows()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

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
  await loadRows()
  mobilePage.total = pagination.total
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

const detailVisible = ref(false)
const detailRow = ref<ReconcileItem | null>(null)

function openDetailDialog(row: ReconcileItem) {
  detailRow.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadProviders()
  loadRows()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

.resource-module .stat-card__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.resource-module .filter-card__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.filter-card__grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

@media (max-width: 1200px) and (min-width: 769px) {
  .filter-card__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-card__grid {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }
}

.subject-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.subject-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.subject-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.subject-ref {
  font-size: 11px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.pair-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pair-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pair-row--margin {
  border-top: 1px dashed var(--color-border);
  margin-top: 2px;
  padding-top: 2px;
}

.pair-label {
  flex: 0 0 28px;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.pair-value {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.value-danger {
  color: #dc2626;
  font-weight: 600;
}

.value-warn {
  color: #d97706;
  font-weight: 600;
}

.value-ok {
  color: #16a34a;
}

.anomaly-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.detail-alert {
  margin-bottom: var(--space-md);
}
</style>
