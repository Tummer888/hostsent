<template>
  <div class="login-page">
    <aside class="login-page__brand" aria-label="平台介绍">
      <header class="brand-header">
        <div class="brand-logo" aria-label="Hostsent 宿派云控">
          <img
            :src="logoMark"
            alt="Hostsent 标识"
            class="brand-logo__mark"
            width="36"
            height="36"
          />
          <div class="brand-logo__text">
            <div class="brand-name">hostsent</div>
            <div class="brand-tag">宿派云控</div>
          </div>
        </div>
        <div class="brand-status" aria-label="系统状态">
          <span class="status-dot" aria-hidden="true"></span>
          <span class="status-label">全部服务正常运行</span>
        </div>
      </header>

      <section class="brand-hero">
        <p class="brand-eyebrow">Cloud Operations Platform</p>
        <h1 class="brand-title">
          掌控你的<span class="brand-title--accent">云基础设施</span>
        </h1>
        <p class="brand-subtitle">
          为 IDC / 云服务商打造的高可用管理平台，集成云主机、用户、工单、计费、
          监控、通知等核心能力，后台由宿派团队自主研发。
        </p>

        <figure class="brand-illustration">
          <img
            :src="heroIllustration"
            alt="Hostsent 云控平台仪表盘预览"
            class="brand-illustration__img"
            loading="eager"
            width="640"
            height="360"
          />
          <figcaption class="brand-illustration__caption">
            <CheckCircleFilledIcon size="14" aria-hidden="true" />
            实时资源监控 · 多云编排 · 一体化运维
          </figcaption>
        </figure>

        <div class="brand-cta">
          <div class="cta-chip" aria-hidden="true">
            <CheckCircleFilledIcon size="14" /> Vue 3 · TDesign
          </div>
          <div class="cta-chip" aria-hidden="true">
            <ShieldErrorIcon size="14" /> RBAC + JWT 鉴权
          </div>
          <div class="cta-chip" aria-hidden="true">
            <CloudIcon size="14" /> 多云资源编排
          </div>
        </div>
      </section>

      <section class="brand-trust" aria-label="平台亮点">
        <article
          v-for="(item, idx) in featureList"
          :key="item.title"
          class="trust-card"
          :style="{ animationDelay: `${60 + idx * 50}ms` }"
        >
          <div class="trust-card__icon" :class="`trust-card__icon--${item.variant}`">
            <component :is="item.icon" size="18" />
          </div>
          <div class="trust-card__body">
            <h3 class="trust-card__title">{{ item.title }}</h3>
            <p class="trust-card__desc">{{ item.desc }}</p>
          </div>
        </article>
      </section>

      <footer class="brand-footer" aria-label="版权信息">
        <span>© 2026 hostsent · 宿派云控</span>
        <span aria-hidden="true">·</span>
        <span>推荐使用 Chrome / Edge 1920×1080</span>
      </footer>
    </aside>

    <main class="login-page__panel" aria-labelledby="login-heading">
      <AdminLoginForm />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, h, shallowRef } from 'vue'

import {
  ChartBubbleIcon,
  CheckCircleFilledIcon,
  CloudIcon,
  LayersIcon,
  ServiceIcon,
  ShieldErrorIcon,
} from 'tdesign-icons-vue-next'

import AdminLoginForm from './components/AdminLoginForm.vue'

defineOptions({ name: 'AdminLoginPage' })

const logoMark =
  'https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=' +
  encodeURIComponent(
    'minimalist logo mark letter H on green gradient background, modern flat vector, square, high contrast, clean lines'
  ) +
  '&image_size=square_hd'

const heroIllustration =
  'https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=' +
  encodeURIComponent(
    'clean light SaaS dashboard UI mockup with charts cards and stats, white background, green accent, modern flat illustration, professional cloud operations platform preview, 16:9'
  ) +
  '&image_size=landscape_16_9'

function wrap(iconComp: unknown) {
  return shallowRef({ render: () => h(iconComp as never) })
}

const featureList = computed(() => [
  {
    title: '安全可靠',
    desc: '多因素认证、操作审计、敏感操作二次验证，全方位保障系统与数据安全。',
    variant: 'security',
    icon: wrap(ShieldErrorIcon),
  },
  {
    title: '高效稳定',
    desc: '基于高可用架构与异步消息链路，99.95% SLA，系统稳定流畅运行。',
    variant: 'stable',
    icon: wrap(ChartBubbleIcon),
  },
  {
    title: '全面管理',
    desc: '覆盖销售、云资源、订单计费、工单、监控报警全流程闭环管理。',
    variant: 'manage',
    icon: wrap(LayersIcon),
  },
  {
    title: '贴心服务',
    desc: '完整的售后工单、邮件 / 短信 / 站内通知，随时响应并可追溯。',
    variant: 'service',
    icon: wrap(ServiceIcon),
  },
])
</script>

