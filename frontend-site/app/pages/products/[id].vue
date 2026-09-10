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
            <span class="detail-price__amount">{{ formatPriceAmount(product.price) }}</span>
            <span v-if="unit" class="detail-price__unit">{{ unit }}</span>
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

/** 控制台地址未配置时不渲染购买入口，避免死链。 */
const purchaseUrl = computed(() => {
  if (!publicConfig.consoleUrl) return ''
  const base = String(publicConfig.consoleUrl).replace(/\/+$/, '')
  return `${base}/shop?product=${product.value?.id ?? ''}`
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
