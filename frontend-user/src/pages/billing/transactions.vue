<template>
  <div class="tx-page">
    <section class="panel">
      <div class="panel-head">
        <h3 class="section-title">资金流水</h3>
        <t-space size="small">
          <t-button variant="outline" size="small" :loading="loading" @click="loadTransactions">
            <template #icon><RefreshIcon /></template>
            刷新
          </t-button>
        </t-space>
      </div>

      <div class="filter-bar">
        <t-select v-model="filter.type" clearable placeholder="全部类型" :options="txTypeOptions" style="width: 140px" @change="handleSearch" />
        <t-select v-model="filter.direction" clearable placeholder="全部方向" :options="directionOptions" style="width: 120px" @change="handleSearch" />
        <t-button theme="primary" size="small" @click="handleSearch">查询</t-button>
        <t-button variant="outline" size="small" @click="handleReset">重置</t-button>
      </div>

      <t-table
        :data="transactions"
        :columns="columns"
        size="small"
        row-key="id"
        :pagination="pagination"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="loading"
        @page-change="handlePageChange"
      >
        <template #tx_no="{ row }">
          <span class="cell-strong">{{ row.tx_no }}</span>
        </template>
        <template #type="{ row }">
          <t-tag :theme="txTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ txTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #direction="{ row }">
          <span>{{ directionLabel(row.direction) }}</span>
        </template>
        <template #amount="{ row }">
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">{{ formatAmount(row.amount, row.direction) }}</span>
        </template>
        <template #balance="{ row }">
          <span class="time-text">¥ {{ formatPrice(row.balance_after) }}</span>
        </template>
        <template #ref_no="{ row }">
          <span class="time-text">{{ row.ref_no || '—' }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无流水记录" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getMyTransactions, type TransactionInfo } from '@/api/finance'
import {
  directionLabel,
  directionOptions,
  formatAmount,
  formatPrice,
  formatTime,
  txTypeLabel,
  txTypeOptions,
  txTypeTheme,
} from '@/pages/billing/constants'

defineOptions({ name: 'BillingTransactions' })

const transactions = ref<TransactionInfo[]>([])
const loading = ref(false)

const filter = reactive<{ type: string | undefined; direction: number | undefined }>({ type: undefined, direction: undefined })

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const columns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', width: 200 },
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'direction', title: '方向', width: 70 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'balance', title: '余额', width: 100 },
  { colKey: 'ref_no', title: '关联单号', ellipsis: true },
  { colKey: 'remark', title: '备注', ellipsis: true },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadTransactions() {
  loading.value = true
  try {
    const { data } = await getMyTransactions({
      type: filter.type,
      direction: filter.direction,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    if (data) {
      transactions.value = data.items
      pagination.total = data.meta.total
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载流水失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTransactions()
}

function handleSearch() {
  pagination.current = 1
  loadTransactions()
}

function handleReset() {
  filter.type = undefined
  filter.direction = undefined
  pagination.current = 1
  loadTransactions()
}

onMounted(loadTransactions)
</script>

<style scoped>
.tx-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0 16px;
  flex-wrap: wrap;
}

.cell-strong {
  font-weight: 600;
  color: #334155;
}

.amount-income {
  color: #059669;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.amount-expense {
  color: #dc2626;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
</style>
