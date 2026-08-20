<template>
  <SecurityListPage
    title="操作审计日志"
    subtitle="记录后台关键操作与接口调用，便于事后追溯与合规审计。"
    table-title="审计记录"
    table-desc="按操作人、模块、动作和结果定位关键变更。"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无审计日志"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #filters>
      <div class="filter-grid">
        <t-input v-model="filters.operator" clearable placeholder="操作人" />
        <t-input v-model="filters.module" clearable placeholder="模块" />
        <t-select v-model="filters.action" clearable :options="actionOptions" placeholder="动作" />
        <t-select v-model="filters.result" clearable :options="resultOptions" placeholder="结果" />
        <t-input v-model="filters.resource_type" clearable placeholder="资源类型" />
        <t-input v-model="filters.resource_id" clearable placeholder="资源 ID" />
      </div>
    </template>

    <template #response_code="{ row }">
      <t-tag :theme="row.response_code < 400 ? 'success' : 'danger'" variant="light-outline">
        {{ row.response_code }}
      </t-tag>
    </template>

    <template #created_at="{ row }">
      {{ formatSecurityTime(row.created_at) }}
    </template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import { getAuditLogList, type AuditLogInfo, type AuditLogListQuery } from '@/api/security'

import SecurityListPage from '../SecurityListPage.vue'
import { formatSecurityTime, securityStatusTagTheme } from '../shared'

defineOptions({ name: 'UserSecurityAuditLogs' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<AuditLogInfo[]>([])

const filters = reactive<AuditLogListQuery>({
  page: 1,
  page_size: 10,
  operator: '',
  module: '',
  action: '',
  result: '',
  resource_type: '',
  resource_id: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const actionOptions = [
  { label: '创建', value: 'create' },
  { label: '更新', value: 'update' },
  { label: '删除', value: 'delete' },
  { label: '登录', value: 'login' },
]

const resultOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

const columns: PrimaryTableCol<AuditLogInfo>[] = [
  { colKey: 'operator_name', title: '操作人', width: 120 },
  { colKey: 'module', title: '模块', width: 120 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'resource_type', title: '资源类型', width: 140 },
  { colKey: 'resource_id', title: '资源 ID', width: 110 },
  { colKey: 'request_method', title: '方法', width: 90 },
  { colKey: 'request_path', title: '路径', minWidth: 180, ellipsis: true },
  { colKey: 'response_code', title: '状态码', width: 100 },
  { colKey: 'trace_id', title: 'Trace ID', minWidth: 160, ellipsis: true },
  { colKey: 'created_at', title: '发生时间', width: 180 },
]

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getAuditLogList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载审计日志失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.operator = ''
  filters.module = ''
  filters.action = ''
  filters.result = ''
  filters.resource_type = ''
  filters.resource_id = ''
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

onMounted(() => {
  void loadData()
})
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1200px) {
  .filter-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
