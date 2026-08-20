<template>
  <SecurityListPage
    title="黑名单管理"
    subtitle="统一管理 IP、账号与设备指纹黑名单，支持人工创建与状态切换。"
    table-title="黑名单列表"
    table-desc="查看命中来源、有效期与当前状态。"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    empty-text="暂无黑名单数据"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #header-actions>
      <t-button theme="primary" @click="openCreate">新增黑名单</t-button>
    </template>

    <template #filters>
      <div class="filter-grid">
        <t-select v-model="filters.type" clearable :options="typeOptions" placeholder="类型" />
        <t-select v-model="filters.status" clearable :options="statusOptions" placeholder="状态" />
        <t-select v-model="filters.source" clearable :options="sourceOptions" placeholder="来源" />
        <t-input v-model="filters.keyword" clearable placeholder="命中值关键词" />
      </div>
    </template>

    <template #type="{ row }">
      {{ typeLabelMap[row.type] || row.type || '—' }}
    </template>

    <template #status="{ row }">
      <t-tag :theme="securityStatusTagTheme[row.status] || 'default'" variant="light-outline">
        {{ statusLabelMap[row.status] || row.status || '—' }}
      </t-tag>
    </template>

    <template #effective_at="{ row }">
      {{ formatSecurityTime(row.effective_at) }}
    </template>

    <template #expired_at="{ row }">
      {{ formatSecurityTime(row.expired_at) }}
    </template>

    <template #hit_count="{ row }">
      {{ formatSecurityCount(row.hit_count) }}
    </template>

    <template #operation="{ row }">
      <t-space size="small">
        <t-link theme="primary" @click="openEdit(row)">编辑</t-link>
        <t-link theme="primary" @click="toggleStatus(row)">
          {{ row.status === 'active' ? '停用' : '启用' }}
        </t-link>
        <t-link theme="danger" @click="release(row)">解除</t-link>
      </t-space>
    </template>
  </SecurityListPage>

  <t-dialog
    v-model:visible="dialogVisible"
    :header="editingId ? '编辑黑名单' : '新增黑名单'"
    :confirm-btn="{ content: '保存', loading: submitting }"
    width="520px"
    @confirm="submitForm"
    @close="resetForm"
  >
    <t-form :data="formData" label-align="top" colonless>
      <t-form-item label="类型">
        <t-select v-model="formData.type" :options="typeOptions" placeholder="请选择类型" />
      </t-form-item>
      <t-form-item label="命中值">
        <t-input v-model="formData.target_value" placeholder="IP / 用户名 / 设备指纹" />
      </t-form-item>
      <t-form-item label="来源">
        <t-select v-model="formData.source" :options="sourceOptions" placeholder="请选择来源" />
      </t-form-item>
      <t-form-item label="状态">
        <t-select v-model="formData.status" :options="statusOptions" placeholder="请选择状态" />
      </t-form-item>
      <t-form-item label="原因">
        <t-textarea v-model="formData.reason" :maxlength="200" placeholder="请输入拉黑原因" />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createBlacklist,
  getBlacklistList,
  releaseBlacklist,
  updateBlacklist,
  updateBlacklistStatus,
  type BlacklistCreateRequest,
  type BlacklistInfo,
  type BlacklistListQuery,
} from '@/api/security'

import SecurityListPage from '../SecurityListPage.vue'
import { formatSecurityCount, formatSecurityTime, securityStatusTagTheme } from '../shared'

defineOptions({ name: 'UserSecurityBlacklist' })

const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const dialogVisible = ref(false)
const editingId = ref(0)
const tableData = ref<BlacklistInfo[]>([])

const filters = reactive<BlacklistListQuery>({
  page: 1,
  page_size: 10,
  type: '',
  status: '',
  source: '',
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

const formData = reactive<BlacklistCreateRequest>({
  type: 'ip',
  target_value: '',
  status: 'active',
  source: 'manual',
  reason: '',
})

const typeOptions = [
  { label: 'IP', value: 'ip' },
  { label: '账号', value: 'user' },
  { label: '设备', value: 'device' },
]

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'inactive' },
]

const sourceOptions = [
  { label: '人工', value: 'manual' },
  { label: '系统', value: 'system' },
]

const typeLabelMap: Record<string, string> = {
  ip: 'IP',
  user: '账号',
  device: '设备',
}

const statusLabelMap: Record<string, string> = {
  active: '启用',
  inactive: '停用',
}

const columns: PrimaryTableCol<BlacklistInfo>[] = [
  { colKey: 'type', title: '类型', width: 90 },
  { colKey: 'target_value', title: '命中值', minWidth: 180, ellipsis: true },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'reason', title: '原因', minWidth: 200, ellipsis: true },
  { colKey: 'hit_count', title: '命中次数', width: 100 },
  { colKey: 'effective_at', title: '生效时间', width: 180 },
  { colKey: 'expired_at', title: '失效时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 180, fixed: 'right' },
]

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getBlacklistList({
      ...filters,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载黑名单失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.type = ''
  filters.status = ''
  filters.source = ''
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

function resetForm() {
  editingId.value = 0
  formData.type = 'ip'
  formData.target_value = ''
  formData.status = 'active'
  formData.source = 'manual'
  formData.reason = ''
}

function openCreate() {
  resetForm()
  dialogVisible.value = true
}

function openEdit(row: BlacklistInfo) {
  editingId.value = row.id
  formData.type = row.type
  formData.target_value = row.target_value
  formData.status = row.status
  formData.source = row.source
  formData.reason = row.reason
  dialogVisible.value = true
}

async function submitForm() {
  if (!formData.target_value?.trim()) {
    MessagePlugin.warning('请输入命中值')
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await updateBlacklist(editingId.value, {
        status: formData.status,
        reason: formData.reason,
      })
    } else {
      await createBlacklist({
        ...formData,
        target_value: formData.target_value.trim(),
      })
    }
    dialogVisible.value = false
    resetForm()
    await loadData()
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: BlacklistInfo) {
  const nextStatus = row.status === 'active' ? 'inactive' : 'active'
  await updateBlacklistStatus(row.id, { status: nextStatus })
  await loadData()
}

async function release(row: BlacklistInfo) {
  await releaseBlacklist(row.id)
  await loadData()
}

onMounted(() => {
  void loadData()
})
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
