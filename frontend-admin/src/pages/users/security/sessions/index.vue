<template>
  <SecurityListPage
    title="会话管理"
    subtitle="集中查看在线、过期与风险会话，支持单个、批量与全量失效。"
    table-title="会话列表"
    table-desc="用于排查异常登录与及时清理过期会话。"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无会话数据"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #header-actions>
      <t-space>
        <t-button variant="outline" @click="batchRevoke">批量失效</t-button>
        <t-button theme="primary" @click="revokeAll">失效用户全部会话</t-button>
      </t-space>
    </template>

    <template #filters>
      <div class="filter-grid">
        <t-input v-model="filters.username" clearable placeholder="用户名" />
        <t-input v-model="filters.ip" clearable placeholder="IP 地址" />
        <t-select v-model="filters.status" clearable :options="statusOptions" placeholder="状态" />
        <t-select v-model="filters.platform" clearable :options="platformOptions" placeholder="平台" />
        <t-select v-model="filters.risk_flag" clearable :options="riskOptions" placeholder="风险标记" />
      </div>
    </template>

    <template #status="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
        {{ statusLabel[row.status] || row.status || '—' }}
      </t-tag>
    </template>

    <template #risk_flag="{ row }">
      <t-tag :theme="securityRiskTagTheme[row.risk_flag] || 'default'" variant="light-outline">
        {{ riskLabel[row.risk_flag] || row.risk_flag || '—' }}
      </t-tag>
    </template>

    <template #login_at="{ row }">
      {{ formatSecurityTime(row.login_at) }}
    </template>

    <template #last_active_at="{ row }">
      {{ formatSecurityTime(row.last_active_at) }}
    </template>

    <template #expired_at="{ row }">
      {{ formatSecurityTime(row.expired_at) }}
    </template>

    <template #operation="{ row }">
      <t-space size="small">
        <t-link theme="primary" @click="revokeOne(row)">失效</t-link>
      </t-space>
    </template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchRevokeSessions,
  getSessionList,
  revokeSession,
  revokeUserAllSessions,
  type SessionInfo,
  type SessionListQuery,
} from '@/api/security'

import SecurityListPage from '../SecurityListPage.vue'
import { formatSecurityTime, securityRiskTagTheme, securityStatusTagTheme } from '../shared'

defineOptions({ name: 'UserSecuritySessions' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<SessionInfo[]>([])

const filters = reactive<SessionListQuery>({
  page: 1,
  page_size: 10,
  username: '',
  status: '',
  platform: '',
  ip: '',
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

const statusOptions = [
  { label: '在线', value: 'online' },
  { label: '已失效', value: 'revoked' },
  { label: '过期', value: 'expired' },
]

const platformOptions = [
  { label: '后台', value: 'admin' },
  { label: '前台', value: 'user' },
]

const riskOptions = [
  { label: '低风险', value: 'low' },
  { label: '高风险', value: 'high' },
  { label: '严重风险', value: 'critical' },
]

const statusLabel: Record<string, string> = {
  online: '在线',
  revoked: '已失效',
  expired: '过期',
}

const riskLabel: Record<string, string> = {
  low: '低风险',
  high: '高风险',
  critical: '严重风险',
}

const columns: PrimaryTableCol<SessionInfo>[] = [
  { colKey: 'username', title: '用户名', width: 120 },
  { colKey: 'platform', title: '平台', width: 90 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'ip_region', title: '归属地', minWidth: 120 },
  { colKey: 'device_fingerprint', title: '设备指纹', minWidth: 180, ellipsis: true },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'risk_flag', title: '风险', width: 110 },
  { colKey: 'login_at', title: '登录时间', width: 180 },
  { colKey: 'last_active_at', title: '最近活跃', width: 180 },
  { colKey: 'expired_at', title: '过期时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 100, fixed: 'right' },
]

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getSessionList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载会话失败'
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
  filters.status = ''
  filters.platform = ''
  filters.ip = ''
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

async function revokeOne(row: SessionInfo) {
  await revokeSession(row.id)
  await loadData()
}

async function batchRevoke() {
  const ids = tableData.value.filter((item) => item.status === 'online').slice(0, 3).map((item) => item.id)
  if (!ids.length) return
  await batchRevokeSessions({ ids })
  await loadData()
}

async function revokeAll() {
  const userId = tableData.value[0]?.user_id
  if (!userId) return
  await revokeUserAllSessions({ user_id: userId })
  await loadData()
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
