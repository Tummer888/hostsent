<template>
  <SecurityListPage
    title="登录日志"
    table-title="登录事件"
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
    :icon="HistoryIcon"
  >
    <template #header-actions>
      <t-button variant="outline" :loading="exporting" @click="handleExportCSV">
        <template #icon><DownloadIcon /></template>
        导出 CSV
      </t-button>
    </template>

    <template #filters>
      <div class="filter-grid">
        <div class="field">
          <span class="field__label">用户名</span>
          <t-input v-model="filters.username" clearable placeholder="用户名" />
        </div>
        <div class="field">
          <span class="field__label">IP 地址</span>
          <t-input v-model="filters.ip" clearable placeholder="IP 地址" />
        </div>
        <div class="field">
          <span class="field__label">登录结果</span>
          <t-select v-model="filters.result" clearable :options="resultOptions" placeholder="登录结果" />
        </div>
        <div class="field">
          <span class="field__label">登录类型</span>
          <t-select v-model="filters.login_type" clearable :options="LOGIN_TYPE_OPTIONS" placeholder="登录类型" />
        </div>
        <div class="field">
          <span class="field__label">风险标记</span>
          <t-select v-model="filters.risk_flag" clearable :options="RISK_FLAG_OPTIONS" placeholder="风险标记" />
        </div>
        <div class="field">
          <span class="field__label">登录时间</span>
          <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleDateChange" />
        </div>
      </div>
    </template>

    <template #result="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.result] || 'default'" variant="light-outline">
        {{ row.result === 'success' ? '成功' : '失败' }}
      </t-tag>
    </template>

    <template #login_type="{ row }">
      {{ LOGIN_TYPE_LABEL[row.login_type] || row.login_type || '—' }}
    </template>

    <template #platform="{ row }">
      {{ LOGIN_PLATFORM_LABEL[row.platform] || row.platform || '—' }}
    </template>

    <template #risk_flag="{ row }">
      <t-tag :theme="securityRiskTagTheme[row.risk_flag] || 'default'" variant="light-outline">
        {{ RISK_FLAG_LABEL[row.risk_flag] || row.risk_flag || '—' }}
      </t-tag>
    </template>

    <template #created_at="{ row }">
      {{ formatSecurityTime(row.created_at) }}
    </template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { DownloadIcon, HistoryIcon } from 'tdesign-icons-vue-next'
import { onMounted, reactive, ref } from 'vue'

import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { exportLoginLogs, getLoginLogList, type LoginLogInfo, type LoginLogListQuery } from '@/api/security'

import SecurityListPage from '../SecurityListPage.vue'
import {
  LOGIN_PLATFORM_LABEL,
  LOGIN_TYPE_LABEL,
  LOGIN_TYPE_OPTIONS,
  RISK_FLAG_LABEL,
  RISK_FLAG_OPTIONS,
  applyDateRange,
  formatSecurityTime,
  securityRiskTagTheme,
  securityStatusTagTheme,
} from '../shared'

defineOptions({ name: 'UserSecurityLoginLogs' })

const loading = ref(false)
const exporting = ref(false)
const errorMessage = ref('')
const tableData = ref<LoginLogInfo[]>([])
const dateRange = ref<string[]>([])

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

const columns: PrimaryTableCol<LoginLogInfo>[] = [
  { colKey: 'username', title: '用户名', minWidth: 140 },
  { colKey: 'login_type', title: '类型', width: 110 },
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

function handleDateChange(value: unknown) {
  applyDateRange(filters, value)
}

/** 导出当前筛选结果为 CSV：GET blob，手动触发浏览器下载（与审计日志导出口径一致）。 */
async function handleExportCSV() {
  exporting.value = true
  try {
    const response = await exportLoginLogs({ ...filters })
    const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
    const objectUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = objectUrl
    link.download = `login-logs-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(objectUrl)
    MessagePlugin.success('导出成功')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '导出失败')
  } finally {
    exporting.value = false
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
  filters.start_time = undefined
  filters.end_time = undefined
  dateRange.value = []
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1200px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
