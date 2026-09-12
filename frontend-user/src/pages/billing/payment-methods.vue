<template>
  <div class="payment-methods-page">
    <header class="pm-hero">
      <div class="pm-hero__left">
        <span class="pm-hero__chip"><WalletIcon size="22" /></span>
        <div class="pm-hero__text">
          <h2 class="pm-hero__title">支付方式</h2>
          <p class="pm-hero__desc">设置默认支付方式与优先级，收银台将按此顺序推荐渠道；同时维护提现收款账户。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadAll">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <p v-if="memberStore.isSub" class="sub-tip">子账号仅可查看，支付方式与收款账户的修改请由主账号操作。</p>

    <!-- 支付方式偏好 -->
    <section class="pm-panel">
      <div class="pm-panel__head">
        <div>
          <h3 class="pm-panel__title">默认方式与优先级</h3>
          <p class="pm-panel__hint">场景「{{ sceneLabel(scene) }}」下可用渠道；拖动序号或点按上下调整顺序，标记为默认的渠道将优先展示。</p>
        </div>
        <t-space size="small">
          <t-select v-model="scene" :options="sceneOptions" size="small" style="width: 180px" @change="loadMethods" />
          <t-button theme="primary" size="small" :loading="saving" :disabled="!canEdit" @click="handleSave">
            <template #icon><CheckCircleIcon /></template>
            保存偏好
          </t-button>
        </t-space>
      </div>

      <t-table
        :data="methods"
        :columns="methodColumns"
        row-key="channel_code"
        size="small"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="loading"
      >
        <template #channel_code="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="cell-sub">{{ row.channel_code }}</span>
          </div>
        </template>
        <template #mode="{ row }">
          <t-tag :theme="row.mode === 'api' ? 'primary' : 'warning'" variant="light" size="small" shape="round">
            {{ row.mode === 'api' ? '接口自动' : '线下人工' }}
          </t-tag>
        </template>
        <template #limits="{ row }">
          <span class="cell-sub">{{ limitText(row) }}</span>
        </template>
        <template #priority="{ row }">
          <t-input-number v-model="row.priority" :min="0" size="small" theme="column" :disabled="!canEdit" style="width: 90px" />
        </template>
        <template #is_default="{ row }">
          <t-radio :value="row.channel_code === defaultCode" :disabled="!canEdit" @change="() => (defaultCode = row.channel_code)" />
        </template>
        <template #empty>
          <t-empty :description="`场景「${sceneLabel(scene)}」暂无可用支付方式`" />
        </template>
      </t-table>
    </section>

    <!-- 提现收款账户 -->
    <section class="pm-panel">
      <div class="pm-panel__head">
        <div>
          <h3 class="pm-panel__title">提现收款账户</h3>
          <p class="pm-panel__hint">提现时将款项打至默认收款账户；账号在服务端加密存储，此处仅展示尾号。</p>
        </div>
        <t-button theme="primary" size="small" :disabled="!canEdit" @click="openAccountDialog">
          <template #icon><AddIcon /></template>
          新增账户
        </t-button>
      </div>

      <t-table
        :data="accounts"
        :columns="accountColumns"
        row-key="id"
        size="small"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="accountLoading"
      >
        <template #channel="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ row.account_name }}</span>
            <span class="cell-sub">{{ channelText(row.channel) }} · {{ row.account_no }}</span>
          </div>
        </template>
        <template #bank_name="{ row }">
          <span class="cell-sub">{{ row.bank_name || '—' }}{{ row.branch ? ` · ${row.branch}` : '' }}</span>
        </template>
        <template #is_default="{ row }">
          <t-tag v-if="row.is_default" theme="success" variant="light" size="small" shape="round">默认</t-tag>
          <span v-else class="cell-sub">—</span>
        </template>
        <template #action="{ row }">
          <t-link v-if="!row.is_default && canEdit" theme="primary" hover="color" @click="handleSetDefault(row)">设为默认</t-link>
          <span v-else class="cell-sub">—</span>
        </template>
        <template #empty>
          <t-empty description="暂无收款账户，点击右上角新增" />
        </template>
      </t-table>
    </section>

    <!-- 我的提现记录 -->
    <section class="pm-panel">
      <div class="pm-panel__head">
        <div>
          <h3 class="pm-panel__title">我的提现记录</h3>
          <p class="pm-panel__hint">每笔提现记录所用收款渠道、账号尾号与打款方式，便于核对到账。</p>
        </div>
        <t-button theme="primary" size="small" :disabled="!canEdit || !accounts.length" @click="openWithdrawDialog">
          <template #icon><MoneyIcon /></template>
          申请提现
        </t-button>
      </div>

      <t-table
        :data="withdrawals"
        :columns="withdrawColumns"
        row-key="id"
        size="small"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="withdrawLoading"
        :pagination="withdrawPagination"
        @page-change="handleWithdrawPageChange"
      >
        <template #withdraw_no="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ row.withdraw_no }}</span>
            <span class="cell-sub">{{ row.payout_no ? `打款单 ${row.payout_no}` : '待生成打款单' }}</span>
          </div>
        </template>
        <template #target="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ channelText(row.channel) }}</span>
            <span class="cell-sub">{{ row.account_name || '—' }} · {{ row.account || '—' }}</span>
          </div>
        </template>
        <template #amount="{ row }">
          <span class="cell-money">¥{{ formatPrice(row.amount) }}</span>
        </template>
        <template #payout_mode="{ row }">
          <span class="cell-sub">{{ payoutModeText(row.payout_mode) }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="withdrawStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ withdrawStatusText(row.status) }}
          </t-tag>
        </template>
        <template #created_at="{ row }">
          <span class="cell-sub">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无提现记录" />
        </template>
      </t-table>
    </section>

    <!-- 新增收款账户 -->
    <t-dialog
      v-model:visible="accountVisible"
      header="新增收款账户"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: accountSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreateAccount"
      @close="accountVisible = false"
    >
      <t-form label-align="top" :data="accountForm" @submit.prevent>
        <t-form-item label="收款渠道">
          <t-select v-model="accountForm.channel" :options="accountChannelOptions" />
        </t-form-item>
        <t-form-item label="账户姓名">
          <t-input v-model="accountForm.account_name" placeholder="与账户实名一致" />
        </t-form-item>
        <t-form-item label="账号">
          <t-input v-model="accountForm.account_no" placeholder="支付宝账号或银行卡号" />
        </t-form-item>
        <t-form-item v-if="accountForm.channel === 'bank'" label="开户行">
          <t-input v-model="accountForm.bank_name" placeholder="如 招商银行" />
        </t-form-item>
        <t-form-item v-if="accountForm.channel === 'bank'" label="开户支行">
          <t-input v-model="accountForm.branch" placeholder="如 深圳科技园支行" />
        </t-form-item>
        <t-form-item label="设为默认">
          <t-switch v-model="accountForm.is_default" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 申请提现 -->
    <t-dialog
      v-model:visible="withdrawVisible"
      header="申请提现"
      width="480px"
      :confirm-btn="{ content: '提交申请', theme: 'primary', loading: withdrawSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleApplyWithdraw"
      @close="withdrawVisible = false"
    >
      <t-alert
        theme="info"
        message="提交后金额将从可用余额转入冻结，后台审核通过后按所选打款方式出账；驳回则解冻退回。"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top" :data="withdrawForm" @submit.prevent>
        <t-form-item label="提现金额（元）">
          <t-input-number v-model="withdrawForm.amount" :min="0" :precision="2" theme="column" placeholder="请输入提现金额" />
        </t-form-item>
        <t-form-item label="收款账户">
          <t-select v-model="withdrawForm.account_id" placeholder="选择收款账户" :options="accountSelectOptions" />
        </t-form-item>
        <t-form-item label="打款方式">
          <t-select v-model="withdrawForm.payout_channel" clearable placeholder="按平台默认" :options="payoutModeOptions" />
        </t-form-item>
        <t-form-item label="备注">
          <t-input v-model="withdrawForm.remark" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, CheckCircleIcon, MoneyIcon, RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createPayoutAccount,
  getPaymentMethods,
  getPaymentPreferences,
  getPayoutAccounts,
  savePaymentPreferences,
  setDefaultPayoutAccount,
  type PaymentChannelInfo,
  type PayoutAccountInfo,
  type WithdrawInfo,
} from '@/api/payment'
import { getMyWithdrawals } from '@/api/finance'
import { useMemberStore } from '@/store/modules/member'

