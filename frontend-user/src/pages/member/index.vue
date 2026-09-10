<template>
  <div class="member-page">
    <section class="member-hero">
      <div class="hero-left">
        <span class="hero-chip"><UsergroupIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">成员管理</span>
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
          <span class="section-title">成员列表</span>
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

    <!-- 新建/编辑成员 -->
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

    <!-- 权限设置 -->
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

    <!-- 成员操作日志 -->
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

    <!-- 一次性密码提示 -->
    <t-dialog v-model:visible="passwordVisible" header="成员创建成功" width="460px" :footer="false">
      <t-space direction="vertical" size="10px">
        <span>成员「{{ createdUsername }}」已创建，请把以下一次性密码转达给成员，登录后请尽快修改：</span>
        <div class="password-box">{{ createdPassword }}</div>
      </t-space>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, UsergroupIcon } from 'tdesign-icons-vue-next'
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

defineOptions({ name: 'MemberManage' })

const loading = ref(false)
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
.member-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.member-hero,
.member-panel {
  padding: 20px 24px;
  border: 1px solid var(--td-brand-color-2, #e5e7eb);
  border-radius: 12px;
  background: var(--td-bg-color-container, #fff);
}

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
  background: var(--td-brand-color-light, #e8f5e9);
  color: var(--td-brand-color, #16a34a);
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.hero-label {
  font-size: 20px;
  font-weight: 700;
}

.hero-desc {
  max-width: 720px;
  color: var(--td-text-color-secondary, #6b7280);
  font-size: 13px;
  line-height: 1.7;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.panel-head__title {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-title {
  font-size: 16px;
  font-weight: 700;
}

.section-sub {
  color: var(--td-text-color-secondary, #6b7280);
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
  color: var(--td-text-color-secondary, #6b7280);
  font-size: 12px;
}

.muted {
  color: var(--td-text-color-placeholder, #9ca3af);
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-tip {
  color: var(--td-text-color-placeholder, #9ca3af);
  font-size: 12px;
}

.log-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.password-box {
  padding: 12px 14px;
  border: 1px dashed var(--td-brand-color-3, #86efac);
  border-radius: 8px;
  background: var(--td-brand-color-light, #f0fdf4);
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 16px;
  letter-spacing: 1px;
  text-align: center;
}

@media (max-width: 768px) {
  .member-hero,
  .panel-head {
    flex-direction: column;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
