<template>
  <div class="referral-page">
    <ReferralNav />

    <div class="referral-header">
      <h2 class="referral-title">返现明细</h2>
      <div class="referral-filters">
        <t-select
          v-model="type"
          class="referral-filter"
          placeholder="全部类型"
          clearable
          :options="typeOptions"
          @change="reload"
        />
        <t-button variant="outline" size="small" :loading="loading" @click="load">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </div>
    </div>

    <t-table
      :data="items"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :pagination="pagination"
      cell-empty-content="—"
      @page-change="onPageChange"
    >
      <template #type="{ row }">
        <div class="cell-strong">{{ typeText(row.type) }}</div>
        <div class="cell-sub">{{ row.remark || '—' }}</div>
      </template>
      <template #source="{ row }">
        <div class="cell-strong">{{ row.invitee_username || '—' }}</div>
        <div class="cell-sub">{{ row.order_no || row.ref_no || '—' }}</div>
      </template>
      <template #amount="{ row }">
        <span :class="row.direction >= 0 ? 'cell-money' : 'cell-money--minus'">
          {{ row.direction >= 0 ? '+' : '-' }}¥{{ (row.amount || 0).toFixed(2) }}
        </span>
      </template>
      <template #balance_after="{ row }">¥{{ (row.balance_after || 0).toFixed(2) }}</template>
      <template #created_at="{ row }">{{ formatTime(row.created_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !items.length" description="暂无返现记录" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import { RefreshIcon } from 'tdesign-icons-vue-next'

import { getReferralCashbacks, type CashbackInfo } from '@/api/referral'
import ReferralNav from '@/components/referral-nav/index.vue'

defineOptions({ name: 'ReferralCashbacks' })

const items = ref<CashbackInfo[]>([])
const loading = ref(false)
const type = ref('')
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const typeOptions = [
  { label: '订单返现', value: 'cashback' },
  { label: '续费返现', value: 'renewal_cashback' },
  { label: '退款冲减', value: 'refund_clawback' },
  { label: '提现冻结', value: 'withdraw_freeze' },
  { label: '提现解冻', value: 'withdraw_return' },
  { label: '转入余额', value: 'transfer_out' },
  { label: '转入回滚', value: 'transfer_return' },
]

const columns: PrimaryTableCol<CashbackInfo>[] = [
  { colKey: 'type', title: '类型', minWidth: 180 },
  { colKey: 'source', title: '来源', minWidth: 180 },
  { colKey: 'amount', title: '变动金额', width: 140 },
  { colKey: 'balance_after', title: '变动后余额', width: 130 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

function typeText(t: string): string {
  const map: Record<string, string> = {
    cashback: '订单返现',
    renewal_cashback: '续费返现',
    refund_clawback: '退款冲减',
    withdraw_freeze: '提现冻结',
    withdraw_return: '提现解冻',
    transfer_out: '转入余额',
    transfer_return: '转入回滚',
  }
  return map[t] || t || '—'
}

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  loading.value = true
  try {
    const { data } = await getReferralCashbacks({
      page: pagination.current,
      page_size: pagination.pageSize,
      type: type.value || undefined,
    })
    items.value = data?.items || []
    pagination.total = data?.meta?.total || 0
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

function reload() {
  pagination.current = 1
  load()
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  load()
}

onMounted(load)
</script>

<style scoped>
.referral-page { padding: 16px 24px; }
.referral-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.referral-title { font-size: 20px; font-weight: 700; margin: 0; }
.referral-filters { display: flex; align-items: center; gap: 8px; }
.referral-filter { width: 160px; }
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-money { color: #e37318; font-weight: 700; }
.cell-money--minus { color: #64748b; font-weight: 700; }
</style>
