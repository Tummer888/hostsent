<template>
  <div class="group-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">用户组管理</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/accounts/list')">查看用户列表</t-button>
        <t-button class="page-btn" theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增用户组
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <h3 class="toolbar__title">筛选条件</h3>
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

      <div class="toolbar__segment">
        <span class="toolbar-field__label">分组类型</span>
        <t-radio-group
          v-model="filters.is_agent_group"
          variant="default-filled"
          class="type-switch"
          @change="handleTypeChange"
        >
          <t-radio-button value="">全部</t-radio-button>
          <t-radio-button value="false">普通用户组</t-radio-button>
          <t-radio-button value="true">代理用户组</t-radio-button>
        </t-radio-group>
      </div>

      <div class="toolbar__grid toolbar__grid--groups">
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
        <h3 class="table-panel__title">用户组列表</h3>
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
        :pagination="isMobile ? undefined : pagination"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        class="group-table"
        @page-change="handlePageChange"
      >
        <template #id="{ row }">
          <span class="id-cell__value">#{{ row.id }}</span>
        </template>

        <template #name="{ row }">
          <div class="group-cell group-cell--primary">
            <span class="group-cell__name">{{ row.name }}</span>
            <span class="group-cell__desc">{{ row.description || '暂无描述' }}</span>
          </div>
        </template>

        <template #code="{ row }">
          <div class="code-cell">
            <span class="code-pill">{{ row.code }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :class="['status-tag', `status-tag--${row.status || 'default'}`]" theme="default" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || row.status || '未知' }}
          </t-tag>
        </template>

        <template #sort_order="{ row }">
          <span class="sort-text">{{ row.sort_order }}</span>
        </template>

        <template #price_policy_id="{ row }">
          <span v-if="row.price_policy_id" class="sort-text">
            {{ policyNameMap[row.price_policy_id] || `策略 #${row.price_policy_id}` }}
          </span>
          <span v-else class="group-cell__desc">未绑定</span>
        </template>

        <template #is_default="{ row }">
          <t-tag v-if="row.is_default" theme="primary" variant="light" size="small" shape="round">默认组</t-tag>
          <span v-else class="group-cell__desc">—</span>
        </template>

        <template #is_agent_group="{ row }">
          <t-tag v-if="row.is_agent_group" theme="warning" variant="light" size="small" shape="round">代理组</t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">普通组</t-tag>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #operation="{ row }">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([
              { content: '编辑', value: 'edit' },
              { content: '删除', value: 'delete', disabled: () => !!row.is_default, theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="small">
            <t-link theme="primary" hover="color" @click="openEdit(row.id)">编辑</t-link>
            <t-tooltip v-if="row.is_default" content="默认用户组不可删除，请先取消默认标记">
              <t-link theme="default" disabled>删除</t-link>
            </t-tooltip>
            <t-popconfirm v-else content="确认删除该用户组？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无用户组数据" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="620px"
      :confirm-btn="{ content: dialogMode === 'create' ? '创建用户组' : '保存修改', loading: submitting }"
      :on-confirm="handleSubmit"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="用户组名称" name="name">
            <t-input v-model="formData.name" placeholder="例如：华东运营中心" maxlength="64" />
          </t-form-item>
          <t-form-item label="编码" name="code">
            <t-input v-model="formData.code" placeholder="例如：east_ops" maxlength="64" />
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="formData.sort_order" :min="0" :max="9999" theme="normal" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-switch">
              <t-radio-button value="active">启用</t-radio-button>
              <t-radio-button value="disabled">禁用</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item class="form-grid__full" label="折扣策略绑定" name="price_policy_id">
            <t-select
              v-model="formData.price_policy_id"
              :options="policyOptions"
              clearable
              filterable
              placeholder="选择折扣策略（不选则不打折）"
            />
          </t-form-item>
        </div>
        <div class="form-flags">
          <t-checkbox v-model="formData.is_default" :disabled="editingIsDefault">设为默认用户组</t-checkbox>
          <t-checkbox v-model="formData.is_agent_group">标记为代理用户组</t-checkbox>
          <p v-if="editingIsDefault" class="form-tip">默认组只能转移不能取消：在另一个用户组上勾选「设为默认」即可切换。</p>
        </div>
        <t-form-item label="描述" name="description">
          <t-textarea
            v-model="formData.description"
            :maxlength="255"
            :autosize="{ minRows: 4, maxRows: 6 }"
            placeholder="输入该用户组的业务职责、使用范围或归属说明"
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
  createUserGroup,
  deleteUserGroup,
  getUserGroupDetail,
  getUserGroupList,
  updateUserGroup,
  type UserGroupInfo,
  type UserGroupListQuery,
  type UserGroupRequest,
} from '@/api/user'
import { getPricePolicyList } from '@/api/product'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'UserAccountsGroups' })

