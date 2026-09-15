<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ChatIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">通知记录</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="goDeliveries">
          <template #icon><RootListIcon aria-hidden="true" /></template>
          去发送日志
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">事件类型</span>
        <t-input v-model="filters.event" placeholder="如：order.paid" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">发送渠道</span>
        <t-select v-model="filters.channel" clearable placeholder="全部渠道" :options="channelOptions" />
      </div>
      <div class="field">
        <span class="field__label">发送状态</span>
        <t-select v-model="filters.send_status" clearable placeholder="全部状态" :options="sendStatusOptions" />
      </div>
      <div class="field">
        <span class="field__label">目标类型</span>
        <t-select v-model="filters.target_type" clearable placeholder="全部目标" :options="targetTypeOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">通知记录</h3>
        <span class="table-card__meta">共 {{ total }} 条记录</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #id="{ row }">
          <span class="cell-muted">{{ row.id }}</span>
        </template>
        <template #target_type="{ row }">
          <t-tag :theme="targetTypeTheme(row.target_type)" variant="light" size="small" shape="round">
            {{ targetTypeLabel(row.target_type) }}
          </t-tag>
        </template>
        <template #event="{ row }">
          <span class="cell-strong">{{ row.event || '—' }}</span>
        </template>
        <template #title="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.title || '—' }}</t-link>
        </template>
        <template #channel="{ row }">
          <t-tag :theme="channelTheme(row.channel)" variant="light" size="small" shape="round">
            {{ channelLabel(row.channel) }}
          </t-tag>
        </template>
        <template #content_format="{ row }">
          <span class="cell-muted">{{ formatLabel(row.content_format) }}</span>
        </template>
        <template #send_status="{ row }">
          <t-tag :theme="sendStatusTheme(row.send_status)" variant="light" size="small" shape="round">
            {{ sendStatusLabel(row.send_status) }}
          </t-tag>
        </template>
        <template #source_module="{ row }">
          <span class="cell-muted">{{ row.source_module || '—' }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #action="{ row }">
<div class="action-cell">
              <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '查看', value: 'detail', theme: 'default' },
                { content: '重发', value: 'resend', hidden: () => !canResend(row), theme: 'warning' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">查看</t-link>
              <t-link
                v-if="canResend(row)"
                theme="warning"
                hover="color"
                @click="handleResend(row)"
              >
                重发
              </t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无通知记录" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="page.current"
        :page-size="page.size"
        :total="total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <!-- 通知内容查看弹窗 -->
    <t-dialog
      v-model:visible="detailVisible"
      header="通知内容"
      width="560px"
      :footer="false"
    >
      <div v-if="currentRow" class="detail-list">
        <div class="detail-row"><span>记录 ID</span><b>{{ currentRow.id }}</b></div>
        <div class="detail-row"><span>目标</span>
          {{ targetTypeLabel(currentRow.target_type) }} · {{ currentRow.target_name || ('ID ' + currentRow.target_id) }}
        </div>
        <div class="detail-row"><span>事件</span>{{ currentRow.event || '—' }}</div>
        <div class="detail-row"><span>渠道</span>
          <t-tag :theme="channelTheme(currentRow.channel)" variant="light" size="small">{{ channelLabel(currentRow.channel) }}</t-tag>
        </div>
        <div class="detail-row"><span>格式</span>{{ formatLabel(currentRow.content_format) }}</div>
        <div class="detail-row"><span>状态</span>
          <t-tag :theme="sendStatusTheme(currentRow.send_status)" variant="light" size="small">{{ sendStatusLabel(currentRow.send_status) }}</t-tag>
        </div>
        <div v-if="currentRow.delivery_id" class="detail-row"><span>投递记录</span>#{{ currentRow.delivery_id }}</div>
        <div class="detail-row"><span>来源模块</span>{{ currentRow.source_module || '—' }}</div>
        <div class="detail-row"><span>创建时间</span>{{ formatTime(currentRow.created_at) }}</div>
        <div v-if="currentRow.sent_at" class="detail-row"><span>发送时间</span>{{ formatTime(currentRow.sent_at) }}</div>
        <div v-if="currentRow.fail_reason" class="detail-row"><span>失败原因</span><span class="fail-text">{{ currentRow.fail_reason }}</span></div>
        <div class="detail-row detail-row--column"><span>标题</span><b>{{ currentRow.title || '—' }}</b></div>
        <div class="detail-row detail-row--column"><span>正文</span>
          <!--
            HTML 正文来自本系统后台模板，且已用 DOMPurify 净化：用户资料类变量
            （用户名/备注）可能含尖括号，直接 v-html 会构成存储型 XSS。
          -->
          <div
            v-if="currentRow.content_format === 'html'"
            class="detail-content detail-content--html"
            v-html="safeHtml(currentRow.content)"
          ></div>
          <pre v-else class="detail-content">{{ currentRow.content || '—' }}</pre>
        </div>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ChatIcon, RefreshIcon, RootListIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import DOMPurify from 'dompurify'

import { getNotifyRecords, resendNotify, type NotifyRecordItem } from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { formatLabel } from '@/pages/notification/constants'

defineOptions({ name: 'NotifyRecords' })

const router = useRouter()
const loading = ref(false)
const { isMobile } = useIsMobile()
const list = ref<NotifyRecordItem[]>([])
const total = ref(0)
const filters = reactive({ event: '', channel: '', send_status: '', target_type: '' })
const page = reactive({ current: 1, size: 10 })

const channelOptions = [
  { label: '站内信', value: 'inbox' },
  { label: '邮件', value: 'mail' },
  { label: '短信', value: 'sms' },
]

const sendStatusOptions = [
  { label: '已发送', value: 'sent' },
  { label: '失败', value: 'failed' },
  { label: '待发送', value: 'pending' },
]

const targetTypeOptions = [
  { label: '用户', value: 'user' },
  { label: '管理员', value: 'admin' },
]

const channelMap: Record<string, { label: string; theme: string }> = {
  inbox: { label: '站内信', theme: 'primary' },
  mail: { label: '邮件', theme: 'success' },
  sms: { label: '短信', theme: 'warning' },
}

const sendStatusMap: Record<string, { label: string; theme: string }> = {
  sent: { label: '已发送', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  pending: { label: '待发送', theme: 'warning' },
}

const targetTypeMap: Record<string, { label: string; theme: string }> = {
  user: { label: '用户', theme: 'primary' },
  admin: { label: '管理员', theme: 'warning' },
}

function channelLabel(c: string): string {
  return channelMap[c]?.label || c
}

function channelTheme(c: string): string {
  return channelMap[c]?.theme || 'default'
}

function sendStatusLabel(s: string): string {
  return sendStatusMap[s]?.label || s
}

function sendStatusTheme(s: string): string {
  return sendStatusMap[s]?.theme || 'default'
}

function targetTypeLabel(t: string): string {
  return targetTypeMap[t]?.label || t
}

function targetTypeTheme(t: string): string {
  return targetTypeMap[t]?.theme || 'default'
}

function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

const columns: PrimaryTableCol[] = [
  { colKey: 'id', title: 'ID', width: 80 },
  { colKey: 'target_type', title: '目标', width: 90 },
  { colKey: 'event', title: '事件', width: 140 },
  { colKey: 'title', title: '标题', minWidth: 180 },
  { colKey: 'channel', title: '渠道', width: 90 },
  { colKey: 'content_format', title: '格式', width: 80 },
  { colKey: 'send_status', title: '发送状态', width: 100 },
  { colKey: 'source_module', title: '来源模块', width: 120 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 120, fixed: 'right', align: 'center' },
]

const pagination = computed(() => ({
  current: page.current,
  pageSize: page.size,
  total: total.value,
  showJumper: true,
}))

async function loadData() {
  loading.value = true
  try {
    const resp = await getNotifyRecords({
      event: filters.event || undefined,
      channel: filters.channel || undefined,
      send_status: filters.send_status || undefined,
      target_type: filters.target_type || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.list || []
    total.value = resp.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载通知记录失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.event = ''
  filters.channel = ''
  filters.send_status = ''
  filters.target_type = ''
  page.current = 1
  loadData()
}

function handlePageChange(info: PageInfo) {
  page.current = info.current
  page.size = info.pageSize
  loadData()
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const totalPages = Math.max(1, Math.ceil(total.value / page.size))
  const clamped = Math.min(Math.max(target, 1), totalPages)
  if (clamped === page.current) return
  void applyMobilePage(clamped, page.size)
}

async function applyMobilePage(current: number, pageSize: number) {
  page.current = current
  page.size = pageSize
  await loadData()
}

function handleMobilePageSizeChange(pageSize: number) {
  void applyMobilePage(1, pageSize)
}


// —— 内容查看 ——
const detailVisible = ref(false)
const currentRow = ref<NotifyRecordItem | null>(null)

function openDetail(row: NotifyRecordItem) {
  currentRow.value = row
  detailVisible.value = true
}

// —— 重发：改为对关联的投递记录重投（doc90 §8.6）——
// 纯站内信（delivery_id=0）没有外发投递，按钮不显示。
function canResend(row: NotifyRecordItem): boolean {
  return row.delivery_id > 0
}

function goDeliveries() {
  router.push('/notification/deliveries')
}

// HTML 正文净化：内容来自本系统模板，但模板变量可能带用户输入（用户名/备注）。
function safeHtml(content: string): string {
  return DOMPurify.sanitize(content || '', { USE_PROFILES: { html: true } })
}

function handleResend(row: NotifyRecordItem) {
  const dialog = DialogPlugin.confirm({
    header: '重发通知',
    body: `将重投记录 #${row.id} 关联的投递（#${row.delivery_id}）。已发送的记录不允许重投。`,
    confirmBtn: { content: '确认重发', theme: 'primary', loading: false },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      dialog.update({ confirmBtn: { content: '发送中...', loading: true } })
      try {
        const msg = await resendNotify(row.id)
        MessagePlugin.success(msg || '已重新投递')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '重发失败')
        dialog.update({ confirmBtn: { content: '确认重发', loading: false } })
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadData)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: NotifyRecordItem) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'resend':
      void handleResend(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.detail-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.detail-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  font-size: 13px;
  color: #334155;
}
.detail-row > span:first-child {
  flex: 0 0 88px;
  color: var(--color-muted-foreground, #94a3b8);
}
.detail-row--column {
  flex-direction: column;
  gap: 4px;
}
.detail-content {
  margin: 0;
  padding: 10px 12px;
  background: var(--hs-surface-2, #f4f6fa);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 240px;
  overflow-y: auto;
}
/* HTML 正文：保留富文本排版，但同样限制高度与换行行为。 */
.detail-content--html {
  white-space: normal;
}
.detail-content--html :deep(img) {
  max-width: 100%;
  height: auto;
}
.detail-content--html :deep(table) {
  width: 100%;
  border-collapse: collapse;
}
.detail-content--html :deep(td),
.detail-content--html :deep(th) {
  border: 1px solid #e5e7eb;
  padding: 4px 8px;
  font-size: 12px;
}
.fail-text {
  color: var(--td-error-color, #d54941);
}
</style>
