<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ order?.order_no || '订单详情' }}</h2>
          <p class="page-header__desc">{{ order?.product_name || '—' }} · 用户 ID {{ order?.user_id ?? '—' }}</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadDetail">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <t-button v-if="isCancellable(order)" variant="danger" @click="handleCancel">取消订单</t-button>
        <t-button v-if="isRefundable(order)" variant="warning" @click="openRefundDialog">退款</t-button>
        <t-button v-if="isActivatable(order)" variant="success" @click="handleActivate">重新开通</t-button>
        <t-button theme="primary" @click="openRemarkDialog">备注</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="base" label="基本信息">
          <div class="tabs-section">
            <t-descriptions v-if="order" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="订单号">{{ order.order_no }}</t-descriptions-item>
              <t-descriptions-item label="用户 ID">{{ order.user_id }}</t-descriptions-item>
              <t-descriptions-item label="产品名称">
                <span class="cell-strong">{{ order.product_name || '—' }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="规格">
                <pre class="spec-pre">{{ order.specs || '—' }}</pre>
              </t-descriptions-item>
              <t-descriptions-item label="数量">{{ order.quantity }}</t-descriptions-item>
              <t-descriptions-item label="价格模型">{{ order.price_model || '—' }}</t-descriptions-item>
              <t-descriptions-item label="应付总额">¥{{ formatPrice(order.total_amount) }}</t-descriptions-item>
              <t-descriptions-item label="实付金额">
                <span class="price-main">¥{{ formatPrice(order.paid_amount) }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="状态">
                <t-tag :theme="orderStatusTheme(order.status)" variant="light" size="small" shape="round">
                  {{ orderStatusLabel(order.status) }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="支付方式">{{ payMethodLabel(order.pay_method) }}</t-descriptions-item>
              <t-descriptions-item label="支付时间">{{ formatTime(order.pay_time) }}</t-descriptions-item>
              <t-descriptions-item label="到期时间">{{ formatTime(order.expire_time) }}</t-descriptions-item>
              <t-descriptions-item label="备注">{{ order.remark || '—' }}</t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(order.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(order.updated_at) }}</t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="items" label="订单明细">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">订单明细</h3>
              <span class="table-card__meta">共 {{ order?.items?.length ?? 0 }} 项</span>
            </div>
            <t-table
              row-key="id"
              :data="order?.items ?? []"
              :columns="itemColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #amount="{ row }">
                <span>¥{{ formatPrice(row.amount) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无明细项" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="refunds" label="退款记录">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">退款记录</h3>
              <span class="table-card__meta">共 {{ order?.refunds?.length ?? 0 }} 条</span>
            </div>
            <t-table
              row-key="id"
              :data="order?.refunds ?? []"
              :columns="refundColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #refund_no="{ row }">
                <t-link theme="primary" hover="color" @click="openRefundDetail(row)">{{ row.refund_no }}</t-link>
              </template>
              <template #amount="{ row }">
                <span>¥{{ formatPrice(row.amount) }}</span>
              </template>
              <template #status="{ row }">
                <t-tag :theme="refundStatusTheme(row.status)" variant="light" size="small" shape="round">
                  {{ refundStatusLabel(row.status) }}
                </t-tag>
              </template>
              <template #empty>
                <t-empty description="暂无退款记录" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>
      </t-tabs>
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
import { useRoute, useRouter } from 'vue-router'

import { AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { activateOrder, cancelOrder, createOrderRefund, getOrderDetail, updateOrderRemark } from '@/api/order'
import {
  formatPrice,
  formatTime,
  orderStatusLabel,
  orderStatusTheme,
  payMethodLabel,
  refundStatusLabel,
  refundStatusTheme,
} from '@/pages/order/constants'
import type { OrderDetail, OrderItemInfo, OrderInfo, RefundInfo } from '@/types/interface'

defineOptions({ name: 'OrderDetail' })

const route = useRoute()
const router = useRouter()

const order = ref<OrderDetail | null>(null)
const loading = ref(false)
const activeTab = ref('base')

const itemColumns: PrimaryTableCol<OrderItemInfo>[] = [
  { colKey: 'product_name', title: '产品名称', minWidth: 180 },
  { colKey: 'spec_code', title: '规格编码', width: 130 },
  { colKey: 'specs', title: '规格', minWidth: 140 },
  { colKey: 'price', title: '单价', width: 100 },
  { colKey: 'quantity', title: '数量', width: 80, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 110 },
]

const refundColumns: PrimaryTableCol<RefundInfo>[] = [
  { colKey: 'refund_no', title: '退款单号', minWidth: 160 },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'reason', title: '原因', minWidth: 140 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'audited_at', title: '审核时间', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

function isCancellable(row: OrderInfo | null): boolean {
  return row?.status === 'pending'
}

function isRefundable(row: OrderInfo | null): boolean {
  return ['active', 'refunding', 'paid', 'provisioning'].includes(row?.status ?? '')
}

function isActivatable(row: OrderInfo | null): boolean {
  return ['paid', 'provisioning'].includes(row?.status ?? '')
}

async function loadDetail() {
  const id = Number(route.params.id)
  loading.value = true
  try {
    order.value = await getOrderDetail(id)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载订单详情失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/orders/list')
}

function openRefundDetail(row: RefundInfo) {
  router.push(`/orders/refunds/${row.id}`)
}

function handleCancel() {
  const target = order.value
  if (!target) return
  const dialog = DialogPlugin.confirm({
    header: '取消订单',
    body: `确认取消订单「${target.order_no}」吗？该操作不可撤销。`,
    confirmBtn: { content: '确认取消', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      try {
        await cancelOrder(target.id)
        MessagePlugin.success('订单已取消')
        dialog.destroy()
        loadDetail()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '取消失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

const remarkVisible = ref(false)
const remarkForm = reactive<{ remark: string }>({ remark: '' })

function openRemarkDialog() {
  remarkForm.remark = order.value?.remark ?? ''
  remarkVisible.value = true
}

async function handleSaveRemark() {
  if (!order.value) return
  try {
    await updateOrderRemark(order.value.id, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

const refundVisible = ref(false)
const refundForm = reactive<{ paid_amount: number; amount: number; reason: string }>({
  paid_amount: 0,
  amount: 0,
  reason: '',
})

function openRefundDialog() {
  const target = order.value
  if (!target) return
  refundForm.paid_amount = target.paid_amount
  refundForm.amount = target.paid_amount
  refundForm.reason = ''
  refundVisible.value = true
}

async function handleCreateRefund() {
  if (!order.value) return
  try {
    await createOrderRefund(order.value.id, {
      amount: refundForm.amount,
      reason: refundForm.reason || undefined,
    })
    MessagePlugin.success('退款单已提交')
    refundVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '发起退款失败')
  }
}

async function handleActivate() {
  if (!order.value) return
  try {
    await activateOrder(order.value.id)
    MessagePlugin.success('已重新触发开通')
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重新开通失败')
  }
}

onMounted(loadDetail)
</script>

<style lang="css">
@import './shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}

.spec-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
  color: #334155;
}
</style>
