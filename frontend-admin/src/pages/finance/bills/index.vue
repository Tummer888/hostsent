<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">账单管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadBills">
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
          <span class="field__label">账期</span>
          <t-input v-model="filters.period" placeholder="如 2026-08" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="billStatusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">账单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个账单</span>
      </div>
      <t-table
        row-key="id"
        :data="billList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #bill_no="{ row }">
          <span class="cell-strong">{{ row.bill_no }}</span>
        </template>

        <template #total_amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.total_amount) }}</span>
        </template>

        <template #refund_amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.refund_amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="billStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ billStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link v-if="row.status === 'unpaid'" theme="warning" hover="color" @click="handleClose(row)">关账</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无账单" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon, SearchIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { closeBill, getBillList } from '@/api/finance'
import {
  billStatusLabel,
  billStatusOptions,
  billStatusTheme,
  formatPrice,
  formatTime,
} from '@/pages/finance/constants'
import type { BillInfo } from '@/types/interface'

defineOptions({ name: 'FinanceBills' })

const billList = ref<BillInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  user_id: string | undefined
  period: string | undefined
  status: string | undefined
}>({
  user_id: undefined,
  period: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'bill_no', title: '账单号', minWidth: 180 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'period', title: '账期', width: 110 },
  { colKey: 'total_amount', title: '消费金额', width: 130 },
  { colKey: 'refund_amount', title: '退款金额', width: 130 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: 120,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadBills() {
  loading.value = true
  try {
    const data = await getBillList({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      period: filters.period,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    billList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载账单失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadBills()
}

function handleSearch() {
  pagination.current = 1
  loadBills()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.period = undefined
  filters.status = undefined
  pagination.current = 1
  loadBills()
}

function handleClose(row: BillInfo) {
  const dialog = DialogPlugin.confirm({
    header: '关账确认',
    body: `确认将账单「${row.bill_no}」（账期 ${row.period}）关账吗？关账后不再允许修改。`,
    confirmBtn: { content: '确认关账', theme: 'warning' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        await closeBill(row.id)
        MessagePlugin.success('账单已关账')
        dialog.destroy()
        loadBills()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '关账失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadBills)
</script>

<style lang="css">
@import '../shared.css';
</style>
