<template>
  <section id="resources" class="site-section resources">
    <div class="site-container">
      <SectionHeading
        :title="heading.title"
        :subtitle="heading.subtitle"
        action-label="查看全部"
        action-to="/products"
      />

      <ul class="resources__grid">
        <li v-for="item in RESOURCES" :key="item.title">
          <component
            :is="item.to ? 'NuxtLink' : 'div'"
            :to="item.to"
            class="res-card"
            :class="{ 'is-pending': !item.to }"
          >
            <PlaceholderArt :variant="item.variant" :icon="item.icon" :label="item.artLabel" />
            <h3 class="res-card__title">{{ item.title }}</h3>
            <p class="res-card__desc">{{ item.desc }}</p>
            <span class="res-card__more">
              {{ item.to ? '查看' : '即将上线' }}
              <SiteIcon name="arrow-right" :stroke-width="1.9" />
            </span>
          </component>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { RESOURCES } from '~/constants/homeContent'

const heading = {
  title: '资源与支持',
  subtitle: '产品更新、技术分享与落地案例，都在这里沉淀。',
}
</script>

<style scoped>
.resources {
  background: var(--site-bg-muted);
}

.resources__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
  margin: 34px 0 0;
  padding: 0;
  list-style: none;
}

.res-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: #fff;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.res-card:not(.is-pending):hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.08);
}

.res-card__title {
  margin-top: 16px;
  padding: 0 6px;
  font-size: 16px;
  font-weight: 600;
}

.res-card__desc {
  margin-top: 8px;
  padding: 0 6px;
  font-size: 13px;
  line-height: 1.75;
  color: var(--site-text-muted);
}

.res-card__more {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: auto;
  padding: 16px 6px 4px;
  font-size: 12.5px;
  color: var(--site-text-subtle);
}

.res-card__more :deep(.site-icon) {
  width: 14px;
  height: 14px;
}

@media (max-width: 1024px) {
  .resources__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .resources__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
