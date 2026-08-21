<template>
  <div class="user-list-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <div class="list-header__icon">
            <UserIcon size="22" aria-hidden="true" />
          </div>
          <h2 class="list-header__title">用户列表</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
      </div>
      <div class="list-header__actions">
        <t-button class="page-btn page-btn--ghost" variant="outline" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增用户
        </t-button>
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
          <span class="toolbar-field__label">登录 IP 归属地</span>
          <t-select
            v-model="filters.last_login_ip_region"
            class="unified-control"
            clearable
            filterable
            placeholder="全部归属地"
            :options="regionOptions"
          />
        </div>
      </div>
    </section>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <h3 class="table-panel__title">用户数据</h3>
      </div>

      <div v-if="errorMessage" class="error-banner" role="alert">
        <ErrorCircleIcon size="16" aria-hidden="true" />
        <span>{{ errorMessage }}</span>
        <t-link class="page-link" theme="primary" hover="color" @click="reload">重试</t-link>
      </div>

      <div
        ref="tableDragRef"
        :class="['table-drag-scroll', { 'table-drag-scroll--dragging': isTableDragging }]"
        @mousedown="handleTableDragStart"
      >
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
          <template #title-id>
            <button type="button" class="id-sort-button" @click="toggleIdSort">
              <span>ID</span>
              <component :is="sortOrder === 'asc' ? ArrowUpIcon : ArrowDownIcon" size="14" aria-hidden="true" />
            </button>
          </template>

          <template #id="{ row }">
            <div class="id-cell">
              <span class="id-cell__value">{{ row.id }}</span>
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
                {{ formatRoleLabel(role) }}
              </t-tag>
            </div>
          </template>

          <template #user_group_name="{ row }">
            <span class="text-muted">{{ row.user_group_name || '未分组' }}</span>
          </template>

          <template #last_login_ip="{ row }">
            <div class="ip-cell">
              <span class="ip-cell__value">{{ row.last_login_ip || '未记录' }}</span>
              <span class="ip-cell__region" :class="{ 'ip-cell__region--muted': !row.last_login_ip_region }">
                {{ row.last_login_ip_region || '未解析' }}
              </span>
            </div>
          </template>

          <template #oauth_provider="{ row }">
            <div class="oauth-cell">
              <t-popup v-for="provider in oauthProviders" :key="provider.key" :content="provider.label" placement="top">
                <span
                  :class="[
                    'oauth-provider-icon',
                    `oauth-provider-icon--${provider.key}`,
                    { 'oauth-provider-icon--inactive': !hasOauthProvider(row, provider.key) },
                  ]"
                >
                  <component :is="provider.icon" size="18" aria-hidden="true" />
                </span>
              </t-popup>
            </div>
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

          <template #total_consume_amount="{ row }">
            <div class="money-cell">
              <span class="money">{{ formatMoney(row.total_consume_amount) }}</span>
            </div>
          </template>

          <template #last_login_at="{ row }">
            <span class="time-text" :class="{ 'time-text--muted': !row.last_login_at }">
              {{ row.last_login_at ? formatDateTime(row.last_login_at) : '未登录' }}
            </span>
          </template>

          <template #action="{ row }">
            <div class="action-cell">
              <t-space size="small">
                <t-link theme="primary" hover="color" @click="goUserDetail(row)">详情</t-link>
                <t-link theme="primary" hover="color" @click="handleRecharge(row)">充值</t-link>
                <t-link theme="primary" hover="color" @click="handleImpersonate(row)">登录</t-link>
                <t-popconfirm
                  :content="row.status === 'active' ? '确认冻结该用户？' : '确认解冻该用户？'"
                  @confirm="toggleStatus(row)"
                >
                  <t-link :theme="row.status === 'active' ? 'warning' : 'primary'" hover="color">
                    {{ row.status === 'active' ? '冻结' : '解冻' }}
                  </t-link>
                </t-popconfirm>
              </t-space>
            </div>
          </template>

          <template #empty>
            <t-empty description="当前筛选条件下暂无用户数据" />
          </template>
        </t-table>
      </div>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      header="新增用户"
      width="620px"
      :confirm-btn="{ content: '创建用户', theme: 'success', loading: submitting }"
      :on-confirm="handleCreateUser"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="用户 ID" name="id">
            <t-input v-model="formData.id" type="number" placeholder="可自定义用户 ID" />
          </t-form-item>

          <t-form-item label="用户名" name="username">
            <t-input v-model="formData.username" placeholder="登录账号，唯一不可重复" maxlength="50" />
          </t-form-item>

          <t-form-item label="邮箱" name="email">
            <t-input v-model="formData.email" placeholder="example@domain.com" />
          </t-form-item>
          <t-form-item label="手机号" name="phone">
            <t-input v-model="formData.phone" placeholder="11位手机号" maxlength="11" />
          </t-form-item>
          <t-form-item label="初始密码" name="password">
            <t-input v-model="formData.password" type="password" placeholder="建议包含字母与数字，至少8位" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-radio-group">
              <t-radio-button value="active">正常</t-radio-button>
              <t-radio-button value="disabled">冻结</t-radio-button>
            </t-radio-group>
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  AddIcon,
  ArrowDownIcon,
  ArrowUpIcon,
  ErrorCircleIcon,
  LogoAndroidIcon,
  LogoAppleFilledIcon,
  LogoGithubFilledIcon,
  LogoQqIcon,
  LogoWechatStrokeIcon,
  RefreshIcon,
  SearchIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { useUserStore } from '@/store/modules/user'
