<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <OrderIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">支付订单</h2>
          <p class="page-header__desc">支付单以业务类型 + 业务单号与账单/充值/订单解耦；线下渠道待支付单可人工确认到账。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadOrders">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon"><MoneyIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(paidAmount) }}</span>
          <span class="stat-card__label">本页已支付金额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><TimeIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ pendingCount }}</span>
          <span class="stat-card__label">本页待支付</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><OrderIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ total }}</span>
          <span class="stat-card__label">支付单总数</span>
        </div>
      </div>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">支付单号</span>
          <t-input v-model="filters.payment_no" placeholder="如 P20260912…" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="filters.user_id" placeholder="按用户 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">业务类型</span>
          <t-select v-model="filters.biz_type" clearable placeholder="全部业务" :options="bizTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">支付渠道</span>
          <t-input v-model="filters.channel_code" placeholder="渠道编码" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="orderStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">创建时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
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
        <h3 class="card-title">支付单列表</h3>
        <span class="table-card__meta">共 {{ total }} 笔</span>
      </div>
      <t-table
        row-key="id"
        :data="orderList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #payment_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.payment_no }}</span>
            <span class="price-sub">{{ row.subject || '—' }}</span>
          </div>
        </template>

        <template #user_id="{ row }">
          <span class="cell-muted">#{{ row.user_id }}</span>
        </template>

        <template #biz="{ row }">
          <div class="price-cell">
            <t-tag theme="default" variant="light" size="small" shape="round">{{ bizTypeLabel(row.biz_type) }}</t-tag>
            <span class="price-sub">{{ row.biz_no || '—' }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <span class="price-main">¥{{ formatPrice(row.amount) }}</span>
        </template>

        <template #channel="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.channel_name || row.channel_code }}</span>
            <span class="price-sub">{{ row.channel_code }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="orderStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ orderStatusLabel(row.status) }}
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
                { content: '详情', value: 'detail' },
                { content: '确认到账', value: 'confirm', hidden: () => !isManualPending(row) || !canOperate, theme: 'success' },
                { content: '主动查单', value: 'sync', hidden: () => !canOperate },
                { content: '关闭', value: 'close', hidden: () => !isClosable(row) || !canOperate, theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="isManualPending(row) && canOperate" theme="success" hover="color" @click="openConfirm(row)">确认到账</t-link>
              <t-link v-if="canOperate" theme="primary" hover="color" @click="handleSync(row)">主动查单</t-link>
              <t-link v-if="isClosable(row) && canOperate" theme="danger" hover="color" @click="handleClose(row)">关闭</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无支付单" />
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

    <!-- 支付单详情 -->
    <t-drawer v-model:visible="detailVisible" header="支付单详情" size="560px" :footer="false">
      <template v-if="detail">
        <div class="detail-block">
          <div class="recon-row"><span class="recon-row__label">支付单号</span><span class="recon-row__value">{{ detail.payment_no }}</span></div>
          <div class="recon-row"><span class="recon-row__label">用户 ID</span><span class="recon-row__value">#{{ detail.user_id }}</span></div>
          <div class="recon-row"><span class="recon-row__label">业务</span><span class="recon-row__value">{{ bizTypeLabel(detail.biz_type) }} · {{ detail.biz_no || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">金额</span><span class="recon-row__value">¥{{ formatPrice(detail.amount) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">手续费</span><span class="recon-row__value">¥{{ formatPrice(detail.fee) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">渠道</span><span class="recon-row__value">{{ detail.channel_name || detail.channel_code }}（{{ detail.channel_type }}）</span></div>
          <div class="recon-row"><span class="recon-row__label">场景</span><span class="recon-row__value">{{ sceneLabel(detail.scene) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">状态</span><span class="recon-row__value">{{ orderStatusLabel(detail.status) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">渠道交易号</span><span class="recon-row__value">{{ detail.channel_tx || '—' }}</span></div>
          <div class="recon-row"><span class="recon-row__label">过期时间</span><span class="recon-row__value">{{ formatTime(detail.expire_at) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">支付时间</span><span class="recon-row__value">{{ formatTime(detail.paid_at) }}</span></div>
          <div class="recon-row"><span class="recon-row__label">创建时间</span><span class="recon-row__value">{{ formatTime(detail.created_at) }}</span></div>
        </div>

        <div v-if="detail.instructions" class="detail-block">
          <h4 class="detail-block__title">支付指引</h4>
          <pre class="detail-pre">{{ detail.instructions }}</pre>
        </div>
        <div v-if="detail.pay_url" class="detail-block">
          <h4 class="detail-block__title">支付链接</h4>
          <a class="detail-link" :href="detail.pay_url" target="_blank" rel="noopener noreferrer">{{ detail.pay_url }}</a>
        </div>
        <div v-if="detail.prepay_params" class="detail-block">
          <h4 class="detail-block__title">渠道下单参数</h4>
          <pre class="detail-pre">{{ prettyJSON(detail.prepay_params) }}</pre>
        </div>
      </template>
    </t-drawer>

    <!-- 人工确认到账 -->
    <t-dialog
      v-model:visible="confirmVisible"
      header="人工确认到账"
      width="480px"
      :confirm-btn="{ content: '确认到账', theme: 'primary', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleConfirm"
      @close="confirmVisible = false"
    >
      <t-alert
        v-if="confirmTarget"
        theme="warning"
        :message="`确认后支付单 ${confirmTarget.payment_no} 将标记为已支付，并按业务类型入账/结清（¥${formatPrice(confirmTarget.amount)}）。`"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top" :data="confirmForm" @submit.prevent>
        <t-form-item label="渠道交易号" name="channel_tx">
          <t-input v-model="confirmForm.channel_tx" placeholder="选填，线下转账流水号" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-input v-model="confirmForm.remark" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MoneyIcon, OrderIcon, RefreshIcon, SearchIcon, TimeIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  closePaymentOrder,
  confirmPaymentOrder,
  getPaymentOrders,
  syncPaymentOrder,
} from '@/api/payment'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import {
  bizTypeLabel,
  bizTypeOptions,
  formatPrice,
  formatTime,
  orderStatusLabel,
  orderStatusOptions,
  orderStatusTheme,
  sceneLabel,
  toDateString,
} from '@/pages/payment/constants'
import type { PaymentOrderInfo } from '@/types/interface'
import { useUserStore } from '@/store'

defineOptions({ name: 'PaymentOrders' })

const userStore = useUserStore()
const canOperate = computed(() => userStore.permissions?.includes('payment:order:operate') || userStore.isSuperAdmin)

const orderList = ref<PaymentOrderInfo[]>([])
const loading = ref(false)
const total = ref(0)
const { isMobile } = useIsMobile()

const filters = reactive<{
  payment_no?: string
  user_id?: string
  biz_type?: string
  channel_code?: string
  status?: string
  dateRange: (string | Date | undefined)[] | undefined
}>({
  payment_no: undefined,
  user_id: undefined,
  biz_type: undefined,
  channel_code: undefined,
  status: undefined,
  dateRange: [],
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const paidAmount = computed(() => orderList.value.filter((o) => o.status === 'paid').reduce((sum, o) => sum + o.amount, 0))
const pendingCount = computed(() => orderList.value.filter((o) => o.status === 'pending' || o.status === 'paying').length)

const columns: PrimaryTableCol<PaymentOrderInfo>[] = [
  { colKey: 'payment_no', title: '支付单', minWidth: 200 },
  { colKey: 'user_id', title: '用户', width: 90, align: 'center' as const },
  { colKey: 'biz', title: '业务', minWidth: 160 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'channel', title: '渠道', minWidth: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 260, fixed: 'right' as const, align: 'center' as const },
]

function isManualPending(row: PaymentOrderInfo): boolean {
  return row.status === 'pending' || row.status === 'paying'
}

function isClosable(row: PaymentOrderInfo): boolean {
  return row.status === 'pending' || row.status === 'paying'
}

async function loadOrders() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getPaymentOrders({
      payment_no: filters.payment_no,
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      biz_type: filters.biz_type,
      channel_code: filters.channel_code,
      status: filters.status,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    orderList.value = data.items || []
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载支付单失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  loadOrders()
}

function handleResetFilters() {
  filters.payment_no = undefined
  filters.user_id = undefined
  filters.biz_type = undefined
  filters.channel_code = undefined
  filters.status = undefined
  filters.dateRange = []
  pagination.current = 1
  loadOrders()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  loadOrders()
}

function goMobilePage(target: number) {
  const maxPage = Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize))
  const clamped = Math.min(Math.max(target, 1), maxPage)
  if (clamped === mobilePage.current) return
  handlePageChange({ current: clamped, pageSize: mobilePage.pageSize } as PageInfo)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  handlePageChange({ current: 1, pageSize } as PageInfo)
}

// ===== 详情 =====
const detailVisible = ref(false)
const detail = ref<PaymentOrderInfo | null>(null)

function openDetail(row: PaymentOrderInfo) {
  detail.value = row
  detailVisible.value = true
}

function prettyJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

// ===== 确认到账 =====
const confirmVisible = ref(false)
const confirmTarget = ref<PaymentOrderInfo | null>(null)
const submitting = ref(false)
const confirmForm = reactive<{ channel_tx?: string; remark?: string }>({ channel_tx: undefined, remark: undefined })

function openConfirm(row: PaymentOrderInfo) {
  confirmTarget.value = row
  confirmForm.channel_tx = row.channel_tx || undefined
  confirmForm.remark = undefined
  confirmVisible.value = true
}

async function handleConfirm() {
  if (!confirmTarget.value) return
  submitting.value = true
  try {
    await confirmPaymentOrder(confirmTarget.value.id, {
      channel_tx: confirmForm.channel_tx,
      remark: confirmForm.remark,
    })
    MessagePlugin.success('已确认到账并触发入账')
    confirmVisible.value = false
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '确认到账失败')
  } finally {
    submitting.value = false
  }
}

// ===== 主动查单 =====
async function handleSync(row: PaymentOrderInfo) {
  try {
    await syncPaymentOrder(row.id)
    MessagePlugin.success('已向渠道发起查单并同步状态')
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '主动查单失败')
  }
}

// ===== 关闭 =====
function handleClose(row: PaymentOrderInfo) {
  const dialog = DialogPlugin.confirm({
    header: '关闭支付单',
    body: `确认关闭支付单 ${row.payment_no}？关闭后用户无法继续支付该单。`,
    confirmBtn: { content: '确认关闭', theme: 'danger' },
    onConfirm: async () => {
      try {
        await closePaymentOrder(row.id)
        MessagePlugin.success('支付单已关闭')
        dialog.hide()
        loadOrders()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '关闭支付单失败')
      }
    },
  })
}

function handleMobileAction(value: string | number | Record<string, any>, row: PaymentOrderInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'confirm':
      openConfirm(row)
      break
    case 'sync':
      handleSync(row)
      break
    case 'close':
      handleClose(row)
      break
  }
}

onMounted(loadOrders)
</script>

<style lang="css" scoped>
.stat-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.detail-block {
  margin-bottom: var(--space-lg);
}

.detail-block__title {
  margin: 0 0 var(--space-sm);
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.detail-pre {
  margin: 0;
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-size: 12px;
  font-family: var(--hs-font-mono);
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-link {
  font-size: 12px;
  color: var(--color-primary);
  word-break: break-all;
}

@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style lang="css">
@import '../shared.css';
</style>
