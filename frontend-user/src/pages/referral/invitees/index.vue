<template>
  <div class="referral-page">
    <ReferralNav />

    <div class="referral-header">
      <h2 class="referral-title">我的邀请</h2>
      <t-button variant="outline" size="small" :loading="loading" @click="load">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </div>

    <t-table
      :data="items"
      :columns="columns"
      :loading="loading"
      row-key="user_id"
      :pagination="pagination"
      cell-empty-content="—"
      @page-change="onPageChange"
    >
      <template #user="{ row }">
        <div class="cell-strong">{{ row.username || '—' }}</div>
        <div class="cell-sub">{{ row.email || '—' }}</div>
      </template>
      <template #cashback_total="{ row }">
        <span class="cell-money">¥{{ (row.cashback_total || 0).toFixed(2) }}</span>
      </template>
      <template #order_count="{ row }">{{ row.order_count || 0 }}</template>
      <template #invited_at="{ row }">{{ formatTime(row.invited_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !items.length" description="还没有邀请记录，去分享你的推广链接吧" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import { RefreshIcon } from 'tdesign-icons-vue-next'

import { getReferralInvitees, type InviteeInfo } from '@/api/referral'
import ReferralNav from '@/components/referral-nav/index.vue'

defineOptions({ name: 'ReferralInvitees' })

const items = ref<InviteeInfo[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<InviteeInfo>[] = [
  { colKey: 'user', title: '被邀请人', minWidth: 200 },
  { colKey: 'cashback_total', title: '累计返现', width: 140 },
  { colKey: 'order_count', title: '成功订单', width: 110 },
  { colKey: 'invited_at', title: '邀请时间', width: 170 },
]

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  loading.value = true
  try {
    const { data } = await getReferralInvitees({
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    items.value = data?.items || []
    pagination.total = data?.meta?.total || 0
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
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
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-money { color: #e37318; font-weight: 700; }
</style>
