<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="20" aria-hidden="true" />
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

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="订单号 / 产品名称" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">用户账号</span>
        <t-input v-model="filters.user_keyword" placeholder="用户名 / 邮箱 / 手机号" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">支付单号</span>
        <t-input v-model="filters.payment_no" placeholder="支付中心支付单号" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">渠道流水号</span>
        <t-input v-model="filters.channel_tx" placeholder="第三方交易号" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">订单类型</span>
        <t-select v-model="filters.order_type" clearable placeholder="全部类型" :options="orderTypeOptions" />
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
        <span class="field__label">核验状态</span>
        <t-select v-model="filters.verify_state" clearable placeholder="全部" :options="verifyStateOptions" />
      </div>
      <div class="field">
        <span class="field__label">支付方式</span>
        <t-select v-model="filters.pay_method" clearable placeholder="全部支付方式" :options="payMethodOptions" />
      </div>
      <div class="field">
        <span class="field__label">下单时间</span>
        <t-date-range-picker v-model="filters.dateRange" clearable separator="~" placeholder="开始日期 ~ 结束日期" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <span class="table-card__meta">共 {{ total }} 个订单</span>
      </div>

      <!-- 批量操作条：勾选后出现（移动端不提供多选，见列定义） -->
      <div class="batch-bar" :class="{ 'batch-bar--active': selectedIds.length > 0 }">
        <span class="batch-bar__count">
          <template v-if="selectedIds.length">已选 {{ selectedIds.length }} 个订单</template>
          <template v-else>勾选订单后可批量取消 / 重新开通 / 核验 / 删除</template>
        </span>
        <t-space size="small">
          <t-button
            size="small"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="handleBatchCancel"
            >批量取消</t-button
          >
          <t-button
            size="small"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="handleBatchActivate"
            >批量重新开通</t-button
          >
          <t-button
            v-if="canVerify"
            size="small"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="openBatchVerifyDialog"
            >批量核验</t-button
          >
          <t-button
            v-if="canDelete"
            size="small"
            theme="danger"
            variant="outline"
            :disabled="!selectedIds.length || batchLoading"
            @click="openBatchDeleteDialog"
            >批量删除</t-button
          >
          <t-button v-if="selectedIds.length" size="small" variant="text" @click="clearSelection">清空选择</t-button>
        </t-space>
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
        :selected-row-keys="selectedIds"
        :pagination="isMobile ? undefined : pagination"
        @select-change="handleSelectChange"
        @page-change="handlePageChange"
      >
        <template #order_no="{ row }">
          <div class="price-cell">
            <t-link theme="primary" hover="color" @click="openDetail(row)">{{ row.order_no }}</t-link>
            <span class="price-sub">
              <t-tag v-if="row.renewal_id" theme="success" variant="light" size="small" shape="round">续费</t-tag>
              <template v-else>普通购买</template>
              <template v-if="row.payment_no"> · 支付单 {{ row.payment_no }}</template>
            </span>
          </div>
        </template>

        <template #product="{ row }">
          <div class="product-cell">
            <router-link
              v-if="row.product_id"
              class="cell-link cell-strong"
              :to="`/product/products/${row.product_id}`"
            >
              {{ row.product_name }}
            </router-link>
            <span v-else class="cell-strong">{{ row.product_name }}</span>
            <span v-if="row.specs" class="product-sub">{{ row.specs }}</span>
          </div>
        </template>

        <!-- 关联实例：订单 → 实例的运维跳转（下单即建占位实例，占位期标「待开通」）。 -->
        <template #instance="{ row }">
          <router-link v-if="row.instance_id" class="cell-link" :to="`/instances/detail/${row.instance_id}`">
            {{ row.instance_pending ? '待开通' : row.instance_no || `#${row.instance_id}` }}
          </router-link>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #amount="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.paid_amount) }}</span>
            <span class="price-sub">应付 ¥{{ formatPrice(row.total_amount) }}</span>
            <span v-if="row.discount_amount > 0" class="price-sub">优惠 ¥{{ formatPrice(row.discount_amount) }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <div class="status-cell">
            <t-tag :theme="orderStatusTheme(row.status)" variant="light" size="small" shape="round">
              {{ orderStatusLabel(row.status) }}
            </t-tag>
            <!-- 核验标记：开关开启时后端在读取期推导 verify_state=pending（服务中未核验）。 -->
            <t-tag
              v-if="row.verify_state"
              :theme="verifyStateTheme(row.verify_state)"
              variant="light-outline"
              size="small"
              shape="round"
            >
              {{ verifyStateLabel(row.verify_state) }}
            </t-tag>
            <!-- 履约异常：订单状态还停在"已支付"，开通其实已卡住 —— 必须显式提示，
                 否则运营只看订单状态会以为一切正常（实测事故：上游余额不足重试多次无人知）。 -->
            <t-tooltip
              v-if="row.provision_status === 'manual'"
              :content="`开通失败待人工处理：${row.provision_error || '未记录原因'}`"
            >
              <t-tag theme="danger" variant="light-outline" size="small" shape="round">开通待人工</t-tag>
            </t-tooltip>
            <t-tooltip
              v-else-if="row.provision_status === 'failed'"
              :content="`开通失败，系统自动重试中：${row.provision_error || '未记录原因'}`"
            >
              <t-tag theme="warning" variant="light-outline" size="small" shape="round">开通重试中</t-tag>
            </t-tooltip>
          </div>
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
              :options="
                buildMobileActionOptions([
                  { content: '详情', value: 'detail' },
                  { content: '取消', value: 'cancel', hidden: () => !isCancellable(row), theme: 'error' },
                  { content: '退款', value: 'refund', hidden: () => !isRefundable(row), theme: 'warning' },
                  { content: '核验', value: 'verify', hidden: () => !isVerifiable(row), theme: 'success' },
                  { content: '重新开通', value: 'activate', hidden: () => !isActivatable(row), theme: 'success' },
                  { content: '备注', value: 'remark' },
                ])
              "
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link v-if="isCancellable(row)" theme="danger" hover="color" @click="handleCancel(row)">取消</t-link>
              <t-link v-if="isRefundable(row)" theme="warning" hover="color" @click="openRefundDialog(row)"
                >退款</t-link
              >
              <t-link v-if="isVerifiable(row)" theme="success" hover="color" @click="handleVerify(row)">核验</t-link>
              <t-link v-if="isActivatable(row)" theme="primary" hover="color" @click="handleActivate(row)"
                >重新开通</t-link
              >
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
        <t-form-item label="退款去向" name="refund_mode">
          <!-- 刻意不用 variant="default-filled"：该变体的高亮块由 JS 逐次测量 px 定位，
               两个选项文字宽度不同 + 与提示同行被 flex 挤压，切换时块会滑动变形。 -->
          <div class="refund-mode">
            <t-radio-group v-model="refundForm.refund_mode">
              <t-radio-button value="balance">退回余额</t-radio-button>
              <t-radio-button value="channel">原路退回</t-radio-button>
            </t-radio-group>
            <p class="form-hint">
              {{
                refundForm.refund_mode === 'balance'
                  ? '退回余额：消费口径不变，仅余额增加。'
                  : '原路退回：财务按本金 + 渠道扣点扣减收入，需支付渠道支持退款接口。'
              }}
            </p>
          </div>
        </t-form-item>
        <t-form-item label="退款金额（元）" name="amount">
          <t-input-number
            v-model="refundForm.amount"
            :min="0"
            :max="refundForm.paid_amount"
            :precision="2"
            theme="column"
            placeholder="请输入退款金额"
          />
        </t-form-item>
        <t-form-item v-if="refundForm.refund_mode === 'channel'" label="渠道扣点（元）" name="fee_amount">
          <t-input-number
            v-model="refundForm.fee_amount"
            :min="0"
            :precision="2"
            theme="column"
            placeholder="渠道不退还的手续费，默认 0"
          />
        </t-form-item>
        <t-form-item label="退款原因" name="reason">
          <!-- 预设只是快捷输入：选中后填入下方文本域，仍可自由修改。 -->
          <div class="refund-reason">
            <t-select
              v-model="refundForm.reasonPreset"
              :options="refundReasonPresets"
              clearable
              placeholder="选择常用原因（自动填入下方，可手改）"
              @change="applyReasonPreset"
            />
            <t-textarea
              v-model="refundForm.reason"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="选填，请填写退款原因"
            />
          </div>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 批量核验：统一填写核验结论 -->
    <t-dialog
      v-model:visible="batchVerifyVisible"
      header="批量核验订单"
      width="480px"
      :confirm-btn="{ content: `核验 ${selectedIds.length} 个订单`, theme: 'primary', loading: batchLoading }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleBatchVerify"
      @close="batchVerifyVisible = false"
    >
      <t-form label-align="top" :data="batchVerifyForm" @submit.prevent>
        <t-form-item label="核验结论（选填，应用于本次全部订单）" name="remark">
          <t-textarea
            v-model="batchVerifyForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="如：配置、时长与订单一致"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 批量操作结果：逐条回显成功/跳过/失败与原因 -->
    <t-dialog v-model:visible="batchResult.visible" :header="`${batchResult.label}结果`" width="640px" :footer="false">
      <p class="batch-result__summary">
        成功 {{ batchResult.data?.succeeded ?? 0 }} 条 · 跳过 {{ batchResult.data?.skipped ?? 0 }} 条 · 失败
        {{ batchResult.data?.failed ?? 0 }} 条
      </p>
      <t-table
        :data="batchResult.data?.items ?? []"
        :columns="batchResultColumns"
        size="small"
        row-key="id"
        table-layout="fixed"
        max-height="360"
      >
        <template #status="{ row }">
          <t-tag :theme="batchStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ batchStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #message="{ row }">
          <span :class="{ 'batch-result__err': row.status === 'failed' }">{{ row.message || '—' }}</span>
        </template>
      </t-table>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchActivateOrders,
  batchCancelOrders,
  batchDeleteOrders,
  batchVerifyOrders,
  activateOrder,
  cancelOrder,
  createOrderRefund,
  getOrderList,
  updateOrderRemark,
  verifyOrder,
} from '@/api/order'
import {
  formatPrice,
  formatTime,
  orderStatusLabel,
  orderStatusOptions,
  orderStatusTheme,
  orderTypeOptions,
  payMethodLabel,
  payMethodOptions,
  refundReasonPresets,
  toDateString,
  verifyStateLabel,
  verifyStateOptions,
  verifyStateTheme,
} from '@/pages/order/constants'
import type { BatchActionResponse, OrderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'OrderList' })

