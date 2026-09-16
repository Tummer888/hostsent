<template>
  <div class="tabs-section">
    <div class="mini-stats">
      <div class="mini-stat">
        <div class="mini-stat__label">钱包余额（元）</div>
        <div class="mini-stat__value">{{ formatAmount(wallet.balance) }}</div>
      </div>
      <div class="mini-stat">
        <div class="mini-stat__label">冻结金额（元）</div>
        <div class="mini-stat__value">{{ formatAmount(wallet.frozen) }}</div>
      </div>
      <div class="mini-stat">
        <div class="mini-stat__label">累计收入（元）</div>
        <div class="mini-stat__value mini-stat__value--income">{{ formatAmount(wallet.total_income) }}</div>
      </div>
      <div class="mini-stat">
        <div class="mini-stat__label">累计支出（元）</div>
        <div class="mini-stat__value mini-stat__value--expense">{{ formatAmount(wallet.total_expense) }}</div>
      </div>
    </div>

    <t-tabs v-model="subTab" theme="normal" class="sub-tabs">
      <t-tab-panel value="orders" :label="`订单（${info.order_count}）`">
        <div class="table-card__head">
          <h3 class="card-title">订单记录</h3>
          <span class="table-card__meta">
            共 {{ info.order_count }} 笔 · 已计入消费金额 ¥{{ formatAmount(info.order_total_amount) }}
          </span>
        </div>
        <t-table
          row-key="id"
          :data="orders"
          :columns="orderColumns"
          :loading="orderLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : orderPagination"
          @page-change="onOrderPageChange"
        >
          <template #order_no="{ row }">
            <span class="cell-mono">{{ row.order_no }}</span>
          </template>
          <template #product_name="{ row }">
            <div>
              <span class="cell-strong">{{ row.product_name || '—' }}</span>
              <div v-if="row.renewal_id > 0" class="cell-sub">续费单</div>
            </div>
          </template>
          <template #final_amount="{ row }">
            <span class="cell-muted">¥{{ formatAmount(row.final_amount) }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="orderStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ orderStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #pay_method="{ row }">
            <span>{{ row.pay_method ? payMethodLabel(row.pay_method) : '—' }}</span>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #op="{ row }">
            <t-link theme="primary" hover="color" @click="goOrderDetail(row.id)">详情</t-link>
          </template>
          <template #empty>
            <t-empty description="该用户暂无订单" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="orderMobile.current"
          :page-size="orderMobile.pageSize"
          :total="orderMobile.total"
          @go="goOrderMobilePage"
          @page-size="onOrderMobileSize"
        />
      </t-tab-panel>

      <t-tab-panel value="bills" :label="`账单（${info.bill_count}）`">
        <div class="table-card__head">
          <h3 class="card-title">账单记录</h3>
          <span class="table-card__meta">
            共 {{ info.bill_count }} 期 · 未结清 {{ info.unpaid_bill_count }} 期
          </span>
        </div>
        <t-table
          row-key="id"
          :data="bills"
          :columns="billColumns"
          :loading="billLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : billPagination"
          @page-change="onBillPageChange"
        >
          <template #bill_no="{ row }">
            <span class="cell-mono">{{ row.bill_no }}</span>
          </template>
          <template #bill_type="{ row }">
            <span>{{ billTypeLabel(row.bill_type) }}</span>
          </template>
          <template #total_amount="{ row }">
            <span class="cell-muted">¥{{ formatAmount(row.total_amount) }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="billStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ billStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #invoice_status="{ row }">
            <span>{{ invoiceStatusLabel(row.invoice_status) }}</span>
          </template>
          <template #op="{ row }">
            <t-link theme="primary" hover="color" @click="goBills(row.period)">账单页</t-link>
          </template>
          <template #empty>
            <t-empty description="该用户暂无账单" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="billMobile.current"
          :page-size="billMobile.pageSize"
          :total="billMobile.total"
          @go="goBillMobilePage"
          @page-size="onBillMobileSize"
        />
      </t-tab-panel>

      <t-tab-panel value="transactions" :label="`资金流水（${info.transaction_count}）`">
        <div class="table-card__head">
          <h3 class="card-title">资金流水</h3>
          <span class="table-card__meta">共 {{ info.transaction_count }} 条</span>
        </div>
        <t-table
          row-key="id"
          :data="transactions"
          :columns="txColumns"
          :loading="txLoading"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          :pagination="isMobile ? undefined : txPagination"
          @page-change="onTxPageChange"
        >
          <template #tx_no="{ row }">
            <span class="cell-mono">{{ row.tx_no }}</span>
          </template>
          <template #type="{ row }">
            <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
              {{ txTypeLabel(row.type) }}
            </t-tag>
          </template>
          <template #amount="{ row }">
            <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">
              {{ row.direction > 0 ? '+' : '-' }}{{ formatAmount(row.amount) }}
            </span>
          </template>
          <template #balance_after="{ row }">
            <span class="cell-muted">¥{{ formatAmount(row.balance_after) }}</span>
          </template>
          <template #remark="{ row }">
            <span class="cell-sub">{{ row.remark || '—' }}</span>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #empty>
            <t-empty description="该用户暂无资金流水" />
          </template>
        </t-table>
        <MobilePagination
          v-if="isMobile"
          :current="txMobile.current"
          :page-size="txMobile.pageSize"
          :total="txMobile.total"
          @go="goTxMobilePage"
          @page-size="onTxMobileSize"
        />
      </t-tab-panel>
    </t-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import { getWallet, getBillList, getTransactionList } from '@/api/finance'
import { getOrderList } from '@/api/order'
import {
  billStatusLabel,
  billStatusTheme,
  formatAmount,
  formatDateTime,
  orderStatusLabel,
  orderStatusTheme,
  payMethodLabel,
  txTypeLabel,
  txTypeTheme,
} from '@/pages/users/constants'
import type { UserDetailSummary } from '@/api/user'
import type { BillInfo, TransactionInfo, WalletInfo } from '@/types/interface'
import type { OrderInfo } from '@/types/order'

const props = defineProps<{
  userId: number
  isMobile: boolean
  summary: UserDetailSummary | null
  /** 聚合接口财务相关段是否降级 */
  degraded?: boolean
}>()

const router = useRouter()
const subTab = ref<'orders' | 'bills' | 'transactions'>('orders')

// 聚合的 summary 段可能因采集失败而为空；模板里用零值兜底，避免整段渲染报错。
const EMPTY_SUMMARY: UserDetailSummary = {
  instance_count: 0,
  running_instance_count: 0,
  order_count: 0,
  order_total_amount: 0,
  bill_count: 0,
  unpaid_bill_count: 0,
  transaction_count: 0,
  ticket_count: 0,
  open_ticket_count: 0,
  login_count: 0,
  active_session_count: 0,
  risk_event_count: 0,
  operation_log_count: 0,
  verification_count: 0,
}
const info = computed(() => props.summary ?? EMPTY_SUMMARY)

// ---------- 钱包摘要 ----------
const wallet = ref<WalletInfo>({ user_id: 0, balance: 0, frozen: 0, total_income: 0, total_expense: 0, version: 0 })

async function loadWallet() {
  if (!props.userId) return
  try {
    wallet.value = await getWallet(props.userId)
  } catch {
    // 钱包不存在时后端返回全 0 结构，这里兜底保持零值，不打断其余 Tab。
    wallet.value = { user_id: props.userId, balance: 0, frozen: 0, total_income: 0, total_expense: 0, version: 0 }
  }
}

// ---------- 订单 ----------
const orders = ref<OrderInfo[]>([])
const orderLoading = ref(false)
const orderPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const orderMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const orderColumns: PrimaryTableCol<OrderInfo>[] = [
  { colKey: 'order_no', title: '订单号', width: 200 },
  { colKey: 'product_name', title: '商品', minWidth: 180 },
  { colKey: 'final_amount', title: '实付', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'pay_method', title: '支付方式', width: 110 },
  { colKey: 'created_at', title: '下单时间', width: 150 },
  { colKey: 'op', title: '操作', width: 80, fixed: 'right' },
]

async function loadOrders() {
  orderLoading.value = true
  try {
    const data = await getOrderList({
      user_id: props.userId,
      page: orderPagination.current,
      page_size: orderPagination.pageSize,
    })
    orders.value = data.items || []
    orderPagination.total = data.meta?.total || 0
    orderMobile.total = orderPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载订单失败')
  } finally {
    orderLoading.value = false
  }
}

function onOrderPageChange(pageInfo: PageInfo) {
  orderPagination.current = pageInfo.current
  orderPagination.pageSize = pageInfo.pageSize
  Object.assign(orderMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadOrders()
}

function goOrderMobilePage(target: number) {
  orderPagination.current = target
  orderMobile.current = target
  void loadOrders()
}

function onOrderMobileSize(size: number) {
  orderPagination.pageSize = size
  orderPagination.current = 1
  orderMobile.pageSize = size
  orderMobile.current = 1
  void loadOrders()
}

// ---------- 账单 ----------
const bills = ref<BillInfo[]>([])
const billLoading = ref(false)
const billPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const billMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const billColumns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'bill_no', title: '账单号', width: 190 },
  { colKey: 'period', title: '账期', width: 100 },
  { colKey: 'bill_type', title: '类型', width: 110 },
  { colKey: 'total_amount', title: '金额', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'invoice_status', title: '开票', width: 100 },
  { colKey: 'op', title: '操作', width: 90, fixed: 'right' },
]

const billTypeLabels: Record<string, string> = {
  consumption: '产品购买',
  renewal: '产品续费',
  mixed: '购买+续费',
  recharge: '余额充值',
}

function billTypeLabel(type: string): string {
  return billTypeLabels[type] || type || '—'
}

const invoiceStatusLabels: Record<string, string> = {
  none: '未开票',
  applied: '已申请',
  issued: '已开票',
}

function invoiceStatusLabel(status: string): string {
  return invoiceStatusLabels[status] || status || '—'
}

async function loadBills() {
  billLoading.value = true
  try {
    const data = await getBillList({
      user_id: props.userId,
      page: billPagination.current,
      page_size: billPagination.pageSize,
    })
    bills.value = data.items || []
    billPagination.total = data.meta?.total || 0
    billMobile.total = billPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载账单失败')
  } finally {
    billLoading.value = false
  }
}

function onBillPageChange(pageInfo: PageInfo) {
  billPagination.current = pageInfo.current
  billPagination.pageSize = pageInfo.pageSize
  Object.assign(billMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadBills()
}

function goBillMobilePage(target: number) {
  billPagination.current = target
  billMobile.current = target
  void loadBills()
}

function onBillMobileSize(size: number) {
  billPagination.pageSize = size
  billPagination.current = 1
  billMobile.pageSize = size
  billMobile.current = 1
  void loadBills()
}

// ---------- 资金流水 ----------
const transactions = ref<TransactionInfo[]>([])
const txLoading = ref(false)
const txPagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const txMobile = reactive({ current: 1, pageSize: 10, total: 0 })
const txColumns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', width: 200 },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'balance_after', title: '余额', width: 120 },
  { colKey: 'remark', title: '备注', minWidth: 180 },
  { colKey: 'created_at', title: '时间', width: 150 },
]

async function loadTransactions() {
  txLoading.value = true
  try {
    const data = await getTransactionList({
      user_id: props.userId,
      page: txPagination.current,
      page_size: txPagination.pageSize,
    })
    transactions.value = data.items || []
    txPagination.total = data.meta?.total || 0
    txMobile.total = txPagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载资金流水失败')
  } finally {
    txLoading.value = false
  }
}

function onTxPageChange(pageInfo: PageInfo) {
  txPagination.current = pageInfo.current
  txPagination.pageSize = pageInfo.pageSize
  Object.assign(txMobile, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void loadTransactions()
}

function goTxMobilePage(target: number) {
  txPagination.current = target
  txMobile.current = target
  void loadTransactions()
}

function onTxMobileSize(size: number) {
  txPagination.pageSize = size
  txPagination.current = 1
  txMobile.pageSize = size
  txMobile.current = 1
  void loadTransactions()
}

// ---------- 跳转 ----------
function goOrderDetail(id: number) {
  router.push({ path: `/orders/detail/${id}` })
}

function goBills(period: string) {
  router.push({ path: '/finance/bills', query: { user_id: String(props.userId), period } })
}

// 懒加载：三个子表按子 Tab 首次进入才请求，避免一次打开触发 4 个请求。
watch(
  () => [props.userId, subTab.value] as const,
  ([uid, tab]) => {
    if (!uid) return
    if (tab === 'orders' && !orders.value.length) void loadOrders()
    if (tab === 'bills' && !bills.value.length) void loadBills()
    if (tab === 'transactions' && !transactions.value.length) void loadTransactions()
  },
  { immediate: true },
)

watch(
  () => props.userId,
  () => {
    orderPagination.current = 1
    billPagination.current = 1
    txPagination.current = 1
    orders.value = []
    bills.value = []
    transactions.value = []
    void loadWallet()
  },
  { immediate: true },
)

defineExpose({
  reload: () => {
    void loadWallet()
    void loadOrders()
    void loadBills()
    void loadTransactions()
  },
})
</script>