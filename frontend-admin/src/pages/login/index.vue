<template>
  <div class="login-page">
    <div class="login-shell">
      <aside class="login-panel login-panel--brand" aria-label="平台介绍">
        <header class="brand-header">
          <div class="brand-logo" aria-label="Hostsent 宿派云控">
            <span class="brand-logo__mark" aria-hidden="true">H</span>
            <div class="brand-logo__text">
              <div class="brand-name">hostsent</div>
              <div class="brand-tag">宿派云控</div>
            </div>
          </div>
        </header>

        <img
          class="brand-illustration"
          :src="loginIllustration"
          alt="宿派云控产品插画"
        />
      </aside>

      <main class="login-panel login-panel--form" aria-labelledby="login-heading">
        <AdminLoginForm />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import AdminLoginForm from './components/AdminLoginForm.vue'
import loginIllustration from '@/assets/images/login-cnter.png'

defineOptions({ name: 'AdminLoginPage' })
</script>

<style scoped lang="css">
/* 外层：浅蓝灰背景（白底为主），中间悬浮白色大卡片 */
.login-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  background: linear-gradient(180deg, #f3f7fc 0%, #eaf1fa 100%);
  color: var(--color-foreground);
  position: relative;
  overflow: hidden;
  isolation: isolate;
}

/* 中间大卡片：左品牌区 + 右白表单区 */
.login-shell {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1120px;
  min-height: 620px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  border-radius: 24px;
  overflow: hidden;
  background: #ffffff;
  border: 1px solid #e7eef7;
  box-shadow: 0 16px 48px rgba(15, 23, 42, 0.1);
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
  background: #ffffff;
}

/* 左：白色品牌区（与表单区以竖向分界线隔开） */
.login-panel--brand {
  position: relative;
  padding: 36px 44px 32px;
  background: #ffffff;
  border-right: 1px solid #eef2f8;
  overflow: visible; /* 允许插画向右出血，盖住分界线 */
  z-index: 2; /* 让出血的插画绘制在表单区之上 */
}

.brand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4%;
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  color: #ffffff;
  background: linear-gradient(135deg, var(--color-primary) 0%, var(--td-brand-color-6) 100%);
  box-shadow: 0 4px 12px rgba(22, 163, 74, 0.28);
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
  color: #0f172a;
  letter-spacing: 0.02em;
  text-transform: lowercase;
}

.brand-tag {
  font-size: 12px;
  color: #64748b;
  margin-top: 2px;
}

/* 插画：绝对定位在品牌区右下，向右出血盖住分界线，形成高级的跨栏效果 */
.brand-illustration {
  position: absolute;
  right: -45px;
  bottom: 120px;
  width: 500px;
  max-width: 90%;
  z-index: 3;
  display: block;
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
  color: var(--color-primary);
}

:deep(.admin-login-card__title) {
  color: var(--color-foreground);
}

:deep(.admin-login-card__subtitle) {
  color: var(--color-muted-foreground);
}

:deep(.accent-word) {
  color: var(--color-primary);
}

:deep(.submit-btn) {
  background: var(--color-primary);
  border: 0;
  color: #ffffff;
  box-shadow: 0 4px 12px rgba(22, 163, 74, 0.22);
}

:deep(.submit-btn:hover) {
  background: var(--td-brand-color-8);
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

  /* 移动端隐藏左侧品牌区：不再堆叠到表单下方，只保留表单 */
  .login-panel--brand {
    display: none;
  }
}
</style>
