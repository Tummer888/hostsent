<template>
  <div class="page-body console-module cart-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CartIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">购物车</h2>
          <p class="page-header__desc">
            共 {{ cartStore.count }} 件商品，预估金额 ¥{{ cartStore.estimatedAmount.toFixed(2) }}
          </p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/shop')">继续选购</t-button>
        <t-button v-if="!cartStore.isEmpty" variant="outline" theme="danger" @click="confirmClear">
          清空
        </t-button>
      </div>
    </header>

    <section v-if="!cartStore.isEmpty" class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">商品清单</h3>
        <span class="table-card__meta">
          结算时金额以实时算价为准，预估金额仅供参考
        </span>
      </div>

      <ul class="cart-list">
        <li v-for="it in cartStore.items" :key="it.key" class="cart-item">
          <div class="cart-item__thumb">
            <img v-if="it.coverImage" :src="it.coverImage" :alt="it.productName" />
            <component :is="ServerIcon" v-else size="24" />
          </div>

          <div class="cart-item__main">
            <button class="cart-item__name" @click="router.push('/shop')">{{ it.productName }}</button>
            <div class="cart-item__meta">
              <t-tag size="small" variant="light">{{ it.specName || '默认规格' }}</t-tag>
              <t-tag v-if="it.cycle" size="small" variant="light" theme="primary">
                {{ cycleLabel(it.cycle) }}
              </t-tag>
            </div>
          </div>

          <div class="cart-item__price price-cell">
            <span class="price-cell__main">¥{{ it.unitPrice.toFixed(2) }}</span>
            <span class="price-cell__sub">{{ priceModelLabel(it.priceModel) }}</span>
          </div>

          <div class="cart-item__qty">
            <t-input-number
              :model-value="it.quantity"
              :min="1"
              :max="99"
              theme="normal"
              size="small"
              @change="(v: number) => cartStore.updateQuantity(it.key, v)"
            />
          </div>

          <div class="cart-item__amount">
            ¥{{ (it.unitPrice * it.quantity).toFixed(2) }}
          </div>

          <div class="cart-item__actions">
            <t-tooltip content="移出购物车">
              <t-button variant="text" theme="danger" size="small" @click="cartStore.remove(it.key)">
                删除
              </t-button>
            </t-tooltip>
          </div>
        </li>
      </ul>
    </section>

    <section v-else class="empty-state surface-card">
      <t-empty description="购物车还是空的">
        <template #action>
          <t-button theme="primary" @click="router.push('/shop')">去选购云主机</t-button>
        </template>
      </t-empty>
    </section>

    <!-- 结算：逐条实时算价，全部成功才允许下单 -->
    <section v-if="!cartStore.isEmpty" class="surface-card settle-bar">
      <div class="settle-bar__sum">
        <span class="settle-bar__label">结算金额</span>
        <span class="settle-bar__value">¥{{ settleTotal.toFixed(2) }}</span>
        <span v-if="settleDiscount > 0" class="settle-bar__hint">
          已优惠 ¥{{ settleDiscount.toFixed(2) }}
        </span>
      </div>
      <t-button
        theme="primary"
        size="large"
        :loading="submitting"
        :disabled="!quotesReady"
        @click="confirmSettle"
      >
        余额支付并开通（{{ cartStore.items.length }}）
      </t-button>
    </section>

    <t-alert v-if="quoteError" theme="warning" :message="quoteError" class="cart-alert" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { CartIcon, ServerIcon } from 'tdesign-icons-vue-next'

import { createOrder, quoteOrder, type QuoteInfo } from '@/api/shop'
import { useCartStore, type CartItem } from '@/store/modules/cart'

defineOptions({ name: 'CartPage' })

const router = useRouter()
const cartStore = useCartStore()

/** 每行的实时算价结果，key 与购物车行 key 一致。 */
const quotes = ref<Record<string, QuoteInfo>>({})
const quoteError = ref('')
const submitting = ref(false)

const settleTotal = computed(() =>
  cartStore.items.reduce((sum, it) => sum + (quotes.value[it.key]?.final_amount ?? 0), 0),
)
const settleDiscount = computed(() =>
  cartStore.items.reduce((sum, it) => sum + (quotes.value[it.key]?.discount_amount ?? 0), 0),
)
/** 所有行都有算价结果才允许提交：有行算价失败说明该商品已下架/周期关闭。 */
const quotesReady = computed(
  () => cartStore.items.length > 0 && cartStore.items.every((it) => !!quotes.value[it.key]),
)

