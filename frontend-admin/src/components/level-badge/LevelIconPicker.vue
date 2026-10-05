<template>
  <t-popup v-model:visible="visible" trigger="click" placement="bottom-left" :overlay-inner-style="{ padding: '12px' }">
    <div class="icon-picker__trigger" :class="{ 'is-empty': !modelValue }">
      <component :is="currentIcon" size="16" class="icon-picker__current" />
      <span class="icon-picker__label">{{ currentLabel }}</span>
      <ChevronDownIcon size="14" class="icon-picker__arrow" />
    </div>
    <template #content>
      <div class="icon-picker__panel">
        <button
          v-for="item in levelIconOptions"
          :key="item.value"
          type="button"
          class="icon-picker__cell"
          :class="{ 'is-active': item.value === modelValue }"
          :title="item.label"
          @click="pick(item.value)"
        >
          <component :is="item.component" size="20" />
        </button>
      </div>
      <div class="icon-picker__foot">
        <t-link theme="default" size="small" @click="pick('')">使用默认图标（按权重）</t-link>
      </div>
    </template>
  </t-popup>
</template>

<script setup lang="ts">
// 图标选择器：网格面板 + 当前选中预览。
//
// 不用 t-select 的原因：这个版本的 t-select 没有 option 插槽，选项只能渲染文字，
// 而挑图标必须看得见图标本身。t-popup + 网格按钮是这里唯一能给出可视化选择的做法。
import { computed, ref } from 'vue'
import { ChevronDownIcon } from 'tdesign-icons-vue-next'

import { defaultLevelIcon, levelIconOptions, resolveLevelIcon } from './icons'

defineOptions({ name: 'LevelIconPicker' })

const props = defineProps<{ modelValue?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const visible = ref(false)

const currentIcon = computed(() => resolveLevelIcon(props.modelValue) || defaultLevelIcon)
const currentLabel = computed(() => {
  if (!props.modelValue) return '默认图标'
  const found = levelIconOptions.find((item) => item.value === props.modelValue)
  // 目录外的 key（历史数据/手工填过）照样显示出来，否则运营会以为配置丢了。
  return found ? found.label : props.modelValue
})

function pick(value: string) {
  emit('update:modelValue', value)
  visible.value = false
}
</script>

<style scoped>
.icon-picker__trigger {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 180px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--td-component-border, #dcdcdc);
  border-radius: var(--td-radius-default, 6px);
  background: var(--td-bg-color-container, #fff);
  cursor: pointer;
  transition: border-color 0.2s;
}

.icon-picker__trigger:hover {
  border-color: var(--td-brand-color, #0052d9);
}

.icon-picker__current {
  color: var(--td-text-color-primary, #333);
  flex-shrink: 0;
}

.icon-picker__label {
  flex: 1;
  font-size: 13px;
  color: var(--td-text-color-primary, #333);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.icon-picker__trigger.is-empty .icon-picker__label {
  color: var(--td-text-color-placeholder, #bbb);
}

.icon-picker__arrow {
  color: var(--td-text-color-placeholder, #bbb);
  flex-shrink: 0;
}

.icon-picker__panel {
  display: grid;
  grid-template-columns: repeat(7, 36px);
  gap: 4px;
}

.icon-picker__cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 1px solid transparent;
  border-radius: var(--td-radius-default, 6px);
  background: transparent;
  color: var(--td-text-color-secondary, #666);
  cursor: pointer;
  transition: all 0.15s;
}

.icon-picker__cell:hover {
  background: var(--td-bg-color-container-hover, #f3f3f3);
  color: var(--td-brand-color, #0052d9);
}

.icon-picker__cell.is-active {
  border-color: var(--td-brand-color, #0052d9);
  background: var(--td-brand-color-light, #f2f3ff);
  color: var(--td-brand-color, #0052d9);
}

.icon-picker__foot {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
  text-align: center;
}
</style>
