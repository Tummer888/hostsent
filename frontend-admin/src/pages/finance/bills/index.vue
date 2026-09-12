<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">账单管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" @click="openGenerate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          生成账单
        </t-button>
        <t-button variant="outline" :loading="loading" @click="loadBills">
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
          <span class="field__label">用户账号</span>
          <t-input v-model="filters.user_keyword" placeholder="用户名或邮箱" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">账单号</span>
          <t-input v-model="filters.keyword" placeholder="账单号模糊匹配" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">账期</span>
          <t-input v-model="filters.period" placeholder="如 202608" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">账单分类</span>
          <t-select v-model="filters.bill_type" clearable placeholder="全部分类" :options="billTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="billStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">发票状态</span>
          <t-select v-model="filters.invoice_status" clearable placeholder="全部" :options="invoiceStatusOptions" />
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
        <h3 class="card-title">账单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个账单</span>
      </div>
      <t-table
        row-key="id"
        :data="billList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #bill_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.bill_no }}</span>
            <span class="price-sub">
              <t-tag :theme="billTypeTheme(row.bill_type)" variant="light" size="small" shape="round">
                {{ billTypeLabel(row.bill_type) }}
              </t-tag>
            </span>
          </div>
        </template>

        <template #total_amount="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.total_amount) }}</span>
            <span class="price-sub">
              消费 {{ formatPrice(row.consume_amount) }} · 续费 {{ formatPrice(row.renewal_amount) }}
            </span>
            <!-- 原路退回扣点只在收入统计基数上再扣一次：账单应结与票面仍是 total_amount -->
            <span v-if="row.refund_fee_amount > 0" class="price-sub">
              收入口径 ¥{{ formatPrice(row.net_amount) }}（含扣点 {{ formatPrice(row.refund_fee_amount) }}）
            </span>
          </div>
        </template>

        <template #refund_amount="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.refund_amount) }}</span>
            <span v-if="row.channel_refund_amount > 0" class="price-sub">
              原路 ¥{{ formatPrice(row.channel_refund_amount) }} · 扣点 ¥{{ formatPrice(row.refund_fee_amount) }}
            </span>
            <span v-else class="price-sub">余额退回（消费口径不变）</span>
          </div>
        </template>

        <template #paid="{ row }">
          <div v-if="row.paid_method || row.paid_amount > 0" class="price-cell">
            <span class="cell-strong">{{ payMethodLabel(row.paid_method) }}</span>
            <span class="price-sub">实收 ¥{{ formatPrice(row.paid_amount) }}</span>
          </div>
          <span v-else class="price-sub">—</span>
        </template>

        <template #invoice_status="{ row }">
          <div class="price-cell">
            <t-tag :theme="invoiceStatusTheme(row.invoice_status)" variant="light" size="small" shape="round">
              {{ invoiceStatusLabel(row.invoice_status) }}
            </t-tag>
            <span v-if="row.invoice_no" class="price-sub">{{ row.invoice_no }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="billStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ billStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '关账', value: 'close', hidden: () => !(row.status === 'unpaid'), theme: 'warning' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link v-if="row.status === 'unpaid'" theme="warning" hover="color" @click="handleClose(row)">关账</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无账单" />
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
      v-model:visible="generateVisible"
      header="生成账单"
      width="460px"
      :confirm-btn="{ content: '生成', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleGenerate"
      @close="generateVisible = false"
    >
      <t-form label-align="top" :data="generateForm" @submit.prevent>
        <t-alert
          theme="info"
          message="按「用户 + 账期」归集当期消费、续费与原路退款扣点，重复生成会覆盖同一账期账单。"
          style="margin-bottom: 12px"
        />
        <t-form-item label="用户 ID">
          <t-input v-model="generateForm.user_id" placeholder="请输入用户 ID" clearable />
        </t-form-item>
        <t-form-item label="账期（YYYYMM）">
          <t-input v-model="generateForm.period" placeholder="如 202608" clearable />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, SearchIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { closeBill, generateBill, getBillList } from '@/api/finance'
import {
  billStatusLabel,
  billStatusOptions,
  billStatusTheme,
  billTypeLabel,
  billTypeOptions,
  billTypeTheme,
  formatPrice,
  formatTime,
  invoiceStatusLabel,
  invoiceStatusOptions,
  invoiceStatusTheme,
  payMethodLabel,
} from '@/pages/finance/constants'
import type { BillInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'FinanceBills' })

const billList = ref<BillInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{
  user_keyword: string | undefined
  user_id: string | undefined
  keyword: string | undefined
  period: string | undefined
  bill_type: string | undefined
  status: string | undefined
  invoice_status: string | undefined
}>({
  user_keyword: undefined,
  user_id: undefined,
  keyword: undefined,
  period: undefined,
  bill_type: undefined,
  status: undefined,
  invoice_status: undefined,
})

// 生成账单弹窗（doc36 §7-2）：手动归集某用户某账期。
const generateVisible = ref(false)
const generateForm = reactive<{ user_id: string | undefined; period: string | undefined }>({
  user_id: undefined,
  period: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers/loadData）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<BillInfo>[] = [
  { colKey: 'bill_no', title: '账单号 / 分类', minWidth: 190 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'period', title: '账期', width: 100 },
  { colKey: 'total_amount', title: '应结金额', width: 170 },
  { colKey: 'refund_amount', title: '退款', width: 170 },
  { colKey: 'paid', title: '支付方式 / 实收', width: 150 },
  { colKey: 'invoice_status', title: '发票', width: 130 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 120,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadBills() {
  loading.value = true
  try {
    const data = await getBillList({
      user_keyword: filters.user_keyword,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      keyword: filters.keyword,
      period: filters.period,
      bill_type: filters.bill_type,
      status: filters.status,
      invoice_status: filters.invoice_status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    billList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载账单失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadBills()
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
  loadBills()
}

function handleResetFilters() {
  filters.user_keyword = undefined
  filters.user_id = undefined
  filters.keyword = undefined
  filters.period = undefined
  filters.bill_type = undefined
  filters.status = undefined
  filters.invoice_status = undefined
  pagination.current = 1
  loadBills()
}

// —— 生成账单（doc36 §7-2）——
function openGenerate() {
  generateForm.user_id = filters.user_id
  generateForm.period = filters.period
  generateVisible.value = true
}

async function handleGenerate() {
  const userId = Number(generateForm.user_id)
  if (!userId || userId <= 0) {
    MessagePlugin.warning('请输入有效的用户 ID')
    return
  }
  if (!generateForm.period || !/^\d{6}$/.test(generateForm.period.trim())) {
    MessagePlugin.warning('账期格式应为 YYYYMM，如 202608')
    return
  }
  loading.value = true
  try {
    const bill = await generateBill({ user_id: userId, period: generateForm.period.trim() })
    MessagePlugin.success(`账单 ${bill.bill_no} 已生成`)
    generateVisible.value = false
    loadBills()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '生成账单失败')
  } finally {
    loading.value = false
  }
}

function handleClose(row: BillInfo) {
  const dialog = DialogPlugin.confirm({
    header: '关账确认',
    body: `确认将账单「${row.bill_no}」（账期 ${row.period}）关账吗？关账后不再允许修改。`,
    confirmBtn: { content: '确认关账', theme: 'warning' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        await closeBill(row.id)
        MessagePlugin.success('账单已关账')
        dialog.destroy()
        loadBills()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '关账失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadBills)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: BillInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'close':
      void handleClose(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>
