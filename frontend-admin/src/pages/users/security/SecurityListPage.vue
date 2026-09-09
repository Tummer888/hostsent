<template>
  <div class="security-page">
    <header class="security-page__header surface-card">
      <div class="security-page__heading">
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
        <t-space>
          <t-button theme="success" @click="$emit('search')">查询</t-button>
          <t-button variant="outline" @click="$emit('reset')">重置</t-button>
        </t-space>
      </div>
      <slot name="filters" />
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
        :pagination="pagination"
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
    </section>
  </div>
</template>

<script setup lang="ts" generic="TItem extends import('tdesign-vue-next').TableRowData">
import type { PageInfo, PaginationProps, PrimaryTableCol } from 'tdesign-vue-next'

defineProps<{
  title: string
  subtitle?: string
  tableTitle: string
  tableDesc?: string
  total: number
  data: TItem[]
  columns: PrimaryTableCol<TItem>[]
  loading: boolean
  errorMessage: string
  emptyText: string
  pagination: PaginationProps
}>()

defineEmits<{
  search: []
  reset: []
  reload: []
  'page-change': [pageInfo: PageInfo]
}>()
</script>

<style scoped lang="css">
.security-page {
  --td-brand-color-1: #f0fdf4;
  --td-brand-color-2: #dcfce7;
  --td-brand-color-3: #bbf7d0;
  --td-brand-color-6: #22c55e;
  --td-brand-color-7: #16a34a;
  --td-brand-color-8: #15803d;
  --td-brand-color: #16a34a;
  --td-brand-color-hover: #15803d;
  --td-brand-color-focus: rgba(22, 163, 74, 0.14);
  --td-brand-color-active: #166534;
  --td-brand-color-disabled: #86efac;
  --td-brand-color-light: #f0fdf4;
  --td-brand-color-light-hover: #dcfce7;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.security-page__header,
.security-page__toolbar,
.security-page__table {
  padding: 18px 20px;
  border: 1px solid #e2ebe6;
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
}
</style>
