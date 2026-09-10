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
          <t-descriptions-item label="问题描述" :span="3">
            <div class="desc-text">{{ ticket.description || '—' }}</div>
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
        <div v-for="reply in replies" :key="reply.id" :class="['reply-item', reply.sender_type]">
          <div class="reply-header">
            <span class="sender">{{ reply.sender_name }}</span>
            <span class="role-tag" :class="reply.sender_type">{{ senderTypeLabel(reply.sender_type) }}</span>
            <span class="time">{{ formatTime(reply.created_at) }}</span>
          </div>
          <div class="reply-content">{{ reply.content }}</div>
        </div>
        <t-empty v-if="replies.length === 0" description="暂无对话记录" />
      </div>

      <!-- 回复输入区 -->
      <div v-if="canReply" class="reply-input">
        <t-textarea
          v-model="replyContent"
          placeholder="输入回复内容..."
          :autosize="{ minRows: 3, maxRows: 6 }"
          :maxlength="2000"
        />
        <div class="reply-input__actions">
          <t-space size="small">
            <t-button theme="primary" :loading="replying" :disabled="!replyContent.trim()" @click="handleReply">
              发送回复
            </t-button>
          </t-space>
        </div>
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ServiceIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getAdminList } from '@/api/admin'
import type { AdminInfo } from '@/api/admin'
import { assignTicket, claimTicket, closeTicket, getTicketDetail, replyTicket, transferTicket, updateTicketStatus } from '@/api/ticket'
import { usePermission } from '@/composables/usePermission'
import {
  formatTime,
  senderTypeLabel,
  ticketPriorityLabel,
  ticketPriorityTheme,
  ticketStatusLabel,
  ticketStatusTheme,
} from '@/pages/ticket/constants'
import type { TicketDetail, TicketLogInfo, TicketReplyInfo } from '@/types/interface'

defineOptions({ name: 'TicketDetail' })

const route = useRoute()
const router = useRouter()
const { has } = usePermission()

const ticket = ref<TicketDetail | null>(null)
const loading = ref(false)
const replies = computed<TicketReplyInfo[]>(() => ticket.value?.replies ?? [])
const logs = computed<TicketLogInfo[]>(() => ticket.value?.logs ?? [])

const replyContent = ref('')
const replying = ref(false)

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

const LOG_ACTION_LABELS: Record<string, string> = {
  create: '创建工单',
  assign: '分配',
  claim: '认领',
  transfer: '转派',
  reply: '回复',
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

// 管理员回复：open 状态自动推进为 in_progress（后端状态机处理）
async function handleReply() {
  const content = replyContent.value.trim()
  if (!content || !ticket.value) return
  replying.value = true
  try {
    ticket.value = await replyTicket(ticket.value.id, { content })
    replyContent.value = ''
    MessagePlugin.success('回复成功')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '回复失败')
  } finally {
    replying.value = false
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

.reply-input {
  margin-top: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reply-input__actions {
  display: flex;
  justify-content: flex-end;
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
