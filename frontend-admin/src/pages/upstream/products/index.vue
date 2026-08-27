<template>
  <div class="upstream-page">
    <header class="page-header surface-card">
      <div>
        <h2 class="page-title">上游商品</h2>
        <p class="page-subtitle">管理上游商品规格映射与定价，用于统一实例交付。</p>
      </div>
    </header>
    <section class="placeholder surface-card">
      <div class="placeholder__inner">
        <AppIcon size="48" class="placeholder__icon" />
        <p class="placeholder__title">模块建设中</p>
        <p class="placeholder__desc">资源管理第一阶段仅落地骨架，商品同步与定价管理将于后续阶段实现。</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { AppIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'UpstreamProducts' })
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
