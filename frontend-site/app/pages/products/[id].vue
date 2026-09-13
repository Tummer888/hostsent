<template>
  <div v-if="product" class="product-detail">
    <div class="site-container">
      <nav class="detail-breadcrumb" aria-label="面包屑">
        <NuxtLink to="/">首页</NuxtLink>
        <span class="detail-breadcrumb__sep">/</span>
        <NuxtLink to="/products">全部产品</NuxtLink>
        <span class="detail-breadcrumb__sep">/</span>
        <span class="detail-breadcrumb__current">{{ product.name }}</span>
      </nav>

      <div class="detail-main">
        <div class="detail-media">
          <img
            v-if="product.coverImage"
            :src="product.coverImage"
            :alt="product.name"
            class="detail-media__img"
          >
          <span v-else class="detail-media__fallback" aria-hidden="true">
            {{ product.name.trim().charAt(0) || 'P' }}
          </span>
        </div>

        <div class="detail-info">
          <h1 class="detail-info__title">{{ product.name }}</h1>
          <p v-if="product.description" class="detail-info__desc">{{ product.description }}</p>

          <div class="detail-price">
            <span class="detail-price__amount">{{ formatPriceAmount(displayPrice) }}</span>
            <span v-if="unit" class="detail-price__unit">{{ unit }}</span>
            <span v-if="selectedSku" class="detail-price__sku">{{ selectedSku.name }}</span>
          </div>

          <!-- 规格与周期只做「选择 + 传递意图」：算价与下单必须在用户中心完成 -->
          <div v-if="product.skus.length" class="detail-option">
            <h3 class="detail-option__title">选择规格</h3>
            <div class="detail-option__list">
              <button
                v-for="sku in product.skus"
                :key="sku.specCode"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': sku.specCode === selectedSpec }"
                :disabled="sku.stock === 0"
                @click="selectedSpec = sku.specCode"
              >
                <span class="detail-option__name">{{ sku.name }}</span>
                <span class="detail-option__price">
                  {{ sku.stock === 0 ? '已售罄' : formatPriceAmount(sku.price) }}
                </span>
              </button>
            </div>
          </div>

          <div v-if="selectableCycles.length" class="detail-option">
            <h3 class="detail-option__title">计费周期</h3>
            <div class="detail-option__list">
              <button
                v-for="c in selectableCycles"
                :key="c"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': c === selectedCycle }"
                @click="selectedCycle = c"
              >
                <span class="detail-option__name">{{ cycleLabel(c) }}</span>
              </button>
            </div>
          </div>

          <div class="detail-actions">
            <a
              v-if="purchaseUrl"
              :href="purchaseUrl"
              class="site-btn site-btn--primary"
              rel="noopener"
            >
              立即选购
            </a>
            <NuxtLink v-else to="/#contact" class="site-btn site-btn--ghost">
              联系咨询
            </NuxtLink>
            <NuxtLink to="/products" class="site-btn site-btn--ghost">返回产品列表</NuxtLink>
          </div>
        </div>
      </div>

      <section v-if="product.specs.length" class="detail-block">
        <h2 class="detail-block__title">规格参数</h2>
        <dl class="detail-specs">
          <div v-for="spec in product.specs" :key="spec.key" class="detail-specs__row">
            <dt class="detail-specs__label">{{ spec.label }}</dt>
            <dd class="detail-specs__value">{{ spec.value }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="product.description" class="detail-block">
        <h2 class="detail-block__title">产品说明</h2>
        <p class="detail-block__text">{{ product.description }}</p>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatPriceAmount, priceUnit } from '#shared/schemas/product'

const route = useRoute()
const { content } = useSiteContent()
const { public: publicConfig } = useRuntimeConfig()

/**
 * 先 await 再解构：await 会剥离 Object.assign 附加的属性，
 * 因此要保留 original 引用读取产品与状态。
 * 必须等待的原因：SSR 阶段同步读取会拿到尚未就绪的数据，
 * 从而把正常商品误判为已下线并返回 404。
 */
const detail = useProduct(route.params.id as string)
await detail

const { product, state } = detail

if (state.value !== 'ok' || !product.value) {
  throw createError(
    state.value === 'unavailable'
      ? { statusCode: 503, statusMessage: 'Product service unavailable', fatal: true }
      : { statusCode: 404, statusMessage: 'Product not found', fatal: true },
  )
}

const unit = computed(() => priceUnit(product.value?.priceModel ?? ''))

/**
 * 规格/周期选择。
 *
 * 官网只负责把选中的 spec_code 与 cycle 拼进跳转链接，不做算价也不下单：
 * 价格随用户组折扣、余额、周期矩阵变化，只有登录后的用户中心才有权威结果。
 */
const selectedSpec = ref('')
const selectedCycle = ref('')

const selectedSku = computed(() =>
  product.value?.skus.find((s) => s.specCode === selectedSpec.value),
)

/** 周期来源：选中规格的自有周期优先，否则回落商品级周期。 */
const selectableCycles = computed(() =>
  selectedSku.value?.cycles.length ? selectedSku.value.cycles : product.value?.cycles ?? [],
)

