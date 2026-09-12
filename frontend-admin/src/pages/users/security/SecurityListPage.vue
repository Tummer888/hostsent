<template>
  <div class="security-page">
    <header class="security-page__header surface-card">
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
      <slot name="filters" />
      <div class="security-page__toolbar-actions">
        <t-space>
          <t-button theme="primary" @click="$emit('search')">查询</t-button>
          <t-button variant="outline" @click="$emit('reset')">重置</t-button>
        </t-space>
      </div>
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
  border-radius: 12px;
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

.security-page__toolbar-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
}

.security-page :deep(.t-table__th) {
  background: rgba(0, 168, 112, 0.07);
  color: #176b50;
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

  .security-page__toolbar-actions {
    justify-content: flex-end;
  }
}
</style>
