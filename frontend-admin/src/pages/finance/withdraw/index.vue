<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <div class="page-header__text">
          <h2 class="page-header__title">提现管理</h2>
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
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="withdrawStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">申请时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
        </div>
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
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #withdraw_no="{ row }">
          <span class="cell-strong">{{ row.withdraw_no }}</span>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="withdrawStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ withdrawStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #account="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.account }}</span>
            <span class="price-sub">{{ row.channel }}</span>
          </div>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link v-if="row.status === 'pending'" theme="success" hover="color" @click="openAuditDialog(row, 'approve')">
              通过
            </t-link>
            <t-link v-if="row.status === 'pending'" theme="danger" hover="color" @click="openAuditDialog(row, 'reject')">
              驳回
            </t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无提现单" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="auditVisible"
      :header="auditForm.action === 'approve' ? '通过提现' : '驳回提现'"
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

import { RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { approveWithdraw, getWithdrawList, rejectWithdraw } from '@/api/finance'
import {
  formatPrice,
  formatTime,
  toDateString,
  withdrawStatusLabel,
  withdrawStatusOptions,
  withdrawStatusTheme,
} from '@/pages/finance/constants'
import type { WithdrawInfo } from '@/types/interface'

defineOptions({ name: 'FinanceWithdrawals' })

const withdrawList = ref<WithdrawInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  user_id: string | undefined
  status: string | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  user_id: undefined,
  status: undefined,
  dateRange: [],
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<WithdrawInfo>[] = [
  { colKey: 'withdraw_no', title: '提现单号', minWidth: 180 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'account', title: '收款账户', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'audited_at', title: '审核时间', width: 170 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: 130,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadWithdrawals() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getWithdrawList({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      status: filters.status,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    withdrawList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提现单失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadWithdrawals()
}

function handleSearch() {
  pagination.current = 1
  loadWithdrawals()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.status = undefined
  filters.dateRange = []
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

function openAuditDialog(row: WithdrawInfo, action: 'approve' | 'reject') {
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
      await approveWithdraw(auditForm.id, { remark: auditForm.remark })
    } else {
      await rejectWithdraw(auditForm.id, { remark: auditForm.remark })
    }
    MessagePlugin.success(auditForm.action === 'approve' ? '已通过提现' : '已驳回提现')
    auditVisible.value = false
    loadWithdrawals()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '审核失败')
  }
}

onMounted(loadWithdrawals)
</script>

<style lang="css">
@import '../shared.css';
</style>
