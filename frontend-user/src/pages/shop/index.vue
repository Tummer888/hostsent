<template>
  <div class="page-body console-module shop-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CartIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">云主机选购</h2>
          <p class="page-header__desc">余额支付即时开通；加入购物车后可一并结算</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/cart')">
          购物车
          <template #suffix v-if="cartStore.count > 0">({{ cartStore.count }})</template>
        </t-button>
      </div>
    </header>

    <!-- 站点跳转带来的意图：?product=<id>&spec=<spec_code>&cycle=<cycle> -->
    <section v-if="intentProductName" class="surface-card intent-card">
      <CheckCircleIcon size="18" class="intent-card__icon" />
      <span>来自官网的选购意向：<strong>{{ intentProductName }}</strong></span>
      <t-button size="small" variant="text" @click="clearIntent">知道了</t-button>
    </section>

    <section class="filter-card surface-card">
      <div class="filter-card__grid">
        <div class="field">
          <label class="field__label" for="shop-keyword">关键词</label>
          <t-input
            id="shop-keyword"
            v-model="keyword"
            placeholder="搜索商品名称"
            clearable
            @enter="search"
            @clear="search"
          />
        </div>
        <div class="field">
          <label class="field__label">只看推荐</label>
          <t-switch v-model="featuredOnly" @change="search" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-button variant="outline" @click="resetFilter">重置</t-button>
        <t-button theme="primary" :loading="loading" @click="search">查询</t-button>
      </div>
    </section>

    <section v-loading="loading" class="shop-grid">
      <article v-for="p in products" :key="p.id" class="surface-card shop-card">
        <div class="shop-card__cover">
          <img v-if="p.cover_image" :src="p.cover_image" :alt="p.name" />
          <component :is="ServerIcon" v-else size="34" />
          <span v-if="p.featured" class="shop-card__flag">推荐</span>
        </div>

        <div class="shop-card__body">
          <h3 class="shop-card__name">{{ p.name }}</h3>
          <p class="shop-card__desc">{{ p.description || '暂无描述' }}</p>

          <div class="shop-card__meta">
            <t-tag size="small" variant="light">{{ sourceModeLabel(p.source_mode) }}</t-tag>
            <t-tag v-if="p.skus.length" size="small" variant="light" theme="primary">
              {{ p.skus.length }} 种规格
            </t-tag>
          </div>

          <div class="shop-card__foot">
            <div class="price-cell">
              <span class="shop-card__price">¥{{ p.price.toFixed(2) }}</span>
              <span class="shop-card__unit">{{ priceModelLabel(p.price_model) }}</span>
            </div>
            <t-button size="small" theme="primary" @click="openBuy(p)">购买</t-button>
          </div>
        </div>
      </article>

      <div v-if="!loading && !products.length" class="empty-state surface-card">
        <t-empty description="暂无可购买商品" />
      </div>
    </section>

    <t-pagination
      v-if="total > pagination.pageSize"
      v-model:current="pagination.current"
      v-model:page-size="pagination.pageSize"
      :total="total"
      :page-size-options="[12, 24, 48]"
      class="shop-pager"
      @change="loadProducts"
    />

    <!-- 购买：规格 + 周期 + 数量 → 实时预结算 -->
    <t-dialog
      v-model:visible="buyVisible"
      header="确认购买"
      width="520px"
      :confirm-btn="{ content: '余额支付并开通', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="submitOrder"
    >
      <div v-if="current" class="buy">
        <div class="buy__product">{{ current.name }}</div>
        <p class="buy__desc">{{ current.description }}</p>

        <div v-if="current.skus.length" class="buy__field">
          <span class="buy__label">规格</span>
          <t-radio-group v-model="selectedSpec" variant="default-filled" @change="refreshQuote">
            <t-radio-button v-for="s in current.skus" :key="s.spec_code" :value="s.spec_code">
              {{ s.name }}
              <em class="buy__spec-price">¥{{ s.price.toFixed(2) }}</em>
            </t-radio-button>
          </t-radio-group>
          <p v-if="selectedSku?.stock === 0" class="buy__warn">该规格已售罄</p>
        </div>

        <div v-if="selectableCycles.length" class="buy__field">
          <span class="buy__label">计费周期</span>
          <t-radio-group v-model="selectedCycle" variant="default-filled" @change="refreshQuote">
            <t-radio-button v-for="c in selectableCycles" :key="c" :value="c">
              {{ cycleLabel(c) }}
            </t-radio-button>
          </t-radio-group>
        </div>

        <div class="buy__field">
          <span class="buy__label">数量</span>
          <t-input-number v-model="quantity" :min="1" :max="99" theme="normal" @change="refreshQuote" />
        </div>

        <div v-loading="quoteLoading" class="buy__prices">
          <div v-if="quote" class="buy__rows">
            <div class="buy__row">
              <span>商品原价</span>
              <span>¥{{ quote.original_amount.toFixed(2) }}</span>
            </div>
            <div v-if="quote.discount_amount > 0" class="buy__row buy__row--discount">
              <span>
                优惠金额
                <t-tag size="small" variant="light" theme="success">
                  {{ sourceLabel(quote.discount_source) }}
                </t-tag>
              </span>
              <span>-¥{{ quote.discount_amount.toFixed(2) }}</span>
            </div>
            <div class="buy__row buy__row--final">
              <span>应付金额</span>
              <span>¥{{ quote.final_amount.toFixed(2) }}</span>
            </div>
          </div>
          <t-empty v-else-if="!quoteLoading" description="价格计算失败，请稍后重试" />
        </div>
      </div>

      <template #footer>
        <div class="buy__footer">
          <t-button variant="outline" :disabled="!current" @click="addCurrentToCart">加入购物车</t-button>
          <div class="buy__footer-right">
            <t-button variant="outline" @click="buyVisible = false">取消</t-button>
            <t-button
              theme="primary"
              :loading="submitting"
              :disabled="!quote || soldOut"
              @click="submitOrder"
            >
              余额支付并开通
            </t-button>
          </div>
        </div>
      </template>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { CartIcon, CheckCircleIcon, ServerIcon } from 'tdesign-icons-vue-next'

