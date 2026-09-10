<template>
  <!-- 单根节点包裹：避免 <Transition> 对多根组件的动画警告 -->
  <div class="audit-page system-page">
    <t-tabs v-model="activeTab" class="audit-tabs">
      <t-tab-panel value="user" label="用户安全审计">
    <SecurityListPage
    title="操作审计"
    subtitle="管理员关键操作留痕查询与导出"
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
    <!-- 页头图标 -->
    <template #header-leading>
      <span class="page-header__chip">
        <HistoryIcon size="22" aria-hidden="true" />
      </span>
    </template>
    <!-- 页头右侧：导出 CSV（GET blob 下载） -->
    <template #header-actions>
      <t-button variant="outline" :loading="exporting" @click="handleExportCSV">
        <template #icon>
          <DownloadIcon />
        </template>
        导出 CSV
      </t-button>
    </template>

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

    <!-- 行操作：查看详情 -->
    <template #operation="{ row }">
      <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
    </template>
  </SecurityListPage>

  <!-- 详情弹窗：展示全部字段，payload 尝试格式化为 JSON -->
  <t-dialog v-model:visible="detailVisible" header="审计详情" width="680px" :footer="false">
    <t-descriptions v-if="detailRow" :column="2" bordered size="small">
      <t-descriptions-item label="ID">{{ detailRow.id }}</t-descriptions-item>
      <t-descriptions-item label="操作人">{{ detailRow.operator_name }}</t-descriptions-item>
      <t-descriptions-item label="模块">{{ detailRow.module }}</t-descriptions-item>
      <t-descriptions-item label="动作">{{ detailRow.action }}</t-descriptions-item>
      <t-descriptions-item label="资源类型">{{ detailRow.resource_type }}</t-descriptions-item>
      <t-descriptions-item label="资源 ID">{{ detailRow.resource_id }}</t-descriptions-item>
      <t-descriptions-item label="请求方法">{{ detailRow.request_method }}</t-descriptions-item>
      <t-descriptions-item label="状态码">
        <t-tag :theme="detailRow.response_code < 400 ? 'success' : 'danger'" variant="light-outline">
          {{ detailRow.response_code }}
        </t-tag>
      </t-descriptions-item>
      <t-descriptions-item label="请求路径" :span="2">
        {{ detailRow.request_path }}
      </t-descriptions-item>
      <t-descriptions-item label="响应信息" :span="2">
        {{ detailRow.response_message || '—' }}
      </t-descriptions-item>
      <t-descriptions-item label="IP">{{ detailRow.ip }}</t-descriptions-item>
      <t-descriptions-item label="Trace ID">{{ detailRow.trace_id }}</t-descriptions-item>
      <t-descriptions-item label="User Agent" :span="2">
        {{ detailRow.user_agent }}
      </t-descriptions-item>
      <t-descriptions-item label="发生时间" :span="2">
        {{ formatSecurityTime(detailRow.created_at) }}
      </t-descriptions-item>
      <t-descriptions-item label="请求负载" :span="2">
        <pre class="payload-block">{{ formatPayload(detailRow.request_payload) }}</pre>
      </t-descriptions-item>
    </t-descriptions>
  </t-dialog>
      </t-tab-panel>

      <!-- 管理操作审计（P2-06）：新表 admin_audit_logs，记录后台写操作 -->
      <t-tab-panel value="admin" label="管理操作审计">
        <t-card :bordered="false" class="admin-audit-card">
          <div class="filter-grid">
            <t-input v-model="adminFilters.keyword" clearable placeholder="操作人 / 路径" @enter="handleAdminSearch" />
            <t-input v-model="adminFilters.resource_type" clearable placeholder="资源类型（如 users）" />
            <t-input v-model="adminFilters.action" clearable placeholder="动作（如 create）" />
            <t-button theme="primary" @click="handleAdminSearch">查询</t-button>
            <t-button variant="outline" @click="handleAdminReset">重置</t-button>
          </div>

          <t-table
            row-key="id"
            :data="adminAuditData"
            :columns="adminColumns"
            :loading="adminAuditLoading"
            hover
            size="small"
            table-layout="fixed"
            class="admin-audit-table"
          >
            <template #method="{ row }">
              <t-tag variant="light-outline" size="small">{{ row.request_method }}</t-tag>
            </template>
            <template #response_code="{ row }">
              <t-tag :theme="row.response_code < 400 ? 'success' : 'danger'" variant="light-outline" size="small">
                {{ row.response_code }}
              </t-tag>
            </template>
            <template #created_at="{ row }">
              {{ row.created_at }}
            </template>
            <template #operation="{ row }">
              <t-link theme="primary" hover="color" @click="openAdminDetail(row)">详情</t-link>
            </template>
            <template #empty>
              <t-empty description="暂无管理操作审计记录" />
            </template>
          </t-table>

          <div class="admin-audit-pagination">
            <t-pagination
              v-model:current="adminPagination.current"
              v-model:page-size="adminPagination.pageSize"
              :total="adminPagination.total"
              show-jumper
              show-page-size
              :page-size-options="[10, 20, 50]"
              @change="loadAdminAudit"
            />
          </div>
        </t-card>
      </t-tab-panel>
    </t-tabs>

    <!-- 管理操作审计详情 -->
    <t-dialog v-model:visible="adminDetailVisible" header="操作审计详情" width="680px" :footer="false">
      <t-descriptions v-if="adminDetailRow" :column="2" bordered size="small">
        <t-descriptions-item label="ID">{{ adminDetailRow.id }}</t-descriptions-item>
        <t-descriptions-item label="操作人">{{ adminDetailRow.admin_name }}（ID {{ adminDetailRow.admin_id }}）</t-descriptions-item>
        <t-descriptions-item label="资源类型">{{ adminDetailRow.resource_type }}</t-descriptions-item>
        <t-descriptions-item label="资源 ID">{{ adminDetailRow.resource_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="动作">{{ adminDetailRow.action }}</t-descriptions-item>
        <t-descriptions-item label="状态码">
          <t-tag :theme="adminDetailRow.response_code < 400 ? 'success' : 'danger'" variant="light-outline">
            {{ adminDetailRow.response_code }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="请求方法">{{ adminDetailRow.request_method }}</t-descriptions-item>
        <t-descriptions-item label="IP">{{ adminDetailRow.ip }}</t-descriptions-item>
        <t-descriptions-item label="请求路径" :span="2">{{ adminDetailRow.request_path }}</t-descriptions-item>
        <t-descriptions-item label="User Agent" :span="2">{{ adminDetailRow.user_agent }}</t-descriptions-item>
        <t-descriptions-item label="发生时间" :span="2">{{ adminDetailRow.created_at }}</t-descriptions-item>
        <t-descriptions-item label="请求负载（已脱敏）" :span="2">
          <pre class="payload-block">{{ formatPayload(adminDetailRow.detail) }}</pre>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'

import type { AxiosResponse } from 'axios'
import { DownloadIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import type { AuditLogInfo, AuditLogListQuery } from '@/api/system'
import { getAuditLogList } from '@/api/system'
import {
  getAdminAuditLogs,
  type AdminAuditLogInfo,
  type AdminAuditLogQuery,
} from '@/api/admin'
import { request } from '@/utils/request'

import SecurityListPage from '../../users/security/SecurityListPage.vue'
import { formatSecurityTime } from '../../users/security/shared'

defineOptions({ name: 'SystemAuditLogs' })

const activeTab = ref<'user' | 'admin'>('user')

const loading = ref(false)
const exporting = ref(false)
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
  { colKey: 'operation', title: '操作', width: 80, fixed: 'right' },
]

const detailVisible = ref(false)
const detailRow = ref<AuditLogInfo | null>(null)

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

function openDetail(row: AuditLogInfo) {
  detailRow.value = row
  detailVisible.value = true
}

/** 尝试将 payload 解析为 JSON 并格式化，失败时原样返回 */
function formatPayload(raw: string): string {
  if (!raw) return '—'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

/** 生成导出文件名时间戳：YYYYMMDD-HHmmss */
function formatFileTimestamp(): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(
    now.getHours(),
  )}${pad(now.getMinutes())}${pad(now.getSeconds())}`
}

/** 导出当前筛选结果为 CSV：GET blob 请求，手动触发浏览器下载 */
async function handleExportCSV() {
  exporting.value = true
  try {
    const response = await request.get<AxiosResponse<Blob>>({
      url: '/security/audit-logs/export',
      params: {
        ...filters,
        page: pagination.current,
        page_size: pagination.pageSize,
      },
      responseType: 'blob',
      _skipResultUnwrap: true,
    })

    const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
    const objectUrl = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = objectUrl
    link.download = `audit-logs-${formatFileTimestamp()}.csv`
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

onMounted(() => {
  void loadData()
})

// ===== 管理操作审计（P2-06） =====
const adminAuditLoading = ref(false)
const adminAuditData = ref<AdminAuditLogInfo[]>([])
const adminDetailVisible = ref(false)
const adminDetailRow = ref<AdminAuditLogInfo | null>(null)

const adminFilters = reactive<AdminAuditLogQuery>({
  keyword: '',
  resource_type: '',
  action: '',
})

const adminPagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})

const adminColumns: PrimaryTableCol<AdminAuditLogInfo>[] = [
  { colKey: 'admin_name', title: '操作人', width: 120 },
  { colKey: 'module', title: '模块', width: 110 },
  { colKey: 'action', title: '动作', width: 120 },
  { colKey: 'resource_type', title: '资源类型', width: 120 },
  { colKey: 'resource_id', title: '资源 ID', width: 90 },
  { colKey: 'method', title: '方法', width: 90 },
  { colKey: 'request_path', title: '路径', minWidth: 200, ellipsis: true },
  { colKey: 'response_code', title: '状态码', width: 90 },
  { colKey: 'created_at', title: '发生时间', width: 170 },
  { colKey: 'operation', title: '操作', width: 80, fixed: 'right' },
]

async function loadAdminAudit() {
  adminAuditLoading.value = true
  try {
    const data = await getAdminAuditLogs({
      ...adminFilters,
      page: adminPagination.current,
      page_size: adminPagination.pageSize,
    })
    adminAuditData.value = data.items
    adminPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载管理操作审计失败')
  } finally {
    adminAuditLoading.value = false
  }
}

function handleAdminSearch() {
  adminPagination.current = 1
  void loadAdminAudit()
}

function handleAdminReset() {
  adminFilters.keyword = ''
  adminFilters.resource_type = ''
  adminFilters.action = ''
  adminPagination.current = 1
  void loadAdminAudit()
}

function openAdminDetail(row: AdminAuditLogInfo) {
  adminDetailRow.value = row
  adminDetailVisible.value = true
}

// 切到管理操作审计 tab 时懒加载
watch(activeTab, (tab) => {
  if (tab === 'admin' && adminAuditData.value.length === 0) {
    void loadAdminAudit()
  }
})
</script>

<style scoped>
@import '../shared.css';

.audit-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.audit-tabs {
  margin-top: -8px;
}

.admin-audit-card {
  padding: 4px 0 0;
}

.admin-audit-card .filter-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr)) auto auto;
  align-items: center;
  margin-bottom: 16px;
}

.admin-audit-table {
  margin-top: 4px;
}

.admin-audit-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 1200px) {
  .admin-audit-card .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

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

.payload-block {
  margin: 0;
  padding: 10px 12px;
  max-height: 320px;
  overflow: auto;
  border-radius: var(--td-radius-medium);
  background: var(--td-bg-color-secondarycontainer, #f3f3f3);
  font-family: var(--hs-font-mono, ui-monospace, Menlo, Consolas, monospace);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
