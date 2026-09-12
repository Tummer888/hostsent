<template>
  <div class="invoice-page">
    <section class="panel">
      <div class="panel-head">
        <div class="panel-head__text">
          <h3 class="section-title">我的发票</h3>
          <p class="section-desc">
            发票只能针对本人<strong>已结清</strong>的账单申请；开具后可在本页查看发票号与下载地址。
          </p>
        </div>
        <t-space size="small">
          <t-button variant="outline" size="small" :loading="loading" @click="loadInvoices">
            <template #icon><RefreshIcon /></template>
            刷新
          </t-button>
          <t-button variant="outline" size="small" @click="router.push('/billing')">返回费用中心</t-button>
        </t-space>
      </div>

      <div class="filter-bar">
        <t-select v-model="filter.status" clearable placeholder="全部状态" :options="invoiceRequestStatusOptions" style="width: 140px" @change="handleSearch" />
        <t-input v-model="filter.bill_no" placeholder="按账单号查询" clearable style="width: 200px" @enter="handleSearch" />
        <t-button theme="primary" size="small" @click="handleSearch">查询</t-button>
        <t-button variant="outline" size="small" @click="handleReset">重置</t-button>
      </div>

      <t-table
        :data="invoices"
        :columns="columns"
        size="small"
        row-key="id"
        :pagination="pagination"
        :bordered="false"
        hover
        cell-empty-content="—"
        :loading="loading"
        @page-change="handlePageChange"
      >
        <template #request_no="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ row.request_no }}</span>
            <span class="cell-sub">账单 {{ row.bill_no }}</span>
          </div>
        </template>
        <template #title="{ row }">
          <div class="cell-main">
            <span class="cell-strong">{{ row.title }}</span>
            <span class="cell-sub">{{ invoiceTypeLabel(row.invoice_type) }}{{ row.tax_no ? ` · ${row.tax_no}` : '' }}</span>
          </div>
        </template>
        <template #amount="{ row }">
          <span class="num-cell num-cell--strong">¥ {{ formatPrice(row.amount) }}</span>
        </template>
        <template #status="{ row }">
          <div class="cell-main">
            <t-tag :theme="invoiceRequestStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ invoiceRequestStatusLabel(row.status) }}
            </t-tag>
            <span v-if="row.status === 'issued' && row.external_no" class="cell-sub">发票号 {{ row.external_no }}</span>
            <span v-else-if="row.status === 'rejected' && row.reject_reason" class="cell-sub">{{ row.reject_reason }}</span>
          </div>
        </template>
        <template #file_url="{ row }">
          <t-space size="small">
            <t-link
              v-if="row.status === 'issued'"
              theme="primary"
              hover="color"
              @click="handleDownload(row)"
            >
              下载发票
            </t-link>
            <t-link
              v-if="row.status === 'issued'"
              theme="default"
              hover="color"
              @click="handleEmail(row)"
            >
              邮件发送
            </t-link>
            <span v-if="row.status !== 'issued'" class="time-text">—</span>
          </t-space>
        </template>
        <template #email="{ row }">
          <span class="time-text">{{ row.email || '—' }}</span>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无开票申请，可在费用中心对已结清账单申请开票" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { downloadInvoice, emailInvoice, getMyInvoices, type InvoiceInfo } from '@/api/finance'
import {
  formatPrice,
  formatTime,
  invoiceRequestStatusLabel,
  invoiceRequestStatusOptions,
  invoiceRequestStatusTheme,
  invoiceTypeLabel,
} from '@/pages/billing/constants'

defineOptions({ name: 'BillingInvoices' })

const router = useRouter()
const invoices = ref<InvoiceInfo[]>([])
const loading = ref(false)

const filter = reactive<{ status: string | undefined; bill_no: string | undefined }>({
  status: undefined,
  bill_no: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })

const columns: PrimaryTableCol<InvoiceInfo>[] = [
  { colKey: 'request_no', title: '申请单 / 账单', minWidth: 200 },
  { colKey: 'title', title: '抬头 / 税号', minWidth: 200 },
  { colKey: 'amount', title: '开票金额', width: 120, align: 'right' },
  { colKey: 'status', title: '状态', width: 170 },
  { colKey: 'file_url', title: '发票文件', width: 110 },
  { colKey: 'email', title: '接收邮箱', minWidth: 160 },
  { colKey: 'created_at', title: '申请时间', width: 170 },
]

async function loadInvoices() {
  loading.value = true
  try {
    const { data } = await getMyInvoices({
      status: filter.status,
      bill_no: filter.bill_no,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    if (data) {
      invoices.value = data.items
      pagination.total = data.meta.total
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载发票列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadInvoices()
}

function handleReset() {
  filter.status = undefined
  filter.bill_no = undefined
  pagination.current = 1
  loadInvoices()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadInvoices()
}

// 取件走接口而不是直接用 row.file_url：由后端统一判定归属、开票状态与文件是否就绪。
// 失败提示由 request 拦截器统一弹出，这里不再重复提示。
async function handleDownload(row: InvoiceInfo) {
  try {
    const { data } = await downloadInvoice(row.id)
    if (data?.file_url) {
      window.open(data.file_url, '_blank')
    } else {
      MessagePlugin.warning('发票文件尚未就绪，请联系客服')
    }
  } catch {
    // 拦截器已提示「发票文件尚未就绪」
  }
}

// 邮件下发为预埋能力：本轮后端明确返回「未接入」，提示由拦截器弹出，引导用户走人工。
async function handleEmail(row: InvoiceInfo) {
  try {
    await emailInvoice(row.id)
    MessagePlugin.success('发票已发送至接收邮箱')
  } catch {
    // 拦截器已提示「发票邮件下发尚未接入」
  }
}

onMounted(loadInvoices)
</script>

<style scoped>
.invoice-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.panel {
  padding: 20px 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.panel-head__text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.section-desc {
  margin: 0;
  font-size: 12.5px;
  color: #64748b;
  line-height: 1.6;
}

.filter-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px 0 16px;
}

.cell-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-strong {
  font-weight: 600;
  color: #334155;
}

.cell-sub {
  font-size: 12px;
  color: #64748b;
}

.num-cell {
  font-variant-numeric: tabular-nums;
  color: #334155;
}

.num-cell--strong {
  font-weight: 600;
  color: #1e293b;
}

.time-text {
  color: #64748b;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
</style>
