<template>
  <t-dialog
    v-model:visible="dialogVisible"
    :header="title"
    width="520px"
    :footer="false"
    @close="close"
  >
    <!-- 第一步：选支付方式，提交后才向渠道下单 -->
    <template v-if="!payment">
      <t-alert
        theme="info"
        :message="`应付 ¥${amountText}。提交后进入收银台，按所选渠道完成支付；线下方式需等待人工确认到账。`"
        style="margin-bottom: 16px"
      />
      <t-form label-align="top" :data="{ channelCode }" @submit.prevent>
        <t-form-item label="支付方式" name="channelCode">
          <t-select
            v-model="channelCode"
            placeholder="请选择支付方式"
            :options="channelOptions"
            :loading="methodsLoading"
          />
        </t-form-item>
      </t-form>
      <p v-if="!methodsLoading && !channelOptions.length" class="cashier-tip">
        当前没有可用支付渠道，请联系平台客服或稍后再试。
      </p>
      <t-space size="small" class="cashier-actions">
        <t-button
          theme="primary"
          :loading="submitting"
          :disabled="!channelCode"
          @click="submitPayment"
        >
          提交并支付
        </t-button>
        <t-button variant="outline" @click="close">稍后支付</t-button>
      </t-space>
    </template>

    <!-- 第二步：渠道支付参数 + 查询支付结果 -->
    <template v-else>
      <t-alert
        theme="success"
        :message="`支付单 ${payment.payment_no} 已创建，应付 ¥${(payment.amount ?? amount).toFixed(2)}`"
        style="margin-bottom: 12px"
      />
      <div v-if="payment.instructions" class="cashier-block">
        <h4 class="cashier-block__title">支付指引</h4>
        <pre class="cashier-pre">{{ payment.instructions }}</pre>
      </div>
      <div v-if="payment.qrcode" class="cashier-block">
        <h4 class="cashier-block__title">收款二维码</h4>
        <img v-if="isImage(payment.qrcode)" class="cashier-qr" :src="payment.qrcode" alt="收款二维码" />
        <pre v-else class="cashier-pre">{{ payment.qrcode }}</pre>
      </div>
      <div v-if="payment.pay_url" class="cashier-block">
        <h4 class="cashier-block__title">前往支付</h4>
        <a class="cashier-link" :href="payment.pay_url" target="_blank" rel="noopener noreferrer">
          {{ payment.pay_url }}
        </a>
      </div>
      <t-space size="small" class="cashier-actions">
        <t-button theme="primary" :loading="checking" @click="checkPayment">我已完成支付</t-button>
        <t-button variant="outline" @click="close">稍后支付</t-button>
      </t-space>
      <p class="cashier-tip">
        支付完成后点击「我已完成支付」刷新状态；线下渠道需等待财务确认到账，到账后资源会自动开通。
      </p>
    </template>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { getPaymentMethods, getPaymentOrder } from '@/api/payment'

/** 收银台渲染所需的支付单字段（充值/订单/账单三类业务同口径）。 */
export interface CashierPayment {
  payment_no: string
  amount: number
  pay_url?: string
  qrcode?: string
  instructions?: string
  status?: string
}

/**
 * 通用收银台组件。
 *
 * 抽出来的原因：充值、订单支付、账单支付三条链路的下单接口不同，但「选渠道 → 展示渠道参数
 * → 轮询支付结果」这套交互完全一致；各页各写一份会出现三处不一致的收银台。
 *
 * 契约（父组件负责业务，组件只负责收银台）：
 *   - submit(channelCode, scene) 由父组件实现，内部完成「建业务单 → 发起支付」，返回支付单；
 *   - 到账后由后端回调驱动业务入账/开通，组件只轮询支付单状态并 emit('paid')；
 *   - 父组件收到 paid 后刷新自己的页面数据。
 */
