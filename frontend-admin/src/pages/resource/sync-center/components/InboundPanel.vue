<template>
  <div class="inbound-panel">
    <!-- 子视图切换：同步记录（每行=一次同步运行）/ 待确认调价（入站的价格变化下游动作）。 -->
    <t-tabs v-model="view" theme="normal" size="small" class="inbound-tabs">
      <t-tab-panel value="runs" label="同步记录" />
      <t-tab-panel
        v-if="showCatalog"
        value="prices"
        :label="pendingPriceCount > 0 ? `待确认调价（${pendingPriceCount}）` : '待确认调价'"
      />
    </t-tabs>

    <!-- ===== 同步记录：一次同步运行一行 ===== -->
    <template v-if="view === 'runs'">
      <FilterCard embedded class="tab-filters" :primary-count="1">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="runFilter.keyword" clearable placeholder="渠道名 / 失败原因" @enter="searchRuns" />
        </div>
        <div class="field">
          <span class="field__label">渠道</span>
          <t-select v-model="runFilter.provider_id" clearable placeholder="全部渠道" :options="providerOptions" />
        </div>
        <div class="field">
          <span class="field__label">同步类型</span>
          <t-select v-model="runFilter.task_type" clearable placeholder="全部类型" :options="scopeOptions" />
        </div>
        <div class="field">
          <span class="field__label">结果</span>
          <t-select v-model="runFilter.status" clearable placeholder="全部结果" :options="statusOptions" />
        </div>
        <template #actions>
          <t-space size="small">
            <t-radio-group v-model="runFilter.status" variant="default-filled" size="small" @change="searchRuns">
              <t-radio-button value="">全部</t-radio-button>
              <t-radio-button value="failed">仅看失败</t-radio-button>
            </t-radio-group>
            <t-button theme="primary" @click="searchRuns">查询</t-button>
            <t-button variant="outline" @click="resetRunFilters">重置</t-button>
          </t-space>
        </template>
      </FilterCard>

      <p class="table-hint">
        一次同步运行一行：从上游拉取商品目录 / 价格 / 资源池 / 区域 / 实例的结果。 「拉到 /
        变更」是本次写入或更新的条数；失败行展开看错误原文。
      </p>

      <t-table
        row-key="id"
        :data="runList"
        :columns="runColumns"
        :loading="runLoading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : runPagination"
        @page-change="handleRunPageChange"
      >
        <template #provider="{ row }">
          <ProviderCell
            :provider-id="row.provider_id"
            :provider-name="row.provider_name"
            :provider-kind="row.provider_kind"
          />
        </template>
        <template #task_type="{ row }">
          <t-tag :theme="scopeTheme(row.task_type)" variant="light" size="small" shape="round">
            {{ scopeLabel(row.task_type) }}
          </t-tag>
        </template>
        <template #result="{ row }">
          <span class="cell-num">{{ row.success_count }} / {{ row.total_count }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabel(row.status) }}
          </t-tag>
        </template>
        <template #duration="{ row }">
          <span class="cell-muted">{{ durationLabel(row) }}</span>
        </template>
        <template #error_message="{ row }">
          <t-tooltip v-if="row.error_message" :content="row.error_message" placement="top-left">
            <span class="cell-error">{{ row.error_message }}</span>
          </t-tooltip>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #created_at="{ row }">
          <span class="cell-muted">{{ formatTime(row.started_at || row.created_at) }}</span>
        </template>
        <template #row_action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([{ content: '明细', value: 'detail', theme: 'default' }])"
              @select="() => openRunDetail(row)"
            />
            <t-link v-else theme="primary" hover="color" @click="openRunDetail(row)">明细</t-link>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无同步记录" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobileRunPage.current"
        :page-size="mobileRunPage.pageSize"
        :total="mobileRunPage.total"
        @go="goMobileRunPage"
        @page-size="handleMobileRunPageSize"
      />
    </template>

    <!-- ===== 待确认调价（仅上游转售链路） ===== -->
    <template v-else>
      <FilterCard embedded class="tab-filters" :primary-count="1">
        <div class="field">
          <span class="field__label">渠道</span>
          <t-select
            v-model="priceFilter.provider_id"
            clearable
            placeholder="全部渠道"
            :options="providerOptions"
            @change="searchPrices"
          />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select
            v-model="priceFilter.status"
            clearable
            placeholder="全部状态"
            :options="priceStatusOptions"
            @change="searchPrices"
          />
        </div>
        <template #actions>
          <t-space size="small">
            <t-button theme="primary" :disabled="!selectedPriceIds.length" @click="openHandleDialog('confirm')"
              >批量确认</t-button
            >
            <t-button
              theme="danger"
              variant="outline"
              :disabled="!selectedPriceIds.length"
              @click="openHandleDialog('reject')"
              >批量驳回</t-button
            >
          </t-space>
        </template>
      </FilterCard>

      <p class="table-hint">
        上游改价幅度超过渠道阈值时进入待确认：<b>确认前不会改动售价</b>。
        确认后按商品加价规则重算售价（未配置规则则仅更新成本价），并写入价格历史。
      </p>

      <t-table
        row-key="id"
        :data="priceList"
        :columns="priceColumns"
        :loading="priceLoading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pricePagination"
        :selected-row-keys="selectedPriceIds"
        @page-change="handlePricePageChange"
        @select-change="handlePriceSelect"
      >
        <template #provider="{ row }">
          <ProviderCell
            :provider-id="row.provider_id"
            :provider-kind="row.provider_kind"
            :provider-name="providerName(row.provider_id)"
          />
        </template>
        <template #product="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.resource_product_name || `上游商品 #${row.resource_product_id}` }}</span>
            <span v-if="row.upstream_id" class="product-sub">上游 ID {{ row.upstream_id }}</span>
          </div>
        </template>
        <template #field="{ row }">
          <t-tag :theme="row.field === 'cost_price' ? 'primary' : 'warning'" variant="light" size="small" shape="round">
            {{ row.field === 'cost_price' ? '成本价' : '建议售价' }}
          </t-tag>
        </template>
        <template #changed="{ row }">
          <span class="cell-num">
            {{ money(row.old_value) }}
            <span class="cell-arrow">→</span>
            {{ money(row.new_value) }}
          </span>
        </template>
        <template #change_ratio="{ row }">
          <span :class="ratioClass(row)">{{ ratioLabel(row) }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="priceStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ priceStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #handle="{ row }">
          <div v-if="row.status === 'pending'" class="action-cell">
            <t-link theme="primary" hover="color" @click="openHandleDialog('confirm', row)">确认</t-link>
            <t-link theme="danger" hover="color" @click="openHandleDialog('reject', row)">驳回</t-link>
          </div>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #empty>
          <t-empty description="暂无调价事件" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePricePage.current"
        :page-size="mobilePricePage.pageSize"
        :total="mobilePricePage.total"
        @go="goMobilePricePage"
        @page-size="handleMobilePricePageSize"
      />
    </template>

    <!-- 同步明细弹窗：日志与差异都挂在这一条运行下面 -->
    <t-dialog
      v-model:visible="detailVisible"
      :header="detailRow ? `同步明细 · ${detailRow.provider_name || `渠道 #${detailRow.provider_id}`}` : '同步明细'"
      width="640px"
      :footer="false"
      @close="detailVisible = false"
    >
      <template v-if="detailRow">
        <t-descriptions :column="2" bordered size="small">
          <t-descriptions-item label="渠道">
            <ProviderCell
              :provider-id="detailRow.provider_id"
              :provider-name="detailRow.provider_name"
              :provider-kind="detailRow.provider_kind"
            />
          </t-descriptions-item>
          <t-descriptions-item label="同步类型">{{ scopeLabel(detailRow.task_type) }}</t-descriptions-item>
          <t-descriptions-item label="结果">
            <t-tag :theme="statusTheme(detailRow.status)" variant="light" size="small" shape="round">
              {{ statusLabel(detailRow.status) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="拉到 / 变更"
            >{{ detailRow.success_count }} / {{ detailRow.total_count }}</t-descriptions-item
          >
          <t-descriptions-item label="开始">{{
            formatTime(detailRow.started_at || detailRow.created_at)
          }}</t-descriptions-item>
          <t-descriptions-item label="结束">{{ formatTime(detailRow.completed_at) }}</t-descriptions-item>
          <t-descriptions-item v-if="detailRow.error_message" label="错误原文" :span="2">
            <span class="cell-error cell-error--wrap">{{ detailRow.error_message }}</span>
          </t-descriptions-item>
        </t-descriptions>

        <!-- 差异记录：这条运行改了什么（新增/字段更新/上游下线/价格变化） -->
        <div class="detail-section__head">
          本次变更
          <span v-if="detailDiffs.length" class="detail-section__count">{{ detailDiffs.length }} 条</span>
        </div>
        <t-table
          v-if="detailDiffs.length"
          row-key="id"
          :data="detailDiffs"
          :columns="diffColumns"
          size="small"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #action="{ row }">
            <t-tag :theme="diffActionTheme(row.action)" variant="light" size="small" shape="round">
              {{ diffActionLabel(row.action) }}
            </t-tag>
          </template>
          <template #changed="{ row }">
            <span v-if="row.field" class="cell-num">
              {{ row.field }}：{{ row.old_value || '空' }} <span class="cell-arrow">→</span> {{ row.new_value || '空' }}
            </span>
            <span v-else class="cell-muted">{{ row.external_id || '—' }}</span>
          </template>
        </t-table>
        <t-empty
          v-else
          :description="detailDiffLoading ? '加载中…' : '本次运行没有产生字段变更（或差异已过保留期）'"
          size="small"
        />
      </template>
    </t-dialog>

    <!-- 调价处置弹窗 -->
    <t-dialog
      v-model:visible="handleDialogVisible"
      :header="handleAction === 'confirm' ? '确认调价' : '驳回调价'"
      width="520px"
      :confirm-btn="{
        content: handleAction === 'confirm' ? '确认并应用' : '驳回',
        theme: handleAction === 'confirm' ? 'primary' : 'danger',
        loading: handleSaving,
      }"
      cancel-btn="取消"
      :on-confirm="submitHandle"
    >
      <t-alert
        :theme="handleAction === 'confirm' ? 'info' : 'warning'"
        class="handle-alert"
        :message="
          handleAction === 'confirm'
            ? '确认后按商品加价规则重算售价并写入价格历史；未配置加价规则时仅更新成本价。'
            : '驳回后保持原售价不变，本次调价不再提示（上游再次调价会重新产生事件）。'
        "
      />
      <p class="handle-count">将处理 {{ handleIds.length }} 条调价事件</p>
      <t-textarea v-model="handleRemark" :autosize="{ minRows: 2 }" placeholder="备注（可选，会随处置记录留痕）" />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'

import { MessagePlugin, type PageInfo, type PrimaryTableCol, type SelectOptions } from 'tdesign-vue-next'

import { getPriceChangeList, getSyncDiffList, getSyncTaskList, handlePriceChanges } from '@/api/admin'
import type { PriceChangeInfo, ProviderInfo, SyncDiffInfo, SyncTaskInfo } from '@/types/interface'
import FilterCard from '@/components/filter-card/index.vue'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import ProviderCell from './ProviderCell.vue'

/**
 * 入站同步面板（上游 → 我方）。
 *
 * 设计要点：把原先三个页签（目录与状态同步 / 同步日志 / 异常与失败）合并成「一次运行一行」。
 * 原因：后端每次同步任务固定写一条同字段的 sync_logs（见 sync_engine.writeLog），
 * 日志表只是任务表的镜像，「异常」又是它的失败子集——三张表看的是同一件事，
 * 分成三个页签只会让运营在三处之间来回核对。
 * 现在：失败用「仅看失败」筛选（不再是独立页签），明细挂到行内弹窗。
 */
const props = defineProps<{
  /** 当前链路筛选（'' = 全部）。 */
  chainKind: string
  /** 渠道下拉（父级已加载，避免每个面板各查一次）。 */
  providers: ProviderInfo[]
  /** scope 下拉（父级已加载）。 */
  scopes: { label: string; value: string }[]
  /** 是否具备上游目录语义（自营平台为 false，隐藏待确认调价子视图）。 */
  showCatalog: boolean
}>()

const emit = defineEmits<{
  (e: 'refresh-price-count', count: number): void
  // 调价处置会改变成本价，可能让对账结论变化——通知父级重取未闭环差异计数。
  (e: 'refresh-diffs'): void
}>()

const { isMobile } = useIsMobile()

const providerOptions = computed(() => props.providers.map((item) => ({ label: item.name, value: item.id })))
const scopeOptions = computed(() => props.scopes)
const providerNameMap = computed<Record<number, string>>(() => {
  const map: Record<number, string> = {}
  for (const item of props.providers) map[item.id] = item.name
  return map
})
function providerName(id: number): string {
  return providerNameMap.value[id] || ''
}

const view = ref<'runs' | 'prices'>('runs')

// ---------- 通用展示 ----------

function formatTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function money(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—'
  return Number(value).toFixed(2)
}

function scopeLabel(scope: string): string {
  const found = scopeOptions.value.find((item) => item.value === scope)
  return found ? found.label : scope
}

function scopeTheme(scope: string): string {
  if (scope === 'catalog') return 'primary'
  if (scope === 'price') return 'warning'
  if (scope === 'pool') return 'success'
  if (scope === 'region') return 'default'
  if (scope === 'instance') return 'danger'
  return 'default'
}

const statusLabels: Record<string, string> = {
  pending: '待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  skipped: '已跳过',
}

function statusLabel(status: string): string {
  return statusLabels[status] || status || '—'
}

function statusTheme(status: string): string {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'default'
}

const statusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '执行中', value: 'running' },
  { label: '待执行', value: 'pending' },
  { label: '已跳过', value: 'skipped' },
]

