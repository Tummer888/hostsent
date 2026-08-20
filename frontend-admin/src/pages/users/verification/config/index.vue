<template>
  <div class="verification-config-page">
    <header class="surface-card page-header">
      <div>
        <h2 class="page-title">认证配置</h2>
        <p class="page-subtitle">预留实名认证配置入口，后续接入认证开关、审核规则和资料模板配置。</p>
      </div>
    </header>

    <section class="surface-card page-body">
      <t-empty description="认证配置功能待接入" />
    </section>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'UserVerificationConfig' })
</script>

<style scoped>
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
.page-subtitle {
  margin: 8px 0 0;
  color: var(--color-muted-foreground);
}
</style>
