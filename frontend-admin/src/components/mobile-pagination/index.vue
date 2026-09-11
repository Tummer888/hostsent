<template>
  <div class="mobile-pagination">
    <div class="mobile-pagination__row">
      <t-button
        class="mobile-pagination__nav-btn"
        variant="outline"
        size="small"
        :disabled="!hasPrev"
        @click="emit('go', current - 1)"
      >
        <template #icon><ChevronLeftIcon aria-hidden="true" /></template>
        上一页
      </t-button>
      <span class="mobile-pagination__summary">{{ summary }}</span>
      <t-button
        class="mobile-pagination__nav-btn"
        variant="outline"
        size="small"
        :disabled="!hasNext"
        @click="emit('go', current + 1)"
      >
        下一页
        <template #icon><ChevronRightIcon aria-hidden="true" /></template>
      </t-button>
    </div>
    <div class="mobile-pagination__row mobile-pagination__row--secondary">
      <t-select
        class="mobile-pagination__size"
        size="small"
        :value="pageSize"
        :options="pageSizeOptions"
        @change="(v: unknown) => emit('page-size', Number(v))"
      />
      <div class="mobile-pagination__jump" :class="{ 'mobile-pagination__jump--invalid': invalid }">
        <span class="mobile-pagination__jump-label">跳至</span>
        <t-input
          v-model="jumpValue"
          class="mobile-pagination__jump-input"
          size="small"
          type="number"
          :placeholder="`1-${totalPages}`"
          @enter="confirmJump"
        />
        <t-button class="mobile-pagination__jump-btn" size="small" variant="outline" @click="confirmJump">
          跳转
        </t-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { ChevronLeftIcon, ChevronRightIcon } from 'tdesign-icons-vue-next'

const props = withDefaults(
  defineProps<{
    current: number
    pageSize: number
    total: number
    pageSizeOptions?: number[]
  }>(),
  {
    pageSizeOptions: () => [10, 20, 50, 100],
  },
)

const emit = defineEmits<{
  go: [page: number]
  'page-size': [pageSize: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const hasPrev = computed(() => props.current > 1)
const hasNext = computed(() => props.current < totalPages.value)
const summary = computed(() => `第 ${props.current} / ${totalPages.value} 页 · 共 ${props.total} 条`)

const pageSizeOptions = computed(() => props.pageSizeOptions.map((size) => ({ label: `${size} 条/页`, value: size })))

const jumpValue = ref<string | number>('')
const invalid = ref(false)

watch(
  () => props.current,
  () => {
    jumpValue.value = ''
    invalid.value = false
  },
)

function confirmJump() {
  const raw = jumpValue.value == null ? '' : String(jumpValue.value).trim()
  if (!raw) return
  const target = Number(raw)
  if (!Number.isFinite(target) || target < 1 || target > totalPages.value) {
    invalid.value = true
    MessagePlugin.warning(`请输入 1 ~ ${totalPages.value} 之间的页码`)
    return
  }
  invalid.value = false
  emit('go', Math.trunc(target))
  jumpValue.value = ''
}
</script>

<style scoped>
/* 移动端自定义分页：两行布局，避免内置分页在窄屏溢出裁切 */
.mobile-pagination {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 4px 2px;
}

.mobile-pagination__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.mobile-pagination__row--secondary {
  justify-content: flex-start;
  gap: 10px;
}

.mobile-pagination__nav-btn {
  flex: 0 0 auto;
  min-width: 84px;
  border-color: var(--td-brand-color-2);
  background: #ffffff;
  color: var(--td-brand-color-8);
}

.mobile-pagination__nav-btn:disabled {
  color: var(--td-text-color-disabled);
  background: var(--td-bg-color-component-disabled);
}

.mobile-pagination__summary {
  flex: 1 1 auto;
  text-align: center;
  font-size: 12px;
  color: var(--color-muted-foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mobile-pagination__size {
  flex: 0 0 118px;
}

.mobile-pagination__size :deep(.t-input),
.mobile-pagination__jump-input :deep(.t-input) {
  background: #ffffff;
  border-color: var(--td-brand-color-2);
  border-radius: var(--hs-radius-md);
}

.mobile-pagination__jump {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1 1 auto;
  min-width: 0;
}

.mobile-pagination__jump-label {
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.mobile-pagination__jump-input {
  flex: 1 1 0;
  min-width: 56px;
}

.mobile-pagination__jump-input :deep(.t-input) {
  width: 100%;
}

.mobile-pagination__jump-btn {
  flex: 0 0 auto;
  border-color: var(--td-brand-color-2);
  background: #ffffff;
  color: var(--td-brand-color-8);
}

.mobile-pagination__jump--invalid :deep(.t-input) {
  border-color: var(--td-error-color-6);
}

@media (max-width: 767px) {
  .mobile-pagination__jump-input :deep(.t-input__inner) {
    -moz-appearance: textfield;
  }

  .mobile-pagination__jump-input :deep(.t-input__inner::-webkit-outer-spin-button),
  .mobile-pagination__jump-input :deep(.t-input__inner::-webkit-inner-spin-button) {
    -webkit-appearance: none;
    margin: 0;
  }
}
</style>
