<template>
  <div class="distribution-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">返利佣金记录</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">统一查看代理返利流水、冻结截止、状态流转与结算信息，支持冻结、解冻、作废和基础维护。</p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/partners/agents')">查看代理商</t-button>
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增佣金记录
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按代理商、返佣类型、佣金状态和关键词筛选佣金流水，便于财务与运营联动核账。</p>
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

      <div class="toolbar__grid toolbar__grid--commissions">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索代理商 / 下级用户 / 订单号"
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
          <span class="toolbar-field__label">返佣类型</span>
          <t-select
            v-model="filters.commission_type"
            class="unified-control"
            clearable
            placeholder="全部类型"
            :options="commissionTypeOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">佣金状态</span>
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
          <h3 class="table-panel__title">佣金流水</h3>
          <p class="table-panel__desc">展示返佣对象、来源订单、冻结截止、可结算状态和结算结果，用于分销财务对账与返利核销。</p>
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

        <template #source="{ row }">
          <div class="primary-cell">
            <div class="primary-cell__title-row">
              <span class="primary-cell__title">{{ row.subordinate_name || '系统直发' }}</span>
              <t-tag v-if="row.subordinate_id" class="status-chip" theme="primary" variant="light" size="small" shape="round">
                #{{ row.subordinate_id }}
              </t-tag>
            </div>
            <span class="primary-cell__sub mono-text">{{ row.order_no }}</span>
          </div>
        </template>

        <template #source_type="{ row }">
          <span class="type-pill">{{ sourceTypeLabelMap[row.source_type] || row.source_type }}</span>
        </template>

        <template #commission_type="{ row }">
          <span class="type-pill">{{ commissionTypeLabelMap[row.commission_type] || row.commission_type }}</span>
        </template>

        <template #rate="{ row }">
          <span class="rate-text">{{ formatPercent(row.rate) }}</span>
        </template>

        <template #base_amount="{ row }">
          <span class="money-text">¥{{ formatMoney(row.base_amount) }}</span>
        </template>

        <template #amount="{ row }">
          <span class="money-text money-text--strong">¥{{ formatMoney(row.amount) }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :class="['status-tag', `status-tag--${row.status || 'default'}`]" theme="default" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || row.status || '未知' }}
          </t-tag>
        </template>

        <template #freeze_until="{ row }">
          <span class="time-text">{{ row.freeze_until ? formatDateTime(row.freeze_until) : '—' }}</span>
        </template>

        <template #settled_at="{ row }">
          <span class="time-text">{{ row.settled_at ? formatDateTime(row.settled_at) : '未结算' }}</span>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #operation="{ row }">
          <t-space size="small" break-line>
            <t-link v-if="canFreeze(row.status)" theme="primary" hover="color" @click="handleFreeze(row)">冻结</t-link>
            <t-link v-if="canUnfreeze(row.status)" theme="primary" hover="color" @click="handleUnfreeze(row)">解冻</t-link>
            <t-link v-if="canCancel(row.status)" theme="warning" hover="color" @click="handleCancel(row)">作废</t-link>
            <t-link theme="primary" hover="color" @click="openEdit(row.id)">编辑</t-link>
            <t-popconfirm content="确认删除该佣金记录？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无佣金记录" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="760px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建记录' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="代理商" name="agent_id">
            <t-select v-model="formData.agent_id" :options="agentOptions" placeholder="请选择代理商" filterable />
          </t-form-item>
          <t-form-item label="下级用户 ID" name="subordinate_id">
            <t-input-number v-model="formData.subordinate_id" :min="1" theme="normal" />
          </t-form-item>
          <t-form-item label="订单号" name="order_no">
            <t-input v-model="formData.order_no" placeholder="请输入订单号" maxlength="64" />
          </t-form-item>
          <t-form-item label="来源类型" name="source_type">
            <t-select v-model="formData.source_type" :options="sourceTypeOptionsWithoutAll" placeholder="请选择来源类型" />
          </t-form-item>
          <t-form-item label="返佣类型" name="commission_type">
            <t-select v-model="formData.commission_type" :options="commissionTypeOptionsWithoutAll" placeholder="请选择返佣类型" />
          </t-form-item>
          <t-form-item label="返佣比例" name="rate">
            <t-input-number v-model="formData.rate" :min="0" :max="1" :step="0.01" theme="normal" />
          </t-form-item>
          <t-form-item label="佣金状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-switch">
              <t-radio-button value="pending">待处理</t-radio-button>
              <t-radio-button value="frozen">冻结中</t-radio-button>
              <t-radio-button value="available">可结算</t-radio-button>
              <t-radio-button value="settled">已结算</t-radio-button>
              <t-radio-button value="cancelled">已作废</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item label="返佣基数" name="base_amount">
            <t-input-number v-model="formData.base_amount" :min="0" :step="100" theme="normal" />
          </t-form-item>
          <t-form-item label="佣金金额" name="amount">
            <t-input-number v-model="formData.amount" :min="0" :step="100" theme="normal" />
          </t-form-item>
        </div>
        <div class="form-grid">
          <t-form-item label="冻结截止时间" name="freeze_until">
            <t-input v-model="formData.freeze_until" placeholder="例如 2026-08-19 10:30:00，非冻结状态可留空" />
          </t-form-item>
          <t-form-item label="结算时间" name="settled_at">
            <t-input v-model="formData.settled_at" placeholder="例如 2026-08-19 10:30:00，未结算可留空" />
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
  cancelCommission,
  createCommission,
  deleteCommission,
  freezeCommission,
  getAgentList,
  getCommissionDetail,
  getCommissionList,
  unfreezeCommission,
  updateCommission,
  type AgentInfo,
  type CommissionInfo,
  type CommissionListQuery,
  type CommissionRequest,
} from '@/api/user'

