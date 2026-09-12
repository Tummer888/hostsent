<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FileIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">发票管理</h2>
          <p class="page-header__desc">
            用户对已结清账单申请开票后在此审核：填写发票号即完成开票；渠道与外部回执号字段已预埋，后续可接入税控系统自动开票与邮件下发。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadInvoices">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
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
          <span class="field__label">账单号</span>
          <t-input v-model="filters.bill_no" placeholder="按账单号筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">申请状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="invoiceRequestStatusOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">开票申请</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="invoiceList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #request_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.request_no }}</span>
            <span class="price-sub">账单 {{ row.bill_no }}</span>
          </div>
        </template>

        <template #user="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">ID {{ row.user_id }}</span>
          </div>
        </template>

        <template #title="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.title }}</span>
            <span class="price-sub">{{ invoiceTypeLabel(row.invoice_type) }}{{ row.tax_no ? ` · ${row.tax_no}` : '' }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #email="{ row }">
          <span class="price-sub">{{ row.email || '—' }}</span>
        </template>

        <template #status="{ row }">
          <div class="price-cell">
            <t-tag :theme="invoiceRequestStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ invoiceRequestStatusLabel(row.status) }}
            </t-tag>
            <span v-if="row.external_no" class="price-sub">{{ row.external_no }}</span>
            <span v-else-if="row.reject_reason" class="price-sub">{{ row.reject_reason }}</span>
          </div>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <template v-if="row.status === 'pending'">
              <MobileAction
                v-if="isMobile"
                :options="buildMobileActionOptions([
                  { content: '开票', value: 'issue', theme: 'success' },
                  { content: '驳回', value: 'reject', theme: 'error' },
                ])"
                @select="(value) => handleMobileAction(value, row)"
              />
              <template v-else>
                <t-link theme="success" hover="color" @click="openIssue(row)">开票</t-link>
                <t-link theme="danger" hover="color" @click="openReject(row)">驳回</t-link>
              </template>
            </template>
            <span v-else class="price-sub">已处理</span>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无开票申请" />
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
      v-model:visible="issueVisible"
      header="开具发票"
      width="480px"
      :confirm-btn="{ content: '确认开票', theme: 'success' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleIssue"
      @close="issueVisible = false"
    >
      <t-form label-align="top" :data="issueForm" @submit.prevent>
        <t-alert
          theme="info"
          :message="`申请单 ${issueForm.label}，金额 ¥${formatPrice(issueForm.amount)}`"
          style="margin-bottom: 12px"
        />
        <t-form-item label="发票号（必填）">
          <t-input v-model="issueForm.invoice_no" placeholder="税控系统发票号码" clearable />
        </t-form-item>
        <t-form-item label="发票文件地址（预埋，选填）">
          <t-input v-model="issueForm.file_url" placeholder="PDF 下载地址，后续可邮件下发用户" clearable />
        </t-form-item>
        <t-form-item label="开具渠道">
          <t-select v-model="issueForm.channel" :options="invoiceChannelOptions" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="rejectVisible"
      header="驳回开票申请"
      width="460px"
      :confirm-btn="{ content: '确认驳回', theme: 'danger' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleReject"
      @close="rejectVisible = false"
    >
      <t-form label-align="top" :data="rejectForm" @submit.prevent>
        <t-form-item label="驳回原因">
          <t-textarea v-model="rejectForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="如：抬头与账单主体不一致" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { FileIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getInvoiceList, issueInvoice, rejectInvoice } from '@/api/finance'
import {
  formatPrice,
  formatTime,
  invoiceRequestStatusLabel,
  invoiceRequestStatusOptions,
  invoiceRequestStatusTheme,
  invoiceTypeLabel,
} from '@/pages/finance/constants'
import type { InvoiceInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'FinanceInvoices' })

const invoiceList = ref<InvoiceInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

// 开票渠道：manual 人工开票；tax_api 为后续接入税控自动开票预留。
const invoiceChannelOptions = [
  { label: '人工开票', value: 'manual' },
  { label: '税控系统（预留）', value: 'tax_api' },
]

const filters = reactive<{ user_id: string | undefined; bill_no: string | undefined; status: string | undefined }>({
  user_id: undefined,
  bill_no: undefined,
  status: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const issueVisible = ref(false)
const issueForm = reactive<{ id: number; label: string; amount: number; invoice_no: string; file_url: string; channel: string }>({
  id: 0,
  label: '',
  amount: 0,
  invoice_no: '',
  file_url: '',
  channel: 'manual',
})

const rejectVisible = ref(false)
const rejectForm = reactive<{ id: number; reason: string }>({ id: 0, reason: '' })

const columns: PrimaryTableCol<InvoiceInfo>[] = [
  { colKey: 'request_no', title: '申请单 / 账单', minWidth: 200 },
  { colKey: 'user', title: '用户', minWidth: 140 },
  { colKey: 'title', title: '抬头 / 税号', minWidth: 200 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'email', title: '接收邮箱', minWidth: 160 },
  { colKey: 'status', title: '状态', width: 150 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 130, fixed: 'right' as const, align: 'center' as const },
]

async function loadInvoices() {
  loading.value = true
  try {
    const data = await getInvoiceList({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      bill_no: filters.bill_no,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    invoiceList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载开票申请失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadInvoices()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

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
  loadInvoices()
}

function handleResetFilters() {
  filters.user_id = undefined
  filters.bill_no = undefined
  filters.status = undefined
  pagination.current = 1
  loadInvoices()
}

function openIssue(row: InvoiceInfo) {
  issueForm.id = row.id
  issueForm.label = row.request_no
  issueForm.amount = row.amount
  issueForm.invoice_no = ''
  issueForm.file_url = ''
  issueForm.channel = 'manual'
  issueVisible.value = true
}

async function handleIssue() {
  if (!issueForm.invoice_no.trim()) {
    MessagePlugin.warning('请填写发票号')
    return
  }
  loading.value = true
  try {
    await issueInvoice(issueForm.id, {
      invoice_no: issueForm.invoice_no.trim(),
      file_url: issueForm.file_url.trim() || undefined,
      channel: issueForm.channel,
    })
    MessagePlugin.success('发票已开具')
    issueVisible.value = false
    loadInvoices()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '开票失败')
  } finally {
    loading.value = false
  }
}

function openReject(row: InvoiceInfo) {
  rejectForm.id = row.id
  rejectForm.reason = ''
  rejectVisible.value = true
}

async function handleReject() {
  loading.value = true
  try {
    await rejectInvoice(rejectForm.id, rejectForm.reason)
    MessagePlugin.success('申请已驳回')
    rejectVisible.value = false
    loadInvoices()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '驳回失败')
  } finally {
    loading.value = false
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: InvoiceInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'issue':
      openIssue(row)
      break
    case 'reject':
      openReject(row)
      break
  }
}

onMounted(loadInvoices)
</script>

<style lang="css">
@import '../shared.css';
</style>