defineOptions({ name: 'BillingPaymentMethods' })

const memberStore = useMemberStore()
const canEdit = computed(() => !memberStore.isSub)

const sceneOptions = [
  { label: 'PC 原生（网页）', value: 'native' },
  { label: '手机网页 H5', value: 'h5' },
  { label: '主扫/被扫（二维码）', value: 'scan' },
]

const accountChannelOptions = [
  { label: '支付宝', value: 'alipay' },
  { label: '银行卡', value: 'bank' },
]

const scene = ref('native')
const loading = ref(false)
const saving = ref(false)
const accountLoading = ref(false)
const accountSubmitting = ref(false)
const withdrawLoading = ref(false)

const methods = ref<Array<PaymentChannelInfo & { priority: number }>>([])
const defaultCode = ref('')
const accounts = ref<PayoutAccountInfo[]>([])
const withdrawals = ref<WithdrawInfo[]>([])
const withdrawPagination = reactive({ current: 1, pageSize: 10, total: 0 })

const methodColumns: PrimaryTableCol<PaymentChannelInfo & { priority: number }>[] = [
  { colKey: 'channel_code', title: '支付方式', minWidth: 180 },
  { colKey: 'mode', title: '模式', width: 110 },
  { colKey: 'limits', title: '单笔限额', minWidth: 160 },
  { colKey: 'priority', title: '优先级', width: 120 },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' as const },
]

