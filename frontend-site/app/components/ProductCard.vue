<template>
  <NuxtLink :to="`/products/${product.id}`" class="product-card">
    <div class="product-card__media">
      <img
        v-if="product.coverImage"
        :src="product.coverImage"
        :alt="product.name"
        class="product-card__img"
        loading="lazy"
      >
      <span v-else class="product-card__fallback" aria-hidden="true">{{ initial }}</span>
    </div>

    <div class="product-card__body">
      <h3 class="product-card__title">{{ product.name }}</h3>
      <p class="product-card__desc">{{ product.description || '按需开通，弹性伸缩，开箱即用。' }}</p>

      <ul v-if="specs.length" class="product-card__specs">
        <li v-for="spec in specs" :key="spec.key" class="product-card__spec">
          <span class="product-card__spec-label">{{ spec.label }}</span>
          <span class="product-card__spec-value">{{ spec.value }}</span>
        </li>
      </ul>

      <div class="product-card__footer">
        <span class="product-card__price">
          <em class="product-card__price-value">{{ formatPriceAmount(product.price) }}</em>
          <span v-if="unit" class="product-card__price-unit">{{ unit }}</span>
        </span>
        <span class="product-card__more">查看详情 →</span>
      </div>
    </div>
  </NuxtLink>
</template>

<script setup lang="ts">
import {
  formatPriceAmount,
  priceUnit,
  type Product,
} from '#shared/schemas/product'

const props = defineProps<{ product: Product }>()

/** 卡片上最多展示 3 条规格，超出会撑高卡片且信息噪音大。 */
const specs = computed(() => props.product.specs.slice(0, 3))
const unit = computed(() => priceUnit(props.product.priceModel))
const initial = computed(() => props.product.name.trim().charAt(0) || 'P')
</script>

<style scoped>
.product-card {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: #fff;
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.product-card:hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.08);
}

.product-card__media {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 148px;
  background: linear-gradient(135deg, var(--site-primary-soft), rgba(15, 23, 42, 0.04));
}

.product-card__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.product-card__fallback {
  font-size: 40px;
  font-weight: 700;
  color: var(--site-primary-strong);
  opacity: 0.5;
}

.product-card__body {
  display: flex;
  flex-direction: column;
  flex: 1;
  padding: 20px;
}

.product-card__title {
  font-size: 17px;
  font-weight: 600;
}

.product-card__desc {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--site-text-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.product-card__specs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 14px 0 0;
  padding: 0;
  list-style: none;
}

.product-card__spec {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border-radius: 999px;
  background: var(--site-bg-muted);
  font-size: 12px;
  color: var(--site-text-muted);
}

.product-card__spec-label {
  color: var(--site-text-subtle);
}

.product-card__spec-value {
  color: var(--site-text);
  font-weight: 500;
}

.product-card__footer {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-top: auto;
  padding-top: 18px;
}

.product-card__price {
  display: inline-flex;
  align-items: baseline;
  gap: 2px;
  color: var(--site-primary-strong);
}

.product-card__price-value {
  font-size: 20px;
  font-weight: 700;
  font-style: normal;
}

.product-card__price-unit {
  font-size: 12px;
  color: var(--site-text-muted);
}

.product-card__more {
  font-size: 13px;
  color: var(--site-text-subtle);
  transition: color 0.18s ease;
}

.product-card:hover .product-card__more {
  color: var(--site-primary);
}
</style>