type DialogMode = 'create' | 'edit'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const { isMobile } = useIsMobile()
const submitting = ref(false)
const errorMessage = ref('')
const dialogVisible = ref(false)
const dialogMode = ref<DialogMode>('create')
const editingId = ref<number>(0)
const formRef = ref<FormInstanceFunctions | null>(null)
const tableData = ref<UserGroupInfo[]>([])

const filters = reactive<UserGroupListQuery>({
  page: 1,
  page_size: 10,
  status: '',
  keyword: '',
  is_agent_group: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50, 100],
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const initFormData = (): UserGroupRequest => ({
  name: '',
  code: '',
  description: '',
  status: 'active',
  sort_order: 0,
  price_policy_id: undefined,
  is_default: false,
  is_agent_group: false,
})

const formData = reactive<UserGroupRequest>(initFormData())

// 折扣策略下拉（P5-06）：用户组绑定后即成为该组用户的折扣来源。
const policyOptions = ref<{ label: string; value: number }[]>([])
const policyNameMap = ref<Record<number, string>>({})

async function loadPolicies() {
  try {
    const data = await getPricePolicyList({ page: 1, page_size: 200, status: 'active' })
    const map: Record<number, string> = {}
    policyOptions.value = (data.items || []).map((item) => {
      map[item.id] = item.name
      return { label: item.name, value: item.id }
    })
    policyNameMap.value = map
  } catch {
    /* 策略加载失败不阻塞用户组列表 */
  }
}

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
  name: [{ required: true, message: '请输入用户组名称', type: 'error', trigger: 'blur' }],
  code: [{ required: true, message: '请输入用户组编码', type: 'error', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const columns = computed<PrimaryTableCol<UserGroupInfo>[]>(() => [
  { colKey: 'id', title: 'ID', width: 96 },
  { colKey: 'name', title: '用户组', minWidth: 260 },
  { colKey: 'code', title: '编码', minWidth: 180 },
  { colKey: 'price_policy_id', title: '折扣策略', width: 120 },
  { colKey: 'is_default', title: '默认组', width: 100, align: 'center' },
  { colKey: 'is_agent_group', title: '分组类型', width: 110, align: 'center' },
  { colKey: 'sort_order', title: '排序', width: 90, align: 'center' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 140, fixed: 'right' },
])

const typeLabelMap: Record<string, string> = {
  true: '代理用户组',
  false: '普通用户组',
}

const activeFilterLabel = computed(() => {
  if (filters.is_agent_group && typeLabelMap[filters.is_agent_group]) {
    return typeLabelMap[filters.is_agent_group]
  }
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增用户组' : '编辑用户组'))

// 编辑的正是当前默认组时禁止取消勾选：默认组是注册/建号的兜底目标，只能转移不能清空。
const editingIsDefault = computed(() => dialogMode.value === 'edit' && Boolean(formData.is_default))

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
  filters.is_agent_group = query.is_agent_group === 'true' || query.is_agent_group === 'false' ? query.is_agent_group : ''
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
  if (filters.keyword) query.keyword = filters.keyword
  if (filters.is_agent_group) query.is_agent_group = filters.is_agent_group
  return query
}

async function replaceRouteQuery() {
  await router.replace({ query: buildQuery() })
}

function resetForm() {
  Object.assign(formData, initFormData())
  editingId.value = 0
}

async function loadGroups() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getUserGroupList({
      page: filters.page,
      page_size: filters.page_size,
      status: filters.status || undefined,
      keyword: filters.keyword || undefined,
      is_agent_group: filters.is_agent_group || undefined,
    })
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    filters.page = data.meta.page
    filters.page_size = data.meta.page_size
  } catch (error) {
    tableData.value = []
    pagination.total = 0
    mobilePage.total = 0
    errorMessage.value = (error as Error)?.message || '加载用户组失败'
  } finally {
    loading.value = false
  }
}

async function handleSearch() {
  filters.page = 1
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleTypeChange() {
  filters.page = 1
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.status = ''
  filters.keyword = ''
  filters.is_agent_group = ''
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
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}


async function reload() {
  await loadGroups()
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
    const data = await getUserGroupDetail(id)
    editingId.value = data.id
    Object.assign(formData, {
      name: data.name,
      code: data.code,
      description: data.description || '',
      status: data.status || 'active',
      sort_order: Number(data.sort_order || 0),
      price_policy_id: data.price_policy_id ?? undefined,
      is_default: Boolean(data.is_default),
      is_agent_group: Boolean(data.is_agent_group),
    })
    dialogVisible.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载用户组详情失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) {
    return
  }

  const payload: UserGroupRequest = {
    name: formData.name.trim(),
    code: formData.code.trim(),
    description: (formData.description || '').trim(),
    status: formData.status,
    sort_order: Number(formData.sort_order || 0),
    price_policy_id: formData.price_policy_id ? Number(formData.price_policy_id) : undefined,
    is_default: Boolean(formData.is_default),
    is_agent_group: Boolean(formData.is_agent_group),
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createUserGroup(payload)
      MessagePlugin.success('用户组创建成功')
    } else {
      await updateUserGroup(editingId.value, payload)
      MessagePlugin.success('用户组更新成功')
    }
    dialogVisible.value = false
    resetForm()
    await loadGroups()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存用户组失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  resetForm()
}

async function handleDelete(row: UserGroupInfo) {
  try {
    await deleteUserGroup(row.id)
    MessagePlugin.success(`已删除用户组 ${row.name}`)
    if (tableData.value.length === 1 && filters.page && filters.page > 1) {
      filters.page -= 1
      pagination.current = filters.page
      await replaceRouteQuery()
      return
    }
    await loadGroups()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除用户组失败')
  }
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    hour12: false,
  })
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadGroups()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  loadPolicies()
  await loadGroups()
})
function handleMobileAction(value: string | number | Record<string, any>, row: UserGroupInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      void openEdit(row.id)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}
</script>

