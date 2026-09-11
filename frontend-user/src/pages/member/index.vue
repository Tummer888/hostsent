<template>
  <div class="member-layout">
    <!-- ============ 左侧分组导航 ============ -->
    <aside class="member-sider">
      <button
        class="sider-item"
        :class="{ 'is-active': view === 'overview' }"
        @click="switchView('overview')"
      >
        <DashboardIcon size="16" />
        <span>概览</span>
      </button>

      <div class="sider-group">
        <button
          class="sider-item sider-item--group"
          :class="{ 'is-active': view === 'members' }"
          @click="groupOpen ? null : (groupOpen = true)"
        >
          <UserIcon size="16" />
          <span>用户管理</span>
          <ChevronDownIcon v-if="groupOpen" size="14" class="sider-arrow" />
          <ChevronRightIcon v-else size="14" class="sider-arrow" />
        </button>
        <div v-show="groupOpen" class="sider-sub">
          <button
            class="sider-item sider-item--sub"
            :class="{ 'is-active': view === 'members' }"
            @click="switchView('members')"
          >
            <span>子用户</span>
          </button>
          <button class="sider-item sider-item--sub" @click="onDevelop('消息接收人')">
            <span>消息接收人</span>
          </button>
        </div>
      </div>

      <button class="sider-item" @click="onDevelop('组管理')">
        <FolderIcon size="16" />
        <span>组管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('策略管理')">
        <SecuredIcon size="16" />
        <span>策略管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('角色管理')">
        <UsergroupIcon size="16" />
        <span>角色管理</span>
      </button>
      <button class="sider-item" @click="onDevelop('外部账号接入')">
        <LinkIcon size="16" />
        <span>外部账号接入</span>
      </button>
      <button class="sider-item" @click="onDevelop('操作记录')">
        <TimeIcon size="16" />
        <span>操作记录</span>
      </button>
      <button class="sider-item" @click="onDevelop('密钥报告')">
        <KeyIcon size="16" />
        <span>密钥报告</span>
      </button>
      <button class="sider-item" @click="onDevelop('设置')">
        <SettingIcon size="16" />
        <span>设置</span>
      </button>
      <button class="sider-item" @click="onDevelop('异常行为分析')">
        <ChartBarIcon size="16" />
        <span>异常行为分析</span>
      </button>
    </aside>

    <!-- ============ 右侧内容 ============ -->
    <div class="member-main">
      <div class="breadcrumb">
        <span>用户中心</span>
        <span class="breadcrumb__sep">/</span>
        <span class="breadcrumb__current">{{ view === 'overview' ? '概览' : '子用户' }}</span>
      </div>

      <!-- ---------- 概览 ---------- -->
      <template v-if="view === 'overview'">
        <h2 class="page-title">概览</h2>

        <section class="stat-grid">
          <div class="stat-card">
            <span class="stat-value">{{ pagination.total }}</span>
            <span class="stat-label">用户数</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">0</span>
            <span class="stat-label">用户组</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">0</span>
            <span class="stat-label">自定义权限策略</span>
          </div>
          <div class="stat-card">
            <span class="stat-value">0</span>
            <span class="stat-label">角色数</span>
          </div>
        </section>

        <div class="overview-cols">
          <div class="overview-left">
            <!-- 子用户登录 -->
            <section class="panel">
              <div class="panel-head">
                <h3 class="panel-title">子用户登录</h3>
              </div>
              <div class="login-row">
                <span class="login-row__label">登录链接：</span>
                <span class="login-row__value">{{ loginUrl }}</span>
                <CopyIcon class="copy-icon" @click="copyText(loginUrl, '登录链接已复制')" />
              </div>
              <div class="login-row">
                <span class="login-row__label">账户别名：</span>
                <span class="login-row__value">{{ userStore.userInfo?.username || '-' }}</span>
                <HelpCircleIcon class="help-icon" />
                <t-link theme="primary" hover="color" @click="onDevelop('账户别名设置')">编辑</t-link>
              </div>
            </section>

            <!-- 操作引导 -->
            <section class="panel">
              <div class="panel-head">
                <h3 class="panel-title">操作引导</h3>
              </div>
              <ul class="guide-list">
                <li v-for="g in guideItems" :key="g.label" class="guide-item">
                  <CheckIcon class="guide-check" size="14" :class="{ 'is-done': g.done }" />
                  <span class="guide-label">{{ g.label }}</span>
                  <span class="guide-status" :class="g.done ? 'is-done' : 'is-undone'">
                    <CheckCircleFilledIcon v-if="g.done" size="14" />
                    <ErrorCircleFilledIcon v-else size="14" />
                    {{ g.done ? '已完成' : '未完成' }}
                  </span>
                </li>
              </ul>
            </section>
          </div>

          <div class="overview-right">
            <!-- 快捷访问入口 -->
            <section class="panel">
              <div class="panel-head">
                <h3 class="panel-title">快捷访问入口</h3>
              </div>
              <div class="quick-grid">
                <button class="quick-btn" @click="openQuickCreate">新建子用户</button>
                <button class="quick-btn" @click="onDevelop('新建消息接收人')">新建消息接收人</button>
                <button class="quick-btn" @click="onDevelop('新建角色')">新建角色</button>
                <button class="quick-btn" @click="onDevelop('新建用户组')">新建用户组</button>
                <button class="quick-btn" @click="onDevelop('新建自定义策略')">新建自定义策略</button>
              </div>
            </section>

            <!-- 上次登录 -->
            <section class="panel">
              <div class="panel-head">
                <h3 class="panel-title">上次登录</h3>
              </div>
              <div class="kv-row">
                <span class="kv-label">上次登录时间：</span>
                <span class="kv-value">{{ lastLoginTime }}</span>
              </div>
              <div class="kv-row">
                <span class="kv-label">上次登录IP：</span>
                <span class="kv-value">{{ lastLoginIp }}</span>
              </div>
            </section>

            <!-- 安全报告下载 -->
            <section class="panel">
              <div class="panel-head">
                <h3 class="panel-title">安全报告下载</h3>
              </div>
              <p class="panel-desc">
                下载您所有子用户的状态和密钥使用情况，您可以每四小时创建一次报告。
              </p>
              <t-button variant="outline" size="small" @click="onDevelop('安全报告下载')">
                下载报告
              </t-button>
            </section>
          </div>
        </div>
      </template>

      <!-- ---------- 子用户列表 ---------- -->
      <template v-else>
        <section class="member-hero">
          <div class="hero-left">
            <span class="hero-chip"><UsergroupIcon size="22" /></span>
            <div class="hero-info">
              <span class="hero-label">子用户</span>
              <span class="hero-desc">
                为主账号创建子账号并分配权限。子账号共用主账号的实例、订单与账单，但不能进行充值、提现和实名操作。
              </span>
            </div>
          </div>
          <t-button theme="primary" size="large" @click="openCreate">
            <template #icon><AddIcon /></template>
            新建成员
          </t-button>
        </section>

        <section class="member-panel">
          <div class="panel-head">
            <div class="panel-head__title">
              <span class="section-title">子用户列表</span>
              <span class="section-sub">
                共 {{ pagination.total }} 个成员
                <template v-if="maxSubAccounts > 0">／上限 {{ maxSubAccounts }} 个</template>
              </span>
            </div>
            <div class="panel-head__filters">
              <t-input v-model="filters.keyword" clearable placeholder="搜索用户名 / 邮箱 / 备注" @enter="handleSearch" />
              <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
              <t-button theme="primary" @click="handleSearch">查询</t-button>
              <t-button variant="outline" @click="handleReset">重置</t-button>
            </div>
          </div>

          <t-table
            row-key="id"
            :data="members"
            :columns="columns"
            :loading="loading"
            :pagination="pagination"
            cell-empty-content="—"
            @page-change="handlePageChange"
          >
            <template #name="{ row }">
              <div class="member-cell">
                <strong>{{ row.name || row.username }}</strong>
                <span class="member-cell__sub">{{ row.username }}<template v-if="row.remark"> · {{ row.remark }}</template></span>
              </div>
            </template>

            <template #permissions="{ row }">
              <t-space size="4px" break-line>
                <t-tag v-for="code in row.permissions" :key="code" theme="primary" variant="light-outline" size="small">
                  {{ permissionLabel(code) }}
                </t-tag>
                <span v-if="!row.permissions?.length" class="muted">未分配权限</span>
              </t-space>
            </template>

            <template #status="{ row }">
              <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small">
                {{ row.status === 'active' ? '启用' : '已禁用' }}
              </t-tag>
            </template>

            <template #last_login_at="{ row }">
              <span class="muted">{{ row.last_login_at ? formatTime(row.last_login_at) : '未登录' }}</span>
            </template>

            <template #operation="{ row }">
              <t-space size="8px">
                <t-link theme="primary" hover="color" @click="openPermissions(row)">权限</t-link>
                <t-link theme="primary" hover="color" @click="openLogs(row)">日志</t-link>
                <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
                <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
              </t-space>
            </template>

            <template #empty>
              <t-empty description="还没有成员，点击右上角「新建成员」添加" />
            </template>
          </t-table>
        </section>
      </template>
    </div>

    <!-- ============ 新建/编辑成员 ============ -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editingId ? '编辑成员' : '新建成员'"
      width="580px"
      :confirm-loading="submitting"
      @confirm="handleSubmit"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="用户名" name="username">
            <t-input v-model="form.username" placeholder="登录用户名" :disabled="Boolean(editingId)" />
          </t-form-item>
          <t-form-item label="姓名 / 备注" name="remark">
            <t-input v-model="form.remark" placeholder="如：运维-张三" />
          </t-form-item>
          <t-form-item label="邮箱" name="email">
            <t-input v-model="form.email" placeholder="member@example.com" />
          </t-form-item>
          <t-form-item label="手机号" name="phone">
            <t-input v-model="form.phone" placeholder="选填" />
          </t-form-item>
        </div>
        <t-form-item v-if="!editingId" label="初始密码" name="password">
          <t-input v-model="form.password" type="password" placeholder="留空则自动生成一次性密码" />
        </t-form-item>
        <t-form-item label="权限" name="permissions">
          <t-space direction="vertical" size="8px">
            <t-checkbox v-for="opt in permissionOptions" :key="opt.code" v-model="permissionSelection[opt.code]">
              {{ opt.label }}
            </t-checkbox>
            <span class="form-tip">充值、提现、退款与实名认证权限不会授予子账号。</span>
          </t-space>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- ============ 权限设置 ============ -->
    <t-dialog
      v-model:visible="permVisible"
      :header="`设置权限 · ${currentRow?.name || currentRow?.username || ''}`"
      width="480px"
      :confirm-loading="submitting"
      @confirm="handlePermissionSubmit"
    >
      <t-space direction="vertical" size="10px">
        <t-checkbox v-for="opt in permissionOptions" :key="opt.code" v-model="permissionSelection[opt.code]">
          {{ opt.label }}
        </t-checkbox>
        <span class="form-tip">保存后立即生效，子账号需重新进入页面。</span>
      </t-space>
    </t-dialog>

    <!-- ============ 成员操作日志 ============ -->
    <t-dialog v-model:visible="logVisible" :header="`操作日志 · ${currentRow?.username || ''}`" width="720px" :footer="false">
      <t-table
        row-key="id"
        :data="logs"
        :columns="logColumns"
        :loading="logLoading"
        size="small"
        cell-empty-content="—"
      >
        <template #created_at="{ row }">
          <span class="muted">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="该成员暂无操作记录" />
        </template>
      </t-table>
      <div class="log-pagination">
        <t-pagination
          v-model:current="logPagination.current"
          v-model:page-size="logPagination.pageSize"
          :total="logPagination.total"
          :page-size-options="[10, 20, 50]"
          @change="loadLogs"
        />
      </div>
    </t-dialog>

    <!-- ============ 一次性密码提示 ============ -->
    <t-dialog v-model:visible="passwordVisible" header="成员创建成功" width="460px" :footer="false">
      <t-space direction="vertical" size="10px">
        <span>成员「{{ createdUsername }}」已创建，请把以下一次性密码转达给成员，登录后请尽快修改：</span>
        <div class="password-box">{{ createdPassword }}</div>
      </t-space>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import {
  AddIcon,
  ChartBarIcon,
  CheckCircleFilledIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CopyIcon,
  DashboardIcon,
  ErrorCircleFilledIcon,
  FolderIcon,
  HelpCircleIcon,
  KeyIcon,
  LinkIcon,
  SecuredIcon,
  SettingIcon,
  TimeIcon,
  UserIcon,
  UsergroupIcon,
} from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  createMember,
  deleteMember,
  getMemberList,
  getMemberLogs,
  setMemberPermissions,
  updateMember,
  type MemberInfo,
  type MemberOperationLog,
  type PermissionOption,
} from '@/api/member'
import { useUserStore } from '@/store'

