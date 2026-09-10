<template>
  <div class="billing-page">
    <!-- 余额总览 -->
    <section class="balance-hero">
      <div class="hero-left">
        <span class="hero-chip"><WalletIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">可用余额（元）</span>
          <span class="hero-value">¥ {{ formatPrice(wallet.balance) }}</span>
        </div>
      </div>
      <div class="hero-right">
        <t-button v-if="memberStore.has('billing:recharge')" theme="primary" size="large" @click="goRecharge">
          <template #icon><AddIcon /></template>
          立即充值
        </t-button>
      </div>
    </section>

    <!-- 统计卡片 -->
    <section class="stat-grid">
      <div class="stat-card">
        <span class="stat-icon" style="background: linear-gradient(135deg, #f59e0b, #d97706); color: #fff">
          <MoneyIcon size="22" />
        </span>
        <div class="stat-info">
          <span class="stat-value">¥ {{ formatPrice(wallet.frozen) }}</span>
          <span class="stat-label">冻结金额</span>
        </div>
      </div>
      <div class="stat-card">
        <span class="stat-icon" style="background: linear-gradient(135deg, #3b82f6, #2563eb); color: #fff">
          <SwapIcon size="22" />
        </span>
        <div class="stat-info">
          <span class="stat-value">¥ {{ formatPrice(wallet.total_income) }}</span>
          <span class="stat-label">累计收入</span>
        </div>
      </div>
      <div class="stat-card">
        <span class="stat-icon" style="background: linear-gradient(135deg, #ef4444, #dc2626); color: #fff">
          <TimeIcon size="22" />
        </span>
        <div class="stat-info">
          <span class="stat-value">¥ {{ formatPrice(wallet.total_expense) }}</span>
          <span class="stat-label">累计支出</span>
        </div>
      </div>
    </section>

    <!-- 列表：最近流水 / 我的账单 -->
    <section class="panel">
      <t-tabs v-model="activeTab" @tab-change="handleTabChange">
        <t-tab-panel value="transactions" label="最近流水">
          <div class="panel-toolbar">
            <t-space size="small">
              <t-button variant="outline" size="small" :loading="txLoading" @click="loadTransactions">
                <template #icon><RefreshIcon /></template>
                刷新
              </t-button>
              <t-link theme="primary" hover="color" @click="$router.push('/billing/transactions')">查看全部</t-link>
            </t-space>
          </div>
          <t-table
            :data="transactions"
            :columns="txColumns"
            size="small"
            row-key="id"
            :pagination="false"
            :bordered="false"
            hover
            cell-empty-content="—"
          >
            <template #type="{ row }">
              <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
                {{ txTypeLabel(row.type) }}
              </t-tag>
            </template>
            <template #amount="{ row }">
              <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">{{ formatAmount(row.amount, row.direction) }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无流水记录" />
            </template>
          </t-table>
        </t-tab-panel>

        <t-tab-panel value="bills" label="我的账单">
          <div class="panel-toolbar">
            <t-space size="small">
              <t-button variant="outline" size="small" :loading="billLoading" @click="loadBills">
                <template #icon><RefreshIcon /></template>
                刷新
              </t-button>
            </t-space>
          </div>
          <t-table
            :data="bills"
            :columns="billColumns"
            size="small"
            row-key="id"
            :pagination="false"
            :bordered="false"
            hover
            cell-empty-content="—"
          >
            <template #status="{ row }">
              <t-tag :theme="billStatusTheme(row.status)" variant="light" size="small" shape="round">
                {{ billStatusLabel(row.status) }}
              </t-tag>
            </template>
            <template #total_amount="{ row }">
              <span class="amount-expense">¥ {{ formatPrice(row.total_amount) }}</span>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #empty>
              <t-empty description="暂无账单" />
            </template>
          </t-table>
        </t-tab-panel>
      </t-tabs>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, MoneyIcon, RefreshIcon, SwapIcon, TimeIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getBalance, getMyBills, getMyTransactions, type BillInfo, type TransactionInfo, type WalletInfo } from '@/api/finance'
import {
  billStatusLabel,
  billStatusTheme,
  formatAmount,
  formatPrice,
  formatTime,
  txTypeLabel,
  txTypeTheme,
} from '@/pages/billing/constants'
import { useMemberStore } from '@/store/modules/member'

defineOptions({ name: 'BillingOverview' })

const router = useRouter()
const memberStore = useMemberStore()
const activeTab = ref('transactions')

const wallet = ref<WalletInfo>({ user_id: 0, balance: 0, frozen: 0, total_income: 0, total_expense: 0 })
const transactions = ref<TransactionInfo[]>([])
const bills = ref<BillInfo[]>([])
const txLoading = ref(false)
const billLoading = ref(false)

const txColumns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'direction', title: '方向', width: 70 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'remark', title: '备注', ellipsis: true },
  { colKey: 'created_at', title: '时间', width: 170 },
]

const billColumns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'period', title: '账期', width: 100 },
  { colKey: 'total_amount', title: '消费金额', width: 120 },
  { colKey: 'refund_amount', title: '退款金额', width: 120 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
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
  txLoading.value = true
  try {
    const { data } = await getMyTransactions({ page: 1, page_size: 5 })
    if (data) transactions.value = data.items
  } catch (error) {
    MessagePlugin.error(getErr(error) || '加载流水失败')
  } finally {
    txLoading.value = false
  }
}

async function loadBills() {
  billLoading.value = true
  try {
    const { data } = await getMyBills({ page: 1, page_size: 10 })
    if (data) bills.value = data.items
  } catch (error) {
    MessagePlugin.error(getErr(error) || '加载账单失败')
  } finally {
    billLoading.value = false
  }
}

function handleTabChange() {
  if (activeTab.value === 'bills') loadBills()
  else loadTransactions()
}

function goRecharge() {
  router.push('/billing/balance')
}

function getErr(error: unknown): string {
  return (error as Error)?.message || ''
}

onMounted(() => {
  loadBalance()
  loadTransactions()
})
</script>

<style scoped>
.billing-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* 余额大卡片 */
.balance-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 28px 32px;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
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
  gap: 6px;
}

.hero-label {
  font-size: 14px;
  opacity: 0.85;
}

.hero-value {
  font-size: 32px;
  font-weight: 700;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.hero-right .t-button {
  height: 44px;
  font-weight: 600;
}

/* 统计卡片 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.stat-card {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
}

/* 面板 */
.panel {
  background: #fff;
  border-radius: 12px;
  padding: 8px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 12px 0;
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
  .stat-grid {
    grid-template-columns: 1fr;
  }

  .balance-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
