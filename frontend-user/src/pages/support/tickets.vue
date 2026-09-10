<template>
  <div class="tickets-page">
    <!-- 页头 -->
    <section class="tickets-hero">
      <div class="hero-left">
        <span class="hero-chip"><ServiceIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">工单中心</span>
          <span class="hero-desc">提交问题、跟进进度，与工程师实时沟通。</span>
        </div>
      </div>
      <t-button theme="primary" size="large" @click="goCreate">
        <template #icon><AddIcon /></template>
        提交工单
      </t-button>
    </section>

    <!-- 工单列表 -->
    <section class="tickets-panel">
      <div class="panel-head">
        <span class="panel-title">我的工单</span>
        <t-button variant="outline" size="small" :loading="loading" @click="loadTickets">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </div>

      <t-loading :loading="loading" show-overlay>
        <t-table
          :data="tickets"
          :columns="columns"
          row-key="id"
          :pagination="pagination"
          :bordered="false"
          hover
          cell-empty-content="—"
          @page-change="onPageChange"
        >
          <template #ticket_no="{ row }">
            <t-link theme="primary" hover="color" @click="goDetail(row.id)">{{ row.ticket_no }}</t-link>
          </template>
          <template #category="{ row }">
            <span>{{ row.category_name || row.category || '—' }}</span>
          </template>
          <template #username="{ row }">
            <span class="time-text">{{ row.username || '主账号' }}</span>
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
          <template #created_at="{ row }">
            <span class="time-text">{{ formatTime(row.created_at) }}</span>
          </template>
          <template #operation="{ row }">
            <t-space size="small">
              <t-link theme="primary" hover="color" @click="goDetail(row.id)">详情</t-link>
              <t-popconfirm v-if="row.status === 'open'" content="确定取消该工单吗？" theme="danger" @confirm="handleCancel(row)">
                <t-link theme="danger" hover="color">取消</t-link>
              </t-popconfirm>
            </t-space>
          </template>
          <template #empty>
            <t-empty description="暂无工单，点击右上角提交您的第一个工单" />
          </template>
        </t-table>
      </t-loading>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, RefreshIcon, ServiceIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { cancelMyTicket, getMyTickets, type TicketInfo } from '@/api/support'
import { formatTime, ticketPriorityLabel, ticketPriorityTheme, ticketStatusLabel, ticketStatusTheme } from '@/pages/support/constants'

defineOptions({ name: 'TicketList' })

const router = useRouter()

const loading = ref(false)
const tickets = ref<TicketInfo[]>([])

// 分页状态（后端分页：page 从 1 开始，默认每页 10 条）
const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: total.value,
  showJumper: true,
}))

const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 表格列定义
const columns: PrimaryTableCol<TicketInfo>[] = [
  { colKey: 'ticket_no', title: '工单号', width: 150 },
  { colKey: 'title', title: '标题', ellipsis: true },
  { colKey: 'category', title: '分类', width: 110 },
  { colKey: 'username', title: '操作人', width: 120 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'reply_count', title: '回复数', width: 80 },
  { colKey: 'created_at', title: '提交时间', width: 160 },
  { colKey: 'operation', title: '操作', width: 120 },
]

// 加载我的工单列表
async function loadTickets() {
  loading.value = true
  try {
    const { data } = await getMyTickets({ page: page.value, page_size: pageSize.value })
    if (data) {
      tickets.value = data.items
      total.value = data.meta?.total ?? 0
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载工单列表失败')
  } finally {
    loading.value = false
  }
}

// 分页变更
function onPageChange(info: PageInfo) {
  page.value = info.current
  pageSize.value = info.pageSize
  loadTickets()
}

// 取消工单（仅待处理可取消）
async function handleCancel(row: TicketInfo) {
  try {
    await cancelMyTicket(row.id)
    MessagePlugin.success('工单已取消')
    loadTickets()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '取消失败')
  }
}

function goCreate() {
  router.push('/support/tickets/create')
}

function goDetail(id: number) {
  router.push(`/support/tickets/${id}`)
}

onMounted(loadTickets)
</script>

<style scoped>
.tickets-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* 页头横幅 */
.tickets-hero {
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

/* 列表面板 */
.tickets-panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.panel-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 768px) {
  .tickets-hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
