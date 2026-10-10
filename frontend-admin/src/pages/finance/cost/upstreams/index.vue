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
            {{ ledger.month || monthValue }} 各上游渠道的期初 / 充值 / 消耗 / 期末
            —— 消耗 = 期初 + 期间充值 − 期末，由余额快照推算，不依赖上游是否开放资金接口
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
          v-if="supportedProviders.length"
          variant="outline"
          :loading="fetchingAll"
          @click="handleFetchAll"
        >
          <template #icon><CloudDownloadIcon aria-hidden="true" /></template>
          抓取全部（{{ supportedProviders.length }}）
        </t-button>
        <t-button variant="outline" @click="openSnapshotDialog()">
          <template #icon><AddIcon aria-hidden="true" /></template>
          录入余额
        </t-button>
        <t-button variant="outline" @click="openTopupDialog()">
          <template #icon><MoneyIcon aria-hidden="true" /></template>
          记录充值
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
        已接入账本的渠道（标「支持抓取」者同时支持流水）点「同步上游账本」即可拉回<strong>消费与充值流水</strong>，
        消耗与成本直接按流水归集、逐笔可查，系统每天也会自动增量同步一次；
        只有余额查询能力的渠道点「抓取余额」落当日快照，并按「财务配置 → 上游余额自动快照小时」每天自动抓一次，
        其余渠道请手工录入（同渠道同日重复录入会覆盖）。
        <strong>首次使用</strong>：账本渠道先点一次「同步上游账本」回填历史（成本趋势立刻有数）；
        非账本渠道需用「录入余额」补一次<strong>上月末余额</strong>（上游只能查当前余额，补不了历史）。
      </template>
    </t-alert>

    <section class="summary-bar surface-card">
      <div class="summary-item">
        <span class="summary-item__label">期间消耗合计</span>
        <span class="summary-item__value amount-expense">¥{{ formatPrice(ledger.total_consumption) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">跟踪渠道</span>
        <span class="summary-item__value">{{ ledger.rows.length }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">支持自动抓取</span>
        <span class="summary-item__value">{{ supportedProviders.length }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">缺快照渠道</span>
        <span class="summary-item__value" :class="missingCount ? 'amount-expense' : ''">{{ missingCount }} 个</span>
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
        <h3 class="card-title">渠道余额与消耗</h3>
        <span class="table-card__meta">
          「缺本月快照」表示期初/期末取自历史数据，消耗暂不可算
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
              <template v-if="isSupported(row.provider_id)"> · 支持抓取</template>
            </span>
          </div>
        </template>
        <template #opening="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ moneyText(row.opening_balance) }}</span>
            <span class="price-sub">{{ row.opening_date || '无历史快照' }}</span>
          </div>
        </template>
        <template #topup_total="{ row }">
          <span :class="row.topup_total >= 0 ? 'amount-income' : 'amount-expense'">
            {{ formatAmount(row.topup_total) }}
          </span>
        </template>
        <template #consumption="{ row }">
          <div class="price-cell">
            <span v-if="row.cost_source === 'ledger'" class="price-main">
              ¥{{ formatPrice(row.ledger_consumption ?? 0) }}
            </span>
            <span v-else-if="row.consumption !== null" class="price-main">¥{{ formatPrice(row.consumption) }}</span>
            <span v-else class="cell-muted">待同步</span>
            <span class="price-sub">
              <template v-if="row.cost_source === 'ledger'">
                {{ row.ledger_entries }} 笔流水<template v-if="row.ledger_diff"> · 与快照差 {{ formatAmount(row.ledger_diff) }}</template>
              </template>
              <template v-else-if="row.cost_source === 'snapshot'">快照推算</template>
              <template v-else>无数据</template>
            </span>
          </div>
        </template>
        <template #closing="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ moneyText(row.closing_balance) }}</span>
            <span class="price-sub">{{ row.closing_date || '本月无快照' }}</span>
          </div>
        </template>
        <template #latest="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ moneyText(row.latest_balance) }}</span>
            <span class="price-sub">{{ row.latest_date || '—' }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <t-tag v-if="row.missing_snapshot" theme="danger" variant="light" size="small" shape="round">缺本月快照</t-tag>
          <t-tag v-else-if="row.estimated" theme="warning" variant="light" size="small" shape="round">截至日估算</t-tag>
          <t-tag v-else theme="success" variant="light" size="small" shape="round">已覆盖整月</t-tag>
        </template>
        <template #op="{ row }">
          <div class="action-cell">
            <t-tooltip
              :content="isSupported(row.provider_id) ? '调用渠道接口抓取当前余额并落今日快照' : '该渠道类型未实现余额查询接口，请手工录入'"
            >
              <t-link
                theme="primary"
                :disabled="!isSupported(row.provider_id) || fetchingId === row.provider_id"
                @click="handleFetch(row)"
              >
                {{ fetchingId === row.provider_id ? '抓取中…' : '抓取余额' }}
              </t-link>
            </t-tooltip>
            <t-link
              v-if="isLedgerSupported(row.provider_id)"
              theme="primary"
              :disabled="syncingId === row.provider_id"
              @click="handleSyncLedger(false, row.provider_id)"
            >
              {{ syncingId === row.provider_id ? '同步中…' : '同步账本' }}
            </t-link>
            <t-link theme="primary" @click="openSnapshotDialog(row.provider_id)">录入余额</t-link>
            <t-link theme="primary" @click="openTopupDialog(row.provider_id)">记录充值</t-link>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无上游渠道：上游转售渠道在「资源管理 → 渠道管理」维护（状态启用后出现在这里）" />
        </template>
      </t-table>
    </section>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" @change="handleTabChange">
        <t-tab-panel value="ledger" label="上游流水（消费）">
          <div class="detail-toolbar">
            <t-select
              v-model="recordFilter.providerId"
              :options="providerFilterOptions"
              placeholder="全部渠道"
              clearable
              style="width: 220px"
              @change="handleRecordFilterChange"
            />
            <span class="table-card__meta">
              按 {{ monthValue }} 筛选 · 共 {{ ledgerMeta.total }} 笔 ·
              消费合计 ¥{{ formatPrice(ledgerTotals.consumption) }}
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
            :pagination="isMobile ? undefined : ledgerPagination"
            @page-change="handleLedgerPageChange"
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
            :current="ledgerPage.current"
            :page-size="ledgerPage.pageSize"
            :total="ledgerMeta.total"
            @go="goLedgerPage"
            @page-size="handleLedgerPageSizeChange"
          />
        </t-tab-panel>

        <t-tab-panel value="snapshots" label="余额快照">
          <div class="detail-toolbar">
            <t-select
              v-model="recordFilter.providerId"
              :options="providerFilterOptions"
              placeholder="全部渠道"
              clearable
              style="width: 220px"
              @change="handleRecordFilterChange"
            />
            <span class="table-card__meta">按 {{ monthValue }} 筛选 · 共 {{ snapshotMeta.total }} 条记录</span>
          </div>
          <t-table
            row-key="id"
            :data="snapshots"
            :columns="snapshotColumns"
            :loading="recordLoading"
            size="small"
            hover
            cell-empty-content="—"
            :pagination="isMobile ? undefined : snapshotPagination"
            @page-change="handleSnapshotPageChange"
          >
            <template #provider_name="{ row }">
              <span class="cell-strong">{{ row.provider_name || `#${row.provider_id}` }}</span>
            </template>
            <template #balance="{ row }">
              <span class="cell-strong">¥{{ formatPrice(row.balance) }}</span>
            </template>
            <template #source="{ row }">
              <t-tag :theme="row.source === 'auto' ? 'primary' : 'default'" variant="light" size="small" shape="round">
                {{ row.source === 'auto' ? '接口抓取' : '手工录入' }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #op="{ row }">
              <t-link theme="danger" @click="handleDeleteSnapshot(row)">删除</t-link>
            </template>
            <template #empty><t-empty description="所选月份暂无余额快照" /></template>
          </t-table>
          <MobilePagination
            v-if="isMobile"
            :current="snapshotPage.current"
            :page-size="snapshotPage.pageSize"
            :total="snapshotMeta.total"
            @go="goSnapshotPage"
            @page-size="handleSnapshotPageSizeChange"
          />
        </t-tab-panel>

        <t-tab-panel value="topups" label="充值记录">
          <div class="detail-toolbar">
            <t-select
              v-model="recordFilter.providerId"
              :options="providerFilterOptions"
              placeholder="全部渠道"
              clearable
              style="width: 220px"
              @change="handleRecordFilterChange"
            />
            <span class="table-card__meta">
              按 {{ monthValue }} 筛选 · 共 {{ topupMeta.total }} 条记录 ·
              充值合计 ¥{{ formatPrice(ledgerTotals.topup) }}
            </span>
          </div>
          <t-table
            row-key="id"
            :data="topups"
            :columns="topupColumns"
            :loading="recordLoading"
            size="small"
            hover
            cell-empty-content="—"
            :pagination="isMobile ? undefined : topupPagination"
            @page-change="handleTopupPageChange"
          >
            <template #provider_name="{ row }">
              <span class="cell-strong">{{ row.provider_name || `#${row.provider_id}` }}</span>
            </template>
            <template #amount="{ row }">
              <span :class="row.amount >= 0 ? 'amount-income' : 'amount-expense'">{{ formatAmount(row.amount) }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #op="{ row }">
              <t-link theme="danger" @click="handleDeleteTopup(row)">删除</t-link>
            </template>
            <template #empty><t-empty description="所选月份暂无充值记录" /></template>
          </t-table>
          <MobilePagination
            v-if="isMobile"
            :current="topupPage.current"
            :page-size="topupPage.pageSize"
            :total="topupMeta.total"
            @go="goTopupPage"
            @page-size="handleTopupPageSizeChange"
          />
        </t-tab-panel>
      </t-tabs>
    </section>

    <t-dialog
      v-model:visible="snapshotVisible"
      header="录入余额快照"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveSnapshot"
      @close="snapshotVisible = false"
    >
      <t-form label-align="top" :data="snapshotForm" @submit.prevent>
        <t-form-item label="渠道">
          <t-select v-model="snapshotForm.providerId" :options="providerOptions" placeholder="选择上游渠道" filterable />
        </t-form-item>
        <t-form-item label="快照日期" help="同渠道同日重复录入会覆盖上一条">
          <t-date-picker v-model="snapshotForm.date" format="YYYY-MM-DD" placeholder="默认今天" style="width: 100%" />
        </t-form-item>
        <t-form-item label="账户余额（元）">
          <t-input-number v-model="snapshotForm.balance" :min="0" :precision="2" theme="column" style="width: 100%" />
        </t-form-item>
        <t-form-item label="备注">
          <t-input v-model="snapshotForm.remark" placeholder="可选，如「月结后余额」" clearable />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="topupVisible"
      header="记录上游充值"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveTopup"
      @close="topupVisible = false"
    >
      <t-form label-align="top" :data="topupForm" @submit.prevent>
        <t-form-item label="渠道">
          <t-select v-model="topupForm.providerId" :options="providerOptions" placeholder="选择上游渠道" filterable />
        </t-form-item>
        <t-form-item label="发生日期">
          <t-date-picker v-model="topupForm.date" format="YYYY-MM-DD" placeholder="默认今天" style="width: 100%" />
        </t-form-item>
        <t-form-item label="金额（元）" help="正数=向上游充值；负数=渠道退款/冲正">
          <t-input-number v-model="topupForm.amount" :precision="2" theme="column" style="width: 100%" />
        </t-form-item>
        <t-form-item label="备注">
          <t-input v-model="topupForm.remark" placeholder="可选，如「支付宝充值」「月结退款」" clearable />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, CloudDownloadIcon, CurrencyExchangeIcon, MoneyIcon, RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import {
  deleteCostSnapshot,
  deleteCostTopup,
  fetchCostBalance,
  getCostLedger,
  getCostLedgerEntries,
  getCostSnapshots,
  getCostTopups,
  saveCostSnapshot,
  saveCostTopup,
  syncCostLedger,
} from '@/api/cost'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatAmount, formatPrice, formatTime, toDateString } from '@/pages/finance/constants'
import type {
  BalanceAlert,
  CostLedgerEntryInfo,
  CostSnapshotInfo,
  CostTopupInfo,
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
    snapshot_supported_providers: [],
    ledger_supported_providers: [],
    ledger_synced_at: '',
    alerts: [],
  }
}

const loading = ref(false)
const recordLoading = ref(false)
const saving = ref(false)
const ledger = ref<UpstreamLedgerResponse>(emptyLedger())
const monthValue = ref<string>(currentMonth())
const activeTab = ref('snapshots')

const snapshots = ref<CostSnapshotInfo[]>([])
const topups = ref<CostTopupInfo[]>([])
const ledgerEntries = ref<CostLedgerEntryInfo[]>([])
const snapshotMeta = reactive({ total: 0 })
const topupMeta = reactive({ total: 0 })
const ledgerMeta = reactive({ total: 0 })
const ledgerTotals = reactive({ consumption: 0, topup: 0 })

const recordFilter = reactive<{ providerId?: number }>({ providerId: undefined })

const snapshotPage = reactive({ current: 1, pageSize: 20 })
const topupPage = reactive({ current: 1, pageSize: 20 })
const ledgerPage = reactive({ current: 1, pageSize: 20 })

const snapshotPagination = computed(() => ({
  current: snapshotPage.current,
  pageSize: snapshotPage.pageSize,
  total: snapshotMeta.total,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
}))

const topupPagination = computed(() => ({
  current: topupPage.current,
  pageSize: topupPage.pageSize,
  total: topupMeta.total,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
}))

const ledgerEntryColumns: PrimaryTableCol<CostLedgerEntryInfo>[] = [
  { colKey: 'occurred_at', title: '发生时间', width: 170 },
  { colKey: 'amount', title: '金额（净额）', width: 160 },
  { colKey: 'category', title: '类型', width: 110 },
  { colKey: 'provider_name', title: '渠道', width: 120 },
  { colKey: 'ref_no', title: '上游账单号', minWidth: 130 },
  { colKey: 'description', title: '上游描述', minWidth: 140 },
]

const ledgerColumns: PrimaryTableCol<UpstreamLedgerRow>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 170 },
  { colKey: 'opening', title: '期初余额', width: 150 },
  { colKey: 'topup_total', title: '期间充值', width: 120 },
  { colKey: 'consumption', title: '期间消耗 / 取数来源', width: 190 },
  { colKey: 'closing', title: '期末余额', width: 150 },
  { colKey: 'latest', title: '最新余额', width: 150 },
  { colKey: 'status', title: '数据状态', width: 130 },
  { colKey: 'op', title: '操作', width: 240, fixed: 'right' },
]

const snapshotColumns: PrimaryTableCol<CostSnapshotInfo>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 150 },
  { colKey: 'snapshot_date', title: '快照日期', width: 130 },
  { colKey: 'balance', title: '余额', width: 130 },
  { colKey: 'source', title: '来源', width: 110 },
  { colKey: 'remark', title: '备注', minWidth: 150 },
  { colKey: 'created_at', title: '录入时间', width: 170 },
  { colKey: 'op', title: '操作', width: 90, fixed: 'right' },
]

const topupColumns: PrimaryTableCol<CostTopupInfo>[] = [
  { colKey: 'provider_name', title: '渠道', minWidth: 150 },
  { colKey: 'occurred_on', title: '发生日期', width: 130 },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'remark', title: '备注', minWidth: 180 },
  { colKey: 'created_at', title: '录入时间', width: 170 },
  { colKey: 'op', title: '操作', width: 90, fixed: 'right' },
]

