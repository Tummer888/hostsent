<template>
  <div class="placeholder-page">
    <t-empty>
      <template #title>我的工单</template>
      <template #description>功能开发中，敬请期待</template>
    </t-empty>
  </div>
</template>

<script setup lang="ts">
defineOptions({ name: 'TicketList' })
</script>

<style scoped>
.placeholder-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}
</style>
