<template>
  <div class="user-list-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <h2 class="list-header__title">用户列表</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
        <p class="list-header__subtitle">
          支持按状态、地域、快捷条件与关键词检索，数据实时来自后端接口。
        </p>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="router.push('/users/overview')">返回总览</t-button>
        <t-button class="page-btn" theme="primary" :loading="loading" @click="reload">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新列表
        </t-button>
      </div>
    </header>

    <section class="toolbar surface-card">
      <div class="toolbar__header">
        <div>
          <h3 class="toolbar__title">筛选条件</h3>
          <p class="toolbar__desc">复用当前后台表单风格，快速定位目标用户并调整列表排序。</p>
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

      <div class="toolbar__grid">
        <div class="toolbar-field toolbar-field--keyword">
          <span class="toolbar-field__label">关键词</span>
          <t-input
            v-model="filters.keyword"
            class="unified-control"
            clearable
            placeholder="搜索用户名 / 姓名 / 邮箱 / 手机号"
            @enter="handleSearch"
          >
            <template #prefix-icon>
              <SearchIcon />
            </template>
          </t-input>
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">用户状态</span>
          <t-select
            v-model="filters.status"
            class="unified-control"
            clearable
            filterable
            placeholder="全部状态"
            :options="statusOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">快捷筛选</span>
          <t-select
            v-model="filters.filter"
            class="unified-control"
            clearable
            placeholder="快捷筛选"
            :options="quickFilterOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">地域分布</span>
          <t-select
            v-model="filters.region"
            class="unified-control"
            clearable
            filterable
            placeholder="全部地域"
            :options="regionOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">排序字段</span>
          <t-select
            v-model="sortField"
            class="unified-control"
            placeholder="选择排序字段"
            :options="sortFieldOptions"
          />
        </div>

        <div class="toolbar-field">
          <span class="toolbar-field__label">排序顺序</span>
          <t-radio-group v-model="sortOrder" variant="default-filled" class="sort-order-group">
            <t-radio-button value="desc">降序</t-radio-button>
            <t-radio-button value="asc">升序</t-radio-button>
          </t-radio-group>
        </div>
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <div>
          <h3 class="table-panel__title">用户数据</h3>
          <p class="table-panel__desc">展示用户主档、联系信息、所属地域、账户状态与最近活跃情况。</p>
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
        :data="sortedTableData"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        class="user-table"
        @page-change="handlePageChange"
      >
        <template #id="{ row }">
          <div class="id-cell">
            <span class="id-cell__value">#{{ row.id }}</span>
          </div>
        </template>

        <template #username="{ row }">
          <div class="user-cell user-cell--primary">
            <t-link class="user-link" theme="primary" hover="color" @click="goUserDetail(row)">
              {{ row.username }}
            </t-link>
            <div class="copy-row" v-if="row.email">
              <span class="copy-row__value copy-row__value--email">{{ row.email }}</span>
              <t-popup content="复制邮箱" placement="top">
                <t-tag class="copy-tag" theme="primary" variant="light" size="small" shape="round" @click="copyText(row.email, '邮箱')">
                  复制
                </t-tag>
              </t-popup>
            </div>
          </div>
        </template>

        <template #real_name="{ row }">
          <div class="user-cell">
            <span class="user-cell__name user-cell__name--secondary">{{ row.real_name || '待补充' }}</span>
            <div class="copy-row" v-if="row.phone">
              <span class="copy-row__value copy-row__value--phone">{{ row.phone }}</span>
              <t-popup content="复制手机号" placement="top">
                <t-tag class="copy-tag" theme="primary" variant="light" size="small" shape="round" @click="copyText(row.phone, '手机号')">
                  复制
                </t-tag>
              </t-popup>
            </div>
          </div>
        </template>

        <template #role="{ row }">
          <div class="role-tags">
            <t-tag
              v-for="role in resolveRoles(row)"
              :key="role"
              class="role-tag"
              theme="primary"
              variant="light"
              size="small"
              shape="round"
            >
              {{ role }}
            </t-tag>
          </div>
        </template>

        <template #region="{ row }">
          <span class="region-text">{{ row.region || '未分配' }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :class="['status-tag', `status-tag--${row.status || 'default'}`]" theme="default" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || row.status || '未知' }}
          </t-tag>
        </template>

        <template #balance="{ row }">
          <div class="money-cell">
            <span class="money">{{ formatMoney(row.balance) }}</span>
          </div>
        </template>

        <template #created_at="{ row }">
          <span class="time-text time-text--created">{{ formatDateTime(row.created_at) }}</span>
        </template>

        <template #last_login_at="{ row }">
          <span class="time-text" :class="{ 'time-text--muted': !row.last_login_at }">
            {{ row.last_login_at ? formatDateTime(row.last_login_at) : '未登录' }}
          </span>
        </template>

        <template #empty>
          <t-empty description="当前筛选条件下暂无用户数据" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ErrorCircleIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getRegionStats, getUserList, type RegionStatItem, type UserInfo, type UserListQuery } from '@/api/user'

