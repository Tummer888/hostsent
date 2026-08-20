<template>
  <div class="distribution-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">代理商列表</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">统一查看代理账号、等级、邀请码、团队规模与佣金沉淀，支持按状态、等级和关键词筛选。</p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/partners/levels')">等级配置</t-button>
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增代理商
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按代理状态、等级与关键词快速定位合作账号，延续当前后台高密度列表操作方式。</p>
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

      <div class="toolbar__grid toolbar__grid--agents">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索用户名 / 邀请码 / 手机号 / 邮箱"
            @enter="handleSearch"
          >
            <template #prefix-icon>
              <SearchIcon />
            </template>
          </t-input>
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

        <div class="toolbar-field">
          <span class="toolbar-field__label">代理等级</span>
          <t-select
            v-model="selectedAgentLevelId"
            class="unified-control"
            clearable
            filterable
            placeholder="全部等级"
            :options="agentLevelOptions"
          />
        </div>
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <div>
          <h3 class="table-panel__title">代理商数据</h3>
          <p class="table-panel__desc">展示代理账号资料、合作等级、团队规模与累计返佣，用于日常分销运营管理。</p>
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
        <template #username="{ row }">
          <div class="primary-cell">
            <div class="primary-cell__title-row">
              <span class="primary-cell__title">{{ row.username }}</span>
              <t-tag class="status-chip" theme="primary" variant="light" size="small" shape="round">#{{ row.user_id }}</t-tag>
            </div>
            <span class="primary-cell__sub">{{ row.real_name || row.phone || row.email || '未补充联系方式' }}</span>
          </div>
        </template>

        <template #agent_level_name="{ row }">
          <div class="level-cell">
            <span class="code-pill">{{ row.agent_level_name || '未设置' }}</span>
            <span class="muted-text">等级 ID：{{ row.agent_level_id || '—' }}</span>
          </div>
        </template>

        <template #invite_code="{ row }">
          <span class="mono-pill">{{ row.invite_code || '—' }}</span>
        </template>

        <template #region="{ row }">
          <span class="region-text">{{ row.region || '未设置' }}</span>
        </template>

        <template #team="{ row }">
          <div class="metric-stack">
            <span>直属 {{ row.direct_user_count }}</span>
            <span>团队 {{ row.team_user_count }}</span>
          </div>
        </template>

        <template #commission="{ row }">
          <div class="metric-stack metric-stack--money">
            <span>累计 ¥{{ formatMoney(row.total_commission_amount) }}</span>
            <span>可提现 ¥{{ formatMoney(row.withdrawable_commission_amount) }}</span>
          </div>
        </template>

        <template #balance="{ row }">
          <span class="money-text">¥{{ formatMoney(row.balance) }}</span>
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
            <t-popconfirm content="确认删除该代理商？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无代理商数据" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="640px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建代理商' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="用户 ID" name="user_id">
            <t-input-number v-model="formData.user_id" :min="1" theme="normal" />
          </t-form-item>
          <t-form-item label="代理等级" name="agent_level_id">
            <t-select v-model="formData.agent_level_id" :options="agentLevelOptions" placeholder="请选择代理等级" />
          </t-form-item>
          <t-form-item label="邀请码" name="invite_code">
            <t-input v-model="formData.invite_code" placeholder="请输入邀请码" maxlength="32" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-switch">
              <t-radio-button value="active">启用</t-radio-button>
              <t-radio-button value="disabled">禁用</t-radio-button>
            </t-radio-group>
          </t-form-item>
        </div>
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
  createAgent,
  deleteAgent,
  getAgentDetail,
  getAgentLevelList,
  getAgentList,
  updateAgent,
  type AgentInfo,
  type AgentLevelInfo,
  type AgentListQuery,
  type AgentRequest,
} from '@/api/user'

defineOptions({ name: 'UserPartnersAgents' })

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
const tableData = ref<AgentInfo[]>([])
const agentLevels = ref<AgentLevelInfo[]>([])
const selectedAgentLevelId = ref<number | undefined>()

