<template>
  <div class="hero-band">
    <section class="hero">
      <span class="hero__glow hero__glow--left" aria-hidden="true" />
      <span class="hero__glow hero__glow--right" aria-hidden="true" />

      <div class="site-container hero__layout">
        <!-- 左：竖向标签栏 -->
        <nav class="hero__rail" aria-label="首页快捷入口">
          <NuxtLink
            v-for="(tab, index) in HERO_TABS"
            :key="tab.label"
            class="hero__tab"
            :class="{ 'is-active': index === 0 }"
            :to="tab.to"
            :aria-current-value="ariaCurrentFor(tab.to)"
          >
            {{ tab.label }}
          </NuxtLink>
        </nav>

        <!-- 中：主标题与行动 -->
        <div class="hero__main">
          <h1 class="hero__title">{{ home.heroTitle }}</h1>
          <p class="hero__subtitle">{{ home.heroSubtitle }}</p>

          <div class="hero__actions">
            <NuxtLink class="hero__cta" :to="home.heroPrimaryLink || '/products'">
              {{ home.heroPrimaryCta }}
            </NuxtLink>
            <NuxtLink
              v-if="home.heroSecondaryCta"
              class="hero__cta hero__cta--ghost"
              :to="secondaryLink"
              :aria-current-value="ariaCurrentFor(secondaryLink)"
            >
              {{ home.heroSecondaryCta }}
            </NuxtLink>
          </div>
        </div>

        <!-- 右：抽象插画区（参考图的 3D 素材/AI 卡片用几何占位，不含任何第三方素材） -->
        <div class="hero__art" aria-hidden="true">
          <span class="hero__art-panel" />

          <span class="hero__art-card hero__art-card--chrome">
            <span class="hero__dots">
              <i class="hero__dot hero__dot--r" />
              <i class="hero__dot hero__dot--y" />
              <i class="hero__dot hero__dot--b" />
            </span>
            <span class="hero__bar hero__bar--wide" />
            <span class="hero__bar" />
          </span>

          <span class="hero__art-card hero__art-card--input">
            <span class="hero__bar hero__bar--grow" />
            <span class="hero__send" />
          </span>

          <span class="hero__art-card hero__art-card--stats">
            <span class="hero__bar hero__bar--wide" />
            <span class="hero__bar" />
            <span class="hero__bar hero__bar--short" />
            <span class="hero__tags">
              <i /><i /><i />
            </span>
          </span>
        </div>
      </div>
    </section>

    <!-- Banner 下沿卡片行：左侧快捷入口卡 + 右侧三栏能力面板 -->
    <div class="site-container hero__shelf">
      <div class="shelf__promo">
        <p class="shelf__promo-head">
          <span class="shelf__promo-accent">{{ HERO_SHELF_HEAD.accent }}</span>
          {{ HERO_SHELF_HEAD.title }}
          <SiteIcon name="chevron-right" :stroke-width="2" />
        </p>

        <ul class="shelf__links">
          <li v-for="link in HERO_SHELF_LINKS" :key="link.label">
            <NuxtLink class="shelf__link" :to="link.to" :aria-current-value="ariaCurrentFor(link.to)">
              <span>{{ link.label }}</span>
              <SiteIcon name="chevron-right" :stroke-width="2" />
            </NuxtLink>
          </li>
        </ul>
      </div>

      <ul class="shelf__cards">
        <li v-for="item in HERO_HIGHLIGHTS" :key="item.title" class="shelf__card">
          <span class="shelf__card-icon"><SiteIcon :name="item.icon" /></span>
          <h3 class="shelf__card-title">{{ item.title }}</h3>
          <p class="shelf__card-desc">{{ item.desc }}</p>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  HERO_HIGHLIGHTS,
  HERO_SHELF_HEAD,
  HERO_SHELF_LINKS,
  HERO_TABS,
} from '~/constants/homeContent'
import { ariaCurrentFor } from '~/utils/nav'

const { content } = useSiteContent()

const home = computed(() => content.value.home)
const secondaryLink = computed(() => home.value.heroSecondaryLink || '/#features')
</script>

<style scoped>
.hero-band {
  background: var(--site-bg);
}

.hero {
  position: relative;
  overflow: hidden;
  padding: 56px 0 68px;
  background: var(--site-hero-wash);
}

/* 抽象光斑：代替参考图右侧的 3D 素材 */
.hero__glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  pointer-events: none;
}

.hero__glow--left {
  width: 380px;
  height: 380px;
  top: -140px;
  left: -160px;
  background: var(--site-primary);
  opacity: 0.14;
}

.hero__glow--right {
  width: 520px;
  height: 520px;
  top: -180px;
  right: -140px;
  background: var(--site-primary);
  opacity: 0.18;
}

.hero__layout {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr) minmax(0, 1.05fr);
  align-items: center;
  gap: 28px 40px;
}

/* ---------- 左：竖向标签栏 ---------- */
.hero__rail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-right: 10px;
  border-right: 1px solid var(--site-border);
}

.hero__tab {
  padding: 13px 14px;
  border-radius: var(--site-radius);
  color: var(--site-text-muted);
  font-size: 14.5px;
  line-height: 1.5;
  transition: color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease;
}

.hero__tab:hover {
  color: var(--site-primary);
  background: rgba(var(--site-primary-rgb), 0.05);
}

.hero__tab.is-active {
  background: #fff;
  color: var(--site-primary);
  font-weight: 600;
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.08);
}

/* ---------- 中：标题与按钮 ---------- */
.hero__title {
  max-width: 620px;
  font-size: 46px;
  font-weight: 800;
  line-height: 1.24;
  letter-spacing: 0.005em;
  color: var(--site-text);
}

