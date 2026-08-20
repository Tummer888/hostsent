<template>
  <div class="level-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">代理商等级配置</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">统一维护代理商等级、返佣比例、升级奖励和下级代理能力，支持筛选、创建、编辑与删除。</p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增等级
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按状态和关键词快速定位代理商等级，便于维护不同合作层级的返佣策略。</p>
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

      <div class="toolbar__grid toolbar__grid--levels">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索名称 / 编码 / 描述"
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
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <div>
          <h3 class="table-panel__title">等级列表</h3>
          <p class="table-panel__desc">展示等级权重、返佣比例和代理权限，支持直接在列表侧重管理策略配置。</p>
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
        class="level-table"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="level-cell level-cell--primary">
            <div class="level-cell__title-row">
              <span class="level-cell__name">{{ row.name }}</span>
              <span class="code-pill">{{ row.code }}</span>
            </div>
            <span class="level-cell__desc">{{ row.description || '暂无描述' }}</span>
          </div>
        </template>

        <template #weight="{ row }">
          <span class="weight-pill">权重 {{ row.weight }}</span>
        </template>

        <template #commissions="{ row }">
          <div class="rate-stack">
            <span>直属 {{ formatPercent(row.direct_commission_rate) }}</span>
            <span>间推 {{ formatPercent(row.indirect_commission_rate) }}</span>
            <span>续费 {{ formatPercent(row.renewal_commission_rate) }}</span>
            <span>自购 {{ formatPercent(row.self_purchase_rebate_rate) }}</span>
          </div>
        </template>

        <template #permissions="{ row }">
          <div class="permission-stack">
            <t-tag class="status-chip" theme="success" variant="light" size="small" shape="round" v-if="row.allow_manual_price">可手动定价</t-tag>
            <t-tag class="status-chip" theme="primary" variant="light" size="small" shape="round" v-if="row.allow_sub_agent">
              下级代理 {{ row.max_sub_agent_depth }} 层
            </t-tag>
            <span v-if="!row.allow_manual_price && !row.allow_sub_agent" class="muted-text">基础权限</span>
          </div>
        </template>

        <template #upgrade_reward_amount="{ row }">
          <span class="money-text">¥{{ formatMoney(row.upgrade_reward_amount) }}</span>
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
            <t-popconfirm content="确认删除该代理商等级？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无代理商等级数据" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="760px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建等级' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="等级名称" name="name">
            <t-input v-model="formData.name" placeholder="例如：核心代理" maxlength="64" />
          </t-form-item>
          <t-form-item label="编码" name="code">
            <t-input v-model="formData.code" placeholder="例如：core_agent" maxlength="64" />
          </t-form-item>
          <t-form-item label="等级权重" name="weight">
            <t-input-number v-model="formData.weight" :min="0" :max="9999" theme="normal" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-switch">
              <t-radio-button value="active">启用</t-radio-button>
              <t-radio-button value="disabled">禁用</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item label="直属返佣比例" name="direct_commission_rate">
            <t-input-number v-model="formData.direct_commission_rate" :min="0" :max="1" :step="0.01" theme="normal" />
          </t-form-item>
          <t-form-item label="间推返佣比例" name="indirect_commission_rate">
            <t-input-number v-model="formData.indirect_commission_rate" :min="0" :max="1" :step="0.01" theme="normal" />
          </t-form-item>
          <t-form-item label="续费佣金比例" name="renewal_commission_rate">
            <t-input-number v-model="formData.renewal_commission_rate" :min="0" :max="1" :step="0.01" theme="normal" />
          </t-form-item>
          <t-form-item label="自购返利比例" name="self_purchase_rebate_rate">
            <t-input-number v-model="formData.self_purchase_rebate_rate" :min="0" :max="1" :step="0.01" theme="normal" />
          </t-form-item>
          <t-form-item label="升级奖励金额" name="upgrade_reward_amount">
            <t-input-number v-model="formData.upgrade_reward_amount" :min="0" :step="100" theme="normal" />
          </t-form-item>
          <t-form-item label="下级代理层级" name="max_sub_agent_depth">
            <t-input-number v-model="formData.max_sub_agent_depth" :min="0" :max="10" theme="normal" :disabled="!formData.allow_sub_agent" />
          </t-form-item>
        </div>
        <div class="switch-grid">
          <t-checkbox v-model="formData.allow_manual_price">允许手动定价</t-checkbox>
          <t-checkbox v-model="formData.allow_sub_agent">允许发展下级代理</t-checkbox>
        </div>
        <t-form-item label="描述" name="description">
          <t-textarea
            v-model="formData.description"
            :maxlength="255"
            :autosize="{ minRows: 4, maxRows: 6 }"
            placeholder="输入该代理商等级的合作定位、返佣说明或适用范围"
          />
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
  createAgentLevel,
  deleteAgentLevel,
  getAgentLevelDetail,
  getAgentLevelList,
  updateAgentLevel,
  type AgentLevelInfo,
  type AgentLevelListQuery,
  type AgentLevelRequest,
} from '@/api/user'

