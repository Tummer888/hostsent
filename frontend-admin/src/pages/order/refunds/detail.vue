<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="20" aria-hidden="true" />
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
        <!-- 事后复核：免审核自动退款后，钱已出去，需财务回头核对一次 -->
        <t-button v-if="refund?.review_state === 'pending'" theme="primary" @click="reviewDialogVisible = true">
          <template #icon>
            <CheckCircleIcon aria-hidden="true" />
          </template>
          复核
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
        <template v-if="auditForm.action === 'approve'">
          <t-form-item label="退款凭据单号（选填）">
            <t-input
              v-model="auditForm.evidence_no"
              clearable
              placeholder="转账流水号 / 渠道退款单号，便于后期查账定位"
            />
          </t-form-item>
          <t-form-item label="凭据附件（选填）">
            <div class="evidence-upload">
              <t-space size="small" break-line>
                <t-tag v-for="f in pendingFiles" :key="f.id" closable variant="light" @close="removePendingFile(f)">
                  {{ f.file_name }}
                </t-tag>
                <t-button size="small" variant="outline" :loading="uploading" @click="triggerFilePick">
                  选择文件
                </t-button>
              </t-space>
              <input ref="fileInput" type="file" class="hidden-file-input" @change="handleFilePicked" />
              <p class="form-hint">支持图片 / PDF / 表格 / 压缩包，单个不超过 10MB，最多 10 个。</p>
            </div>
          </t-form-item>
        </template>
      </t-form>
    </t-dialog>

    <!-- 事后复核弹窗 -->
    <t-dialog
      v-model:visible="reviewDialogVisible"
      header="复核自动退款"
      width="480px"
      :confirm-btn="{ content: '确认复核', theme: 'primary', loading: reviewing }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleReview"
      @close="reviewDialogVisible = false"
    >
      <t-alert
        theme="info"
        :message="`该单为免审核自动退款（已退 ¥${formatPrice(refund?.amount || 0)} 至客户余额）。请核对资金与凭证后确认复核。`"
      />
      <t-form label-align="top" class="review-form" @submit.prevent>
        <t-form-item label="复核结论（选填）">
          <t-textarea
            v-model="reviewForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="如：已核对余额入账，凭证相符"
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
          <t-descriptions-item label="申请来源">
            <t-tag :theme="refund.origin === 'user' ? 'warning' : 'default'" variant="light" size="small" shape="round">
              {{ refund.origin === 'user' ? '客户自助申请' : '运营发起' }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="复核状态">
            <t-tag
              v-if="refund.review_state"
              :theme="refund.review_state === 'pending' ? 'warning' : 'success'"
              variant="light"
              size="small"
              shape="round"
            >
              {{ refund.review_state === 'pending' ? '待复核' : '已复核' }}
            </t-tag>
            <span v-else class="cell-muted">—</span>
          </t-descriptions-item>
          <t-descriptions-item label="审核人">{{ refund.audit_by_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="审核时间">{{ formatTime(refund.audited_at) }}</t-descriptions-item>
          <t-descriptions-item label="审核备注" :span="2">{{ refund.audit_remark || '—' }}</t-descriptions-item>
          <t-descriptions-item label="凭据单号" :span="2">{{ refund.evidence_no || '—' }}</t-descriptions-item>
          <t-descriptions-item label="复核人">{{ refund.reviewed_by_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="复核时间">{{ formatTime(refund.reviewed_at) }}</t-descriptions-item>
          <t-descriptions-item label="复核结论" :span="2">{{ refund.review_remark || '—' }}</t-descriptions-item>
          <t-descriptions-item label="创建时间">{{ formatTime(refund.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="更新时间">{{ formatTime(refund.updated_at) }}</t-descriptions-item>
        </t-descriptions>

        <!-- 审核凭据附件：人工退款打款的凭证材料，资金审计用 -->
        <div v-if="refund" class="desc-block evidence-block">
          <h3 class="card-title">审核凭据</h3>
          <div v-if="refund.attachments?.length" class="evidence-list">
            <t-link
              v-for="att in refund.attachments"
              :key="att.id"
              theme="primary"
              hover="color"
              @click="downloadAttachment(att)"
            >
              {{ att.file_name }}（{{ formatFileSize(att.file_size) }}）
            </t-link>
          </div>
          <span v-else class="cell-muted">暂无凭据附件</span>
        </div>
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

import {
  approveRefund,
  downloadRefundAttachment,
  getRefundDetail,
  rejectRefund,
  reviewRefund,
  uploadRefundAttachment,
} from '@/api/order'
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
import type { RefundAttachmentInfo, RefundInfo } from '@/types/interface'
import { formatFileSize } from '@/pages/ticket/constants'

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
  auditForm.evidence_no = ''
  pendingFiles.value = []
  auditDialogVisible.value = true
}

const auditDialogVisible = ref(false)
const auditForm = reactive<{ action: 'approve' | 'reject'; remark: string; evidence_no: string }>({
  action: 'approve',
  remark: '',
  evidence_no: '',
})

// 凭据附件：选择后即时上传拿到 ID，随审核提交时带 attachment_ids。
const pendingFiles = ref<RefundAttachmentInfo[]>([])
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

function triggerFilePick() {
  fileInput.value?.click()
}

async function handleFilePicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !refund.value) return
  if (file.size > 10 * 1024 * 1024) {
    MessagePlugin.warning('附件不能超过 10MB')
    return
  }
  uploading.value = true
  try {
    const info = await uploadRefundAttachment(refund.value.id, file)
    pendingFiles.value.push(info)
    MessagePlugin.success(`已上传：${info.file_name}`)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '附件上传失败')
  } finally {
    uploading.value = false
  }
}

function removePendingFile(target: RefundAttachmentInfo) {
  // 已上传的文件不做事后删除（凭据留存有利于审计），仅从本次提交中移除引用。
  pendingFiles.value = pendingFiles.value.filter((f) => f.id !== target.id)
}

// 附件下载：走鉴权端点（blob + JSON 信封辨别，与工单附件同口径）。
async function downloadAttachment(att: RefundAttachmentInfo) {
  try {
    await downloadRefundAttachment(att.id, att.file_name)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下载失败')
  }
}

// —— 事后复核 ——
const reviewDialogVisible = ref(false)
const reviewing = ref(false)
const reviewForm = reactive<{ remark: string }>({ remark: '' })

async function handleReview() {
  if (!refund.value) return
  reviewing.value = true
  try {
    await reviewRefund(refund.value.id, { remark: reviewForm.remark })
    MessagePlugin.success('复核完成')
    reviewDialogVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '复核失败')
  } finally {
    reviewing.value = false
  }
}

// 审核备注写入退款单（后端 RefundApproveRequest.remark 早已支持，此前前端恒传空对象）。
async function handleAudit() {
  const target = refund.value
  if (!target) return
  const isApprove = auditForm.action === 'approve'
  try {
    if (isApprove) {
      await approveRefund(target.id, {
        remark: auditForm.remark,
        evidence_no: auditForm.evidence_no,
        attachment_ids: pendingFiles.value.map((f) => f.id),
      })
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
