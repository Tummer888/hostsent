<template>
  <section id="console" class="site-section featured">
    <div class="site-container">
      <SectionHeading :title="heading.title" :subtitle="heading.subtitle" />

      <div class="featured__grid">
        <!-- 左：深色控制台入口卡 -->
        <aside class="featured__panel">
          <span class="featured__panel-logo">{{ site.name }}</span>
          <h3 class="featured__panel-title">控制台快速入口</h3>
          <p class="featured__panel-desc">
            产品、订单、实例与工单都在用户控制台里闭环。
          </p>

          <ul class="featured__entries">
            <li v-for="entry in CONSOLE_ENTRIES" :key="entry.label">
              <component
                :is="entryUrl(entry.path) ? 'a' : 'div'"
                :href="entryUrl(entry.path) || undefined"
                class="featured__entry"
                :class="{ 'is-disabled': !entryUrl(entry.path) }"
                rel="noopener"
              >
                <span class="featured__entry-icon"><SiteIcon :name="entry.icon" /></span>
                <span class="featured__entry-label">{{ entry.label }}</span>
                <SiteIcon name="chevron-right" :stroke-width="1.9" class="featured__entry-arrow" />
              </component>
            </li>
          </ul>

          <p v-if="!consoleBase" class="featured__panel-hint">
            未配置控制台地址（<code>NUXT_PUBLIC_CONSOLE_URL</code>），入口暂不可点。
          </p>
        </aside>

        <!-- 右：2×2 链接卡 -->
        <ul class="featured__links">
          <li v-for="link in FEATURED_LINKS" :key="link.title">
            <component
              :is="link.to ? 'NuxtLink' : 'div'"
              :to="link.to"
              class="link-card"
              :class="{ 'is-pending': !link.to }"
            >
              <span class="link-card__head">
                <span class="link-card__icon"><SiteIcon :name="link.icon" /></span>
                <span v-if="link.badge" class="link-card__badge">{{ link.badge }}</span>
              </span>
              <h4 class="link-card__title">{{ link.title }}</h4>
              <p class="link-card__desc">{{ link.desc }}</p>
              <span class="link-card__more">
                {{ link.to ? '前往' : '即将上线' }}
                <SiteIcon :name="link.to ? 'arrow-right' : 'chevron-right'" :stroke-width="1.9" />
              </span>
            </component>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { CONSOLE_ENTRIES, FEATURED_LINKS } from '~/constants/homeContent'

const { content } = useSiteContent()
const { public: publicConfig } = useRuntimeConfig()

const site = computed(() => content.value.site)

const heading = {
  title: '上手与支持',
  subtitle: '从文档到控制台，把开通、排障与自动化接口都放在顺手的位置。',
}

/** 控制台地址未配置时返回空串，入口降级为不可点，避免造死链。 */
const consoleBase = computed(() => String(publicConfig.consoleUrl || '').replace(/\/+$/, ''))

function entryUrl(path: string): string {
  return consoleBase.value ? `${consoleBase.value}${path}` : ''
}
</script>

<style scoped>
.featured {
  background: var(--site-bg);
}

.featured__grid {
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr);
  gap: 24px;
  margin-top: 34px;
}

/* 左侧控制台入口卡：浅蓝渐变底，与右侧白卡拉开主次 */
.featured__panel {
  display: flex;
  flex-direction: column;
  padding: 28px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.16);
  border-radius: var(--site-radius-lg);
  background: linear-gradient(
    160deg,
    rgba(var(--site-primary-rgb), 0.1) 0%,
    rgba(var(--site-primary-rgb), 0.03) 100%
  );
}

.featured__panel-logo {
  align-self: flex-start;
  padding: 5px 12px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.22);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.7);
  font-size: 12px;
  color: var(--site-primary-strong);
}

.featured__panel-title {
  margin-top: 20px;
  font-size: 22px;
  font-weight: 700;
  color: var(--site-text);
}

.featured__panel-desc {
  margin-top: 10px;
  font-size: 13.5px;
  line-height: 1.8;
  color: var(--site-text-muted);
}

.featured__entries {
  margin: 22px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.featured__entry {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.featured__entry:not(.is-disabled):hover {
  transform: translateY(-1px);
  border-color: var(--site-primary-border);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.07);
}

.featured__entry.is-disabled {
  opacity: 0.55;
}

.featured__entry-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border-radius: 8px;
  background: var(--site-primary-soft);
  color: var(--site-primary-strong);
}

.featured__entry-icon :deep(.site-icon) {
  width: 17px;
  height: 17px;
}

.featured__entry-label {
  flex: 1;
  font-size: 14px;
  color: var(--site-text);
}

.featured__entry-arrow {
  width: 15px;
  height: 15px;
  color: var(--site-text-subtle);
}

.featured__panel-hint {
  margin-top: 18px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--site-text-muted);
}

.featured__panel-hint code {
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(var(--site-primary-rgb), 0.08);
  color: var(--site-primary-strong);
}

/* 右侧 2×2 */
.featured__links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.link-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 22px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: var(--site-bg-muted);
  transition: border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
}

.link-card:not(.is-pending):hover {
  transform: translateY(-3px);
  border-color: var(--site-primary-border);
  background: #fff;
}

.link-card__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.link-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: var(--site-radius);
  background: #fff;
  color: var(--site-primary-strong);
  border: 1px solid var(--site-border);
}

.link-card__icon :deep(.site-icon) {
  width: 20px;
  height: 20px;
}

.link-card__badge {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--site-primary);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.link-card__title {
  margin-top: 16px;
  font-size: 16px;
  font-weight: 600;
}

.link-card__desc {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.75;
  color: var(--site-text-muted);
}

.link-card__more {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-top: auto;
  padding-top: 16px;
  font-size: 12.5px;
  color: var(--site-text-subtle);
}

.link-card__more :deep(.site-icon) {
  width: 14px;
  height: 14px;
}

@media (max-width: 960px) {
  .featured__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .featured__links {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