const router = useRouter()
const { has } = usePermission()

const orderList = ref<OrderInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

// 批量操作（移动端不提供多选，与实例列表同口径）
const selectedIds = ref<Array<string | number>>([])
const batchLoading = ref(false)
const canVerify = computed(() => has('order:verify'))
const canDelete = computed(() => has('order:delete'))

const filters = reactive<{
  keyword: string | undefined
  user_keyword: string | undefined
  payment_no: string | undefined
  channel_tx: string | undefined
  order_type: string | undefined
  product_id: string | undefined
  status: string | undefined
  verify_state: string | undefined
  pay_method: string | undefined
  dateRange: (string | Date | undefined)[] | undefined
}>({
  keyword: undefined,
  user_keyword: undefined,
  payment_no: undefined,
  channel_tx: undefined,
  order_type: undefined,
  product_id: undefined,
  status: undefined,
  verify_state: undefined,
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
const columns = computed<PrimaryTableCol<OrderInfo>[]>(() => {
  const base: PrimaryTableCol<OrderInfo>[] = [
    { colKey: 'order_no', title: '订单号', minWidth: 180 },
    { colKey: 'user_id', title: '用户ID', width: 90, align: 'center' as const },
    { colKey: 'product', title: '产品', minWidth: 200 },
    { colKey: 'instance', title: '实例', minWidth: 130 },
    { colKey: 'amount', title: '金额', width: 150 },
    { colKey: 'status', title: '状态', width: 150 },
    { colKey: 'pay_method', title: '支付方式', width: 110 },
    { colKey: 'created_at', title: '下单时间', width: 170 },
    {
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : 250,
      fixed: 'right' as const,
      align: 'center' as const,
    },
  ]
  // 多选列只在桌面端出现（移动端多选框会挤压操作列，实例列表同款取舍）。
  if (!isMobile.value) {
    base.unshift({ colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const })
  }
  return base
})

function isCancellable(row: OrderInfo): boolean {
  return row.status === 'pending'
}

function isRefundable(row: OrderInfo): boolean {
  return ['active', 'refunding', 'paid', 'provisioning'].includes(row.status)
}

function isActivatable(row: OrderInfo): boolean {
  return ['paid', 'provisioning'].includes(row.status)
}

// 核验入口：已开通且未核验（开关关闭时也允许手工核验，只是列表不提示待核验）。
function isVerifiable(row: OrderInfo): boolean {
  if (row.verified_at) return false
  return ['active', 'refunding', 'completed'].includes(row.status)
}

function handleSelectChange(keys: Array<string | number>) {
  selectedIds.value = keys.map((k) => Number(k)).filter((id) => Number.isFinite(id) && id > 0)
}

function clearSelection() {
  selectedIds.value = []
}

function selectedOrderIds(): number[] {
  return selectedIds.value.map((k) => Number(k))
}

async function loadOrders() {
  loading.value = true
  try {
    const [startDate, endDate] = filters.dateRange ?? []
    const data = await getOrderList({
      keyword: filters.keyword,
      user_keyword: filters.user_keyword,
      payment_no: filters.payment_no,
      channel_tx: filters.channel_tx,
      order_type: filters.order_type,
      product_id: filters.product_id ? Number(filters.product_id) : undefined,
      status: filters.status,
      verify_state: filters.verify_state,
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
  filters.payment_no = undefined
  filters.channel_tx = undefined
  filters.order_type = undefined
  filters.product_id = undefined
  filters.status = undefined
  filters.verify_state = undefined
  filters.pay_method = undefined
  filters.dateRange = undefined
  pagination.current = 1
  loadOrders()
}

function handleMobileAction(value: string | number | Record<string, unknown>, row: OrderInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
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
    case 'verify':
      void handleVerify(row)
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
const refundForm = reactive<{
  orderId: number
  paid_amount: number
  amount: number
  refund_mode: string
  fee_amount: number
  reason: string
  reasonPreset: string
}>({
  orderId: 0,
  paid_amount: 0,
  amount: 0,
  refund_mode: 'balance',
  fee_amount: 0,
  reason: '',
  reasonPreset: '',
})

// 选中预设即填入原因文本域，仍可继续手改（预设只是快捷输入，不锁定文本）。
function applyReasonPreset(value: unknown): void {
  const text = typeof value === 'string' ? value : ''
  if (text) refundForm.reason = text
}

function openRefundDialog(row: OrderInfo) {
  refundForm.orderId = row.id
  refundForm.paid_amount = row.paid_amount
  refundForm.amount = row.paid_amount
  refundForm.refund_mode = 'balance'
  refundForm.fee_amount = 0
  refundForm.reason = ''
  refundForm.reasonPreset = ''
  refundVisible.value = true
}

async function handleCreateRefund() {
  if (refundForm.amount <= 0) {
    MessagePlugin.warning('退款金额需大于 0')
    return
  }
  if (refundForm.refund_mode === 'channel' && refundForm.fee_amount < 0) {
    MessagePlugin.warning('渠道扣点不能为负数')
    return
  }
  try {
    await createOrderRefund(refundForm.orderId, {
      amount: refundForm.amount,
      reason: refundForm.reason || undefined,
      refund_mode: refundForm.refund_mode,
      fee_amount: refundForm.refund_mode === 'channel' ? refundForm.fee_amount : 0,
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

async function handleVerify(row: OrderInfo) {
  const dialog = DialogPlugin.confirm({
    header: '核验订单',
    body: `确认已完成对订单「${row.order_no}」的开通结果核对？核验后不可撤销。`,
    confirmBtn: { content: '确认核验', theme: 'primary' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await verifyOrder(row.id)
        MessagePlugin.success('订单已核验')
        dialog.destroy()
        loadOrders()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '核验失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

// —— 批量操作 ——

const batchVerifyVisible = ref(false)
const batchVerifyForm = reactive<{ remark: string }>({ remark: '' })
const batchResult = reactive<{
  visible: boolean
  label: string
  data: BatchActionResponse | null
}>({ visible: false, label: '', data: null })

const batchResultColumns: PrimaryTableCol<{ id: number; status: string; message?: string }>[] = [
  { colKey: 'id', title: 'ID', width: 90 },
  { colKey: 'status', title: '结果', width: 100 },
  { colKey: 'message', title: '说明', minWidth: 220 },
]

function batchStatusLabel(status: string): string {
  const map: Record<string, string> = { success: '成功', skipped: '跳过', failed: '失败' }
  return map[status] || status
}

function batchStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'skipped':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'default'
  }
}

function showBatchResult(label: string, resp: BatchActionResponse): void {
  batchResult.label = label
  batchResult.data = resp
  batchResult.visible = true
}

async function runBatch(label: string, action: () => Promise<BatchActionResponse>): Promise<void> {
  batchLoading.value = true
  try {
    const resp = await action()
    showBatchResult(label, resp)
    clearSelection()
    loadOrders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${label}失败`)
  } finally {
    batchLoading.value = false
  }
}

function handleBatchCancel() {
  const count = selectedIds.value.length
  if (!count) return
  const dialog = DialogPlugin.confirm({
    header: `批量取消 ${count} 个订单？`,
    body: '仅「待支付」订单会被取消，其余将跳过并说明原因。取消后会释放占位实例并回补库存，不可撤销。',
    confirmBtn: { content: '批量取消', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      dialog.destroy()
      await runBatch('批量取消', () => batchCancelOrders(selectedOrderIds()))
    },
    onClose: () => dialog.destroy(),
  })
}

function handleBatchActivate() {
  const count = selectedIds.value.length
  if (!count) return
  const dialog = DialogPlugin.confirm({
    header: `批量重新开通 ${count} 个订单？`,
    body: '仅「已支付 / 开通中」订单会重新投递开通任务；重试会复用已生成的上游账单，不会重复下单。',
    confirmBtn: { content: '重新开通', theme: 'primary' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      dialog.destroy()
      await runBatch('批量重新开通', () => batchActivateOrders(selectedOrderIds()))
    },
    onClose: () => dialog.destroy(),
  })
}

function openBatchVerifyDialog() {
  if (!selectedIds.value.length) return
  batchVerifyForm.remark = ''
  batchVerifyVisible.value = true
}

async function handleBatchVerify() {
  batchVerifyVisible.value = false
  await runBatch('批量核验', () => batchVerifyOrders({ ids: selectedOrderIds(), remark: batchVerifyForm.remark }))
}

function openBatchDeleteDialog() {
  const count = selectedIds.value.length
  if (!count) return
  const dialog = DialogPlugin.confirm({
    header: `批量删除 ${count} 个订单？`,
    body: '仅「已取消 / 已关闭」且无退款记录的订单会被删除；其余将跳过并说明原因。订单为财务凭证，删除是从列表隐藏（软删除），数据仍保留可审计。',
    confirmBtn: { content: '批量删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      dialog.destroy()
      await runBatch('批量删除', () => batchDeleteOrders(selectedOrderIds()))
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(loadOrders)
</script>

<style lang="css">
@import './shared.css';
</style>

<style scoped>
/* 批量操作条：常态灰色提示，勾选后高亮（与产品列表同款结构）。 */
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  margin-bottom: var(--space-md);
  padding: 10px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  transition:
    border-color var(--hs-duration-fast),
    background-color var(--hs-duration-fast);
}

.batch-bar--active {
  border-color: var(--td-brand-color-3);
  background: var(--td-brand-color-1);
}

.batch-bar__count {
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.batch-bar--active .batch-bar__count {
  color: #3f5848;
  font-weight: 600;
}

.batch-result__summary {
  margin: 0 0 var(--space-md);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.batch-result__err {
  color: var(--td-error-color-6);
}
</style>
