<template>
  <div class="login-page">
    <div class="login-shell">
      <aside class="login-panel login-panel--brand" aria-label="平台介绍">
        <header class="brand-header">
          <div class="brand-logo" aria-label="Hostsent 宿派云控">
            <img
              :src="logoMark"
              alt="Hostsent 标识"
              class="brand-logo__mark"
              width="38"
              height="38"
            />
            <div class="brand-logo__text">
              <div class="brand-name">hostsent</div>
              <div class="brand-tag">宿派云控</div>
            </div>
          </div>
        </header>

        <section class="brand-hero">
          <h1 class="brand-title">
            掌控你的<span class="brand-title--accent">云基础设施</span>
          </h1>
          <p class="brand-subtitle">
            为 IDC / 云服务商打造的高可用管理平台，集成云主机、用户、工单、计费、
            监控、通知等核心能力。
          </p>
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

      <main class="login-panel login-panel--form" aria-labelledby="login-heading">
        <AdminLoginForm />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

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

const featureList = computed(() => [
  {
    title: '安全可靠',
    desc: '多因素认证、操作审计、敏感操作二次验证，全方位保障系统与数据安全。',
    variant: 'security',
    icon: ShieldErrorIcon,
  },
  {
    title: '高效稳定',
    desc: '基于高可用架构与异步消息链路，99.95% SLA，系统稳定流畅运行。',
    variant: 'stable',
    icon: ChartBubbleIcon,
  },
  {
    title: '全面管理',
    desc: '覆盖销售、云资源、订单计费、工单、监控报警全流程闭环管理。',
    variant: 'manage',
    icon: LayersIcon,
  },
  {
    title: '贴心服务',
    desc: '完整的售后工单、邮件 / 短信 / 站内通知，随时响应并可追溯。',
    variant: 'service',
    icon: ServiceIcon,
  },
])
</script>

<style scoped lang="css">
/* 外层：深蓝渐变背景，中间悬浮大卡片 */
.login-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  background: #3b82f6;
  color: var(--color-foreground);
  position: relative;
  overflow: hidden;
  isolation: isolate;
}

/* 中间大卡片：左蓝品牌区 + 右白表单区 */
.login-shell {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1120px;
  min-height: 620px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  border-radius: 28px;
  overflow: hidden;
  background: #ffffff;
  box-shadow: 0 24px 60px rgba(2, 6, 23, 0.45);
}

.login-panel {
  position: relative;
  display: flex;
  flex-direction: column;
}

/* 右：白色表单区 */
.login-panel--form {
  align-items: center;
  justify-content: center;
  padding: 48px 52px 40px;
  background: linear-gradient(180deg, #ffffff 0%, #f5f8fb 100%);
}

/* 左：蓝色品牌区，与右侧白区直接平分 */
.login-panel--brand {
  position: relative;
  padding: 36px 44px 24px;
  background:
    radial-gradient(560px 420px at 90% -10%, rgba(147, 197, 253, 0.28), transparent 62%),
    linear-gradient(160deg, #1d4ed8 0%, #2563eb 60%, #3b82f6 100%);
  overflow: hidden;
}

.brand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12% 0 4%;
}

.brand-logo {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.brand-logo__mark {
  width: 38px;
  height: 38px;
  border-radius: var(--hs-radius-md);
  display: block;
  object-fit: cover;
  box-shadow: 0 4px 12px rgba(2, 6, 23, 0.28);
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
  color: #ffffff;
  letter-spacing: 0.02em;
  text-transform: lowercase;
}

.brand-tag {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.7);
  margin-top: 2px;
}

.brand-hero {
  max-width: 640px;
  padding-left: 6%;
  margin-top: 34px;
}

.brand-title {
  font-family: var(--hs-font-heading);
  font-size: clamp(24px, 2.6vw, 32px);
  font-weight: 700;
  line-height: 1.2;
  margin: 0 0 12px;
  color: #ffffff;
}

.brand-title--accent {
  color: #a7f3d0;
}

.brand-subtitle {
  margin: 0 0 18px;
  font-size: 14px;
  line-height: 1.7;
  max-width: 48ch;
  color: rgba(255, 255, 255, 0.84);
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
  color: rgba(255, 255, 255, 0.92);
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.28);
  border-radius: var(--hs-radius-xl);
  backdrop-filter: blur(4px);
}

.brand-trust {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: auto;
  padding-left: 6%;
  max-width: 720px;
  align-self: stretch;
}

.trust-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border-radius: var(--hs-radius-lg);
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.2);
  backdrop-filter: blur(4px);
  transition:
    border-color var(--hs-duration-fast),
    background-color var(--hs-duration-fast),
    transform var(--hs-duration-fast);
  opacity: 0;
  transform: translateY(8px);
  animation: trustCardIn 360ms var(--hs-ease-out) forwards;
}

.trust-card:hover {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.4);
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
  color: #ffffff;
}

.trust-card__desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: rgba(255, 255, 255, 0.78);
}

.brand-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  padding-left: 6%;
  margin-top: 16px;
}

/* 表单组件融入左侧白色面板 */
:deep(.admin-login-card) {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 400px;
  padding: 0;
  background: transparent;
  border: 0;
  box-shadow: none;
}

:deep(.admin-login-card__eyebrow) {
  color: #16a34a;
}

:deep(.admin-login-card__title) {
  color: var(--color-foreground);
}

:deep(.admin-login-card__subtitle) {
  color: var(--color-muted-foreground);
}

:deep(.accent-word) {
  color: #16a34a;
}

:deep(.submit-btn) {
  background: #16a34a;
  border: 0;
  color: #ffffff;
  box-shadow: 0 4px 12px rgba(22, 163, 74, 0.22);
}

:deep(.submit-btn:hover) {
  background: #15803d;
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(22, 163, 74, 0.3);
}

:deep(.submit-btn:active) {
  transform: translateY(0);
}

@media (max-width: 1080px) {
  .login-page {
    padding: 24px 16px;
    align-items: stretch;
  }

  .login-shell {
    grid-template-columns: 1fr;
    max-width: 560px;
    min-height: auto;
    border-radius: 20px;
    overflow: visible;
  }

  .login-panel--form {
    padding: 32px 24px 28px;
    order: 1;
  }

  .login-panel--brand {
    padding: 28px 24px 16px;
    order: 2;
  }

  .brand-header,
  .brand-hero,
  .brand-trust,
  .brand-footer {
    padding-left: 0;
  }

  .brand-hero {
    margin-top: 18px;
  }

  .brand-trust {
    grid-template-columns: 1fr;
    margin-top: 16px;
  }
}
</style>