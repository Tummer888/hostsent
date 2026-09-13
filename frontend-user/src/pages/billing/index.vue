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
      <button class="sider-item" @click="router.push('/billing/invoices')">
        <FilePasteIcon size="16" />
        <span>发票管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('合同管理')">
        <FileIcon size="16" />
        <span>合同管理</span>
      </button>
    </aside>

    <!-- ============ 右侧内容 ============ -->
    <div class="billing-main page-body console-module">
      <header class="page-header surface-card">
        <div class="page-header__main">
          <span class="page-header__chip">
            <MoneyIcon size="22" aria-hidden="true" />
          </span>
          <div class="page-header__text">
            <h2 class="page-header__title">{{ view === 'overview' ? '消费总览' : '资源月账单' }}</h2>
            <p class="page-header__desc">余额支付即时到账；当月账单次月 1 日 9 点后可作为对账依据</p>
          </div>
        </div>
        <div class="page-header__actions">
          <t-button variant="outline" :loading="loading" @click="reload">刷新</t-button>
          <t-button
            v-if="memberStore.has('billing:recharge')"
            theme="primary"
            @click="router.push('/billing/balance')"
          >
            <template #icon><AddIcon /></template>
            立即充值
          </t-button>
        </div>
      </header>

      <!-- 资金概览：与首页仪表盘/管理端财务页同一套统计卡片 -->
      <section class="billing-stat-grid">
        <div class="stat-card surface-card stat-card--blue">
          <span class="stat-card__icon"><WalletIcon size="24" aria-hidden="true" /></span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(wallet.balance) }}</span>
            <span class="stat-card__hint">可用余额</span>
          </div>
        </div>
        <div class="stat-card surface-card stat-card--warning">
          <span class="stat-card__icon"><LockOnIcon size="24" aria-hidden="true" /></span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(wallet.frozen) }}</span>
            <span class="stat-card__hint">冻结金额</span>
          </div>
        </div>
        <div class="stat-card surface-card stat-card--green">
          <span class="stat-card__icon"><ChartBarIcon size="24" aria-hidden="true" /></span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(summary.payable) }}</span>
            <span class="stat-card__hint">本期应付{{ filter.month ? `（${filter.month}）` : '' }}</span>
          </div>
        </div>
        <div class="stat-card surface-card stat-card--orange">
          <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(summary.paid) }}</span>
            <span class="stat-card__hint">本期已付</span>
          </div>
        </div>
      </section>

      <t-alert theme="info" class="billing-alert">
        当月最终账单在次月1日9点后支持查看/导出，在此之前的数据查询结果仅供参考，不作为对账依据。
      </t-alert>

      <t-tabs v-model="view" class="top-tabs">
        <t-tab-panel value="overview" label="消费总览" />
        <t-tab-panel value="bills" label="资源月账单" />
      </t-tabs>

      <!-- ---------- 消费总览 ---------- -->
      <template v-if="view === 'overview'">
        <section class="surface-card billing-panel">
          <div class="table-card__head">
            <h3 class="card-title">支付详情</h3>
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
        <section class="surface-card billing-panel">
          <div class="table-card__head">
            <h3 class="card-title">资源月账单</h3>
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
            <template #bill_no="{ row }">
              <div class="cell-main">
                <span class="cell-strong">{{ row.bill_no }}</span>
                <t-tag :theme="billTypeTheme(row.bill_type)" variant="light" size="small" shape="round">
                  {{ billTypeLabel(row.bill_type) }}
                </t-tag>
              </div>
            </template>
            <template #total_amount="{ row }">
              <div class="cell-main">
                <span class="num-cell num-cell--strong">¥ {{ formatPrice(row.total_amount) }}</span>
                <span v-if="(row.channel_refund_amount || 0) > 0" class="cell-sub">
                  原路退回 ¥{{ formatPrice(row.channel_refund_amount) }}
                </span>
              </div>
            </template>
            <template #refund_amount="{ row }">
              <div class="cell-main">
                <span class="num-cell">¥ {{ formatPrice(row.refund_amount) }}</span>
                <span v-if="(row.refund_fee_amount || 0) > 0" class="cell-sub">
                  扣点 ¥{{ formatPrice(row.refund_fee_amount) }}
                </span>
              </div>
            </template>
            <template #paid_method="{ row }">
              <div v-if="row.status === 'paid'" class="cell-main">
                <span class="cell-strong">{{ payMethodLabel(row.paid_method) }}</span>
                <span class="cell-sub">实收 ¥{{ formatPrice(row.paid_amount ?? row.total_amount - row.refund_amount) }}</span>
              </div>
              <span v-else class="time-text">—</span>
            </template>
            <template #invoice_status="{ row }">
              <div class="cell-main">
                <t-tag :theme="billInvoiceStatusTheme(row.invoice_status)" variant="light" size="small" shape="round">
                  {{ billInvoiceStatusLabel(row.invoice_status) }}
                </t-tag>
                <span v-if="row.invoice_no" class="cell-sub">{{ row.invoice_no }}</span>
              </div>
            </template>
            <template #action="{ row }">
              <div class="cell-main">
                <t-link
                  v-if="row.status === 'unpaid' && memberStore.has('billing:recharge')"
                  theme="primary"
                  hover="color"
                  @click="openBillPay(row)"
                >
                  去支付
                </t-link>
                <t-link
                  v-else-if="canApplyInvoice(row)"
                  theme="primary"
                  hover="color"
                  @click="openInvoiceDialog(row)"
                >
                  申请开票
                </t-link>
                <span v-else class="time-text">—</span>
              </div>
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

    <!-- 账单支付收银台 -->
    <t-dialog
      v-model:visible="billPayVisible"
      header="账单支付"
      width="520px"
      :confirm-btn="{ content: '发起支付', theme: 'primary', loading: billPaying }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleBillPay"
      @close="billPayVisible = false"
    >
      <t-alert
        v-if="billPayTarget"
        theme="info"
        :message="`账单 ${billPayTarget.bill_no}（账期 ${billPayTarget.period}）应结 ¥${formatPrice(billDue)}`"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top" :data="billPayForm" @submit.prevent>
        <t-form-item label="支付方式">
          <t-select v-model="billPayForm.channel_code" placeholder="选择支付方式" :options="billPayChannelOptions" />
        </t-form-item>
      </t-form>
      <p class="dialog-tip">支付单将由平台收银台创建；线下人工方式会返回转账指引，需后台确认到账。</p>
    </t-dialog>

    <!-- 申请开票（doc36 §3.3） -->
    <t-dialog
      v-model:visible="invoiceVisible"
      header="申请开票"
      width="520px"
      :confirm-btn="{ content: '提交申请', theme: 'primary', loading: invoiceSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleApplyInvoice"
      @close="invoiceVisible = false"
    >
      <t-alert
        v-if="invoiceTarget"
        theme="info"
        :message="`账单 ${invoiceTarget.bill_no}（账期 ${invoiceTarget.period}）可开票金额 ¥${formatPrice(invoiceTarget.total_amount)}`"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top" :data="invoiceForm" @submit.prevent>
        <t-form-item label="发票类型">
          <t-select v-model="invoiceForm.invoice_type" :options="invoiceTypeOptions" />
        </t-form-item>
        <t-form-item label="发票抬头（必填）">
          <t-input v-model="invoiceForm.title" placeholder="企业或个人名称" clearable />
        </t-form-item>
        <t-form-item label="纳税人识别号">
          <t-input v-model="invoiceForm.tax_no" placeholder="专票必填" clearable />
        </t-form-item>
        <t-form-item label="接收邮箱">
          <t-input v-model="invoiceForm.email" placeholder="发票开具后发送至该邮箱" clearable />
        </t-form-item>
      </t-form>
      <p class="dialog-tip">提交后由平台审核开票；驳回后账单回到未开票状态，可重新申请。</p>
    </t-dialog>
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
  LockOnIcon,
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
  applyInvoice,
  type BillInfo,
  type InvoiceApplyRequest,
  type TransactionInfo,
  type WalletInfo,
} from '@/api/finance'
import {
  billInvoiceStatusLabel,
  billInvoiceStatusTheme,
  billStatusLabel,
  billStatusTheme,
  billTypeLabel,
  billTypeTheme,
  formatPrice,
  formatTime,
  invoiceTypeOptions,
  payMethodLabel,
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
  { colKey: 'period', title: '账期', width: 100 },
  { colKey: 'bill_no', title: '账单号 / 分类', minWidth: 190 },
  { colKey: 'total_amount', title: '消费金额', width: 140 },
  { colKey: 'refund_amount', title: '退款金额', width: 140 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'paid_method', title: '支付方式', width: 150 },
  { colKey: 'invoice_status', title: '发票', width: 130 },
  { colKey: 'action', title: '操作', width: 110 },
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

// ========== 账单支付（支付中心收银台） ==========
const billPayVisible = ref(false)
const billPaying = ref(false)
const billPayTarget = ref<BillInfo | null>(null)
const billPayForm = reactive<{ channel_code: string | undefined }>({ channel_code: undefined })
const billPayChannelOptions = ref<Array<{ label: string; value: string }>>([])

const billDue = computed(() => {
  if (!billPayTarget.value) return 0
  return Math.max(0, Number(billPayTarget.value.total_amount || 0) - Number(billPayTarget.value.refund_amount || 0))
})

async function openBillPay(row: BillInfo) {
  billPayTarget.value = row
  billPayForm.channel_code = undefined
  try {
    const { getPaymentMethods } = await import('@/api/payment')
    const { data } = await getPaymentMethods({ scene: 'native', amount: billDue.value })
    const channels = data?.channels || []
    billPayChannelOptions.value = channels.map((c) => ({ label: c.name, value: c.channel_code }))
    billPayForm.channel_code = data?.default || channels[0]?.channel_code
  } catch {
    billPayChannelOptions.value = []
  }
  billPayVisible.value = true
}

async function handleBillPay() {
  if (!billPayTarget.value) return
  billPaying.value = true
  try {
    const { payBill } = await import('@/api/payment')
    const target = billPayTarget.value
    const { data } = await payBill({
      bill_id: target.id,
      bill_no: target.bill_no,
      amount: billDue.value,
      channel_code: billPayForm.channel_code,
    })
    billPayVisible.value = false
    // 线下渠道返回转账指引；在线渠道返回支付链接/二维码。
    if (data?.instructions) {
      MessagePlugin.success(`支付单 ${data.payment_no} 已创建，请按指引完成转账`)
    } else {
      MessagePlugin.success(`支付单 ${data?.payment_no || ''} 已创建，请在渠道完成支付`)
    }
    if (data?.pay_url) {
      window.open(data.pay_url, '_blank', 'noopener')
    }
    void loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '发起账单支付失败')
  } finally {
    billPaying.value = false
  }
}