defineOptions({ name: 'UserAccountsList' })

type SortField = 'id' | 'created_at' | 'balance'
type SortOrder = 'asc' | 'desc'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<UserInfo[]>([])
const regionItems = ref<RegionStatItem[]>([])
const sortField = ref<SortField>('id')
const sortOrder = ref<SortOrder>('desc')

const filters = reactive<UserListQuery>({
  page: 1,
  page_size: 10,
  status: '',
  filter: '',
  region: '',
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

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '正常', value: 'active' },
  { label: '冻结', value: 'disabled' },
  { label: '待审核', value: 'pending' },
  { label: '已注销', value: 'cancelled' },
]

const quickFilterOptions = [
  { label: '今日新增', value: 'today' },
  { label: '待实名', value: 'pending_real_name' },
]

const sortFieldOptions = [
  { label: 'ID', value: 'id' },
  { label: '注册时间', value: 'created_at' },
  { label: '账户余额', value: 'balance' },
]

const statusLabelMap: Record<string, string> = {
  active: '正常',
  disabled: '冻结',
  pending: '待审核',
  cancelled: '已注销',
}

const regionOptions = computed(() => [
  { label: '全部地域', value: '' },
  ...regionItems.value.map((item) => ({ label: `${item.region} (${item.count})`, value: item.region })),
])

const activeFilterLabel = computed(() => {
  if (filters.filter === 'today') return '今日新增'
  if (filters.filter === 'pending_real_name') return '待实名认证'
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.region) return filters.region
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const sortedTableData = computed(() => {
  const orderFactor = sortOrder.value === 'asc' ? 1 : -1
  return [...tableData.value].sort((left, right) => {
    if (sortField.value === 'created_at') {
      return (new Date(left.created_at).getTime() - new Date(right.created_at).getTime()) * orderFactor
    }
    if (sortField.value === 'balance') {
      return (Number(left.balance || 0) - Number(right.balance || 0)) * orderFactor
    }
    return (Number(left.id || 0) - Number(right.id || 0)) * orderFactor
  })
})

const columns: PrimaryTableCol<UserInfo>[] = [
  { colKey: 'id', title: 'ID', width: 96 },
  { colKey: 'username', title: '账号信息', minWidth: 250 },
  { colKey: 'real_name', title: '实名信息', minWidth: 220 },
  { colKey: 'role', title: '角色', minWidth: 180 },
  { colKey: 'region', title: '地域', width: 120 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'balance', title: '账户余额', width: 130, align: 'right' },
  { colKey: 'created_at', title: '注册时间', width: 180 },
  { colKey: 'last_login_at', title: '最近登录', width: 180 },
]

function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.status = query.status || ''
  filters.filter = query.filter || ''
  filters.region = query.region || ''
  filters.keyword = query.keyword || ''
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
  if (filters.filter) query.filter = filters.filter
  if (filters.region) query.region = filters.region
  if (filters.keyword) query.keyword = filters.keyword
  return query
}

async function replaceRouteQuery() {
  await router.replace({ query: buildQuery() })
}

async function loadRegions() {
  try {
    const data = await getRegionStats()
    regionItems.value = data.items || []
  } catch {
    regionItems.value = []
  }
}

async function loadUsers() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getUserList({
      page: filters.page,
      page_size: filters.page_size,
      status: filters.status || undefined,
      filter: filters.filter || undefined,
      region: filters.region || undefined,
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
    errorMessage.value = (error as Error)?.message || '加载用户列表失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  await Promise.all([loadRegions(), loadUsers()])
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
  filters.filter = ''
  filters.region = ''
  filters.keyword = ''
  sortField.value = 'id'
  sortOrder.value = 'desc'
  pagination.current = 1
  pagination.pageSize = 10
  await replaceRouteQuery()
}

async function reload() {
  await loadAll()
}

async function handlePageChange(pageInfo: PageInfo) {
  filters.page = pageInfo.current
  filters.page_size = pageInfo.pageSize
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  await replaceRouteQuery()
}

function resolveRoles(row: UserInfo) {
  if (Array.isArray(row.roles) && row.roles.length) return row.roles
  if (row.role) return [row.role]
  return ['未分配']
}

function formatMoney(value: number) {
  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(value || 0))
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    hour12: false,
  })
}

async function copyText(value: string, label: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(`${label}已复制`)
  } catch {
    MessagePlugin.error(`${label}复制失败`)
  }
}

function goUserDetail(row: UserInfo) {
  router.push({
    path: '/users/accounts/detail',
    query: { id: String(row.id) },
  })
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadUsers()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  await loadAll()
})
</script>

