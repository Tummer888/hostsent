<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <VerifyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">实例对账</h2>
          <p class="page-header__desc">已开通的上游实例：本地售价 / 上游成本价、本地到期 / 上游到期日期的比对异常检测</p>
        </div>
      </div>
    </header>

    <section class="table-card surface-card">
      <t-empty description="实例对账接口开发中（S4）" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { VerifyIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'ResourceReconcile' })
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
