<template>
  <div class="point-page">
    <PointNav />

    <div class="point-header">
      <h2 class="point-title">积分明细</h2>
      <t-button variant="outline" size="small" :loading="loading" @click="loadTransactions">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </div>

    <t-alert theme="info" class="point-alert">
      积分独立于余额账本：以下流水只反映积分的获得与消耗，与资金流水无关。
    </t-alert>

    <section class="point-panel">
      <div class="filter-bar">
        <t-select v-model="filter.type" clearable placeholder="全部类型" :options="pointTxTypeOptions" style="width: 150px" @change="handleSearch" />
        <t-select v-model="filter.direction" clearable placeholder="全部方向" :options="pointDirectionOptions" style="width: 120px" @change="handleSearch" />
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
          <t-tag :theme="pointTxTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ pointTxTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #points="{ row }">
          <span :class="row.direction > 0 ? 'point-earn' : 'point-spend'">
            {{ formatPoints(row.direction > 0 ? row.points : -row.points, true) }}
          </span>
        </template>
        <template #balance="{ row }">
          <span class="time-text">{{ formatPoints(row.balance_after) }}</span>
        </template>
        <template #ref_no="{ row }">
          <span class="time-text">{{ row.ref_no || '—' }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无积分流水" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getMyPointTransactions, type PointTransactionInfo } from '@/api/point'
import {
  formatPoints,
  formatTime,
  pointDirectionOptions,
  pointTxTypeLabel,
  pointTxTypeOptions,
  pointTxTypeTheme,
} from '@/pages/points/constants'
import PointNav from '@/components/point-nav/index.vue'

defineOptions({ name: 'MyPointTransactions' })

const transactions = ref<PointTransactionInfo[]>([])
const loading = ref(false)

const filter = reactive<{ type: string | undefined; direction: number | undefined }>({
  type: undefined,
  direction: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const columns: PrimaryTableCol<PointTransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', width: 200 },
  { colKey: 'type', title: '类型', width: 110 },
  { colKey: 'points', title: '积分变动', width: 120, align: 'right' },
  { colKey: 'balance', title: '变动后余额', width: 120, align: 'right' },
  { colKey: 'ref_no', title: '关联单号', minWidth: 170, ellipsis: true },
  { colKey: 'remark', title: '备注', minWidth: 160, ellipsis: true },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadTransactions() {
  loading.value = true
  try {
    const { data } = await getMyPointTransactions({
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
    MessagePlugin.error((error as Error)?.message || '加载积分流水失败')
  } finally {
    loading.value = false
  }
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

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTransactions()
}

onMounted(loadTransactions)
</script>

<style scoped src="./shared.css"></style>
<style scoped>
.point-earn {
  color: #16a34a;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.point-spend {
  color: #d97706;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.cell-strong {
  font-weight: 600;
  color: #1e293b;
}
</style>
