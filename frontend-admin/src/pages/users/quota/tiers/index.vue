<template>
  <div class="quota-page">
    <header class="page-header surface-card"><div><h2 class="page-title">用户等级管理</h2><p class="page-subtitle">管理等级权重、绑定默认模板和实例 / CPU / 内存 / 磁盘上限。</p></div></header>
    <section class="toolbar surface-card"><t-space wrap><t-input v-model="filters.keyword" clearable placeholder="搜索等级名称 / 编码" @enter="handleSearch" /><t-select v-model="filters.status" clearable placeholder="状态" :options="statusOptions" /><t-button theme="primary" @click="handleSearch">查询</t-button><t-button variant="outline" @click="handleReset">重置</t-button></t-space></section>
    <section class="table-panel surface-card"><t-table row-key="id" :data="tableData" :columns="columns" :loading="loading" :pagination="pagination" cell-empty-content="—" @page-change="handlePageChange"><template #name="{ row }"><div class="primary-cell"><strong>{{ row.name }}</strong><span>{{ row.code }}</span></div></template><template #default_template_name="{ row }">{{ row.default_template_name || '—' }}</template><template #resource="{ row }"><div class="primary-cell"><span>实例 {{ row.max_instance_count }}</span><span>CPU {{ row.max_cpu_cores }} / 内存 {{ row.max_memory_gb }}GB / 磁盘 {{ row.max_disk_gb }}GB</span></div></template><template #status="{ row }"><t-tag theme="primary" variant="light">{{ row.status }}</t-tag></template></t-table></section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'
import { getUserLevelList, type UserLevelInfo, type UserLevelListQuery } from '@/api/user'

defineOptions({ name: 'UserQuotaTiers' })
const loading = ref(false)
const tableData = ref<UserLevelInfo[]>([])
const filters = reactive<UserLevelListQuery>({ page: 1, page_size: 10, keyword: '', status: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50, 100] })
const statusOptions = [{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }]
const columns: PrimaryTableCol<UserLevelInfo>[] = [
  { colKey: 'name', title: '等级信息', minWidth: 220 },
  { colKey: 'weight', title: '权重', width: 100 },
  { colKey: 'default_template_name', title: '默认模板', width: 160 },
  { colKey: 'resource', title: '资源限制', minWidth: 260 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'description', title: '说明', minWidth: 220 },
]
async function loadData() { loading.value = true; try { const data = await getUserLevelList(filters); tableData.value = data.items || []; pagination.current = data.meta.page; pagination.pageSize = data.meta.page_size; pagination.total = data.meta.total } catch (error) { MessagePlugin.error((error as Error)?.message || '加载用户等级失败') } finally { loading.value = false } }
function handleSearch() { filters.page = 1; pagination.current = 1; void loadData() }
function handleReset() { Object.assign(filters, { page: 1, page_size: 10, keyword: '', status: '' }); pagination.current = 1; pagination.pageSize = 10; void loadData() }
function handlePageChange(pageInfo: PageInfo) { filters.page = pageInfo.current; filters.page_size = pageInfo.pageSize; void loadData() }
onMounted(() => { void loadData() })
</script>

<style scoped>
.quota-page { display: flex; flex-direction: column; gap: 16px; }
.page-header,.toolbar,.table-panel { padding: 16px 20px; }
.page-title { margin: 0; font-size: 22px; }
.page-subtitle { margin: 8px 0 0; color: var(--color-muted-foreground); }
.primary-cell { display: flex; flex-direction: column; gap: 4px; }
</style>
