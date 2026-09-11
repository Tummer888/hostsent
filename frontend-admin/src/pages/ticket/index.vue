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
      <!-- 工作台视图（P2-07）：我的待办 / 未分配池 / 我参与的 / SLA 超时 -->
      <div class="workbench-tabs" role="tablist" aria-label="工单视图">
        <button
          v-for="tab in viewTabs"
          :key="tab.value"
          type="button"
          role="tab"
          class="workbench-tab"
          :class="{ 'is-active': activeView === tab.value }"
          :aria-selected="activeView === tab.value"
          @click="handleViewChange(tab.value)"
        >
          {{ tab.label }}
        </button>
      </div>
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
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
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
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
        :pagination="isMobile ? undefined : pagination"
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
        <template #sla="{ row }">
          <t-tag v-if="row.sla_breached" theme="danger" variant="light" size="small" shape="round">超时</t-tag>
          <t-tag v-else-if="row.sla_hours > 0" theme="success" variant="light" size="small" shape="round">
            {{ row.sla_hours }}h
          </t-tag>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail' },
                { content: '认领', value: 'claim', hidden: () => !canClaim(row) },
                { content: '关闭', value: 'close', hidden: () => !(isClosable(row) && has('ticket:close')), theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="canClaim(row)" theme="success" hover="color" @click="handleClaim(row)">认领</t-link>
              <t-link v-if="isClosable(row) && has('ticket:close')" theme="danger" hover="color" @click="handleClose(row)">关闭</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无工单数据" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { RefreshIcon, SearchIcon, ServiceIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { closeTicket, claimTicket, getTicketCategories, getTickets } from '@/api/ticket'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'
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
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'TicketList' })

const router = useRouter()
const { has } = usePermission()
const ticketList = ref<TicketInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const categoryOptions = ref<{ label: string; value: string }[]>([])

// 工作台视图：空串表示全部
const viewTabs = [
  { label: '全部工单', value: '' },
  { label: '我的待办', value: 'my_todo' },
  { label: '未分配池', value: 'unassigned' },
  { label: '我参与的', value: 'involved' },
  { label: 'SLA 超时', value: 'sla_breached' },
]
const activeView = ref('')

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

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<TicketInfo>[] = [
  { colKey: 'ticket_no', title: '工单号', minWidth: 170 },
  { colKey: 'user', title: '用户', minWidth: 140 },
  { colKey: 'title', title: '标题', minWidth: 200, ellipsis: true },
  { colKey: 'category', title: '分类', width: 110 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 110 },
  { colKey: 'sla', title: 'SLA', width: 80, align: 'center' as const },
  { colKey: 'created_at', title: '提交时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 150, fixed: 'right' as const, align: 'center' as const },
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
      view: activeView.value || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    ticketList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载工单列表失败')
  } finally {
    loading.value = false
  }
}

function handleViewChange(view: string) {
  activeView.value = view
  pagination.current = 1
  loadTickets()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTickets()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
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

function handleMobileAction(value: string | number | Record<string, any>, row: TicketInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'claim':
      void handleClaim(row)
      break
    case 'close':
      void handleClose(row)
      break
  }
}

function openDetail(row: TicketInfo) {
  router.push(`/tickets/detail/${row.id}`)
}

// 仅非终态工单可关闭
function isClosable(row: TicketInfo): boolean {
  return row.status !== 'closed' && row.status !== 'cancelled'
}

// 未分配且非终态可认领
function canClaim(row: TicketInfo): boolean {
  return !row.assigned_to && isClosable(row) && has('ticket:list')
}

async function handleClaim(row: TicketInfo) {
  try {
    await claimTicket(row.id)
    MessagePlugin.success('认领成功')
    loadTickets()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '认领失败')
  }
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

<style scoped>
/* ---------- 工作台视图切换 ---------- */
.workbench-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--color-border);
}

.workbench-tab {
  border: 1px solid transparent;
  background: var(--hs-surface-2);
  color: var(--color-muted-foreground);
  font-size: 13px;
  padding: 6px 14px;
  border-radius: 999px;
  cursor: pointer;
  transition: color var(--hs-duration-fast), background-color var(--hs-duration-fast), border-color var(--hs-duration-fast);
}

.workbench-tab:hover {
  color: var(--color-foreground);
  border-color: var(--color-border);
}

.workbench-tab.is-active {
  background: var(--color-primary);
  color: #ffffff;
  border-color: var(--color-primary);
}
</style>
