<template>
  <div class="agent-page">
    <AgentNav />

    <div class="agent-header">
      <h2 class="agent-title">结算记录</h2>
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
      <template #settlement_no="{ row }">
        <div class="cell-strong">{{ row.settlement_no }}</div>
        <div class="cell-sub">{{ formatTime(row.period_start) }} ~ {{ formatTime(row.period_end) }}</div>
      </template>
      <template #commission_total="{ row }">¥{{ (row.commission_total || 0).toFixed(2) }}</template>
      <template #deduction_total="{ row }">¥{{ (row.deduction_total || 0).toFixed(2) }}</template>
      <template #payable_total="{ row }">
        <span class="cell-money">¥{{ (row.payable_total || 0).toFixed(2) }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
          {{ statusText(row.status) }}
        </t-tag>
      </template>
      <template #paid_at="{ row }">{{ formatTime(row.paid_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !items.length" description="暂无结算记录" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getAgentSettlements, type SettlementInfo } from '@/api/agent'
import AgentNav from '@/components/agent-nav/index.vue'

defineOptions({ name: 'AgentSettlements' })

const items = ref<SettlementInfo[]>([])
const loading = ref(false)
const status = ref('')
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const statusOptions = [
  { label: '待结算', value: 'pending' },
  { label: '已发放', value: 'paid' },
  { label: '已驳回', value: 'rejected' },
]

const columns: PrimaryTableCol<SettlementInfo>[] = [
  { colKey: 'settlement_no', title: '结算单', minWidth: 220 },
  { colKey: 'commission_total', title: '佣金合计', width: 130 },
  { colKey: 'deduction_total', title: '扣减', width: 120 },
  { colKey: 'payable_total', title: '应发金额', width: 130 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'paid_at', title: '发放时间', width: 170 },
]

function statusText(s: string): string {
  return { pending: '待结算', paid: '已发放', rejected: '已驳回' }[s] || s || '—'
}

function statusTheme(s: string): 'success' | 'warning' | 'danger' | 'default' {
  if (s === 'paid') return 'success'
  if (s === 'pending') return 'warning'
  if (s === 'rejected') return 'danger'
  return 'default'
}

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  loading.value = true
  try {
    const { data } = await getAgentSettlements({
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
