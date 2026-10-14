<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ order?.order_no || '订单详情' }}</h2>
          <p class="page-header__desc">
            <router-link v-if="order?.product_id" class="header-link" :to="`/product/products/${order.product_id}`">
              {{ order?.product_name || '—' }}
            </router-link>
            <span v-else>{{ order?.product_name || '—' }}</span>
            · 用户 ID {{ order?.user_id ?? '—' }}
          </p>
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
        <!-- variant 只接受 base/outline/dashed/text，颜色走 theme；
             此前误把 danger/warning/success 写进 variant，按钮退化成透明文字块。 -->
        <t-button v-if="isCancellable(order)" variant="outline" theme="danger" @click="handleCancel">
          <template #icon>
            <CloseCircleIcon aria-hidden="true" />
          </template>
          取消订单
        </t-button>
        <t-button v-if="isRefundable(order)" variant="outline" theme="warning" @click="openRefundDialog">
          <template #icon>
            <MoneyIcon aria-hidden="true" />
          </template>
          退款
        </t-button>
        <t-button v-if="isVerifiable(order)" variant="outline" theme="primary" @click="openVerifyDialog">
          <template #icon>
            <CheckCircleIcon aria-hidden="true" />
          </template>
          核验
        </t-button>
        <t-button v-if="isActivatable(order)" variant="outline" theme="primary" @click="handleActivate">
          <template #icon>
            <RotateIcon aria-hidden="true" />
          </template>
          重新开通
        </t-button>
        <t-button theme="primary" @click="openRemarkDialog">
          <template #icon>
            <EditIcon aria-hidden="true" />
          </template>
          备注
        </t-button>
      </t-space>
    </header>

    <t-alert
      v-if="provisionAlert"
      class="provision-alert"
      :theme="provisionAlert.theme"
      :title="provisionAlert.title"
      :message="provisionAlert.message"
    />

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="base" label="基本信息">
          <div class="tabs-section">
            <template v-if="order">
              <!-- 关键信息条：一眼锁定「什么状态、花了多少、什么时候到期」 -->
              <div class="key-facts">
                <div class="key-fact">
                  <span class="key-fact__label">订单状态</span>
                  <span class="key-fact__value key-fact__value--status">
                    <t-tag :theme="orderStatusTheme(order.status)" variant="light" size="medium" shape="round">
                      {{ orderStatusLabel(order.status) }}
                    </t-tag>
                    <t-tag
                      v-if="order.verify_state"
                      :theme="verifyStateTheme(order.verify_state)"
                      variant="light-outline"
                      size="small"
                      shape="round"
                    >
                      {{ verifyStateLabel(order.verify_state) }}
                    </t-tag>
                  </span>
                </div>
                <div class="key-fact key-fact--accent">
                  <span class="key-fact__label">实付金额</span>
                  <span class="key-fact__value key-fact__value--money">¥{{ formatPrice(order.paid_amount) }}</span>
                  <span v-if="order.discount_amount > 0" class="key-fact__sub">
                    优惠 ¥{{ formatPrice(order.discount_amount)
                    }}{{ order.discount_source ? `（${discountSourceLabel(order.discount_source)}）` : '' }}
                  </span>
                </div>
                <div class="key-fact">
                  <span class="key-fact__label">计费周期</span>
                  <span class="key-fact__value">{{ order.cycle || '—' }}</span>
                  <span class="key-fact__sub">{{ order.renewal_id ? '产品续费' : '普通购买' }}</span>
                </div>
                <div class="key-fact">
                  <span class="key-fact__label">到期时间</span>
                  <span class="key-fact__value">{{ formatTime(order.expire_time) }}</span>
                  <span class="key-fact__sub">{{ payMethodLabel(order.pay_method) }}</span>
                </div>
              </div>

              <!-- 关联入口：从订单直达商品与实例，运维排查少跳两次 -->
              <div class="link-row">
                <router-link v-if="order.product_id" class="link-chip" :to="`/product/products/${order.product_id}`">
                  <FileIcon aria-hidden="true" />
                  <span>查看商品</span>
                </router-link>
                <router-link v-if="order.instance_id" class="link-chip" :to="`/instances/detail/${order.instance_id}`">
                  <ServerIcon aria-hidden="true" />
                  <span>关联实例{{ order.instance_no ? `（${order.instance_no}）` : '' }}</span>
                  <t-tag
                    v-if="order.instance_pending"
                    theme="warning"
                    variant="light-outline"
                    size="small"
                    shape="round"
                    >待开通</t-tag
                  >
                </router-link>
                <span v-else class="link-chip link-chip--muted">
                  <ServerIcon aria-hidden="true" />
                  <span>暂无关联实例</span>
                </span>
              </div>

              <div class="desc-block">
                <h3 class="card-title">交易信息</h3>
                <t-descriptions :column="descColumn" size="medium" class="detail-desc">
                  <t-descriptions-item label="订单号">{{ order.order_no }}</t-descriptions-item>
                  <t-descriptions-item label="订单类型">
                    <t-tag :theme="order.renewal_id ? 'success' : 'default'" variant="light" size="small" shape="round">
                      {{ order.renewal_id ? '产品续费' : '普通购买' }}
                    </t-tag>
                  </t-descriptions-item>
                  <t-descriptions-item label="用户 ID">{{ order.user_id }}</t-descriptions-item>
                  <t-descriptions-item label="产品名称">
                    <router-link
                      v-if="order.product_id"
                      class="cell-link cell-strong"
                      :to="`/product/products/${order.product_id}`"
                    >
                      {{ order.product_name || '—' }}
                    </router-link>
                    <span v-else>{{ order.product_name || '—' }}</span>
                  </t-descriptions-item>
                  <t-descriptions-item label="规格">
                    <pre class="spec-pre">{{ order.specs || '—' }}</pre>
                  </t-descriptions-item>
                  <t-descriptions-item label="数量">{{ order.quantity }}</t-descriptions-item>
                </t-descriptions>
              </div>

              <div class="desc-block">
                <h3 class="card-title">金额与支付</h3>
                <t-descriptions :column="descColumn" size="medium" class="detail-desc">
                  <t-descriptions-item label="应付总额">¥{{ formatPrice(order.total_amount) }}</t-descriptions-item>
                  <t-descriptions-item label="实付金额">
                    <span class="price-main">¥{{ formatPrice(order.paid_amount) }}</span>
                  </t-descriptions-item>
                  <t-descriptions-item label="算价快照">
                    <div class="price-cell">
                      <span class="price-sub">原价 ¥{{ formatPrice(order.original_amount) }}</span>
                      <span class="price-sub"
                        >优惠 ¥{{ formatPrice(order.discount_amount)
                        }}{{ order.discount_source ? `（${discountSourceLabel(order.discount_source)}）` : '' }}</span
                      >
                      <span class="price-sub">实付 ¥{{ formatPrice(order.final_amount) }}</span>
                    </div>
                  </t-descriptions-item>
                  <t-descriptions-item label="支付方式">{{ payMethodLabel(order.pay_method) }}</t-descriptions-item>
                  <t-descriptions-item label="支付单号">{{ order.payment_no || '—' }}</t-descriptions-item>
                  <t-descriptions-item label="渠道流水号">{{ order.channel_tx || '—' }}</t-descriptions-item>
                </t-descriptions>
              </div>

              <div class="desc-block">
                <h3 class="card-title">履约与时间</h3>
                <t-descriptions :column="descColumn" size="medium" class="detail-desc">
                  <t-descriptions-item label="开通状态">
                    <div class="status-cell">
                      <t-tag
                        v-if="order.provision_status"
                        :theme="provisionStatusTheme(order.provision_status)"
                        variant="light"
                        size="small"
                        shape="round"
                      >
                        {{ provisionStatusLabel(order.provision_status) }}
                      </t-tag>
                      <span v-else class="cell-muted">—</span>
                      <span v-if="order.provision_attempts" class="cell-muted"
                        >已尝试 {{ order.provision_attempts }} 次</span
                      >
                    </div>
                  </t-descriptions-item>
                  <t-descriptions-item label="关联实例">
                    <router-link
                      v-if="order.instance_id"
                      class="cell-link"
                      :to="`/instances/detail/${order.instance_id}`"
                    >
                      {{ order.instance_no || `#${order.instance_id}` }}{{ order.instance_pending ? '（待开通）' : '' }}
                    </router-link>
                    <span v-else class="cell-muted">—</span>
                  </t-descriptions-item>
                  <t-descriptions-item label="核验状态">
                    <div class="status-cell">
                      <t-tag
                        v-if="order.verify_state"
                        :theme="verifyStateTheme(order.verify_state)"
                        variant="light"
                        size="small"
                        shape="round"
                      >
                        {{ verifyStateLabel(order.verify_state) }}
                      </t-tag>
                      <span v-else class="cell-muted">{{ order.verify_required ? '—' : '未开启核验' }}</span>
                      <span v-if="order.verified_by_name" class="cell-muted">
                        {{ order.verified_by_name }} · {{ formatTime(order.verified_at || '') }}
                      </span>
                    </div>
                  </t-descriptions-item>
                  <t-descriptions-item label="核验备注">{{ order.verify_remark || '—' }}</t-descriptions-item>
                  <t-descriptions-item label="支付时间">{{ formatTime(order.pay_time) }}</t-descriptions-item>
                  <t-descriptions-item label="到期时间">{{ formatTime(order.expire_time) }}</t-descriptions-item>
                  <t-descriptions-item label="创建时间">{{ formatTime(order.created_at) }}</t-descriptions-item>
                  <t-descriptions-item label="更新时间">{{ formatTime(order.updated_at) }}</t-descriptions-item>
                  <t-descriptions-item label="备注" :span="2">{{ order.remark || '—' }}</t-descriptions-item>
                </t-descriptions>
              </div>
            </template>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="items" label="订单明细">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">订单明细</h3>
              <span class="table-card__meta">共 {{ order?.items?.length ?? 0 }} 项</span>
            </div>
            <!-- 明细改为卡片式：左侧产品与规格，右侧金额大字，按视觉重心分层，
                 替代原来六列平铺的表格（列宽挤压后关键信息互相淹没）。 -->
            <div v-if="order?.items?.length" class="item-list">
              <article v-for="item in order.items" :key="item.id" class="item-card">
                <span class="item-card__chip">
                  <FileIcon aria-hidden="true" />
                </span>
                <div class="item-card__body">
                  <div class="item-card__title">
                    <router-link
                      v-if="item.product_id"
                      class="cell-link cell-strong"
                      :to="`/product/products/${item.product_id}`"
                    >
                      {{ item.product_name }}
                    </router-link>
                    <span v-else class="cell-strong">{{ item.product_name }}</span>
                    <t-tag v-if="item.spec_code" variant="light-outline" size="small" shape="round">{{
                      item.spec_code
                    }}</t-tag>
                    <t-tag v-if="item.cycle" variant="light-outline" size="small" shape="round">{{ item.cycle }}</t-tag>
                    <span class="item-card__qty">×{{ item.quantity }}</span>
                  </div>
                  <p v-if="item.specs" class="item-card__specs">{{ item.specs }}</p>
                </div>
                <div class="item-card__amount">
                  <span class="item-card__total">¥{{ formatPrice(item.amount) }}</span>
                  <span class="item-card__unit">单价 ¥{{ formatPrice(item.price) }} × {{ item.quantity }}</span>
                </div>
              </article>
            </div>
            <t-empty v-else description="暂无明细项" />
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
                <div class="price-cell">
                  <span>¥{{ formatPrice(row.amount) }}</span>
                  <span v-if="row.refund_mode === 'channel' && row.fee_amount > 0" class="price-sub">
                    扣点 ¥{{ formatPrice(row.fee_amount) }}
                  </span>
                </div>
              </template>
              <template #refund_mode="{ row }">
                <t-tag :theme="refundModeTheme(row.refund_mode)" variant="light" size="small" shape="round">
                  {{ refundModeLabel(row.refund_mode) }}
                </t-tag>
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
      v-model:visible="verifyVisible"
      header="核验订单"
      width="480px"
      :confirm-btn="{ content: '确认核验', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleVerify"
      @close="verifyVisible = false"
    >
      <t-form label-align="top" :data="verifyForm" @submit.prevent>
        <t-form-item label="核验结论（选填）" name="remark">
          <t-textarea
            v-model="verifyForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="如：配置、时长与订单一致"
          />
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
               两个选项文字宽度不同 + 与下方提示同行被 flex 挤压，切换时块会滑动变形。
               默认 outline 变体没有测量块，形状天然稳定。 -->
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  AppIcon,
  CheckCircleIcon,
  CloseCircleIcon,
  EditIcon,
  MoneyIcon,
  FileIcon,
  RefreshIcon,
  RotateIcon,
  ServerIcon,
} from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  activateOrder,
  cancelOrder,
  createOrderRefund,
  getOrderDetail,
  updateOrderRemark,
  verifyOrder,
} from '@/api/order'
import {
  discountSourceLabel,
  formatPrice,
  formatTime,
  orderStatusLabel,
  orderStatusTheme,
  payMethodLabel,
  refundModeLabel,
  refundModeTheme,
  refundReasonPresets,
  refundStatusLabel,
  refundStatusTheme,
  verifyStateLabel,
  verifyStateTheme,
} from '@/pages/order/constants'
import type { OrderDetail, OrderInfo, RefundInfo } from '@/types/interface'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'OrderDetail' })