defineOptions({ name: 'MemberManage' })

const userStore = useUserStore()

// ========== 视图切换 ==========
type ViewKey = 'overview' | 'members'
const view = ref<ViewKey>('overview')
const groupOpen = ref(true)

function switchView(next: ViewKey) {
  view.value = next
  if (!loaded) void loadMembers()
}

function onDevelop(name: string) {
  MessagePlugin.info(`${name}功能开发中`)
}

// ========== 概览 ==========
const loginUrl = computed(() => `${window.location.origin}/login`)

const lastLoginTime = ref('-')
const lastLoginIp = ref('-')

const guideItems = computed(() => [
  { label: '为主账号开启MFA', done: false },
  { label: '不要为主账号创建AK密钥', done: true },
  { label: '使用子用户进行日常工作', done: pagination.total > 0 },
  { label: '为子用户开启MFA', done: false },
  { label: '使用用户组进行日常管理', done: false },
  { label: '创建并绑定自定义策略', done: false },
])

async function copyText(value: string, successMessage: string) {
  if (!value || value === '-') {
    MessagePlugin.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(successMessage)
  } catch {
    MessagePlugin.warning('复制失败，请手动选择内容')
  }
}

// ========== 成员列表 ==========
const loading = ref(false)
const loaded = ref(false)
const submitting = ref(false)
const members = ref<MemberInfo[]>([])
const permissionOptions = ref<PermissionOption[]>([])
const permissionSelection = reactive<Record<string, boolean>>({})
const maxSubAccounts = ref(0)

