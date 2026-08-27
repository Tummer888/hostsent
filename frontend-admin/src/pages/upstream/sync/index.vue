<template>
  <div class="upstream-page">
    <header class="page-header surface-card">
      <div>
        <h2 class="page-title">同步任务</h2>
        <p class="page-subtitle">管理上游商品 / 实例同步任务状态与日志。</p>
      </div>
    </header>
    <section class="placeholder surface-card">
      <div class="placeholder__inner">
        <RefreshIcon size="48" class="placeholder__icon" />
        <p class="placeholder__title">模块建设中</p>
        <p class="placeholder__desc">资源管理第一阶段仅落地骨架，同步任务编排与日志将于后续阶段实现。</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { RefreshIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'UpstreamSync' })
</script>

<style scoped>
.upstream-page { display: flex; flex-direction: column; gap: 16px; }
.page-header, .placeholder { padding: 16px 20px; }
.page-title { margin: 0; font-size: 22px; }
.page-subtitle { margin: 8px 0 0; color: var(--color-muted-foreground); }
.placeholder { display: flex; align-items: center; justify-content: center; min-height: 320px; }
.placeholder__inner { text-align: center; }
.placeholder__icon { color: var(--color-muted-foreground); }
.placeholder__title { margin: 16px 0 0; font-size: 18px; font-weight: 600; }
.placeholder__desc { margin: 8px 0 0; color: var(--color-muted-foreground); }
</style>