const route = useRoute()
const router = useRouter()
const { isMobile } = useIsMobile()

const order = ref<OrderDetail | null>(null)
const loading = ref(false)
const activeTab = ref('base')
const descColumn = computed(() => (isMobile.value ? 1 : 2))

// 开通履约异常提示：manual=需人工处理（如上游余额不足），failed=仍在自动重试。
// 订单状态本身停留在 paid/provisioning，没有这块提示，运营看不出"为什么一直没开通"。
const provisionAlert = computed(() => {
  const status = order.value?.provision_status
  if (status !== 'manual' && status !== 'failed') return null
  const reason = order.value?.provision_error || '未记录失败原因'
  if (status === 'manual') {
    return {
      theme: 'error' as const,
      title: '开通失败，需人工处理',
      message: `上游已明确拒绝（重试无效）：${reason}。处理后可点右上角「重新开通」重试；重试会复用已生成的上游账单，不会重复下单。`,
    }
  }
  return {
    theme: 'warning' as const,
    title: '开通失败，系统将自动重试',
    message: `最近一次失败原因：${reason}`,
  }
})

const refundColumns: PrimaryTableCol<RefundInfo>[] = [
  { colKey: 'refund_no', title: '退款单号', minWidth: 160 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'refund_mode', title: '退款去向', width: 110 },
  { colKey: 'reason', title: '原因', minWidth: 140 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'audit_by_name', title: '审核人', width: 100 },
  { colKey: 'audited_at', title: '审核时间', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

// 履约状态中文映射（详情页展示用；manual/failed 另有顶部告警条）。
function provisionStatusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: '待开通',
    running: '开通中',
    success: '开通成功',
    failed: '开通重试中',
    manual: '开通待人工',
  }
  return map[status] || status
}

