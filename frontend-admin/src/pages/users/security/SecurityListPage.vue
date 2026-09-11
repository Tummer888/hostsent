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
      </div>
      <slot name="filters" />
      <div class="security-page__toolbar-actions">
        <t-space>
          <t-button theme="success" @click="$emit('search')">查询</t-button>
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
  tableTitle: string
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
    justify-content: stretch;
  }

  .security-page__toolbar-actions .t-button {
    flex: 1 1 0;
  }
}
</style>
