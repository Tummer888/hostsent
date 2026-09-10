<template>
  <div class="verification-config-page">
    <header class="surface-card page-header">
      <h2 class="page-title">认证配置</h2>
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
</style>