.hero__subtitle {
  margin-top: 18px;
  max-width: 520px;
  font-size: 15.5px;
  line-height: 1.85;
  color: var(--site-text-muted);
}

.hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 32px;
}

.hero__cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 48px;
  min-width: 148px;
  padding: 0 30px;
  border: 1px solid transparent;
  border-radius: var(--site-radius);
  background: var(--site-primary);
  color: #fff;
  font-size: 15.5px;
  font-weight: 600;
  box-shadow: 0 12px 26px var(--site-primary-border);
  transition: background-color 0.18s ease, transform 0.18s ease, border-color 0.18s ease;
}

.hero__cta:hover {
  background: var(--site-primary-strong);
  transform: translateY(-1px);
}

.hero__cta--ghost {
  background: #fff;
  border-color: var(--site-border);
  color: var(--site-text);
  box-shadow: none;
}

.hero__cta--ghost:hover {
  background: #fff;
  border-color: var(--site-primary-border);
  color: var(--site-primary);
}

/* ---------- 右：抽象插画 ---------- */
.hero__art {
  position: relative;
  height: 336px;
}

.hero__art-panel {
  position: absolute;
  inset: 0;
  border-radius: 20px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.14);
  background: linear-gradient(
    150deg,
    rgba(var(--site-primary-rgb), 0.16) 0%,
    rgba(var(--site-primary-rgb), 0.05) 100%
  );
}

.hero__art-card {
  position: absolute;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid var(--site-border-soft);
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.09);
}

.hero__art-card--chrome {
  top: 20px;
  right: -10px;
  width: 192px;
  height: 84px;
}

.hero__art-card--input {
  top: 132px;
  left: -22px;
  width: 236px;
  height: 60px;
  flex-direction: row;
  align-items: center;
  gap: 12px;
}

.hero__art-card--stats {
  right: 26px;
  bottom: 18px;
  width: 208px;
  height: 104px;
}

.hero__dots {
  display: flex;
  gap: 5px;
}

.hero__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: rgba(var(--site-primary-rgb), 0.28);
}

.hero__dot--r {
  background: #fda4af;
}

.hero__dot--y {
  background: #fcd34d;
}

.hero__dot--b {
  background: #93c5fd;
}

.hero__bar {
  display: block;
  width: 82px;
  height: 7px;
  border-radius: 4px;
  background: rgba(var(--site-primary-rgb), 0.16);
}

.hero__bar--wide {
  width: 128px;
}

.hero__bar--short {
  width: 54px;
}

.hero__bar--grow {
  flex: 1;
  width: auto;
}

.hero__send {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--site-primary);
}

.hero__tags {
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.hero__tags i {
  width: 34px;
  height: 12px;
  border-radius: 999px;
  background: rgba(var(--site-primary-rgb), 0.12);
}

/* ---------- Banner 下沿卡片行 ---------- */
.hero__shelf {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: minmax(0, 344px) minmax(0, 1fr);
  gap: 20px;
  margin-top: -34px;
  padding-bottom: 10px;
}

.shelf__promo {
  padding: 22px 24px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: #fff;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.07);
}

.shelf__promo-head {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 16.5px;
  font-weight: 700;
  color: var(--site-text);
}

.shelf__promo-accent {
  color: var(--site-primary);
}

.shelf__promo-head :deep(.site-icon) {
  width: 16px;
  height: 16px;
  color: var(--site-text-subtle);
}

.shelf__links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
  margin: 14px 0 0;
  padding: 0;
  list-style: none;
}

/* 每行一条虚线，分隔感来自参考图那张权益卡 */
.shelf__link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 10px 2px;
  border-bottom: 1px dashed var(--site-border);
  color: var(--site-text-muted);
  font-size: 13.5px;
  transition: color 0.18s ease;
}

.shelf__link:hover {
  color: var(--site-primary);
}

.shelf__link :deep(.site-icon) {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  color: var(--site-text-subtle);
}

.shelf__cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  padding: 0;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: #fff;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.07);
  list-style: none;
  overflow: hidden;
}

.shelf__card {
  padding: 24px 26px;
}

.shelf__card + .shelf__card {
  border-left: 1px solid var(--site-border-soft);
}

.shelf__card-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 1px solid var(--site-border);
  border-radius: 10px;
  background: var(--site-bg-muted);
  color: var(--site-primary);
}

.shelf__card-icon :deep(.site-icon) {
  width: 19px;
  height: 19px;
}

.shelf__card-title {
  margin-top: 16px;
  font-size: 16.5px;
  font-weight: 600;
  color: var(--site-text);
}

.shelf__card-desc {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.75;
  color: var(--site-text-muted);
}

@media (max-width: 1080px) {
  .hero__layout {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  /* 标签栏在窄屏收起：横向排布会挤掉标题的呼吸感 */
  .hero__rail {
    display: none;
  }

  .hero__title {
    font-size: 38px;
  }

  .hero__shelf {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 860px) {
  .hero {
    padding: 40px 0 56px;
  }

  .hero__layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero__title {
    font-size: 31px;
  }

  .hero__subtitle {
    font-size: 15px;
  }

  .hero__art {
    height: 260px;
  }

  .hero__art-card--input {
    left: 0;
  }

  .hero__art-card--chrome {
    right: 0;
  }

  .hero__shelf {
    margin-top: -28px;
  }
}

@media (max-width: 700px) {
  .shelf__cards {
    grid-template-columns: minmax(0, 1fr);
  }

  .shelf__card + .shelf__card {
    border-left: none;
    border-top: 1px solid var(--site-border-soft);
  }
}

@media (max-width: 560px) {
  .hero__title {
    font-size: 27px;
  }

  .hero__cta {
    min-width: 0;
    flex: 1 1 auto;
  }

  .shelf__links {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero__art {
    height: 224px;
  }
}
</style>
