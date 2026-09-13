<template>
  <div class="page-body console-module order-detail-module" v-loading="loading">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <OrderIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">订单详情</h2>
          <p class="page-header__desc">
            {{ order?.order_no || '—' }}
            <span v-if="order" class="order-detail__created">下单于 {{ formatTime(order.created_at) }}</span>
          </p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/order')">返回列表</t-button>
        <template v-if="isPending">
          <t-button variant="outline" :loading="cancelling" @click="confirmCancel">取消订单</t-button>
          <t-button theme="primary" @click="cashierVisible = true">去支付</t-button>
        </template>
        <t-button
          v-else-if="order"
          variant="outline"
          :loading="provisioning"
          @click="refreshProvision"
        >
          刷新开通状态
        </t-button>
      </div>
    </header>

    <template v-if="order">
      <!-- 状态与履约：开通失败时把原因直接摊开，用户不必再提工单问 -->
      <section class="surface-card status-card">
        <div class="status-card__left">
          <t-tag :theme="statusTheme(order.status)" variant="light" size="large" shape="round">
            {{ statusText(order.status) }}
          </t-tag>
          <span class="status-card__amount">¥{{ payAmount.toFixed(2) }}</span>
          <span class="status-card__method">{{ isPending ? '待付款' : payMethodText(order.pay_method) }}</span>
        </div>
        <div v-if="order.provision_status" class="status-card__right">
          <t-tag :theme="provisionTheme(order.provision_status)" variant="light">
            开通：{{ provisionText(order.provision_status) }}
          </t-tag>
          <span v-if="order.provision_error" class="status-card__error">{{ order.provision_error }}</span>
        </div>
      </section>

      <!-- 待支付提示：截止时间由后端按下单时间 + 有效期推算，过期由调度器关单 -->
      <t-alert
        v-if="isPending"
        theme="warning"
        class="order-detail__alert"
        :message="payDeadlineMessage"
      />

      <section class="surface-card detail-card">
        <div class="table-card__head">
          <h3 class="card-title">商品信息</h3>
        </div>
        <t-descriptions :column="2" size="medium" bordered>
          <t-descriptions-item label="商品名称">{{ order.product_name }}</t-descriptions-item>
          <t-descriptions-item label="数量">{{ order.quantity || 1 }}</t-descriptions-item>
          <t-descriptions-item label="规格">
            {{ specText || '—' }}
          </t-descriptions-item>
          <t-descriptions-item label="计费周期">{{ cycleLabel(order.cycle) || '—' }}</t-descriptions-item>
          <t-descriptions-item label="下单账号">
            {{ order.actor_name || '主账号' }}
          </t-descriptions-item>
          <t-descriptions-item label="计费到期" v-if="order.expire_time">
            {{ formatTime(order.expire_time) }}
          </t-descriptions-item>
          <t-descriptions-item label="备注" :span="2" v-if="order.remark">
            {{ order.remark }}
          </t-descriptions-item>
        </t-descriptions>
      </section>

      <section class="surface-card detail-card">
        <div class="table-card__head">
          <h3 class="card-title">费用明细</h3>
          <span class="table-card__meta">金额口径与下单时一致的快照值</span>
        </div>

        <div class="fee-rows">
          <div class="fee-row">
            <span>商品原价</span>
            <span>¥{{ order.original_amount.toFixed(2) }}</span>
          </div>
          <div class="fee-row fee-row--discount" v-if="order.discount_amount > 0">
            <span>
              优惠金额
              <t-tag size="small" variant="light" theme="success">
                {{ sourceLabel(order.discount_source) }}
              </t-tag>
            </span>
            <span>-¥{{ order.discount_amount.toFixed(2) }}</span>
          </div>
          <div class="fee-row fee-row--final">
            <span>实付金额</span>
            <span>¥{{ payAmount.toFixed(2) }}</span>
          </div>
        </div>

        <div v-if="ruleSnapshot.length" class="rule-list">
          <span class="rule-list__title">优惠构成</span>
          <ul>
            <li v-for="(r, i) in ruleSnapshot" :key="i">
              {{ ruleLabel(r) }}
            </li>
          </ul>
        </div>
      </section>

      <section class="surface-card detail-card">
        <div class="table-card__head">
          <h3 class="card-title">时间线</h3>
        </div>
        <t-descriptions :column="2" size="medium" bordered>
          <t-descriptions-item label="下单时间">{{ formatTime(order.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="支付时间">
            {{ order.pay_time ? formatTime(order.pay_time) : '—' }}
          </t-descriptions-item>
          <t-descriptions-item v-if="order.pay_expire_at" label="支付截止" :span="2">
            {{ formatTime(order.pay_expire_at) }}
            <span class="order-detail__hint">（超时未支付系统自动关单）</span>
          </t-descriptions-item>
        </t-descriptions>
      </section>
    </template>

    <section v-else-if="!loading" class="empty-state surface-card">
      <t-empty description="订单不存在或无权查看">
        <template #action>
          <t-button theme="primary" @click="router.push('/order')">返回订单列表</t-button>
        </template>
      </t-empty>
    </section>

    <!-- 待支付订单的收银台：金额取订单应付，不信任前端传入 -->
    <PaymentCashier
      v-if="order"
      v-model:visible="cashierVisible"
      :amount="payAmount"
      :submit="submitPay"
      @paid="onPaid"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { OrderIcon } from 'tdesign-icons-vue-next'

import { cancelOrder, getOrderDetail, payOrder, type OrderInfo } from '@/api/shop'
import PaymentCashier, { type CashierPayment } from '@/components/payment-cashier/index.vue'

defineOptions({ name: 'OrderDetail' })

const route = useRoute()
const router = useRouter()

const order = ref<OrderInfo | null>(null)
const loading = ref(false)
const provisioning = ref(false)

/** 实付优先取算价快照，兼容未写快照的存量订单。 */
const payAmount = computed(() => order.value?.final_amount || order.value?.paid_amount || 0)

const isPending = computed(() => order.value?.status === 'pending')

const cashierVisible = ref(false)
const cancelling = ref(false)

function submitPay(channelCode: string, scene: string): Promise<CashierPayment> {
  return payOrder(order.value!.id, { channel_code: channelCode, scene }).then(({ data }) => ({
    payment_no: data.payment_no,
    amount: data.amount,
    pay_url: data.pay_url,
    qrcode: data.qrcode,
    instructions: data.instructions,
    status: data.status,
  }))
}

/** 到账后订单由后端回调推进到已支付/开通中，重取详情即为最新状态（含履约任务）。 */
async function onPaid() {
  MessagePlugin.success('支付完成，资源开通中')
  await load()
}

/** 支付截止提示：后端返回的是推算值（下单时间 + 订单有效期），前端只做倒计时文案。 */
const payDeadlineMessage = computed(() => {
  const raw = order.value?.pay_expire_at
  if (!raw) return '该订单尚未支付，请尽快完成支付；超时未付系统将自动关闭订单。'
  const deadline = new Date(raw).getTime()
  if (Number.isNaN(deadline)) return '该订单尚未支付，请尽快完成支付。'
  const left = deadline - Date.now()
  if (left <= 0) return '已超过支付截止时间，系统即将关闭该订单。'
  const minutes = Math.floor(left / 60000)
  const remain = minutes < 60 ? `${minutes} 分钟` : `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分钟`
  return `请在 ${formatTime(raw)} 前完成支付（剩余约 ${remain}）；超时未付将自动关闭订单并释放库存。`
})

/**
 * 取消待支付订单。
 *
 * 后端是 CAS 状态迁移：如果此刻渠道已经回调到账，取消会失败并返回「不可取消」，
 * 这时刷新详情反而能看到真实的已支付状态，比前端乐观改状态更可靠。
 */
function confirmCancel() {
  const dialog = DialogPlugin.confirm({
    header: '取消订单',
    body: '取消后订单作废，已占用的库存会释放；需要的话可以重新下单。确认取消？',
    confirmBtn: { content: '确认取消', theme: 'danger' },
    onConfirm: async () => {
      dialog.destroy()
      cancelling.value = true
      try {
        const { data } = await cancelOrder(order.value!.id)
        if (data) order.value = data
        MessagePlugin.success('订单已取消')
      } catch {
        // 已支付/已被关单：刷新看真实状态，不误导用户
        await load()
      } finally {
        cancelling.value = false
      }
    },
    onClose: () => dialog.destroy(),
  })
}

/** 规格：优先展示规格快照 JSON 里的可读项，取不到就退回原始字符串。 */
const specText = computed(() => {
  const raw = order.value?.specs
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') {
      const parts = Object.entries(parsed)
        .filter(([, v]) => v !== null && v !== '' && v !== undefined)
        .map(([k, v]) => `${k}: ${v}`)
      if (parts.length) return parts.join(' / ')
    }
  } catch {
    // 非 JSON（存量订单存的是自由文本），原样展示
  }
  return raw
})

interface PriceRule {
  source?: string
  code?: string
  type?: string
  value?: number
}

/** price_snapshot 是命中规则明细 JSON（P5-04），解析失败按无内容处理。 */
const ruleSnapshot = computed<PriceRule[]>(() => {
  const raw = order.value?.price_snapshot
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
})

function ruleLabel(r: PriceRule): string {
  const source = sourceLabel(r.source || '')
  const value = r.type === 'percent' ? `${((r.value ?? 0) * 100).toFixed(0)}%` : `¥${(r.value ?? 0).toFixed(2)}`
  return r.code ? `${source} · ${r.code} ${value}` : `${source} ${value}`
}

function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

function cycleLabel(c?: string): string {
  if (!c) return ''
  const map: Record<string, string> = {
    hourly: '按小时',
    monthly: '按月',
    quarterly: '按季',
    semiannually: '半年',
    annually: '按年',
    biennially: '两年',
    onetime: '一次性',
  }
  return map[c] || c
}

function payMethodText(m: string): string {
  const map: Record<string, string> = {
    balance: '余额支付',
    alipay: '支付宝',
    wechat: '微信支付',
  }
  return map[m] || m || ''
}

function statusText(s: string): string {
  return (
    {
      pending: '待支付',
      paid: '已支付',
      provisioning: '开通中',
      active: '服务中',
      cancelled: '已取消',
      closed: '已关闭',
      refunded: '已退款',
      completed: '已完成',
    }[s] || s
  )
}

function statusTheme(s: string): 'success' | 'warning' | 'default' | 'danger' {
  if (s === 'active' || s === 'paid' || s === 'completed') return 'success'
  if (s === 'provisioning' || s === 'pending') return 'warning'
  if (s === 'cancelled' || s === 'refunded' || s === 'closed') return 'danger'
  return 'default'
}

function provisionText(s: string): string {
  return (
    {
      pending: '排队中',
      running: '开通中',
      success: '已开通',
      failed: '开通失败',
      manual: '需人工处理',
    }[s] || s
  )
}

function provisionTheme(s: string): 'success' | 'warning' | 'default' | 'danger' {
  if (s === 'success') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'manual') return 'warning'
  return 'default'
}

