<template>
  <div class="page-body notify-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <NotificationIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">公告管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新增公告
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="标题关键词" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
        <div class="field">
          <span class="field__label">平台</span>
          <t-select v-model="filters.platform" clearable placeholder="全部平台" :options="platformOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">公告列表</h3>
        <span class="table-card__meta">共 {{ total }} 条公告</span>
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
        <template #title="{ row }">
          <span class="cell-strong">{{ row.title }}</span>
        </template>
        <template #platform="{ row }">
          <t-tag :theme="platformTheme(row.platform)" variant="light" size="small" shape="round">
            {{ platformLabel(row.platform) }}
          </t-tag>
        </template>
        <template #level="{ row }">
          <t-tag :theme="levelTheme(row.level)" variant="light" size="small" shape="round">
            {{ levelLabel(row.level) }}
          </t-tag>
        </template>
        <template #popup="{ row }">
          <t-tag v-if="row.popup" theme="primary" variant="light" size="small" shape="round">是</t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">否</t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabel(row.status) }}
          </t-tag>
        </template>
        <template #publish_at="{ row }">
          <span class="time-text">{{ formatTime(row.publish_at) }}</span>
        </template>
        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '发布', value: 'publish', hidden: () => !(row.status !== 'published'), theme: 'success' },
                { content: '下线', value: 'offline', hidden: () => !(row.status === 'published'), theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link
                v-if="row.status !== 'published'"
                theme="success"
                hover="color"
                @click="handlePublish(row)"
              >
                发布
              </t-link>
              <t-link
                v-if="row.status === 'published'"
                theme="warning"
                hover="color"
                @click="handleOffline(row)"
              >
                下线
              </t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无公告" />
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

    <!-- 新增/编辑公告抽屉 -->
    <t-drawer
      v-model:visible="formVisible"
      :header="editingId ? '编辑公告' : '新增公告'"
      size="480px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="标题" name="title">
          <t-input v-model="form.title" placeholder="请输入公告标题" :maxlength="120" />
        </t-form-item>
        <t-form-item label="正文" name="content">
          <t-textarea
            v-model="form.content"
            placeholder="请输入公告正文"
            :autosize="{ minRows: 4, maxRows: 10 }"
          />
        </t-form-item>
        <t-form-item label="平台范围" name="platform">
          <t-select v-model="form.platform" :options="platformOptions" />
        </t-form-item>
        <t-form-item label="级别" name="level">
          <t-select v-model="form.level" :options="levelOptions" />
        </t-form-item>
        <t-form-item label="是否弹窗" name="popup">
          <t-switch v-model="form.popup" />
        </t-form-item>
        <t-form-item label="定时发布时间" name="publish_at">
          <t-date-picker
            v-model="form.publish_at"
            placeholder="不填则即时发布"
            enable-time-picker
            clearable
            style="width: 100%"
          />
        </t-form-item>
      </t-form>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, NotificationIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createAnnouncement,
  deleteAnnouncement,
  getAnnouncements,
  offlineAnnouncement,
  publishAnnouncement,
  updateAnnouncement,
  type AnnouncementItem,
  type AnnouncementSaveRequest,
} from '@/api/notification'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'NotifyAnnouncements' })

const loading = ref(false)
const { isMobile } = useIsMobile()
const saving = ref(false)
const list = ref<AnnouncementItem[]>([])
const total = ref(0)
const filters = reactive({ keyword: '', status: '', platform: '' })
const page = reactive({ current: 1, size: 10 })

const statusOptions = [
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已下线', value: 'offline' },
]

const platformOptions = [
  { label: '用户端', value: 'user' },
  { label: '管理端', value: 'admin' },
  { label: '全部端', value: 'both' },
]

const levelOptions = [
  { label: '普通', value: 'info' },
  { label: '警告', value: 'warning' },
  { label: '紧急', value: 'critical' },
]

const statusMap: Record<string, { label: string; theme: string }> = {
  draft: { label: '草稿', theme: 'default' },
  published: { label: '已发布', theme: 'success' },
  offline: { label: '已下线', theme: 'warning' },
}

const platformMap: Record<string, { label: string; theme: string }> = {
  user: { label: '用户端', theme: 'primary' },
  admin: { label: '管理端', theme: 'warning' },
  both: { label: '全部端', theme: 'default' },
}

