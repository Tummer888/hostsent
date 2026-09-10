<template>
  <div class="order-page">
    <div class="order-header">
      <h2 class="order-title">我的订单</h2>
    </div>

    <t-table :data="orders" :columns="columns" :loading="loading" row-key="id" :pagination="pagination" cell-empty-content="—" @page-change="onPageChange">
      <template #product_name="{ row }">
        <div class="cell-strong">{{ row.product_name }}</div>
        <div class="cell-sub">{{ row.order_no }}</div>
      </template>
      <template #amount="{ row }">
        <span class="cell-amount">¥{{ payAmount(row).toFixed(2) }}</span>
        <div class="cell-sub cell-original" v-if="row.discount_amount > 0">原价 ¥{{ row.original_amount.toFixed(2) }}</div>
      </template>
      <template #discount="{ row }">
        <template v-if="row.discount_amount > 0">
          <span class="cell-discount">-¥{{ row.discount_amount.toFixed(2) }}</span>
          <t-tag size="small" variant="light" :theme="row.discount_source === 'agent' ? 'warning' : 'success'">
            {{ sourceLabel(row.discount_source) }}
          </t-tag>
        </template>
        <span v-else>—</span>
      </template>
      <template #actor_name="{ row }">
        <span :class="{ 'cell-sub': !row.actor_name }">{{ row.actor_name || '主账号' }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">{{ statusText(row.status) }}</t-tag>
      </template>
      <template #created_at="{ row }">{{ row.created_at?.replace('T', ' ').slice(0, 16) || '—' }}</template>
    </t-table>

    <t-empty v-if="!loading && !orders.length" description="暂无订单" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getMyOrders, type OrderInfo } from '@/api/shop'

defineOptions({ name: 'OrderList' })

const orders = ref<OrderInfo[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<OrderInfo>[] = [
  { colKey: 'product_name', title: '产品', minWidth: 160 },
  { colKey: 'amount', title: '实付金额', width: 140 },
  { colKey: 'discount', title: '优惠', width: 160 },
  { colKey: 'actor_name', title: '操作人', width: 120 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '下单时间', width: 180 },
]

/** 实付优先取算价快照，兼容未写快照的存量订单。 */
function payAmount(row: OrderInfo): number {
  return row.final_amount || row.paid_amount
}

/** 折扣来源中文标签（P5-06）。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '代理价',
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

function statusText(s: string): string {
  return (
    {
      pending: '待支付',
      paid: '已支付',
      provisioning: '开通中',
      active: '服务中',
      cancelled: '已取消',
      refunded: '已退款',
    }[s] || s
  )
}
function statusTheme(s: string): 'success' | 'warning' | 'default' | 'danger' {
  if (s === 'active') return 'success'
  if (s === 'provisioning' || s === 'pending') return 'warning'
  if (s === 'cancelled' || s === 'refunded') return 'danger'
  return 'default'
}

async function loadOrders() {
  loading.value = true
  try {
    const { data } = await getMyOrders({ page: pagination.current, page_size: pagination.pageSize })
    orders.value = data?.items || []
    pagination.total = data?.total || 0
  } catch {
    orders.value = []
  } finally {
    loading.value = false
  }
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  loadOrders()
}

onMounted(loadOrders)
</script>

<style scoped>
.order-page { padding: 16px 24px; }
.order-title { font-size: 20px; font-weight: 700; margin: 0 0 16px; }
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-amount { color: #e37318; font-weight: 700; }
.cell-original { text-decoration: line-through; }
.cell-discount { color: #e37318; margin-right: 6px; }
</style>
