<template>
  <div class="distribution-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">代理结算单</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">统一查看代理结算批次、结算区间、应付金额与状态流转，支持创建草稿、确认打款、作废和维护备注。</p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/partners/commissions')">查看佣金记录</t-button>
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增结算单
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按代理商、结算状态、结算区间和关键词筛选批次，用于财务确认与打款核销。</p>
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

      <div class="toolbar__grid toolbar__grid--settlements">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索结算单号 / 备注"
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
          <span class="toolbar-field__label">结算状态</span>
          <t-select
            v-model="filters.status"
            class="unified-control"
            clearable
            placeholder="全部状态"
            :options="statusOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">开始日期</span>
          <t-input v-model="filters.start_date" class="unified-control" clearable placeholder="YYYY-MM-DD" />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">结束日期</span>
          <t-input v-model="filters.end_date" class="unified-control" clearable placeholder="YYYY-MM-DD" />
        </div>
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <div>
          <h3 class="table-panel__title">结算批次</h3>
          <p class="table-panel__desc">展示结算对象、结算周期、佣金汇总、扣减与应付金额，方便按批次推进确认和打款。</p>
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
        <template #settlement_no="{ row }">
          <div class="primary-cell">
            <span class="primary-cell__title mono-text">{{ row.settlement_no }}</span>
            <span class="primary-cell__sub">代理：{{ row.agent_name }}</span>
          </div>
        </template>

        <template #period="{ row }">
          <div class="primary-cell">
            <span class="primary-cell__title">{{ formatDate(row.period_start) }}</span>
            <span class="primary-cell__sub">至 {{ formatDate(row.period_end) }}</span>
          </div>
        </template>

        <template #commission_count="{ row }">
          <span class="count-pill">{{ row.commission_count }} 笔</span>
        </template>

        <template #commission_total="{ row }">
          <span class="money-text">¥{{ formatMoney(row.commission_total) }}</span>
        </template>

        <template #deduction_total="{ row }">
          <span class="money-text money-text--muted">¥{{ formatMoney(row.deduction_total) }}</span>
        </template>

        <template #payable_total="{ row }">
          <span class="money-text money-text--strong">¥{{ formatMoney(row.payable_total) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :class="['status-tag', `status-tag--${row.status || 'default'}`]" theme="default" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || row.status || '未知' }}
          </t-tag>
        </template>

        <template #confirmed_by_name="{ row }">
          <div class="primary-cell">
            <span class="primary-cell__title">{{ row.confirmed_by_name || '未确认' }}</span>
            <span class="primary-cell__sub">{{ row.confirmed_at ? formatDateTime(row.confirmed_at) : '—' }}</span>
          </div>
        </template>

        <template #paid_at="{ row }">
          <span class="time-text">{{ row.paid_at ? formatDateTime(row.paid_at) : '未打款' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #operation="{ row }">
          <t-space size="small" break-line>
            <t-link v-if="canConfirm(row.status)" theme="primary" hover="color" @click="handleConfirm(row)">确认</t-link>
            <t-link v-if="canPay(row.status)" theme="primary" hover="color" @click="handlePay(row)">打款</t-link>
            <t-link v-if="canCancel(row.status)" theme="warning" hover="color" @click="handleCancel(row)">作废</t-link>
            <t-link v-if="canEdit(row.status)" theme="primary" hover="color" @click="openEdit(row.id)">编辑</t-link>
            <t-popconfirm v-if="canDelete(row.status)" content="确认删除该结算单？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无结算单" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="760px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建结算单' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item v-if="dialogMode === 'create'" label="代理商" name="agent_id">
            <t-select v-model="formData.agent_id" :options="agentOptions" placeholder="请选择代理商" filterable />
          </t-form-item>
          <t-form-item v-if="dialogMode === 'create'" label="结算单号" name="settlement_no">
            <t-input v-model="formData.settlement_no" placeholder="可留空自动生成" maxlength="64" />
          </t-form-item>
          <t-form-item label="结算开始日期" name="period_start">
            <t-input v-model="formData.period_start" placeholder="YYYY-MM-DD" />
          </t-form-item>
          <t-form-item label="结算结束日期" name="period_end">
            <t-input v-model="formData.period_end" placeholder="YYYY-MM-DD" />
          </t-form-item>
          <t-form-item label="扣减金额" name="deduction_total">
            <t-input-number v-model="formData.deduction_total" :min="0" :step="100" theme="normal" />
          </t-form-item>
        </div>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="formData.remark" placeholder="请输入备注" :maxlength="255" autosize />
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
  cancelSettlement,
  confirmSettlement,
  createSettlement,
  deleteSettlement,
  getAgentList,
  getSettlementDetail,
  getSettlementList,
  paySettlement,
  updateSettlement,
  type AgentInfo,
  type SettlementInfo,
  type SettlementListQuery,
  type SettlementRequest,
} from '@/api/user'

defineOptions({ name: 'UserPartnersSettlements' })

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
const tableData = ref<SettlementInfo[]>([])
const agents = ref<AgentInfo[]>([])
const selectedAgentId = ref<number | undefined>()

const filters = reactive<SettlementListQuery>({
  page: 1,
  page_size: 10,
  agent_id: undefined,
  status: '',
  start_date: '',
  end_date: '',
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

const initFormData = (): SettlementRequest => ({
  agent_id: undefined,
  settlement_no: '',
  period_start: '',
  period_end: '',
  deduction_total: 0,
  remark: '',
  commission_ids: [],
})

const formData = reactive<SettlementRequest>(initFormData())

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已确认', value: 'confirmed' },
  { label: '已打款', value: 'paid' },
  { label: '已作废', value: 'cancelled' },
]

const statusLabelMap: Record<string, string> = {
  draft: '草稿',
  confirmed: '已确认',
  paid: '已打款',
  cancelled: '已作废',
}

const rules: Record<string, FormRule[]> = {
  agent_id: [{ required: true, message: '请选择代理商', type: 'error', trigger: 'change' }],
  period_start: [{ required: true, message: '请输入开始日期', type: 'error', trigger: 'blur' }],
  period_end: [{ required: true, message: '请输入结束日期', type: 'error', trigger: 'blur' }],
}

const columns: PrimaryTableCol<SettlementInfo>[] = [
  { colKey: 'settlement_no', title: '结算单号 / 代理商', minWidth: 220 },
  { colKey: 'period', title: '结算周期', minWidth: 180 },
  { colKey: 'commission_count', title: '佣金笔数', width: 110, align: 'center' },
  { colKey: 'commission_total', title: '佣金汇总', width: 120, align: 'right' },
  { colKey: 'deduction_total', title: '扣减金额', width: 120, align: 'right' },
  { colKey: 'payable_total', title: '应付金额', width: 120, align: 'right' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'confirmed_by_name', title: '确认信息', minWidth: 160 },
  { colKey: 'paid_at', title: '打款时间', width: 180 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 240, fixed: 'right' },
]

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增结算单' : '编辑结算单'))

const agentOptions = computed(() =>
  agents.value.map((item) => ({
    label: `${item.username} / ${item.agent_level_name || '未设等级'}`,
    value: item.id,
  })),
)

const activeFilterLabel = computed(() => {
  const agent = agents.value.find((item) => item.id === filters.agent_id)
  if (agent) return agent.username
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
  filters.start_date = query.start_date || ''
  filters.end_date = query.end_date || ''
  filters.keyword = query.keyword || ''
  selectedAgentId.value = filters.agent_id
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.agent_id) query.agent_id = String(filters.agent_id)
  if (filters.status) query.status = filters.status
  if (filters.start_date) query.start_date = filters.start_date
  if (filters.end_date) query.end_date = filters.end_date
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

function canConfirm(status: string) {
  return status === 'draft'
}

function canPay(status: string) {
  return status === 'confirmed'
}

function canCancel(status: string) {
  return status === 'draft' || status === 'confirmed'
}

function canEdit(status: string) {
  return status === 'draft'
}

function canDelete(status: string) {
  return status === 'draft' || status === 'cancelled'
}

async function loadAgents() {
  try {
    const data = await getAgentList({ page: 1, page_size: 100 })
    agents.value = data.items || []
  } catch {
    agents.value = []
  }
}

async function loadSettlements() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getSettlementList({
      page: filters.page,
      page_size: filters.page_size,
      agent_id: filters.agent_id,
      status: filters.status || undefined,
      start_date: filters.start_date || undefined,
      end_date: filters.end_date || undefined,
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
    errorMessage.value = (error as Error)?.message || '加载结算单失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadAgents(), loadSettlements()])
}

async function handleSearch() {
  filters.page = 1
  filters.agent_id = selectedAgentId.value
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.agent_id = undefined
  filters.status = ''
  filters.start_date = ''
  filters.end_date = ''
  filters.keyword = ''
  selectedAgentId.value = undefined
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
    const data = await getSettlementDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      period_start: formatDateInput(data.period_start),
      period_end: formatDateInput(data.period_end),
      deduction_total: Number(data.deduction_total || 0),
      remark: data.remark || '',
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载结算单详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return

  const payload: SettlementRequest = {
    period_start: normalizeDateInput(formData.period_start),
    period_end: normalizeDateInput(formData.period_end),
    deduction_total: Number(formData.deduction_total || 0),
    remark: formData.remark?.trim() || undefined,
  }

  if (dialogMode.value === 'create') {
    payload.agent_id = Number(formData.agent_id || 0)
    payload.settlement_no = formData.settlement_no?.trim() || undefined
    payload.commission_ids = []
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createSettlement(payload)
      MessagePlugin.success('结算单创建成功')
    } else {
      await updateSettlement(editingId.value, payload)
      MessagePlugin.success('结算单更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadSettlements()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存结算单失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleConfirm(row: SettlementInfo) {
  try {
    await confirmSettlement(row.id, { remark: '管理端确认结算单' })
    MessagePlugin.success(`结算单 ${row.settlement_no} 已确认`)
    await loadSettlements()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '确认结算单失败')
  }
}

async function handlePay(row: SettlementInfo) {
  try {
    await paySettlement(row.id, { remark: '管理端确认打款' })
    MessagePlugin.success(`结算单 ${row.settlement_no} 已打款`)
    await loadSettlements()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '结算打款失败')
  }
}

async function handleCancel(row: SettlementInfo) {
  try {
    await cancelSettlement(row.id, { remark: '管理端作废结算单' })
    MessagePlugin.success(`结算单 ${row.settlement_no} 已作废`)
    await loadSettlements()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '作废结算单失败')
  }
}

async function handleDelete(row: SettlementInfo) {
  try {
    await deleteSettlement(row.id)
    MessagePlugin.success(`已删除结算单 ${row.settlement_no}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadSettlements()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除结算单失败')
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

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('zh-CN')
}

function formatDateInput(value: string) {
  if (!value) return ''
  return value.slice(0, 10)
}

function normalizeDateInput(value: string) {
  return value.trim().length === 10 ? `${value.trim()}T00:00:00+08:00` : value.trim()
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadSettlements()
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

.toolbar__grid--settlements {
  grid-template-columns: minmax(220px, 2fr) repeat(4, minmax(150px, 1fr));
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

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
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

.count-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
  font-size: 12px;
  font-weight: 600;
}

.money-text {
  color: #0284c7;
  font-weight: 700;
}

.money-text--muted {
  color: #64748b;
}

.money-text--strong {
  color: var(--color-primary);
}

.mono-text {
  font-family: var(--hs-font-mono);
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
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.1);
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

:deep(.status-tag--draft) {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

:deep(.status-tag--confirmed) {
  color: #1d4ed8;
  background: #eff6ff;
  border-color: #bfdbfe;
}

:deep(.status-tag--paid) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.status-tag--cancelled) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

@media (max-width: 1280px) {
  .toolbar__grid--settlements,
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