/** 展示价：选了规格就用规格价，否则用商品价。 */
const displayPrice = computed(() => selectedSku.value?.price ?? product.value?.price ?? 0)

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

// 默认选中第一个可售规格与第一个周期，让「立即选购」开箱可用
watchEffect(() => {
  const skus = product.value?.skus ?? []
  if (selectedSpec.value || !skus.length) return
  const preferred = skus.find((s) => s.stock !== 0) ?? skus[0]
  if (preferred) selectedSpec.value = preferred.specCode
})

watch(
  selectableCycles,
  (cycles) => {
    if (cycles.length && !cycles.includes(selectedCycle.value)) {
      selectedCycle.value = cycles[0] as string
    }
  },
  { immediate: true },
)

/** 控制台地址未配置时不渲染购买入口，避免死链。 */
const purchaseUrl = computed(() => {
  if (!publicConfig.consoleUrl) return ''
  const base = String(publicConfig.consoleUrl).replace(/\/+$/, '')
  const params = new URLSearchParams({ product: String(product.value?.id ?? '') })
  if (selectedSpec.value) params.set('spec', selectedSpec.value)
  if (selectedCycle.value) params.set('cycle', selectedCycle.value)
  return `${base}/shop?${params.toString()}`
})

useSeoMeta({
  title: () => product.value?.name ?? '',
  description: () => product.value?.description || content.value.site.slogan,
  ogTitle: () => `${product.value?.name ?? ''} · ${content.value.site.name}`,
  ogDescription: () => product.value?.description || content.value.site.slogan,
  ogType: 'website',
  ogImage: () => product.value?.coverImage || undefined,
})
</script>

<style scoped>
.product-detail {
  padding: 28px 0 72px;
}

.detail-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--site-text-muted);
}

.detail-breadcrumb a:hover {
  color: var(--site-primary);
}

.detail-breadcrumb__sep {
  color: var(--site-text-subtle);
}

.detail-breadcrumb__current {
  color: var(--site-text);
}

.detail-main {
  display: grid;
  grid-template-columns: minmax(0, 440px) minmax(0, 1fr);
  gap: 40px;
  margin-top: 26px;
}

.detail-media {
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 4 / 3;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: linear-gradient(135deg, var(--site-primary-soft), rgba(15, 23, 42, 0.04));
  overflow: hidden;
}

.detail-media__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-media__fallback {
  font-size: 64px;
  font-weight: 700;
  color: var(--site-primary-strong);
  opacity: 0.5;
}

.detail-info__title {
  font-size: 28px;
  font-weight: 700;
}

.detail-info__desc {
  margin-top: 14px;
  font-size: 14.5px;
  line-height: 1.9;
  color: var(--site-text-muted);
}

.detail-price {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-top: 26px;
  padding: 18px 20px;
  border-radius: var(--site-radius);
  background: var(--site-bg-muted);
  color: var(--site-primary-strong);
}

.detail-price__amount {
  font-size: 32px;
  font-weight: 700;
}

.detail-price__unit {
  font-size: 14px;
  color: var(--site-text-muted);
}

.detail-price__sku {
  margin-left: auto;
  font-size: 13px;
  color: var(--site-text-muted);
}

/* ---------- 规格 / 周期选择 ---------- */
.detail-option {
  margin-top: 24px;
}

.detail-option__title {
  margin-bottom: 10px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--site-text);
}

.detail-option__list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.detail-option__item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 120px;
  padding: 10px 14px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
  color: var(--site-text);
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease;
}

.detail-option__item:hover:not(:disabled) {
  border-color: var(--site-primary);
}

.detail-option__item.is-active {
  border-color: var(--site-primary);
  background: var(--site-primary-soft);
  color: var(--site-primary-strong);
}

.detail-option__item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.detail-option__price {
  font-size: 12px;
  color: var(--site-text-muted);
}

.detail-option__item.is-active .detail-option__price {
  color: var(--site-primary-strong);
}

.detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 26px;
}

.detail-block {
  margin-top: 52px;
}

.detail-block__title {
  font-size: 20px;
  font-weight: 600;
}

.detail-block__text {
  margin-top: 14px;
  font-size: 14.5px;
  line-height: 1.9;
  color: var(--site-text-muted);
  white-space: pre-line;
}

.detail-specs {
  margin: 18px 0 0;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  overflow: hidden;
}

.detail-specs__row {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  border-bottom: 1px solid var(--site-border-soft);
}

.detail-specs__row:last-child {
  border-bottom: none;
}

.detail-specs__label,
.detail-specs__value {
  margin: 0;
  padding: 12px 18px;
  font-size: 14px;
}

.detail-specs__label {
  background: var(--site-bg-muted);
  color: var(--site-text-muted);
}

.detail-specs__value {
  color: var(--site-text);
}

@media (max-width: 900px) {
  .detail-main {
    grid-template-columns: minmax(0, 1fr);
    gap: 26px;
  }

  .detail-info__title {
    font-size: 23px;
  }
}

@media (max-width: 560px) {
  .detail-specs__row {
    grid-template-columns: minmax(0, 1fr);
  }

  .detail-specs__label {
    padding-bottom: 0;
  }
}
</style>
