<template>
  <div class="referral-page">
    <ReferralNav />

    <div class="referral-header">
      <h2 class="referral-title">提现与转出</h2>
      <div class="referral-actions">
        <t-button
          theme="primary"
          size="small"
          :disabled="!canOperate"
          @click="openWithdraw"
        >
          申请提现
        </t-button>
        <t-button variant="outline" size="small" :disabled="!canOperate" @click="openTransfer">
          转入现金余额
        </t-button>
        <t-button variant="outline" size="small" :loading="loading" @click="loadAll">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </div>
    </div>

    <section class="balance-bar">
      <div class="balance-item">
        <span class="balance-item__label">可用返现余额</span>
        <span class="balance-item__value">¥{{ money(profile?.balance) }}</span>
      </div>
      <div class="balance-item">
        <span class="balance-item__label">审核中冻结</span>
        <span class="balance-item__value">¥{{ money(profile?.frozen) }}</span>
      </div>
      <div class="balance-item">
        <span class="balance-item__label">最低提现</span>
        <span class="balance-item__value">¥{{ money(profile?.min_withdraw_amount) }}</span>
      </div>
    </section>

    <p v-if="memberStore.isSub" class="sub-tip">子账号仅可查看推广数据，提现与转入余额请由主账号操作。</p>
    <p v-else-if="!profile?.enabled" class="sub-tip">推广返现当前未开放，无法提现或转入余额。</p>
    <p v-else-if="(profile?.balance || 0) <= 0" class="sub-tip">
      可用返现余额为 {{ money(profile?.balance) }}（退款冲减可能形成欠款），补足后方可提现或转入余额。
    </p>

    <t-table
      :data="items"
      :columns="columns"
      :loading="loading"
      row-key="id"
      :pagination="pagination"
      cell-empty-content="—"
      @page-change="onPageChange"
    >
      <template #withdraw_no="{ row }">
        <div class="cell-strong">{{ row.withdraw_no }}</div>
        <div class="cell-sub">{{ channelText(row.channel) }} · {{ row.account || '—' }}</div>
      </template>
      <template #amount="{ row }">
        <span class="cell-money">¥{{ (row.amount || 0).toFixed(2) }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
          {{ statusText(row.status) }}
        </t-tag>
      </template>
      <template #audit="{ row }">
        <div>{{ row.audit_by_name || '—' }}</div>
        <div class="cell-sub">{{ formatTime(row.audited_at) }}</div>
      </template>
      <template #created_at="{ row }">{{ formatTime(row.created_at) }}</template>
    </t-table>

    <t-empty v-if="!loading && !items.length" description="暂无提现记录" />

    <!-- 申请提现 -->
    <t-dialog
      v-model:visible="withdrawVisible"
      header="申请提现"
      :confirm-btn="{ loading: submitting }"
      @confirm="submitWithdraw"
    >
      <t-form :data="withdrawForm" label-align="top">
        <t-form-item label="提现金额">
          <t-input-number
            v-model="withdrawForm.amount"
            :min="0"
            :max="profile?.balance || 0"
            :decimal-places="2"
            theme="normal"
            placeholder="请输入提现金额"
          />
        </t-form-item>
        <t-form-item label="提现方式">
          <t-select v-model="withdrawForm.channel" :options="channelOptions" />
        </t-form-item>
        <t-form-item label="收款账户">
          <t-input v-model="withdrawForm.account" placeholder="支付宝账号或银行卡号" />
        </t-form-item>
      </t-form>
      <p class="dialog-tip">申请后金额将从可用余额转入冻结，待后台审核通过后出账；驳回则解冻退回。</p>
    </t-dialog>

    <!-- 转入余额 -->
    <t-dialog
      v-model:visible="transferVisible"
      header="转入现金余额"
      :confirm-btn="{ loading: submitting }"
      @confirm="submitTransfer"
    >
      <t-form :data="transferForm" label-align="top">
        <t-form-item label="转入金额">
          <t-input-number
            v-model="transferForm.amount"
            :min="0"
            :max="profile?.balance || 0"
            :decimal-places="2"
            theme="normal"
            placeholder="请输入转入金额"
          />
        </t-form-item>
      </t-form>
      <p class="dialog-tip">即时到账，转入后可在费用中心余额中使用。</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import { RefreshIcon } from 'tdesign-icons-vue-next'

import {
  applyReferralWithdrawal,
  getReferralProfile,
  getReferralWithdrawals,
  transferReferralToWallet,
  type ReferralProfile,
  type WithdrawalInfo,
} from '@/api/referral'
import ReferralNav from '@/components/referral-nav/index.vue'
import { useMemberStore } from '@/store/modules/member'

defineOptions({ name: 'ReferralWithdrawals' })

const memberStore = useMemberStore()

const loading = ref(false)
const submitting = ref(false)
const profile = ref<ReferralProfile | null>(null)
const items = ref<WithdrawalInfo[]>([])
const pagination = reactive({ current: 1, pageSize: 10, total: 0 })

const withdrawVisible = ref(false)
const transferVisible = ref(false)
const withdrawForm = reactive({ amount: 0, channel: 'alipay', account: '' })
const transferForm = reactive({ amount: 0 })

const channelOptions = [
  { label: '支付宝', value: 'alipay' },
  { label: '银行卡', value: 'bank' },
]

const columns: PrimaryTableCol<WithdrawalInfo>[] = [
  { colKey: 'withdraw_no', title: '提现单号', minWidth: 200 },
  { colKey: 'amount', title: '提现金额', width: 130 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'audit', title: '审核', width: 160 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
]

const canOperate = computed(
  () => !memberStore.isSub && Boolean(profile.value?.enabled) && (profile.value?.balance || 0) > 0,
)

function money(v?: number): string {
  return Number(v || 0).toFixed(2)
}

function statusText(s: string): string {
  return { pending: '审核中', approved: '已通过', rejected: '已驳回' }[s] || s || '—'
}

function statusTheme(s: string): 'success' | 'warning' | 'danger' | 'default' {
  if (s === 'approved') return 'success'
  if (s === 'pending') return 'warning'
  if (s === 'rejected') return 'danger'
  return 'default'
}

function channelText(c: string): string {
  return { alipay: '支付宝', bank: '银行卡' }[c] || c || '—'
}

function formatTime(v: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function loadProfile() {
  try {
    const { data } = await getReferralProfile()
    profile.value = data
  } catch {
    profile.value = null
  }
}

async function loadList() {
  loading.value = true
  try {
    const { data } = await getReferralWithdrawals({
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

async function loadAll() {
  await Promise.all([loadProfile(), loadList()])
}

function onPageChange(info: { current: number; pageSize: number }) {
  pagination.current = info.current
  pagination.pageSize = info.pageSize
  loadList()
}

function openWithdraw() {
  withdrawForm.amount = 0
  withdrawForm.channel = 'alipay'
  withdrawForm.account = ''
  withdrawVisible.value = true
}

function openTransfer() {
  transferForm.amount = 0
  transferVisible.value = true
}

async function submitWithdraw() {
  const amount = Number(withdrawForm.amount || 0)
  const min = Number(profile.value?.min_withdraw_amount || 0)
  if (amount <= 0) {
    MessagePlugin.warning('请输入提现金额')
    return
  }
  if (amount < min) {
    MessagePlugin.warning(`单笔提现不得低于 ¥${min.toFixed(2)}`)
    return
  }
  if (amount > Number(profile.value?.balance || 0)) {
    MessagePlugin.warning('提现金额超过可用余额')
    return
  }
  submitting.value = true
  try {
    await applyReferralWithdrawal({
      amount,
      channel: withdrawForm.channel,
      account: withdrawForm.account,
    })
    MessagePlugin.success('提现申请已提交，请等待审核')
    withdrawVisible.value = false
    await loadAll()
  } catch (e: any) {
    MessagePlugin.error(e?.message || '提现申请失败')
  } finally {
    submitting.value = false
  }
}

async function submitTransfer() {
  const amount = Number(transferForm.amount || 0)
  if (amount <= 0) {
    MessagePlugin.warning('请输入转入金额')
    return
  }
  if (amount > Number(profile.value?.balance || 0)) {
    MessagePlugin.warning('转入金额超过可用余额')
    return
  }
  submitting.value = true
  try {
    await transferReferralToWallet(amount)
    MessagePlugin.success('已转入现金余额')
    transferVisible.value = false
    await loadAll()
  } catch (e: any) {
    MessagePlugin.error(e?.message || '转入失败')
  } finally {
    submitting.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.referral-page { padding: 16px 24px; }
.referral-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.referral-title { font-size: 20px; font-weight: 700; margin: 0; }
.referral-actions { display: flex; align-items: center; gap: 8px; }
.balance-bar {
  display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 12px;
}
.balance-item {
  flex: 1; min-width: 160px; background: #fff; border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 12px 16px; display: flex; flex-direction: column; gap: 4px;
}
.balance-item__label { font-size: 13px; color: #888; }
.balance-item__value { font-size: 20px; font-weight: 700; color: #b76a00; }
.sub-tip { color: #c2761a; background: #fff8ec; border-radius: 8px; padding: 8px 12px; font-size: 12.5px; margin: 0 0 12px; }
.cell-strong { font-weight: 600; }
.cell-sub { color: #999; font-size: 12px; }
.cell-money { color: #e37318; font-weight: 700; }
.dialog-tip { color: #999; font-size: 12px; margin: 8px 0 0; line-height: 1.6; }
</style>