const supportedProviders = computed(() => ledger.value.snapshot_supported_providers || [])
const ledgerSupported = computed(() => ledger.value.ledger_supported_providers || [])
/** 只显示非 ok 的告警（ok 的余额水位在提示语里可见，不必刷屏）。 */
const visibleAlerts = computed<BalanceAlert[]>(() => (ledger.value.alerts || []).filter((a) => a.level !== 'ok'))
const ledgerSyncedLabel = computed(() => {
  const hasLedger = ledgerSupported.value.length > 0
  if (!hasLedger) return '该上游未接入账本读取'
  if (!ledger.value.ledger_synced_at) return '尚未同步（点「同步上游账本」）'
  return `最近 ${formatTime(ledger.value.ledger_synced_at)}`
})

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

function isLedgerSupported(providerId: number): boolean {
  return ledgerSupported.value.includes(providerId)
}

const ledgerPagination = computed(() => ({
  current: ledgerPage.current,
  pageSize: ledgerPage.pageSize,
  total: ledgerMeta.total,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
}))
const missingCount = computed(() => ledger.value.rows.filter((row) => row.missing_snapshot).length)
const providerOptions = computed(() =>
  ledger.value.rows.map((row) => ({
    label: `${row.provider_name || `#${row.provider_id}`}${isSupported(row.provider_id) ? '（可抓取）' : ''}`,
    value: row.provider_id,
  })),
)
const providerFilterOptions = computed(() =>
  ledger.value.rows.map((row) => ({ label: row.provider_name || `#${row.provider_id}`, value: row.provider_id })),
)

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

