<template>
  <div class="billing-layout">
    <!-- ============ 左侧导航 ============ -->
    <aside class="billing-sider">
      <button class="sider-item" :class="{ 'is-active': view === 'overview' }" @click="view = 'overview'">
        <DashboardIcon size="16" />
        <span>财务总览</span>
      </button>

      <div class="sider-group">
        <button class="sider-item sider-item--group" @click="billingOpen = !billingOpen">
          <MoneyIcon size="16" />
          <span>消费中心</span>
          <ChevronDownIcon v-if="billingOpen" size="14" class="sider-arrow" />
          <ChevronRightIcon v-else size="14" class="sider-arrow" />
        </button>
        <div v-show="billingOpen" class="sider-sub">
          <button class="sider-item sider-item--sub" :class="{ 'is-active': view === 'overview' }" @click="view = 'overview'">
            <span>消费总览</span>
          </button>
          <button class="sider-item sider-item--sub" :class="{ 'is-active': view === 'bills' }" @click="view = 'bills'">
            <span>资源月账单</span>
          </button>
        </div>
      </div>

      <button class="sider-item" @click="onDevelop('成本管理')">
        <ChartBarIcon size="16" />
        <span>成本管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('订购管理')">
        <CartIcon size="16" />
        <span>订购管理</span>
      </button>
      <button class="sider-item" @click="router.push('/billing/transactions')">
        <SwapIcon size="16" />
        <span>收支明细</span>
      </button>
      <button class="sider-item" @click="onDevelop('导出记录')">
        <DownloadIcon size="16" />
        <span>导出记录</span>
      </button>
      <button class="sider-item" @click="router.push('/billing/balance')">
        <WalletIcon size="16" />
        <span>资产管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('发票管理')">
        <FilePasteIcon size="16" />
        <span>发票管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('合同管理')">
        <FileIcon size="16" />
        <span>合同管理</span>
      </button>
    </aside>

    <!-- ============ 右侧内容 ============ -->
    <div class="billing-main">
      <div class="page-head">
        <h2 class="page-title">消费总览</h2>
        <t-button variant="text" theme="primary" @click="onDevelop('操作指南')">
          <template #icon><FilePasteIcon /></template>
          操作指南
        </t-button>
      </div>

      <!-- 余额条 -->
      <div class="balance-strip">
        <span class="balance-strip__item">
          可用余额
          <strong class="balance-strip__value">¥ {{ formatPrice(wallet.balance) }}</strong>
        </span>
        <span class="balance-strip__item">
          冻结
          <strong class="balance-strip__value">¥ {{ formatPrice(wallet.frozen) }}</strong>
        </span>
        <span class="balance-strip__item">
          累计收入
          <strong class="balance-strip__value">¥ {{ formatPrice(wallet.total_income) }}</strong>
        </span>
        <span class="balance-strip__spacer"></span>
        <t-button
          v-if="memberStore.has('billing:recharge')"
          theme="primary"
          size="small"
          @click="router.push('/billing/balance')"
        >
          <template #icon><AddIcon /></template>
          立即充值
        </t-button>
      </div>

      <t-alert theme="info" class="billing-alert">
        当月最终账单在次月1日9点后支持查看/导出，在此之前的数据查询结果仅供参考，不作为对账依据。
      </t-alert>

      <t-tabs v-model="view" class="top-tabs">
        <t-tab-panel value="overview" label="消费总览" />
        <t-tab-panel value="bills" label="资源月账单" />
      </t-tabs>

      <!-- ---------- 消费总览 ---------- -->
      <template v-if="view === 'overview'">
        <section class="panel">
          <div class="panel-head">
            <h3 class="panel-title">支付详情</h3>
          </div>

          <div class="filter-row">
            <t-radio-group v-model="filter.mode" variant="default-filled" size="small">
              <t-radio-button value="month">按月</t-radio-button>
              <t-radio-button value="custom">自定义</t-radio-button>
            </t-radio-group>

            <t-date-picker
              v-model="filter.month"
              mode="month"
              format="YYYY-MM"
              value-type="YYYY-MM"
              clearable
              placeholder="选择月份"
              size="small"
              style="width: 160px"
            />

            <t-select v-model="filter.payMethod" size="small" style="width: 150px" :options="payMethodOptions" />

            <t-checkbox v-model="filter.showAdjust">查看调减金额</t-checkbox>

            <span class="filter-spacer"></span>

            <t-button variant="outline" size="small" @click="onDevelop('消费总览 PDF')">
              <template #icon><DownloadIcon /></template>
              消费总览 PDF
            </t-button>
            <t-button variant="outline" size="small" @click="onDevelop('费用汇总 CSV')">
              <template #icon><DownloadIcon /></template>
              费用汇总 CSV
            </t-button>
            <button class="icon-btn" aria-label="刷新" @click="reload">
              <RefreshIcon size="16" />
            </button>
          </div>

          <div class="amount-grid">
            <!-- 应付金额（主卡） -->
            <div class="amount-card amount-card--primary">
              <div class="amount-main">
                <span class="amount-label">
                  应付金额
                  <HelpCircleIcon size="13" class="amount-help" />
                </span>
                <span class="amount-value amount-value--primary">¥{{ formatPrice(summary.payable) }}</span>
              </div>
              <div class="amount-sub">
                <span class="amount-label">已付部分</span>
                <span class="amount-value">¥{{ formatPrice(summary.paid) }}</span>
              </div>
            </div>

            <!-- 右侧明细 -->
            <div class="amount-side">
              <div class="amount-card">
                <span class="amount-label">目录价金额</span>
                <span class="amount-value">¥{{ formatPrice(summary.listPrice) }}</span>
              </div>

              <div class="amount-card-row">
                <div class="amount-card amount-card--half">
                  <span class="amount-label">
                    优惠金额
                    <HelpCircleIcon size="13" class="amount-help" />
                  </span>
                  <span class="amount-value">¥{{ formatPrice(summary.discount) }}</span>
                </div>
                <div class="amount-card amount-card--half">
                  <span class="amount-label">优惠券金额</span>
                  <span class="amount-value">¥{{ formatPrice(summary.coupon) }}</span>
                </div>
              </div>

              <div class="amount-card">
                <span class="amount-label">调减金额</span>
                <span class="amount-value">¥{{ formatPrice(summary.adjust) }}</span>
              </div>
            </div>
          </div>

          <div class="summary-head">
            <t-tabs v-model="summaryTab" size="medium">
              <t-tab-panel value="product" label="产品汇总" />
              <t-tab-panel value="tag" label="标签汇总" />
            </t-tabs>
            <div class="summary-actions">
              <button class="icon-btn" aria-label="下载" @click="onDevelop('汇总导出')">
                <DownloadIcon size="16" />
              </button>
              <button class="icon-btn" aria-label="刷新" @click="reload">
                <RefreshIcon size="16" />
              </button>
            </div>
          </div>

          <p class="formula-text">应付金额 = 目录价金额 - 优惠金额 - 调减金额</p>

          <t-table
            v-if="summaryTab === 'product'"
            row-key="name"
            :data="productRows"
            :columns="productColumns"
            :loading="loading"
            size="small"
            :bordered="false"
            cell-empty-content="—"
          >
            <template #listPrice="{ row }">
              <span class="num-cell">¥{{ formatPrice(row.listPrice) }}</span>
            </template>
            <template #payable="{ row }">
              <span class="num-cell num-cell--strong">¥{{ formatPrice(row.payable) }}</span>
            </template>
            <template #discount="{ row }">
              <span class="num-cell">¥{{ formatPrice(row.discount) }}</span>
            </template>
            <template #adjust="{ row }">
              <span class="num-cell">¥{{ formatPrice(row.adjust) }}</span>
            </template>
            <template #operation>
              <t-link theme="primary" hover="color" @click="router.push('/billing/transactions')">查看明细</t-link>
            </template>
            <template #empty>
              <t-empty description="当前账期暂无消费记录" />
            </template>
          </t-table>

          <t-table
            v-else
            row-key="name"
            :data="tagRows"
            :columns="tagColumns"
            size="small"
            :bordered="false"
            cell-empty-content="—"
          >
            <template #listPrice="{ row }">
              <span class="num-cell">¥{{ formatPrice(row.listPrice) }}</span>
            </template>
            <template #payable="{ row }">
              <span class="num-cell num-cell--strong">¥{{ formatPrice(row.payable) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无标签汇总数据" />
            </template>
          </t-table>
        </section>
      </template>

      <!-- ---------- 资源月账单 ---------- -->
      <template v-else>
        <section class="panel">
          <div class="panel-head">
            <h3 class="panel-title">资源月账单</h3>
            <t-button variant="outline" size="small" :loading="loading" @click="reload">
              <template #icon><RefreshIcon /></template>
              刷新
            </t-button>
          </div>

          <t-table
            row-key="id"
            :data="filteredBills"
            :columns="billColumns"
            :loading="loading"
            size="small"
            :bordered="false"
            cell-empty-content="—"
          >
            <template #status="{ row }">
              <t-tag :theme="billStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ billStatusLabel(row.status) }}
              </t-tag>
            </template>
            <template #total_amount="{ row }">
              <span class="num-cell num-cell--strong">¥ {{ formatPrice(row.total_amount) }}</span>
            </template>
            <template #refund_amount="{ row }">
              <span class="num-cell">¥ {{ formatPrice(row.refund_amount) }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无账单" />
            </template>
          </t-table>
        </section>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import {
  AddIcon,
  CartIcon,
  ChartBarIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  DashboardIcon,
  DownloadIcon,
  FileIcon,
  FilePasteIcon,
  HelpCircleIcon,
  MoneyIcon,
  RefreshIcon,
  SwapIcon,
  WalletIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getBalance,
  getMyBills,
  getMyTransactions,
  type BillInfo,
  type TransactionInfo,
  type WalletInfo,
} from '@/api/finance'
import {
  billStatusLabel,
  billStatusTheme,
  formatPrice,
  formatTime,
  txTypeLabel,
} from '@/pages/billing/constants'
import { useMemberStore } from '@/store/modules/member'

defineOptions({ name: 'BillingOverview' })

const router = useRouter()
const memberStore = useMemberStore()

// ========== 视图 ==========
type ViewKey = 'overview' | 'bills'
const view = ref<ViewKey>('overview')
const billingOpen = ref(true)
const summaryTab = ref<'product' | 'tag'>('product')

// ========== 筛选 ==========
const filter = reactive({
  mode: 'month' as 'month' | 'custom',
  month: '',
  payMethod: 'all',
  showAdjust: true,
})

const payMethodOptions = [
  { label: '付费方式：全部', value: 'all' },
  { label: '余额支付', value: 'balance' },
  { label: '在线支付', value: 'online' },
]

// ========== 数据 ==========
const loading = ref(false)
const wallet = ref<WalletInfo>({ user_id: 0, balance: 0, frozen: 0, total_income: 0, total_expense: 0 })
const bills = ref<BillInfo[]>([])
const consumeTxs = ref<TransactionInfo[]>([])

const filteredBills = computed(() => {
  if (!filter.month) return bills.value
  return bills.value.filter((b) => (b.period || '').startsWith(filter.month))
})

// 汇总：目录价 = 账单消费合计；调减 = 退款合计；优惠/优惠券暂无数据来源记为 0
const summary = computed(() => {
  const listPrice = filteredBills.value.reduce((sum, b) => sum + Number(b.total_amount || 0), 0)
  const adjust = filteredBills.value.reduce((sum, b) => sum + Number(b.refund_amount || 0), 0)
  const discount = 0
  const coupon = 0
  const payable = Math.max(0, listPrice - discount - adjust)
  const paid = filteredBills.value
    .filter((b) => b.status === 'paid')
    .reduce((sum, b) => sum + Number(b.total_amount || 0) - Number(b.refund_amount || 0), 0)
  return { listPrice, discount, coupon, adjust, payable, paid }
})

// 产品汇总：按消费流水的备注分组
const productRows = computed(() => {
  const map = new Map<string, { name: string; listPrice: number; payable: number; discount: number; adjust: number }>()
  for (const tx of consumeTxs.value) {
    if (tx.type !== 'consume') continue
    const name = tx.remark || txTypeLabel(tx.type) || '其他消费'
    const amount = Number(tx.amount || 0)
    const row = map.get(name) || { name, listPrice: 0, payable: 0, discount: 0, adjust: 0 }
    row.listPrice += amount
    row.payable += amount
    map.set(name, row)
  }
  return Array.from(map.values()).sort((a, b) => b.payable - a.payable)
})

const tagRows = computed<{ name: string; listPrice: number; payable: number }[]>(() => [])

const productColumns: PrimaryTableCol<{ name: string }>[] = [
  { colKey: 'name', title: '产品名称', minWidth: 220 },
  { colKey: 'listPrice', title: '目录价金额', width: 140, align: 'right' },
  { colKey: 'payable', title: '应付金额', width: 140, align: 'right' },
  { colKey: 'discount', title: '优惠金额', width: 120, align: 'right' },
  { colKey: 'adjust', title: '调减金额', width: 120, align: 'right' },
  { colKey: 'operation', title: '操作', width: 100 },
]

const tagColumns: PrimaryTableCol<{ name: string }>[] = [
  { colKey: 'name', title: '标签', minWidth: 220 },
  { colKey: 'listPrice', title: '目录价金额', width: 140, align: 'right' },
  { colKey: 'payable', title: '应付金额', width: 140, align: 'right' },
]

const billColumns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'period', title: '账期', width: 110 },
  { colKey: 'bill_no', title: '账单号', minWidth: 200, ellipsis: true },
  { colKey: 'total_amount', title: '消费金额', width: 130, align: 'right' },
  { colKey: 'refund_amount', title: '退款金额', width: 130, align: 'right' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

function onDevelop(name: string) {
  MessagePlugin.info(`${name}功能开发中`)
}

async function loadAll() {
  loading.value = true
  try {
    const [balRes, billRes, txRes] = await Promise.all([
      getBalance(),
      getMyBills({ page: 1, page_size: 100 }),
      getMyTransactions({ type: 'consume', page: 1, page_size: 100 }),
    ])
    if (balRes.data) wallet.value = balRes.data
    bills.value = billRes.data?.items || []
    consumeTxs.value = txRes.data?.items || []
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载费用数据失败')
  } finally {
    loading.value = false
  }
}

function reload() {
  void loadAll()
}

onMounted(loadAll)
</script>

<style scoped>
.billing-layout {
  display: flex;
  gap: 20px;
  align-items: stretch;
  min-height: calc(100vh - 112px);
}

/* ============ 左侧导航 ============ */
.billing-sider {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 10px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
  position: sticky;
  top: 80px;
  min-height: calc(100vh - 112px);
}

.sider-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #475569;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.sider-item:hover {
  background: #f1f5f9;
  color: #2563eb;
}

.sider-item.is-active {
  background: #eff6ff;
  color: #2563eb;
  font-weight: 600;
}

.sider-arrow {
  margin-left: auto;
  color: #94a3b8;
}

.sider-sub {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-left: 14px;
}

.sider-item--sub {
  padding: 8px 10px;
  font-size: 13px;
}

/* ============ 主区 ============ */
.billing-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
}

/* 余额条 */
.balance-strip {
  display: flex;
  align-items: center;
  gap: 28px;
  padding: 12px 20px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
}

.balance-strip__item {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
  color: #64748b;
}

.balance-strip__value {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.balance-strip__spacer {
  flex: 1;
}

.billing-alert {
  border-radius: 6px;
}

.top-tabs {
  margin-bottom: -4px;
}

/* ============ 面板 ============ */
.panel {
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
  padding: 18px 20px 22px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

/* ============ 筛选行 ============ */
.filter-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding-bottom: 16px;
}

.filter-spacer {
  flex: 1;
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #fff;
  color: #64748b;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.icon-btn:hover {
  color: #2563eb;
  border-color: #bfdbfe;
}

/* ============ 金额卡片 ============ */
.amount-grid {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.amount-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border: 1px solid #eef1f5;
  border-radius: 8px;
  background: #fff;
}

.amount-card--primary {
  justify-content: space-between;
  min-height: 100%;
}

.amount-side {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.amount-card-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.amount-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #64748b;
}

.amount-help {
  color: #94a3b8;
}

.amount-value {
  font-size: 20px;
  font-weight: 600;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.amount-value--primary {
  font-size: 30px;
  font-weight: 700;
  color: #f97316;
}

.amount-main {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.amount-sub {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 14px;
  border-top: 1px solid #f1f5f9;
}

/* ============ 汇总 ============ */
.summary-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.summary-head :deep(.t-tabs__nav) {
  margin-bottom: 0;
}

.summary-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.formula-text {
  margin: 14px 0;
  font-size: 13.5px;
  font-weight: 600;
  color: #334155;
}

.num-cell {
  font-variant-numeric: tabular-nums;
  color: #334155;
}

.num-cell--strong {
  font-weight: 600;
  color: #1e293b;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

/* ============ 深色模式 ============ */
.dark .billing-sider,
.dark .panel,
.dark .balance-strip,
.dark .amount-card {
  background: #141414;
  border-color: #262626;
}

.dark .sider-item {
  color: #cbd5e1;
}

.dark .sider-item:hover {
  background: #1f1f1f;
}

.dark .sider-item.is-active {
  background: #10192e;
}

.dark .page-title,
.dark .panel-title,
.dark .amount-value,
.dark .balance-strip__value,
.dark .num-cell,
.dark .formula-text {
  color: #e5e7eb;
}

.dark .balance-strip__value,
.dark .num-cell--strong {
  color: #e5e7eb;
}

.dark .amount-sub {
  border-top-color: #262626;
}

/* ============ 响应式 ============ */
@media (max-width: 1180px) {
  .amount-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 768px) {
  .billing-layout {
    flex-direction: column;
  }

  .billing-sider {
    width: 100%;
    position: static;
    min-height: auto;
  }

  .balance-strip {
    flex-wrap: wrap;
    gap: 14px;
  }

  .amount-card-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