function provisionStatusTheme(status: string): string {
  switch (status) {
    case 'success':
      return 'success'
    case 'manual':
      return 'danger'
    case 'failed':
      return 'warning'
    case 'running':
      return 'primary'
    default:
      return 'default'
  }
}

function isCancellable(row: OrderInfo | null): boolean {
  return row?.status === 'pending'
}

function isRefundable(row: OrderInfo | null): boolean {
  return ['active', 'refunding', 'paid', 'provisioning'].includes(row?.status ?? '')
}

function isActivatable(row: OrderInfo | null): boolean {
  return ['paid', 'provisioning'].includes(row?.status ?? '')
}

// 核验入口：已开通且未核验（开关关闭时也允许手工核验，只是列表不提示待核验）。
function isVerifiable(row: OrderInfo | null): boolean {
  if (!row || row.verified_at) return false
  return ['active', 'refunding', 'completed'].includes(row.status)
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

const verifyVisible = ref(false)
const verifyForm = reactive<{ remark: string }>({ remark: '' })

function openVerifyDialog() {
  verifyForm.remark = ''
  verifyVisible.value = true
}

async function handleVerify() {
  if (!order.value) return
  try {
    await verifyOrder(order.value.id, verifyForm.remark)
    MessagePlugin.success('订单已核验')
    verifyVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '核验失败')
  }
}

