<template>
  <section class="filter-card surface-card" :class="{ 'filter-card--open': expanded }">
    <!-- 标题行之上的内容（留白给后续需要标签切换的页面）。 -->
    <slot name="pre" />

    <div v-if="showHead" class="filter-card__head">
      <h3 v-if="title" class="card-title">{{ title }}</h3>
      <span v-if="meta" class="filter-card__meta">{{ meta }}</span>
    </div>

    <div
      :id="gridId"
      ref="gridRef"
      class="filter-card__grid"
      @input="measure"
      @change="measure"
    >
      <slot />
    </div>

    <!--
      移动端折叠开关：窄屏只保留第一个筛选框，其余收进「更多筛选」。
      只在「窄屏 + 字段多于保留数」时出现 —— 只有一个筛选框的页面
      （如我的订单）不该多出一个点了没反应的按钮。

      与管理端 filter-card 的行为一致，选择器同样挂在 .filter-card__grid 上：
      两端各有一份实现，但折叠阈值、保留数、角标语义都对齐，改一处要对着改另一处。
    -->
    <button
      v-if="toggleVisible"
      type="button"
      class="filter-card__toggle"
      :aria-expanded="expanded ? 'true' : 'false'"
      :aria-controls="gridId"
      @click="expanded = !expanded"
    >
      <ChevronDownIcon class="filter-card__toggle-icon" size="16" aria-hidden="true" />
      <span>{{ expanded ? '收起筛选' : `更多筛选（${hiddenCount}）` }}</span>
      <!--
        折叠区里已有生效条件时必须说出来：否则用户在宽屏设好的条件，窄屏下看不见
        却仍在生效，排查时只会认为「结果不对」，而不是想到去展开筛选区。
      -->
      <span v-if="!expanded && hiddenActiveCount > 0" class="filter-card__toggle-badge">
        {{ hiddenActiveCount }} 项已生效
      </span>
    </button>

    <slot name="note" />

    <div v-if="hasActions" class="filter-card__actions">
      <slot name="actions" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { ChevronDownIcon } from 'tdesign-icons-vue-next'

import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 用户控制台统一筛选卡。
 *
 * 与管理端同名组件是同一套约定：窄屏只留第一个（主）筛选框，其余折叠到
 * 「更多筛选」；宽屏行为与改造前一致（字段照常平铺）。保留第一个而不是全部
 * 收起，是因为最常用的一定是第一个（状态/类型/关键词），把它一起收起来
 * 等于每次筛选都要多点一下。
 *
 * 控制台页面的筛选框都不多（1–2 个），但折叠仍然统一做：移动端上少滚一屏，
 * 表格就能早一屏进入视野。
 */

const props = withDefaults(
  defineProps<{
    /** 卡片标题；默认不渲染标题行。 */
    title?: string
    /** 标题行右侧的补充说明。 */
    meta?: string
    /** 窄屏保留几个字段不折叠。 */
    primaryCount?: number
    /** 关掉折叠。 */
    collapsible?: boolean
  }>(),
  { title: '', meta: '', primaryCount: 1, collapsible: true },
)

const slots = defineSlots<{
  default?: () => unknown
  actions?: () => unknown
  pre?: () => unknown
  note?: () => unknown
}>()

const { isMobile } = useIsMobile()

const gridRef = ref<HTMLElement | null>(null)
const fieldCount = ref(0)
const hiddenActiveCount = ref(0)
const expanded = ref(false)
const gridId = useId()

const showHead = computed(() => Boolean(props.title || props.meta))
const hasActions = computed(() => Boolean(slots.actions))
const hiddenCount = computed(() => Math.max(0, fieldCount.value - props.primaryCount))
const toggleVisible = computed(
  () => props.collapsible && isMobile.value && hiddenCount.value > 0,
)

/**
 * 数折叠区里有多少个已填值的条件。
 *
 * 判定读渲染后的 DOM：筛选条件是各页自己的响应式对象，形状（字符串 / 数字 /
 * 布尔）各不相同，组件拿不到也不需要知道。漏判的后果只是角标少一个数字，
 * 不影响筛选本身。
 */
function measure() {
  const grid = gridRef.value
  if (!grid) return
  fieldCount.value = grid.children.length

  if (!isMobile.value) {
    hiddenActiveCount.value = 0
    return
  }

  let active = 0
  for (let i = props.primaryCount; i < grid.children.length; i += 1) {
    const el = grid.children[i] as HTMLElement | undefined
    if (!el) continue
    let filled = false
    for (const input of Array.from(el.querySelectorAll<HTMLInputElement>('input'))) {
      if (input.type === 'checkbox' || input.type === 'radio') {
        if (input.checked) {
          filled = true
          break
        }
      } else if (input.value.trim() !== '') {
        filled = true
        break
      }
    }
    // 开关与单选按钮组的状态不落在 input 上，改判 TDesign 的选中类。
    if (!filled && el.querySelector('.t-is-checked')) filled = true
    if (filled) active += 1
  }
  hiddenActiveCount.value = active
}

let observer: MutationObserver | null = null

onMounted(async () => {
  await nextTick()
  measure()
  if (gridRef.value && typeof MutationObserver !== 'undefined') {
    observer = new MutationObserver(() => measure())
    observer.observe(gridRef.value, { childList: true, subtree: true })
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})

watch(isMobile, () => {
  void nextTick(measure)
})
</script>

<style scoped>
/* ---------- 折叠开关（仅窄屏出现） ---------- */
.filter-card__toggle {
  display: none;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 40px;
  margin-top: var(--space-md);
  padding: 0 12px;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md, 8px);
  background: var(--color-muted, #f8fafc);
  color: var(--color-muted-foreground);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: color 150ms ease, border-color 150ms ease, background 150ms ease;
}

.filter-card__toggle:hover {
  color: var(--td-brand-color);
  border-color: var(--td-brand-color-4);
  background: var(--td-brand-color-1);
}

.filter-card__toggle:focus-visible {
  outline: 2px solid var(--td-brand-color-4);
  outline-offset: 1px;
}

.filter-card__toggle-icon {
  flex: none;
  transition: transform 200ms cubic-bezier(0.22, 1, 0.36, 1);
}

.filter-card--open .filter-card__toggle-icon {
  transform: rotate(180deg);
}

.filter-card__toggle-badge {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-7);
  font-size: 11.5px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .filter-card__toggle {
    display: inline-flex;
  }

  /* 主筛选框之外的条件默认隐藏；点「更多筛选」后整片展开。
     用 :nth-child 而不是逐个加类：字段来自各页插槽，组件不该反过来
     要求每个页面配合打标记。 */
  .filter-card:not(.filter-card--open) :deep(.filter-card__grid) > *:nth-child(n + 2) {
    display: none;
  }
}
</style>
