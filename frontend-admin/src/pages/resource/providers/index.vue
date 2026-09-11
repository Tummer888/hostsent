<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CloudIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">上游提供商</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="handleRefreshList">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新列表
        </t-button>
        <t-button theme="primary" @click="router.push('/resource/providers/create')">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          添加提供商
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            clearable
            placeholder="搜索名称 / API 地址"
            @enter="handleSearch"
          >
            <template #prefix-icon>
              <SearchIcon size="14" aria-hidden="true" />
            </template>
          </t-input>
        </div>
        <div class="field">
          <span class="field__label">提供商类型</span>
          <t-select v-model="filters.provider_type" clearable placeholder="全部类型" :options="typeOptions" />
        </div>
        <div class="field">
          <span class="field__label">链路</span>
          <t-select v-model="filters.kind" clearable placeholder="全部链路" :options="kindFilterOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusFilterOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">提供商列表</h3>
        <span class="table-card__meta">共 {{ total }} 条记录</span>
      </div>
      <t-table
        row-key="id"
        :data="providerList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="name-cell">
            <span class="name-cell__text">{{ row.name }}</span>
            <t-tooltip v-if="row.credential_error" :content="row.credential_error" placement="top">
              <t-tag theme="danger" variant="light" size="small" shape="round">凭证异常</t-tag>
            </t-tooltip>
          </div>
        </template>

        <template #provider_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.provider_type) }}</t-tag>
        </template>

        <template #kind="{ row }">
          <t-tag :theme="row.kind === 'compute' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.kind === 'compute' ? '算力平台' : '上游转售' }}
          </t-tag>
        </template>

        <template #resources="{ row }">
          <div class="resource-cell">
            <div class="resource-row">
              <span class="resource-label">CPU</span>
              <t-progress :percentage="usagePercent(row.used_cpu, row.total_cpu)" :color="usageColor(row.used_cpu, row.total_cpu)" :stroke-width="6" theme="line" />
            </div>
            <div class="resource-row">
              <span class="resource-label">内存</span>
              <t-progress :percentage="usagePercent(row.used_memory, row.total_memory)" :color="usageColor(row.used_memory, row.total_memory)" :stroke-width="6" theme="line" />
            </div>
            <div class="resource-row">
              <span class="resource-label">磁盘</span>
              <t-progress :percentage="usagePercent(row.used_disk, row.total_disk)" :color="usageColor(row.used_disk, row.total_disk)" :stroke-width="6" theme="line" />
            </div>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="resolveStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || '未知' }}
          </t-tag>
        </template>

        <template #last_sync_at="{ row }">
          <span class="time-text">{{ row.last_sync_at ? formatTime(row.last_sync_at) : '从未同步' }}</span>
        </template>

        <template #sync_state="{ row }">
          <t-tooltip v-if="row.sync_paused" :content="row.last_sync_error || '同步已暂停'" placement="top">
            <t-tag theme="warning" variant="light" size="small" shape="round">
              {{ row.last_sync_error === '适配器未实现' ? '未接入' : '已暂停' }}
            </t-tag>
          </t-tooltip>
          <t-tag v-else-if="row.consecutive_failures > 0" theme="danger" variant="light" size="small" shape="round">
            失败 {{ row.consecutive_failures }} 次
          </t-tag>
          <t-tag v-else theme="success" variant="light" size="small" shape="round">正常</t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="router.push(`/resource/providers/${row.id}`)">详情</t-link>
            <t-link theme="primary" hover="color" @click="openEditDialog(row)">编辑</t-link>
            <t-link theme="primary" hover="color" :loading="testingId === row.id" @click="handleTestConnection(row)">测试连接</t-link>
            <t-link v-if="row.sync_paused" theme="warning" hover="color" :loading="resumingId === row.id" @click="handleResumeSync(row)">恢复同步</t-link>
            <t-popconfirm content="确认删除该提供商？该操作不可恢复" @confirm="handleDelete(row)">
              <t-link theme="danger">删除</t-link>
            </t-popconfirm>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无提供商数据" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      header="编辑提供商"
      width="680px"
      :confirm-btn="{ content: '保存', theme: 'success', loading: submitting }"
      cancel-btn="取消"
      :on-confirm="handleSaveDialog"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top">
        <CapabilityMatrix
          v-if="editingDescriptor"
          :descriptor="editingDescriptor"
          :title="`${typeLabel(formData.provider_type)} 能力矩阵`"
          class="dialog-matrix"
        />
        <div class="form-grid">
          <t-form-item label="提供商名称" name="name">
            <t-input v-model="formData.name" placeholder="例如：华东 OpenStack" maxlength="50" />
          </t-form-item>
          <t-form-item label="类型" name="provider_type">
            <t-input :model-value="typeLabel(formData.provider_type)" disabled />
          </t-form-item>
          <t-form-item label="API 地址" name="api_endpoint">
            <t-input v-model="formData.api_endpoint" placeholder="https://api.example.com" />
          </t-form-item>
          <t-form-item label="区域" name="region">
            <t-input v-model="formData.region" placeholder="例如：cn-east-1" />
          </t-form-item>
          <t-form-item label="同步间隔（秒）" name="sync_interval">
            <t-input-number v-model="formData.sync_interval" :min="0" :step="60" placeholder="默认 3600" />
          </t-form-item>
          <t-form-item label="启用实例同步" name="sync_enabled">
            <t-switch v-model="formData.sync_enabled" />
          </t-form-item>
        </div>
        <CredentialFormFields
          v-if="editingCredentialFields.length"
          v-model="formData.credentials"
          :fields="editingCredentialFields"
        />
        <div v-else class="form-grid">
          <t-form-item label="API 密钥" name="api_key">
            <t-input v-model="formData.api_key" type="password" placeholder="留空表示不修改" />
          </t-form-item>
          <t-form-item label="API 密码" name="api_secret">
            <t-input v-model="formData.api_secret" type="password" placeholder="留空表示不修改" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, CloudIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  deleteProvider,
  getProviderList,
  getProviderTypes,
  resumeProviderSync,
  testConnection,
  updateProvider,
} from '@/api/admin'
import type { CapabilityDescriptor, ProviderInfo, ProviderTypeItem } from '@/types/interface'
import CapabilityMatrix from './components/CapabilityMatrix.vue'
import CredentialFormFields from './components/CredentialFormFields.vue'

