<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">充值管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" @click="openCreateDialog">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          登记充值
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadRecharges">
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
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="rechargeStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">支付方式</span>
          <t-select v-model="filters.method" clearable placeholder="全部方式" :options="rechargeMethodOptions" />
        </div>
        <div class="field">
          <span class="field__label">创建时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">充值单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个充值单</span>
      </div>
      <t-table
        row-key="id"
        :data="rechargeList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #recharge_no="{ row }">
          <span class="cell-strong">{{ row.recharge_no }}</span>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #method="{ row }">
          <span>{{ rechargeMethodLabel(row.method) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="rechargeStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ rechargeStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link v-if="row.status === 'pending'" theme="success" hover="color" @click="openConfirmDialog(row)">
              确认到账
            </t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无充值单" />
        </template>
      </t-table>
    </section>

    <!-- 登记充值 -->
    <t-dialog
      v-model:visible="createVisible"
      header="登记充值"
      width="480px"
      :confirm-btn="{ content: '提交', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreate"
      @close="createVisible = false"
    >
      <t-form label-align="top" :data="createForm" @submit.prevent>
        <t-form-item label="用户 ID" name="user_id">
          <t-input v-model="createForm.user_id" placeholder="请输入用户 ID" />
        </t-form-item>
        <t-form-item label="充值金额（元）" name="amount">
          <t-input-number v-model="createForm.amount" :min="0" :precision="2" theme="column" placeholder="请输入金额" />
        </t-form-item>
        <t-form-item label="支付方式" name="method">
          <t-select v-model="createForm.method" placeholder="请选择方式" :options="rechargeMethodOptions" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="createForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 确认到账 -->
    <t-dialog
      v-model:visible="confirmVisible"
      header="确认到账"
      width="480px"
      :confirm-btn="{ content: '确认到账', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleConfirm"
      @close="confirmVisible = false"
    >
      <t-form label-align="top" :data="confirmForm" @submit.prevent>
        <t-alert theme="info" :message="`充值单 ${confirmForm.label}，确认后即将 ¥${formatPrice(confirmForm.amount)} 计入用户余额`" style="margin-bottom: 12px" />
        <t-form-item label="渠道交易号" name="channel_tx">
          <t-input v-model="confirmForm.channel_tx" placeholder="选填，渠道流水编号" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-input v-model="confirmForm.remark" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { approveRecharge, createRecharge, getRechargeList } from '@/api/finance'
import {
  formatPrice,
  formatTime,
  rechargeMethodLabel,
  rechargeMethodOptions,
  rechargeStatusLabel,
  rechargeStatusOptions,
  rechargeStatusTheme,
  toDateString,
} from '@/pages/finance/constants'
import type { RechargeInfo } from '@/types/interface'

defineOptions({ name: 'FinanceRecharges' })

const rechargeList = ref<RechargeInfo[]>([])
const loading = ref(false)
const total = ref(0)

const filters = reactive<{
  user_id: string | undefined
  status: string | undefined
  method: string | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  user_id: undefined,
  status: undefined,
  method: undefined,
  dateRange: [],
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<RechargeInfo>[] = [
  { colKey: 'recharge_no', title: '充值单号', minWidth: 180 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 130 },
  { colKey: 'method', title: '支付方式', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'channel_tx', title: '渠道交易号', minWidth: 140 },
  { colKey: 'remark', title: '备注', minWidth: 120 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: 120,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadRecharges() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getRechargeList({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      status: filters.status,
      method: filters.method,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    rechargeList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载充值单失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadRecharges()
}

function handleSearch() {
  pagination.current = 1
  loadRecharges()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.status = undefined
  filters.method = undefined
  filters.dateRange = []
  pagination.current = 1
  loadRecharges()
}

const createVisible = ref(false)
const createForm = reactive<{
  user_id: string | undefined
  amount: number | undefined
  method: string | undefined
  remark: string | undefined
}>({
  user_id: undefined,
  amount: undefined,
  method: undefined,
  remark: undefined,
})

function openCreateDialog() {
  createForm.user_id = undefined
  createForm.amount = undefined
  createForm.method = undefined
  createForm.remark = undefined
  createVisible.value = true
}

async function handleCreate() {
  const userId = Number(createForm.user_id)
  if (!userId || userId <= 0) {
    MessagePlugin.warning('请输入有效的用户 ID')
    return
  }
  if (!createForm.amount || createForm.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的充值金额')
    return
  }
  if (!createForm.method) {
    MessagePlugin.warning('请选择支付方式')
    return
  }
  try {
    await createRecharge({
      user_id: userId,
      amount: createForm.amount,
      method: createForm.method,
      remark: createForm.remark,
    })
    MessagePlugin.success('充值单已登记，等待确认到账')
    createVisible.value = false
    loadRecharges()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '登记充值失败')
  }
}

const confirmVisible = ref(false)
const confirmForm = reactive<{
  id: number
  label: string
  amount: number
  channel_tx: string | undefined
  remark: string | undefined
}>({
  id: 0,
  label: '',
  amount: 0,
  channel_tx: undefined,
  remark: undefined,
})

function openConfirmDialog(row: RechargeInfo) {
  confirmForm.id = row.id
  confirmForm.label = row.recharge_no
  confirmForm.amount = row.amount
  confirmForm.channel_tx = row.channel_tx || undefined
  confirmForm.remark = row.remark || undefined
  confirmVisible.value = true
}

async function handleConfirm() {
  try {
    await approveRecharge(confirmForm.id, {
      channel_tx: confirmForm.channel_tx,
      remark: confirmForm.remark,
    })
    MessagePlugin.success('已确认到账')
    confirmVisible.value = false
    loadRecharges()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '确认到账失败')
  }
}

onMounted(loadRecharges)
</script>

<style lang="css">
@import '../shared.css';
</style>