const filters = reactive({ keyword: '', status: '' })
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '已禁用', value: 'disabled' },
]

const columns: PrimaryTableCol<MemberInfo>[] = [
  { colKey: 'name', title: '成员', minWidth: 200 },
  { colKey: 'permissions', title: '权限', minWidth: 260 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'last_login_at', title: '最近登录', width: 170 },
  { colKey: 'operation', title: '操作', width: 220, fixed: 'right' },
]

const logColumns: PrimaryTableCol<MemberOperationLog>[] = [
  { colKey: 'created_at', title: '时间', width: 170 },
  { colKey: 'module', title: '模块', width: 100 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'target', title: '对象', minWidth: 200, ellipsis: true },
  { colKey: 'ip', title: 'IP', width: 140 },
]

const formRef = ref<FormInstanceFunctions>()
const formVisible = ref(false)
const editingId = ref<number | null>(null)

const form = reactive({
  username: '',
  email: '',
  phone: '',
  remark: '',
  password: '',
})

const rules: Record<string, FormRule[]> = {
  username: [{ required: true, message: '请输入用户名', type: 'error' }],
  email: [
    { required: true, message: '请输入邮箱', type: 'error' },
    { email: true, message: '邮箱格式不正确', type: 'error' },
  ],
}

const permVisible = ref(false)
const logVisible = ref(false)
const logLoading = ref(false)
const logs = ref<MemberOperationLog[]>([])
const logPagination = reactive({ current: 1, pageSize: 10, total: 0 })
const currentRow = ref<MemberInfo | null>(null)

