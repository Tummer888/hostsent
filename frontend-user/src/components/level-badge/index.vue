<template>
  <span v-if="name" class="level-badge" :class="[`level-badge--${size}`, { 'level-badge--plain': plain }]" :style="badgeStyle">
    <component :is="icon" size="14" class="level-badge__icon" />
    <span class="level-badge__text">{{ name }}</span>
  </span>
  <span v-else-if="showEmpty" class="level-badge__empty">未分级</span>
</template>

<script setup lang="ts">
// 会员等级徽章（用户端）。
//
// 图标与配色来自运营在管理端「用户等级」页的配置；空值按权重回落。
// 与 frontend-admin 的同名组件共用一套兜底规则（见 ./icons.ts）。
import { computed } from 'vue'

import { defaultLevelIcon, resolveLevelColor, resolveLevelIcon } from './icons'

defineOptions({ name: 'LevelBadge' })

const props = withDefaults(
  defineProps<{
    name?: string
    icon?: string
    color?: string
    weight?: number
    /** small = 顶栏/列表内联；medium = 个人中心等处。 */
    size?: 'small' | 'medium'
    /** 只有图标与文字，不带底色胶囊。 */
    plain?: boolean
    /** 名称为空时是否显示「未分级」占位；默认隐藏（顶栏里显示占位反而难看）。 */
    showEmpty?: boolean
  }>(),
  { size: 'small', plain: false, showEmpty: false, weight: 0 },
)

const icon = computed(() => resolveLevelIcon(props.icon) || defaultLevelIcon)
const levelColor = computed(() => resolveLevelColor(props.color, props.weight))

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
  border: 1px solid transparent;
  border-radius: 999px;
  font-weight: 500;
  white-space: nowrap;
  vertical-align: middle;
}

.level-badge--small {
  padding: 0 6px;
  font-size: 12px;
  line-height: 18px;
}

.level-badge--medium {
  padding: 2px 10px;
  font-size: 13px;
  line-height: 22px;
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
  font-size: 12px;
}
</style>
