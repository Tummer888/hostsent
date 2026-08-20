<template>
  <div class="quota-page">
    <header class="page-header surface-card"><div><h2 class="page-title">配额调整记录</h2><p class="page-subtitle">展示配额变更前后值、变更来源、操作人和原因，支撑审计追踪。</p></div></header>
    <section class="toolbar surface-card"><t-space wrap><t-input v-model="filters.username" clearable placeholder="用户名" @enter="handleSearch" /><t-input v-model="filters.operator_name" clearable placeholder="操作人" @enter="handleSearch" /><t-select v-model="filters.source" clearable placeholder="来源" :options="sourceOptions" /><t-button theme="primary" @click="handleSearch">查询</t-button><t-button variant="outline" @click="handleReset">重置</t-button></t-space></section>
    <section class="table-panel surface-card"><t-table row-key="id" :data="tableData" :columns="columns" :loading="loading" :pagination="pagination" cell-empty-content="—" @page-change="handlePageChange"><template #quota="{ row }"><div class="primary-cell"><strong>{{ row.username }}</strong><span>{{ row.quota_name }} / {{ row.quota_code }}</span></div></template><template #change="{ row }">{{ formatNumber(row.before_value) }} → {{ formatNumber(row.after_value) }} ({{ formatNumber(row.delta_value) }})</template><template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template></t-table></section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { getQuotaAdjustmentList, type QuotaAdjustmentInfo, type QuotaAdjustmentListQuery } from '@/api/user'

defineOptions({ name: 'UserQuotaChanges' })
const loading = ref(false)
const tableData = ref<QuotaAdjustmentInfo[]>([])
const filters = reactive<QuotaAdjustmentListQuery>({ page: 1, page_size: 10, username: '', operator_name: '', source: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50, 100] })
const sourceOptions = [{ label: '手工', value: 'manual' }, { label: '系统', value: 'system' }, { label: '模板', value: 'template' }, { label: '等级', value: 'level' }]
const columns: PrimaryTableCol<QuotaAdjustmentInfo>[] = [
  { colKey: 'quota', title: '用户 / 配额', minWidth: 240 },
  { colKey: 'change', title: '变更值', minWidth: 180 },
  { colKey: 'adjustment_type', title: '类型', width: 120 },
  { colKey: 'source', title: '来源', width: 120 },
  { colKey: 'operator_name', title: '操作人', width: 120 },
  { colKey: 'reason', title: '原因', minWidth: 200 },
  { colKey: 'created_at', title: '调整时间', width: 180 },
]
async function loadData() { loading.value = true; try { const data = await getQuotaAdjustmentList(filters); tableData.value = data.items || []; pagination.current = data.meta.page; pagination.pageSize = data.meta.page_size; pagination.total = data.meta.total } catch (error) { MessagePlugin.error((error as Error)?.message || '加载调整记录失败') } finally { loading.value = false } }
function handleSearch() { filters.page = 1; pagination.current = 1; void loadData() }
function handleReset() { Object.assign(filters, { page: 1, page_size: 10, username: '', operator_name: '', source: '' }); pagination.current = 1; pagination.pageSize = 10; void loadData() }
function handlePageChange(pageInfo: PageInfo) { filters.page = pageInfo.current; filters.page_size = pageInfo.pageSize; void loadData() }
function formatNumber(value: number) { return Number(value || 0).toFixed(2) }
function formatDateTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false }) }
onMounted(() => { void loadData() })
</script>

<style scoped>
.quota-page { display: flex; flex-direction: column; gap: 16px; }
.page-header,.toolbar,.table-panel { padding: 16px 20px; }
.page-title { margin: 0; font-size: 22px; }
.page-subtitle { margin: 8px 0 0; color: var(--color-muted-foreground); }
.primary-cell { display: flex; flex-direction: column; gap: 4px; }
</style>
