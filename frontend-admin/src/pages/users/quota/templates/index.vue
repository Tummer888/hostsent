<template>
  <div class="quota-page">
    <header class="page-header surface-card"><div><h2 class="page-title">配额模板管理</h2><p class="page-subtitle">维护模板名称、范围、状态和模板明细，用于等级绑定和用户初始化。</p></div></header>
    <section class="toolbar surface-card"><t-space wrap><t-input v-model="filters.keyword" clearable placeholder="搜索模板名称 / 编码" @enter="handleSearch" /><t-select v-model="filters.status" clearable placeholder="状态" :options="statusOptions" /><t-select v-model="filters.scope" clearable placeholder="范围" :options="scopeOptions" /><t-button theme="primary" @click="handleSearch">查询</t-button><t-button variant="outline" @click="handleReset">重置</t-button></t-space></section>
    <section class="table-panel surface-card"><t-table row-key="id" :data="tableData" :columns="columns" :loading="loading" :pagination="pagination" cell-empty-content="—" @page-change="handlePageChange"><template #name="{ row }"><div class="primary-cell"><strong>{{ row.name }}</strong><span>{{ row.code }}</span></div></template><template #status="{ row }"><t-tag theme="success" variant="light">{{ row.status }}</t-tag></template><template #updated_at="{ row }">{{ formatDateTime(row.updated_at) }}</template></t-table></section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { type PageInfo, type PrimaryTableCol, MessagePlugin } from 'tdesign-vue-next'
import { getQuotaTemplateList, type QuotaTemplateInfo, type QuotaTemplateListQuery } from '@/api/user'

defineOptions({ name: 'UserQuotaTemplates' })
const loading = ref(false)
const tableData = ref<QuotaTemplateInfo[]>([])
const filters = reactive<QuotaTemplateListQuery>({ page: 1, page_size: 10, keyword: '', status: '', scope: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50, 100] })
const statusOptions = [{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }]
const scopeOptions = [{ label: '默认', value: 'default' }, { label: '行业', value: 'industry' }, { label: '渠道', value: 'channel' }]
const columns: PrimaryTableCol<QuotaTemplateInfo>[] = [
  { colKey: 'name', title: '模板信息', minWidth: 240 },
  { colKey: 'scope', title: '范围', width: 120 },
  { colKey: 'version', title: '版本', width: 100 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'description', title: '说明', minWidth: 240 },
  { colKey: 'updated_at', title: '更新时间', width: 180 },
]
async function loadData() { loading.value = true; try { const data = await getQuotaTemplateList(filters); tableData.value = data.items || []; pagination.current = data.meta.page; pagination.pageSize = data.meta.page_size; pagination.total = data.meta.total } catch (error) { MessagePlugin.error((error as Error)?.message || '加载模板失败') } finally { loading.value = false } }
function handleSearch() { filters.page = 1; pagination.current = 1; void loadData() }
function handleReset() { Object.assign(filters, { page: 1, page_size: 10, keyword: '', status: '', scope: '' }); pagination.current = 1; pagination.pageSize = 10; void loadData() }
function handlePageChange(pageInfo: PageInfo) { filters.page = pageInfo.current; filters.page_size = pageInfo.pageSize; void loadData() }
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
