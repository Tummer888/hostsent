<template>
  <div class="error-page">
    <t-empty>
      <template #title>404</template>
      <template #description>抱歉，您访问的页面不存在</template>
      <template #footer>
        <t-button theme="primary" @click="$router.push('/')">返回首页</t-button>
      </template>
    </t-empty>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'NotFound' })
</script>

<style scoped>
.error-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}
</style>
