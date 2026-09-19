<template>
  <SecurityListPage
    title="会话管理"
    table-title="会话列表"
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
    :icon="RefreshIcon"
  >
    <template #header-actions>
      <t-space>
        <t-button v-permission="'security:session:manage'" variant="outline" @click="batchRevoke">批量失效</t-button>
        <t-button v-permission="'security:session:manage'" theme="primary" @click="revokeAll">失效用户全部会话</t-button>
      </t-space>
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
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable :options="SESSION_STATUS_OPTIONS" placeholder="状态" />
        </div>
        <div class="field">
          <span class="field__label">平台</span>
          <t-select v-model="filters.platform" clearable :options="SESSION_PLATFORM_OPTIONS" placeholder="平台" />
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

    <template #platform="{ row }">
      {{ SESSION_PLATFORM_LABEL[row.platform] || row.platform || '—' }}
    </template>

    <template #status="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
        {{ SESSION_STATUS_LABEL[row.status] || row.status || '—' }}
      </t-tag>
    </template>

    <template #risk_flag="{ row }">
      <t-tag :theme="securityRiskTagTheme[row.risk_flag] || 'default'" variant="light-outline">
        {{ RISK_FLAG_LABEL[row.risk_flag] || row.risk_flag || '—' }}
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
      <t-link v-if="!isMobile" v-permission="'security:session:manage'" theme="primary" @click="revokeOne(row)">失效</t-link>
      <MobileAction
        v-else
        :options="[{ content: '失效', value: 'revoke' }]"
        @select="(value) => handleMobileAction(value, row)"
      />
    </template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { RefreshIcon } from 'tdesign-icons-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'

import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchRevokeSessions,
  getSessionList,
  revokeSession,
  revokeUserAllSessions,
  type SessionInfo,
  type SessionListQuery,
} from '@/api/security'

import MobileAction from '@/components/mobile-action/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import SecurityListPage from '../SecurityListPage.vue'
import {
  RISK_FLAG_LABEL,
  RISK_FLAG_OPTIONS,
  SESSION_PLATFORM_LABEL,
  SESSION_PLATFORM_OPTIONS,
  SESSION_STATUS_LABEL,
  SESSION_STATUS_OPTIONS,
  applyDateRange,
  formatSecurityTime,
  securityRiskTagTheme,
  securityStatusTagTheme,
} from '../shared'

defineOptions({ name: 'UserSecuritySessions' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<SessionInfo[]>([])
const dateRange = ref<string[]>([])

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

const { isMobile } = useIsMobile()

const columns = computed<PrimaryTableCol<SessionInfo>[]>(() => [
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
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 100, fixed: 'right' },
])

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

function handleDateChange(value: unknown) {
  applyDateRange(filters, value)
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

async function revokeOne(row: SessionInfo) {
  await revokeSession(row.id)
  await loadData()
}

async function batchRevoke() {
  // 只挑仍在线的会话（库里 active 才是有效会话，online 是旧前端编的值）。
  const ids = tableData.value.filter((item) => item.status === 'active').slice(0, 3).map((item) => item.id)
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
function handleMobileAction(value: string | number | Record<string, any>, row: SessionInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  if (action === 'revoke') void revokeOne(row)
}
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
