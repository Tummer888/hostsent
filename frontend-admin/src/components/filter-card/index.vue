<template>
  <component :is="embedded ? 'div' : 'section'" :class="rootClass" data-filter-card>
    <!-- 标题行之上的内容（工单页的视图切换标签就落在这里）。 -->
    <slot name="pre" />

    <div v-if="showHead" class="filter-card__head">
      <h3 v-if="title" class="card-title">{{ title }}</h3>
      <span v-if="meta" class="filter-card__meta">{{ meta }}</span>
      <slot name="head-extra" />
    </div>

    <!--
      标题与字段之间、不占栅格位的整行内容（任务队列的「任务类别」单选条）。
      必须放在栅格之外：放进去就会被当成一个字段，窄屏折叠时要么抢走唯一的保留位、
      要么被 :nth-child 隐藏掉。
    -->
    <slot name="above-fields" />

    <!--
      字段容器。刻意不让各页自己写这一层：折叠规则要挂在 .filter-card__grid 的
      直接子元素上，容器收在组件里，规则才有一处可查的归属。
    -->
    <div
      :id="gridId"
      ref="gridRef"
      class="filter-card__grid"
      :class="gridClass"
      @input="measure"
      @change="measure"
    >
      <slot />
    </div>

    <!--
      移动端折叠开关。只在「窄屏 + 字段数超过保留数」时出现 ——
      字段本来就只有一两个的页面不该多出一个点了没反应的按钮。
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

    <div v-if="hasActions" class="filter-card__actions">
      <slot name="actions" />
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { ChevronDownIcon } from 'tdesign-icons-vue-next'

import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 列表页统一筛选卡。
 *
 * 解决三件事：
 *
 * 1. **窄屏筛选区太长**。管理端多数列表页有 3–9 个筛选框，单列铺下来要划一屏才
 *    看得见表格。这里在窄屏只保留第一个（主）筛选框，其余收进「更多筛选」。
 *    保留第一个而不是全部收起：最常用的一定是关键词/状态那一个，把它也收起来，
 *    等于每次筛选都要多点一下。
 *
 * 2. **结构漂移**。此前 50 多个页面各自手写 `head + grid + actions` 三段，
 *    标题文案、按钮顺序、窄屏行为各写各的。收进组件后一处改、处处生效。
 *
 * 3. **列数各写各的**。资源/产品等模块还各自写了「桌面 4 列 → 中屏 2 列 →
 *    窄屏 1 列」的媒体查询，且写在页面的 scoped 样式里。栅格搬进组件后那些
 *    选择器不再命中（组件元素身上没有页面的 scope 属性），因此列数改为
 *    `columns` 属性一处声明。
 *
 * 宽屏行为与引入本组件之前一致：字段照常平铺，只是外层多了一个语义等价的
 * 容器。折叠只由 CSS 在窄屏媒体查询里触发（见文件末尾），首屏不闪。
 */

const props = withDefaults(
  defineProps<{
    /** 卡片标题；默认不渲染标题行（「筛选条件」这种同质标题对用户没有信息量）。 */
    title?: string
    /** 标题行右侧的补充说明（统计口径、容差参数等）。 */
    meta?: string
    /** 窄屏保留几个字段不折叠。 */
    primaryCount?: number
    /** 关掉折叠（如整行宽的日期区间，收起主控反而碍事）。 */
    collapsible?: boolean
    /** 追加到筛选栅格上的类（如 filter-card__grid--inline）。 */
    gridClass?: string
    /**
     * 宽屏固定列数；0 表示按字段宽度自适应换列。
     * 中屏（≤1200px）与窄屏（≤768px）的降列由组件统一定义。
     */
    columns?: number
    /**
     * 嵌在别的卡片里（表格卡内的筛选行），不画自己的卡片外壳与内边距。
     */
    embedded?: boolean
  }>(),
  {
    title: '',
    meta: '',
    primaryCount: 1,
    collapsible: true,
    gridClass: '',
    columns: 0,
    embedded: false,
  },
)