const accountColumns: PrimaryTableCol<PayoutAccountInfo>[] = [
  { colKey: 'channel', title: '账户', minWidth: 220 },
  { colKey: 'bank_name', title: '开户行', minWidth: 160 },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' as const },
  { colKey: 'action', title: '操作', width: 100 },
]

const withdrawColumns: PrimaryTableCol<WithdrawInfo>[] = [
  { colKey: 'withdraw_no', title: '提现单', minWidth: 200 },
  { colKey: 'target', title: '收款方式', minWidth: 190 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'payout_mode', title: '打款方式', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '申请时间', width: 160 },
]

function sceneLabel(value: string): string {
  return sceneOptions.find((s) => s.value === value)?.label || value
}

function limitText(row: PaymentChannelInfo): string {
  const min = row.min_amount_fen > 0 ? `¥${(row.min_amount_fen / 100).toFixed(2)}` : '不限'
  const max = row.max_amount_fen > 0 ? `¥${(row.max_amount_fen / 100).toFixed(2)}` : '不限'
  return `${min} ~ ${max}`
}

function channelText(value: string): string {
  return { alipay: '支付宝', bank: '银行卡' }[value] || value || '—'
}

function payoutModeText(value: string): string {
  return { api: '接口打款', manual: '人工打款' }[value] || '按平台默认'
}

function withdrawStatusText(status: string): string {
  return { pending: '待审核', approved: '已通过', paying: '打款中', paid: '已打款', rejected: '已驳回', failed: '打款失败' }[status] || status || '—'
}

function withdrawStatusTheme(status: string): 'success' | 'warning' | 'danger' | 'default' {
  if (status === 'paid') return 'success'
  if (status === 'pending' || status === 'approved' || status === 'paying') return 'warning'
  if (status === 'rejected' || status === 'failed') return 'danger'
  return 'default'
}

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatTime(value: string): string {
  return value ? value.replace('T', ' ').slice(0, 16) : '—'
}

// 加载可用方式，并按已保存偏好回填优先级/默认项。
async function loadMethods() {
  loading.value = true
  try {
    const [{ data: options }, prefRes] = await Promise.all([
      getPaymentMethods({ scene: scene.value }),
      getPaymentPreferences().catch(() => ({ data: { items: [] } })),
    ])
    const preferences = (prefRes?.data?.items || []).filter((p) => !p.scene || p.scene === scene.value)
    const list = (options?.channels || []).map((ch, index) => {
      const pref = preferences.find((p) => p.channel_code === ch.channel_code)
      return { ...ch, priority: pref?.priority ?? preferences.length + index }
    })
    methods.value = list
    const prefDefault = preferences.find((p) => p.is_default)?.channel_code
    defaultCode.value = prefDefault || options?.default || list[0]?.channel_code || ''
  } catch (error) {
    methods.value = []
    MessagePlugin.error((error as Error)?.message || '加载支付方式失败')
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!methods.value.length) {
    MessagePlugin.warning('当前场景暂无可用支付方式')
    return
  }
  saving.value = true
  try {
    // 按优先级倒序提交：后端以 len-i 记录，数值越大越靠前。
    const ordered = [...methods.value].sort((a, b) => (b.priority || 0) - (a.priority || 0))
    await savePaymentPreferences({
      scene: scene.value,
      default: defaultCode.value,
      priorities: ordered.map((m) => ({
        scene: scene.value,
        channel_code: m.channel_code,
        priority: m.priority || 0,
        is_default: m.channel_code === defaultCode.value,
      })),
    })
    MessagePlugin.success('支付方式偏已保存')
    await loadMethods()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存偏好失败')
  } finally {
    saving.value = false
  }
}

async function loadAccounts() {
  accountLoading.value = true
  try {
    const { data } = await getPayoutAccounts()
    accounts.value = data?.items || []
  } catch {
    accounts.value = []
  } finally {
    accountLoading.value = false
  }
}

async function loadWithdrawals() {
  withdrawLoading.value = true
  try {
    const { data } = await getMyWithdrawals({
      page: withdrawPagination.current,
      page_size: withdrawPagination.pageSize,
    })
    withdrawals.value = data?.items || []
    withdrawPagination.total = data?.meta?.total || 0
  } catch {
    withdrawals.value = []
  } finally {
    withdrawLoading.value = false
  }
}

