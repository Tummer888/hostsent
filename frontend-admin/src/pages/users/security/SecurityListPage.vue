<template>
  <div class="security-page">
    <!-- noHeader：多列表合并进同一页时由外层渲染统一页头（实名认证审核三合一，doc 无编号，用户反馈） -->
    <header v-if="!noHeader" class="security-page__header surface-card">
      <div class="security-page__heading">
        <span v-if="!$slots['header-leading']" class="security-page__chip">
          <component :is="icon ?? AppIcon" size="22" aria-hidden="true" />
        </span>
        <slot name="header-leading" />
        <h2 class="security-page__title">{{ title }}</h2>
      </div>
      <div class="security-page__actions">
        <slot name="header-actions" />
      </div>
    </header>

    <section class="security-page__toolbar surface-card">
      <div class="security-page__toolbar-head">
        <h3 class="security-page__section-title">筛选条件</h3>
      </div>
      <!-- 统一筛选卡（embedded：外壳由上方 surface-card 提供），窄屏自动折叠只留主筛选字段 -->
      <FilterCard embedded :primary-count="1">
        <slot name="filters" />
        <template #actions>
          <t-space>
            <t-button theme="primary" @click="$emit('search')">查询</t-button>
            <t-button variant="outline" @click="$emit('reset')">重置</t-button>
          </t-space>
        </template>
      </FilterCard>
    </section>

    <section class="security-page__table surface-card">
      <div class="security-page__table-head">
        <h3 class="security-page__section-title">{{ tableTitle }}</h3>
        <div class="security-page__table-meta">共 {{ total }} 条</div>
      </div>

      <div v-if="errorMessage" class="security-page__error">
        <span>{{ errorMessage }}</span>
        <t-link theme="primary" hover="color" @click="$emit('reload')">重试</t-link>
      </div>

      <t-table
        row-key="id"
        :data="data"
        :columns="columns"
        :loading="loading"
        :pagination="isMobile ? undefined : pagination"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        @page-change="$emit('page-change', $event)"
      >
        <template v-for="(_, slotName) in $slots" #[slotName]="scope">
          <slot :name="slotName" v-bind="scope || {}" />
        </template>
        <template #empty>
          <t-empty :description="emptyText" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="pagination.current ?? 1"
        :page-size="pagination.pageSize ?? 10"
        :total="total"
        @go="(p: number) => $emit('page-change', { current: p, previous: pagination.current ?? 1, pageSize: pagination.pageSize ?? 10 })"
        @page-size="(s: number) => $emit('page-change', { current: 1, previous: pagination.current ?? 1, pageSize: s })"
      />
    </section>
  </div>
</template>

<script setup lang="ts" generic="TItem extends import('tdesign-vue-next').TableRowData">
import type { Component } from 'vue'
import type { PageInfo, PaginationProps, PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon } from 'tdesign-icons-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import FilterCard from '@/components/filter-card/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineProps<{
  title: string
  tableTitle: string
  total: number
  data: TItem[]
  columns: PrimaryTableCol<TItem>[]
  loading: boolean
  errorMessage: string
  emptyText: string
  pagination: PaginationProps
  /** 页头标题前的图标（不传则用通用图标） */
  icon?: Component
  /** 不渲染本组件的页头卡（外层已有统一页头时使用） */
  noHeader?: boolean
}>()

defineEmits<{
  search: []
  reset: []
  reload: []
  'page-change': [pageInfo: PageInfo]
}>()

const { isMobile } = useIsMobile()
</script>

<style scoped lang="css">
.security-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.security-page__header,
.security-page__toolbar,
.security-page__table {
  padding: 18px 20px;
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  border-color: var(--td-brand-color-2);
  box-shadow: none;
}

.security-page__header,
.security-page__toolbar-head,
.security-page__table-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.security-page__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.security-page__table-meta {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.security-page__section-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

/* 页头标题图标：主题色渐变底、无投影（与用户列表页基准一致） */
.security-page__chip {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, var(--td-brand-color-6), var(--color-primary));
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.security-page__heading {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 字段来自各页 #filters 插槽，直接是 FilterCard 栅格（.filter-card__grid）的子元素 */
.security-page__toolbar :deep(:where(.filter-card__grid > .field)) {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

/* 筛选控件与用户列表页（unified-control）同款：浅灰控件底、统一描边与品牌色聚焦环。
   放在外壳组件里一次覆盖四个安全页与三个实名认证列表页（t-select / 日期区间
   内部都是 .t-input，无需各页再加类名）。 */
.security-page__toolbar :deep(.t-input),
.security-page__toolbar :deep(.t-input__wrap),
.security-page__toolbar :deep(.t-input-adornment),
.security-page__toolbar :deep(.t-input__suffix),
.security-page__toolbar :deep(.t-select__wrap) {
  background: var(--hs-surface-2);
}

.security-page__toolbar :deep(.t-input),
.security-page__toolbar :deep(.t-select__wrap) {
  border-color: var(--color-border);
  border-radius: var(--hs-radius-md);
}

.security-page__toolbar :deep(.t-input:focus-within),
.security-page__toolbar :deep(.t-select__wrap:focus-within) {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(var(--color-primary-rgb), 0.1);
}

.security-page__toolbar :deep(.filter-card__grid > .field .field__label) {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.security-page :deep(.t-table__th) {
  /* 表头需不透明：固定列是 sticky 单元格，半透明底会在横向滚动时透视其它列标题 */
  background-color: var(--hs-surface-1);
  background-image: linear-gradient(rgba(var(--color-primary-rgb), 0.07), rgba(var(--color-primary-rgb), 0.07));
  color: var(--td-brand-color-8);
  font-weight: 600;
}

.security-page :deep(.t-table__td) {
  border-color: rgba(15, 23, 42, 0.06);
}

.security-page__error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  padding: 10px 12px;
  border: 1px solid rgba(239, 68, 68, 0.18);
  border-radius: var(--hs-radius-md);
  background: rgba(239, 68, 68, 0.06);
  color: var(--color-destructive);
}

@media (max-width: 768px) {
  .security-page__header,
  .security-page__toolbar-head,
  .security-page__table-head {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