// ========== 申请开票（doc36 §3.3） ==========
const invoiceVisible = ref(false)
const invoiceSubmitting = ref(false)
const invoiceTarget = ref<BillInfo | null>(null)
const invoiceForm = reactive<{ invoice_type: string; title: string; tax_no: string; email: string }>({
  invoice_type: 'normal',
  title: '',
  tax_no: '',
  email: '',
})

// 只有已结清且未开票（或曾被驳回）的账单可申请开票。
function canApplyInvoice(row: BillInfo): boolean {
  if (row.status !== 'paid') return false
  return !row.invoice_status || row.invoice_status === 'none' || row.invoice_status === 'rejected'
}

function openInvoiceDialog(row: BillInfo) {
  invoiceTarget.value = row
  invoiceForm.invoice_type = 'normal'
  invoiceForm.title = ''
  invoiceForm.tax_no = ''
  invoiceForm.email = ''
  invoiceVisible.value = true
}

async function handleApplyInvoice() {
  if (!invoiceTarget.value) return
  if (!invoiceForm.title.trim()) {
    MessagePlugin.warning('请填写发票抬头')
    return
  }
  invoiceSubmitting.value = true
  try {
    const payload: InvoiceApplyRequest = {
      bill_id: invoiceTarget.value.id,
      invoice_type: invoiceForm.invoice_type,
      title: invoiceForm.title.trim(),
      tax_no: invoiceForm.tax_no.trim() || undefined,
      email: invoiceForm.email.trim() || undefined,
    }
    const { data } = await applyInvoice(payload)
    MessagePlugin.success(`开票申请 ${data.request_no} 已提交，请等待平台审核`)
    invoiceVisible.value = false
    void loadAll()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '提交开票申请失败')
  } finally {
    invoiceSubmitting.value = false
  }
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
/* 主区本身就是 .console-module 骨架（page-body），间距由骨架契约提供；
   这里只补骨架没覆盖的部分。 */