<style scoped lang="css">
.login-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  grid-template-columns: 1.05fr 0.95fr;
  background: var(--color-background);
  color: var(--color-foreground);
  position: relative;
  overflow: hidden;
  isolation: isolate;
}

/* Subtle dotted background for brand side */
.login-page::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(15, 23, 42, 0.04) 1px, transparent 1px);
  background-size: 18px 18px;
  pointer-events: none;
  z-index: 0;
}

/* Brand side */
.login-page__brand {
  padding: 28px 36px 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  z-index: 1;
  background: linear-gradient(180deg, #ffffff 0%, #f1f5f9 100%);
  border-right: 1px solid var(--color-border);
}

.brand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.brand-logo__mark {
  width: 36px;
  height: 36px;
  border-radius: var(--hs-radius-md);
  display: block;
  object-fit: cover;
  box-shadow: var(--hs-shadow-sm);
}

.brand-logo__text {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}

.brand-name {
  font-family: var(--hs-font-heading);
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: 0.02em;
  text-transform: lowercase;
}

.brand-tag {
  font-size: 12px;
  color: var(--color-muted-foreground);
  margin-top: 2px;
}

.brand-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--hs-radius-xl);
  font-size: 12px;
  color: #15803d;
  background: #ecfdf5;
  border: 1px solid #bbf7d0;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #16a34a;
  box-shadow: 0 0 0 0 rgba(22, 163, 74, 0.5);
  animation: statusPulse 2.4s ease-in-out infinite;
}

@keyframes statusPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(22, 163, 74, 0.5); }
  70% { box-shadow: 0 0 0 6px rgba(22, 163, 74, 0); }
}

.brand-hero {
  max-width: 640px;
}

.brand-eyebrow {
  font-family: var(--hs-font-mono);
  font-size: 11px;
  letter-spacing: 0.22em;
  text-transform: uppercase;
  color: #16a34a;
  margin: 0 0 10px;
}

.brand-title {
  font-family: var(--hs-font-heading);
  font-size: clamp(24px, 2.6vw, 34px);
  font-weight: 700;
  line-height: 1.2;
  margin: 0 0 12px;
  color: var(--color-foreground);
}

.brand-title--accent {
  color: #16a34a;
}

.brand-subtitle {
  margin: 0 0 18px;
  font-size: 14px;
  line-height: 1.7;
  max-width: 56ch;
  color: var(--color-muted-foreground);
}

.brand-illustration {
  margin: 0 0 16px;
  border-radius: var(--hs-radius-lg);
  overflow: hidden;
  background: var(--hs-surface-2);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-md);
  position: relative;
}

.brand-illustration__img {
  width: 100%;
  height: auto;
  display: block;
  aspect-ratio: 16 / 9;
  object-fit: cover;
}

.brand-illustration__caption {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--color-muted-foreground);
  background: var(--hs-surface-1);
  border-top: 1px solid var(--color-border);
  color: #15803d;
}

.brand-cta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cta-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  color: #334155;
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-xl);
}

.brand-trust {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: auto;
  max-width: 720px;
}

.trust-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border: 1px solid var(--color-border);
  box-shadow: var(--hs-shadow-xs);
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast),
    transform var(--hs-duration-fast);
  opacity: 0;
  transform: translateY(8px);
  animation: trustCardIn 360ms var(--hs-ease-out) forwards;
}

.trust-card:hover {
  border-color: #bbf7d0;
  box-shadow: var(--hs-shadow-sm);
  transform: translateY(-1px);
}

@keyframes trustCardIn {
  to { opacity: 1; transform: translateY(0); }
}

.trust-card__icon {
  width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  flex-shrink: 0;
}

.trust-card__icon--security { background: #16a34a; }
.trust-card__icon--stable { background: #2563eb; }
.trust-card__icon--manage { background: #6366f1; }
.trust-card__icon--service { background: #d97706; }

.trust-card__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.trust-card__title {
  margin: 0;
  font-family: var(--hs-font-heading);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.trust-card__desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--color-muted-foreground);
}

.brand-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 11px;
  color: #94a3b8;
}

/* Panel side */
.login-page__panel {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 32px 28px;
  position: relative;
  z-index: 1;
}

@media (max-width: 1080px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .login-page__brand {
    padding: 24px 20px 12px;
    border-right: 0;
    border-bottom: 1px solid var(--color-border);
  }

  .brand-trust {
    grid-template-columns: 1fr;
    margin-top: 16px;
  }

  .brand-illustration {
    display: none;
  }

  .login-page__panel {
    padding: 20px 20px 32px;
  }
}
</style>
