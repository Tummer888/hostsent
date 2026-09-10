<template>
  <div class="sec-head">
    <div class="sec-head__text">
      <h2 class="sec-head__title">{{ title }}</h2>
      <p v-if="subtitle" class="sec-head__subtitle">{{ subtitle }}</p>
    </div>

    <NuxtLink v-if="actionLabel && actionTo" class="site-pill" :to="actionTo">
      {{ actionLabel }}
      <SiteIcon name="arrow-right" :stroke-width="1.8" class="sec-head__arrow" />
    </NuxtLink>
  </div>
</template>

<script setup lang="ts">
/**
 * 区块标题栏：左侧标题（+副标题），右侧「查看全部」胶囊按钮。
 * 参考图里多数区块都是这个骨架，抽成组件避免各区块重复一遍样式。
 */
defineProps<{
  title: string
  subtitle?: string
  actionLabel?: string
  actionTo?: string
}>()
</script>

<style scoped>
.sec-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  flex-wrap: wrap;
}

.sec-head__text {
  min-width: 0;
}

.sec-head__title {
  font-size: 27px;
  font-weight: 700;
  letter-spacing: 0.01em;
}

.sec-head__subtitle {
  margin-top: 10px;
  max-width: 640px;
  font-size: 14.5px;
  line-height: 1.75;
  color: var(--site-text-muted);
}

.sec-head__arrow {
  width: 15px;
  height: 15px;
}

@media (max-width: 768px) {
  .sec-head__title {
    font-size: 21px;
  }
}
</style>
