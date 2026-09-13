<template>
  <div class="page-body ticket-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServiceIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ ticket?.ticket_no || '工单详情' }}</h2>
          <p class="page-header__desc">{{ ticket?.title || '—' }} · 提交人 {{ ticket?.username || '—' }}</p>
        </div>
      </div>
      <t-space size="small">
        <t-tag v-if="ticket" :theme="ticketStatusTheme(ticket.status)" variant="light" size="medium" shape="round">
          {{ ticketStatusLabel(ticket.status) }}
        </t-tag>
        <t-tag v-if="ticket?.review_status === 'pending'" theme="warning" variant="light" size="medium" shape="round">
          待复核
        </t-tag>
        <t-tag v-if="ticket?.sla_breached" theme="danger" variant="light" size="medium" shape="round">SLA 超时</t-tag>
        <t-button variant="outline" :loading="loading" @click="loadDetail">刷新</t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">工单信息</h3>
        <t-space size="small">
          <t-button v-if="canClaim" variant="outline" size="small" theme="primary" @click="handleClaim">认领工单</t-button>
          <t-button v-if="canAssign" variant="outline" size="small" @click="openAssign">分配处理人</t-button>
          <t-button v-if="canTransfer" variant="outline" size="small" @click="openTransfer">转派</t-button>
          <t-popconfirm v-if="canClose" content="确认关闭该工单吗？" @confirm="handleClose">
            <t-button variant="outline" size="small" theme="danger">关闭工单</t-button>
          </t-popconfirm>
        </t-space>
      </div>
      <div class="tabs-section">
        <t-descriptions v-if="ticket" :column="3" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="状态">
            <t-tag :theme="ticketStatusTheme(ticket.status)" variant="light" size="small" shape="round">
              {{ ticketStatusLabel(ticket.status) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="分类">{{ ticket.category_name || ticket.category }}</t-descriptions-item>
          <t-descriptions-item label="优先级">
            <t-tag :theme="ticketPriorityTheme(ticket.priority)" variant="light" size="small" shape="round">
              {{ ticketPriorityLabel(ticket.priority) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="提交用户">{{ ticket.username }}（ID {{ ticket.user_id }}）</t-descriptions-item>
          <t-descriptions-item label="归属部门">{{ ticket.department_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="处理人">{{ ticket.assigned_name || '未分配' }}</t-descriptions-item>
          <t-descriptions-item label="提交时间">
            <span class="time-text">{{ formatTime(ticket.created_at) }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="首次响应">
            <span class="time-text">{{ formatTime(ticket.first_reply_at) }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="解决时间">
            <span class="time-text">{{ formatTime(ticket.resolved_at) }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="关闭时间">
            <span class="time-text">{{ formatTime(ticket.closed_at) }}</span>
          </t-descriptions-item>
          <t-descriptions-item v-if="ticket.review_status === 'pending'" label="复核状态">
            <t-tag theme="warning" variant="light" size="small" shape="round">待复核</t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="问题描述" :span="3">
            <div class="desc-text">{{ ticket.description || '—' }}</div>
          </t-descriptions-item>
          <t-descriptions-item label="工单附件" :span="3">
            <div v-if="ticketAttachments.length" class="attachment-list">
              <div v-for="file in ticketAttachments" :key="file.id" class="attachment-item">
                <t-link theme="primary" hover="color" @click="handleDownload(file)">
                  {{ file.file_name }}
                </t-link>
                <span class="attachment-item__meta">
                  {{ formatFileSize(file.file_size) }}
                  <t-tag v-if="file.is_internal" theme="warning" variant="light" size="small" shape="round">内部</t-tag>
                </span>
              </div>
            </div>
            <span v-else class="cell-muted">—</span>
          </t-descriptions-item>
        </t-descriptions>
        <t-empty v-else description="暂无数据" />
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">对话记录</h3>
        <span class="table-card__meta">共 {{ replies.length + 1 }} 条消息</span>
      </div>
      <div class="reply-stream">
        <!-- 工单原始描述作为第一条消息 -->
        <div v-if="ticket" class="reply-item user">
          <div class="reply-header">
            <span class="sender">{{ ticket.username }}</span>
            <span class="role-tag">用户</span>
            <span class="time">{{ formatTime(ticket.created_at) }}</span>
          </div>
          <div class="reply-content">{{ ticket.description || '—' }}</div>
        </div>
        <div v-for="reply in replies" :key="reply.id" :class="['reply-item', reply.sender_type, { 'is-internal': reply.is_internal }]">
          <div class="reply-header">
            <span class="sender">{{ reply.sender_name }}</span>
            <span class="role-tag" :class="reply.sender_type">{{ senderTypeLabel(reply.sender_type) }}</span>
            <t-tag v-if="reply.is_internal" theme="warning" variant="light" size="small" shape="round">内部备注</t-tag>
            <t-tag
              v-if="reply.review_status"
              :theme="reviewStatusTheme(reply.review_status)"
              variant="light"
              size="small"
              shape="round"
            >
              {{ reviewStatusLabel(reply.review_status) }}
            </t-tag>
            <span class="time">{{ formatTime(reply.created_at) }}</span>
          </div>
          <div class="reply-content">{{ reply.content }}</div>
          <div v-if="reply.attachments?.length" class="reply-attachments">
            <div v-for="file in reply.attachments" :key="file.id" class="attachment-item">
              <t-link theme="primary" hover="color" @click="handleDownload(file)">{{ file.file_name }}</t-link>
              <span class="attachment-item__meta">
                {{ formatFileSize(file.file_size) }}
                <t-tag v-if="file.is_internal" theme="warning" variant="light" size="small" shape="round">内部</t-tag>
              </span>
            </div>
          </div>
          <!-- 待复核回复：复核人可直接在此处置（S3） -->
          <div v-if="reply.review_status === 'pending'" class="review-inline">
            <div class="review-inline__head">
              <span class="review-inline__tip">
                该回复待复核{{ reply.reviewer_name ? `（复核人 ${reply.reviewer_name}）` : '' }}，复核通过后才对用户可见。
              </span>
              <t-space size="small">
                <t-button
                  theme="success"
                  variant="outline"
                  size="small"
                  :disabled="!canReviewReply(reply)"
                  @click="openReview(reply, 'approve')"
                >
                  通过
                </t-button>
                <t-button
                  theme="danger"
                  variant="outline"
                  size="small"
                  :disabled="!canReviewReply(reply)"
                  @click="openReview(reply, 'reject')"
                >
                  驳回
                </t-button>
              </t-space>
            </div>
            <p v-if="!canReviewReply(reply)" class="review-inline__self-tip">你本人提交的回复需由其他复核人处理。</p>
          </div>
          <!-- 已复核回复：展示复核结果（管理端可见，用户端不可见） -->
          <div v-else-if="reply.review_status" class="review-result">
            <span>{{ reviewStatusLabel(reply.review_status) }}</span>
            <span v-if="reply.reviewer_name"> · 复核人 {{ reply.reviewer_name }}</span>
            <span v-if="reply.reviewed_at"> · {{ formatTime(reply.reviewed_at) }}</span>
            <span v-if="reply.review_note"> · {{ reply.review_note }}</span>
          </div>
        </div>
        <t-empty v-if="replies.length === 0" description="暂无对话记录" />
      </div>

      <!-- 回复输入区 -->
      <div v-if="canReply" class="reply-input">
        <t-textarea
          v-model="replyContent"
          :placeholder="isInternalNote ? '内部备注仅客服团队可见，用户端不会展示…' : '输入回复内容，将发送给用户…'"
          :autosize="{ minRows: 3, maxRows: 6 }"
          :maxlength="2000"
        />
        <div v-if="pendingFiles.length" class="reply-input__files">
          <div v-for="(file, index) in pendingFiles" :key="file.id" class="attachment-item">
            <span class="attachment-item__name">{{ file.file_name }}</span>
            <span class="attachment-item__meta">{{ formatFileSize(file.file_size) }}</span>
            <t-link theme="danger" hover="color" @click="pendingFiles.splice(index, 1)">移除</t-link>
          </div>
        </div>
        <div class="reply-input__actions">
          <t-space size="small">
            <t-checkbox v-if="has('ticket:internal_note')" v-model="isInternalNote">
              内部备注（用户不可见）
            </t-checkbox>
            <input
              ref="fileInputRef"
              type="file"
              class="hidden-file-input"
              multiple
              @change="handleFilePicked"
            />
            <t-button
              variant="outline"
              size="small"
              :disabled="uploading || maxAttachmentReached"
              @click="triggerFilePick"
            >
              添加附件
            </t-button>
          </t-space>
          <t-space size="small">
            <t-button
              :theme="isInternalNote ? 'warning' : 'primary'"
              :loading="replying"
              :disabled="!replyContent.trim()"
              @click="handleReply"
            >
              {{ isInternalNote ? '保存内部备注' : '发送回复' }}
            </t-button>
          </t-space>
        </div>
        <p class="reply-input__hint">附件在发送时一并提交，单条回复最多 {{ MAX_ATTACHMENTS }} 个。</p>
      </div>
      <t-alert v-else theme="warning" message="工单已结束（已关闭/已取消），不可继续回复。" />
    </section>

    <!-- 操作日志时间线（P2-02） -->
    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">操作日志</h3>
        <span class="table-card__meta">共 {{ logs.length }} 条</span>
      </div>
      <ul v-if="logs.length" class="log-stream">
        <li v-for="log in logs" :key="log.id" class="log-item">
          <span class="log-item__dot" :class="`is-${log.action}`" aria-hidden="true"></span>
          <div class="log-item__body">
            <div class="log-item__head">
              <span class="log-item__action">{{ logActionLabel(log.action) }}</span>
              <span class="log-item__operator">{{ log.operator_name || '系统' }}</span>
              <span class="log-item__time">{{ formatTime(log.created_at) }}</span>
            </div>
            <div class="log-item__detail">
              <template v-if="log.from_value || log.to_value">{{ log.from_value || '—' }} → {{ log.to_value || '—' }}</template>
              <template v-if="log.note"> · {{ log.note }}</template>
            </div>
          </div>
        </li>
      </ul>
      <t-empty v-else description="暂无操作日志" />
    </section>

    <!-- 分配处理人对话框 -->
    <t-dialog
      v-model:visible="assignVisible"
      header="分配处理人"
      width="420px"
      :confirm-btn="{ content: '确认分配', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleAssign"
      @close="assignVisible = false"
    >
      <t-form label-align="top" :data="{}" @submit.prevent>
        <t-form-item label="处理人（管理员）" name="assigned_to">
          <t-select
            v-model="assignForm.assigned_to"
            clearable
            filterable
            placeholder="选择管理员"
            :options="adminOptions"
            :loading="adminLoading"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 转派对话框 -->
    <t-dialog
      v-model:visible="transferVisible"
      header="转派工单"
      width="420px"
      :confirm-btn="{ content: '确认转派', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleTransfer"
      @close="transferVisible = false"
    >
      <t-form label-align="top" :data="{}" @submit.prevent>
        <t-form-item label="转派给" name="to_id">
          <t-select
            v-model="transferForm.to_id"
            clearable
            filterable
            placeholder="选择接收员工"
            :options="adminOptions"
            :loading="adminLoading"
          />
        </t-form-item>
        <t-form-item label="转派说明" name="note">
          <t-textarea v-model="transferForm.note" placeholder="可选，说明转派原因" :autosize="{ minRows: 2, maxRows: 4 }" :maxlength="255" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 复核对话框（S3） -->
    <t-dialog
      v-model:visible="reviewVisible"
      :header="reviewForm.action === 'approve' ? '通过复核' : '驳回复核'"
      width="520px"
      :confirm-btn="{
        content: reviewForm.action === 'approve' ? '确认通过' : '确认驳回',
        theme: reviewForm.action === 'approve' ? 'success' : 'danger',
        loading: reviewing,
      }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleReviewSubmit"
      @close="reviewVisible = false"
    >
      <div v-if="reviewTarget" class="review-dialog__body">
        <p class="review-dialog__line">
          <span class="review-dialog__label">提交人</span>
          <span>{{ reviewTarget.sender_name || '—' }}</span>
        </p>
        <div class="review-dialog__quote">{{ reviewTarget.content }}</div>
        <t-form label-align="top" :data="reviewForm" @submit.prevent>
          <t-form-item :label="reviewForm.action === 'approve' ? '复核意见（选填）' : '驳回说明（必填）'">
            <t-textarea
              v-model="reviewForm.note"
              :placeholder="reviewForm.action === 'approve' ? '可补充说明，用户端不可见' : '请说明驳回原因，将通知提交人'"
              :autosize="{ minRows: 3, maxRows: 6 }"
              :maxlength="500"
              show-limit-number
            />
          </t-form-item>
        </t-form>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ServiceIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getAdminList } from '@/api/admin'
import type { AdminInfo } from '@/api/admin'
import {
  assignTicket,
  claimTicket,
  closeTicket,
  downloadTicketAttachment,
  getTicketDetail,
  replyTicket,
  reviewTicketReply,
  transferTicket,
  updateTicketStatus,
  uploadTicketAttachment,
} from '@/api/ticket'
import { usePermission } from '@/composables/usePermission'
import { useUserStore } from '@/store'
import {
  formatFileSize,
  formatTime,
  reviewStatusLabel,
  reviewStatusTheme,
  senderTypeLabel,
  ticketPriorityLabel,
  ticketPriorityTheme,
  ticketStatusLabel,
  ticketStatusTheme,
} from '@/pages/ticket/constants'
import type { TicketAttachmentInfo, TicketDetail, TicketLogInfo, TicketReplyInfo, TicketReviewRequest } from '@/types/interface'

defineOptions({ name: 'TicketDetail' })

const route = useRoute()
const router = useRouter()
const { has } = usePermission()
const userStore = useUserStore()

const ticket = ref<TicketDetail | null>(null)
const loading = ref(false)
const replies = computed<TicketReplyInfo[]>(() => ticket.value?.replies ?? [])
const logs = computed<TicketLogInfo[]>(() => ticket.value?.logs ?? [])
const ticketAttachments = computed<TicketAttachmentInfo[]>(() => ticket.value?.attachments ?? [])

const replyContent = ref('')
const replying = ref(false)
const isInternalNote = ref(false)

// 附件（S2）：先上传拿到 ID，发送回复时随 attachment_ids 提交
const MAX_ATTACHMENTS = 3
const fileInputRef = ref<HTMLInputElement | null>(null)
const pendingFiles = ref<TicketAttachmentInfo[]>([])
const uploading = ref(false)
const maxAttachmentReached = computed(() => pendingFiles.value.length >= MAX_ATTACHMENTS)

// 复核（S3）
const reviewVisible = ref(false)
const reviewing = ref(false)
const reviewTarget = ref<TicketReplyInfo | null>(null)
const reviewForm = reactive<{ action: 'approve' | 'reject'; note: string }>({ action: 'approve', note: '' })
const currentAdminID = computed(() => Number(userStore.userInfo?.id ?? 0))

const assignVisible = ref(false)
const transferVisible = ref(false)
const adminLoading = ref(false)
const adminOptions = ref<{ label: string; value: number }[]>([])
const assignForm = reactive<{ assigned_to: number | undefined }>({ assigned_to: undefined })
const transferForm = reactive<{ to_id: number | undefined; note: string }>({ to_id: undefined, note: '' })

const isFinal = computed(() => {
  const status = ticket.value?.status
  return status === 'closed' || status === 'cancelled'
})

// 非终态可回复；分配/转派需对应权限（后端仍会校验）
const canReply = computed(() => !isFinal.value && has('ticket:reply'))
const canAssign = computed(() => !isFinal.value && !!ticket.value?.assigned_to && has('ticket:assign'))
const canTransfer = computed(() => !isFinal.value && !!ticket.value?.assigned_to && has('ticket:assign'))
const canClaim = computed(() => !isFinal.value && !ticket.value?.assigned_to && has('ticket:list'))
const canClose = computed(() => !isFinal.value && has('ticket:close'))

// 复核人 ≠ 提交人（后端硬约束，前端提前禁用避免无效请求）
function canReviewReply(reply: TicketReplyInfo): boolean {
  if (!has('ticket:review')) return false
  if (reply.review_status !== 'pending') return false
  if (currentAdminID.value > 0 && reply.sender_id === currentAdminID.value) return false
  return true
}

const LOG_ACTION_LABELS: Record<string, string> = {
  create: '创建工单',
  assign: '分配',
  claim: '认领',
  transfer: '转派',
  reply: '回复',
  internal_note: '内部备注',
  review_request: '提交待复核',
  review: '复核',
  status: '状态变更',
  close: '关闭',
  cancel: '取消',
}

function logActionLabel(action: string): string {
  return LOG_ACTION_LABELS[action] || action
}

async function loadDetail() {
  const id = Number(route.params.id)
  if (!Number.isFinite(id)) return
  loading.value = true
  try {
    ticket.value = await getTicketDetail(id)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载工单详情失败')
  } finally {
    loading.value = false
  }
}

// 管理员回复：普通回复 open 状态自动推进为 in_progress；内部备注不改变状态（后端处理）
async function handleReply() {
  const content = replyContent.value.trim()
  if (!content || !ticket.value) return
  replying.value = true
  try {
    ticket.value = await replyTicket(ticket.value.id, {
      content,
      is_internal: isInternalNote.value,
      attachment_ids: pendingFiles.value.map((file) => file.id),
    })
    replyContent.value = ''
    isInternalNote.value = false
    pendingFiles.value = []
    MessagePlugin.success('已提交')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '提交失败')
  } finally {
    replying.value = false
  }
}

// —— 附件上传 ——
function triggerFilePick() {
  fileInputRef.value?.click()
}

async function handleFilePicked(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (!files.length || !ticket.value) return
  const remaining = MAX_ATTACHMENTS - pendingFiles.value.length
  uploading.value = true
  try {
    for (const file of files.slice(0, remaining)) {
      const uploaded = await uploadTicketAttachment(ticket.value.id, file, isInternalNote.value)
      pendingFiles.value.push(uploaded)
    }
    if (files.length > remaining) {
      MessagePlugin.warning(`单条回复最多 ${MAX_ATTACHMENTS} 个附件，超出部分已忽略`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '附件上传失败')
  } finally {
    uploading.value = false
  }
}

async function handleDownload(file: TicketAttachmentInfo) {
  try {
    await downloadTicketAttachment(file.id, file.file_name)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '附件下载失败')
  }
}

// —— 复核 ——
function openReview(reply: TicketReplyInfo, action: 'approve' | 'reject') {
  if (!canReviewReply(reply)) {
    MessagePlugin.warning('不能复核自己提交的回复')
    return
  }
  reviewTarget.value = reply
  reviewForm.action = action
  reviewForm.note = ''
  reviewVisible.value = true
}

async function handleReviewSubmit() {
  if (!reviewTarget.value) return
  if (reviewForm.action === 'reject' && !reviewForm.note.trim()) {
    MessagePlugin.warning('驳回需填写说明')
    return
  }
  reviewing.value = true
  try {
    const payload: TicketReviewRequest = {
      action: reviewForm.action,
      note: reviewForm.note.trim() || undefined,
    }
    ticket.value = await reviewTicketReply(reviewTarget.value.id, payload)
    reviewVisible.value = false
    MessagePlugin.success(reviewForm.action === 'approve' ? '复核已通过，回复已对用户可见' : '已驳回')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '复核操作失败')
  } finally {
    reviewing.value = false
  }
}

function openAssign() {
  assignForm.assigned_to = ticket.value?.assigned_to || undefined
  assignVisible.value = true
  loadAdmins()
}

// 加载管理员列表供分配选择
async function loadAdmins() {
  if (adminOptions.value.length > 0) return
  adminLoading.value = true
  try {
    const data = await getAdminList({ page: 1, page_size: 100, status: 'active' })
    adminOptions.value = data.items.map((item: AdminInfo) => ({ label: item.username, value: item.id }))
  } catch {
    // 管理员列表加载失败时提示
    MessagePlugin.error('管理员列表加载失败')
  } finally {
    adminLoading.value = false
  }
}

async function handleAssign() {
  if (!ticket.value) return
  if (!assignForm.assigned_to) {
    MessagePlugin.warning('请选择处理人')
    return
  }
  try {
    ticket.value = await assignTicket(ticket.value.id, { assigned_to: assignForm.assigned_to })
    assignVisible.value = false
    MessagePlugin.success('分配成功')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '分配失败')
  }
}

function openTransfer() {
  transferForm.to_id = undefined
  transferForm.note = ''
  transferVisible.value = true
  loadAdmins()
}

async function handleTransfer() {
  if (!ticket.value) return
  if (!transferForm.to_id) {
    MessagePlugin.warning('请选择转派对象')
    return
  }
  try {
    ticket.value = await transferTicket(ticket.value.id, { to_id: transferForm.to_id, note: transferForm.note })
    transferVisible.value = false
    MessagePlugin.success('转派成功')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '转派失败')
  }
}

async function handleClaim() {
  if (!ticket.value) return
  try {
    ticket.value = await claimTicket(ticket.value.id)
    MessagePlugin.success('认领成功')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '认领失败')
  }
}

// 待响应工单需先转为处理中再关闭（状态机约束）
async function handleClose() {
  if (!ticket.value) return
  try {
    if (ticket.value.status === 'open') {
      ticket.value = await updateTicketStatus(ticket.value.id, { status: 'in_progress' })
    }
    ticket.value = await closeTicket(ticket.value.id)
    MessagePlugin.success('工单已关闭')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '关闭失败')
  }
}

function goBack() {
  router.push('/tickets/list')
}

onMounted(loadDetail)
</script>

<style lang="css">
@import './shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}

.desc-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 13px;
  color: #334155;
  line-height: 1.7;
}