<style scoped lang="css">
.user-list-page {
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
  grid-template-columns: minmax(260px, 2fr) repeat(5, minmax(150px, 1fr));
  gap: 14px;
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

.sort-order-group {
  width: 100%;
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

.id-cell,
.user-cell,
.money-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.user-cell--primary {
  gap: 8px;
}

.id-cell__value {
  color: var(--color-foreground);
  font-weight: 700;
}

.user-cell__name {
  color: #166534;
  font-weight: 700;
}

.user-cell__name--secondary {
  color: #0f172a;
  font-weight: 600;
}

.copy-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.copy-row__value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

.copy-row__value--email {
  color: #475569;
}

.copy-row__value--phone {
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.region-text {
  color: #334155;
  font-weight: 500;
}

.money {
  font-variant-numeric: tabular-nums;
  color: var(--color-primary);
  font-weight: 700;
}

.time-text {
  color: #334155;
  font-size: 12px;
  line-height: 1.6;
}

.time-text--created {
  color: #0f172a;
}

.time-text--muted {
  color: var(--color-muted-foreground);
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
:deep(.role-tag.t-tag--primary.t-tag--variant-light),
:deep(.copy-tag.t-tag--primary.t-tag--variant-light) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.copy-tag) {
  cursor: pointer;
  opacity: 0;
  transform: translateX(-2px);
  pointer-events: none;
  transition: opacity var(--hs-duration-fast), transform var(--hs-duration-fast), background-color var(--hs-duration-fast), border-color var(--hs-duration-fast), color var(--hs-duration-fast);
}

.copy-row:hover :deep(.copy-tag),
.copy-row:focus-within :deep(.copy-tag) {
  opacity: 1;
  transform: translateX(0);
  pointer-events: auto;
}

:deep(.copy-tag:hover) {
  color: #166534;
  background: #dcfce7;
  border-color: #86efac;
}

:deep(.page-link),
:deep(.page-link.t-link),
:deep(.user-link),
:deep(.user-link.t-link) {
  color: var(--color-primary);
}

:deep(.page-link:hover),
:deep(.page-link.t-link:hover),
:deep(.user-link:hover),
:deep(.user-link.t-link:hover) {
  color: #15803d;
}

:deep(.user-link) {
  font-weight: 700;
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

:deep(.sort-order-group .t-radio-button) {
  border-color: #dcfce7;
  color: var(--color-muted-foreground);
  background: #ffffff;
}

:deep(.sort-order-group .t-radio-button.t-is-checked) {
  color: #15803d;
  background: #ecfdf5;
  border-color: #bbf7d0;
}

:deep(.user-table .t-table) {
  border-color: #dcfce7;
}

:deep(.user-table .t-table__header th) {
  color: var(--color-muted-foreground);
  background: #f8fffb;
  font-weight: 600;
  border-bottom-color: #dcfce7;
  transition: background-color var(--hs-duration-fast), color var(--hs-duration-fast);
}

:deep(.user-table .t-table__header th:hover) {
  color: #166534;
  background: #f0fdf4;
}

:deep(.user-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: #f0fdf4;
  vertical-align: middle;
}

:deep(.user-table .t-table__row--hover td) {
  background: rgba(22, 163, 74, 0.03);
}

:deep(.user-table .t-table__pagination) {
  padding-top: 16px;
}

:deep(.user-table .t-pagination) {
  color: var(--color-muted-foreground);
}

:deep(.user-table .t-pagination__number),
:deep(.user-table .t-pagination__btn) {
  min-width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  border-color: #dcfce7;
  background: #ffffff;
  transition: background-color var(--hs-duration-fast), border-color var(--hs-duration-fast), color var(--hs-duration-fast);
}

:deep(.user-table .t-pagination__number:hover),
:deep(.user-table .t-pagination__btn:not(.t-is-disabled):hover) {
  color: #15803d;
  border-color: #bbf7d0;
  background: #f0fdf4;
}

:deep(.user-table .t-pagination__number.t-is-current) {
  color: #15803d;
  border-color: #bbf7d0;
  background: #ecfdf5;
  font-weight: 700;
}

:deep(.user-table .t-pagination__select-input .t-input),
:deep(.user-table .t-pagination__size .t-select__wrap),
:deep(.user-table .t-pagination .t-input) {
  border-radius: var(--hs-radius-md);
  border-color: #dcfce7;
  background: #ffffff;
}

:deep(.user-table .t-pagination .t-input:focus-within),
:deep(.user-table .t-pagination .t-select__wrap:focus-within) {
  border-color: #16a34a;
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.10);
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

:deep(.status-tag--pending) {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

:deep(.status-tag--cancelled),
:deep(.status-tag--default) {
  color: #475569;
  background: #f8fafc;
  border-color: #e2e8f0;
}

@media (max-width: 1400px) {
  .toolbar__grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 1200px) {
  .toolbar__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .table-panel__head {
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

  .toolbar__grid {
    grid-template-columns: 1fr;
  }

  .toolbar__actions,
  .list-header__actions {
    justify-content: flex-start;
  }
}
</style>