.billing-main {
  flex: 1;
  min-width: 0;
}

/* 资金概览：与首页仪表盘/管理端财务页同一套 stat-card（间距沿用骨架 gap） */
.billing-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--space-lg);
}

.billing-alert {
  border-radius: var(--hs-radius-lg);
}

.top-tabs {
  margin-bottom: -4px;
}

/* ============ 面板（surface-card 内的内容留白） ============ */
.billing-panel {
  padding: var(--space-lg) 20px 22px;
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
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-1);
  color: var(--color-muted-foreground);
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.icon-btn:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
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
  color: var(--color-foreground);
}

/* 数字列：右对齐用的等宽数字。cell-strong / cell-sub / time-text 由骨架契约提供 */
.num-cell {
  font-variant-numeric: tabular-nums;
  color: var(--color-foreground);
}

.num-cell--strong {
  font-weight: 600;
}

/* 金额列里「金额 + 副说明」的竖排（账单表多处使用） */
.cell-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: flex-start;
}

.dialog-tip {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
  line-height: 1.6;
}

/* ============ 深色模式 ============ */
.dark .billing-sider,
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

.dark .amount-value,
.dark .num-cell,
.dark .formula-text {
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

  .billing-panel {
    padding: var(--space-lg) var(--space-md);
  }

  .amount-card-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