const passwordVisible = ref(false)
const createdUsername = ref('')
const createdPassword = ref('')

async function loadMembers() {
  loading.value = true
  try {
    const { data } = await getMemberList({
      page: pagination.current,
      page_size: pagination.pageSize,
      keyword: filters.keyword || undefined,
      status: filters.status || undefined,
    })
    members.value = data.items || []
    pagination.total = data.meta.total
    maxSubAccounts.value = data.max_sub_accounts || 0
    if (data.permission_options?.length) {
      permissionOptions.value = data.permission_options
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载成员列表失败')
  } finally {
    loading.value = false
    loaded.value = true
  }
}

function handleSearch() {
  pagination.current = 1
  void loadMembers()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  pagination.current = 1
  void loadMembers()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadMembers()
}

function resetPermissionSelection(codes: string[] = []) {
  for (const key of Object.keys(permissionSelection)) {
    delete permissionSelection[key]
  }
  for (const opt of permissionOptions.value) {
    permissionSelection[opt.code] = codes.includes(opt.code)
  }
}

function selectedPermissions(): string[] {
  return Object.entries(permissionSelection)
    .filter(([, checked]) => checked)
    .map(([code]) => code)
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { username: '', email: '', phone: '', remark: '', password: '' })
  resetPermissionSelection()
  formVisible.value = true
}

