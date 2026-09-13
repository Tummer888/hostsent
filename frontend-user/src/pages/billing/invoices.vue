<template>
  <div class="page-body console-module invoice-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FilePasteIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">我的发票</h2>
          <p class="page-header__desc">
            只能对本人<strong>已结清</strong>的账单申请开票；开具后可在此查看发票号与下载
          </p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/billing')">返回费用中心</t-button>
        <t-button variant="outline" :loading="loading" @click="loadInvoices">刷新</t-button>
      </div>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__grid">
        <div class="field">
          <label class="field__label">申请状态</label>
          <t-select v-model="filter.status" clearable placeholder="全部状态" :options="invoiceRequestStatusOptions" @change="handleSearch" />
        </div>
        <div class="field">
          <label class="field__label">账单号</label>
          <t-input v-model="filter.bill_no" placeholder="按账单号查询" clearable @enter="handleSearch" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-button variant="outline" @click="handleReset">重置</t-button>
        <t-button theme="primary" :loading="loading" @click="handleSearch">查询</t-button>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">开票申请</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 条</span>
      </div>

      <t-table
        :data="invoices"
        :columns="columns"
        row-key="id"
        :pagination="isMobile ? undefined : pagination"
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
            <span class="cell-sub">
              {{ invoiceTypeLabel(row.invoice_type) }}{{ row.tax_no ? ` · ${row.tax_no}` : '' }}
            </span>
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
            <span v-if="row.status === 'issued' && row.external_no" class="cell-sub">
              发票号 {{ row.external_no }}
            </span>
            <span v-else-if="row.status === 'rejected' && row.reject_reason" class="cell-sub">
              {{ row.reject_reason }}
            </span>
          </div>
        </template>
        <template #file_url="{ row }">
          <span class="action-cell">
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
          </span>
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

      <!-- 移动端翻页：与 admin 列表页同一套（桌面用表格内建分页） -->
      <MobilePagination
        v-if="isMobile"
        :current="pagination.current"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        @change="handlePageChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { FilePasteIcon } from 'tdesign-icons-vue-next'
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
import { useIsMobile } from '@/composables/useIsMobile'
import MobilePagination from '@/components/mobile-pagination/index.vue'

defineOptions({ name: 'BillingInvoices' })

const router = useRouter()
const { isMobile } = useIsMobile()

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
  { colKey: 'file_url', title: '发票文件', width: 140 },
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
/* 数字列用等宽数字；cell-main / cell-strong / cell-sub / time-text / action-cell
   均由 console-module 骨架契约提供。 */
.num-cell {
  font-variant-numeric: tabular-nums;
}

.num-cell--strong {
  font-weight: 600;
}
</style>