function cycleLabel(c: string): string {
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

function priceModelLabel(m: string): string {
  const map: Record<string, string> = {
    monthly: '/月',
    quarterly: '/季',
    annually: '/年',
    hourly: '/小时',
    fixed: '一次性',
    onetime: '一次性',
  }
  return map[m] || ''
}

/**
 * 逐行实时算价。
 *
 * 不信任购物车里存的 unitPrice（那是加入时的快照，可能已过期）：
 * 结算金额一律以 quoteOrder 的结果为准，算价失败的行会置空从而禁用提交。
 */
async function refreshQuotes() {
  quoteError.value = ''
  const next: Record<string, QuoteInfo> = {}
  const failed: CartItem[] = []

  for (const it of cartStore.items) {
    try {
      const { data } = await quoteOrder({
        productId: it.productId,
        specCode: it.specCode,
        cycle: it.cycle,
        quantity: it.quantity,
      })
      if (data) next[it.key] = data
    } catch {
      failed.push(it)
    }
  }

  quotes.value = next
  if (failed.length) {
    quoteError.value = `${failed.map((it) => it.productName).join('、')} 已下架或该规格/周期不再可售，请从购物车移除后再结算。`
  }
}

function confirmClear() {
  const dialog = DialogPlugin.confirm({
    header: '清空购物车',
    body: '确认移出购物车中的全部商品？',
    confirmBtn: { content: '清空', theme: 'danger' },
    onConfirm: () => {
      cartStore.clear()
      quotes.value = {}
      dialog.destroy()
    },
    onClose: () => dialog.destroy(),
  })
}

/**
 * 结算 = 逐条下单。
 *
 * 后端下单是「余额支付 + 即刻开通」，没有购物车批量结算接口（doc87 §6.2 澄清 2），
 * 这里按行串行调用 POST /uc/orders；成功的行移出购物车，失败的行保留并提示，
 * 用户能一眼看出哪一条没买成，不用整单重来。
 */
async function settle() {
  submitting.value = true
  const failed: string[] = []
  let lastOrderId = 0

  for (const it of cartStore.items) {
    if (!quotes.value[it.key]) continue
    try {
      const { data } = await createOrder({
        productId: it.productId,
        specCode: it.specCode,
        cycle: it.cycle,
        quantity: it.quantity,
      })
      if (data?.id) lastOrderId = data.id
      cartStore.remove(it.key)
    } catch {
      failed.push(it.productName)
    }
  }

  submitting.value = false

  if (failed.length) {
    MessagePlugin.warning(`以下商品下单失败：${failed.join('、')}`)
    await refreshQuotes()
    return
  }

  MessagePlugin.success('下单成功，资源开通中')
  if (lastOrderId) router.push(`/order/${lastOrderId}`)
  else router.push('/order')
}

function confirmSettle() {
  const dialog = DialogPlugin.confirm({
    header: '确认结算',
    body: `将从账户余额扣除 ¥${settleTotal.value.toFixed(2)}，下单后即时开通资源。`,
    confirmBtn: { content: '确认支付' },
    onConfirm: () => {
      dialog.destroy()
      settle()
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(refreshQuotes)
</script>

<style scoped>
/* ============ 清单（用列表而非表格：移动端不需要横向滚动） ============ */
.cart-list {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.cart-item {
  display: grid;
  grid-template-columns: 56px minmax(0, 1fr) 110px 120px 110px 72px;
  align-items: center;
  gap: var(--space-md);
  padding: 14px 0;
  border-bottom: 1px solid var(--color-border);
}

.cart-item:last-child {
  border-bottom: none;
}

.cart-item__thumb {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-3);
  color: var(--color-muted-foreground);
  overflow: hidden;
}

.cart-item__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cart-item__main {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.cart-item__name {
  padding: 0;
  border: none;
  background: transparent;
  color: var(--color-foreground);
  font-size: 14px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cart-item__name:hover {
  color: var(--color-primary);
}

.cart-item__meta {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  flex-wrap: wrap;
}

.cart-item__amount {
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.cart-item__actions {
  text-align: right;
}

/* ============ 结算条 ============ */
.settle-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  padding: var(--space-lg) var(--space-xl);
  flex-wrap: wrap;
}

.settle-bar__sum {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
}

.settle-bar__label {
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.settle-bar__value {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.settle-bar__hint {
  font-size: 12px;
  color: var(--color-success);
}

.cart-alert {
  border-radius: var(--hs-radius-lg);
}

/* ============ 响应式 ============ */
@media (max-width: 900px) {
  .cart-item {
    grid-template-columns: 48px minmax(0, 1fr) auto;
    grid-template-areas:
      'thumb main actions'
      'thumb price qty';
    gap: var(--space-sm) var(--space-md);
  }

  .cart-item__thumb { grid-area: thumb; width: 48px; height: 48px; }
  .cart-item__main { grid-area: main; }
  .cart-item__price { grid-area: price; }
  .cart-item__qty { grid-area: qty; justify-self: end; }
  .cart-item__actions { grid-area: actions; }
  /* 单行小计在窄屏冗余（数量 × 单价已可见），隐藏以省空间 */
  .cart-item__amount { display: none; }
}

@media (max-width: 768px) {
  .settle-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .settle-bar__sum {
    justify-content: space-between;
  }
}
</style>