const slots = defineSlots<{
  default?: () => unknown
  actions?: () => unknown
  /** 标题行之上的内容（如工单页的视图切换标签）。 */
  pre?: () => unknown
  /** 标题与字段之间的整行内容（不参与栅格）。 */
  'above-fields'?: () => unknown
  'head-extra'?: () => unknown
}>()

const { isMobile } = useIsMobile()

const gridRef = ref<HTMLElement | null>(null)
const fieldCount = ref(0)
const hiddenActiveCount = ref(0)
const expanded = ref(false)
// aria-controls 要一个稳定 id；useId 在多次挂载间不撞号，且 SSR/CSR 一致。
const gridId = useId()

const showHead = computed(() => Boolean(props.title || props.meta || slots['head-extra']))
const hasActions = computed(() => Boolean(slots.actions))
const hiddenCount = computed(() => Math.max(0, fieldCount.value - props.primaryCount))
const toggleVisible = computed(
  () => props.collapsible && isMobile.value && hiddenCount.value > 0,
)

const rootClass = computed(() => [
  props.embedded ? 'filter-embed' : 'filter-card surface-card',
  {
    'filter-card--open': expanded.value,
    [`filter-card--cols-${props.columns}`]: props.columns >= 2 && props.columns <= 4,
  },
])

/**
 * 数折叠区里有多少个已填值的条件。
 *
 * 读渲染后的 DOM 而不是业务数据：筛选条件是各页自己的响应式对象，形状
 * （字符串 / 数字 / 区间数组 / 布尔）五花八门，组件拿不到也不需要知道。
 *
 * 判定依据（按 TDesign 控件的实际表现）：
 * - 输入框与下拉：值都镜像在 `input.t-input__inner` 的 value 上，包括
 *   t-select（选中后显示的是选项文案）与 t-date-range-picker（显示区间）。
 * - 复选框 / 单选按钮：input.checked。
 * - 开关与单选按钮组：状态不落在 input 上，改判 `.t-is-checked`。
 *
 * 注意「有没有值」不能写成「值真不真」：`provider.status = 0`（禁用）是合法
 * 筛选值，用真值判断会把它当成没设置。
 *
 * 漏判的后果只是角标少一个数字，不影响筛选本身，因此这个启发式够用。
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
    if (!filled && el.querySelector('.t-is-checked')) filled = true
    if (filled) active += 1
  }
  hiddenActiveCount.value = active
}

// 字段数量会随页面数据变化（如「分类」下拉只在 news/help 两种内容形态下出现），
// 用 MutationObserver 跟随，否则按钮上的数量会停在首次渲染的结果。
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

// 视口在宽窄之间切换时重算（resize 由 useIsMobile 跟踪，这里只跟随它的结果）。
watch(isMobile, () => {
  void nextTick(measure)
})

watch(
  () => props.primaryCount,
  () => measure(),
)
</script>

<style>
/* 非 scoped：字段与操作区来自各页插槽，选择器需要命中插槽内容。 */

/* ---------- 自足的基础栅格 ---------- */
.filter-card__grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

/* 固定列数：桌面按声明，中屏一律 2 列，窄屏 1 列。
   此前这段媒体查询在 7 个页面各写了一遍，且阈值（1200/769）也是各写各的。 */
.filter-card--cols-2 .filter-card__grid,
.filter-card--cols-3 .filter-card__grid,
.filter-card--cols-4 .filter-card__grid {
  grid-template-columns: repeat(var(--filter-card-cols), minmax(0, 1fr));
}

.filter-card--cols-2 {
  --filter-card-cols: 2;
}

.filter-card--cols-3 {
  --filter-card-cols: 3;
}

.filter-card--cols-4 {
  --filter-card-cols: 4;
}

/* 跨列字段（任务队列的创建时间区间）。 */
.filter-card__grid > .field--wide {
  grid-column: span 2;
}

/* 与字段同排的操作格（渠道配置等把按钮放进栅格的页面）。 */
.filter-card__grid > .field--actions {
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
}

