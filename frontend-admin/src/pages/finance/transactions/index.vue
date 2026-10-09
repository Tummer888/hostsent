<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">资金流水</h2>
          <p class="page-header__desc">每一笔余额变动都可按用户、类型、单号与时间定位；汇总与列表同筛选口径</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="exporting" @click="handleExport">
          <template #icon><DownloadIcon aria-hidden="true" /></template>
          导出明细
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadTransactions">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="流水号 / 订单号 / 关联单号 / 用户名" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">用户 ID</span>
        <t-input v-model="filters.user_id" placeholder="精确匹配（可选）" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">流水类型</span>
        <t-select v-model="filters.type" clearable placeholder="全部类型" :options="txTypeOptions" />
      </div>
      <div class="field">
        <span class="field__label">收支方向</span>
        <t-select v-model="filters.direction" clearable placeholder="全部方向" :options="directionOptions" />
      </div>
      <div class="field">
        <span class="field__label">发生时间</span>
        <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="summary-bar surface-card">
      <div class="summary-item">
        <span class="summary-item__label">筛选结果收入</span>
        <span class="summary-item__value amount-income">+{{ formatPrice(summary.income_total) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">筛选结果支出</span>
        <span class="summary-item__value amount-expense">-{{ formatPrice(summary.expense_total) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">净额</span>
        <span class="summary-item__value" :class="summary.net_total >= 0 ? 'amount-income' : 'amount-expense'">
          {{ formatPrice(summary.net_total) }}
        </span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">口径内笔数</span>
        <span class="summary-item__value">{{ summary.tx_count }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">内部划转</span>
        <span class="summary-item__value cell-muted">{{ summary.internal_count }} 笔</span>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">流水列表</h3>
        <span class="table-card__meta">共 {{ total }} 条 · 汇总为全量口径（非当前页）</span>
      </div>
      <t-table
        row-key="id"
        :data="transactionList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #tx_no="{ row }">
          <div class="tx-no-cell">
            <span class="cell-strong">{{ row.tx_no }}</span>
            <span v-if="row.biz_type" class="cell-muted tx-no-cell__biz">{{ row.biz_type }}</span>
          </div>
        </template>

        <template #username="{ row }">
          <div class="user-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="cell-muted">ID {{ row.user_id }}</span>
          </div>
        </template>

        <template #type="{ row }">
          <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ txTypeLabel(row.type) }}
          </t-tag>
        </template>

        <template #amount="{ row }">
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">
            {{ formatAmount(row.amount, row.direction) }}
          </span>
        </template>

        <template #balance="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.balance_after) }}</span>
            <span class="price-sub">前 ¥{{ formatPrice(row.balance_before) }}</span>
          </div>
        </template>

        <template #direction="{ row }">
          <span>{{ directionLabel(row.direction) }}</span>
        </template>

        <template #ref_no="{ row }">
          <t-tooltip v-if="row.ref_no" content="点击复制关联单号">
            <button type="button" class="ref-cell" @click="copyRefNo(row.ref_no)">
              <span class="ref-cell__text">{{ row.ref_no }}</span>
              <CopyIcon size="14" aria-hidden="true" />
            </button>
          </t-tooltip>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无资金流水" />
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
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { onMounted, reactive, ref } from 'vue'

import { CopyIcon, DownloadIcon, MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { exportTransactions, getTransactionList } from '@/api/finance'
import {
  directionLabel,
  directionOptions,
  formatAmount,
  formatPrice,
  formatTime,
  toDateString,
  txTypeLabel,
  txTypeOptions,
  txTypeTheme,
} from '@/pages/finance/constants'
import type { TransactionInfo, TransactionSummary } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'FinanceTransactions' })

const transactionList = ref<TransactionInfo[]>([])
const loading = ref(false)
const exporting = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const emptySummary = (): TransactionSummary => ({
  income_total: 0,
  expense_total: 0,
  net_total: 0,
  tx_count: 0,
  internal_count: 0,
})
const summary = ref<TransactionSummary>(emptySummary())

const filters = reactive<{
  keyword: string | undefined
  user_id: string | undefined
  type: string | undefined
  direction: number | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  keyword: undefined,
  user_id: undefined,
  type: undefined,
  direction: undefined,
  dateRange: [],
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadTransactions）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', minWidth: 200 },
  { colKey: 'username', title: '用户', minWidth: 130 },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'direction', title: '方向', width: 80 },
  { colKey: 'amount', title: '变动金额', width: 130 },
  { colKey: 'balance', title: '变动后余额', width: 140 },
  { colKey: 'ref_no', title: '关联单号', minWidth: 170 },
  { colKey: 'remark', title: '备注', minWidth: 140 },
  { colKey: 'created_at', title: '发生时间', width: 170 },
]

/** 当前筛选条件（列表 / 汇总 / 导出共用同一组参数，避免三者口径不一致）。 */
function buildQuery(page: number, pageSize: number) {
  const [startDate, endDate] = filters.dateRange ?? []
  return {
    keyword: filters.keyword?.trim() || undefined,
    user_id: filters.user_id ? Number(filters.user_id) : undefined,
    type: filters.type,
    direction: filters.direction,
    start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
    end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
    page,
    page_size: pageSize,
  }
}

async function loadTransactions() {
  loading.value = true
  try {
    const data = await getTransactionList(buildQuery(pagination.current, pagination.pageSize))
    transactionList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    summary.value = data.summary ?? emptySummary()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载资金流水失败')
    summary.value = emptySummary()
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTransactions()
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
  loadTransactions()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.user_id = undefined
  filters.type = undefined
  filters.direction = undefined
  filters.dateRange = []
  pagination.current = 1
  loadTransactions()
}

/** 关联单号一键复制：对账时要把单号粘到订单/充值/退款页去核对。 */
async function copyRefNo(refNo: string) {
  try {
    await navigator.clipboard.writeText(refNo)
    MessagePlugin.success('关联单号已复制')
  } catch {
    MessagePlugin.warning('复制失败，请手动选择复制')
  }
}

/** 导出当前筛选条件的明细 CSV（后端按同一组条件、最多 20000 条）。 */
async function handleExport() {
  exporting.value = true
  try {
    const response = await exportTransactions(buildQuery(pagination.current, pagination.pageSize))
    const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
    const objectUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = objectUrl
    link.download = `finance-transactions-${Date.now()}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(objectUrl)
    MessagePlugin.success('明细已导出')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '导出失败')
  } finally {
    exporting.value = false
  }
}

onMounted(loadTransactions)
</script>

<style lang="css">
@import '../shared.css';

.finance-module .summary-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xl);
  padding: var(--space-md) var(--space-xl);
}

.finance-module .summary-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 140px;
}

.finance-module .summary-item__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.finance-module .summary-item__value {
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.finance-module .tx-no-cell,
.finance-module .user-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.finance-module .tx-no-cell__biz {
  font-size: 11px;
}

.finance-module .ref-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 2px 6px;
  border: 1px dashed var(--color-border);
  border-radius: var(--hs-radius-sm, 6px);
  background: transparent;
  color: var(--td-text-color-primary);
  font-size: 12.5px;
  cursor: pointer;
}

.finance-module .ref-cell:hover {
  border-color: var(--finance-green-border);
  background: var(--finance-green-soft);
}

.finance-module .ref-cell__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