const levelMap: Record<string, { label: string; theme: string }> = {
  info: { label: '普通', theme: 'default' },
  warning: { label: '警告', theme: 'warning' },
  critical: { label: '紧急', theme: 'danger' },
}

function statusLabel(s: string): string {
  return statusMap[s]?.label || s
}

function statusTheme(s: string): string {
  return statusMap[s]?.theme || 'default'
}

function platformLabel(p: string): string {
  return platformMap[p]?.label || p
}

function platformTheme(p: string): string {
  return platformMap[p]?.theme || 'default'
}

function levelLabel(l: string): string {
  return levelMap[l]?.label || l
}

function levelTheme(l: string): string {
  return levelMap[l]?.theme || 'default'
}

function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

const columns: PrimaryTableCol[] = [
  { colKey: 'title', title: '标题', minWidth: 200 },
  { colKey: 'platform', title: '平台', width: 100 },
  { colKey: 'level', title: '级别', width: 90 },
  { colKey: 'popup', title: '弹窗', width: 80, align: 'center' },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'publish_at', title: '发布时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 200, fixed: 'right', align: 'center' },
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
    const resp = await getAnnouncements({
      keyword: filters.keyword || undefined,
      status: filters.status || undefined,
      platform: filters.platform || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.list || []
    total.value = resp.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载公告列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  filters.platform = ''
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


// —— 新增 / 编辑 ——
const formVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<{
  title: string
  content: string
  platform: 'user' | 'admin' | 'both'
  level: 'info' | 'warning' | 'critical'
  popup: boolean
  publish_at: string
}>({
  title: '',
  content: '',
  platform: 'user',
  level: 'info',
  popup: false,
  publish_at: '',
})

function resetForm() {
  form.title = ''
  form.content = ''
  form.platform = 'user'
  form.level = 'info'
  form.popup = false
  form.publish_at = ''
}

function openCreate() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEdit(row: AnnouncementItem) {
  editingId.value = row.id
  form.title = row.title
  form.content = row.content
  form.platform = row.platform
  form.level = row.level
  form.popup = !!row.popup
  form.publish_at = row.publish_at ? row.publish_at.replace('T', ' ').slice(0, 19) : ''
  formVisible.value = true
}

async function handleSave() {
  if (!form.title.trim()) {
    MessagePlugin.warning('请输入公告标题')
    return
  }
  if (!form.content.trim()) {
    MessagePlugin.warning('请输入公告正文')
    return
  }
  saving.value = true
  try {
    const payload: AnnouncementSaveRequest = {
      title: form.title.trim(),
      content: form.content.trim(),
      platform: form.platform,
      level: form.level,
      popup: form.popup,
      publish_at: form.publish_at || undefined,
    }
    if (editingId.value) {
      await updateAnnouncement(editingId.value, payload)
      MessagePlugin.success('公告已更新')
    } else {
      await createAnnouncement(payload)
      MessagePlugin.success('公告已创建')
    }
    formVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存公告失败')
  } finally {
    saving.value = false
  }
}

// —— 发布 / 下线 / 删除 ——
async function handlePublish(row: AnnouncementItem) {
  try {
    await publishAnnouncement(row.id)
    MessagePlugin.success('公告已发布')
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '发布失败')
  }
}

async function handleOffline(row: AnnouncementItem) {
  const dialog = DialogPlugin.confirm({
    header: '下线公告',
    body: `确认下线公告「${row.title}」吗？下线后用户端将不再展示。`,
    confirmBtn: { content: '确认下线', theme: 'warning' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await offlineAnnouncement(row.id)
        MessagePlugin.success('公告已下线')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '下线失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function handleDelete(row: AnnouncementItem) {
  const dialog = DialogPlugin.confirm({
    header: '删除公告',
    body: `确认删除公告「${row.title}」吗？此操作不可恢复。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteAnnouncement(row.id)
        MessagePlugin.success('公告已删除')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadData)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: AnnouncementItem) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'publish':
      void handlePublish(row)
      break
    case 'offline':
      void handleOffline(row)
      break
    case 'delete':
      void handleDelete(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>
