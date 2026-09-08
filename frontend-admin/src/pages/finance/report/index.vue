<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><ChartBarIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">财务报表</h2>
          <p class="page-header__desc">应收/实收/未付账单与收支汇总，辅助财务分析。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">刷新</t-button>
      </t-space>
    </header>

    <section class="wallet-stat-grid">
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><FileIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(totalAmount) }}</span>
          <span class="stat-card__label">应收总额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><CheckCircleIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(refundAmount) }}</span>
          <span class="stat-card__label">退款总额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><ErrorCircleIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(netAmount) }}</span>
          <span class="stat-card__label">净应收</span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">账单明细</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table row-key="id" :data="items" :columns="columns" :loading="loading" size="small" hover bordered cell-empty-content="—" :pagination="pagination" @page-change="onPageChange">
        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">{{ statusLabel(row.status) }}</t-tag>
        </template>
        <template #empty><t-empty description="暂无账单数据" /></template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ChartBarIcon, CheckCircleIcon, ErrorCircleIcon, FileIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getBillList } from '@/api/finance'
import { formatPrice } from '@/pages/finance/constants'
import type { BillInfo } from '@/types/interface'

defineOptions({ name: 'FinanceReport' })

const items = ref<BillInfo[]>([])
const loading = ref(false)
const total = ref(0)
const totalAmount = ref(0)
const refundAmount = ref(0)
const netAmount = ref(0)

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const columns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'bill_no', title: '账单号', minWidth: 150 },
  { colKey: 'period', title: '周期', width: 100 },
  { colKey: 'total_amount', title: '金额', width: 110 },
  { colKey: 'refund_amount', title: '退款', width: 110 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '生成时间', width: 170 },
]

function statusLabel(v: string): string {
  return { unpaid: '未支付', paid: '已支付', overdue: '逾期' }[v] || v
}
function statusTheme(v: string): string {
  return { paid: 'success', overdue: 'warning' }[v] || 'default'
}

async function load() {
  loading.value = true
  try {
    const data = await getBillList({ page: 1, page_size: 100 })
    items.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    let ta = 0
    let rf = 0
    for (const it of data.items) {
      ta += it.total_amount
      rf += it.refund_amount
    }
    totalAmount.value = ta
    refundAmount.value = rf
    netAmount.value = ta - rf
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载失败')
    items.value = []
  } finally {
    loading.value = false
  }
}

function onPageChange(info: PageInfo) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  load()
}

onMounted(load)
</script>

<style lang="css">
@import '../shared.css';
</style>