defineOptions({ name: 'ResourceProviders' })

const router = useRouter()

const providerList = ref<ProviderInfo[]>([])
const loading = ref(false)
const submitting = ref(false)
const testingId = ref<number | null>(null)
const resumingId = ref<number | null>(null)
const total = ref(0)
const typeOptions = ref<{ label: string; value: string }[]>([])
const typeNameMap = ref<Record<string, string>>({})

const statusLabelMap: Record<number, string> = {
  1: '启用',
  0: '禁用',
}

const statusFilterOptions = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 0 },
]

const kindFilterOptions = [
  { label: '上游转售', value: 'upstream' },
  { label: '算力平台', value: 'compute' },
]

const filters = reactive({
  keyword: '',
  provider_type: '',
  kind: '',
  status: 0 as number | '',
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

function typeLabel(type: string): string {
  return typeNameMap.value[type] || type
}

function resolveStatusTheme(status: number) {
  if (status === 1) return 'success'
  return 'default'
}

function usagePercent(used: number, total: number): number {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

function usageColor(used: number, total: number): string {
  const pct = usagePercent(used, total)
  if (pct >= 80) return '#dc2626'
  if (pct >= 60) return '#d97706'
  return '#16a34a'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns: PrimaryTableCol<ProviderInfo>[] = [
  { colKey: 'name', title: '名称', minWidth: 200 },
  { colKey: 'provider_type', title: '类型', width: 120 },
  { colKey: 'kind', title: '链路', width: 110 },
  { colKey: 'api_endpoint', title: 'API 地址', minWidth: 240, ellipsis: true },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'sync_state', title: '同步状态', width: 110 },
  { colKey: 'resources', title: '资源概览', minWidth: 220 },
  { colKey: 'last_sync_at', title: '最后同步', width: 160 },
  {
    colKey: 'action',
    title: '操作',
    width: 260,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadTypes() {
  try {
    const types = await getProviderTypes()
    typeOptions.value = types.map((item: ProviderTypeItem) => ({ label: item.name, value: item.type }))
    typeNameMap.value = Object.fromEntries(types.map((item: ProviderTypeItem) => [item.type, item.name]))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商类型失败')
  }
}

async function loadProviders() {
  loading.value = true
  try {
    const data = await getProviderList({
      page: pagination.current,
      page_size: pagination.pageSize,
      keyword: filters.keyword || undefined,
      provider_type: filters.provider_type || undefined,
      kind: filters.kind || undefined,
      status: filters.status === '' ? undefined : filters.status,
    })
    providerList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadProviders()
}

function handleSearch() {
  pagination.current = 1
  loadProviders()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.provider_type = ''
  filters.kind = ''
  filters.status = ''
  pagination.current = 1
  loadProviders()
}

function handleRefreshList() {
  loadProviders()
}

async function handleTestConnection(row: ProviderInfo) {
  testingId.value = row.id
  try {
    const result = await testConnection(row.id)
    if (result.success) {
      MessagePlugin.success(`连接测试通过：${result.message}`)
    } else {
      MessagePlugin.error(`连接测试失败：${result.message}`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '连接测试失败')
  } finally {
    testingId.value = null
  }
}

async function handleResumeSync(row: ProviderInfo) {
  resumingId.value = row.id
  try {
    await resumeProviderSync(row.id)
    MessagePlugin.success('已恢复同步，下次调度将重新尝试')
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复同步失败')
  } finally {
    resumingId.value = null
  }
}

async function handleDelete(row: ProviderInfo) {  try {
    await deleteProvider(row.id)
    MessagePlugin.success('提供商已删除')
    if (providerList.value.length === 1 && pagination.current > 1) {
      pagination.current -= 1
    }
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除提供商失败')
  }
}

type ProviderForm = {
  name: string
  provider_type: string
  api_endpoint: string
  region: string
  api_key: string
  api_secret: string
  credentials: Record<string, string>
  sync_interval: number
  sync_enabled: boolean
  status: number
}

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const editingDescriptor = ref<CapabilityDescriptor | null>(null)
const formRef = ref<FormInstanceFunctions | null>(null)

const editingCredentialFields = computed(() => editingDescriptor.value?.credential_schema || [])

const formData = reactive<ProviderForm>({
  name: '',
  provider_type: '',
  api_endpoint: '',
  region: '',
  api_key: '',
  api_secret: '',
  credentials: {},
  sync_interval: 3600,
  sync_enabled: false,
  status: 1,
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' },
    { pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
}

function openEditDialog(row: ProviderInfo) {
  editingId.value = row.id
  editingDescriptor.value = row.capabilities || null
  Object.assign(formData, {
    name: row.name,
    provider_type: row.provider_type,
    api_endpoint: row.api_endpoint,
    region: row.region,
    api_key: '',
    api_secret: '',
    credentials: { ...(row.credentials || {}) },
    sync_interval: row.sync_interval,
    sync_enabled: row.sync_enabled,
    status: row.status,
  })
  formRef.value?.clearValidate?.()
  dialogVisible.value = true
}

async function handleSaveDialog() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  if (editingId.value === null) return
  submitting.value = true
  try {
    const payload = {
      name: formData.name,
      api_endpoint: formData.api_endpoint,
      region: formData.region,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      status: formData.status,
    } as Parameters<typeof updateProvider>[1]
    if (editingCredentialFields.value.length) {
      payload.credentials = formData.credentials
    } else {
      payload.api_key = formData.api_key || undefined
      payload.api_secret = formData.api_secret || undefined
    }
    await updateProvider(editingId.value, payload)
    MessagePlugin.success('提供商已保存')
    dialogVisible.value = false
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存提供商失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  formRef.value?.clearValidate?.()
}

onMounted(() => {
  loadTypes()
  loadProviders()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
  --chip-shadow: 0 4px 10px rgba(22, 163, 74, 0.25);
}

.filter-card__grid {
  grid-template-columns: minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(140px, 1fr);
}

.resource-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.resource-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-label {
  width: 34px;
  flex-shrink: 0;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.dialog-matrix {
  margin-bottom: var(--space-lg);
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.name-cell__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1200px) and (min-width: 769px) {
  .filter-card__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