/* ---------- 对话消息流 ---------- */
.reply-stream {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 520px;
  overflow-y: auto;
  padding: 4px 2px;
}

.reply-item {
  max-width: 78%;
  padding: 10px 14px;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: var(--hs-surface-2);
}

/* 用户消息靠左，管理员消息靠右对齐形成会话方向感 */
.reply-item.admin {
  margin-left: auto;
  background: #eef6ff;
  border-color: #cfe3fb;
}

.reply-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.reply-header .sender {
  font-size: 12px;
  font-weight: 600;
  color: #334155;
}

.reply-header .time {
  font-size: 12px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
  margin-left: auto;
}

.role-tag {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 999px;
  background: #e2e8f0;
  color: #475569;
}

.role-tag.admin {
  background: #dbeafe;
  color: #1d4ed8;
}

.reply-content {
  font-size: 13px;
  line-height: 1.7;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-all;
}

/* 内部备注（S2）：与用户可见回复做视觉区分 */
.reply-item.is-internal {
  background: #fffbeb;
  border-color: #fde68a;
}

.reply-attachments {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.attachment-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.attachment-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.attachment-item__name {
  color: var(--color-foreground);
}

.attachment-item__meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* 复核区块（S3） */
.review-inline {
  margin-top: 8px;
  border-top: 1px dashed #fcd34d;
  padding-top: 8px;
}

.review-inline__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.review-inline__tip {
  font-size: 12px;
  color: #92400e;
}

.review-inline__self-tip {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.review-result {
  margin-top: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.review-dialog__body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.review-dialog__line {
  margin: 0;
  font-size: 13px;
  color: var(--color-foreground);
  display: flex;
  gap: 8px;
}

.review-dialog__label {
  color: var(--color-muted-foreground);
  flex: none;
  width: 48px;
}

.review-dialog__quote {
  font-size: 13px;
  line-height: 1.7;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--hs-surface-2);
  border-radius: var(--hs-radius-md);
  padding: 10px 12px;
  max-height: 160px;
  overflow-y: auto;
}

.reply-input {
  margin-top: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reply-input__files {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.reply-input__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.reply-input__hint {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.hidden-file-input {
  display: none;
}

/* ---------- 操作日志时间线 ---------- */
.log-stream {
  list-style: none;
  margin: 0;
  padding: 4px 2px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 360px;
  overflow-y: auto;
}

.log-item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.log-item__dot {
  flex: none;
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  background: #94a3b8;
}

.log-item__dot.is-create {
  background: #16a34a;
}
.log-item__dot.is-assign,
.log-item__dot.is-claim,
.log-item__dot.is-transfer {
  background: #2563eb;
}
.log-item__dot.is-close,
.log-item__dot.is-cancel {
  background: #dc2626;
}

.log-item__body {
  flex: 1;
  min-width: 0;
}

.log-item__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-item__action {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.log-item__operator {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.log-item__time {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.log-item__detail {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-muted-foreground);
  word-break: break-all;
}
</style>
