<template>
  <div class="page-body console-module tx-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SwapIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">资金流水</h2>
          <p class="page-header__desc">余额的每一笔增减都在这里，金额正负号与方向列一致</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/billing')">返回费用中心</t-button>
        <t-button variant="outline" :loading="loading" @click="loadTransactions">刷新</t-button>
      </div>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__grid">
        <div class="field">
          <label class="field__label">流水类型</label>
          <t-select v-model="filter.type" clearable placeholder="全部类型" :options="txTypeOptions" @change="handleSearch" />
        </div>
        <div class="field">
          <label class="field__label">收支方向</label>
          <t-select v-model="filter.direction" clearable placeholder="全部方向" :options="directionOptions" @change="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-button variant="outline" @click="handleReset">重置</t-button>
        <t-button theme="primary" :loading="loading" @click="handleSearch">查询</t-button>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">流水明细</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 条</span>
      </div>

      <t-table
        :data="transactions"
        :columns="columns"
        row-key="id"
        :pagination="isMobile ? undefined : pagination"
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
          <span :class="row.direction > 0 ? 'amount-income' : 'amount-expense'">
            {{ formatAmount(row.amount, row.direction) }}
          </span>
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

      <!-- 移动端翻页：与 admin 列表页同一套（桌面用表格内建分页） -->
      <MobilePagination
        v-if="isMobile"
        :current="pagination.current"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        @change="handlePageChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { SwapIcon } from 'tdesign-icons-vue-next'
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
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'BillingTransactions' })

const router = useRouter()
const { isMobile } = useIsMobile()

const transactions = ref<TransactionInfo[]>([])
const loading = ref(false)

const filter = reactive<{ type: string | undefined; direction: number | undefined }>({
  type: undefined,
  direction: undefined,
})

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
/* amount-income / amount-expense / time-text 由 console-module 骨架契约提供 */
</style>
