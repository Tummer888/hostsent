<template>
  <div class="ticket-detail-page">
    <!-- 页头 -->
    <section class="detail-hero">
      <t-button variant="text" shape="square" aria-label="返回" class="back-btn" @click="goBack">
        <template #icon><ChevronLeftIcon /></template>
      </t-button>
      <div class="hero-info">
        <div class="hero-title-row">
          <span class="hero-title">{{ ticket?.ticket_no || '工单详情' }}</span>
          <t-tag v-if="ticket" :theme="ticketStatusTheme(ticket.status)" variant="light" shape="round">
            {{ ticketStatusLabel(ticket.status) }}
          </t-tag>
        </div>
        <span class="hero-desc">{{ ticket?.title || '加载中…' }}</span>
      </div>
      <t-popconfirm
        v-if="ticket && ticket.status === 'open'"
        content="确定取消该工单吗？"
        theme="danger"
        @confirm="handleCancel"
      >
        <t-button theme="danger" variant="outline">取消工单</t-button>
      </t-popconfirm>
    </section>

    <t-loading :loading="loading" show-overlay>
      <!-- 工单信息 -->
      <section class="info-panel">
        <span class="panel-title">工单信息</span>
        <t-descriptions :column="3" layout="horizontal" bordered size="medium">
          <t-descriptions-item label="问题分类">{{ ticket?.category_name || ticket?.category || '—' }}</t-descriptions-item>
          <t-descriptions-item label="优先级">
            <t-tag v-if="ticket" :theme="ticketPriorityTheme(ticket.priority)" variant="light" size="small" shape="round">
              {{ ticketPriorityLabel(ticket.priority) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="处理人">{{ ticket?.assigned_name || '暂未分配' }}</t-descriptions-item>
          <t-descriptions-item label="提交时间">{{ formatTime(ticket?.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="首次响应">{{ formatTime(ticket?.first_reply_at) }}</t-descriptions-item>
          <t-descriptions-item label="解决时间">{{ formatTime(ticket?.resolved_at) }}</t-descriptions-item>
          <t-descriptions-item label="问题描述" :span="3" class="desc-item">
            <span class="problem-desc">{{ ticket?.description || '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item v-if="ticketAttachments.length" label="相关附件" :span="3">
            <div class="attachment-list">
              <div v-for="file in ticketAttachments" :key="file.id" class="attachment-item">
                <t-link theme="primary" hover="color" @click="handleDownload(file)">{{ file.file_name }}</t-link>
                <span class="attachment-item__meta">{{ formatFileSize(file.file_size) }}</span>
              </div>
            </div>
          </t-descriptions-item>
        </t-descriptions>
      </section>

      <!-- 对话消息流 -->
      <section class="reply-panel">
        <span class="panel-title">沟通记录（{{ replies.length }}）</span>
        <div v-if="replies.length > 0" class="reply-stream">
          <div
            v-for="reply in replies"
            :key="reply.id"
            class="reply-item"
            :class="{ self: reply.sender_type === 'user' }"
          >
            <div class="reply-avatar">{{ reply.sender_name?.slice(0, 1).toUpperCase() || '?' }}</div>
            <div class="reply-bubble">
              <div class="reply-meta">
                <span class="reply-name">{{ reply.sender_type === 'user' ? '我' : `${reply.sender_name}（客服）` }}</span>
                <span class="reply-time">{{ formatTime(reply.created_at) }}</span>
              </div>
              <div class="reply-content">{{ reply.content }}</div>
              <div v-if="reply.attachments?.length" class="attachment-list attachment-list--inline">
                <div v-for="file in reply.attachments" :key="file.id" class="attachment-item">
                  <t-link theme="primary" hover="color" @click="handleDownload(file)">{{ file.file_name }}</t-link>
                  <span class="attachment-item__meta">{{ formatFileSize(file.file_size) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <t-empty v-else description="暂无沟通记录" />

        <!-- 回复输入区（终态工单不可回复） -->
        <div v-if="canReply" class="reply-editor">
          <t-textarea
            v-model="replyContent"
            placeholder="请输入回复内容…"
            :maxlength="2000"
            :autosize="{ minRows: 3, maxRows: 8 }"
            show-limit-number
          />
          <div v-if="pendingFiles.length" class="attachment-list">
            <div v-for="(file, index) in pendingFiles" :key="file.id" class="attachment-item">
              <span class="attachment-item__name">{{ file.file_name }}</span>
              <span class="attachment-item__meta">{{ formatFileSize(file.file_size) }}</span>
              <t-link theme="danger" hover="color" @click="pendingFiles.splice(index, 1)">移除</t-link>
            </div>
          </div>
          <div class="editor-actions">
            <input ref="fileInputRef" type="file" class="hidden-file-input" multiple @change="handleFilePicked" />
            <t-space size="small">
              <t-button variant="outline" :disabled="uploading || maxAttachmentReached" @click="triggerFilePick">
                {{ uploading ? '上传中…' : '添加附件' }}
              </t-button>
              <t-button theme="primary" :loading="replying" :disabled="!replyContent.trim()" @click="handleReply">发送回复</t-button>
            </t-space>
          </div>
          <p class="attachment-tip">
            支持图片、文本与压缩包，单个不超过 10MB，最多 {{ MAX_ATTACHMENTS }} 个。
          </p>
        </div>
        <div v-else class="reply-closed-tip">工单已结束，如需继续处理请重新提交工单。</div>
      </section>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ChevronLeftIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  cancelMyTicket,
  downloadMyTicketAttachment,
  getMyTicketDetail,
  replyMyTicket,
  uploadMyTicketAttachment,
  type TicketAttachmentInfo,
  type TicketDetail,
  type TicketReplyInfo,
} from '@/api/support'
import { formatTime, ticketPriorityLabel, ticketPriorityTheme, ticketStatusLabel, ticketStatusTheme } from '@/pages/support/constants'

defineOptions({ name: 'UserTicketDetail' })

const route = useRoute()
const router = useRouter()

const ticket = ref<TicketDetail | null>(null)
const loading = ref(false)
const replies = computed<TicketReplyInfo[]>(() => ticket.value?.replies ?? [])
const ticketAttachments = computed<TicketAttachmentInfo[]>(() => ticket.value?.attachments ?? [])

// 终态（已关闭/已取消）不可再回复
const isFinalStatus = (status?: string) => status === 'closed' || status === 'cancelled'
const canReply = computed(() => !!ticket.value && !isFinalStatus(ticket.value.status))

const replyContent = ref('')
const replying = ref(false)

// 附件：先上传（ticket_id 已知，走归属校验），发送回复时携带 ID
const MAX_ATTACHMENTS = 3
const fileInputRef = ref<HTMLInputElement | null>(null)
const pendingFiles = ref<TicketAttachmentInfo[]>([])
const uploading = ref(false)
const maxAttachmentReached = computed(() => pendingFiles.value.length >= MAX_ATTACHMENTS)

const ticketId = computed(() => String(route.params.id || ''))

// 加载工单详情（含回复记录）
async function loadDetail() {
  if (!ticketId.value) return
  loading.value = true
  try {
    const { data } = await getMyTicketDetail(ticketId.value)
    if (data) ticket.value = data
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载工单详情失败')
  } finally {
    loading.value = false
  }
}

// 发送回复：成功后用后端返回的最新详情刷新对话流
async function handleReply() {
  const content = replyContent.value.trim()
  if (!content) return
  replying.value = true
  try {
    const { data } = await replyMyTicket(ticketId.value, content, pendingFiles.value.map((file) => file.id))
    if (data) ticket.value = data
    replyContent.value = ''
    pendingFiles.value = []
    MessagePlugin.success('回复成功')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '回复失败，请稍后重试')
  } finally {
    replying.value = false
  }
}

function triggerFilePick() {
  fileInputRef.value?.click()
}

async function handleFilePicked(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (!files.length) return
  const remaining = MAX_ATTACHMENTS - pendingFiles.value.length
  uploading.value = true
  try {
    for (const file of files.slice(0, remaining)) {
      const { data } = await uploadMyTicketAttachment(ticketId.value, file)
      if (data) pendingFiles.value.push(data)
    }
    if (files.length > remaining) {
      MessagePlugin.warning(`单条回复最多 ${MAX_ATTACHMENTS} 个附件，超出部分已忽略`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '附件上传失败')
  } finally {
    uploading.value = false
  }
}

async function handleDownload(file: TicketAttachmentInfo) {
  try {
    await downloadMyTicketAttachment(file.id, file.file_name)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '附件下载失败')
  }
}

// 取消工单（仅待处理 open 状态可取消）
async function handleCancel() {
  try {
    const { data } = await cancelMyTicket(ticketId.value)
    if (data) ticket.value = data
    MessagePlugin.success('工单已取消')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '取消失败')
  }
}

function goBack() {
  router.back()
}

// 文件大小 → 人类可读文案
function formatFileSize(bytes: number): string {
  const size = Number(bytes || 0)
  if (size <= 0) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(2)} MB`
}

onMounted(loadDetail)
</script>

<style scoped>
.ticket-detail-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 960px;
}

/* 页头 */
.detail-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 20px 28px;
  color: #fff;
  display: flex;
  align-items: center;
  gap: 12px;
}

.back-btn {
  color: #fff;
}

.hero-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.hero-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hero-title {
  font-size: 18px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.hero-desc {
  font-size: 13px;
  opacity: 0.85;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 面板通用 */
.info-panel,
.reply-panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.problem-desc {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.7;
}

/* 对话消息流 */
.reply-stream {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.reply-item {
  display: flex;
  gap: 12px;
  max-width: 80%;
}

/* 自己（用户）的消息靠右 */
.reply-item.self {
  margin-left: auto;
  flex-direction: row-reverse;
}

.reply-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #94a3b8, #64748b);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  flex-shrink: 0;
}

.reply-item.self .reply-avatar {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
}

.reply-bubble {
  background: #f1f5f9;
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.reply-item.self .reply-bubble {
  background: #eff6ff;
}

.reply-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: #64748b;
}

.reply-name {
  font-weight: 600;
  color: #334155;
}

.reply-content {
  font-size: 14px;
  color: #1e293b;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
}

/* 附件 */
.attachment-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.attachment-list--inline {
  margin-top: 4px;
  padding-top: 6px;
  border-top: 1px dashed #e2e8f0;
}

.attachment-item {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.attachment-item__name {
  color: #1e293b;
}

.attachment-item__meta {
  font-size: 12px;
  color: #94a3b8;
}

/* 回复输入区 */
.reply-editor {
  border-top: 1px solid #f1f5f9;
  padding-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.editor-actions {
  display: flex;
  justify-content: flex-end;
}

.attachment-tip {
  margin: 0;
  font-size: 12px;
  color: #94a3b8;
}

.hidden-file-input {
  display: none;
}

.reply-closed-tip {
  border-top: 1px solid #f1f5f9;
  padding-top: 16px;
  text-align: center;
  color: #94a3b8;
  font-size: 13px;
}

@media (max-width: 768px) {
  .reply-item {
    max-width: 95%;
  }
}
</style>