/** 耗时：开始到结束的时长；未结束显示「进行中」。 */
function durationLabel(row: SyncTaskInfo): string {
  if (!row.started_at) return '—'
  if (!row.completed_at) return '进行中'
  const ms = new Date(row.completed_at).getTime() - new Date(row.started_at).getTime()
  if (!Number.isFinite(ms) || ms < 0) return '—'
  if (ms < 1000) return `${ms} ms`
  const sec = ms / 1000
  if (sec < 60) return `${sec.toFixed(1)} 秒`
  return `${Math.floor(sec / 60)} 分 ${Math.round(sec % 60)} 秒`
}

// ---------- 同步记录 ----------

const runList = ref<SyncTaskInfo[]>([])
const runLoading = ref(false)
const runFilter = reactive<{
  keyword: string
  provider_id: number | undefined
  task_type: string | undefined
  status: string
}>({ keyword: '', provider_id: undefined, task_type: undefined, status: '' })
const runPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobileRunPage = reactive({ current: 1, pageSize: 10, total: 0 })

const runColumns: PrimaryTableCol<SyncTaskInfo>[] = [
  { colKey: 'provider', title: '渠道', minWidth: 170 },
  { colKey: 'task_type', title: '同步类型', width: 110 },
  { colKey: 'result', title: '拉到 / 变更', width: 110 },
  { colKey: 'status', title: '结果', width: 90 },
  { colKey: 'duration', title: '耗时', width: 100 },
  { colKey: 'error_message', title: '失败原因', minWidth: 200 },
  { colKey: 'created_at', title: '开始时间', minWidth: 170 },
  {
    colKey: 'row_action',
    title: '操作',
    width: isMobile.value ? 70 : 80,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadRuns() {
  runLoading.value = true
  try {
    const data = await getSyncTaskList({
      provider_id: runFilter.provider_id,
      task_type: runFilter.task_type,
      status: runFilter.status || undefined,
      provider_kind: props.chainKind || undefined,
      page: runPagination.current,
      page_size: runPagination.pageSize,
    })
    // 关键词按渠道名/错误原因在前端过滤：任务表没有关键词入参，
    // 而这两个字段恰好是运营在同步记录里唯一会搜的东西。
    const kw = runFilter.keyword.trim().toLowerCase()
    runList.value = kw
      ? data.items.filter(
          (item) =>
            (item.provider_name || '').toLowerCase().includes(kw) ||
            (item.error_message || '').toLowerCase().includes(kw),
        )
      : data.items
    runPagination.total = data.meta.total
    mobileRunPage.total = data.meta.total
    mobileRunPage.current = runPagination.current
    mobileRunPage.pageSize = runPagination.pageSize
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载同步记录失败')
  } finally {
    runLoading.value = false
  }
}

function searchRuns() {
  runPagination.current = 1
  loadRuns()
}

function resetRunFilters() {
  runFilter.keyword = ''
  runFilter.provider_id = undefined
  runFilter.task_type = undefined
  runFilter.status = ''
  searchRuns()
}

function handleRunPageChange(pageInfo: PageInfo) {
  runPagination.current = pageInfo.current
  runPagination.pageSize = pageInfo.pageSize
  loadRuns()
}

async function goMobileRunPage(target: number) {
  const max = Math.max(1, Math.ceil(mobileRunPage.total / mobileRunPage.pageSize))
  const clamped = Math.min(Math.max(target, 1), max)
  if (clamped === mobileRunPage.current) return
  runPagination.current = clamped
  runPagination.pageSize = mobileRunPage.pageSize
  await loadRuns()
}

async function handleMobileRunPageSize(size: number) {
  runPagination.pageSize = size
  runPagination.current = 1
  await loadRuns()
}

// ---------- 运行明细（变更记录） ----------

const detailVisible = ref(false)
const detailRow = ref<SyncTaskInfo | null>(null)
const detailDiffs = ref<SyncDiffInfo[]>([])
const detailDiffLoading = ref(false)

const diffColumns: PrimaryTableCol<SyncDiffInfo>[] = [
  { colKey: 'action', title: '变更类型', width: 110 },
  { colKey: 'changed', title: '变更内容', minWidth: 260 },
]

const diffActionLabels: Record<string, string> = {
  created: '新增',
  updated: '字段更新',
  offline: '上游下线',
  missing: '本地缺失',
  price_changed: '价格变化',
}

function diffActionLabel(action: string): string {
  return diffActionLabels[action] || action
}

function diffActionTheme(action: string): string {
  if (action === 'created') return 'success'
  if (action === 'offline') return 'danger'
  if (action === 'updated') return 'primary'
  if (action === 'missing') return 'warning'
  if (action === 'price_changed') return 'warning'
  return 'default'
}

async function openRunDetail(row: SyncTaskInfo) {
  detailRow.value = row
  detailDiffs.value = []
  detailVisible.value = true
  detailDiffLoading.value = true
  try {
    const data = await getSyncDiffList({ task_id: row.id, page: 1, page_size: 100 })
    detailDiffs.value = data.items
  } catch {
    // 明细拉取失败不影响主信息（渠道/时间/结果已在行上）。
    detailDiffs.value = []
  } finally {
    detailDiffLoading.value = false
  }
}

// ---------- 待确认调价 ----------

const priceList = ref<PriceChangeInfo[]>([])
const priceLoading = ref(false)
const priceFilter = reactive<{ provider_id: number | undefined; status: string | undefined }>({
  provider_id: undefined,
  status: undefined,
})
const pricePagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePricePage = reactive({ current: 1, pageSize: 10, total: 0 })
const selectedPriceIds = ref<number[]>([])
const pendingPriceCount = ref(0)

const priceStatusOptions = [
  { label: '待确认', value: 'pending' },
  { label: '已确认', value: 'confirmed' },
  { label: '已驳回', value: 'rejected' },
  { label: '自动应用', value: 'auto_applied' },
]

const priceStatusLabels: Record<string, string> = {
  pending: '待确认',
  confirmed: '已确认',
  rejected: '已驳回',
  auto_applied: '自动应用',
}

function priceStatusLabel(status: string): string {
  return priceStatusLabels[status] || status
}

function priceStatusTheme(status: string): string {
  if (status === 'pending') return 'warning'
  if (status === 'confirmed') return 'success'
  if (status === 'rejected') return 'danger'
  return 'default'
}

function ratioLabel(row: PriceChangeInfo): string {
  if (row.change_ratio === null || row.change_ratio === undefined) return '—'
  const pct = Number(row.change_ratio) * 100
  const threshold = row.threshold ? Number(row.threshold) * 100 : 0
  return `${pct.toFixed(2)}%${threshold ? `（阈值 ${threshold.toFixed(2)}%）` : ''}`
}

function ratioClass(row: PriceChangeInfo): string {
  if (row.status === 'pending') return 'cell-ratio cell-ratio--high'
  return 'cell-ratio'
}

const priceColumns: PrimaryTableCol<PriceChangeInfo>[] = [
  { colKey: 'row-select', type: 'multiple', width: 50 },
  { colKey: 'provider', title: '渠道', width: 170 },
  { colKey: 'product', title: '上游商品', minWidth: 200 },
  { colKey: 'field', title: '口径', width: 100 },
  { colKey: 'changed', title: '变动', minWidth: 150 },
  { colKey: 'change_ratio', title: '幅度', minWidth: 170 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '时间', minWidth: 160 },
  { colKey: 'handle', title: '操作', width: 110, fixed: 'right' as const, align: 'center' as const },
]

async function loadPrices() {
  priceLoading.value = true
  try {
    const data = await getPriceChangeList({
      provider_id: priceFilter.provider_id,
      status: priceFilter.status,
      provider_kind: props.chainKind || undefined,
      page: pricePagination.current,
      page_size: pricePagination.pageSize,
    })
    priceList.value = data.items
    pricePagination.total = data.meta.total
    mobilePricePage.total = data.meta.total
    mobilePricePage.current = pricePagination.current
    mobilePricePage.pageSize = pricePagination.pageSize
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载调价事件失败')
  } finally {
    priceLoading.value = false
  }
}

/** 待确认数量：驱动子视图页签角标，让"有活要干"在页面顶部就能看见。 */
async function loadPendingPriceCount() {
  if (!props.showCatalog) {
    pendingPriceCount.value = 0
    emit('refresh-price-count', 0)
    return
  }
  try {
    const data = await getPriceChangeList({
      status: 'pending',
      provider_kind: props.chainKind || undefined,
      page: 1,
      page_size: 1,
    })
    pendingPriceCount.value = data.meta.total
    emit('refresh-price-count', data.meta.total)
  } catch {
    pendingPriceCount.value = 0
  }
}

function searchPrices() {
  pricePagination.current = 1
  selectedPriceIds.value = []
  loadPrices()
}

function handlePricePageChange(pageInfo: PageInfo) {
  pricePagination.current = pageInfo.current
  pricePagination.pageSize = pageInfo.pageSize
  loadPrices()
}

async function goMobilePricePage(target: number) {
  const max = Math.max(1, Math.ceil(mobilePricePage.total / mobilePricePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), max)
  if (clamped === mobilePricePage.current) return
  pricePagination.current = clamped
  pricePagination.pageSize = mobilePricePage.pageSize
  await loadPrices()
}

async function handleMobilePricePageSize(size: number) {
  pricePagination.pageSize = size
  pricePagination.current = 1
  await loadPrices()
}

function handlePriceSelect(keys: Array<string | number>, options: SelectOptions<PriceChangeInfo>) {
  void options
  selectedPriceIds.value = keys.map((key) => Number(key))
}

const handleDialogVisible = ref(false)
const handleSaving = ref(false)
const handleAction = ref<'confirm' | 'reject'>('confirm')
const handleIds = ref<number[]>([])
const handleRemark = ref('')

function openHandleDialog(action: 'confirm' | 'reject', row?: PriceChangeInfo) {
  handleAction.value = action
  handleIds.value = row ? [row.id] : [...selectedPriceIds.value]
  handleRemark.value = ''
  if (!handleIds.value.length) {
    MessagePlugin.warning('请先选择待处理事件')
    return
  }
  handleDialogVisible.value = true
}

async function submitHandle() {
  if (!handleIds.value.length) return
  handleSaving.value = true
  try {
    const res = await handlePriceChanges({
      ids: handleIds.value,
      action: handleAction.value,
      remark: handleRemark.value,
    })
    MessagePlugin.success(`已处理 ${res.handled} 条调价事件`)
    handleDialogVisible.value = false
    selectedPriceIds.value = []
    loadPrices()
    loadPendingPriceCount()
    emit('refresh-diffs')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '处理调价事件失败')
  } finally {
    handleSaving.value = false
  }
}

// ---------- 对外入口 ----------

/** 被「异常与失败」旧入口调用时，切到只看失败的同步记录。 */
function showFailedOnly() {
  view.value = 'runs'
  runFilter.status = 'failed'
  searchRuns()
}

async function reload() {
  if (view.value === 'prices' && props.showCatalog) {
    await loadPrices()
    return
  }
  await loadRuns()
}

// 链路切换后两个子视图都要按新链路重查。
async function reloadForChain() {
  await loadPendingPriceCount()
  await reload()
}

defineExpose({ reload, reloadForChain, showFailedOnly })

// 子视图切换时才拉对应数据。
//
// 修复的缺陷（用户反馈）：切到「待确认调价」后表格全空，而页签上明明标着待办 210。
// 原因是切换 view 只改了渲染分支，没有任何地方触发 loadPrices —— 列表数据从未被请求，
// 而页签上的计数来自另一条独立查询（loadPendingPriceCount），两者口径脱节。
// 「计数可见但列表为空」是最难自查的一类半加载状态，因此这里用 watch 显式绑住加载。
watch(view, (next) => {
  if (next === 'prices' && props.showCatalog) {
    searchPrices()
  } else {
    loadRuns()
  }
})

onMounted(async () => {
  await loadRuns()
  void loadPendingPriceCount()
})
</script>

<style scoped lang="css">
.inbound-tabs {
  margin-bottom: var(--space-sm);
}

.tab-filters {
  margin: var(--space-md) 0 var(--space-md);
}

.table-hint {
  margin: 0 0 var(--space-md);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.cell-strong {
  font-weight: 600;
  color: var(--color-foreground);
}

.cell-num {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.cell-muted {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.cell-arrow {
  margin: 0 4px;
  color: var(--color-muted-foreground);
}

.cell-error {
  font-size: 12px;
  color: #dc2626;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.cell-error--wrap {
  display: block;
  -webkit-line-clamp: unset;
  word-break: break-all;
  white-space: pre-wrap;
}

.cell-ratio {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--color-muted-foreground);
}

.cell-ratio--high {
  color: #d97706;
  font-weight: 600;
}

.product-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.product-sub {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.detail-section__head {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
  margin: var(--space-lg) 0 var(--space-xs);
  font-weight: 600;
}

.detail-section__count {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-muted-foreground);
}

.handle-alert {
  margin-bottom: var(--space-md);
}

.handle-count {
  margin: 0 0 var(--space-sm);
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