function handleWithdrawPageChange(info: PageInfo) {
  withdrawPagination.current = info.current
  withdrawPagination.pageSize = info.pageSize
  loadWithdrawals()
}

const accountVisible = ref(false)
const accountForm = reactive<{
  channel: string
  account_name: string
  account_no: string
  bank_name: string
  branch: string
  is_default: boolean
}>({ channel: 'alipay', account_name: '', account_no: '', bank_name: '', branch: '', is_default: false })

function openAccountDialog() {
  Object.assign(accountForm, { channel: 'alipay', account_name: '', account_no: '', bank_name: '', branch: '', is_default: false })
  accountVisible.value = true
}

async function handleCreateAccount() {
  if (!accountForm.account_name.trim() || !accountForm.account_no.trim()) {
    MessagePlugin.warning('请填写账户姓名与账号')
    return
  }
  accountSubmitting.value = true
  try {
    await createPayoutAccount({
      channel: accountForm.channel,
      account_name: accountForm.account_name.trim(),
      account_no: accountForm.account_no.trim(),
      bank_name: accountForm.bank_name.trim() || undefined,
      branch: accountForm.branch.trim() || undefined,
      is_default: accountForm.is_default,
    })
    MessagePlugin.success('收款账户已保存')
    accountVisible.value = false
    await loadAccounts()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存收款账户失败')
  } finally {
    accountSubmitting.value = false
  }
}

async function handleSetDefault(row: PayoutAccountInfo) {
  try {
    await setDefaultPayoutAccount(row.id)
    MessagePlugin.success('已设为默认收款账户')
    await loadAccounts()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '设置默认账户失败')
  }
}

// ===== 申请提现 =====
const withdrawVisible = ref(false)
const withdrawSubmitting = ref(false)
const withdrawForm = reactive<{ amount: number | undefined; account_id: number | undefined; payout_channel: string | undefined; remark: string }>({
  amount: undefined,
  account_id: undefined,
  payout_channel: undefined,
  remark: '',
})

const payoutModeOptions = [
  { label: '人工打款', value: 'manual' },
  { label: '接口自动打款', value: 'api' },
]

const accountSelectOptions = computed(() =>
  accounts.value.map((a) => ({
    label: `${channelText(a.channel)} · ${a.account_name} ${a.account_no}`,
    value: a.id,
  })),
)

function openWithdrawDialog() {
  const preferred = accounts.value.find((a) => a.is_default) || accounts.value[0]
  Object.assign(withdrawForm, {
    amount: undefined,
    account_id: preferred?.id,
    payout_channel: undefined,
    remark: '',
  })
  withdrawVisible.value = true
}

async function handleApplyWithdraw() {
  if (!withdrawForm.amount || withdrawForm.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的提现金额')
    return
  }
  if (!withdrawForm.account_id) {
    MessagePlugin.warning('请选择收款账户')
    return
  }
  withdrawSubmitting.value = true
  try {
    const { applyWithdraw } = await import('@/api/payment')
    const { data } = await applyWithdraw({
      amount: withdrawForm.amount,
      account_id: withdrawForm.account_id,
      payout_channel: withdrawForm.payout_channel,
      remark: withdrawForm.remark || undefined,
    })
    MessagePlugin.success(`提现单 ${data?.withdraw_no || ''} 已提交，等待审核打款`)
    withdrawVisible.value = false
    await loadWithdrawals()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '提交提现申请失败')
  } finally {
    withdrawSubmitting.value = false
  }
}

async function loadAll() {
  await Promise.all([loadMethods(), loadAccounts(), loadWithdrawals()])
}

onMounted(loadAll)
</script>

<style scoped>
.payment-methods-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.pm-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
  border-radius: 16px;
  background: linear-gradient(135deg, #0f766e 0%, #14b8a6 100%);
  color: #fff;
}

.pm-hero__left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.pm-hero__chip {
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.18);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.pm-hero__title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.pm-hero__desc {
  margin: 4px 0 0;
  font-size: 13px;
  opacity: 0.88;
}

.sub-tip {
  margin: 0;
  padding: 10px 14px;
  border-radius: 10px;
  background: #fff7ed;
  color: #b45309;
  font-size: 13px;
}

.pm-panel {
  padding: 20px 22px;
  border-radius: 16px;
  background: #fff;
  border: 1px solid #eef2f6;
}

.pm-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.pm-panel__title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #182230;
}

.pm-panel__hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: #64748b;
  max-width: 620px;
  line-height: 1.6;
}

.cell-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-strong {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.cell-sub {
  font-size: 12px;
  color: #64748b;
}

.cell-money {
  font-size: 13px;
  font-weight: 600;
  color: #0f766e;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 768px) {
  .pm-hero {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
