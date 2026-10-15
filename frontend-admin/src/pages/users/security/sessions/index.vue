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
    :icon="RefreshIcon"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #header-actions>
      <t-space>
        <t-button v-permission="'security:session:manage'" variant="outline" @click="batchRevoke">批量失效</t-button>
        <t-button v-permission="'security:session:manage'" theme="primary" @click="revokeAll"
          >失效用户全部会话</t-button
        >
      </t-space>
    </template>

    <template #filters>
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
        <span class="field__label">会话主体</span>
        <t-select v-model="filters.subject_type" clearable :options="SUBJECT_TYPE_OPTIONS" placeholder="全部主体" />
      </div>
      <div class="field">
        <span class="field__label">风险标记</span>
        <t-select v-model="filters.risk_flag" clearable :options="RISK_FLAG_OPTIONS" placeholder="风险标记" />
      </div>
      <div class="field field--wide">
        <span class="field__label">登录时间</span>
        <t-date-range-picker v-model="dateRange" clearable allow-input @change="handleDateChange" />
      </div>
    </template>

    <template #platform="{ row }">
      {{ SESSION_PLATFORM_LABEL[row.platform] || row.platform || '—' }}
    </template>

    <template #subject_type="{ row }">
      <t-tag :theme="row.subject_type === 'admin' ? 'warning' : 'primary'" variant="light-outline">
        {{ SUBJECT_TYPE_LABEL[row.subject_type] || row.subject_type || '客户' }}
      </t-tag>
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
      <t-link v-if="!isMobile" v-permission="'security:session:manage'" theme="primary" @click="revokeOne(row)"
        >失效</t-link
      >
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
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
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
  SUBJECT_TYPE_LABEL,
  SUBJECT_TYPE_OPTIONS,
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
  subject_type: undefined,
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
  { colKey: 'subject_type', title: '主体', width: 100 },
  { colKey: 'platform', title: '平台', width: 90 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
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
  filters.subject_type = undefined
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
  try {
    await revokeSession(row.id)
    MessagePlugin.success('会话已失效')
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '失效会话失败')
  }
}

/**
 * 批量失效**当前页所有在线会话**。
 *
 * 此前这里带一个 `.slice(0, 3)`：按钮文案是「批量失效」，实际只失效前 3 条，
 * 界面上既不提示也不报错，运营以为整页都踢掉了。该上限来自早期的联调保护，
 * 不是产品要求；去掉它，改为按当前页实际在线数失效并回显条数。
 */
async function batchRevoke() {
  const ids = tableData.value.filter((item) => item.status === 'active').map((item) => item.id)
  if (!ids.length) {
    MessagePlugin.warning('当前页没有在线会话')
    return
  }
  const dialog = DialogPlugin.confirm({
    header: '批量失效会话',
    body: `确认让当前页 ${ids.length} 个在线会话全部失效？这些用户需要重新登录。`,
    confirmBtn: { content: '确认失效', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        const result = await batchRevokeSessions({ ids })
        MessagePlugin.success(`已失效 ${result.items?.length ?? ids.length} 个会话`)
        dialog.destroy()
        await loadData()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '批量失效失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

/**
 * 让**当前页第一条会话的账号**全部会话失效。
 *
 * 原实现直接取 `tableData[0].user_id` 且无二次确认：用户点「失效用户全部会话」
 * 时，作用对象是列表首行的账号 —— 这一行既不一定是用户想操作的人，界面上也
 * 没有任何提示，等于按一个按钮就随机踢掉某个账号（实测副作用：误踢后该用户
 * 所有终端被迫重新登录）。改为先确认并明确显示用户名与将失效的会话数。
 */
async function revokeAll() {
  const target = tableData.value[0]
  if (!target?.user_id) {
    MessagePlugin.warning('当前页没有可操作的会话')
    return
  }
  const activeCount = tableData.value.filter(
    (item) => item.user_id === target.user_id && item.status === 'active',
  ).length
  const dialog = DialogPlugin.confirm({
    header: '失效用户全部会话',
    body: `确认让「${target.username || `用户 #${target.user_id}`}」的全部会话失效？该账号在所有终端都需要重新登录。`,
    confirmBtn: { content: '确认失效', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        const result = await revokeUserAllSessions({ user_id: target.user_id })
        MessagePlugin.success(`已失效 ${result.items?.length ?? activeCount} 个会话`)
        dialog.destroy()
        await loadData()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '失效用户会话失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(() => {
  void loadData()
})
function handleMobileAction(value: string | number | Record<string, unknown>, row: SessionInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  if (action === 'revoke') void revokeOne(row)
}
</script>
