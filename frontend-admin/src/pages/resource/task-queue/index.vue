<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <RefreshIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">任务队列</h2>
          <p class="page-header__desc">平台动作任务（暂停 / 续费 / 重启等）是否通过接口成功到达上游</p>
        </div>
      </div>
    </header>

    <section class="table-card surface-card">
      <t-empty description="任务队列聚合接口开发中（S3）" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { RefreshIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'ResourceTaskQueue' })
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

.table-card {
  padding: var(--space-lg) 24px;
}
</style>
