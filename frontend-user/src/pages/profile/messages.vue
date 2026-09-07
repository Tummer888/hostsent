<template>
  <div class="messages-page">
    <!-- 页头 -->
    <section class="messages-hero">
      <div class="hero-left">
        <span class="hero-chip"><NotificationIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">我的消息</span>
          <span class="hero-desc">查看系统通知与平台公告，及时掌握账户动态。</span>
        </div>
      </div>
      <div class="hero-right">
        <t-tag v-if="unreadCount > 0" theme="danger" variant="light" shape="round">
          {{ unreadCount }} 条未读
        </t-tag>
        <t-tag v-else theme="success" variant="light" shape="round">已全部阅读</t-tag>
      </div>
    </section>

    <!-- 主体面板 -->
    <section class="panel">
      <t-tabs v-model="activeTab" @change="onTabChange">
        <!-- 通知 -->
        <t-tab-panel value="notifications" label="通知">
          <div class="tab-head">
            <span class="tab-title">站内通知</span>
            <t-space size="small">
              <t-button
                theme="primary"
                size="small"
                :loading="markingAll"
                :disabled="!notifications.length"
                @click="handleReadAll"
              >
                <template #icon><CheckCircleIcon /></template>
                全部已读
              </t-button>
              <t-button variant="outline" size="small" :loading="notifLoading" @click="loadNotifications">
                <template #icon><RefreshIcon /></template>
                刷新
              </t-button>
            </t-space>
          </div>

          <t-table
            :data="notifications"
            :columns="notifColumns"
            size="small"
            row-key="id"
            :pagination="pagination"
            :bordered="false"
            hover
            cell-empty-content="—"
            :loading="notifLoading"
            @page-change="onNotifPageChange"
          >
            <template #title="{ row }">
              <t-link theme="primary" hover="color" @click="openNotifDetail(row)">
                {{ row.title }}
              </t-link>
            </template>
            <template #event="{ row }">
              <t-tag theme="primary" variant="light" size="small" shape="round">
                {{ eventLabel(row.event) }}
              </t-tag>
            </template>
            <template #is_read="{ row }">
              <t-tag
                :theme="row.is_read ? 'success' : 'warning'"
                variant="light"
                size="small"
                shape="round"
              >
                {{ row.is_read ? '已读' : '未读' }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              <span class="time-text">{{ formatTime(row.created_at) }}</span>
            </template>
            <template #operation="{ row }">
              <t-link theme="primary" hover="color" @click="openNotifDetail(row)">详情</t-link>
            </template>
            <template #empty>
              <t-empty description="暂无通知消息" />
            </template>
          </t-table>
        </t-tab-panel>

        <!-- 公告 -->
        <t-tab-panel value="announcements" label="公告">
          <div class="tab-head">
            <span class="tab-title">平台公告</span>
            <t-button variant="outline" size="small" :loading="annLoading" @click="loadAnnouncements">
              <template #icon><RefreshIcon /></template>
              刷新
            </t-button>
          </div>

          <t-loading :loading="annLoading" show-overlay>
            <div v-if="announcements.length" class="announcement-list">
              <t-card
                v-for="ann in announcements"
                :key="ann.id"
                class="announcement-card"
                :bordered="false"
              >
                <div class="ann-header">
                  <span class="ann-title">{{ ann.title }}</span>
                  <t-tag
                    :theme="levelTheme(ann.level)"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    {{ levelLabel(ann.level) }}
                  </t-tag>
                </div>
                <p class="ann-content">{{ ann.content }}</p>
                <div class="ann-footer">
                  <span class="time-text">发布时间：{{ formatTime(ann.published_at || ann.created_at) }}</span>
                </div>
              </t-card>
            </div>
            <t-empty v-else description="暂无公告" />
          </t-loading>
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- 通知详情弹窗 -->
    <t-dialog
      v-model:visible="detailVisible"
      :header="currentNotif?.title || '通知详情'"
      width="520px"
      :footer="false"
      @close="detailVisible = false"
    >
      <div v-if="currentNotif" class="notif-detail">
        <div class="detail-meta">
          <t-tag theme="primary" variant="light" size="small" shape="round">
            {{ eventLabel(currentNotif.event) }}
          </t-tag>
          <t-tag
            :theme="currentNotif.is_read ? 'success' : 'warning'"
            variant="light"
            size="small"
            shape="round"
          >
            {{ currentNotif.is_read ? '已读' : '未读' }}
          </t-tag>
          <span class="time-text">{{ formatTime(currentNotif.created_at) }}</span>
        </div>
        <div class="detail-content">{{ currentNotif.content }}</div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { CheckCircleIcon, NotificationIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getMyAnnouncements,
  getMyNotifications,
  getNotificationDetail,
  getUnreadCount,
  readAll,
  type AnnouncementInfo,
  type NotificationInfo,
} from '@/api/notification'

defineOptions({ name: 'UserMessages' })

const activeTab = ref<'notifications' | 'announcements'>('notifications')

// ========== 通知列表 ==========
const notifications = ref<NotificationInfo[]>([])
const notifLoading = ref(false)
const markingAll = ref(false)
const unreadCount = ref(0)

const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: total.value,
  showJumper: true,
}))

