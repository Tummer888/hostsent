<template>
  <div class="page-body referral-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">返现台账</h2>
          <p class="page-header__desc">订单完成计提、退款按比例冲减、提现冻结与转入余额的完整流水</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadCashbacks">
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
          <span class="field__label">返现归属人 ID</span>
          <t-input v-model="filters.user_id" placeholder="按邀请人用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">台账类型</span>
          <t-select v-model="filters.type" clearable placeholder="全部类型" :options="referralTxTypeOptions" />
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
        <h3 class="card-title">台账流水</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="cashbacks"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #type="{ row }">
          <t-tag :theme="referralTxTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ referralTxTypeLabel(row.type) }}
          </t-tag>
        </template>

        <template #amount="{ row }">
          <span :class="row.direction >= 0 ? 'amount-income' : 'amount-expense'">
            {{ row.direction >= 0 ? '+' : '-' }}¥{{ formatPrice(row.amount) }}
          </span>
        </template>

        <template #balance_after="{ row }">
          <span class="price-main">¥{{ formatPrice(row.balance_after) }}</span>
        </template>

        <template #inviter="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.inviter_username || '—' }}</span>
            <span class="price-sub">ID {{ row.inviter_user_id || '—' }}</span>
          </div>
        </template>

        <template #invitee="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.invitee_username || '—' }}</span>
            <span class="price-sub">ID {{ row.invitee_user_id || '—' }}</span>
          </div>
        </template>

        <template #ref_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.order_no || row.ref_no || '—' }}</span>
            <span class="price-sub">{{ row.remark || row.tx_no }}</span>
          </div>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无返现记录" />
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
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getReferralCashbacks } from '@/api/referral'
import {
  formatPrice,
  formatTime,
  referralTxTypeLabel,
  referralTxTypeOptions,
  referralTxTypeTheme,
} from '@/pages/referral/constants'
import type { ReferralCashbackInfo } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ReferralCashbacks' })

const cashbacks = ref<ReferralCashbackInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{ user_id: string | undefined; type: string | undefined }>({
  user_id: undefined,
  type: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<ReferralCashbackInfo>[] = [
  { colKey: 'type', title: '类型', width: 110 },
  { colKey: 'inviter', title: '返现归属人', minWidth: 150 },
  { colKey: 'invitee', title: '被邀请人', minWidth: 150 },
  { colKey: 'ref_no', title: '来源单号', minWidth: 200 },
  { colKey: 'amount', title: '变动金额', width: 130 },
  { colKey: 'balance_after', title: '变动后余额', width: 130 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadCashbacks() {
  loading.value = true
  try {
    const data = await getReferralCashbacks({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      type: filters.type,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    cashbacks.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载返现台账失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadCashbacks()
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
  loadCashbacks()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.type = undefined
  pagination.current = 1
  loadCashbacks()
}

onMounted(loadCashbacks)
</script>

<style lang="css">
@import '../shared.css';
</style>
