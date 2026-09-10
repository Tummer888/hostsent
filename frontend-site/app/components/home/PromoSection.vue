<template>
  <section id="promos" class="site-section site-section--tight promo">
    <div class="site-container">
      <SectionHeading
        :title="heading.title"
        :subtitle="heading.subtitle"
        action-label="查看全部活动"
        action-to="/products"
      />

      <ul class="site-shelf promo__shelf">
        <li v-for="item in PROMOS" :key="item.title" class="promo__cell">
          <component
            :is="item.to ? 'NuxtLink' : 'div'"
            :to="item.to"
            :aria-current-value="item.to ? ariaCurrentFor(item.to) : undefined"
            class="promo-card"
            :class="[`promo-card--${item.variant}`, { 'is-pending': !item.to }]"
          >
            <span class="promo-card__tag">{{ item.tag }}</span>
            <h3 class="promo-card__title">{{ item.title }}</h3>
            <p class="promo-card__desc">{{ item.desc }}</p>

            <span v-if="item.to" class="promo-card__more">
              查看详情
              <SiteIcon name="arrow-right" :stroke-width="1.9" />
            </span>
            <span v-else class="promo-card__soon">即将上线</span>
          </component>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { PROMOS } from '~/constants/homeContent'
import { ariaCurrentFor } from '~/utils/nav'

const heading = {
  title: '新用户与长期客户优惠',
  subtitle: '代金券、首购折扣与续费同价都在这里，活动信息未来由后台活动模块下发。',
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