defineOptions({ name: 'UserPartnersLevels' })

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
const tableData = ref<AgentLevelInfo[]>([])

const filters = reactive<AgentLevelListQuery>({
  page: 1,
  page_size: 10,
  status: '',
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

const initFormData = (): AgentLevelRequest => ({
  name: '',
  code: '',
  weight: 0,
  direct_commission_rate: 0,
  indirect_commission_rate: 0,
  renewal_commission_rate: 0,
  upgrade_reward_amount: 0,
  self_purchase_rebate_rate: 0,
  allow_manual_price: false,
  allow_sub_agent: false,
  max_sub_agent_depth: 0,
  status: 'active',
  description: '',
})

const formData = reactive<AgentLevelRequest>(initFormData())

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
  name: [{ required: true, message: '请输入等级名称', type: 'error', trigger: 'blur' }],
  code: [{ required: true, message: '请输入等级编码', type: 'error', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const columns: PrimaryTableCol<AgentLevelInfo>[] = [
  { colKey: 'name', title: '等级信息', minWidth: 280 },
  { colKey: 'weight', title: '权重', width: 100, align: 'center' },
  { colKey: 'commissions', title: '返佣比例', minWidth: 220 },
  { colKey: 'permissions', title: '权限能力', minWidth: 200 },
  { colKey: 'upgrade_reward_amount', title: '升级奖励', width: 120, align: 'right' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 140, fixed: 'right' },
]

const activeFilterLabel = computed(() => {
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增代理商等级' : '编辑代理商等级'))

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.status = query.status || ''
  filters.keyword = query.keyword || ''
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
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

async function loadLevels() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getAgentLevelList({
      page: filters.page,
      page_size: filters.page_size,
      status: filters.status || undefined,
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
    errorMessage.value = (error as Error)?.message || '加载代理商等级失败'
  } finally {
    loading.value = false
  }
}

async function handleSearch() {
  filters.page = 1
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.status = ''
  filters.keyword = ''
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
  await loadLevels()
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
    const data = await getAgentLevelDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      name: data.name,
      code: data.code,
      weight: Number(data.weight || 0),
      direct_commission_rate: Number(data.direct_commission_rate || 0),
      indirect_commission_rate: Number(data.indirect_commission_rate || 0),
      renewal_commission_rate: Number(data.renewal_commission_rate || 0),
      upgrade_reward_amount: Number(data.upgrade_reward_amount || 0),
      self_purchase_rebate_rate: Number(data.self_purchase_rebate_rate || 0),
      allow_manual_price: Boolean(data.allow_manual_price),
      allow_sub_agent: Boolean(data.allow_sub_agent),
      max_sub_agent_depth: Number(data.max_sub_agent_depth || 0),
      status: data.status || 'active',
      description: data.description || '',
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载代理商等级详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) {
    return
  }

  const payload: AgentLevelRequest = {
    name: formData.name.trim(),
    code: formData.code.trim(),
    weight: Number(formData.weight || 0),
    direct_commission_rate: Number(formData.direct_commission_rate || 0),
    indirect_commission_rate: Number(formData.indirect_commission_rate || 0),
    renewal_commission_rate: Number(formData.renewal_commission_rate || 0),
    upgrade_reward_amount: Number(formData.upgrade_reward_amount || 0),
    self_purchase_rebate_rate: Number(formData.self_purchase_rebate_rate || 0),
    allow_manual_price: Boolean(formData.allow_manual_price),
    allow_sub_agent: Boolean(formData.allow_sub_agent),
    max_sub_agent_depth: formData.allow_sub_agent ? Number(formData.max_sub_agent_depth || 0) : 0,
    status: formData.status,
    description: formData.description.trim(),
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createAgentLevel(payload)
      MessagePlugin.success('代理商等级创建成功')
    } else {
      await updateAgentLevel(editingId.value, payload)
      MessagePlugin.success('代理商等级更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadLevels()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存代理商等级失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleDelete(row: AgentLevelInfo) {
  try {
    await deleteAgentLevel(row.id)
    MessagePlugin.success(`已删除代理商等级 ${row.name}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadLevels()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除代理商等级失败')
  }
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    hour12: false,
  })
}

function formatPercent(value: number) {
  return `${(Number(value || 0) * 100).toFixed(2)}%`
}

function formatMoney(value: number) {
  return Number(value || 0).toFixed(2)
}

watch(
  () => formData.allow_sub_agent,
  (enabled) => {
    if (!enabled) {
      formData.max_sub_agent_depth = 0
    }
  },
)

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadLevels()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  await loadLevels()
})
</script>

<style scoped lang="css">
.level-page {
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

.toolbar__grid--levels {
  grid-template-columns: minmax(260px, 2fr) minmax(180px, 1fr);
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

.level-cell,
.rate-stack,
.permission-stack {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.level-cell__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.level-cell__name {
  color: var(--color-foreground);
  font-weight: 700;
}

.level-cell__desc,
.muted-text,
.time-text {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.code-pill,
.weight-pill {
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

.weight-pill {
  border: 1px solid #d9f99d;
  background: #f7fee7;
  color: #3f6212;
}

.rate-stack span,
.money-text {
  color: #334155;
  font-size: 12px;
  line-height: 1.6;
}

.money-text {
  font-weight: 600;
}

.switch-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
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

:deep(.page-chip.t-tag--primary.t-tag--variant-light) {
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

:deep(.level-table .t-table) {
  border-color: #dcfce7;
}

:deep(.level-table .t-table__header th) {
  color: var(--color-muted-foreground);
  background: #f8fffb;
  font-weight: 600;
  border-bottom-color: #dcfce7;
}

:deep(.level-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: #f0fdf4;
  vertical-align: middle;
}

:deep(.level-table .t-table__row--hover td) {
  background: rgba(22, 163, 74, 0.03);
}

:deep(.level-table .t-table__pagination) {
  padding-top: 16px;
}

:deep(.level-table .t-pagination__number),
:deep(.level-table .t-pagination__btn) {
  min-width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  border-color: #dcfce7;
  background: #ffffff;
}

:deep(.level-table .t-pagination__number.t-is-current) {
  color: #15803d;
  border-color: #bbf7d0;
  background: #ecfdf5;
  font-weight: 700;
}

:deep(.level-table .t-pagination__select-input .t-input),
:deep(.level-table .t-pagination__size .t-select__wrap),
:deep(.level-table .t-pagination .t-input) {
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

:deep(.status-chip.t-tag) {
  width: fit-content;
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
}

@media (max-width: 768px) {
  .list-header,
  .toolbar__header {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar__grid--levels,
  .form-grid,
  .switch-grid {
    grid-template-columns: 1fr;
  }

  .toolbar__actions,
  .list-header__actions {
    justify-content: flex-start;
  }
}
</style>