defineOptions({ name: 'UserPartnersCommissions' })

type DialogMode = 'create' | 'edit'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const actionLoadingId = ref<number | null>(null)
const errorMessage = ref('')
const dialogVisible = ref(false)
const dialogMode = ref<DialogMode>('create')
const editingId = ref<number>(0)
const formRef = ref<FormInstanceFunctions | null>(null)
const tableData = ref<CommissionInfo[]>([])
const agents = ref<AgentInfo[]>([])
const selectedAgentId = ref<number | undefined>()

const filters = reactive<CommissionListQuery>({
  page: 1,
  page_size: 10,
  agent_id: undefined,
  status: '',
  commission_type: '',
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

const initFormData = (): CommissionRequest => ({
  agent_id: 0,
  subordinate_id: undefined,
  order_no: '',
  source_type: 'order',
  commission_type: 'direct',
  base_amount: 0,
  rate: 0,
  amount: 0,
  status: 'pending',
  freeze_until: '',
  settled_at: '',
  remark: '',
})

const formData = reactive<CommissionRequest>(initFormData())

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '待处理', value: 'pending' },
  { label: '冻结中', value: 'frozen' },
  { label: '可结算', value: 'available' },
  { label: '已结算', value: 'settled' },
  { label: '已作废', value: 'cancelled' },
]

const sourceTypeOptions = [
  { label: '全部来源', value: '' },
  { label: '订单返佣', value: 'order' },
  { label: '续费返佣', value: 'renewal' },
]

const commissionTypeOptions = [
  { label: '全部类型', value: '' },
  { label: '直属返佣', value: 'direct' },
  { label: '间推返佣', value: 'indirect' },
  { label: '续费佣金', value: 'renewal' },
  { label: '自购返利', value: 'self_purchase' },
]

const sourceTypeOptionsWithoutAll = computed(() => sourceTypeOptions.slice(1))
const commissionTypeOptionsWithoutAll = computed(() => commissionTypeOptions.slice(1))

const statusLabelMap: Record<string, string> = {
  pending: '待处理',
  frozen: '冻结中',
  available: '可结算',
  settled: '已结算',
  cancelled: '已作废',
}

const sourceTypeLabelMap: Record<string, string> = {
  order: '订单返佣',
  renewal: '续费返佣',
}

const commissionTypeLabelMap: Record<string, string> = {
  direct: '直属返佣',
  indirect: '间推返佣',
  renewal: '续费佣金',
  self_purchase: '自购返利',
}

const rules: Record<string, FormRule[]> = {
  agent_id: [{ required: true, message: '请选择代理商', type: 'error', trigger: 'change' }],
  order_no: [{ required: true, message: '请输入订单号', type: 'error', trigger: 'blur' }],
  source_type: [{ required: true, message: '请选择来源类型', type: 'error', trigger: 'change' }],
  commission_type: [{ required: true, message: '请选择返佣类型', type: 'error', trigger: 'change' }],
  status: [{ required: true, message: '请选择佣金状态', type: 'error', trigger: 'change' }],
}

const columns: PrimaryTableCol<CommissionInfo>[] = [
  { colKey: 'agent_name', title: '代理商', minWidth: 180 },
  { colKey: 'source', title: '下级用户 / 订单', minWidth: 220 },
  { colKey: 'source_type', title: '来源类型', width: 120 },
  { colKey: 'commission_type', title: '返佣类型', width: 120 },
  { colKey: 'rate', title: '返佣比例', width: 100, align: 'right' },
  { colKey: 'base_amount', title: '返佣基数', width: 120, align: 'right' },
  { colKey: 'amount', title: '佣金金额', width: 120, align: 'right' },
  { colKey: 'status', title: '佣金状态', width: 110 },
  { colKey: 'freeze_until', title: '冻结截止', width: 180 },
  { colKey: 'settled_at', title: '结算时间', width: 180 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 220, fixed: 'right' },
]

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增佣金记录' : '编辑佣金记录'))

