<template>
  <div class="page-body ticket-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServiceIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">工单列表</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadTickets">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="工单号 / 标题" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户账号</span>
          <t-input v-model="filters.user_keyword" placeholder="用户名 / 邮箱 / 手机号" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">工单分类</span>
          <t-select v-model="filters.category" clearable placeholder="全部分类" :options="categoryOptions" />
        </div>
        <div class="field">
          <span class="field__label">优先级</span>
          <t-select v-model="filters.priority" clearable placeholder="全部优先级" :options="ticketPriorityOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="ticketStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">提交时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">工单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个工单</span>
      </div>
      <t-table
        row-key="id"
        :data="ticketList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #ticket_no="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.ticket_no }}</t-link>
        </template>
        <template #user="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="product-sub">ID {{ row.user_id }}</span>
          </div>
        </template>
        <template #category="{ row }">
          <t-tag variant="light" size="small" shape="round">{{ row.category_name || row.category }}</t-tag>
        </template>
        <template #priority="{ row }">
          <t-tag :theme="ticketPriorityTheme(row.priority)" variant="light" size="small" shape="round">
            {{ ticketPriorityLabel(row.priority) }}
          </t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="ticketStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ ticketStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #assigned_name="{ row }">
          <span v-if="row.assigned_name" class="cell-strong">{{ row.assigned_name }}</span>
          <span v-else class="cell-muted">未分配</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
            <t-link v-if="isClosable(row)" theme="danger" hover="color" @click="handleClose(row)">关闭</t-link>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无工单数据" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { RefreshIcon, SearchIcon, ServiceIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { closeTicket, getTicketCategories, getTickets } from '@/api/ticket'
import {
  formatTime,
  ticketPriorityLabel,
  ticketPriorityOptions,
  ticketPriorityTheme,
  ticketStatusLabel,
  ticketStatusOptions,
  ticketStatusTheme,
  toDateString,
} from '@/pages/ticket/constants'
import type { TicketCategoryInfo, TicketInfo } from '@/types/interface'

defineOptions({ name: 'TicketList' })

const router = useRouter()
const ticketList = ref<TicketInfo[]>([])
const loading = ref(false)
const total = ref(0)
const categoryOptions = ref<{ label: string; value: string }[]>([])

const filters = reactive<{
  keyword: string | undefined
  user_keyword: string | undefined
  category: string | undefined
  priority: string | undefined
  status: string | undefined
  dateRange: [string, string] | undefined
}>({
  keyword: undefined,
  user_keyword: undefined,
  category: undefined,
  priority: undefined,
  status: undefined,
  dateRange: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<TicketInfo>[] = [
  { colKey: 'ticket_no', title: '工单号', minWidth: 170 },
  { colKey: 'user', title: '用户', minWidth: 140 },
  { colKey: 'title', title: '标题', minWidth: 200, ellipsis: true },
  { colKey: 'category', title: '分类', width: 110 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 110 },
  { colKey: 'created_at', title: '提交时间', width: 160 },
  { colKey: 'action', title: '操作', width: 120, fixed: 'right' as const, align: 'center' as const },
]

// 加载工单分类下拉（供筛选使用）
async function loadCategories() {
  try {
    const items: TicketCategoryInfo[] = await getTicketCategories()
    categoryOptions.value = items.map((item) => ({ label: item.name, value: item.code }))
  } catch {
    // 分类下拉加载失败不阻断列表
  }
}

async function loadTickets() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getTickets({
      keyword: filters.keyword,
      user_keyword: filters.user_keyword,
      category: filters.category,
      priority: filters.priority,
      status: filters.status,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    ticketList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载工单列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTickets()
}

function handleSearch() {
  pagination.current = 1
  loadTickets()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.user_keyword = undefined
  filters.category = undefined
  filters.priority = undefined
  filters.status = undefined
  filters.dateRange = undefined
  pagination.current = 1
  loadTickets()
}

function openDetail(row: TicketInfo) {
  router.push(`/tickets/detail/${row.id}`)
}

// 仅非终态工单可关闭
function isClosable(row: TicketInfo): boolean {
  return row.status !== 'closed' && row.status !== 'cancelled'
}

function handleClose(row: TicketInfo) {
  const dialog = DialogPlugin.confirm({
    header: '关闭工单',
    body: `确认关闭工单「${row.ticket_no}」吗？关闭后用户不可再回复。`,
    confirmBtn: { content: '确认关闭', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        await closeTicket(row.id)
        MessagePlugin.success('工单已关闭')
        dialog.destroy()
        loadTickets()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '关闭失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(() => {
  loadCategories()
  loadTickets()
})
</script>

<style lang="css">
@import './shared.css';
</style>
