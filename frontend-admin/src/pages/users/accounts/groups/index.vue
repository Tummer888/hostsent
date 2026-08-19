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
        <p class="list-header__subtitle">统一管理用户组织、部门与客户分组，支持筛选、创建、编辑与删除。</p>
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
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">按状态和关键词快速定位组织节点，保持与用户列表一致的操作密度。</p>
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
        <div>
          <h3 class="table-panel__title">用户组列表</h3>
          <p class="table-panel__desc">展示组织名称、编码、排序和启用状态，支持直接进入弹窗编辑。</p>
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

        <template #created_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #operation="{ row }">
          <t-space size="small">
            <t-link theme="primary" hover="color" @click="openEdit(row.id)">编辑</t-link>
            <t-popconfirm content="确认删除该用户组？" @confirm="handleDelete(row)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无用户组数据" />
        </template>
      </t-table>
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

defineOptions({ name: 'UserAccountsGroups' })

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
const tableData = ref<UserGroupInfo[]>([])

const filters = reactive<UserGroupListQuery>({
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

const initFormData = (): UserGroupRequest => ({
  name: '',
  code: '',
  description: '',
  status: 'active',
  sort_order: 0,
})

const formData = reactive<UserGroupRequest>(initFormData())

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

const columns: PrimaryTableCol<UserGroupInfo>[] = [
  { colKey: 'id', title: 'ID', width: 96 },
  { colKey: 'name', title: '用户组', minWidth: 260 },
  { colKey: 'code', title: '编码', minWidth: 180 },
  { colKey: 'sort_order', title: '排序', width: 90, align: 'center' },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 140, fixed: 'right' },
]

const activeFilterLabel = computed(() => {
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const dialogTitle = computed(() => (dialogMode.value === 'create' ? '新增用户组' : '编辑用户组'))

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

async function loadGroups() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getUserGroupList({
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
    description: formData.description.trim(),
    status: formData.status,
    sort_order: Number(formData.sort_order || 0),
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
  await loadGroups()
})
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
  border: 1px solid #bbf7d0;
  border-radius: 999px;
  background: #ecfdf5;
  color: #15803d;
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

:deep(.group-table .t-table) {
  border-color: #dcfce7;
}

:deep(.group-table .t-table__header th) {
  color: var(--color-muted-foreground);
  background: #f8fffb;
  font-weight: 600;
  border-bottom-color: #dcfce7;
}

:deep(.group-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: #f0fdf4;
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
  border-color: #dcfce7;
  background: #ffffff;
}

:deep(.group-table .t-pagination__number.t-is-current) {
  color: #15803d;
  border-color: #bbf7d0;
  background: #ecfdf5;
  font-weight: 700;
}

:deep(.group-table .t-pagination__select-input .t-input),
:deep(.group-table .t-pagination__size .t-select__wrap),
:deep(.group-table .t-pagination .t-input) {
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
