<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="工单摘要采集失败，下方为实时查询结果；若同样为空请稍后重试。"
    />

    <div class="table-card__head">
      <h3 class="card-title">服务工单</h3>
      <span class="table-card__meta">
        共 {{ pagination.total }} 条<template v-if="summary"> · 处理中 {{ summary.open_ticket_count }} 条</template>
      </span>
    </div>

    <t-table
      row-key="id"
      :data="items"
      :columns="columns"
      :loading="loading"
      size="small"
      hover
      table-layout="fixed"
      cell-empty-content="—"
      :pagination="isMobile ? undefined : pagination"
      @page-change="onPageChange"
    >
      <template #ticket_no="{ row }">
        <span class="cell-mono">{{ row.ticket_no }}</span>
      </template>
      <template #title="{ row }">
        <span class="cell-strong">{{ row.title || '—' }}</span>
      </template>
      <template #category="{ row }">
        <span>{{ row.category_name || row.category || '—' }}</span>
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
        <span>{{ row.assigned_name || '未指派' }}</span>
      </template>
      <template #updated_at="{ row }">
        <span class="time-text">{{ formatDateTime(row.updated_at) }}</span>
      </template>
      <template #op="{ row }">
        <t-link theme="primary" hover="color" @click="goDetail(row.id)">详情</t-link>
      </template>
      <template #empty>
        <t-empty description="该用户暂无服务工单" />
      </template>
    </t-table>

    <MobilePagination
      v-if="isMobile"
      :current="mobilePage.current"
      :page-size="mobilePage.pageSize"
      :total="mobilePage.total"
      @go="goMobilePage"
      @page-size="onMobileSize"
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import { getTickets } from '@/api/ticket'
import {
  formatDateTime,
  ticketPriorityLabel,
  ticketPriorityTheme,
  ticketStatusLabel,
  ticketStatusTheme,
} from '@/pages/users/constants'
import type { UserDetailSummary } from '@/api/user'
import type { TicketInfo } from '@/types/interface'

const props = defineProps<{
  userId: number
  isMobile: boolean
  summary?: UserDetailSummary | null
  degraded?: boolean
}>()

const router = useRouter()
const items = ref<TicketInfo[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<TicketInfo>[] = [
  { colKey: 'ticket_no', title: '工单号', width: 190 },
  { colKey: 'title', title: '标题', minWidth: 200 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 120 },
  { colKey: 'updated_at', title: '最近更新', width: 150 },
  { colKey: 'op', title: '操作', width: 80, fixed: 'right' },
]

async function load() {
  if (!props.userId) return
  loading.value = true
  try {
    const data = await getTickets({
      user_id: props.userId,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    items.value = data.items || []
    pagination.total = data.meta?.total || 0
    mobilePage.total = pagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载工单失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  Object.assign(mobilePage, { current: pageInfo.current, pageSize: pageInfo.pageSize })
  void load()
}

function goMobilePage(target: number) {
  pagination.current = target
  mobilePage.current = target
  void load()
}

function onMobileSize(size: number) {
  pagination.pageSize = size
  pagination.current = 1
  mobilePage.pageSize = size
  mobilePage.current = 1
  void load()
}

function goDetail(id: number) {
  router.push({ path: `/tickets/detail/${id}` })
}

watch(() => props.userId, () => {
  pagination.current = 1
  mobilePage.current = 1
  void load()
}, { immediate: true })

defineExpose({ reload: load })
</script>