function openQuickCreate() {
  view.value = 'members'
  openCreate()
}

function openEdit(row: MemberInfo) {
  editingId.value = row.id
  Object.assign(form, {
    username: row.username,
    email: row.email,
    phone: row.phone,
    remark: row.remark,
    password: '',
  })
  resetPermissionSelection(row.permissions)
  formVisible.value = true
}

function openPermissions(row: MemberInfo) {
  currentRow.value = row
  resetPermissionSelection(row.permissions)
  permVisible.value = true
}

async function handleSubmit() {
  const valid = await formRef.value?.validate?.()
  if (valid !== true) return
  submitting.value = true
  try {
    if (editingId.value) {
      await updateMember(editingId.value, { remark: form.remark })
      await setMemberPermissions(editingId.value, selectedPermissions())
      MessagePlugin.success('成员已更新')
    } else {
      const { data } = await createMember({
        username: form.username,
        email: form.email,
        phone: form.phone || undefined,
        remark: form.remark || undefined,
        password: form.password || undefined,
        permissions: selectedPermissions(),
      })
      createdUsername.value = data.member.username
      createdPassword.value = data.password
      passwordVisible.value = true
    }
    formVisible.value = false
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存成员失败')
  } finally {
    submitting.value = false
  }
}

async function handlePermissionSubmit() {
  if (!currentRow.value) return
  submitting.value = true
  try {
    await setMemberPermissions(currentRow.value.id, selectedPermissions())
    MessagePlugin.success('权限已更新')
    permVisible.value = false
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '设置权限失败')
  } finally {
    submitting.value = false
  }
}

