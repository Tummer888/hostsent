<template>
  <div class="page-body point-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <HistoryIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">积分流水</h2>
          <p class="page-header__desc">
            只增不改的积分账本：每条流水关联来源业务单号，同一来源只发放一次；到期回收与兑换消耗在此留痕。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadTransactions">
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
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">流水类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="pointTxTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">方向</span>
          <t-select v-model="filters.direction" clearable placeholder="全部方向" :options="pointDirectionOptions" />
        </div>
        <div class="field">
          <span class="field__label">开始时间</span>
          <t-date-picker v-model="filters.start_time" enable-time-picker allow-input clearable placeholder="起始时间" />
        </div>
        <div class="field">
          <span class="field__label">结束时间</span>
          <t-date-picker v-model="filters.end_time" enable-time-picker allow-input clearable placeholder="结束时间" />
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
        <h3 class="card-title">流水列表</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="txList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #user="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">ID {{ row.user_id }}</span>
          </div>
        </template>

        <template #type="{ row }">
          <t-tag :theme="pointTxTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ pointTxTypeLabel(row.type) }}
          </t-tag>
        </template>

        <template #points="{ row }">
          <span :class="row.direction >= 0 ? 'amount-income' : 'amount-expense'">
            {{ row.direction >= 0 ? '+' : '-' }}{{ formatPoints(row.points) }}
          </span>
        </template>

        <template #balance_after="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ formatPoints(row.balance_after) }}</span>
            <span class="price-sub">变更前 {{ formatPoints(row.balance_before) }}</span>
          </div>
        </template>

        <template #ref_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.ref_no || '—' }}</span>
            <span class="price-sub">{{ row.biz_type || row.tx_no }}</span>
          </div>
        </template>

        <template #remark="{ row }">
          <span class="price-sub">{{ row.remark || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无积分流水" />
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

import { HistoryIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getPointTransactions } from '@/api/point'
import {
  formatPoints,
  formatTime,
  pointDirectionOptions,
  pointTxTypeLabel,
  pointTxTypeOptions,
  pointTxTypeTheme,
} from '@/pages/points/constants'
import type { PointTransactionInfo } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'PointsTransactions' })

const router = useRouter()
const txList = ref<PointTransactionInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{
  user_id: string | undefined
  type: string | undefined
  direction: number | undefined
  start_time: string | undefined
  end_time: string | undefined
}>({
  user_id: undefined,
  type: undefined,
  direction: undefined,
  start_time: undefined,
  end_time: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<PointTransactionInfo>[] = [
  { colKey: 'user', title: '用户', minWidth: 150 },
  { colKey: 'type', title: '类型', width: 110 },
  { colKey: 'points', title: '积分变动', width: 120 },
  { colKey: 'balance_after', title: '变动后积分', width: 150 },
  { colKey: 'ref_no', title: '来源', minWidth: 190 },
  { colKey: 'remark', title: '备注', minWidth: 160 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadTransactions() {
  loading.value = true
  try {
    const data = await getPointTransactions({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      type: filters.type,
      direction: filters.direction,
      start_time: filters.start_time,
      end_time: filters.end_time,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    txList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载积分流水失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadTransactions()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

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
  loadTransactions()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.type = undefined
  filters.direction = undefined
  filters.start_time = undefined
  filters.end_time = undefined
  pagination.current = 1
  loadTransactions()
}

onMounted(() => {
  // 从积分账户页「查看流水」跳转时携带 user_id，预填后直接查询。
  const userId = router.currentRoute.value.query.user_id
  if (typeof userId === 'string' && userId) filters.user_id = userId
  loadTransactions()
})
</script>

<style lang="css">
@import '../shared.css';
</style>
