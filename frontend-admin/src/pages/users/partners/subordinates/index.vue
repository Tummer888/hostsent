<template>
  <div class="distribution-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">下级用户管理</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">统一管理代理与下级用户的绑定关系，支持按代理、层级、状态和关键词筛选与维护。</p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/partners/agents')">查看代理商</t-button>
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增关系
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按代理商、关系层级与状态快速过滤团队成员，便于运营校对上下级绑定关系。</p>
        </div>
        <div class="toolbar__actions">
          <t-space>
            <t-button class="page-btn" theme="primary" @click="handleSearch">
              <template #icon>
                <SearchIcon aria-hidden="true" />
              </template>
              查询
            </t-button>
            <t-button class="page-btn page-btn--ghost" variant="outline" @click="handleReset">重置</t-button>
          </t-space>
        </div>
      </div>

      <div class="toolbar__grid toolbar__grid--subordinates">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索代理商 / 用户名 / 手机号 / 关系链"
            @enter="handleSearch"
          >
            <template #prefix-icon>
              <SearchIcon />
            </template>
          </t-input>
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">代理商</span>
          <t-select
            v-model="selectedAgentId"
            class="unified-control"
            clearable
            filterable
            placeholder="全部代理商"
            :options="agentOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">关系层级</span>
          <t-select
            v-model="selectedLevelDepth"
            class="unified-control"
            clearable
            placeholder="全部层级"
            :options="depthOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">状态</span>
          <t-select
            v-model="filters.status"
            class="unified-control"
            clearable
            placeholder="全部状态"
            :options="statusOptions"
          />
        </div>
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <div>
          <h3 class="table-panel__title">下级关系列表</h3>
          <p class="table-panel__desc">展示上级代理、下级用户、关系链、贡献金额与已产生佣金，支撑团队关系核对与数据纠偏。</p>
        </div>
        <div class="table-panel__meta">
          <span>共 {{ pagination.total }} 条</span>
          <span>当前第 {{ pagination.current }} 页</span>
        </div>
      </div>

      <div v-if="errorMessage" class="error-banner" role="alert">
        <ErrorCircleIcon size="16" aria-hidden="true" />
        <span>{{ errorMessage }}</span>
        <t-link class="page-link" theme="primary" hover="color" @click="reload">重试</t-link>
      </div>

      <t-table
        row-key="id"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        class="distribution-table"
        @page-change="handlePageChange"
      >
        <template #agent_name="{ row }">
          <div class="primary-cell">
            <span class="primary-cell__title">{{ row.agent_name }}</span>
            <span class="primary-cell__sub">代理 ID：{{ row.agent_id }}</span>
          </div>
        </template>

        <template #username="{ row }">
          <div class="primary-cell">
            <div class="primary-cell__title-row">
              <span class="primary-cell__title">{{ row.username }}</span>
              <t-tag class="status-chip" theme="primary" variant="light" size="small" shape="round">#{{ row.user_id }}</t-tag>
            </div>
            <span class="primary-cell__sub">{{ row.real_name || row.phone || '未补充实名/手机号' }}</span>
          </div>
        </template>

        <template #level_depth="{ row }">
          <span class="depth-pill">第 {{ row.level_depth }} 层</span>
        </template>

        <template #relation_path="{ row }">
          <span class="mono-pill">{{ row.relation_path || '—' }}</span>
        </template>

        <template #contribution_amount="{ row }">
          <span class="money-text">¥{{ formatMoney(row.contribution_amount) }}</span>
        </template>

        <template #commission_amount="{ row }">
          <span class="money-text money-text--secondary">¥{{ formatMoney(row.commission_amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :class="['status-tag', `status-tag--${row.status || 'default'}`]" theme="default" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || row.status || '未知' }}
          </t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #operation="{ row }">
          <t-space size="small">
            <t-link theme="primary" hover="color" @click="openEdit(row.id)">编辑</t-link>
            <t-popconfirm content="确认删除该下级关系？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无下级关系数据" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="760px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建关系' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="代理商" name="agent_id">
            <t-select v-model="formData.agent_id" :options="agentOptions" placeholder="请选择代理商" filterable />
          </t-form-item>
          <t-form-item label="用户 ID" name="user_id">
            <t-input-number v-model="formData.user_id" :min="1" theme="normal" />
          </t-form-item>
          <t-form-item label="关系层级" name="level_depth">
            <t-input-number v-model="formData.level_depth" :min="1" :max="10" theme="normal" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-switch">
              <t-radio-button value="active">启用</t-radio-button>
              <t-radio-button value="disabled">禁用</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item label="贡献金额" name="contribution_amount">
            <t-input-number v-model="formData.contribution_amount" :min="0" :step="100" theme="normal" />
          </t-form-item>
          <t-form-item label="佣金金额" name="commission_amount">
            <t-input-number v-model="formData.commission_amount" :min="0" :step="100" theme="normal" />
          </t-form-item>
        </div>
        <t-form-item label="关系路径" name="relation_path">
          <t-input v-model="formData.relation_path" placeholder="例如 A001>B201>U3001" maxlength="255" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AddIcon, ErrorCircleIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createSubordinate,
  deleteSubordinate,
  getAgentList,
  getSubordinateDetail,
  getSubordinateList,
  updateSubordinate,
  type AgentInfo,
  type SubordinateInfo,
  type SubordinateListQuery,
  type SubordinateRequest,
} from '@/api/user'