function formatTime(v?: string): string {
  return v ? v.replace('T', ' ').slice(0, 16) : '—'
}

async function load() {
  const id = Number(route.params.id)
  if (!id || Number.isNaN(id)) {
    order.value = null
    return
  }
  loading.value = true
  try {
    const { data } = await getOrderDetail(id)
    order.value = data || null
  } catch {
    // 拦截器已提示；他人订单后端按「不存在」处理，这里落到空态
    order.value = null
  } finally {
    loading.value = false
  }
}

/** 开通是异步任务（T5.1），用户不必刷新整页，只重取履约状态。 */
async function refreshProvision() {
  provisioning.value = true
  await load()
  provisioning.value = false
}

onMounted(load)
</script>

<style scoped>
.order-detail__created {
  margin-left: var(--space-sm);
  color: var(--color-muted-foreground);
}

.order-detail__alert {
  border-radius: var(--hs-radius-lg);
}

.order-detail__hint {
  margin-left: 6px;
  color: var(--color-muted-foreground);
}

/* ============ 状态条 ============ */
.status-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  padding: var(--space-lg) var(--space-xl);
  flex-wrap: wrap;
}

.status-card__left,
.status-card__right {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
  min-width: 0;
}

.status-card__amount {
  font-size: 22px;
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.status-card__method {
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.status-card__error {
  font-size: 12.5px;
  color: var(--color-danger);
  word-break: break-all;
}

/* ============ 信息卡 ============ */
.detail-card {
  padding: var(--space-lg) 20px;
}

/* ============ 费用行 ============ */
.fee-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 480px;
}

.fee-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  font-size: 13.5px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.fee-row--discount {
  color: var(--color-accent);
}

.fee-row--final {
  margin-top: 4px;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border);
  font-size: 15px;
  font-weight: 700;
  color: var(--color-accent);
}

/* ============ 优惠构成 ============ */
.rule-list {
  margin-top: var(--space-md);
  padding-top: var(--space-md);
  border-top: 1px solid var(--color-border);
}

.rule-list__title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.rule-list ul {
  margin: 8px 0 0;
  padding-left: 18px;
  font-size: 12.5px;
  line-height: 1.9;
  color: var(--color-muted-foreground);
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .status-card,
  .detail-card {
    padding: var(--space-lg) var(--space-md);
  }

  .status-card__amount {
    font-size: 19px;
  }
}
</style>