import {
  createOrder,
  getProductDetail,
  getProducts,
  quoteOrder,
  type ProductInfo,
  type QuoteInfo,
  type SkuInfo,
} from '@/api/shop'
import { useCartStore } from '@/store/modules/cart'

defineOptions({ name: 'Shop' })

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()

// ========== 列表 ==========
const products = ref<ProductInfo[]>([])
const loading = ref(false)
const keyword = ref('')
const featuredOnly = ref(false)
const total = ref(0)
const pagination = ref({ current: 1, pageSize: 12 })

/**
 * 官网跳转带来的选购意图（doc87 §2.2）：
 * /shop?product=<id>&spec=<spec_code>&cycle=<cycle>。
 * 意图只用来「预填 + 自动拉起购买弹窗」，不直接下单 —— 支付金额必须由用户确认。
 */
const intentProductName = ref('')

// ========== 购买弹窗 ==========
const buyVisible = ref(false)
const current = ref<ProductInfo | null>(null)
const selectedSpec = ref('')
const selectedCycle = ref('')
const quantity = ref(1)
const quote = ref<QuoteInfo | null>(null)
const quoteLoading = ref(false)
const submitting = ref(false)

const selectedSku = computed<SkuInfo | undefined>(() =>
  current.value?.skus.find((s) => s.spec_code === selectedSpec.value),
)

/** 周期来源：选中规格的自有周期优先，否则回落商品级周期。 */
const selectableCycles = computed(() => selectedSku.value?.cycles?.length ? selectedSku.value.cycles : current.value?.cycles ?? [])

const soldOut = computed(() => selectedSku.value?.stock === 0)

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

function sourceModeLabel(m: string): string {
  return m === 'upstream' ? '上游直供' : '自营'
}