defineOptions({ name: 'UserPartnersSubordinates' })

type DialogMode = 'create' | 'edit'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const dialogVisible = ref(false)
const dialogMode = ref<DialogMode>('create')
const editingId = ref<number>(0)
const formRef = ref<FormInstanceFunctions | null>(null)
const tableData = ref<SubordinateInfo[]>([])
const agents = ref<AgentInfo[]>([])
const selectedAgentId = ref<number | undefined>()
const selectedLevelDepth = ref<number | undefined>()

const filters = reactive<SubordinateListQuery>({
  page: 1,
  page_size: 10,
  agent_id: undefined,
  status: '',
  level_depth: undefined,
  keyword: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50, 100],
})

const initFormData = (): SubordinateRequest => ({
  agent_id: 0,
  user_id: 0,
  level_depth: 1,
  relation_path: '',
  contribution_amount: 0,
  commission_amount: 0,
  status: 'active',
})

const formData = reactive<SubordinateRequest>(initFormData())

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const depthOptions = [1, 2, 3, 4, 5].map((depth) => ({ label: `第 ${depth} 层`, value: depth }))

const statusLabelMap: Record<string, string> = {
  active: '启用',
  disabled: '禁用',
}

const rules: Record<string, FormRule[]> = {
  agent_id: [{ required: true, message: '请选择代理商', type: 'error', trigger: 'change' }],
  user_id: [{ required: true, message: '请输入用户 ID', type: 'error', trigger: 'blur' }],
  level_depth: [{ required: true, message: '请输入关系层级', type: 'error', trigger: 'blur' }],
  relation_path: [{ required: true, message: '请输入关系路径', type: 'error', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const columns: PrimaryTableCol<SubordinateInfo>[] = [
  { colKey: 'agent_name', title: '上级代理', minWidth: 180 },
  { colKey: 'username', title: '下级用户', minWidth: 220 },
  { colKey: 'level_depth', title: '层级', width: 100, align: 'center' },
  { colKey: 'relation_path', title: '关系链路', minWidth: 180 },
  { colKey: 'contribution_amount', title: '贡献金额', width: 120, align: 'right' },
  { colKey: 'commission_amount', title: '累计佣金', width: 120, align: 'right' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 140, fixed: 'right' },
]

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增下级关系' : '编辑下级关系'))

const agentOptions = computed(() =>
  agents.value.map((item) => ({
    label: `${item.username} / ${item.agent_level_name || '未设等级'}`,
    value: item.id,
  })),
)

const activeFilterLabel = computed(() => {
  const agent = agents.value.find((item) => item.id === filters.agent_id)
  if (agent) return agent.username
  if (filters.level_depth) return `第 ${filters.level_depth} 层`
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.agent_id = query.agent_id ? Number(query.agent_id) : undefined
  filters.status = query.status || ''
  filters.level_depth = query.level_depth ? Number(query.level_depth) : undefined
  filters.keyword = query.keyword || ''
  selectedAgentId.value = filters.agent_id
  selectedLevelDepth.value = filters.level_depth
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.agent_id) query.agent_id = String(filters.agent_id)
  if (filters.status) query.status = filters.status
  if (filters.level_depth) query.level_depth = String(filters.level_depth)
  if (filters.keyword) query.keyword = filters.keyword
  return query
}

async function replaceRouteQuery() {
  await router.replace({ query: buildQuery() })
}

function resetForm() {
  Object.assign(formData, initFormData())
  editingId.value = 0
}

async function loadAgents() {
  try {
    const data = await getAgentList({ page: 1, page_size: 100 })
    agents.value = data.items || []
  } catch {
    agents.value = []
  }
}

async function loadSubordinates() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getSubordinateList({
      page: filters.page,
      page_size: filters.page_size,
      agent_id: filters.agent_id,
      status: filters.status || undefined,
      level_depth: filters.level_depth,
      keyword: filters.keyword || undefined,
    })
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
    filters.page = data.meta.page
    filters.page_size = data.meta.page_size
  } catch (error) {
    tableData.value = []
    pagination.total = 0
    errorMessage.value = (error as Error)?.message || '加载下级关系列表失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadAgents(), loadSubordinates()])
}

async function handleSearch() {
  filters.page = 1
  filters.agent_id = selectedAgentId.value
  filters.level_depth = selectedLevelDepth.value
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.agent_id = undefined
  filters.status = ''
  filters.level_depth = undefined
  filters.keyword = ''
  selectedAgentId.value = undefined
  selectedLevelDepth.value = undefined
  pagination.current = 1
  pagination.pageSize = 10
  await replaceRouteQuery()
}

async function handlePageChange(pageInfo: PageInfo) {
  filters.page = pageInfo.current
  filters.page_size = pageInfo.pageSize
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  await replaceRouteQuery()
}

async function reload() {
  await loadAll()
}

function openCreate() {
  dialogMode.value = 'create'
  resetForm()
  dialogVisible.value = true
}

async function openEdit(id: number) {
  dialogMode.value = 'edit'
  resetForm()
  submitting.value = true
  try {
    const data = await getSubordinateDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      agent_id: Number(data.agent_id || 0),
      user_id: Number(data.user_id || 0),
      level_depth: Number(data.level_depth || 1),
      relation_path: data.relation_path || '',
      contribution_amount: Number(data.contribution_amount || 0),
      commission_amount: Number(data.commission_amount || 0),
      status: data.status || 'active',
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载下级关系详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return

  const payload: SubordinateRequest = {
    agent_id: Number(formData.agent_id || 0),
    user_id: Number(formData.user_id || 0),
    level_depth: Number(formData.level_depth || 1),
    relation_path: formData.relation_path.trim(),
    contribution_amount: Number(formData.contribution_amount || 0),
    commission_amount: Number(formData.commission_amount || 0),
    status: formData.status,
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createSubordinate(payload)
      MessagePlugin.success('下级关系创建成功')
    } else {
      await updateSubordinate(editingId.value, payload)
      MessagePlugin.success('下级关系更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadSubordinates()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存下级关系失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleDelete(row: SubordinateInfo) {
  try {
    await deleteSubordinate(row.id)
    MessagePlugin.success(`已删除下级关系 ${row.username}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadSubordinates()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除下级关系失败')
  }
}

function formatMoney(value: number) {
  return Number(value || 0).toFixed(2)
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadSubordinates()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  await loadAll()
})
</script>

<style scoped lang="css">
.distribution-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.list-header,
.toolbar,
.table-panel {
  border-radius: var(--hs-radius-lg);
}

.list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 24px;
  border-color: #d1fae5;
}

.list-header__main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
}

.list-header__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.list-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.list-header__subtitle {
  margin: 0;
  color: var(--color-muted-foreground);
  font-size: 13px;
  line-height: 1.7;
}

.list-header__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar {
  padding: 18px 20px;
  background: var(--hs-surface-1);
  border-color: #dcfce7;
}

.toolbar__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.toolbar__title,
.table-panel__title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.toolbar__desc,
.table-panel__desc {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.toolbar__actions {
  display: flex;
  justify-content: flex-end;
}

.toolbar__grid {
  display: grid;
  gap: 14px;
}

.toolbar__grid--subordinates {
  grid-template-columns: minmax(260px, 2fr) repeat(3, minmax(160px, 1fr));
}

.toolbar-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.toolbar-field--keyword {
  min-width: 0;
}

.toolbar-field__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.table-panel {
  padding: 16px;
  background: var(--hs-surface-1);
  border-color: #dcfce7;
}

.table-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.table-panel__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(239, 68, 68, 0.18);
  border-radius: var(--hs-radius-md);
  background: rgba(239, 68, 68, 0.06);
  color: var(--color-destructive);
}

.primary-cell,
.metric-stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.primary-cell__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.primary-cell__title {
  color: var(--color-foreground);
  font-weight: 700;
}

.primary-cell__sub,
.time-text {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.depth-pill,
.mono-pill {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.depth-pill {
  border: 1px solid #d9f99d;
  background: #f7fee7;
  color: #3f6212;
}

.mono-pill {
  border: 1px solid #dbeafe;
  background: #eff6ff;
  color: #1d4ed8;
  font-family: var(--hs-font-mono);
}

.money-text {
  color: var(--color-primary);
  font-weight: 700;
}

.money-text--secondary {
  color: #0284c7;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

:deep(.page-btn.t-button--theme-primary) {
  background-color: var(--color-primary);
  border-color: var(--color-primary);
}

:deep(.page-btn.t-button--theme-primary:hover),
:deep(.page-btn.t-button--theme-primary:focus-visible) {
  background-color: #15803d;
  border-color: #15803d;
}

:deep(.page-btn--ghost) {
  color: var(--color-primary);
  border-color: #bbf7d0;
  background: #ecfdf5;
}

:deep(.page-btn--ghost:hover),
:deep(.page-btn--ghost:focus-visible) {
  color: #15803d;
  border-color: #86efac;
  background: #dcfce7;
}

:deep(.page-chip.t-tag--primary.t-tag--variant-light),
:deep(.status-chip.t-tag--primary.t-tag--variant-light) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.page-link),
:deep(.page-link.t-link) {
  color: var(--color-primary);
}

:deep(.page-link:hover),
:deep(.page-link.t-link:hover) {
  color: #15803d;
}

:deep(.unified-control .t-input),
:deep(.unified-control .t-input__wrap),
:deep(.unified-control .t-input-adornment),
:deep(.unified-control .t-input__suffix),
:deep(.unified-control .t-select__wrap) {
  background: var(--hs-surface-2);
}

:deep(.unified-control .t-input),
:deep(.unified-control .t-select__wrap) {
  border-color: var(--color-border);
  border-radius: var(--hs-radius-md);
}

:deep(.unified-control.t-is-focused .t-input),
:deep(.unified-control.t-is-focused .t-select__wrap),
:deep(.unified-control .t-input:focus-within),
:deep(.unified-control .t-select__wrap:focus-within) {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.10);
}

:deep(.distribution-table .t-table) {
  border-color: #dcfce7;
}

:deep(.distribution-table .t-table__header th) {
  color: var(--color-muted-foreground);
  background: #f8fffb;
  font-weight: 600;
  border-bottom-color: #dcfce7;
}

:deep(.distribution-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: #f0fdf4;
  vertical-align: middle;
}

:deep(.distribution-table .t-table__row--hover td) {
  background: rgba(22, 163, 74, 0.03);
}

:deep(.distribution-table .t-table__pagination) {
  padding-top: 16px;
}

:deep(.distribution-table .t-pagination__number),
:deep(.distribution-table .t-pagination__btn) {
  min-width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  border-color: #dcfce7;
  background: #ffffff;
}

:deep(.distribution-table .t-pagination__number.t-is-current) {
  color: #15803d;
  border-color: #bbf7d0;
  background: #ecfdf5;
  font-weight: 700;
}

:deep(.distribution-table .t-pagination__select-input .t-input),
:deep(.distribution-table .t-pagination__size .t-select__wrap),
:deep(.distribution-table .t-pagination .t-input) {
  border-radius: var(--hs-radius-md);
  border-color: #dcfce7;
  background: #ffffff;
}

:deep(.status-tag) {
  border-radius: var(--hs-radius-xl);
  font-weight: 600;
  border: 1px solid transparent;
}

:deep(.status-tag--active) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.status-tag--disabled) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

:deep(.status-switch .t-radio-button) {
  border-color: #dcfce7;
  color: var(--color-muted-foreground);
  background: #ffffff;
}

:deep(.status-switch .t-radio-button.t-is-checked) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

@media (max-width: 1280px) {
  .toolbar__grid--subordinates,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .table-panel__head,
  .toolbar__header {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 768px) {
  .list-header,
  .toolbar__header {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar__actions,
  .list-header__actions {
    justify-content: flex-start;
  }
}
</style>
