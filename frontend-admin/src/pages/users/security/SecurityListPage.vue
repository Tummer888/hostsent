<template>
  <div class="security-page">
    <header class="security-page__header surface-card">
      <div>
        <h2 class="security-page__title">{{ title }}</h2>
        <p class="security-page__subtitle">{{ subtitle }}</p>
      </div>
      <div class="security-page__actions">
        <slot name="header-actions" />
      </div>
    </header>

    <section class="security-page__toolbar surface-card">
      <div class="security-page__toolbar-head">
        <div>
          <h3 class="security-page__section-title">筛选条件</h3>
          <p class="security-page__section-desc">按主体对象、状态、风险和时间范围快速检索安全事件与处置结果。</p>
        </div>
        <t-space>
          <t-button theme="primary" @click="$emit('search')">查询</t-button>
          <t-button variant="outline" @click="$emit('reset')">重置</t-button>
        </t-space>
      </div>
      <slot name="filters" />
    </section>

    <section class="security-page__table surface-card">
      <div class="security-page__table-head">
        <div>
          <h3 class="security-page__section-title">{{ tableTitle }}</h3>
          <p class="security-page__section-desc">{{ tableDesc }}</p>
        </div>
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
        bordered
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

<script setup lang="ts" generic="TItem">
import type { PageInfo, PaginationProps, PrimaryTableCol } from 'tdesign-vue-next'

defineProps<{
  title: string
  subtitle: string
  tableTitle: string
  tableDesc: string
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
  border-radius: var(--hs-radius-lg);
}

.security-page__header,
.security-page__toolbar,
.security-page__table {
  padding: 18px 20px;
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

.security-page__subtitle,
.security-page__section-desc,
.security-page__table-meta {
  margin: 6px 0 0;
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
