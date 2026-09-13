<template>
  <div class="page-body sales-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FileIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">提成提现审核</h2>
          <p class="page-header__desc">审核销售提成提现申请；通过后进入打款流程，驳回则解冻退回可用余额</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadWithdrawals">
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
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
        <div class="field">
          <span class="field__label">销售</span>
          <t-select
            v-model="filters.admin_id"
            clearable
            filterable
            placeholder="全部销售"
            :options="salesOptions"
          />
        </div>
        <div class="field">
          <span class="field__label">所属部门</span>
          <t-select
            v-model="filters.department_id"
            clearable
            filterable
            placeholder="全部部门"
            :options="departmentOptions"
          />
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
        <h3 class="card-title">提现单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个提现单</span>
      </div>
      <t-table
        row-key="id"
        :data="withdrawList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #withdraw_no="{ row }">
          <span class="cell-strong">{{ row.withdraw_no }}</span>
        </template>

        <template #admin="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.admin_real_name || row.admin_name || '—' }}</span>
            <span class="price-sub">{{ row.department_name || '未分部门' }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #account="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.account || '—' }}</span>
            <span class="price-sub">{{ channelLabel(row.channel) }}{{ row.bank_name ? ` · ${row.bank_name}` : '' }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTheme(row.status)" variant="light" size="small" shape="round">
            {{ row.status_label || statusLabel(row.status) }}
          </t-tag>
        </template>

        <template #audit_by_name="{ row }">
          <span class="cell-muted">{{ row.audit_by_name || '—' }}</span>
        </template>

        <template #payout_no="{ row }">
          <span class="cell-muted">{{ row.payout_no || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '通过', value: 'approve', hidden: () => !(row.status === 'pending' && canAudit) },
                { content: '驳回', value: 'reject', hidden: () => !(row.status === 'pending' && canAudit) },
                { content: '登记打款', value: 'pay', hidden: () => !(isSettleable(row.status) && canSettle) },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link
                v-if="row.status === 'pending' && canAudit"
                theme="primary"
                hover="color"
                @click="openAuditDialog(row, 'approve')"
              >
                通过
              </t-link>
              <t-link
                v-if="row.status === 'pending' && canAudit"
                theme="danger"
                hover="color"
                @click="openAuditDialog(row, 'reject')"
              >
                驳回
              </t-link>
              <t-link
                v-if="isSettleable(row.status) && canSettle"
                theme="primary"
                hover="color"
                @click="openPayDialog(row)"
              >
                登记打款
              </t-link>
              <span v-if="!hasRowAction(row)" class="cell-muted">—</span>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无提成提现单" />
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

    <!-- 审核 -->
    <t-dialog
      v-model:visible="auditVisible"
      :header="auditForm.action === 'approve' ? '通过提成提现' : '驳回提成提现'"
      width="480px"
      :confirm-btn="{ content: auditForm.action === 'approve' ? '确认通过' : '确认驳回', theme: auditForm.action === 'approve' ? 'success' : 'danger' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleAudit"
      @close="auditVisible = false"
    >
      <t-form label-align="top" :data="auditForm" @submit.prevent>
        <t-alert
          :theme="auditForm.action === 'approve' ? 'info' : 'warning'"
          :message="`提现单 ${auditForm.label}，金额 ¥${formatPrice(auditForm.amount)}，销售 ${auditForm.adminLabel}`"
          style="margin-bottom: 12px"
        />
        <t-form-item label="审核备注" name="remark">
          <t-textarea v-model="auditForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="选填，审核说明" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 登记打款 -->
    <t-dialog
      v-model:visible="payVisible"
      header="登记打款"
      width="500px"
      :confirm-btn="{ content: '确认已打款', theme: 'primary', loading: paySubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handlePayConfirm"
      @close="payVisible = false"
    >
      <t-form label-align="top" :data="payForm" @submit.prevent>
        <t-alert
          theme="info"
          :message="`提现单 ${payForm.label}，金额 ¥${formatPrice(payForm.amount)}；渠道流水号与回执链接至少填写一项`"
          style="margin-bottom: 12px"
        />
        <t-form-item label="渠道流水号" name="channel_tx">
          <t-input v-model="payForm.channel_tx" placeholder="如：支付宝/银行流水号" />
        </t-form-item>
        <t-form-item label="回执链接" name="receipt_url">
          <t-input v-model="payForm.receipt_url" placeholder="打款回执截图 / 凭证链接" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="payForm.remark" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="选填，打款说明" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { FileIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import {
  approveSalesWithdrawal,
  getSalesCandidates,
  getSalesWithdrawals,
  paySalesWithdrawal,
  rejectSalesWithdrawal,
} from '@/api/sales'
import { getDepartmentList } from '@/api/admin'
import { useUserStore } from '@/store'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import type { SalesWithdrawalInfo } from '@/types/interface'

defineOptions({ name: 'SalesWithdrawals' })

const { isMobile } = useIsMobile()
const userStore = useUserStore()

const canAudit = computed(() => userStore.hasPermission('sales:commission:audit'))
const canSettle = computed(() => userStore.hasPermission('sales:commission:settle'))

const withdrawList = ref<SalesWithdrawalInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  status: string | undefined
  admin_id: number | undefined
  department_id: number | undefined
}>({
  status: undefined,
  admin_id: undefined,
  department_id: undefined,
})

const salesOptions = ref<Array<{ label: string; value: number }>>([])
const departmentOptions = ref<Array<{ label: string; value: number }>>([])

const statusOptions = [
  { label: '待审核', value: 'pending' },
  { label: '待打款', value: 'approved' },
  { label: '打款中', value: 'paying' },
  { label: '已打款', value: 'paid' },
  { label: '已驳回', value: 'rejected' },
  { label: '打款失败', value: 'failed' },
]

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const columns: PrimaryTableCol<SalesWithdrawalInfo>[] = [
  { colKey: 'withdraw_no', title: '提现单号', minWidth: 190 },
  { colKey: 'admin', title: '销售', minWidth: 150 },
  { colKey: 'amount', title: '金额', width: 120, align: 'right' },
  { colKey: 'account', title: '收款账号', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'payout_no', title: '打款单号', minWidth: 170 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 150, fixed: 'right' as const, align: 'center' as const },
]

function channelLabel(channel: string): string {
  return { alipay: '支付宝', bank: '银行卡' }[channel] || channel || '—'
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '待打款',
    paying: '打款中',
    paid: '已打款',
    rejected: '已驳回',
    failed: '打款失败',
  }
  return map[status] || status || '—'
}

function statusTheme(status: string): 'success' | 'danger' | 'warning' | 'primary' | 'default' {
  switch (status) {
    case 'paid':
      return 'success'
    case 'pending':
      return 'warning'
    case 'rejected':
    case 'failed':
      return 'danger'
    case 'approved':
    case 'paying':
      return 'primary'
    default:
      return 'default'
  }
}

// approved/paying 都可登记打款：approved 是审核通过待打款，paying 是渠道已受理等结算回调。
function isSettleable(status: string): boolean {
  return status === 'approved' || status === 'paying'
}

function hasRowAction(row: SalesWithdrawalInfo): boolean {
  if (row.status === 'pending' && canAudit.value) return true
  if (isSettleable(row.status) && canSettle.value) return true
  return false
}

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

async function loadWithdrawals() {
  loading.value = true
  try {
    const data = await getSalesWithdrawals({
      status: filters.status,
      admin_id: filters.admin_id,
      department_id: filters.department_id,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    withdrawList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    withdrawList.value = []
    MessagePlugin.error((error as Error).message || '加载提成提现单失败')
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  try {
    const data = await getSalesCandidates({})
    salesOptions.value = (data.items || []).map((item) => ({
      label: `${item.real_name || item.username}${item.department_name ? `（${item.department_name}）` : ''}`,
      value: item.admin_id,
    }))
  } catch {
    salesOptions.value = []
  }
  try {
    const data = await getDepartmentList({ status: 'active', flat: 1 })
    departmentOptions.value = (data.items || []).map((item) => ({ label: item.name, value: item.id }))
  } catch {
    departmentOptions.value = []
  }
}

function handleSearch() {
  pagination.current = 1
  loadWithdrawals()
}

function handleResetFilters() {
  filters.status = undefined
  filters.admin_id = undefined
  filters.department_id = undefined
  pagination.current = 1
  loadWithdrawals()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadWithdrawals()
}

function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  pagination.current = clamped
  mobilePage.current = clamped
  loadWithdrawals()
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  pagination.pageSize = pageSize
  pagination.current = 1
  mobilePage.current = 1
  loadWithdrawals()
}

// —— 审核 ——
const auditVisible = ref(false)
const auditForm = reactive<{
  id: number
  action: 'approve' | 'reject'
  label: string
  amount: number
  adminLabel: string
  remark: string | undefined
}>({
  id: 0,
  action: 'approve',
  label: '',
  amount: 0,
  adminLabel: '',
  remark: undefined,
})

function openAuditDialog(row: SalesWithdrawalInfo, action: 'approve' | 'reject') {
  auditForm.id = row.id
  auditForm.action = action
  auditForm.label = row.withdraw_no
  auditForm.amount = row.amount
  auditForm.adminLabel = row.admin_real_name || row.admin_name || '—'
  auditForm.remark = undefined
  auditVisible.value = true
}

async function handleAudit() {
  try {
    if (auditForm.action === 'approve') {
      await approveSalesWithdrawal(auditForm.id, { remark: auditForm.remark })
    } else {
      await rejectSalesWithdrawal(auditForm.id, { remark: auditForm.remark })
    }
    MessagePlugin.success(auditForm.action === 'approve' ? '已通过提成提现' : '已驳回提成提现')
    auditVisible.value = false
    loadWithdrawals()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '审核失败')
  }
}

// —— 登记打款 ——
const payVisible = ref(false)
const paySubmitting = ref(false)
const payForm = reactive<{
  id: number
  label: string
  amount: number
  channel_tx: string
  receipt_url: string
  remark: string
}>({
  id: 0,
  label: '',
  amount: 0,
  channel_tx: '',
  receipt_url: '',
  remark: '',
})

function openPayDialog(row: SalesWithdrawalInfo) {
  payForm.id = row.id
  payForm.label = row.withdraw_no
  payForm.amount = row.amount
  payForm.channel_tx = ''
  payForm.receipt_url = ''
  payForm.remark = ''
  payVisible.value = true
}

async function handlePayConfirm() {
  if (!payForm.channel_tx.trim() && !payForm.receipt_url.trim()) {
    MessagePlugin.warning('渠道流水号与回执链接至少填写一项')
    return
  }
  paySubmitting.value = true
  try {
    await paySalesWithdrawal(payForm.id, {
      channel_tx: payForm.channel_tx.trim() || undefined,
      receipt_url: payForm.receipt_url.trim() || undefined,
      remark: payForm.remark.trim() || undefined,
    })
    MessagePlugin.success('已登记打款')
    payVisible.value = false
    loadWithdrawals()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '登记打款失败')
  } finally {
    paySubmitting.value = false
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: SalesWithdrawalInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'approve':
      openAuditDialog(row, 'approve')
      break
    case 'reject':
      openAuditDialog(row, 'reject')
      break
    case 'pay':
      openPayDialog(row)
      break
  }
}

onMounted(() => {
  loadOptions()
  loadWithdrawals()
})
</script>

<style lang="css">
@import '../shared.css';
</style>
