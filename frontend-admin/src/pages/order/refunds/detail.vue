<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ refund?.refund_no || '退款详情' }}</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadDetail">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <t-button v-if="refund?.status === 'pending'" theme="primary" @click="openAuditDialog('approve')">通过</t-button>
        <t-button v-if="refund?.status === 'pending'" theme="danger" @click="openAuditDialog('reject')">驳回</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="tabs-section">
        <t-descriptions v-if="refund" :column="2" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="退款单号">{{ refund.refund_no }}</t-descriptions-item>
          <t-descriptions-item label="关联订单">
            <t-link theme="primary" hover="color" @click="openOrder">{{ refund.order_no || `#${refund.order_id}` }}</t-link>
          </t-descriptions-item>
          <t-descriptions-item label="用户 ID">{{ refund.user_id }}</t-descriptions-item>
          <t-descriptions-item label="退款去向">
            <t-tag :theme="refundModeTheme(refund.refund_mode)" variant="light" size="small" shape="round">
              {{ refundModeLabel(refund.refund_mode) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="退款金额">
            <span class="price-main">¥{{ formatPrice(refund.amount) }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="渠道扣点">
            <span class="price-sub">{{ refund.refund_mode === 'channel' ? `¥${formatPrice(refund.fee_amount)}` : '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="财务净额">
            <span class="price-sub">
              {{ refund.refund_mode === 'channel' ? `¥${formatPrice(refund.net_amount)}` : '消费口径不变' }}
            </span>
          </t-descriptions-item>
          <t-descriptions-item label="渠道退款单">
            <div class="price-cell">
              <span class="price-sub">{{ refund.channel_refund_no || '—' }}</span>
              <t-tag
                v-if="refund.refund_mode === 'channel'"
                :theme="channelRefundStatusTheme(refund.channel_refund_status)"
                variant="light"
                size="small"
                shape="round"
              >
                {{ channelRefundStatusLabel(refund.channel_refund_status) }}
              </t-tag>
            </div>
          </t-descriptions-item>
          <t-descriptions-item label="退款原因">{{ refund.reason || '—' }}</t-descriptions-item>
          <t-descriptions-item label="状态">
            <t-tag :theme="refundStatusTheme(refund.status)" variant="light" size="small" shape="round">
              {{ refundStatusLabel(refund.status) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="审核人">{{ refund.audit_by_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="审核时间">{{ formatTime(refund.audited_at) }}</t-descriptions-item>
          <t-descriptions-item label="创建时间">{{ formatTime(refund.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="更新时间">{{ formatTime(refund.updated_at) }}</t-descriptions-item>
        </t-descriptions>
        <t-empty v-else description="暂无数据" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { MoneyIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'

import { approveRefund, getRefundDetail, rejectRefund } from '@/api/order'
import {
  channelRefundStatusLabel,
  channelRefundStatusTheme,
  formatPrice,
  formatTime,
  refundModeLabel,
  refundModeTheme,
  refundStatusLabel,
  refundStatusTheme,
} from '@/pages/order/constants'
import type { RefundInfo } from '@/types/interface'

defineOptions({ name: 'OrderRefundDetail' })

const route = useRoute()
const router = useRouter()

const refund = ref<RefundInfo | null>(null)
const loading = ref(false)

async function loadDetail() {
  const id = Number(route.params.id)
  loading.value = true
  try {
    refund.value = await getRefundDetail(id)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载退款详情失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/orders/refunds')
}

function openOrder() {
  if (refund.value) {
    router.push(`/orders/detail/${refund.value.order_id}`)
  }
}

function openAuditDialog(action: 'approve' | 'reject') {
  const target = refund.value
  if (!target) return
  const isApprove = action === 'approve'
  const dialog = DialogPlugin.confirm({
    header: isApprove ? '通过退款' : '驳回退款',
    body: isApprove
      ? `确认通过退款单「${target.refund_no}」，退款金额 ¥${formatPrice(target.amount)} 吗？`
      : `确认驳回退款单「${target.refund_no}」吗？`,
    confirmBtn: { content: isApprove ? '确认通过' : '确认驳回', theme: isApprove ? 'success' : 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        if (isApprove) {
          await approveRefund(target.id, {})
        } else {
          await rejectRefund(target.id, {})
        }
        MessagePlugin.success(isApprove ? '退款已通过' : '退款已驳回')
        dialog.destroy()
        loadDetail()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '操作失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadDetail)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}
</style>