const filters = reactive<AgentListQuery>({
  page: 1,
  page_size: 10,
  status: '',
  agent_level_id: undefined,
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

const initFormData = (): AgentRequest => ({
  user_id: 0,
  agent_level_id: 0,
  invite_code: '',
  status: 'active',
})

const formData = reactive<AgentRequest>(initFormData())

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const statusLabelMap: Record<string, string> = {
  active: '启用',
  disabled: '禁用',
}

const rules: Record<string, FormRule[]> = {
  user_id: [{ required: true, message: '请输入用户 ID', type: 'error', trigger: 'blur' }],
  agent_level_id: [{ required: true, message: '请选择代理等级', type: 'error', trigger: 'change' }],
  invite_code: [{ required: true, message: '请输入邀请码', type: 'error', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const columns: PrimaryTableCol<AgentInfo>[] = [
  { colKey: 'username', title: '代理账号', minWidth: 220 },
  { colKey: 'agent_level_name', title: '代理等级', minWidth: 160 },
  { colKey: 'invite_code', title: '邀请码', width: 140 },
  { colKey: 'region', title: '地域', width: 110 },
  { colKey: 'team', title: '团队规模', width: 130 },
  { colKey: 'commission', title: '佣金概览', minWidth: 180 },
  { colKey: 'balance', title: '账户余额', width: 120, align: 'right' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 140, fixed: 'right' },
]

const activeFilterLabel = computed(() => {
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  const level = agentLevels.value.find((item) => item.id === filters.agent_level_id)
  if (level) return level.name
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增代理商' : '编辑代理商'))

const agentLevelOptions = computed(() =>
  agentLevels.value.map((item) => ({
    label: item.name,
    value: item.id,
  })),
)

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.status = query.status || ''
  filters.agent_level_id = query.agent_level_id ? Number(query.agent_level_id) : undefined
  filters.keyword = query.keyword || ''
  selectedAgentLevelId.value = filters.agent_level_id
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
  if (filters.agent_level_id) query.agent_level_id = String(filters.agent_level_id)
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

async function loadAgentLevels() {
  try {
    const data = await getAgentLevelList({ page: 1, page_size: 100 })
    agentLevels.value = data.items || []
  } catch {
    agentLevels.value = []
  }
}

async function loadAgents() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getAgentList({
      page: filters.page,
      page_size: filters.page_size,
      status: filters.status || undefined,
      agent_level_id: filters.agent_level_id,
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
    errorMessage.value = (error as Error)?.message || '加载代理商列表失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadAgentLevels(), loadAgents()])
}

async function handleSearch() {
  filters.page = 1
  filters.agent_level_id = selectedAgentLevelId.value
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.status = ''
  filters.agent_level_id = undefined
  filters.keyword = ''
  selectedAgentLevelId.value = undefined
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
    const data = await getAgentDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      user_id: Number(data.user_id || 0),
      agent_level_id: Number(data.agent_level_id || 0),
      invite_code: data.invite_code || '',
      status: data.status || 'active',
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载代理商详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return

  const payload: AgentRequest = {
    user_id: Number(formData.user_id || 0),
    agent_level_id: Number(formData.agent_level_id || 0),
    invite_code: formData.invite_code.trim(),
    status: formData.status,
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createAgent(payload)
      MessagePlugin.success('代理商创建成功')
    } else {
      await updateAgent(editingId.value, payload)
      MessagePlugin.success('代理商更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadAgents()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存代理商失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleDelete(row: AgentInfo) {
  try {
    await deleteAgent(row.id)
    MessagePlugin.success(`已删除代理商 ${row.username}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadAgents()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除代理商失败')
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
    await loadAgents()
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

.toolbar__grid--agents {
  grid-template-columns: minmax(260px, 2fr) repeat(2, minmax(180px, 1fr));
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
.level-cell,
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
.muted-text,
.time-text,
.region-text,
.metric-stack span {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.code-pill,
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

.code-pill {
  border: 1px solid #bbf7d0;
  background: #ecfdf5;
  color: #15803d;
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

.metric-stack--money span {
  color: #334155;
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

@media (max-width: 1200px) {
  .table-panel__head,
  .toolbar__header {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar__grid--agents,
  .form-grid {
    grid-template-columns: 1fr;
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
