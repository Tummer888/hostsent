<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <div class="page-header__text">
          <h2 class="page-header__title">财务总览</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">刷新</t-button>
      </t-space>
    </header>

    <section class="wallet-stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(totalIncome) }}</span>
          <span class="stat-card__label">收入总额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--red">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(totalExpense) }}</span>
          <span class="stat-card__label">支出总额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><ChartBarIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(netAmount) }}</span>
          <span class="stat-card__label">净额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><FileIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ totalCount }}</span>
          <span class="stat-card__label">交易笔数</span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近交易</h3>
        <span class="table-card__meta">近 20 笔</span>
      </div>
      <t-table row-key="id" :data="items" :columns="columns" :loading="loading" size="small" hover cell-empty-content="—">
        <template #amount="{ row }">
          <span :class="row.amount >= 0 ? 'tx-in' : 'tx-out'">{{ row.amount >= 0 ? '+' : '' }}¥{{ formatPrice(Math.abs(row.amount)) }}</span>
        </template>
        <template #type="{ row }"><span>{{ txTypeLabel(row.type) }}</span></template>
        <template #empty><t-empty description="暂无交易流水" /></template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ChartBarIcon, FileIcon, MoneyIcon } from 'tdesign-icons-vue-next'
import { type PrimaryTableCol } from 'tdesign-vue-next'

import { getTransactionList } from '@/api/finance'
import { formatPrice, txTypeLabel } from '@/pages/finance/constants'
import type { TransactionInfo } from '@/types/interface'

defineOptions({ name: 'FinanceOverview' })

const items = ref<TransactionInfo[]>([])
const loading = ref(false)
const totalIncome = ref(0)
const totalExpense = ref(0)
const netAmount = ref(0)
const totalCount = ref(0)

const columns: PrimaryTableCol<TransactionInfo>[] = [
  { colKey: 'tx_no', title: '流水号', minWidth: 150 },
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'balance_after', title: '余额', width: 110 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function load() {
  loading.value = true
  try {
    const data = await getTransactionList({ page: 1, page_size: 100 })
    items.value = data.items
    totalCount.value = data.meta.total
    let inc = 0
    let exp = 0
    for (const it of data.items) {
      if (it.amount >= 0) inc += it.amount
      else exp += -it.amount
    }
    totalIncome.value = inc
    totalExpense.value = exp
    netAmount.value = inc - exp
  } catch (error) {
    console.error(error)
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style lang="css">
@import '../shared.css';

.finance-module .tx-in {
  color: #2ba471;
  font-weight: 600;
}
.finance-module .tx-out {
  color: #e34d59;
  font-weight: 600;
}
</style>
