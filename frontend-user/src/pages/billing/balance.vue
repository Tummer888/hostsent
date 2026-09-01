<template>
  <div class="billing-page">
    <!-- 余额 -->
    <section class="balance-hero">
      <div class="hero-left">
        <span class="hero-chip"><WalletIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">可用余额（元）</span>
          <span class="hero-value">¥ {{ formatPrice(wallet.balance) }}</span>
          <span class="hero-sub">冻结 ¥ {{ formatPrice(wallet.frozen) }} · 累计收入 ¥ {{ formatPrice(wallet.total_income) }}</span>
        </div>
      </div>
      <div class="hero-right">
        <t-button theme="primary" size="large" @click="openRechargeDialog">
          <template #icon><AddIcon /></template>
          余额充值
        </t-button>
      </div>
    </section>

    <!-- 我的充值单 -->
    <section v-if="recharges.length" class="panel">
      <h3 class="section-title">我的充值单</h3>
      <t-table :data="recharges" :columns="rechargeColumns" size="small" row-key="id" :pagination="false" :bordered="false" hover cell-empty-content="—">
        <template #amount="{ row }">
          <span class="amount-income">¥ {{ formatPrice(row.amount) }}</span>
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
    <section class="panel">
      <div class="panel-head">
        <h3 class="section-title">资金流水</h3>
        <t-space size="small">
          <t-select v-model="filter.type" clearable placeholder="类型" :options="txTypeOptions" size="small" style="width: 120px" @change="handleSearch" />
          <t-select v-model="filter.direction" clearable placeholder="方向" :options="directionOptions" size="small" style="width: 100px" @change="handleSearch" />
          <t-button variant="outline" size="small" :loading="loading" @click="loadTransactions">
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
        :pagination="pagination"
        :bordered="false"
        hover
        cell-empty-content="—"
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
      <t-alert theme="info" message="提交后将生成待支付充值单，需人工/渠道确认后到账。" style="margin-bottom: 16px" />
      <t-form label-align="top" :data="rechargeForm" @submit.prevent>
        <t-form-item label="充值金额（元）" name="amount">
          <t-input-number v-model="rechargeForm.amount" :min="1" :precision="2" theme="column" placeholder="请输入充值金额" />
        </t-form-item>
        <t-form-item label="支付方式" name="method">
          <t-select v-model="rechargeForm.method" placeholder="请选择支付方式" :options="rechargeMethodOptions" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-input v-model="rechargeForm.remark" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createRecharge,
  getBalance,
  getMyTransactions,
  type RechargeInfo,
  type TransactionInfo,
  type WalletInfo,
} from '@/api/finance'
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

defineOptions({ name: 'BillingBalance' })

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
  { colKey: 'method', title: '方式', width: 100 },
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

const rechargeVisible = ref(false)
const rechargeForm = reactive<{ amount: number | undefined; method: string | undefined; remark: string | undefined }>({
  amount: undefined,
  method: undefined,
  remark: undefined,
})

function openRechargeDialog() {
  rechargeForm.amount = undefined
  rechargeForm.method = undefined
  rechargeForm.remark = undefined
  rechargeVisible.value = true
}

async function handleRecharge() {
  if (!rechargeForm.amount || rechargeForm.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的充值金额')
    return
  }
  if (!rechargeForm.method) {
    MessagePlugin.warning('请选择支付方式')
    return
  }
  try {
    const { data } = await createRecharge({
      amount: rechargeForm.amount,
      method: rechargeForm.method,
      remark: rechargeForm.remark,
    })
    rechargeVisible.value = false
    MessagePlugin.success(`充值单 ${data?.recharge_no || ''} 已提交，等待确认到账`)
    recharges.value = [data as RechargeInfo, ...recharges.value]
    loadBalance()
  } catch (error) {
    MessagePlugin.error(getErr(error) || '提交充值单失败')
  }
}

function getErr(error: unknown): string {
  return (error as Error)?.message || ''
}

onMounted(() => {
  loadBalance()
  loadTransactions()
  // 展示我的充值单：从流水里过滤充值类型
  getMyTransactions({ type: 'recharge', page: 1, page_size: 20 })
    .then(({ data }) => {
      if (data?.items?.length) {
        recharges.value = data.items as unknown as RechargeInfo[]
      }
    })
    .catch(() => {
      /* 忽略，仅作展示 */
    })
})
</script>

<style scoped>
.billing-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.balance-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 24px 28px;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.hero-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.hero-chip {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.18);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-label {
  font-size: 14px;
  opacity: 0.85;
}

.hero-value {
  font-size: 30px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
}

.hero-sub {
  font-size: 13px;
  opacity: 0.8;
}

.hero-right .t-button {
  height: 44px;
  font-weight: 600;
}

.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 8px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.amount-income {
  color: #059669;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.amount-expense {
  color: #dc2626;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 768px) {
  .balance-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