const props = withDefaults(
  defineProps<{
    visible: boolean
    /** 标题（默认「收银台」）。 */
    title?: string
    /** 应付金额（元）：用于渠道限额过滤与展示。 */
    amount: number
    /** 支付场景：native/h5/jsapi/scan，默认 native。 */
    scene?: string
    /** 发起支付的业务回调，返回渠道支付参数。 */
    submit: (channelCode: string, scene: string) => Promise<CashierPayment>
  }>(),
  { title: '收银台', scene: 'native' },
)

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  /** 支付单已确认到账。 */
  paid: [payment: CashierPayment]
  /** 弹窗关闭（含「稍后支付」与右上角关闭），用于清理父组件状态。 */
  closed: []
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit('update:visible', v),
})

const amountText = computed(() => props.amount.toFixed(2))

const channelCode = ref('')
const channelOptions = ref<Array<{ label: string; value: string }>>([])
const methodsLoading = ref(false)
const submitting = ref(false)
const checking = ref(false)
const payment = ref<CashierPayment | null>(null)

/**
 * 打开时重置并拉取可用渠道（渠道限额与金额相关，金额变了渠道集合可能不同）。
 *
 * immediate 是必需的：父组件可能在挂载时就以 visible=true 渲染（例如用 :key 切订单时重建），
 * 这时没有 true→false 的变化可等，不立即执行就永远拉不到渠道。
 */
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      payment.value = null
      submitting.value = false
      checking.value = false
      void loadMethods()
      return
    }
    // 关闭后清掉渠道缓存，下次打开按最新金额重新过滤
    channelOptions.value = []
    channelCode.value = ''
  },
  { immediate: true },
)

async function loadMethods() {
  methodsLoading.value = true
  try {
    const { data } = await getPaymentMethods({
      scene: props.scene,
      amount: props.amount > 0 ? props.amount : undefined,
    })
    const channels = data?.channels || []
    channelOptions.value = channels.map((c) => ({ label: c.name, value: c.channel_code }))
    // 默认选中后端给出的偏好渠道，其次第一个
    channelCode.value = data?.default || channelOptions.value[0]?.value || ''
  } catch {
    channelOptions.value = []
    channelCode.value = ''
  } finally {
    methodsLoading.value = false
  }
}

async function submitPayment() {
  if (!channelCode.value) {
    MessagePlugin.warning('请选择支付方式')
    return
  }
  submitting.value = true
  try {
    const info = await props.submit(channelCode.value, props.scene)
    payment.value = info
    // 渠道给了跳转链接且无二维码/指引时直接打开，减少一次点击
    if (info.pay_url && !info.instructions && !info.qrcode) {
      window.open(info.pay_url, '_blank', 'noopener')
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '发起支付失败')
  } finally {
    submitting.value = false
  }
}

/** 轮询支付单：渠道回调到账后前端可感知（组件只查询，不改业务状态）。 */
async function checkPayment() {
  if (!payment.value) return
  checking.value = true
  try {
    const { data } = await getPaymentOrder(payment.value.payment_no)
    if (!data) return
    payment.value = { ...payment.value, ...data }
    if (data.status === 'paid') {
      MessagePlugin.success('支付已完成')
      emit('paid', payment.value)
      dialogVisible.value = false
      return
    }
    if (data.status === 'pending' || data.status === 'paying') {
      MessagePlugin.info('支付尚未完成，请稍候再试；线下方式需等待人工确认到账')
      return
    }
    MessagePlugin.warning(`支付单状态：${data.status || '未知'}`)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '查询支付状态失败')
  } finally {
    checking.value = false
  }
}

function close() {
  dialogVisible.value = false
  emit('closed')
}

function isImage(value: string): boolean {
  return /^(data:image|https?:\/\/.*\.(png|jpe?g|gif|webp|svg))/i.test(value)
}
</script>

<style scoped>
.cashier-block {
  margin-bottom: 14px;
}

.cashier-block__title {
  margin: 0 0 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.cashier-pre {
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--hs-surface-3, #f8fafc);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.cashier-qr {
  width: 180px;
  height: 180px;
  object-fit: contain;
  border: 1px solid var(--color-border, #e2e8f0);
  border-radius: 8px;
}

.cashier-link {
  font-size: 12px;
  color: var(--color-primary, #2563eb);
  word-break: break-all;
}

.cashier-actions {
  margin-top: 6px;
}

.cashier-tip {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
  line-height: 1.6;
}
</style>