/* ---------- 嵌在表格卡里的筛选行 ---------- */
/* 不画外壳：外层已有 surface-card，再套一层会多出一圈内边距、与表头对不齐。 */
.filter-embed .filter-card__head {
  margin-bottom: var(--space-md, 12px);
}

.filter-embed .filter-card__actions {
  margin-top: var(--space-md, 12px);
  padding-top: 0;
  border-top: 0;
}

/* ---------- 折叠开关（仅窄屏出现） ---------- */
.filter-card__toggle {
  display: none;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  min-height: 40px;
  margin-top: 12px;
  padding: 0 12px;
  border: 1px dashed var(--color-border, #e2e8f0);
  border-radius: var(--hs-radius-md, 6px);
  background: var(--hs-surface-2, #f8fafc);
  color: var(--color-muted-foreground, #64748b);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition:
    color var(--hs-duration-fast, 150ms),
    border-color var(--hs-duration-fast, 150ms),
    background var(--hs-duration-fast, 150ms);
}

.filter-card__toggle:hover {
  color: var(--td-brand-color, #16a34a);
  border-color: var(--td-brand-color-4, #86efac);
  background: var(--td-brand-color-1, #f0fdf4);
}

.filter-card__toggle:focus-visible {
  outline: 2px solid var(--td-brand-color-4, #86efac);
  outline-offset: 1px;
}

.filter-card__toggle-icon {
  flex: none;
  transition: transform var(--hs-duration-base, 200ms) var(--hs-ease-out, ease);
}

.filter-card--open .filter-card__toggle-icon {
  transform: rotate(180deg);
}

/* 折叠区里已有生效条件时的提示角标 */
.filter-card__toggle-badge {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--td-brand-color-1, #f0fdf4);
  color: var(--td-brand-color-7, #16a34a);
  font-size: 11.5px;
  font-weight: 600;
}

/* ---------- 中屏降列 ---------- */
@media (max-width: 1200px) and (min-width: 769px) {
  .filter-card--cols-3,
  .filter-card--cols-4 {
    --filter-card-cols: 2;
  }

  .filter-card--cols-2 .filter-card__grid > .field--wide {
    grid-column: span 2;
  }
}

/* ---------- 窄屏：只留主筛选框，其余折叠 ---------- */
@media (max-width: 768px) {
  .filter-card__toggle {
    display: inline-flex;
  }

  .filter-card__grid {
    gap: 12px;
    grid-template-columns: 1fr;
  }

  /* 固定列数在窄屏一律回落单列，跨列字段随之归位。 */
  .filter-card--cols-2 .filter-card__grid,
  .filter-card--cols-3 .filter-card__grid,
  .filter-card--cols-4 .filter-card__grid {
    grid-template-columns: 1fr;
  }

  .filter-card__grid > .field--wide,
  .filter-card__grid > .field--actions {
    grid-column: auto;
  }

  .filter-card__grid > .field--actions {
    justify-content: flex-start;
  }

  /* 主筛选框之外的条件默认隐藏；点「更多筛选」后整片展开。
     用 :nth-child 而不是逐个加类：字段是各页插槽内容，
     组件不该反过来要求每个页面配合打标记。
     :is() 让卡片版与嵌入式版共用同一条折叠规则，免得两处各写一遍后走偏。 */
  :is(.filter-card, .filter-embed):not(.filter-card--open) .filter-card__grid > *:nth-child(n + 2) {
    display: none;
  }

  /* 触控目标不低于 40px：默认 32px 的控件在手机上容易点错。 */
  :is(.filter-card, .filter-embed) .t-input,
  :is(.filter-card, .filter-embed) .t-select,
  :is(.filter-card, .filter-embed) .t-input-number,
  :is(.filter-card, .filter-embed) .t-date-picker,
  :is(.filter-card, .filter-embed) .t-date-range-picker {
    min-height: 40px;
  }

  :is(.filter-card, .filter-embed) .field__label {
    font-size: 12.5px;
  }
}
</style>
