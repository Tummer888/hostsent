<template>
  <section id="pricing" class="site-section site-section--tight promo">
    <div class="site-container">
      <SectionHeading
        :title="heading.title"
        :subtitle="heading.subtitle"
        action-label="浏览全部产品"
        action-to="/products"
      />

      <ul class="site-shelf promo__shelf">
        <li v-for="item in PRICING_CARDS" :key="item.title" class="promo__cell">
          <!--
            卡片目的地：站内页走 NuxtLink；用户中心入口走 <a> 拼控制台地址
            （与 FeaturedShowcase 的控制台入口同一套规则）。控制台地址未配置时
            降级成不可点的 <div>，而不是造一个点进去 404 的链接。
          -->
          <component
            :is="cardUrl(item) ? (item.to ? 'NuxtLink' : 'a') : 'div'"
            :to="item.to"
            :href="item.to ? undefined : cardUrl(item) || undefined"
            :rel="item.to ? undefined : 'noopener'"
            :aria-current-value="item.to ? ariaCurrentFor(item.to) : undefined"
            class="promo-card"
            :class="[`promo-card--${item.variant}`, { 'is-pending': !cardUrl(item) }]"
          >
            <span class="promo-card__tag">{{ item.tag }}</span>
            <h3 class="promo-card__title">{{ item.title }}</h3>
            <p class="promo-card__desc">{{ item.desc }}</p>

            <span v-if="cardUrl(item)" class="promo-card__more">
              查看详情
              <SiteIcon name="arrow-right" :stroke-width="1.9" />
            </span>
            <span v-else class="promo-card__soon">需先配置控制台地址</span>
          </component>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { PRICING_CARDS, type PricingCard } from '~/constants/homeContent'
import { ariaCurrentFor } from '~/utils/nav'

const { public: publicConfig } = useRuntimeConfig()

const heading = {
  title: '计费方式与优惠',
  subtitle: '周期价格、邀请返利与自动续费都是平台已经在跑的能力，入口直达对应页面。',
}

const consoleBase = computed(() => String(publicConfig.consoleUrl || '').replace(/\/+$/, ''))

/** 解析卡片目的地：站内地址直接返回；控制台相对路径拼控制台地址。 */
function cardUrl(item: PricingCard): string {
  if (item.to) return item.to
  if (!item.consolePath || !consoleBase.value) return ''
  return `${consoleBase.value}${item.consolePath}`
}
</script>

<style scoped>
.promo {
  background: var(--site-bg);
}

.promo__shelf {
  margin-top: 32px;
}

.promo__cell {
  width: 268px;
}

.promo-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 196px;
  padding: 22px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.14);
  border-radius: var(--site-radius-lg);
  /* 渐变是主色低透明度，底色浅，文字走深色（与 PlaceholderArt 同一条规则） */
  color: var(--site-text);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.promo-card--1 {
  background: var(--site-art-1);
}

.promo-card--2 {
  background: var(--site-art-2);
}

.promo-card--3 {
  background: var(--site-art-3);
}

.promo-card--4 {
  background: var(--site-art-4);
}

.promo-card--5 {
  background: var(--site-art-5);
}

.promo-card:not(.is-pending):hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.1);
}

.promo-card__tag {
  align-self: flex-start;
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.85);
  color: var(--site-primary-strong);
  font-size: 12px;
  font-weight: 600;
}

.promo-card__title {
  margin-top: 16px;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.4;
}

.promo-card__desc {
  margin-top: 10px;
  font-size: 13px;
  line-height: 1.75;
  color: var(--site-text-muted);
}

.promo-card__more,
.promo-card__soon {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: auto;
  padding-top: 18px;
  color: var(--site-primary);
  font-size: 12.5px;
  font-weight: 500;
}

.promo-card__more :deep(.site-icon) {
  width: 14px;
  height: 14px;
}

.promo-card__soon {
  color: var(--site-text-subtle);
}
</style>