function handleDelete(row: MemberInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除成员',
    body: `确认删除成员「${row.username}」？删除后该账号将无法登录，名下数据仍归属主账号。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteMember(row.id)
        MessagePlugin.success('成员已删除')
        dialog.destroy()
        await loadMembers()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '删除成员失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function openLogs(row: MemberInfo) {
  currentRow.value = row
  logPagination.current = 1
  logVisible.value = true
  await loadLogs()
}

async function loadLogs() {
  if (!currentRow.value) return
  logLoading.value = true
  try {
    const { data } = await getMemberLogs(currentRow.value.id, {
      page: logPagination.current,
      page_size: logPagination.pageSize,
    })
    logs.value = data.items || []
    logPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载操作日志失败')
  } finally {
    logLoading.value = false
  }
}

function permissionLabel(code: string): string {
  return permissionOptions.value.find((item) => item.code === code)?.label || code
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  void loadMembers()
})
</script>

<style scoped>
.member-layout {
  display: flex;
  gap: 20px;
  align-items: stretch;
  min-height: calc(100vh - 112px);
}

/* ============ 左侧导航 ============ */
.member-sider {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px 10px;
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
  position: sticky;
  top: 80px;
  min-height: calc(100vh - 112px);
}

.sider-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #475569;
  font-size: 13.5px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.sider-item:hover {
  background: #f1f5f9;
  color: #2563eb;
}

.sider-item.is-active {
  background: #eff6ff;
  color: #2563eb;
  font-weight: 600;
}

.sider-arrow {
  margin-left: auto;
  color: #94a3b8;
}

.sider-sub {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-left: 14px;
}

.sider-item--sub {
  padding: 8px 10px;
  font-size: 13px;
}

.member-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ============ 面包屑 / 标题 ============ */
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #94a3b8;
}

.breadcrumb__current {
  color: #475569;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
}

/* ============ 统计卡片 ============ */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  background: #eef1f5;
  border: 1px solid #eef1f5;
  border-radius: 8px;
  overflow: hidden;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 20px 24px;
  background: #f8fafc;
}

.stat-value {
  font-size: 26px;
  font-weight: 600;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
}

/* ============ 概览两栏 ============ */
.overview-cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 16px;
  align-items: flex-start;
}

.overview-left,
.overview-right {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

/* ============ 通用面板 ============ */
.panel,
.member-hero,
.member-panel {
  background: #fff;
  border: 1px solid var(--td-border-level-1-color, #eef1f5);
  border-radius: 8px;
  padding: 18px 20px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.panel-title {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  color: #1e293b;
}

.panel-desc {
  margin: 0 0 14px;
  font-size: 12.5px;
  line-height: 1.7;
  color: #8b95a8;
}

/* ============ 子用户登录 ============ */
.login-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 0;
  font-size: 13px;
}

.login-row__label {
  color: #64748b;
  flex-shrink: 0;
}

.login-row__value {
  color: #334155;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copy-icon,
.help-icon {
  color: #94a3b8;
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.15s ease;
}

.copy-icon:hover,
.help-icon:hover {
  color: #2563eb;
}

/* ============ 操作引导 ============ */
.guide-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.guide-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 0;
  font-size: 13.5px;
  border-bottom: 1px solid #f1f5f9;
}

.guide-item:last-child {
  border-bottom: none;
}

.guide-check {
  color: #94a3b8;
  flex-shrink: 0;
}

.guide-check.is-done {
  color: #10b981;
}

.guide-label {
  flex: 1;
  color: #334155;
}

.guide-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12.5px;
}

.guide-status.is-done {
  color: #10b981;
}

.guide-status.is-undone {
  color: #f59e0b;
}

/* ============ 快捷入口 ============ */
.quick-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.quick-btn {
  padding: 6px 14px;
  border: none;
  border-radius: 4px;
  background: #eaf2ff;
  color: #2563eb;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.quick-btn:hover {
  background: #d8e8ff;
}

/* ============ 键值行 ============ */
.kv-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 7px 0;
  font-size: 13px;
}

.kv-label {
  color: #64748b;
}

.kv-value {
  color: #334155;
}

/* ============ 子用户列表 ============ */
.member-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.hero-left {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.hero-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: #eff6ff;
  color: #2563eb;
  flex-shrink: 0;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.hero-label {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

.hero-desc {
  max-width: 720px;
  color: #8b95a8;
  font-size: 13px;
  line-height: 1.7;
}

.panel-head__title {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-title {
  font-size: 15px;
  font-weight: 700;
  color: #1e293b;
}

.section-sub {
  color: #8b95a8;
  font-size: 12px;
}

.panel-head__filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.member-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.member-cell__sub {
  color: #8b95a8;
  font-size: 12px;
}

.muted {
  color: #9ca3af;
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-tip {
  color: #9ca3af;
  font-size: 12px;
}

.log-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.password-box {
  padding: 12px 14px;
  border: 1px dashed #86efac;
  border-radius: 8px;
  background: #f0fdf4;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 16px;
  letter-spacing: 1px;
  text-align: center;
}

/* ============ 深色模式 ============ */
.dark .member-sider,
.dark .panel,
.dark .member-hero,
.dark .member-panel {
  background: #141414;
  border-color: #262626;
}

.dark .sider-item {
  color: #cbd5e1;
}

.dark .sider-item:hover {
  background: #1f1f1f;
}

.dark .sider-item.is-active {
  background: #10192e;
}

.dark .page-title,
.dark .panel-title,
.dark .hero-label,
.dark .section-title,
.dark .stat-value {
  color: #e5e7eb;
}

.dark .stat-grid {
  background: #262626;
  border-color: #262626;
}

.dark .stat-card {
  background: #171717;
}

.dark .guide-item {
  border-bottom-color: #262626;
}

.dark .guide-label,
.dark .login-row__value,
.dark .kv-value {
  color: #cbd5e1;
}

/* ============ 响应式 ============ */
@media (max-width: 1180px) {
  .overview-cols {
    grid-template-columns: minmax(0, 1fr);
  }

  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .member-layout {
    flex-direction: column;
  }

  .member-sider {
    width: 100%;
    position: static;
    min-height: auto;
  }

  .member-hero,
  .panel-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
