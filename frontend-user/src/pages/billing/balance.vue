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

    <!-- 充值弹窗：只收金额，支付渠道在收银台里选（渠道限额与金额相关，必须在收银台内过滤） -->
    <t-dialog
      v-model:visible="rechargeVisible"
      header="余额充值"
      width="480px"
      :confirm-btn="{ content: '下一步：选择支付方式', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRecharge"
      @close="rechargeVisible = false"
    >
      <t-alert theme="info" message="提交后进入支付中心收银台，按所选渠道完成支付后可即时到账；线下方式需后台确认。" style="margin-bottom: 16px" />
      <t-form label-align="top" :data="rechargeForm" @submit.prevent>
        <t-form-item label="充值金额（元）" name="amount">
          <t-input-number v-model="rechargeForm.amount" :min="1" :precision="2" theme="column" placeholder="请输入充值金额" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 收银台：与订单支付/账单支付共用同一组件 -->
    <PaymentCashier
      v-model:visible="cashierVisible"
      :amount="cashierAmount"
      :submit="submitRecharge"
      @paid="onRechargePaid"
    />
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
import { payRecharge } from '@/api/payment'
import PaymentCashier, { type CashierPayment } from '@/components/payment-cashier/index.vue'
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
const rechargeForm = reactive<{ amount: number | undefined }>({ amount: undefined })

function openRechargeDialog() {
  rechargeForm.amount = undefined
  rechargeVisible.value = true
}

const cashierVisible = ref(false)
/** 本次充值的金额：先记下来，收银台拉起后按金额过滤可选渠道。 */
const cashierAmount = ref(0)

/** 收银台回调：金额校验 + 创建充值单并发起支付（渠道由收银台选定）。 */
function submitRecharge(channelCode: string, scene: string): Promise<CashierPayment> {
  return payRecharge({
    amount: cashierAmount.value,
    channel_code: channelCode,
    scene,
  }).then(({ data }) => ({
    payment_no: data.payment_no,
    amount: data.amount,
    pay_url: data.pay_url,
    qrcode: data.qrcode,
    instructions: data.instructions,
    status: data.status,
  }))
}

async function handleRecharge() {
  if (!rechargeForm.amount || rechargeForm.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的充值金额')
    return
  }
  cashierAmount.value = rechargeForm.amount
  rechargeVisible.value = false
  cashierVisible.value = true
}

/** 到账后余额与流水都变了，重新拉取（不本地加金额，避免与线下确认到账的时点不一致）。 */
async function onRechargePaid() {
  MessagePlugin.success('支付已完成，余额已到账')
  await Promise.all([loadBalance(), loadTransactions()])
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

@media (max-width: 768px) {
  .balance-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
