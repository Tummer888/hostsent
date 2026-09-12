<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">支付概览</h2>
          <p class="page-header__desc">支付中心全链路一屏：渠道健康、收款与退款、出款打款、回调与对账。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(stats.paidAmount) }}</span>
          <span class="stat-card__label">已支付金额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><TimeIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ stats.payingCount }}</span>
          <span class="stat-card__label">待支付 / 支付中</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><LinkIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ stats.enabledChannels }} / {{ stats.totalChannels }}</span>
          <span class="stat-card__label">启用渠道 / 全部</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--purple">
        <span class="stat-card__icon"><UploadIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ stats.pendingPayouts }}</span>
          <span class="stat-card__label">待处理打款</span>
        </div>
      </div>
    </section>

    <div class="overview-grid">
      <section class="table-card surface-card">
        <div class="table-card__head">
          <h3 class="card-title">渠道健康</h3>
          <span class="table-card__meta">共 {{ channelList.length }} 个渠道</span>
        </div>
        <t-table row-key="id" :data="channelList" :columns="channelColumns" :loading="loading" size="small" hover cell-empty-content="—">
          <template #channel_code="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ row.name }}</span>
              <span class="price-sub">{{ row.channel_code }}</span>
            </div>
          </template>
          <template #mode="{ row }">
            <t-tag :theme="modeTheme(row.mode)" variant="light" size="small" shape="round">{{ modeLabel(row.mode) }}</t-tag>
          </template>
          <template #health_status="{ row }">
            <t-tag :theme="healthTheme(row.health_status)" variant="light" size="small" shape="round">
              {{ healthLabel(row.health_status) }}
            </t-tag>
          </template>
          <template #status="{ row }">
            <t-tag :theme="channelStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ channelStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #empty><t-empty description="暂无渠道" /></template>
        </t-table>
      </section>

      <section class="table-card surface-card">
        <div class="table-card__head">
          <h3 class="card-title">最近支付单</h3>
          <span class="table-card__meta">近 10 笔</span>
        </div>
        <t-table row-key="id" :data="recentOrders" :columns="orderColumns" :loading="loading" size="small" hover cell-empty-content="—">
          <template #payment_no="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ row.payment_no }}</span>
              <span class="price-sub">{{ bizTypeLabel(row.biz_type) }}</span>
            </div>
          </template>
          <template #amount="{ row }">
            <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
          </template>
          <template #status="{ row }">
            <t-tag :theme="orderStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ orderStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #created_at="{ row }">
            <span class="time-text">{{ formatTime(row.created_at) }}</span>
          </template>
          <template #empty><t-empty description="暂无支付单" /></template>
        </t-table>
      </section>
    </div>

    <div class="overview-grid">
      <section class="table-card surface-card">
        <div class="table-card__head">
          <h3 class="card-title">退款单</h3>
          <span class="table-card__meta">近 {{ refundList.length }} 笔</span>
        </div>
        <t-table row-key="id" :data="refundList" :columns="refundColumns" :loading="loading" size="small" hover cell-empty-content="—">
          <template #refund_no="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ row.refund_no }}</span>
              <span class="price-sub">{{ row.payment_no }}</span>
            </div>
          </template>
          <template #amount="{ row }"><span class="price-main">¥{{ formatPrice(row.amount) }}</span></template>
          <template #status="{ row }">
            <t-tag :theme="refundStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ refundStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #empty><t-empty description="暂无退款单" /></template>
        </t-table>
      </section>

      <section class="table-card surface-card">
        <div class="table-card__head">
          <h3 class="card-title">打款单</h3>
          <span class="table-card__meta">近 {{ payoutList.length }} 笔</span>
        </div>
        <t-table row-key="id" :data="payoutList" :columns="payoutColumns" :loading="loading" size="small" hover cell-empty-content="—">
          <template #payout_no="{ row }">
            <div class="price-cell">
              <span class="cell-strong">{{ row.payout_no }}</span>
              <span class="price-sub">{{ payoutModeLabel(row.mode) }}</span>
            </div>
          </template>
          <template #amount="{ row }"><span class="price-main">¥{{ formatPrice(row.amount) }}</span></template>
          <template #status="{ row }">
            <t-tag :theme="payoutStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ payoutStatusLabel(row.status) }}
            </t-tag>
          </template>
          <template #empty><t-empty description="暂无打款单" /></template>
        </t-table>
      </section>
    </div>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近回调</h3>
        <span class="table-card__meta">近 10 条</span>
      </div>
      <t-table row-key="id" :data="recentCallbacks" :columns="callbackColumns" :loading="loading" size="small" hover cell-empty-content="—">
        <template #payment_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.payment_no || '—' }}</span>
            <span class="price-sub">{{ row.channel_code }}</span>
          </div>
        </template>
        <template #verify_ok="{ row }">
          <t-tag :theme="row.verify_ok ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.verify_ok ? '验签通过' : '验签失败' }}
          </t-tag>
        </template>
        <template #handle_status="{ row }">
          <t-tag :theme="callbackHandleTheme(row.handle_status)" variant="light" size="small" shape="round">
            {{ callbackHandleLabel(row.handle_status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty><t-empty description="暂无回调记录" /></template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { LinkIcon, MoneyIcon, RefreshIcon, TimeIcon, UploadIcon } from 'tdesign-icons-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getCallbacks, getChannelList, getPaymentOrders, getPayouts, getRefunds } from '@/api/payment'
import {
  bizTypeLabel,
  callbackHandleLabel,
  callbackHandleTheme,
  channelStatusLabel,
  channelStatusTheme,
  formatPrice,
  formatTime,
  healthLabel,
  healthTheme,
  modeLabel,
  modeTheme,
  orderStatusLabel,
  orderStatusTheme,
  payoutModeLabel,
  payoutStatusLabel,
  payoutStatusTheme,
  refundStatusLabel,
  refundStatusTheme,
} from '@/pages/payment/constants'
import type { CallbackInfo, ChannelInfo, PaymentOrderInfo, PayoutInfo, PaymentRefundInfo } from '@/types/interface'

defineOptions({ name: 'PaymentOverview' })

const loading = ref(false)
const channelList = ref<ChannelInfo[]>([])
const recentOrders = ref<PaymentOrderInfo[]>([])
const recentCallbacks = ref<CallbackInfo[]>([])
const refundList = ref<PaymentRefundInfo[]>([])
const payoutList = ref<PayoutInfo[]>([])

const stats = reactive({
  paidAmount: 0,
  payingCount: 0,
  enabledChannels: 0,
  totalChannels: 0,
  pendingPayouts: 0,
})

const channelColumns: PrimaryTableCol<ChannelInfo>[] = [
  { colKey: 'channel_code', title: '渠道', minWidth: 160 },
  { colKey: 'mode', title: '模式', width: 100 },
  { colKey: 'health_status', title: '健康', width: 90 },
  { colKey: 'status', title: '启用', width: 80, align: 'center' as const },
]

const orderColumns: PrimaryTableCol<PaymentOrderInfo>[] = [
  { colKey: 'payment_no', title: '支付单', minWidth: 170 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '时间', width: 150 },
]

const refundColumns: PrimaryTableCol<PaymentRefundInfo>[] = [
  { colKey: 'refund_no', title: '退款单', minWidth: 170 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
]

const payoutColumns: PrimaryTableCol<PayoutInfo>[] = [
  { colKey: 'payout_no', title: '打款单', minWidth: 170 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
]

const callbackColumns: PrimaryTableCol<CallbackInfo>[] = [
  { colKey: 'payment_no', title: '支付单 / 渠道', minWidth: 180 },
  { colKey: 'verify_ok', title: '验签', width: 100 },
  { colKey: 'handle_status', title: '处理', width: 100 },
  { colKey: 'created_at', title: '时间', width: 150 },
]

async function loadAll() {
  loading.value = true
  try {
    const [channels, orders, callbacks, refunds, payouts] = await Promise.all([
      getChannelList({ page: 1, page_size: 100 }),
      getPaymentOrders({ page: 1, page_size: 20 }),
      getCallbacks({ page: 1, page_size: 10 }),
      getRefunds({ page: 1, page_size: 10 }),
      getPayouts({ page: 1, page_size: 10 }),
    ])
    channelList.value = channels.items || []
    recentOrders.value = (orders.items || []).slice(0, 10)
    recentCallbacks.value = callbacks.items || []
    refundList.value = refunds.items || []
    payoutList.value = payouts.items || []

    stats.totalChannels = channels.meta.total
    stats.enabledChannels = channelList.value.filter((c) => c.status === 1).length
    stats.paidAmount = (orders.items || []).filter((o) => o.status === 'paid').reduce((sum, o) => sum + o.amount, 0)
    stats.payingCount = (orders.items || []).filter((o) => o.status === 'pending' || o.status === 'paying').length
    stats.pendingPayouts = payoutList.value.filter((p) => p.status === 'pending' || p.status === 'paying').length
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<style lang="css" scoped>
.stat-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.overview-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

@media (max-width: 1200px) {
  .stat-grid,
  .overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
