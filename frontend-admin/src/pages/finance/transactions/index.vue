<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">资金流水</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadTransactions">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">流水类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="txTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">收支方向</span>
          <t-select v-model="filters.direction" clearable placeholder="全部方向" :options="directionOptions" />
        </div>
        <div class="field">
          <span class="field__label">发生时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">流水列表</h3>
        <span class="table-card__meta">共 {{ total }} 条流水</span>
      </div>
      <t-table
        row-key="id"
        :data="transactionList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
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

        <template #amount="{ row }">
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">
            {{ formatAmount(row.amount, row.direction) }}
          </span>
        </template>

        <template #balance="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.balance_after) }}</span>
            <span class="price-sub">前 ¥{{ formatPrice(row.balance_before) }}</span>
          </div>
        </template>

        <template #direction="{ row }">
          <span>{{ directionLabel(row.direction) }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无资金流水" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getTransactionList } from '@/api/finance'
import {
  directionLabel,
  directionOptions,
  formatAmount,
  formatPrice,
  formatTime,
  toDateString,
  txTypeLabel,
  txTypeOptions,
  txTypeTheme,
} from '@/pages/finance/constants'
import type { TransactionInfo } from '@/types/interface'

defineOptions({ name: 'FinanceTransactions' })

const transactionList = ref<TransactionInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  user_id: string | undefined
  type: string | undefined
  direction: number | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  user_id: undefined,
  type: undefined,
  direction: undefined,
  dateRange: [],
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', minWidth: 180 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'direction', title: '方向', width: 90 },
  { colKey: 'amount', title: '变动金额', width: 130 },
  { colKey: 'balance', title: '变动后余额', width: 150 },
  { colKey: 'ref_no', title: '关联单号', minWidth: 160 },
  { colKey: 'remark', title: '备注', minWidth: 140 },
  { colKey: 'created_at', title: '发生时间', width: 170 },
]

async function loadTransactions() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getTransactionList({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      type: filters.type,
      direction: filters.direction,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    transactionList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载资金流水失败')
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

function handleResetFilters() {
  filters.user_id = undefined
  filters.type = undefined
  filters.direction = undefined
  filters.dateRange = []
  pagination.current = 1
  loadTransactions()
}

onMounted(loadTransactions)
</script>

<style lang="css">
@import '../shared.css';
</style>
