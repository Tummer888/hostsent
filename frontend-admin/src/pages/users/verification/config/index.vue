<template>
  <div class="verification-config-page">
    <header class="surface-card page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <h2 class="page-title">认证配置</h2>
      </div>
    </header>

    <section class="surface-card page-body">
      <t-empty description="认证配置功能待接入" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { SettingIcon } from 'tdesign-icons-vue-next'
defineOptions({ name: 'UserVerificationConfig' })
</script>

<style scoped>
.page-header__main {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-header__chip {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--td-brand-color-6), var(--color-primary));
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.verification-config-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-header,
.page-body {
  padding: 18px 20px;
}
.page-title {
  margin: 0;
  font-size: 22px;
}
</style>