const agentOptions = computed(() =>
  agents.value.map((item) => ({
    label: `${item.username} / ${item.agent_level_name || '未设等级'}`,
    value: item.id,
  })),
)

const activeFilterLabel = computed(() => {
  const agent = agents.value.find((item) => item.id === filters.agent_id)
  if (agent) return agent.username
  if (filters.commission_type) return commissionTypeLabelMap[filters.commission_type] || filters.commission_type
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
  filters.commission_type = query.commission_type || ''
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
  if (filters.commission_type) query.commission_type = filters.commission_type
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

function canFreeze(status: string) {
  return status === 'pending'
}

function canUnfreeze(status: string) {
  return status === 'frozen'
}

function canCancel(status: string) {
  return status === 'pending' || status === 'frozen' || status === 'available'
}

async function loadAgents() {
  try {
    const data = await getAgentList({ page: 1, page_size: 100 })
    agents.value = data.items || []
  } catch {
    agents.value = []
  }
}

async function loadCommissions() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getCommissionList({
      page: filters.page,
      page_size: filters.page_size,
      agent_id: filters.agent_id,
      status: filters.status || undefined,
      commission_type: filters.commission_type || undefined,
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
    errorMessage.value = (error as Error)?.message || '加载佣金记录失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadAgents(), loadCommissions()])
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
  filters.commission_type = ''
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
    const data = await getCommissionDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      agent_id: Number(data.agent_id || 0),
      subordinate_id: data.subordinate_id ? Number(data.subordinate_id) : undefined,
      order_no: data.order_no || '',
      source_type: data.source_type || 'order',
      commission_type: data.commission_type || 'direct',
      base_amount: Number(data.base_amount || 0),
      rate: Number(data.rate || 0),
      amount: Number(data.amount || 0),
      status: data.status || 'pending',
      freeze_until: data.freeze_until || '',
      settled_at: data.settled_at || '',
      remark: data.remark || '',
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载佣金详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return

  const payload: CommissionRequest = {
    agent_id: Number(formData.agent_id || 0),
    subordinate_id: formData.subordinate_id ? Number(formData.subordinate_id) : undefined,
    order_no: formData.order_no.trim(),
    source_type: formData.source_type,
    commission_type: formData.commission_type,
    base_amount: Number(formData.base_amount || 0),
    rate: Number(formData.rate || 0),
    amount: Number(formData.amount || 0),
    status: formData.status,
    freeze_until: formData.freeze_until?.trim() || undefined,
    settled_at: formData.settled_at?.trim() || undefined,
    remark: formData.remark?.trim() || undefined,
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createCommission(payload)
      MessagePlugin.success('佣金记录创建成功')
    } else {
      await updateCommission(editingId.value, payload)
      MessagePlugin.success('佣金记录更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadCommissions()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存佣金记录失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleFreeze(row: CommissionInfo) {
  actionLoadingId.value = row.id
  try {
    await freezeCommission(row.id, { remark: '管理端手动冻结' })
    MessagePlugin.success(`佣金 #${row.id} 已冻结`)
    await loadCommissions()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '冻结佣金失败')
  } finally {
    actionLoadingId.value = null
  }
}

async function handleUnfreeze(row: CommissionInfo) {
  actionLoadingId.value = row.id
  try {
    await unfreezeCommission(row.id, { remark: '管理端手动解冻' })
    MessagePlugin.success(`佣金 #${row.id} 已解冻`)
    await loadCommissions()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '解冻佣金失败')
  } finally {
    actionLoadingId.value = null
  }
}

async function handleCancel(row: CommissionInfo) {
  actionLoadingId.value = row.id
  try {
    await cancelCommission(row.id, { remark: '管理端手动作废' })
    MessagePlugin.success(`佣金 #${row.id} 已作废`)
    await loadCommissions()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '作废佣金失败')
  } finally {
    actionLoadingId.value = null
  }
}

async function handleDelete(row: CommissionInfo) {
  try {
    await deleteCommission(row.id)
    MessagePlugin.success(`已删除佣金记录 #${row.id}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadCommissions()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除佣金记录失败')
  }
}

function formatMoney(value: number) {
  return Number(value || 0).toFixed(2)
}

function formatPercent(value: number) {
  return `${(Number(value || 0) * 100).toFixed(2)}%`
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
    await loadCommissions()
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

.toolbar__grid--commissions {
  grid-template-columns: minmax(260px, 2fr) repeat(3, minmax(170px, 1fr));
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
.time-text,
.rate-text {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.type-pill {
  display: inline-flex;
  align-items: center;
  width: fit-content;
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

:deep(.status-tag--pending) {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

:deep(.status-tag--frozen) {
  color: #1d4ed8;
  background: #eff6ff;
  border-color: #bfdbfe;
}

:deep(.status-tag--available) {
  color: #0369a1;
  background: #ecfeff;
  border-color: #a5f3fc;
}

:deep(.status-tag--settled) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.status-tag--cancelled) {
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
  .toolbar__grid--commissions,
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
