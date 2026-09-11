<template>
  <t-dropdown
    trigger="click"
    :options="options"
    @click="(value: string | number | Record<string, any>) => emit('select', value)"
  >
    <t-button
      theme="default"
      variant="outline"
      size="small"
      shape="square"
      class="mobile-action-button"
      aria-label="操作"
    >
      <template #icon><MoreIcon aria-hidden="true" /></template>
    </t-button>
  </t-dropdown>
</template>

<script setup lang="ts">
import { MoreIcon } from 'tdesign-icons-vue-next'

// 移动端操作列：省略号按钮 + 下拉菜单。
// options: [{ content, value, disabled?, theme? }]，与 TDesign DropdownOption 一致
defineProps<{
  options: Array<{ content: string; value: string | number; disabled?: boolean; theme?: 'default' | 'success' | 'warning' | 'error' }>
}>()

const emit = defineEmits<{
  select: [value: string | number | Record<string, any>]
}>()
</script>

<style>
/* 非 scoped：按钮位于表格固定列内，需覆盖 TDesign 图标压缩问题 */
.mobile-action-button {
  width: 32px;
  height: 32px;
  min-width: 32px;
  padding: 0;
  color: #475569;
  background: #ffffff;
  border-color: transparent;
}

/* TDesign 原生 MoreIcon 是描边式三竖点（1em svg），在固定列 flex 压缩下会被
   压成 2px 宽导致"空白/只显示一个点"，这里锁定尺寸并禁止 flex 收缩。 */
.mobile-action-button .t-icon {
  width: 16px;
  height: 16px;
  min-width: 16px;
  max-width: none;
  flex: 0 0 auto;
  overflow: visible;
}

@media (max-width: 767px) {
  .mobile-action-button {
    width: 30px;
    height: 30px;
    min-width: 30px;
  }
}
</style>