const notifColumns: PrimaryTableCol<NotificationInfo>[] = [
  { colKey: 'title', title: '标题', ellipsis: true, width: 260 },
  { colKey: 'event', title: '事件', width: 120 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  { colKey: 'is_read', title: '状态', width: 90 },
  { colKey: 'operation', title: '操作', width: 90 },
]

async function loadUnreadCount() {
  try {
    const { data } = await getUnreadCount()
    if (data) unreadCount.value = data.count ?? 0
  } catch (error) {
    /* 静默：未读计数失败不应阻塞主流程 */
    console.warn('Load unread count failed:', error)
  }
}

async function loadNotifications() {
  notifLoading.value = true
  try {
    const { data } = await getMyNotifications({
      page: page.value,
      page_size: pageSize.value,
    })
    if (data) {
      notifications.value = data.list ?? []
      total.value = data.total ?? 0
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载通知列表失败')
  } finally {
    notifLoading.value = false
  }
}

function onNotifPageChange(info: PageInfo) {
  page.value = info.current
  pageSize.value = info.pageSize
  loadNotifications()
}

// 查看通知详情（自动标记为已读）
async function openNotifDetail(row: NotificationInfo) {
  // 先用列表项填充弹窗，便于即时展示
  currentNotif.value = { ...row }
  detailVisible.value = true

  try {
    const { data } = await getNotificationDetail(row.id)
    if (data) currentNotif.value = data
    // 后端在 GET 详情时自动标记为已读
    if (!row.is_read) {
      row.is_read = true
      unreadCount.value = Math.max(0, unreadCount.value - 1)
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载通知详情失败')
  }
}

// 全部标记为已读
async function handleReadAll() {
  markingAll.value = true
  try {
    await readAll()
    notifications.value.forEach((n) => (n.is_read = true))
    unreadCount.value = 0
    MessagePlugin.success('已将全部通知标记为已读')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '操作失败')
  } finally {
    markingAll.value = false
  }
}

// ========== 公告列表 ==========
const announcements = ref<AnnouncementInfo[]>([])
const annLoading = ref(false)
let annLoaded = false

async function loadAnnouncements() {
  annLoading.value = true
  try {
    const { data } = await getMyAnnouncements()
    announcements.value = data ?? []
    annLoaded = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载公告失败')
  } finally {
    annLoading.value = false
  }
}

// 切换 Tab 时按需懒加载
function onTabChange(value: string | number) {
  if (value === 'announcements' && !annLoaded) loadAnnouncements()
}

// ========== 详情弹窗 ==========
const detailVisible = ref(false)
const currentNotif = ref<NotificationInfo | null>(null)

// ========== 工具函数 ==========
function formatTime(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const EVENT_LABELS: Record<string, string> = {
  order_created: '订单创建',
  order_paid: '订单支付',
  order_cancelled: '订单取消',
  instance_started: '实例启动',
  instance_stopped: '实例停止',
  instance_expired: '实例到期',
  instance_renewed: '实例续费',
  ticket_replied: '工单回复',
  ticket_closed: '工单关闭',
  recharge_arrived: '充值到账',
  system: '系统通知',
}

function eventLabel(event?: string): string {
  if (!event) return '—'
  return EVENT_LABELS[event] || event
}

const LEVEL_THEMES: Record<string, string> = {
  info: 'primary',
  warning: 'warning',
  critical: 'danger',
}

function levelTheme(level?: string): string {
  return LEVEL_THEMES[level || ''] || 'default'
}

const LEVEL_LABELS: Record<string, string> = {
  info: '普通',
  warning: '提醒',
  critical: '重要',
}

function levelLabel(level?: string): string {
  return LEVEL_LABELS[level || ''] || level || '—'
}

// ========== 初始化 ==========
onMounted(() => {
  loadUnreadCount()
  loadNotifications()
})
</script>

<style scoped>
.messages-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 页头横幅 */
.messages-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 24px 32px;
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
  flex-shrink: 0;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-label {
  font-size: 18px;
  font-weight: 700;
}

.hero-desc {
  font-size: 13px;
  opacity: 0.85;
}

.hero-right :deep(.t-tag) {
  background: rgba(255, 255, 255, 0.18) !important;
  color: #fff !important;
  border: 1px solid rgba(255, 255, 255, 0.25);
}

/* 面板 */
.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.tab-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}

.tab-title {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

/* 公告卡片 */
.announcement-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.announcement-card {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
  transition: box-shadow 0.2s;
}

.announcement-card:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.ann-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.ann-title {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.ann-content {
  margin: 0 0 10px;
  font-size: 14px;
  color: #475569;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.ann-footer {
  display: flex;
  justify-content: flex-end;
}

/* 详情弹窗 */
.notif-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail-content {
  font-size: 14px;
  color: #1e293b;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  padding: 12px 14px;
  background: #f8fafc;
  border-radius: 8px;
}

@media (max-width: 768px) {
  .messages-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
