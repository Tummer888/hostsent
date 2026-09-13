<template>
  <div class="page-body console-module balance-module">
    <!-- 余额概览：与管理端财务页/首页仪表盘同一套统计卡片 -->
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">余额与充值</h2>
          <p class="page-header__desc">余额支付即时到账；冻结金额为退款/提现处理中占用</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/billing')">返回费用中心</t-button>
        <t-button theme="primary" @click="openRechargeDialog">
          <template #icon><AddIcon /></template>
          余额充值
        </t-button>
      </div>
    </header>

    <section class="balance-stat-grid">
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
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.total_income) }}</span>
          <span class="stat-card__hint">累计收入</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.total_expense) }}</span>
          <span class="stat-card__hint">累计支出</span>
        </div>
      </div>
    </section>

    <!-- 我的充值单 -->
    <section v-if="recharges.length" class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">我的充值单</h3>
      </div>
      <t-table :data="recharges" :columns="rechargeColumns" row-key="id" :pagination="false" hover cell-empty-content="—">
        <template #amount="{ row }">
          <span class="amount-income">¥ {{ formatPrice(row.amount) }}</span>
        </template>
        <template #method="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ rechargeMethodLabel(row.method) }}</span>
            <span v-if="row.channel_code" class="cell-sub">{{ row.channel_code }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <t-tag :theme="rechargeStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ rechargeStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无充值单" />
        </template>
      </t-table>
    </section>

    <!-- 资金流水 -->
    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">资金流水</h3>
        <t-space size="small">
          <t-select v-model="filter.type" clearable placeholder="类型" :options="txTypeOptions" size="small" style="width: 120px" @change="handleSearch" />
          <t-select v-model="filter.direction" clearable placeholder="方向" :options="directionOptions" size="small" style="width: 100px" @change="handleSearch" />
          <t-button variant="outline" size="small" :loading="loading" @click="loadAll">
            <template #icon><RefreshIcon /></template>
            刷新
          </t-button>
        </t-space>
      </div>
      <t-table
        :data="transactions"
        :columns="txColumns"
        size="small"
        row-key="id"
        :pagination="isMobile ? undefined : pagination"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="loading"
        @page-change="handlePageChange"
      >
        <template #type="{ row }">
          <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ txTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #direction="{ row }">
          <span>{{ directionLabel(row.direction) }}</span>
        </template>
        <template #amount="{ row }">
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">{{ formatAmount(row.amount, row.direction) }}</span>
        </template>
        <template #balance="{ row }">
          <span class="time-text">{{ formatPrice(row.balance_after) }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无流水记录" />
        </template>
      </t-table>

      <!-- 移动端翻页：与 admin 列表页同一套（桌面用表格内建分页） -->
      <MobilePagination
        v-if="isMobile"
        :current="pagination.current"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        @change="handlePageChange"
      />
    </section>

    <!-- 充值弹窗 -->
    <t-dialog
      v-model:visible="rechargeVisible"
      header="余额充值"
      width="480px"
      :confirm-btn="{ content: '提交充值单', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRecharge"
      @close="rechargeVisible = false"
    >
      <t-alert theme="info" message="提交后进入支付中心收银台，按所选渠道完成支付后可即时到账；线下方式需后台确认。" style="margin-bottom: 16px" />
      <t-form label-align="top" :data="rechargeForm" @submit.prevent>
        <t-form-item label="充值金额（元）" name="amount">
          <t-input-number v-model="rechargeForm.amount" :min="1" :precision="2" theme="column" placeholder="请输入充值金额" />
        </t-form-item>
        <t-form-item label="支付方式" name="method">
          <t-select v-model="rechargeForm.method" placeholder="请选择支付方式" :options="rechargeChannelOptions" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 收银台：渠道支付参数 -->
    <t-dialog
      v-model:visible="cashierVisible"
      header="收银台"
      width="520px"
      :footer="false"
      @close="cashierVisible = false"
    >
      <template v-if="cashierOrder">
        <t-alert theme="success" :message="`支付单 ${cashierOrder.payment_no} 已创建，应付 ¥${formatPrice(cashierOrder.amount)}`" style="margin-bottom: 12px" />
        <div v-if="cashierOrder.instructions" class="cashier-block">
          <h4 class="cashier-block__title">支付指引</h4>
          <pre class="cashier-pre">{{ cashierOrder.instructions }}</pre>
        </div>
        <div v-if="cashierOrder.qrcode" class="cashier-block">
          <h4 class="cashier-block__title">收款二维码</h4>
          <img v-if="isImage(cashierOrder.qrcode)" class="cashier-qr" :src="cashierOrder.qrcode" alt="收款二维码" />
          <pre v-else class="cashier-pre">{{ cashierOrder.qrcode }}</pre>
        </div>
        <div v-if="cashierOrder.pay_url" class="cashier-block">
          <h4 class="cashier-block__title">前往支付</h4>
          <a class="cashier-link" :href="cashierOrder.pay_url" target="_blank" rel="noopener noreferrer">{{ cashierOrder.pay_url }}</a>
        </div>
        <t-space size="small" class="cashier-actions">
          <t-button theme="primary" :loading="checking" @click="checkPayment">我已完成支付</t-button>
          <t-button variant="outline" @click="cashierVisible = false">稍后支付</t-button>
        </t-space>
        <p class="cashier-tip">支付完成后点击「我已完成支付」刷新状态；线下渠道需等待财务确认到账。</p>
      </template>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, LockOnIcon, MoneyIcon, RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getBalance,
  getMyRecharges,
  getMyTransactions,
  type RechargeInfo,
  type TransactionInfo,
  type WalletInfo,
} from '@/api/finance'
import { getPaymentMethods, getPaymentOrder, payRecharge, type PaymentOrderInfo } from '@/api/payment'
import {
  directionLabel,
  directionOptions,
  formatAmount,
  formatPrice,
  formatTime,
  rechargeMethodOptions,
  rechargeStatusLabel,
  rechargeStatusTheme,
  txTypeLabel,
  txTypeOptions,
  txTypeTheme,
} from '@/pages/billing/constants'
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'BillingBalance' })

