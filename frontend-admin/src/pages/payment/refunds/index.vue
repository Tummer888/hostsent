<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RefreshIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">渠道退款</h2>
          <p class="page-header__desc">订单退款审核通过后原路退回渠道；无渠道支付记录（余额支付）时回补钱包余额，二者均在此留痕。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadRefunds">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
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
          <span class="field__label">支付单号</span>
          <t-input v-model="filters.payment_no" placeholder="如 P20260912…" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="refundStatusOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">退款单列表</h3>
        <span class="table-card__meta">共 {{ total }} 笔</span>
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
          <div class="price-cell">
            <span class="cell-strong">{{ row.refund_no }}</span>
            <span class="price-sub">{{ row.payment_no }}</span>
          </div>
        </template>

        <template #order_refund_no="{ row }">
          <span class="cell-muted">{{ row.order_refund_no || '余额回补' }}</span>
        </template>

        <template #user_id="{ row }">
          <span class="cell-muted">#{{ row.user_id }}</span>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="refundStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ refundStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #reason="{ row }">
          <span class="cell-muted" :title="row.fail_reason || row.reason">{{ row.reason || row.fail_reason || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
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

import { RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getRefunds } from '@/api/payment'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatPrice, formatTime, refundStatusLabel, refundStatusOptions, refundStatusTheme } from '@/pages/payment/constants'
import type { PaymentRefundInfo } from '@/types/interface'

defineOptions({ name: 'PaymentRefunds' })

const refundList = ref<PaymentRefundInfo[]>([])
const loading = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const filters = reactive<{ payment_no?: string; user_id?: string; status?: string }>({
  payment_no: undefined,
  user_id: undefined,
  status: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<PaymentRefundInfo>[] = [
  { colKey: 'refund_no', title: '退款单 / 支付单', minWidth: 210 },
  { colKey: 'order_refund_no', title: '关联订单退款', minWidth: 150 },
  { colKey: 'user_id', title: '用户', width: 90, align: 'center' as const },
  { colKey: 'amount', title: '退款金额', width: 120 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'reason', title: '原因 / 失败原因', minWidth: 180 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
]

async function loadRefunds() {
  loading.value = true
  try {
    const data = await getRefunds({
      payment_no: filters.payment_no,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    refundList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载退款单失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadRefunds()
}

function handleResetFilters() {
  filters.payment_no = undefined
  filters.user_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadRefunds()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadRefunds()
}

function goMobilePage(target: number) {
  const maxPage = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === mobilePage.current) return
  handlePageChange({ current: clamped, pageSize: mobilePage.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  handlePageChange({ current: 1, pageSize } as PageInfo)
}

onMounted(loadRefunds)
</script>

<style lang="css">
@import '../shared.css';
</style>