<style scoped lang="css">
.group-page {
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

.list-header__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar {
  padding: 18px 20px;
  background: var(--hs-surface-1);
  border-color: var(--td-brand-color-2);
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

.toolbar__actions {
  display: flex;
  justify-content: flex-end;
}

.toolbar__segment {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.toolbar__grid {
  display: grid;
  gap: 14px;
}

.toolbar__grid--groups {
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
  border-color: var(--td-brand-color-2);
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

.group-cell,
.code-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.group-cell__name,
.id-cell__value {
  color: var(--color-foreground);
  font-weight: 700;
}

.group-cell__desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.code-pill {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--td-brand-color-3);
  border-radius: 999px;
  background: #ecfdf5;
  color: var(--td-brand-color-8);
  font-size: 12px;
  font-weight: 600;
}

.sort-text,
.time-text {
  color: #334155;
  font-size: 12px;
  line-height: 1.6;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-grid__full {
  grid-column: 1 / -1;
}

.form-flags {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0 0 16px;
}

.form-tip {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

:deep(.page-btn.t-button--theme-primary) {
  background-color: var(--color-primary);
  border-color: var(--color-primary);
}

:deep(.page-btn.t-button--theme-primary:hover),
:deep(.page-btn.t-button--theme-primary:focus-visible) {
  background-color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-8);
}

:deep(.page-btn--ghost) {
  color: var(--color-primary);
  border-color: var(--td-brand-color-3);
  background: #ecfdf5;
}

:deep(.page-btn--ghost:hover),
:deep(.page-btn--ghost:focus-visible) {
  color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-4);
  background: var(--td-brand-color-2);
}

:deep(.page-chip.t-tag--primary.t-tag--variant-light) {
  color: var(--td-brand-color-8);
  background: #ecfdf5;
  border-color: var(--td-brand-color-3);
}

:deep(.page-link),
:deep(.page-link.t-link) {
  color: var(--color-primary);
}

:deep(.page-link:hover),
:deep(.page-link.t-link:hover) {
  color: var(--td-brand-color-8);
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

:deep(.group-table .t-table) {
  border-color: var(--td-brand-color-2);
}

:deep(.group-table .t-table__header th) {
  color: var(--color-muted-foreground);
  background: #f8fffb;
  font-weight: 600;
  border-bottom-color: var(--td-brand-color-2);
}

:deep(.group-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: var(--td-brand-color-1);
  vertical-align: middle;
}

:deep(.group-table .t-table__row--hover td) {
  background: rgba(22, 163, 74, 0.03);
}

:deep(.group-table .t-table__pagination) {
  padding-top: 16px;
}

:deep(.group-table .t-pagination__number),
:deep(.group-table .t-pagination__btn) {
  min-width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  border-color: var(--td-brand-color-2);
  background: #ffffff;
}

:deep(.group-table .t-pagination__number.t-is-current) {
  color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-3);
  background: #ecfdf5;
  font-weight: 700;
}

:deep(.group-table .t-pagination__select-input .t-input),
:deep(.group-table .t-pagination__size .t-select__wrap),
:deep(.group-table .t-pagination .t-input) {
  border-radius: var(--hs-radius-md);
  border-color: var(--td-brand-color-2);
  background: #ffffff;
}

:deep(.status-tag) {
  border-radius: var(--hs-radius-xl);
  font-weight: 600;
  border: 1px solid transparent;
}

:deep(.status-tag--active) {
  color: var(--td-brand-color-8);
  background: #ecfdf5;
  border-color: var(--td-brand-color-3);
}

:deep(.status-tag--disabled) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

:deep(.status-switch .t-radio-button),
:deep(.type-switch .t-radio-button) {
  border-color: var(--td-brand-color-2);
  color: var(--color-muted-foreground);
  background: #ffffff;
}

:deep(.status-switch .t-radio-button.t-is-checked),
:deep(.type-switch .t-radio-button.t-is-checked) {
  color: var(--td-brand-color-8);
  background: #ecfdf5;
  border-color: var(--td-brand-color-3);
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

  .toolbar__grid--groups,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .toolbar__actions,
  .list-header__actions {
    justify-content: flex-start;
  }
}
</style>
