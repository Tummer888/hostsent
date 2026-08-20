<template>
  <SecurityListPage
    title="登录日志"
    subtitle="聚合管理员与用户登录轨迹，快速识别异常来源、失败爆发与高风险访问。"
    table-title="登录事件"
    table-desc="支持按账号、IP、登录结果和风险等级进行筛查。"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无登录日志"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #filters>
      <div class="filter-grid">
        <t-input v-model="filters.username" clearable placeholder="用户名" />
        <t-input v-model="filters.ip" clearable placeholder="IP 地址" />
        <t-select v-model="filters.result" clearable :options="resultOptions" placeholder="登录结果" />
        <t-select v-model="filters.login_type" clearable :options="loginTypeOptions" placeholder="登录类型" />
        <t-select v-model="filters.risk_flag" clearable :options="riskOptions" placeholder="风险标记" />
      </div>
    </template>

    <template #result="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.result] || 'default'" variant="light-outline">
        {{ row.result === 'success' ? '成功' : '失败' }}
      </t-tag>
    </template>

    <template #risk_flag="{ row }">
      <t-tag :theme="securityRiskTagTheme[row.risk_flag] || 'default'" variant="light-outline">
        {{ riskLabelMap[row.risk_flag] || row.risk_flag || '—' }}
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

import { getLoginLogList, type LoginLogInfo, type LoginLogListQuery } from '@/api/security'

import SecurityListPage from '../SecurityListPage.vue'
import { formatSecurityTime, securityRiskTagTheme, securityStatusTagTheme } from '../shared'

defineOptions({ name: 'UserSecurityLoginLogs' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<LoginLogInfo[]>([])

const filters = reactive<LoginLogListQuery>({
  page: 1,
  page_size: 10,
  username: '',
  ip: '',
  result: '',
  login_type: '',
  risk_flag: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const resultOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
]

const loginTypeOptions = [
  { label: '后台', value: 'admin' },
  { label: '前台', value: 'user' },
]

const riskOptions = [
  { label: '低风险', value: 'low' },
  { label: '中风险', value: 'medium' },
  { label: '高风险', value: 'high' },
  { label: '严重风险', value: 'critical' },
]

const riskLabelMap: Record<string, string> = {
  low: '低风险',
  medium: '中风险',
  high: '高风险',
  critical: '严重风险',
}

const columns: PrimaryTableCol<LoginLogInfo>[] = [
  { colKey: 'username', title: '用户名', minWidth: 140 },
  { colKey: 'login_type', title: '类型', width: 100 },
  { colKey: 'result', title: '结果', width: 100 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'ip_region', title: '归属地', minWidth: 120 },
  { colKey: 'platform', title: '平台', width: 100 },
  { colKey: 'risk_flag', title: '风险', width: 110 },
  { colKey: 'failure_reason', title: '失败原因', minWidth: 150, ellipsis: true },
  { colKey: 'created_at', title: '登录时间', width: 180 },
]

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getLoginLogList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载登录日志失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.username = ''
  filters.ip = ''
  filters.result = ''
  filters.login_type = ''
  filters.risk_flag = ''
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
  grid-template-columns: repeat(5, minmax(0, 1fr));
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