const refundVisible = ref(false)
const refundForm = reactive<{
  paid_amount: number
  amount: number
  refund_mode: string
  fee_amount: number
  reason: string
  reasonPreset: string
}>({
  paid_amount: 0,
  amount: 0,
  refund_mode: 'balance',
  fee_amount: 0,
  reason: '',
  reasonPreset: '',
})

// 选中预设即填入原因输入框，仍可继续手改（预设只是快捷输入，不锁定文本）。
function applyReasonPreset(value: unknown): void {
  const text = typeof value === 'string' ? value : ''
  if (text) refundForm.reason = text
}

function openRefundDialog() {
  const target = order.value
  if (!target) return
  refundForm.paid_amount = target.paid_amount
  refundForm.amount = target.paid_amount
  refundForm.refund_mode = 'balance'
  refundForm.fee_amount = 0
  refundForm.reason = ''
  refundForm.reasonPreset = ''
  refundVisible.value = true
}

async function handleCreateRefund() {
  if (!order.value) return
  if (refundForm.amount <= 0) {
    MessagePlugin.warning('退款金额需大于 0')
    return
  }
  if (refundForm.refund_mode === 'channel' && refundForm.fee_amount < 0) {
    MessagePlugin.warning('渠道扣点不能为负数')
    return
  }
  try {
    await createOrderRefund(order.value.id, {
      amount: refundForm.amount,
      reason: refundForm.reason || undefined,
      refund_mode: refundForm.refund_mode,
      fee_amount: refundForm.refund_mode === 'channel' ? refundForm.fee_amount : 0,
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
