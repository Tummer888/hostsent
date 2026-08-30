<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">退款管理</h2>
          <p class="page-header__desc">审核与跟踪所有退款单，支持通过 / 驳回操作。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadRefunds">
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
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="退款单号 / 原因" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">关联订单 ID</span>
          <t-input v-model="filters.order_id" placeholder="精确匹配订单 ID" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="refundStatusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">退款单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个退款单</span>
      </div>
      <t-table
        row-key="id"
        :data="refundList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #refund_no="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.refund_no }}</t-link>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="refundStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ refundStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
            <t-link v-if="row.status === 'pending'" theme="warning" hover="color" @click="openDetail(row)">审核</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无退款单" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getRefundList } from '@/api/order'
import {
  formatPrice,
  formatTime,
  refundStatusLabel,
  refundStatusOptions,
  refundStatusTheme,
} from '@/pages/order/constants'
import type { RefundInfo } from '@/types/interface'

defineOptions({ name: 'OrderRefunds' })

const router = useRouter()

const refundList = ref<RefundInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  keyword: string | undefined
  order_id: string | undefined
  status: string | undefined
}>({
  keyword: undefined,
  order_id: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<RefundInfo>[] = [
  { colKey: 'refund_no', title: '退款单号', minWidth: 180 },
  { colKey: 'order_no', title: '关联订单', minWidth: 170 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'reason', title: '原因', minWidth: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'audited_at', title: '审核时间', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: 130,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadRefunds() {
  loading.value = true
  try {
    const data = await getRefundList({
      keyword: filters.keyword,
      order_id: filters.order_id ? Number(filters.order_id) : undefined,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    refundList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载退款单列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadRefunds()
}

function handleSearch() {
  pagination.current = 1
  loadRefunds()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.order_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadRefunds()
}

function openDetail(row: RefundInfo) {
  router.push(`/orders/refunds/${row.id}`)
}

onMounted(loadRefunds)
</script>

<style lang="css">
@import '../shared.css';
</style>
