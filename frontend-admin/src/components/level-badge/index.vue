<template>
  <span v-if="name" class="level-badge" :class="{ 'level-badge--plain': variant === 'plain' }" :style="badgeStyle">
    <component :is="icon" v-if="icon" size="14" class="level-badge__icon" />
    <span class="level-badge__text">{{ name }}</span>
  </span>
  <span v-else class="level-badge__empty">—</span>
</template>

<script setup lang="ts">
// 等级徽章：图标 + 名称 + 等级主题色。
//
// 配色与图标都来自运营在「用户等级」页的配置（user_levels.icon / color），
// 空值按权重回落到默认色阶与星星图标。用户端（frontend-user）有一份同源实现，
// 两端的兜底规则必须一致，否则同一个等级在管理端和用户端会是两个颜色。
import { computed } from 'vue'

import { defaultLevelIcon, resolveLevelColor, resolveLevelIcon } from './icons'

defineOptions({ name: 'LevelBadge' })

const props = withDefaults(
  defineProps<{
    /** 等级名称；为空时渲染占位符「—」。 */
    name?: string
    /** 图标 key（user_levels.icon）。 */
    icon?: string
    /** 主题色（user_levels.color）。 */
    color?: string
    /** 权重，用于未配颜色时推导兜底色（必须由后端下发，否则兜底色会漂移）。 */
    weight?: number
    /** light = 带底色的胶囊（列表/详情）；plain = 仅图标与文字（紧凑处）。 */
    variant?: 'light' | 'plain'
  }>(),
  { variant: 'light', weight: 0 },
)

const icon = computed(() => resolveLevelIcon(props.icon) || defaultLevelIcon)
const levelColor = computed(() => resolveLevelColor(props.color, props.weight))

// 底色与边框由主题色派生，避免额外维护一套颜色变量：
// 8 位十六进制是带 alpha 的 #RRGGBBAA，浏览器原生支持。
const badgeStyle = computed(() => ({
  color: levelColor.value,
  backgroundColor: `${levelColor.value}1A`,
  borderColor: `${levelColor.value}33`,
}))
</script>

<style scoped>
.level-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border: 1px solid transparent;
  border-radius: 999px;
  font-size: 12px;
  line-height: 20px;
  font-weight: 500;
  white-space: nowrap;
}

.level-badge--plain {
  padding: 0;
  border-color: transparent;
  background-color: transparent;
}

.level-badge__icon {
  flex-shrink: 0;
}

.level-badge__empty {
  color: var(--td-text-color-placeholder, #bbb);
}
</style>
