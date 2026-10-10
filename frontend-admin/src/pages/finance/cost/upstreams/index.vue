<template>
  <div class="page-body finance-module cost-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">上游余额台账</h2>
          <p class="page-header__desc">
            {{ ledger.month || monthValue }} 各上游渠道的余额与流水 —— 消耗/充值来自上游账本，
            余额每天自动抓取一次；无需手工录入
          </p>
        </div>
      </div>
      <t-space size="small" align="center">
        <t-date-picker
          v-model="monthValue"
          mode="month"
          format="YYYY-MM"
          :clearable="false"
          placeholder="选择月份"
          style="width: 150px"
          @change="handleMonthChange"
        />
        <t-button
          v-if="balanceSupported.length"
          variant="outline"
          :loading="fetchingAll"
          @click="handleFetchAll"
        >
          <template #icon><CloudDownloadIcon aria-hidden="true" /></template>
          抓取全部（{{ balanceSupported.length }}）
        </t-button>
        <t-button
          v-if="ledgerSupported.length"
          variant="outline"
          :loading="syncingLedger"
          @click="handleSyncLedger(false)"
        >
          <template #icon><CurrencyExchangeIcon aria-hidden="true" /></template>
          同步上游账本
        </t-button>
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <t-alert
      v-for="alert in visibleAlerts"
      :key="alert.provider_id"
      :theme="alertTheme(alert.level)"
      class="cost-hint"
      :message="`【${alert.provider_name}】${alert.message}`"
    >
      <template #operation>
        <span class="table-card__meta">上游余额水位</span>
      </template>
    </t-alert>

    <t-alert theme="info" class="cost-hint">
      <template #message>
        上游成本<strong>全自动取数</strong>：支持账本的渠道由系统自动同步<strong>消费与充值流水</strong>
        （逐笔可查、每天增量同步一次），余额每天自动抓取一次（抓取时点见「财务配置 → 上游余额自动快照小时」）。
        <strong>首次使用</strong>：点一次「同步上游账本」回填历史，成本趋势立刻有数；
        未接入账本的渠道没有自动成本，可用「成本项配置」登记该渠道的固定/一次性支出。
      </template>
    </t-alert>

    <section class="summary-bar surface-card">
      <div class="summary-item">
        <span class="summary-item__label">期间消耗合计</span>
        <span class="summary-item__value amount-expense">¥{{ formatPrice(ledger.total_consumption) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">期间充值合计</span>
        <span class="summary-item__value amount-income">¥{{ formatPrice(ledger.total_topup) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">跟踪渠道</span>
        <span class="summary-item__value">{{ ledger.rows.length }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">支持账本同步</span>
        <span class="summary-item__value">{{ ledgerSupported.length }}</span>
      </div>
      <div class="summary-item summary-item--hint">
        <span class="summary-item__label">账本同步</span>
        <span class="summary-item__value cell-muted">
          {{ ledgerSyncedLabel }}
        </span>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">渠道余额与流水</h3>
        <span class="table-card__meta">
          「最新余额」为每日自动抓取的余额快照；消耗/充值取自上游账本流水
        </span>
      </div>
      <t-table
        row-key="provider_id"
        :data="ledger.rows"
        :columns="ledgerColumns"
        :loading="loading"
        size="small"
        hover
        cell-empty-content="—"
      >
        <template #provider_name="{ row }">
          <div class="provider-cell">
            <span class="cell-strong">{{ row.provider_name || `#${row.provider_id}` }}</span>
            <span class="cell-muted">
              {{ row.provider_type || '—' }}
              <template v-if="isBalanceSupported(row.provider_id)"> · 支持抓取</template>
            </span>
          </div>
        </template>
        <template #latest="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ moneyText(row.latest_balance) }}</span>
            <span class="price-sub">{{ row.latest_date || '暂无快照' }}</span>
          </div>
        </template>
        <template #consumption="{ row }">
          <div class="price-cell">
            <span v-if="row.consumption !== null" class="price-main amount-expense">
              ¥{{ formatPrice(row.consumption) }}
            </span>
            <span v-else class="cell-muted">—</span>
            <span v-if="row.cost_source === 'ledger'" class="price-sub">{{ row.consumption_entries }} 笔流水</span>
            <span v-else class="price-sub">{{ sourceHint(row) }}</span>
          </div>
        </template>
        <template #topup_total="{ row }">
          <span v-if="row.topup_total !== null" class="amount-income">{{ formatAmount(row.topup_total) }}</span>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #cost_source="{ row }">
          <t-tag v-if="row.cost_source === 'ledger'" theme="primary" variant="light" size="small" shape="round">
            账本流水
          </t-tag>
          <t-tag v-else-if="isLedgerSupported(row.provider_id)" theme="warning" variant="light" size="small" shape="round">
            待同步
          </t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">未接入账本</t-tag>
        </template>
        <template #op="{ row }">
          <div class="action-cell">
            <t-tooltip
              :content="isBalanceSupported(row.provider_id) ? '调用渠道接口抓取当前余额并落今日快照' : '该渠道类型未实现余额查询接口'"
            >
              <t-link
                theme="primary"
                :disabled="!isBalanceSupported(row.provider_id) || fetchingId === row.provider_id"
                @click="handleFetch(row)"
              >
                {{ fetchingId === row.provider_id ? '抓取中…' : '抓取余额' }}
              </t-link>
            </t-tooltip>
            <t-tooltip
              :content="isLedgerSupported(row.provider_id) ? '增量同步该渠道的消费/充值流水' : '该渠道类型未实现账本读取'"
            >
              <t-link
                theme="primary"
                :disabled="!isLedgerSupported(row.provider_id) || syncingId === row.provider_id"
                @click="handleSyncLedger(false, row.provider_id)"
              >
                {{ syncingId === row.provider_id ? '同步中…' : '同步账本' }}
              </t-link>
            </t-tooltip>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无上游渠道：上游转售渠道在「资源管理 → 渠道管理」维护（状态启用后出现在这里）" />
        </template>
      </t-table>
    </section>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" @change="handleTabChange">
        <t-tab-panel value="consume" label="消费流水">
          <div class="detail-toolbar">
            <t-select
              v-model="providerFilter"
              :options="providerFilterOptions"
              placeholder="全部渠道"
              clearable
              style="width: 220px"
              @change="handleFilterChange"
            />
            <span class="table-card__meta">
              按 {{ monthValue }} 筛选 · 共 {{ entryMeta.total }} 笔 ·
              消费合计 ¥{{ formatPrice(entryTotals.consumption) }}
            </span>
          </div>
          <t-table
            row-key="id"
            :data="ledgerEntries"
            :columns="ledgerEntryColumns"
            :loading="recordLoading"
            size="small"
            hover
            cell-empty-content="—"
            :pagination="isMobile ? undefined : entryPagination"
            @page-change="handleEntryPageChange"
          >
            <template #occurred_at="{ row }">
              <span class="time-text">{{ formatTime(row.occurred_at) }}</span>
            </template>
            <template #amount="{ row }">
              <div class="price-cell">
                <span class="amount-expense">¥{{ formatPrice(row.net_amount) }}</span>
                <span v-if="row.refund_amount" class="price-sub">含退款 ¥{{ formatPrice(row.refund_amount) }}</span>
              </div>
            </template>
            <template #category="{ row }">
              <t-tag :theme="row.category === '续费' ? 'warning' : 'primary'" variant="light" size="small" shape="round">
                {{ row.category || '消费' }}
              </t-tag>
            </template>
            <template #ref_no="{ row }">
              <span class="cell-muted">{{ row.ref_no || '—' }}</span>
            </template>
            <template #provider_name="{ row }">
              <span class="cell-muted">{{ row.provider_name || `#${row.provider_id}` }}</span>
            </template>
            <template #empty>
              <t-empty description="所选月份暂无上游消费流水（先点「同步上游账本」）" />
            </template>
          </t-table>
          <MobilePagination
            v-if="isMobile"
            :current="entryPage.current"
            :page-size="entryPage.pageSize"
            :total="entryMeta.total"
            @go="goEntryPage"
            @page-size="handleEntryPageSizeChange"
          />
        </t-tab-panel>

        <t-tab-panel value="topup" label="充值流水">
          <div class="detail-toolbar">
            <t-select
              v-model="providerFilter"
              :options="providerFilterOptions"
              placeholder="全部渠道"
              clearable
              style="width: 220px"
              @change="handleFilterChange"
            />
            <span class="table-card__meta">
              按 {{ monthValue }} 筛选 · 共 {{ entryMeta.total }} 笔 ·
              充值合计 ¥{{ formatPrice(entryTotals.topup) }}
            </span>
          </div>
          <t-table
            row-key="id"
            :data="ledgerEntries"
            :columns="topupEntryColumns"
            :loading="recordLoading"
            size="small"
            hover
            cell-empty-content="—"
            :pagination="isMobile ? undefined : entryPagination"
            @page-change="handleEntryPageChange"
          >
            <template #occurred_at="{ row }">
              <span class="time-text">{{ formatTime(row.occurred_at) }}</span>
            </template>
            <template #amount="{ row }">
              <span class="amount-income">¥{{ formatPrice(row.net_amount) }}</span>
            </template>
            <template #category="{ row }">
              <t-tag theme="success" variant="light" size="small" shape="round">
                {{ row.category || '充值' }}
              </t-tag>
            </template>
            <template #ref_no="{ row }">
              <span class="cell-muted">{{ row.ref_no || '—' }}</span>
            </template>
            <template #provider_name="{ row }">
              <span class="cell-muted">{{ row.provider_name || `#${row.provider_id}` }}</span>
            </template>
            <template #empty>
              <t-empty description="所选月份暂无上游充值流水（先点「同步上游账本」）" />
            </template>
          </t-table>
          <MobilePagination
            v-if="isMobile"
            :current="entryPage.current"
            :page-size="entryPage.pageSize"
            :total="entryMeta.total"
            @go="goEntryPage"
            @page-size="handleEntryPageSizeChange"
          />
        </t-tab-panel>
      </t-tabs>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { CloudDownloadIcon, CurrencyExchangeIcon, RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import { fetchCostBalance, getCostLedger, getCostLedgerEntries, syncCostLedger } from '@/api/cost'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatAmount, formatPrice, formatTime } from '@/pages/finance/constants'
import type {
  BalanceAlert,
  CostLedgerEntryInfo,
  UpstreamLedgerResponse,
  UpstreamLedgerRow,
} from '@/types/interface'

defineOptions({ name: 'FinanceCostUpstreams' })

const { isMobile } = useIsMobile()

function emptyLedger(): UpstreamLedgerResponse {
  return {
    month: '',
    rows: [],
    total_consumption: 0,
    total_topup: 0,
    balance_supported_providers: [],
    ledger_supported_providers: [],
    ledger_synced_at: '',
    alerts: [],
  }
}

const loading = ref(false)
const recordLoading = ref(false)
const ledger = ref<UpstreamLedgerResponse>(emptyLedger())
const monthValue = ref<string>(currentMonth())
// 流水页签：消费 / 充值（同一分页与渠道筛选，数据源都是上游账本）
const activeTab = ref<'consume' | 'topup'>('consume')

const ledgerEntries = ref<CostLedgerEntryInfo[]>([])
const entryMeta = reactive({ total: 0 })
const entryTotals = reactive({ consumption: 0, topup: 0 })
const entryPage = reactive({ current: 1, pageSize: 20 })
const providerFilter = ref<number | undefined>(undefined)

const ledgerColumns: PrimaryTableCol<UpstreamLedgerRow>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 170 },
  { colKey: 'latest', title: '最新余额', width: 150 },
  { colKey: 'consumption', title: '期间消耗（流水）', width: 180 },
  { colKey: 'topup_total', title: '期间充值', width: 120 },
  { colKey: 'cost_source', title: '取数来源', width: 120 },
  { colKey: 'op', title: '操作', width: 180, fixed: 'right' },
]

const ledgerEntryColumns: PrimaryTableCol<CostLedgerEntryInfo>[] = [
  { colKey: 'occurred_at', title: '发生时间', width: 170 },
  { colKey: 'amount', title: '金额（净额）', width: 160 },
  { colKey: 'category', title: '类型', width: 110 },
  { colKey: 'provider_name', title: '渠道', width: 120 },
  { colKey: 'ref_no', title: '上游账单号', minWidth: 130 },
  { colKey: 'description', title: '上游描述', minWidth: 140 },
]

const topupEntryColumns: PrimaryTableCol<CostLedgerEntryInfo>[] = [
  { colKey: 'occurred_at', title: '发生时间', width: 170 },
  { colKey: 'amount', title: '金额', width: 140 },
  { colKey: 'category', title: '类型', width: 120 },
  { colKey: 'provider_name', title: '渠道', width: 120 },
  { colKey: 'ref_no', title: '交易号', minWidth: 130 },
  { colKey: 'description', title: '上游描述', minWidth: 140 },
]

const balanceSupported = computed(() => ledger.value.balance_supported_providers || [])
const ledgerSupported = computed(() => ledger.value.ledger_supported_providers || [])
/** 只显示非 ok 的告警（ok 的余额水位在提示语里可见，不必刷屏）。 */
const visibleAlerts = computed<BalanceAlert[]>(() => (ledger.value.alerts || []).filter((a) => a.level !== 'ok'))
const ledgerSyncedLabel = computed(() => {
  if (!ledgerSupported.value.length) return '该上游未接入账本读取'
  if (!ledger.value.ledger_synced_at) return '尚未同步（点「同步上游账本」）'
  return `最近 ${formatTime(ledger.value.ledger_synced_at)}`
})

const entryPagination = computed(() => ({
  current: entryPage.current,
  pageSize: entryPage.pageSize,
  total: entryMeta.total,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
}))

const providerFilterOptions = computed(() =>
  ledger.value.rows.map((row) => ({ label: row.provider_name || `#${row.provider_id}`, value: row.provider_id })),
)

function alertTheme(level: string): 'success' | 'warning' | 'error' | 'info' {
  switch (level) {
    case 'critical':
      return 'error'
    case 'warning':
      return 'warning'
    case 'unknown':
      return 'info'
    default:
      return 'success'
  }
}

function isBalanceSupported(providerId: number): boolean {
  return balanceSupported.value.includes(providerId)
}

function isLedgerSupported(providerId: number): boolean {
  return ledgerSupported.value.includes(providerId)
}

/** 无流水渠道的说明：区分「能力没接入」与「接了还没同步」。 */
function sourceHint(row: UpstreamLedgerRow): string {
  return isLedgerSupported(row.provider_id) ? '待同步账本' : '该渠道未接入账本'
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function currentMonth(): string {
  const now = new Date()
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`
}

function normalizeMonth(value: unknown): string {
  if (!value) return ''
  if (value instanceof Date) return `${value.getFullYear()}-${pad(value.getMonth() + 1)}`
  const matched = String(value).match(/^(\d{4})-(\d{2})/)
  return matched ? `${matched[1]}-${matched[2]}` : ''
}

function moneyText(value: number | null): string {
  return value === null || value === undefined ? '—' : `¥${formatPrice(value)}`
}

async function load() {
  loading.value = true
  try {
    ledger.value = await getCostLedger({ month: monthValue.value })
    // 渠道列表可能变化（新增/停用上游渠道），下拉选项跟着刷新
    if (providerFilter.value && !ledger.value.rows.some((row) => row.provider_id === providerFilter.value)) {
      providerFilter.value = undefined
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载上游余额台账失败')
    ledger.value = emptyLedger()
  } finally {
    loading.value = false
  }
  await loadEntries()
}

async function loadEntries() {
  recordLoading.value = true
  try {
    const data = await getCostLedgerEntries({
      provider_id: providerFilter.value,
      kind: activeTab.value,
      month: monthValue.value,
      page: entryPage.current,
      page_size: entryPage.pageSize,
    })
    ledgerEntries.value = data.items
    entryMeta.total = data.meta.total
    entryTotals.consumption = data.consumption_total
    entryTotals.topup = data.topup_total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载流水明细失败')
    ledgerEntries.value = []
    entryMeta.total = 0
  } finally {
    recordLoading.value = false
  }
}

function handleMonthChange(value: unknown) {
  monthValue.value = normalizeMonth(value)
  entryPage.current = 1
  void load()
}

function handleTabChange() {
  entryPage.current = 1
  void loadEntries()
}

function handleFilterChange() {
  entryPage.current = 1
  void loadEntries()
}

function handleEntryPageChange(pageInfo: PageInfo) {
  entryPage.current = pageInfo.current
  entryPage.pageSize = pageInfo.pageSize
  void loadEntries()
}

function goEntryPage(current: number) {
  entryPage.current = current
  void loadEntries()
}

function handleEntryPageSizeChange(pageSize: number) {
  entryPage.pageSize = pageSize
  entryPage.current = 1
  void loadEntries()
}

// ===== 上游账本同步 =====

const syncingId = ref(0)
const syncingLedger = ref(false)

/**
 * 同步上游账本。providerId 省略=全部支持的渠道；full=true 全量回填（首次接入/数据修复）。
 * 首次使用建议先回填一次历史（把过去月份的流水一次拉回，成本趋势立刻有数）。
 */
async function handleSyncLedger(full: boolean, providerId?: number) {
  if (providerId) {
    if (!isLedgerSupported(providerId)) {
      MessagePlugin.warning('该渠道类型未实现上游账本读取')
      return
    }
    syncingId.value = providerId
  } else {
    syncingLedger.value = true
  }
  try {
    const data = await syncCostLedger({ provider_id: providerId ?? 0, full })
    const results = data.results || []
    const failed = results.filter((item) => item.failed)
    if (!results.length) {
      MessagePlugin.warning('没有支持账本同步的渠道')
    } else if (failed.length === 0) {
      const written = results.reduce((sum, item) => sum + item.consumption + item.topup, 0)
      MessagePlugin.success(
        full
          ? `已全量回填 ${results.length} 个渠道，写入 ${written} 条流水`
          : `账本已同步，写入 ${written} 条流水`,
      )
    } else {
      MessagePlugin.warning(`${failed.map((item) => `${item.provider_name}：${item.failed}`).join('；')}`)
    }
    await load()
  } catch (error) {
    // 30008（渠道不支持账本读取）时后端给出可执行提示，原样展示。
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncingId.value = 0
    syncingLedger.value = false
  }
}

// ===== 抓取余额 =====

const fetchingId = ref(0)
const fetchingAll = ref(false)

async function handleFetch(row: UpstreamLedgerRow) {
  if (!isBalanceSupported(row.provider_id)) {
    MessagePlugin.warning('该渠道类型未实现余额查询接口')
    return
  }
  fetchingId.value = row.provider_id
  try {
    const snapshot = await fetchCostBalance(row.provider_id)
    MessagePlugin.success(`已抓取 ${row.provider_name || `#${row.provider_id}`} 余额 ¥${formatPrice(snapshot.balance)}`)
    await load()
  } catch (error) {
    // 30008（渠道不支持自动查询）时后端已给出可执行提示，原样展示即可。
    MessagePlugin.error(`${row.provider_name || `#${row.provider_id}`}：${(error as Error).message || '抓取失败'}`)
  } finally {
    fetchingId.value = 0
  }
}

/** 抓取全部（只含具备余额读取能力的渠道）：逐渠道串行，失败不打断其它渠道。 */
async function handleFetchAll() {
  const targets = ledger.value.rows.filter((row) => isBalanceSupported(row.provider_id))
  if (!targets.length) {
    MessagePlugin.warning('没有支持自动抓取余额的渠道')
    return
  }
  fetchingAll.value = true
  let saved = 0
  const failures: string[] = []
  try {
    for (const row of targets) {
      try {
        await fetchCostBalance(row.provider_id)
        saved += 1
      } catch (error) {
        failures.push(`${row.provider_name || `#${row.provider_id}`}（${(error as Error).message || '抓取失败'}）`)
      }
    }
    if (failures.length === 0) {
      MessagePlugin.success(`已抓取 ${saved} 个渠道余额并落今日快照`)
    } else if (saved === 0) {
      MessagePlugin.error(`抓取失败：${failures.join('；')}`)
    } else {
      MessagePlugin.warning(`成功 ${saved} 个，失败 ${failures.length} 个：${failures.join('；')}`)
    }
    await load()
  } finally {
    fetchingAll.value = false
  }
}

onMounted(load)
</script>

<style lang="css">
@import '../../shared.css';

.finance-module.cost-module .cost-hint {
  border-radius: var(--hs-radius-lg);
}

.finance-module.cost-module .provider-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.finance-module.cost-module .summary-item--hint {
  min-width: 140px;
}

.finance-module.cost-module .detail-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-md) 0;
  flex-wrap: wrap;
}
</style>
