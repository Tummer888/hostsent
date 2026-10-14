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
        <t-button v-if="refund?.status === 'pending'" theme="primary" @click="openAuditDialog('approve')">
          <template #icon>
            <CheckCircleIcon aria-hidden="true" />
          </template>
          通过
        </t-button>
        <t-button
          v-if="refund?.status === 'pending'"
          variant="outline"
          theme="danger"
          @click="openAuditDialog('reject')"
        >
          <template #icon>
            <CloseCircleIcon aria-hidden="true" />
          </template>
          驳回
        </t-button>
      </t-space>
    </header>

    <!-- 审核弹窗：备注会写入退款单留存（此前恒传空对象，审核依据无处记录） -->
    <t-dialog
      v-model:visible="auditDialogVisible"
      :header="auditForm.action === 'approve' ? '通过退款' : '驳回退款'"
      width="480px"
      :confirm-btn="{
        content: auditForm.action === 'approve' ? '确认通过' : '确认驳回',
        theme: auditForm.action === 'approve' ? 'primary' : 'danger',
      }"
      :cancel-btn="{ content: '再想想' }"
      @confirm="handleAudit"
      @close="auditDialogVisible = false"
    >
      <t-form label-align="top" :data="auditForm" @submit.prevent>
        <t-form-item v-if="refund" label="退款单">
          <span class="cell-strong">{{ refund.refund_no }} · ¥{{ formatPrice(refund.amount) }}</span>
        </t-form-item>
        <t-form-item label="审核备注（选填）" name="remark">
          <t-textarea
            v-model="auditForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            :placeholder="
              auditForm.action === 'approve' ? '如：已与客户确认，同意退款' : '如：不符合退款条件，活动商品不退'
            "
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <section class="table-card surface-card">
      <div class="tabs-section">
        <t-descriptions v-if="refund" :column="2" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="退款单号">{{ refund.refund_no }}</t-descriptions-item>
          <t-descriptions-item label="关联订单">
            <t-link theme="primary" hover="color" @click="openOrder">{{
              refund.order_no || `#${refund.order_id}`
            }}</t-link>
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
            <span class="price-sub">{{
              refund.refund_mode === 'channel' ? `¥${formatPrice(refund.fee_amount)}` : '—'
            }}</span>
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
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { CheckCircleIcon, CloseCircleIcon, MoneyIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

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
  auditForm.action = action
  auditForm.remark = ''
  auditDialogVisible.value = true
}

const auditDialogVisible = ref(false)
const auditForm = reactive<{ action: 'approve' | 'reject'; remark: string }>({ action: 'approve', remark: '' })

// 审核备注写入退款单（后端 RefundApproveRequest.remark 早已支持，此前前端恒传空对象）。
async function handleAudit() {
  const target = refund.value
  if (!target) return
  const isApprove = auditForm.action === 'approve'
  try {
    if (isApprove) {
      await approveRefund(target.id, { remark: auditForm.remark })
    } else {
      await rejectRefund(target.id, { remark: auditForm.remark })
    }
    MessagePlugin.success(isApprove ? '退款已通过' : '退款已驳回')
    auditDialogVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '操作失败')
  }
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