/** 折扣来源中文标签（P5-06）：折扣仅由用户组价格策略承载。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

// ========== 数据加载 ==========
async function loadProducts() {
  loading.value = true
  try {
    const { data } = await getProducts({
      keyword: keyword.value || undefined,
      featured: featuredOnly.value || undefined,
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
    })
    products.value = data?.items || []
    total.value = data?.total || 0
  } catch {
    products.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.value.current = 1
  loadProducts()
}

function resetFilter() {
  keyword.value = ''
  featuredOnly.value = false
  search()
}

// ========== 购买流程 ==========
async function openBuy(product: ProductInfo, specCode = '', cycle = '') {
  // 列表里的商品不带 skus/cycles（列表接口不下发），拉一次详情再开弹窗。
  current.value = product
  buyVisible.value = true
  quote.value = null
  quantity.value = 1

  try {
    const { data } = await getProductDetail(product.id)
    if (data) current.value = data
  } catch {
    // 详情拿不到就按商品级价格下单，不阻断（存量商品本就没有 SKU）
  }

  selectedSpec.value = specCode || current.value?.skus[0]?.spec_code || ''
  const cycles = selectedSpec.value
    ? current.value?.skus.find((s) => s.spec_code === selectedSpec.value)?.cycles ?? []
    : []
  const available = cycles.length ? cycles : current.value?.cycles ?? []
  selectedCycle.value = cycle && available.includes(cycle) ? cycle : available[0] || ''

  refreshQuote()
}

async function refreshQuote() {
  if (!current.value) return
  quoteLoading.value = true
  try {
    const { data } = await quoteOrder({
      productId: current.value.id,
      specCode: selectedSpec.value,
      cycle: selectedCycle.value,
      quantity: quantity.value,
    })
    quote.value = data
  } catch {
    quote.value = null
  } finally {
    quoteLoading.value = false
  }
}

async function submitOrder() {
  if (!current.value || !quote.value) return
  if (soldOut.value) {
    MessagePlugin.warning('所选规格已售罄')
    return
  }
  submitting.value = true
  try {
    const { data } = await createOrder({
      productId: current.value.id,
      specCode: selectedSpec.value,
      cycle: selectedCycle.value,
      quantity: quantity.value,
    })
    MessagePlugin.success(`已购买「${current.value.name}」，资源开通中`)
    buyVisible.value = false
    cartStore.remove(
      // 下单成功后把同商品同规格同周期的购物车行移除，避免重复结算
      `${current.value.id}::${selectedSpec.value || '-'}::${selectedCycle.value || '-'}`,
    )
    if (data?.id) router.push(`/order/${data.id}`)
  } catch {
    // 请求拦截器已提示错误原因（余额不足/库存不足等）
  } finally {
    submitting.value = false
  }
}

function addCurrentToCart() {
  if (!current.value) return
  const sku = selectedSku.value
  cartStore.add({
    productId: current.value.id,
    productName: current.value.name,
    coverImage: current.value.cover_image,
    specCode: selectedSpec.value,
    specName: sku?.name || '默认规格',
    cycle: selectedCycle.value,
    quantity: quantity.value,
    unitPrice: quote.value?.final_amount && quantity.value
      ? quote.value.final_amount / quantity.value
      : sku?.price || current.value.price,
    priceModel: current.value.price_model,
  })
  MessagePlugin.success('已加入购物车')
  buyVisible.value = false
}

// ========== 官网跳转意图 ==========
function clearIntent() {
  intentProductName.value = ''
  router.replace({ path: '/shop', query: {} })
}

async function consumeIntent() {
  const raw = route.query.product
  const productId = Number(Array.isArray(raw) ? raw[0] : raw)
  if (!productId || Number.isNaN(productId)) return

  const spec = String(route.query.spec ?? '')
  const cycle = String(route.query.cycle ?? '')

  try {
    const { data } = await getProductDetail(productId)
    if (!data) return
    intentProductName.value = data.name
    // 商品已下架时详情接口直接报错，这里自然走不到
    await openBuy(data, spec, cycle)
  } catch {
    MessagePlugin.warning('官网带来的商品不可购买，可能已下架')
    clearIntent()
  }
}

onMounted(async () => {
  await loadProducts()
  await consumeIntent()
})
</script>

<style scoped>
/* ============ 来自官网的选购意图提示条 ============ */
.intent-card {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 12px var(--space-xl);
  font-size: 13px;
  color: var(--color-foreground);
}

.intent-card__icon {
  color: var(--color-primary);
  flex-shrink: 0;
}

.intent-card :deep(.t-button) {
  margin-left: auto;
}

/* ============ 商品网格 ============ */
.shop-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(272px, 1fr));
  gap: var(--space-lg);
  min-height: 160px;
}

.shop-grid .empty-state {
  grid-column: 1 / -1;
}

.shop-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: transform var(--hs-duration-base) var(--hs-ease-out),
    box-shadow var(--hs-duration-base) var(--hs-ease-out);
}

.shop-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--hs-shadow-md);
}

.shop-card__cover {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 116px;
  background: linear-gradient(135deg, var(--hs-surface-3) 0%, var(--hs-surface-2) 100%);
  color: var(--color-muted-foreground);
  overflow: hidden;
}

.shop-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.shop-card__flag {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--color-primary);
  color: #fff;
  font-size: 11px;
}

.shop-card__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-lg);
  flex: 1;
}

.shop-card__name {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--color-foreground);
}

.shop-card__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 40px;
}

.shop-card__meta {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  flex-wrap: wrap;
}

.shop-card__foot {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-top: auto;
  padding-top: var(--space-sm);
}

.shop-card__price {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.shop-card__unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.shop-pager {
  justify-content: flex-end;
}

/* ============ 购买弹窗 ============ */
.buy {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.buy__product {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.buy__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.buy__field {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.buy__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.buy__spec-price {
  margin-left: 4px;
  font-style: normal;
  opacity: 0.75;
  font-size: 12px;
}

.buy__warn {
  margin: 0;
  font-size: 12px;
  color: var(--color-danger);
}

.buy__prices {
  min-height: 96px;
  border-top: 1px solid var(--color-border);
  padding-top: var(--space-md);
}

.buy__rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.buy__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.buy__row--discount {
  color: var(--color-accent);
}

.buy__row--final {
  margin-top: 4px;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border);
  font-size: 15px;
  font-weight: 700;
  color: var(--color-accent);
}

.buy__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  width: 100%;
}

.buy__footer-right {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .shop-grid {
    grid-template-columns: 1fr;
    gap: var(--space-md);
  }

  .intent-card {
    padding: 12px var(--space-md);
  }

  .buy__footer {
    flex-direction: column-reverse;
    align-items: stretch;
  }

  .buy__footer-right {
    flex-direction: column-reverse;
    align-items: stretch;
  }
}
</style>