import { createUser, getRegionStats, getUserList, impersonateUser, updateUserStatus, type RegionStatItem, type UserCreateRequest, type UserInfo, type UserListQuery } from '@/api/user'

defineOptions({ name: 'UserAccountsList' })

type SortOrder = 'asc' | 'desc'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<UserInfo[]>([])
const regionItems = ref<RegionStatItem[]>([])
const sortOrder = ref<SortOrder>('desc')
const tableDragRef = ref<HTMLElement | null>(null)
const activeTableScrollRef = ref<HTMLElement | null>(null)
const isTableDragging = ref(false)
const dragState = {
  startX: 0,
  startScrollLeft: 0,
}

const filters = reactive<UserListQuery>({
  page: 1,
  page_size: 10,
  status: '',
  filter: '',
  last_login_ip_region: '',
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

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstanceFunctions | null>(null)
const formData = reactive<UserCreateRequest>({
  id: undefined,
  username: '',
  email: '',
  phone: '',
  password: '',
  status: 'active',
  role_ids: [],
})

const rules: Record<string, FormRule[]> = {
  id: [
    {
      validator: (value) => !value || (Number.isInteger(Number(value)) && Number(value) > 0),
      message: '用户ID需为正整数',
      type: 'error',
    },
  ],
  username: [
    { required: true, message: '请输入用户名', type: 'error', trigger: 'blur' },
    { min: 3, message: '用户名至少 3 个字符', type: 'error', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', type: 'error', trigger: 'blur' },
    { email: true, message: '请输入正确的邮箱格式', type: 'error', trigger: 'blur' },
  ],
  phone: [
    { required: true, message: '请输入手机号', type: 'error', trigger: 'blur' },
    { pattern: /^\d{11}$/, message: '手机号需为11位数字', type: 'error', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入初始密码', type: 'error', trigger: 'blur' },
    { min: 8, message: '密码至少需要 8 位', type: 'error', trigger: 'blur' },
  ],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

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

const statusLabelMap: Record<string, string> = {
  active: '正常',
  disabled: '冻结',
  pending: '待审核',
  cancelled: '已注销',
}

const roleLabelMap: Record<string, string> = {
  super_admin: '超级管理员',
  admin: '管理员',
  user: '普通用户',
  agent: '代理',
  member: '会员',
  finance: '财务',
  operator: '运营',
  support: '客服',
  guest: '访客',
  unassigned: '未分配',
}

const oauthProviders = [
  { key: 'wechat', label: '微信', icon: LogoWechatStrokeIcon },
  { key: 'qq', label: 'QQ', icon: LogoQqIcon },
  { key: 'github', label: 'GitHub', icon: LogoGithubFilledIcon },
  { key: 'apple', label: 'Apple', icon: LogoAppleFilledIcon },
  { key: 'android', label: 'Android', icon: LogoAndroidIcon },
] as const

const regionOptions = computed(() => [
  { label: '全部归属地', value: '' },
  ...regionItems.value.map((item) => ({ label: `${item.region} (${item.count})`, value: item.region })),
])

const activeFilterLabel = computed(() => {
  if (filters.filter === 'today') return '今日新增'
  if (filters.filter === 'pending_real_name') return '待实名认证'
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.last_login_ip_region) return filters.last_login_ip_region
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const sortedTableData = computed(() => {
  const orderFactor = sortOrder.value === 'asc' ? 1 : -1
  return [...tableData.value].sort((left, right) => (Number(left.id || 0) - Number(right.id || 0)) * orderFactor)
})

const columns: PrimaryTableCol<UserInfo>[] = [
  { colKey: 'id', title: 'ID', width: 92 },
  { colKey: 'username', title: '账号信息', minWidth: 260 },
  { colKey: 'real_name', title: '实名信息', minWidth: 220 },
  { colKey: 'balance', title: '账户余额', width: 130, align: 'right' },
  { colKey: 'total_consume_amount', title: '总消费金额', width: 150, align: 'right' },
  { colKey: 'role', title: '角色', minWidth: 180 },
  { colKey: 'user_group_name', title: '用户组', minWidth: 180 },
  { colKey: 'last_login_ip', title: '登录 IP', minWidth: 220 },
  { colKey: 'oauth_provider', title: '第三方登录', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'last_login_at', title: '最近登录', width: 180 },
  { colKey: 'action', title: '操作', width: 260, fixed: 'right' },
]

function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.status = query.status || ''
  filters.filter = query.filter || ''
  filters.last_login_ip_region = query.last_login_ip_region || ''
  filters.keyword = query.keyword || ''
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function hasOauthProvider(row: UserInfo, provider: string) {
  if (Array.isArray(row.oauth_providers) && row.oauth_providers.length > 0) {
    return row.oauth_providers.includes(provider)
  }
  return row.oauth_provider === provider
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
  if (filters.filter) query.filter = filters.filter
  if (filters.last_login_ip_region) query.last_login_ip_region = filters.last_login_ip_region
  if (filters.keyword) query.keyword = filters.keyword
  return query
}

function toggleIdSort() {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
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
      last_login_ip_region: filters.last_login_ip_region || undefined,
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
  filters.last_login_ip_region = ''
  filters.keyword = ''
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
  return ['unassigned']
}

function formatRoleLabel(role: string) {
  const normalizedRole = String(role || '').trim().toLowerCase()
  return roleLabelMap[normalizedRole] || roleLabelMap[role] || role || '未分配'
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

function handleRecharge(row: UserInfo) {
  MessagePlugin.info(`充值功能开发中 - 用户: ${row.username}`)
}

async function handleImpersonate(row: UserInfo) {
  if (row.status !== 'active') {
    MessagePlugin.warning('仅可代登录正常状态用户')
    return
  }
  try {
    const res = await impersonateUser({ user_id: row.id })
    userStore.token = res.token
    userStore.userInfo = {
      id: res.user_info.id,
      name: res.user_info.username,
      username: res.user_info.username,
      role: res.user_info.role,
      roles: res.user_info.roles?.length ? res.user_info.roles : [res.user_info.role],
      email: res.user_info.email,
      phone: res.user_info.phone,
      status: res.user_info.status,
    }
    MessagePlugin.success(`已代登录用户 ${row.username}`)
    await router.push('/')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '代登录失败')
  }
}

function resolveTableScrollContainer() {
  const root = tableDragRef.value
  if (!root) return null

  const candidates = root.querySelectorAll<HTMLElement>('.t-table__content, .t-table__body, .t-table, .t-table__inner, .t-table__header')
  for (const candidate of candidates) {
    if (candidate.scrollWidth > candidate.clientWidth) {
      return candidate
    }
  }

  return root.scrollWidth > root.clientWidth ? root : null
}

function handleTableDragStart(event: MouseEvent) {
  if (event.button !== 0) return
  const target = event.target as HTMLElement | null
  if (target?.closest('a,button,input,textarea,[role="button"],.t-link,.t-button,.t-input,.t-tag,.t-popup')) return
  const container = resolveTableScrollContainer()
  if (!container) return
  activeTableScrollRef.value = container
  isTableDragging.value = true
  dragState.startX = event.clientX
  dragState.startScrollLeft = container.scrollLeft
  document.body.classList.add('user-table-dragging')
  window.addEventListener('mousemove', handleTableDragging)
  window.addEventListener('mouseup', handleTableDragEnd)
  window.addEventListener('mouseleave', handleTableDragEnd)
  event.preventDefault()
}

function handleTableDragging(event: MouseEvent) {
  if (!isTableDragging.value || !activeTableScrollRef.value) return
  const deltaX = event.clientX - dragState.startX
  activeTableScrollRef.value.scrollLeft = dragState.startScrollLeft - deltaX
  event.preventDefault()
}

function handleTableDragEnd() {
  activeTableScrollRef.value = null
  if (!isTableDragging.value) return
  isTableDragging.value = false
  document.body.classList.remove('user-table-dragging')
  window.removeEventListener('mousemove', handleTableDragging)
  window.removeEventListener('mouseup', handleTableDragEnd)
  window.removeEventListener('mouseleave', handleTableDragEnd)
}

function initFormData(): UserCreateRequest {
  return {
    username: '',
    email: '',
    phone: '',
    password: '',
    status: 'active',
    role_ids: [],
  }
}

function openCreate() {
  Object.assign(formData, initFormData())
  formRef.value?.clearValidate?.()
  dialogVisible.value = true
}

async function handleCreateUser() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  submitting.value = true
  try {
    await createUser({
      id: formData.id ? Number(formData.id) : undefined,
      username: formData.username,
      email: formData.email,
      phone: formData.phone,
      password: formData.password,
      status: formData.status,
    })
    MessagePlugin.success('用户创建成功')
    dialogVisible.value = false
    filters.page = 1
    pagination.current = 1
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '创建用户失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  formRef.value?.clearValidate?.()
}

async function toggleStatus(row: UserInfo) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await updateUserStatus(row.id, { status: newStatus })
    MessagePlugin.success(newStatus === 'active' ? '用户已解冻' : '用户已冻结')
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '操作失败')
  }
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

onBeforeUnmount(() => {
  handleTableDragEnd()
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

.list-header__icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, #22c55e, #16a34a);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 10px rgba(34, 197, 94, 0.25);
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

.id-sort-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
}

.id-sort-button:hover {
  color: var(--td-brand-color);
}

.table-panel {
  padding: 16px;
  background: var(--hs-surface-1);
  border-color: #dcfce7;
}

.table-drag-scroll {
  overflow-x: auto;
  cursor: grab;
}

.table-drag-scroll--dragging {
  cursor: grabbing;
}

.table-drag-scroll--dragging :deep(*) {
  user-select: none;
}

.table-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
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
.money-cell,
.ip-cell {
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

.oauth-cell {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.oauth-provider-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  transition: color 0.2s ease, opacity 0.2s ease;
}

.oauth-provider-icon--inactive {
  color: #cbd5e1;
}

.oauth-provider-icon--wechat:not(.oauth-provider-icon--inactive) {
  color: #07c160;
}

.oauth-provider-icon--qq:not(.oauth-provider-icon--inactive) {
  color: #12b7f5;
}

.oauth-provider-icon--github:not(.oauth-provider-icon--inactive),
.oauth-provider-icon--apple:not(.oauth-provider-icon--inactive) {
  color: #111827;
}

.oauth-provider-icon--android:not(.oauth-provider-icon--inactive) {
  color: #34a853;
}

.action-cell {
  display: flex;
  gap: 6px;
  align-items: center;
}

.ip-cell__value {
  color: #0f172a;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.ip-cell__region {
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.ip-cell__region--muted {
  color: var(--color-muted-foreground);
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
  font-size: 15px;
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


:deep(.user-table .t-table) {
  min-width: 1450px;
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

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.status-radio-group {
  width: 220px;
}

.status-radio-group :deep(.t-radio-button) {
  min-width: 96px;
  text-align: center;
}
</style>
