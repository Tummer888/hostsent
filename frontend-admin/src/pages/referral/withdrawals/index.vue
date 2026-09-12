<template>
  <div class="page-body referral-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FileIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">返现提现审核</h2>
          <p class="page-header__desc">审核用户返现余额提现申请；通过后出账，驳回则解冻退回可用余额</p>
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
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="referralWithdrawStatusOptions" />
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

        <template #username="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">ID {{ row.user_id }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #account="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.account || '—' }}</span>
            <span class="price-sub">{{ channelLabel(row.channel) }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="referralWithdrawStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ referralWithdrawStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #audit_by_name="{ row }">
          <span class="cell-muted">{{ row.audit_by_name || '—' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '通过', value: 'approve', hidden: () => !(row.status === 'pending'), theme: 'success' },
                { content: '驳回', value: 'reject', hidden: () => !(row.status === 'pending'), theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link v-if="row.status === 'pending'" theme="primary" hover="color" @click="openAuditDialog(row, 'approve')">
                通过
              </t-link>
              <t-link v-if="row.status === 'pending'" theme="danger" hover="color" @click="openAuditDialog(row, 'reject')">
                驳回
              </t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无返现提现单" />
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

    <t-dialog
      v-model:visible="auditVisible"
      :header="auditForm.action === 'approve' ? '通过返现提现' : '驳回返现提现'"
      width="480px"
      :confirm-btn="{ content: auditForm.action === 'approve' ? '确认通过' : '确认驳回', theme: auditForm.action === 'approve' ? 'success' : 'danger' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleAudit"
      @close="auditVisible = false"
    >
      <t-form label-align="top" :data="auditForm" @submit.prevent>
        <t-alert
          :theme="auditForm.action === 'approve' ? 'info' : 'warning'"
          :message="`提现单 ${auditForm.label}，金额 ¥${formatPrice(auditForm.amount)}`"
          style="margin-bottom: 12px"
        />
        <t-form-item label="审核备注" name="remark">
          <t-textarea v-model="auditForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="选填，审核说明" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { FileIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { approveReferralWithdrawal, getReferralWithdrawals, rejectReferralWithdrawal } from '@/api/referral'
import {
  formatPrice,
  formatTime,
  referralWithdrawStatusLabel,
  referralWithdrawStatusOptions,
  referralWithdrawStatusTheme,
} from '@/pages/referral/constants'
import type { ReferralWithdrawInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ReferralWithdrawals' })

const withdrawList = ref<ReferralWithdrawInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{ user_id: string | undefined; status: string | undefined }>({
  user_id: undefined,
  status: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<ReferralWithdrawInfo>[] = [
  { colKey: 'withdraw_no', title: '提现单号', minWidth: 190 },
  { colKey: 'username', title: '用户', minWidth: 150 },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'account', title: '收款账户', minWidth: 170 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 130, fixed: 'right' as const, align: 'center' as const },
]

function channelLabel(channel: string): string {
  return { alipay: '支付宝', bank: '银行卡' }[channel] || channel || '—'
}

async function loadWithdrawals() {
  loading.value = true
  try {
    const data = await getReferralWithdrawals({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    withdrawList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载返现提现单失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadWithdrawals()
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
  loadWithdrawals()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.status = undefined
  pagination.current = 1
  loadWithdrawals()
}

const auditVisible = ref(false)
const auditForm = reactive<{
  id: number
  action: 'approve' | 'reject'
  label: string
  amount: number
  remark: string | undefined
}>({
  id: 0,
  action: 'approve',
  label: '',
  amount: 0,
  remark: undefined,
})

function openAuditDialog(row: ReferralWithdrawInfo, action: 'approve' | 'reject') {
  auditForm.id = row.id
  auditForm.action = action
  auditForm.label = row.withdraw_no
  auditForm.amount = row.amount
  auditForm.remark = undefined
  auditVisible.value = true
}

async function handleAudit() {
  try {
    if (auditForm.action === 'approve') {
      await approveReferralWithdrawal(auditForm.id, { remark: auditForm.remark })
    } else {
      await rejectReferralWithdrawal(auditForm.id, { remark: auditForm.remark })
    }
    MessagePlugin.success(auditForm.action === 'approve' ? '已通过返现提现' : '已驳回返现提现')
    auditVisible.value = false
    loadWithdrawals()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '审核失败')
  }
}

onMounted(loadWithdrawals)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: ReferralWithdrawInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'approve':
      openAuditDialog(row, 'approve')
      break
    case 'reject':
      openAuditDialog(row, 'reject')
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>
