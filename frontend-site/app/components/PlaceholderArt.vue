<template>
  <div class="ph-art" :class="[`ph-art--${variant}`]" :style="{ height: height ? `${height}px` : undefined }">
    <span class="ph-art__glow" aria-hidden="true" />
    <span v-if="icon" class="ph-art__icon" aria-hidden="true">
      <SiteIcon :name="icon" :stroke-width="1.5" />
    </span>
    <span v-if="label" class="ph-art__label">{{ label }}</span>
  </div>
</template>

<script setup lang="ts">
/**
 * 配图占位块。
 *
 * 官网当前没有对象存储与上传能力，产品封面/活动图/案例图都还没有真实素材，
 * 因此统一用「品牌色渐变 + 图标 + 文案」的占位块顶上：
 *  - 换主题色时占位块跟着变，不会出现与品牌色冲突的死图；
 *  - 后台可配 cover_image 的位子在拿到真图后直接替换组件即可。
 */
const props = withDefaults(
  defineProps<{
    /** 1–5，对应 main.css 中的 --site-art-N 渐变 */
    variant?: number
    icon?: string
    label?: string
    height?: number
  }>(),
  { variant: 1 },
)

const variant = computed(() => {
  const index = Math.min(Math.max(Math.round(props.variant), 1), 5)
  return index
})
</script>

<style scoped>
.ph-art {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 132px;
  padding: 20px;
  border: 1px solid rgba(var(--site-primary-rgb), 0.14);
  border-radius: var(--site-radius-lg);
  overflow: hidden;
  /* 渐变取的是主色低透明度，底色偏浅，文字必须用深色主色才够对比度 */
  color: var(--site-primary-strong);
  text-align: center;
}

.ph-art--1 {
  background: var(--site-art-1);
}

.ph-art--2 {
  background: var(--site-art-2);
}

.ph-art--3 {
  background: var(--site-art-3);
}

.ph-art--4 {
  background: var(--site-art-4);
}

.ph-art--5 {
  background: var(--site-art-5);
}

/* 高光斑：让纯渐变不至于太平 */
.ph-art__glow {
  position: absolute;
  width: 180px;
  height: 180px;
  top: -70px;
  right: -50px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.6);
  filter: blur(26px);
}

.ph-art__icon {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.ph-art__icon :deep(.site-icon) {
  width: 30px;
  height: 30px;
}

.ph-art__label {
  position: relative;
  font-size: 13.5px;
  font-weight: 600;
  letter-spacing: 0.01em;
}
</style>
