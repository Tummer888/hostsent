<template>
  <section id="partners" class="site-section site-section--tight partners">
    <div class="site-container">
      <SectionHeading
        :title="heading.title"
        :subtitle="heading.subtitle"
      />

      <ul class="site-shelf partners__shelf">
        <li v-for="item in PARTNERS" :key="item.name" class="partners__tile">
          <span class="partners__mark" :class="`partners__mark--${item.variant}`" aria-hidden="true">
            {{ item.name.charAt(0) }}
          </span>
          <span class="partners__name">{{ item.name }}</span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { PARTNERS } from '~/constants/homeContent'

const heading = {
  title: '已对接的上游与生态',
  subtitle: '财务与云平台上游各接一次，产品、订单、实例与账单在同一处收敛。',
}
</script>

<style scoped>
.partners {
  background: linear-gradient(180deg, var(--site-primary-soft) 0%, rgba(255, 255, 255, 0) 100%);
}

.partners__shelf {
  margin-top: 30px;
  gap: 12px;
}

.partners__tile {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 196px;
  padding: 14px 16px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
}

/* 小色块只做点缀，用实底 + 白字保证首字母可读；浅色底靠描边区分 */
.partners__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 9px;
  color: #fff;
  font-size: 14px;
  font-weight: 700;
}

.partners__mark--1 {
  background: var(--site-primary);
}

.partners__mark--2 {
  background: var(--site-primary-strong);
}

.partners__mark--3 {
  border: 1px solid var(--site-primary-border);
  background: var(--site-primary-soft);
  color: var(--site-primary-strong);
}

.partners__mark--4 {
  background: linear-gradient(140deg, var(--site-primary) 0%, var(--site-primary-strong) 100%);
}

.partners__name {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--site-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 560px) {
  .partners__tile {
    width: 168px;
  }
}
</style>