const router = useRouter()
const { isMobile } = useIsMobile()

const wallet = ref<WalletInfo>({ user_id: 0, balance: 0, frozen: 0, total_income: 0, total_expense: 0 })
const transactions = ref<TransactionInfo[]>([])
const recharges = ref<RechargeInfo[]>([])
const loading = ref(false)

const filter = reactive<{ type: string | undefined; direction: number | undefined }>({ type: undefined, direction: undefined })

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const txColumns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'direction', title: '方向', width: 70 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'balance', title: '余额', width: 100 },
  { colKey: 'remark', title: '备注', ellipsis: true },
  { colKey: 'created_at', title: '时间', width: 170 },
]

const rechargeColumns: PrimaryTableCol<RechargeInfo>[] = [
  { colKey: 'recharge_no', title: '充值单号', width: 200 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'method', title: '支付方式', width: 120 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadBalance() {
  try {
    const { data } = await getBalance()
    if (data) wallet.value = data
  } catch (error) {
    MessagePlugin.error(getErr(error) || '加载余额失败')
  }
}

async function loadTransactions() {
  loading.value = true
  try {
    const { data } = await getMyTransactions({
      type: filter.type,
      direction: filter.direction,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    if (data) {
      transactions.value = data.items
      pagination.total = data.meta.total
    }
  } catch (error) {
    MessagePlugin.error(getErr(error) || '加载流水失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTransactions()
}

function handleSearch() {
  pagination.current = 1
  loadTransactions()
}

/** 顶栏「刷新」：余额与流水一起重取（充值单是历史记录，不随余额变动）。 */
function loadAll() {
  void Promise.all([loadBalance(), loadTransactions()])
}

const rechargeVisible = ref(false)
const rechargeForm = reactive<{ amount: number | undefined; method: string | undefined }>({
  amount: undefined,
  method: undefined,
})
// 收银台可用渠道（来自支付中心，按我的偏好排序）
const rechargeChannelOptions = ref<Array<{ label: string; value: string }>>([])

async function loadPaymentMethods() {
  try {
    const { data } = await getPaymentMethods({ scene: 'native' })
    const channels = data?.channels || []
    rechargeChannelOptions.value = channels.map((c) => ({ label: c.name, value: c.channel_code }))
  } catch {
    rechargeChannelOptions.value = []
  }
}

function openRechargeDialog() {
  rechargeForm.amount = undefined
  rechargeForm.method = undefined
  void loadPaymentMethods()
  rechargeVisible.value = true
}

const cashierVisible = ref(false)
const cashierOrder = ref<PaymentOrderInfo | null>(null)
const checking = ref(false)

function isImage(value: string): boolean {
  return /^(data:image|https?:\/\/.*\.(png|jpe?g|gif|webp|svg))/i.test(value)
}

async function handleRecharge() {
  if (!rechargeForm.amount || rechargeForm.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的充值金额')
    return
  }
  try {
    const { data } = await payRecharge({
      amount: rechargeForm.amount,
      channel_code: rechargeForm.method,
    })
    rechargeVisible.value = false
    cashierOrder.value = data || null
    cashierVisible.value = true
    if (data?.pay_url && !data?.instructions && !data?.qrcode) {
      window.open(data.pay_url, '_blank', 'noopener')
    }
  } catch (error) {
    MessagePlugin.error(getErr(error) || '发起充值失败')
  }
}

// 轮询支付单状态：渠道回调到账后前端可感知。
async function checkPayment() {
  if (!cashierOrder.value) return
  checking.value = true
  try {
    const { data } = await getPaymentOrder(cashierOrder.value.payment_no)
    cashierOrder.value = data || cashierOrder.value
    if (data?.status === 'paid') {
      MessagePlugin.success('支付已完成，余额已到账')
      cashierVisible.value = false
      wallet.value = { ...wallet.value, balance: wallet.value.balance + Number(data.amount || 0) }
      await Promise.all([loadBalance(), loadTransactions()])
      return
    }
    if (data?.status === 'pending' || data?.status === 'paying') {
      MessagePlugin.info('支付尚未完成，请稍候再试；线下方式需等待财务确认到账')
      return
    }
    MessagePlugin.warning(`支付单状态：${data?.status || '未知'}`)
  } catch (error) {
    MessagePlugin.error(getErr(error) || '查询支付状态失败')
  } finally {
    checking.value = false
  }
}

function getErr(error: unknown): string {
  return (error as Error)?.message || ''
}

// 充值单支付方式：优先展示支付中心渠道，回退到历史 method 标签。
function rechargeMethodLabel(method: string): string {
  const found = rechargeMethodOptions.find((m) => m.value === method)
  return found ? found.label : method || '—'
}

function loadRecharges() {
  getMyRecharges({ page: 1, page_size: 20 })
    .then(({ data }) => {
      recharges.value = data?.items || []
    })
    .catch(() => {
      recharges.value = []
    })
}

onMounted(() => {
  loadBalance()
  loadTransactions()
  loadRecharges()
})
</script>

<style scoped>
/* 余额概览：与 admin 首页仪表盘/财务页同一套统计卡片栅格 */
.balance-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--space-lg);
}

/* 收银台 */
.cashier-block {
  margin-bottom: 14px;
}

.cashier-block__title {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
}

.cashier-pre {
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f8fafc;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.cashier-qr {
  width: 180px;
  height: 180px;
  object-fit: contain;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.cashier-link {
  font-size: 12px;
  color: #2563eb;
  word-break: break-all;
}

.cashier-actions {
  margin-top: 6px;
}

.cashier-tip {
  margin: 10px 0 0;
  font-size: 12px;
  color: #64748b;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .balance-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
