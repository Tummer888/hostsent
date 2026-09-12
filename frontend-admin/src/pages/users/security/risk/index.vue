<template>
  <SecurityListPage
    title="异常行为监控"
    table-title="风险事件"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无风险事件"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
    :icon="ChartBarIcon"
  >
    <template #filters>
      <div class="filter-grid">
        <div class="field">
          <span class="field__label">风险类型</span>
          <t-select v-model="filters.risk_type" clearable :options="riskTypeOptions" placeholder="风险类型" />
        </div>
        <div class="field">
          <span class="field__label">风险等级</span>
          <t-select v-model="filters.risk_level" clearable :options="riskLevelOptions" placeholder="风险等级" />
        </div>
        <div class="field">
          <span class="field__label">处置状态</span>
          <t-select v-model="filters.status" clearable :options="statusOptions" placeholder="处置状态" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="关键词 / 用户名 / IP" />
        </div>
      </div>
    </template>

    <template #risk_level="{ row }">
      <t-tag :theme="securityRiskTagTheme[row.risk_level] || 'default'" variant="light-outline">
        {{ riskLevelLabel[row.risk_level] || row.risk_level || '—' }}
      </t-tag>
    </template>

    <template #status="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
        {{ statusLabel[row.status] || row.status || '—' }}
      </t-tag>
    </template>

    <template #occur_count="{ row }">
      {{ formatSecurityCount(row.occur_count) }}
    </template>

    <template #first_occurred_at="{ row }">
      {{ formatSecurityTime(row.first_occurred_at) }}
    </template>

    <template #last_occurred_at="{ row }">
      {{ formatSecurityTime(row.last_occurred_at) }}
    </template>

    <template #operation="{ row }">
      <MobileAction
        v-if="isMobile"
        :options="buildMobileActionOptions([
          { content: '忽略', value: 'ignore' },
          { content: '处置', value: 'resolve' },
          { content: '拉黑', value: 'blacklist', theme: 'error' },
          { content: '失效会话', value: 'revoke' },
        ])"
        @select="(value) => handleMobileAction(value, row)"
      />
      <t-space v-else size="small">
        <t-link theme="primary" @click="handleIgnore(row)">忽略</t-link>
        <t-link theme="primary" @click="handleResolve(row)">处置</t-link>
        <t-link theme="danger" @click="handleBlacklist(row)">拉黑</t-link>
        <t-link theme="primary" @click="handleRevoke(row)">失效会话</t-link>
      </t-space>
    </template>
  </SecurityListPage>
</template>

<script setup lang="ts">
import { ChartBarIcon } from 'tdesign-icons-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'

import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  blacklistRiskEvent,
  getRiskEventList,
  handleRiskEvent,
  ignoreRiskEvent,
  revokeRiskEventSessions,
  type RiskEventInfo,
  type RiskEventListQuery,
} from '@/api/security'

import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import SecurityListPage from '../SecurityListPage.vue'
import { formatSecurityCount, formatSecurityTime, securityRiskTagTheme, securityStatusTagTheme } from '../shared'

defineOptions({ name: 'UserSecurityRisk' })

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<RiskEventInfo[]>([])

const filters = reactive<RiskEventListQuery>({
  page: 1,
  page_size: 10,
  risk_type: '',
  risk_level: '',
  status: '',
  keyword: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const riskTypeOptions = [
  { label: '异地登录', value: '异地登录' },
  { label: '高频失败登录', value: '高频失败登录' },
  { label: '越权尝试', value: '越权尝试' },
]

const riskLevelOptions = [
  { label: '低', value: 'low' },
  { label: '中', value: 'medium' },
  { label: '高', value: 'high' },
  { label: '严重', value: 'critical' },
]

const statusOptions = [
  { label: '待处理', value: 'pending' },
  { label: '已忽略', value: 'ignored' },
  { label: '已处置', value: 'handled' },
]

const riskLevelLabel: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重',
}

const statusLabel: Record<string, string> = {
  pending: '待处理',
  ignored: '已忽略',
  handled: '已处置',
}

const { isMobile } = useIsMobile()

const columns = computed<PrimaryTableCol<RiskEventInfo>[]>(() => [
  { colKey: 'risk_type', title: '风险类型', width: 130 },
  { colKey: 'risk_level', title: '等级', width: 100 },
  { colKey: 'username', title: '用户名', width: 120 },
  { colKey: 'ip', title: 'IP 地址', width: 130 },
  { colKey: 'rule_code', title: '规则编码', width: 160 },
  { colKey: 'summary', title: '摘要', minWidth: 200, ellipsis: true },
  { colKey: 'occur_count', title: '命中次数', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'first_occurred_at', title: '首次发生', width: 180 },
  { colKey: 'last_occurred_at', title: '最近发生', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 240, fixed: 'right' },
])

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getRiskEventList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载风险事件失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.risk_type = ''
  filters.risk_level = ''
  filters.status = ''
  filters.keyword = ''
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

async function handleIgnore(row: RiskEventInfo) {
  await ignoreRiskEvent(row.id)
  await loadData()
}

async function handleResolve(row: RiskEventInfo) {
  await handleRiskEvent(row.id)
  await loadData()
}

async function handleBlacklist(row: RiskEventInfo) {
  await blacklistRiskEvent(row.id)
  await loadData()
}

async function handleRevoke(row: RiskEventInfo) {
  await revokeRiskEventSessions(row.id)
  await loadData()
}

onMounted(() => {
  void loadData()
})
function handleMobileAction(value: string | number | Record<string, any>, row: RiskEventInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'ignore':
      handleIgnore(row)
      break
    case 'resolve':
      handleResolve(row)
      break
    case 'blacklist':
      handleBlacklist(row)
      break
    case 'revoke':
      handleRevoke(row)
      break
  }
}
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
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
