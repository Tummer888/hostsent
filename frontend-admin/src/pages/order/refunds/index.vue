<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">退款管理</h2>
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
      <div class="filter-card__actions">
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
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
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
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
                { content: '审核', value: 'audit', hidden: () => !(row.status === 'pending'), theme: 'warning' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="row.status === 'pending'" theme="warning" hover="color" @click="openDetail(row)">审核</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无退款单" />
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
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'OrderRefunds' })

const router = useRouter()

const refundList = ref<RefundInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
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

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers/loadData）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
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
    width: isMobile.value ? 70 : 130,
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
    mobilePage.total = data.meta.total
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

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: RefundInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'audit':
      openDetail(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>
