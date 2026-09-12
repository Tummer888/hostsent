<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">订单列表</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadOrders">
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
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="订单号 / 产品名称" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">用户账号</span>
          <t-input v-model="filters.user_keyword" placeholder="用户名 / 邮箱 / 手机号" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">产品 ID</span>
          <t-input v-model="filters.product_id" placeholder="按产品 ID 筛选" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="orderStatusOptions" />
        </div>
        <div class="field">
          <span class="field__label">支付方式</span>
          <t-select v-model="filters.pay_method" clearable placeholder="全部支付方式" :options="payMethodOptions" />
        </div>
        <div class="field">
          <span class="field__label">下单时间</span>
          <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
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
        <h3 class="card-title">订单列表</h3>
        <span class="table-card__meta">共 {{ total }} 个订单</span>
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
        <template #order_no="{ row }">
          <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.order_no }}</t-link>
        </template>

        <template #product="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.product_name }}</span>
            <span v-if="row.specs" class="product-sub">{{ row.specs }}</span>
          </div>
        </template>

        <template #amount="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.paid_amount) }}</span>
            <span class="price-sub">应付 ¥{{ formatPrice(row.total_amount) }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="orderStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ orderStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #pay_method="{ row }">
          <span>{{ payMethodLabel(row.pay_method) }}</span>
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
                { content: '取消', value: 'cancel', hidden: () => !isCancellable(row), theme: 'error' },
                { content: '退款', value: 'refund', hidden: () => !isRefundable(row), theme: 'warning' },
                { content: '重新开通', value: 'activate', hidden: () => !isActivatable(row), theme: 'success' },
                { content: '备注', value: 'remark' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="isCancellable(row)" theme="danger" hover="color" @click="handleCancel(row)">取消</t-link>
              <t-link v-if="isRefundable(row)" theme="warning" hover="color" @click="openRefundDialog(row)">退款</t-link>
              <t-link v-if="isActivatable(row)" theme="primary" hover="color" @click="handleActivate(row)">重新开通</t-link>
              <t-link theme="primary" hover="color" @click="openRemarkDialog(row)">备注</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无订单数据" />
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
      v-model:visible="remarkVisible"
      header="订单备注"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveRemark"
      @close="remarkVisible = false"
    >
      <t-form label-align="top" :data="remarkForm" @submit.prevent>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="remarkForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入订单备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="refundVisible"
      header="发起退款"
      width="480px"
      :confirm-btn="{ content: '提交', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCreateRefund"
      @close="refundVisible = false"
    >
      <t-form label-align="top" :data="refundForm" @submit.prevent>
        <t-form-item label="可退金额" name="paid_amount">
          <t-alert theme="warning" :message="`本订单实付 ¥${formatPrice(refundForm.paid_amount)}`" />
        </t-form-item>
        <t-form-item label="退款金额（元）" name="amount">
          <t-input-number v-model="refundForm.amount" :min="0" :max="refundForm.paid_amount" :precision="2" theme="column" placeholder="请输入退款金额" />
        </t-form-item>
        <t-form-item label="退款原因" name="reason">
          <t-textarea v-model="refundForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写退款原因" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { activateOrder, cancelOrder, createOrderRefund, getOrderList, updateOrderRemark } from '@/api/order'
import {
  formatPrice,
  formatTime,
  orderStatusLabel,
  orderStatusOptions,
  orderStatusTheme,
  payMethodLabel,
  payMethodOptions,
  toDateString,
} from '@/pages/order/constants'
import type { OrderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'OrderList' })

const router = useRouter()

const orderList = ref<OrderInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{
  keyword: string | undefined
  user_keyword: string | undefined
  product_id: string | undefined
  status: string | undefined
  pay_method: string | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  keyword: undefined,
  user_keyword: undefined,
  product_id: undefined,
  status: undefined,
  pay_method: undefined,
  dateRange: undefined,
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
const columns: PrimaryTableCol<OrderInfo>[] = [
  { colKey: 'order_no', title: '订单号', minWidth: 180 },
  { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
  { colKey: 'product', title: '产品', minWidth: 200 },
  { colKey: 'amount', title: '金额', width: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'pay_method', title: '支付方式', width: 110 },
  { colKey: 'created_at', title: '下单时间', width: 170 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 230,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function isCancellable(row: OrderInfo): boolean {
  return row.status === 'pending'
}

function isRefundable(row: OrderInfo): boolean {
  return ['active', 'refunding', 'paid', 'provisioning'].includes(row.status)
}

function isActivatable(row: OrderInfo): boolean {
  return ['paid', 'provisioning'].includes(row.status)
}

async function loadOrders() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getOrderList({
      keyword: filters.keyword,
      user_keyword: filters.user_keyword,
      product_id: filters.product_id ? Number(filters.product_id) : undefined,
      status: filters.status,
      pay_method: filters.pay_method,
      start_time: toDateString(startDate) ? `${toDateString(startDate)} 00:00:00` : undefined,
      end_time: toDateString(endDate) ? `${toDateString(endDate)} 23:59:59` : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    orderList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载订单列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadOrders()
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
  loadOrders()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.user_keyword = undefined
  filters.product_id = undefined
  filters.status = undefined
  filters.pay_method = undefined
  filters.dateRange = undefined
  pagination.current = 1
  loadOrders()
}

function handleMobileAction(value: string | number | Record<string, any>, row: OrderInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'cancel':
      void handleCancel(row)
      break
    case 'refund':
      openRefundDialog(row)
      break
    case 'activate':
      void handleActivate(row)
      break
    case 'remark':
      openRemarkDialog(row)
      break
  }
}

function openDetail(row: OrderInfo) {
  router.push(`/orders/detail/${row.id}`)
}

function handleCancel(row: OrderInfo) {
  const dialog = DialogPlugin.confirm({
    header: '取消订单',
    body: `确认取消订单「${row.order_no}」吗？该操作不可撤销。`,
    confirmBtn: { content: '确认取消', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        await cancelOrder(row.id)
        MessagePlugin.success('订单已取消')
        dialog.destroy()
        loadOrders()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '取消失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

const remarkVisible = ref(false)
const remarkForm = reactive<{ id: number; remark: string }>({ id: 0, remark: '' })

function openRemarkDialog(row: OrderInfo) {
  remarkForm.id = row.id
  remarkForm.remark = row.remark
  remarkVisible.value = true
}

async function handleSaveRemark() {
  try {
    await updateOrderRemark(remarkForm.id, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

const refundVisible = ref(false)
const refundForm = reactive<{ orderId: number; paid_amount: number; amount: number; reason: string }>({
  orderId: 0,
  paid_amount: 0,
  amount: 0,
  reason: '',
})

function openRefundDialog(row: OrderInfo) {
  refundForm.orderId = row.id
  refundForm.paid_amount = row.paid_amount
  refundForm.amount = row.paid_amount
  refundForm.reason = ''
  refundVisible.value = true
}

async function handleCreateRefund() {
  try {
    await createOrderRefund(refundForm.orderId, {
      amount: refundForm.amount,
      reason: refundForm.reason || undefined,
    })
    MessagePlugin.success('退款单已提交')
    refundVisible.value = false
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '发起退款失败')
  }
}

async function handleActivate(row: OrderInfo) {
  try {
    await activateOrder(row.id)
    MessagePlugin.success('已重新触发开通')
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重新开通失败')
  }
}

onMounted(loadOrders)
</script>

<style lang="css">
@import './shared.css';
</style>
