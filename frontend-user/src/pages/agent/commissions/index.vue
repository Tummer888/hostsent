<template>
  <div class="agent-page">
    <AgentNav />

    <div class="agent-header">
      <h2 class="agent-title">佣金明细</h2>
      <t-select v-model="status" class="agent-filter" placeholder="全部状态" clearable :options="statusOptions" @change="reload" />
    </div>

    <t-table
      :data="items"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :pagination="pagination"
      cell-empty-content="—"
      @page-change="onPageChange"
    >
      <template #order_no="{ row }">
        <div class="cell-strong">{{ row.order_no || '—' }}</div>
        <div class="cell-sub">{{ typeText(row.commission_type) }} · {{ sourceText(row.source_type) }}</div>
      </template>
      <template #base_amount="{ row }">¥{{ (row.base_amount || 0).toFixed(2) }}</template>
      <template #rate="{ row }">{{ ((row.rate || 0) * 100).toFixed(2) }}%</template>
      <template #amount="{ row }">
        <span class="cell-money">¥{{ (row.amount || 0).toFixed(2) }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
          {{ statusText(row.status) }}
        </t-tag>
      </template>
      <template #created_at="{ row }">{{ formatTime(row.created_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !items.length" description="暂无佣金记录" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getAgentCommissions, type CommissionInfo } from '@/api/agent'
import AgentNav from '@/components/agent-nav/index.vue'

defineOptions({ name: 'AgentCommissions' })

const items = ref<CommissionInfo[]>([])
const loading = ref(false)
const status = ref('')
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const statusOptions = [
  { label: '待结算', value: 'pending' },
  { label: '已结算', value: 'settled' },
  { label: '已取消', value: 'cancelled' },
]

const columns: PrimaryTableCol<CommissionInfo>[] = [
  { colKey: 'order_no', title: '订单', minWidth: 180 },
  { colKey: 'base_amount', title: '计佣基数', width: 130 },
  { colKey: 'rate', title: '佣金率', width: 100 },
  { colKey: 'amount', title: '佣金金额', width: 130 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '产生时间', width: 170 },
]

function typeText(t: string): string {
  return { direct: '直属佣金', indirect: '间接佣金', self: '自购返佣' }[t] || '佣金'
}

function sourceText(s: string): string {
  return { order: '新购订单', renewal: '续费订单' }[s] || s || '订单'
}

function statusText(s: string): string {
  return { pending: '待结算', settled: '已结算', cancelled: '已取消' }[s] || s || '—'
}

function statusTheme(s: string): 'success' | 'warning' | 'danger' | 'default' {
  if (s === 'settled') return 'success'
  if (s === 'pending') return 'warning'
  if (s === 'cancelled') return 'danger'
  return 'default'
}

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  loading.value = true
  try {
    const { data } = await getAgentCommissions({
      page: pagination.current,
      page_size: pagination.pageSize,
      status: status.value || undefined,
    })
    items.value = data?.items || []
    pagination.total = data?.total || 0
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  load()
}

onMounted(load)
</script>

<style scoped>
.agent-page { padding: 16px 24px; }
.agent-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.agent-title { font-size: 20px; font-weight: 700; margin: 0; }
.agent-filter { width: 160px; }
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-money { color: #e37318; font-weight: 700; }
</style>
