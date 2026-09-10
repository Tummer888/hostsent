<template>
  <section class="site-section site-section--tight quick">
    <div class="site-container">
      <div class="quick__grid">
        <NuxtLink
          v-for="item in QUICK_ACTIONS"
          :key="item.title"
          :to="item.to"
          :aria-current-value="ariaCurrentFor(item.to)"
          class="quick-card"
        >
          <span class="quick-card__icon"><SiteIcon :name="item.icon" /></span>
          <h3 class="quick-card__title">{{ item.title }}</h3>
          <p class="quick-card__desc">{{ item.desc }}</p>
          <span class="quick-card__action">
            {{ item.actionLabel }}
            <SiteIcon name="arrow-right" :stroke-width="1.9" />
          </span>
        </NuxtLink>

        <aside class="quick-promise">
          <h3 class="quick-promise__title">{{ SERVICE_PROMISE.title }}</h3>
          <p class="quick-promise__desc">{{ SERVICE_PROMISE.desc }}</p>
          <ul class="quick-promise__list">
            <li v-for="point in SERVICE_PROMISE.points" :key="point">
              <SiteIcon name="chevron-right" :stroke-width="2" />
              {{ point }}
            </li>
          </ul>
        </aside>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { QUICK_ACTIONS, SERVICE_PROMISE } from '~/constants/homeContent'
import { ariaCurrentFor } from '~/utils/nav'
</script>

<style scoped>
.quick {
  background: var(--site-bg);
}

.quick__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr)) minmax(0, 0.9fr);
  gap: 20px;
}

.quick-card {
  display: flex;
  flex-direction: column;
  padding: 26px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.14);
  border-radius: var(--site-radius-lg);
  background: var(--site-art-3);
  color: var(--site-text);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.quick-card:last-of-type {
  background: var(--site-art-1);
}

.quick-card:hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.1);
}

.quick-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: var(--site-radius);
  background: rgba(255, 255, 255, 0.85);
  color: var(--site-primary-strong);
}

.quick-card__icon :deep(.site-icon) {
  width: 22px;
  height: 22px;
}

.quick-card__title {
  margin-top: 18px;
  font-size: 20px;
  font-weight: 700;
}

.quick-card__desc {
  margin-top: 10px;
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.quick-card__action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: auto;
  padding-top: 22px;
  color: var(--site-primary);
  font-size: 14px;
  font-weight: 600;
}

.quick-card__action :deep(.site-icon) {
  width: 16px;
  height: 16px;
}

.quick-promise {
  padding: 26px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: var(--site-bg-muted);
}

.quick-promise__title {
  font-size: 16px;
  font-weight: 700;
}

.quick-promise__desc {
  margin-top: 10px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.quick-promise__list {
  margin: 18px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.quick-promise__list li {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--site-text);
}

.quick-promise__list :deep(.site-icon) {
  width: 14px;
  height: 14px;
  color: var(--site-primary);
}

@media (max-width: 960px) {
  .quick__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
