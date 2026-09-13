<template>
  <div class="page-body ticket-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">复核中心</h2>
          <p class="page-header__desc">待复核回复按提交时间由早到晚排列，复核人不可复核自己提交的回复。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadQueue">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">待复核队列</h3>
        <span class="table-card__meta">共 {{ total }} 条待处理</span>
      </div>

      <t-loading :loading="loading" show-overlay>
        <div v-if="items.length" class="review-list">
          <article v-for="item in items" :key="item.reply_id" class="review-card">
            <div class="review-card__head">
              <div class="review-card__ticket">
                <t-link theme="primary" hover="color" @click="openTicket(item)">
                  {{ item.ticket_no }}
                </t-link>
                <span class="review-card__title">{{ item.title }}</span>
              </div>
              <t-space size="small" break-line>
                <t-tag variant="light" size="small" shape="round">{{ item.category_name || item.category || '未分类' }}</t-tag>
                <t-tag v-if="item.department_name" variant="light" size="small" shape="round" theme="primary">
                  {{ item.department_name }}
                </t-tag>
              </t-space>
            </div>

            <div class="review-card__meta">
              <span class="review-card__sender">
                <strong>{{ item.sender_name || '—' }}</strong> 提交待复核
              </span>
              <span class="review-card__time">{{ formatTime(item.created_at) }}</span>
              <t-tag theme="warning" variant="light" size="small" shape="round">
                已等待 {{ formatWait(item.waiting_seconds) }}
              </t-tag>
            </div>

            <div class="review-card__content">{{ item.content }}</div>

            <div class="review-card__actions">
              <t-button variant="outline" size="small" @click="openTicket(item)">查看工单</t-button>
              <t-button
                theme="success"
                variant="outline"
                size="small"
                :disabled="item.sender_id === currentAdminId"
                @click="openReview(item, 'approve')"
              >
                通过
              </t-button>
              <t-button
                theme="danger"
                variant="outline"
                size="small"
                :disabled="item.sender_id === currentAdminId"
                @click="openReview(item, 'reject')"
              >
                驳回
              </t-button>
            </div>
            <p v-if="item.sender_id === currentAdminId" class="review-card__self-tip">
              该回复由你本人提交，需由其他复核人处理。
            </p>
          </article>
        </div>
        <t-empty v-else description="暂无待复核回复" />
      </t-loading>

      <t-pagination
        v-if="total > 0"
        class="review-pagination"
        :current="pagination.current"
        :page-size="pagination.pageSize"
        :total="total"
        show-jumper
        @current-change="handlePageChange"
        @page-size-change="handlePageSizeChange"
      />
    </section>

    <!-- 复核意见对话框 -->
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
      <div v-if="activeItem" class="review-dialog__body">
        <p class="review-dialog__line">
          <span class="review-dialog__label">工单</span>
          <span>{{ activeItem.ticket_no }} · {{ activeItem.title }}</span>
        </p>
        <p class="review-dialog__line">
          <span class="review-dialog__label">提交人</span>
          <span>{{ activeItem.sender_name || '—' }}</span>
        </p>
        <div class="review-dialog__quote">{{ activeItem.content }}</div>
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
import { useRouter } from 'vue-router'
import { RefreshIcon, VerifyIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getTicketReviews, reviewTicketReply } from '@/api/ticket'
import { useUserStore } from '@/store'
import { formatTime, formatWait } from '@/pages/ticket/constants'
import type { ReviewQueueItem } from '@/types/interface'

defineOptions({ name: 'TicketReviews' })

const router = useRouter()
const userStore = useUserStore()

const items = ref<ReviewQueueItem[]>([])
const loading = ref(false)
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

const currentAdminId = computed(() => Number(userStore.userInfo?.id ?? 0))

const reviewVisible = ref(false)
const reviewing = ref(false)
const activeItem = ref<ReviewQueueItem | null>(null)
const reviewForm = reactive<{ action: 'approve' | 'reject'; note: string }>({ action: 'approve', note: '' })

async function loadQueue() {
  loading.value = true
  try {
    const data = await getTicketReviews({ page: pagination.current, page_size: pagination.pageSize })
    items.value = data.items ?? []
    total.value = data.meta?.total ?? 0
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载复核队列失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: { current: number; pageSize: number }) {
  pagination.current = pageInfo.current
  loadQueue()
}

function handlePageSizeChange(pageSize: number) {
  pagination.pageSize = pageSize
  pagination.current = 1
  loadQueue()
}

function openTicket(item: ReviewQueueItem) {
  router.push(`/tickets/detail/${item.ticket_id}`)
}

function openReview(item: ReviewQueueItem, action: 'approve' | 'reject') {
  if (item.sender_id === currentAdminId.value) {
    MessagePlugin.warning('不能复核自己提交的回复')
    return
  }
  activeItem.value = item
  reviewForm.action = action
  reviewForm.note = ''
  reviewVisible.value = true
}

async function handleReviewSubmit() {
  if (!activeItem.value) return
  if (reviewForm.action === 'reject' && !reviewForm.note.trim()) {
    MessagePlugin.warning('驳回需填写说明')
    return
  }
  reviewing.value = true
  try {
    await reviewTicketReply(activeItem.value.reply_id, {
      action: reviewForm.action,
      note: reviewForm.note.trim() || undefined,
    })
    MessagePlugin.success(reviewForm.action === 'approve' ? '复核已通过，回复已对用户可见' : '已驳回，将通知提交人')
    reviewVisible.value = false
    loadQueue()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '复核操作失败')
  } finally {
    reviewing.value = false
  }
}

onMounted(loadQueue)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.review-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding-top: var(--space-sm);
}

.review-card {
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-lg);
  padding: var(--space-md) var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: var(--hs-surface-1);
  transition: border-color var(--hs-duration-fast);
}

.review-card:hover {
  border-color: var(--td-brand-color-3);
}

.review-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.review-card__ticket {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.review-card__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.review-card__meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.review-card__time {
  font-variant-numeric: tabular-nums;
}

.review-card__content {
  font-size: 13px;
  line-height: 1.7;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--hs-surface-2);
  border-radius: var(--hs-radius-md);
  padding: 10px 12px;
  max-height: 120px;
  overflow-y: auto;
}

.review-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.review-card__self-tip {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
  text-align: right;
}

.review-pagination {
  margin-top: var(--space-lg);
  justify-content: flex-end;
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
</style>