function isSupported(providerId: number): boolean {
  return supportedProviders.value.includes(providerId)
}

async function load() {
  loading.value = true
  try {
    ledger.value = await getCostLedger({ month: monthValue.value })
    // 渠道列表可能变化（新增/停用上游渠道），下拉选项跟着刷新
    if (recordFilter.providerId && !ledger.value.rows.some((row) => row.provider_id === recordFilter.providerId)) {
      recordFilter.providerId = undefined
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载上游余额台账失败')
    ledger.value = emptyLedger()
  } finally {
    loading.value = false
  }
  await loadRecords()
}

async function loadRecords() {
  recordLoading.value = true
  try {
    if (activeTab.value === 'ledger') {
      const data = await getCostLedgerEntries({
        provider_id: recordFilter.providerId,
        kind: 'consume',
        month: monthValue.value,
        page: ledgerPage.current,
        page_size: ledgerPage.pageSize,
      })
      ledgerEntries.value = data.items
      ledgerMeta.total = data.meta.total
      ledgerTotals.consumption = data.consumption_total
      ledgerTotals.topup = data.topup_total
      return
    }
    if (activeTab.value === 'topups') {
      const data = await getCostTopups({
        provider_id: recordFilter.providerId,
        month: monthValue.value,
        page: topupPage.current,
        page_size: topupPage.pageSize,
      })
      topups.value = data.items
      topupMeta.total = data.meta.total
    } else {
      const data = await getCostSnapshots({
        provider_id: recordFilter.providerId,
        month: monthValue.value,
        page: snapshotPage.current,
        page_size: snapshotPage.pageSize,
      })
      snapshots.value = data.items
      snapshotMeta.total = data.meta.total
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载明细失败')
    snapshots.value = []
    topups.value = []
    ledgerEntries.value = []
    snapshotMeta.total = 0
    topupMeta.total = 0
    ledgerMeta.total = 0
  } finally {
    recordLoading.value = false
  }
}

function handleMonthChange(value: unknown) {
  monthValue.value = normalizeMonth(value)
  snapshotPage.current = 1
  topupPage.current = 1
  ledgerPage.current = 1
  void load()
}

function handleTabChange() {
  void loadRecords()
}

function handleRecordFilterChange() {
  snapshotPage.current = 1
  topupPage.current = 1
  ledgerPage.current = 1
  void loadRecords()
}

function handleLedgerPageChange(pageInfo: PageInfo) {
  ledgerPage.current = pageInfo.current
  ledgerPage.pageSize = pageInfo.pageSize
  void loadRecords()
}

function goLedgerPage(current: number) {
  ledgerPage.current = current
  void loadRecords()
}

function handleLedgerPageSizeChange(pageSize: number) {
  ledgerPage.pageSize = pageSize
  ledgerPage.current = 1
  void loadRecords()
}

function handleSnapshotPageChange(pageInfo: PageInfo) {
  snapshotPage.current = pageInfo.current
  snapshotPage.pageSize = pageInfo.pageSize
  void loadRecords()
}

function handleTopupPageChange(pageInfo: PageInfo) {
  topupPage.current = pageInfo.current
  topupPage.pageSize = pageInfo.pageSize
  void loadRecords()
}

function goSnapshotPage(current: number) {
  snapshotPage.current = current
  void loadRecords()
}

function handleSnapshotPageSizeChange(pageSize: number) {
  snapshotPage.pageSize = pageSize
  snapshotPage.current = 1
  void loadRecords()
}

function goTopupPage(current: number) {
  topupPage.current = current
  void loadRecords()
}

function handleTopupPageSizeChange(pageSize: number) {
  topupPage.pageSize = pageSize
  topupPage.current = 1
  void loadRecords()
}

// ===== 余额快照 =====

const snapshotVisible = ref(false)
const snapshotForm = reactive<{ providerId?: number; date: string; balance: number; remark: string }>({
  providerId: undefined,
  date: '',
  balance: 0,
  remark: '',
})

function openSnapshotDialog(providerId?: number) {
  snapshotForm.providerId = providerId ?? ledger.value.rows[0]?.provider_id
  snapshotForm.date = ''
  snapshotForm.balance = 0
  snapshotForm.remark = ''
  snapshotVisible.value = true
}

async function handleSaveSnapshot() {
  if (!snapshotForm.providerId) {
    MessagePlugin.warning('请选择上游渠道')
    return
  }
  saving.value = true
  try {
    await saveCostSnapshot({
      provider_id: snapshotForm.providerId,
      snapshot_date: toDateString(snapshotForm.date) || undefined,
      balance: Number(snapshotForm.balance || 0),
      remark: snapshotForm.remark.trim(),
    })
    MessagePlugin.success('余额快照已保存')
    snapshotVisible.value = false
    snapshotPage.current = 1
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleDeleteSnapshot(row: CostSnapshotInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除余额快照',
    body: `确认删除 ${row.provider_name || `#${row.provider_id}`} 在 ${row.snapshot_date} 的余额快照？删除后该月消耗可能无法推算。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteCostSnapshot(row.id)
        MessagePlugin.success('快照已删除')
        dialog.hide()
        await load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

// ===== 上游充值 =====

const topupVisible = ref(false)
const topupForm = reactive<{ providerId?: number; date: string; amount: number; remark: string }>({
  providerId: undefined,
  date: '',
  amount: 0,
  remark: '',
})

function openTopupDialog(providerId?: number) {
  topupForm.providerId = providerId ?? ledger.value.rows[0]?.provider_id
  topupForm.date = ''
  topupForm.amount = 0
  topupForm.remark = ''
  topupVisible.value = true
}

async function handleSaveTopup() {
  if (!topupForm.providerId) {
    MessagePlugin.warning('请选择上游渠道')
    return
  }
  if (!Number(topupForm.amount)) {
    MessagePlugin.warning('充值金额不能为 0')
    return
  }
  saving.value = true
  try {
    await saveCostTopup({
      provider_id: topupForm.providerId,
      occurred_on: toDateString(topupForm.date) || undefined,
      amount: Number(topupForm.amount),
      remark: topupForm.remark.trim(),
    })
    MessagePlugin.success('充值记录已保存，期间消耗随之重算')
    topupVisible.value = false
    topupPage.current = 1
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleDeleteTopup(row: CostTopupInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除充值记录',
    body: `确认删除 ${row.provider_name || `#${row.provider_id}`} 在 ${row.occurred_on} 的充值记录？删除后期间消耗会相应变化。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteCostTopup(row.id)
        MessagePlugin.success('充值记录已删除')
        dialog.hide()
        await load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
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
      MessagePlugin.warning('该渠道类型未实现上游账本读取，请手工录入')
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
  if (!isSupported(row.provider_id)) {
    MessagePlugin.warning('该渠道类型未实现余额查询接口，请手工录入余额快照')
    return
  }
  fetchingId.value = row.provider_id
  try {
    const snapshot = await fetchCostBalance(row.provider_id)
    MessagePlugin.success(`已抓取 ${row.provider_name || `#${row.provider_id}`} 余额 ¥${formatPrice(snapshot.balance)}`)
    snapshotPage.current = 1
    await load()
  } catch (error) {
    // 30008（渠道不支持自动查询）时后端已给出可执行提示（手工录入快照），原样展示即可。
    MessagePlugin.error(`${row.provider_name || `#${row.provider_id}`}：${(error as Error).message || '抓取失败'}`)
  } finally {
    fetchingId.value = 0
  }
}

/** 抓取全部（只含具备余额读取能力的渠道）：逐渠道串行，失败不打断其它渠道。 */
async function handleFetchAll() {
  const targets = ledger.value.rows.filter((row) => isSupported(row.provider_id))
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
    snapshotPage.current = 1
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
