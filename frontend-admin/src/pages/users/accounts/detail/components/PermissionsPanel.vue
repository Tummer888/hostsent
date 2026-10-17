<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="权限或角色数据采集失败，以下内容可能不完整。"
    />

    <section class="section-block">
      <div class="section-label">客户侧权限</div>
      <p class="section-hint">
        客户侧权限是固定枚举，落 sub_account_permissions；主账号天然拥有全部权限，子账号按勾选授予。
      </p>
      <div v-if="profile?.is_sub_account">
        <div v-if="permissions.length" class="tag-group">
          <t-tag v-for="code in permissions" :key="code" theme="success" variant="light" shape="round">
            {{ customerPermissionLabel(code) }}
          </t-tag>
        </div>
        <div v-else class="empty-state empty-state--compact">该子账号未授予任何客户侧权限</div>
      </div>
      <div v-else class="tag-group">
        <t-tag v-for="opt in customerPermissionOptions" :key="opt.value" theme="primary" variant="light" shape="round">
          {{ opt.label }}
        </t-tag>
        <t-tag theme="warning" variant="light" shape="round">成员管理</t-tag>
      </div>
    </section>

    <section class="section-block">
      <div class="section-label">客户角色绑定</div>
      <p class="section-hint">
        这里是客户角色（roles.scope=user，如「普通用户」）。员工角色（scope=admin） 由员工管理维护，不在此处展示 ——
        客户账号持有员工角色不会获得任何后台权限。
      </p>
      <div v-if="rbacRoles.length" class="tag-group">
        <t-tag v-for="role in rbacRoles" :key="role.id" theme="primary" variant="light" shape="round">
          {{ role.name }}（{{ role.code }}）· {{ roleScopeLabel(role.scope) }}
        </t-tag>
      </div>
      <div v-else class="empty-state empty-state--compact">该账号未绑定任何客户角色</div>
    </section>

    <section v-if="!profile?.is_sub_account" class="section-block">
      <div class="table-card__head">
        <div class="member-head">
          <h3 class="card-title">成员（子账号）</h3>
          <span class="table-card__meta">共 {{ members.length }} 个成员</span>
        </div>
        <t-space size="small">
          <t-button size="small" variant="outline" :loading="membersLoading" @click="loadMembers">
            <template #icon><RefreshIcon /></template>
            刷新
          </t-button>
          <t-button size="small" theme="primary" @click="openCreate">
            <template #icon><AddIcon /></template>
            新建成员
          </t-button>
        </t-space>
      </div>

      <t-table
        row-key="id"
        :data="members"
        :columns="memberColumns"
        :loading="membersLoading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #username="{ row }">
          <div>
            <span class="cell-strong">{{ row.username }}</span>
            <div class="cell-sub">{{ row.remark || row.name || '未实名' }}</div>
          </div>
        </template>
        <template #contact="{ row }">
          <div>{{ row.email || '—' }}</div>
          <div class="cell-sub">{{ row.phone || '未填写' }}</div>
        </template>
        <template #permissions="{ row }">
          <div v-if="row.permissions?.length" class="tag-group">
            <t-tag v-for="code in row.permissions" :key="code" size="small" variant="outline" shape="round">
              {{ customerPermissionLabel(code) }}
            </t-tag>
          </div>
          <span v-else class="cell-sub">未授予权限</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="userStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ userStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #last_login_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.last_login_at) }}</span>
        </template>
        <template #op="{ row }">
          <t-space size="small" break-line>
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" hover="color" @click="openPermissions(row)">权限</t-link>
            <t-link theme="primary" hover="color" @click="openResetPassword(row)">重置密码</t-link>
            <t-link theme="primary" hover="color" @click="openLogs(row)">日志</t-link>
            <t-link v-if="row.status !== 'disabled'" theme="danger" hover="color" @click="handleDisable(row)">禁用</t-link>
            <t-link v-else theme="success" hover="color" @click="handleEnable(row)">启用</t-link>
          </t-space>
        </template>
        <template #empty>
          <t-empty description="该主账号下暂无成员" />
        </template>
      </t-table>
    </section>

    <!-- 新建 / 编辑成员 -->
    <t-dialog
      v-model:visible="formVisible"
      :header="formMode === 'create' ? '新建成员' : '编辑成员'"
      width="560px"
      :confirm-btn="{ loading: formSubmitting }"
      @confirm="submitForm"
    >
      <t-form ref="formRef" :data="form" :rules="formRules" label-align="top" @submit="submitForm">
        <t-form-item label="登录名" name="username">
          <t-input v-model="form.username" :disabled="formMode === 'edit'" placeholder="成员登录用的账号名" />
        </t-form-item>
        <t-form-item label="邮箱" name="email">
          <t-input v-model="form.email" :disabled="formMode === 'edit'" placeholder="用于找回密码" />
        </t-form-item>
        <t-form-item label="手机号" name="phone">
          <t-input v-model="form.phone" :disabled="formMode === 'edit'" placeholder="选填" />
        </t-form-item>
        <t-form-item v-if="formMode === 'create'" label="初始密码" name="password">
          <t-input v-model="form.password" type="password" placeholder="留空则自动生成一次性密码" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-input v-model="form.remark" placeholder="如「运维-张三」，便于区分成员" />
        </t-form-item>
        <template v-if="formMode === 'create'">
          <t-form-item label="初始权限">
            <t-checkbox-group v-model="form.permissions" :options="permissionOptions" />
          </t-form-item>
          <p class="section-hint section-hint--tight">
            资金进出（充值/提现/退款）与实名认证永不授予子账号，此处不列出。
          </p>
        </template>
      </t-form>
    </t-dialog>

    <!-- 设置权限 -->
    <t-dialog
      v-model:visible="permVisible"
      header="设置成员权限"
      width="520px"
      :confirm-btn="{ loading: permSubmitting }"
      @confirm="submitPermissions"
    >
      <p class="section-hint">
        覆盖式保存：勾选即为该成员的全部权限。主账号保留的资金/实名/成员管理权限不在此列。
      </p>
      <t-checkbox-group v-model="permSelection" :options="permissionOptions" />
    </t-dialog>

    <!-- 重置密码结果：一次性密码只在此显示一次 -->
    <t-dialog
      v-model:visible="pwdVisible"
      header="重置密码完成"
      width="440px"
      :footer="false"
    >
      <t-alert theme="warning" message="一次性密码仅显示这一次，请立即复制并转交给成员。" class="degraded-tip" />
      <div class="pwd-row">
        <code class="pwd-value">{{ pwdResult.password }}</code>
        <t-button size="small" variant="outline" @click="copyPassword">复制</t-button>
      </div>
      <p v-if="pwdResult.revoked_sessions > 0" class="section-hint">
        已连带撤销该成员 {{ pwdResult.revoked_sessions }} 个登录会话。
      </p>
    </t-dialog>

    <!-- 成员操作日志 -->
    <t-drawer v-model:visible="logsVisible" header="成员操作日志" size="640px" :footer="false">
      <t-table
        row-key="id"
        :data="logs"
        :columns="logColumns"
        :loading="logsLoading"
        size="small"
        cell-empty-content="—"
      >
        <template #created_at="{ row }">
          <span class="time-text">{{ row.created_at }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无操作记录" />
        </template>
      </t-table>
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next'
import { AddIcon, RefreshIcon } from 'tdesign-icons-vue-next'

import {
  createUserMember,
  disableUserMember,
  getMemberPermissionOptions,
  getUserMemberLogs,
  getUserMembers,
  resetUserMemberPassword,
  setUserMemberPermissions,
  updateUserMember,
} from '@/api/user'
import type {
  SubAccountCreateRequest,
  SubAccountLogInfo,
  SubAccountMemberInfo,
  SubAccountResetPasswordResult,
  UserInfo,
  UserRoleBrief,
} from '@/api/user'
import {
  customerPermissionLabel,
  customerPermissionOptions,
  formatDateTime,
  roleScopeLabel,
  userStatusLabel,
  userStatusTheme,
} from '@/pages/users/constants'

const props = defineProps<{
  userId: number
  profile: UserInfo | null
  rbacRoles: UserRoleBrief[]
  permissions: string[]
  degraded?: boolean
}>()

const members = ref<SubAccountMemberInfo[]>([])
const membersLoading = ref(false)

// 列宽合计贴近容器宽度（约 1140px），避免 fixed 布局把富余宽度摊给短列。
// 已授权限是标签组、宽度需求最大，操作列最后固定。
const memberColumns: PrimaryTableCol<SubAccountMemberInfo>[] = [
  { colKey: 'username', title: '成员', width: 200 },
  { colKey: 'contact', title: '联系方式', width: 230 },
  { colKey: 'permissions', title: '已授权限', width: 300 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'last_login_at', title: '最近登录', width: 150 },
  { colKey: 'op', title: '操作', width: 260, fixed: 'right' as const },
]

// 只有主账号才有成员列表；子账号不请求该接口（后端返回空 items 但没必要发请求）。
const canListMembers = computed(() => !!props.profile && !props.profile.is_sub_account)

async function loadMembers() {
  if (!props.userId || !canListMembers.value) {
    members.value = []
    return
  }
  membersLoading.value = true
  try {
    const data = await getUserMembers(props.userId)
    members.value = data.items || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载成员列表失败')
  } finally {
    membersLoading.value = false
  }
}

// —— 可选权限项：由后端下发（与用户端同一份枚举来源），避免两端各写一份中文标签 ——
const permissionOptions = ref<{ label: string; value: string }[]>([])

async function loadPermissionOptions() {
  if (permissionOptions.value.length) return
  try {
    const opts = await getMemberPermissionOptions()
    permissionOptions.value = (opts || []).map((o) => ({ label: o.label, value: o.code }))
  } catch {
    // 拉取失败时回落本地常量：权限开关缺失不该让整页不可用。
    permissionOptions.value = customerPermissionOptions.map((o) => ({ label: o.label, value: o.value }))
  }
}

// —— 新建 / 编辑 ——
const formVisible = ref(false)
const formMode = ref<'create' | 'edit'>('create')
const formSubmitting = ref(false)
const formRef = ref<FormInstanceFunctions>()
const editingId = ref<number | null>(null)

const form = ref<Required<Pick<SubAccountCreateRequest, 'username' | 'email' | 'phone' | 'password' | 'remark'>> & { permissions: string[] }>({
  username: '',
  email: '',
  phone: '',
  password: '',
  remark: '',
  permissions: [],
})

const formRules: Record<string, FormRule[]> = {
  username: [{ required: true, message: '请输入登录名', type: 'error' }],
  email: [
    { required: true, message: '请输入邮箱', type: 'error' },
    { email: { ignore_max_length: true }, message: '邮箱格式不正确', type: 'error' },
  ],
}

function openCreate() {
  formMode.value = 'create'
  editingId.value = null
  form.value = { username: '', email: '', phone: '', password: '', remark: '', permissions: [] }
  formVisible.value = true
}

function openEdit(row: SubAccountMemberInfo) {
  formMode.value = 'edit'
  editingId.value = row.id
  form.value = {
    username: row.username,
    email: row.email || '',
    phone: row.phone || '',
    password: '',
    remark: row.remark || '',
    permissions: [],
  }
  formVisible.value = true
}

async function submitForm() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  formSubmitting.value = true
  try {
    if (formMode.value === 'create') {
      const result = await createUserMember(props.userId, {
        username: form.value.username,
        email: form.value.email,
        phone: form.value.phone || undefined,
        password: form.value.password || undefined,
        remark: form.value.remark || undefined,
        permissions: form.value.permissions,
      })
      formVisible.value = false
      // 一次性密码只在创建响应里出现一次，必须当场展示（否则成员无法登录）。
      pwdResult.value = { password: result.password, revoked_sessions: 0 }
      pwdVisible.value = true
      MessagePlugin.success('成员已创建')
    } else if (editingId.value) {
      await updateUserMember(props.userId, editingId.value, { remark: form.value.remark })
      formVisible.value = false
      MessagePlugin.success('已保存')
    }
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    formSubmitting.value = false
  }
}

// —— 设置权限 ——
const permVisible = ref(false)
const permSubmitting = ref(false)
const permSelection = ref<string[]>([])
const permMemberId = ref<number | null>(null)

function openPermissions(row: SubAccountMemberInfo) {
  permMemberId.value = row.id
  permSelection.value = [...(row.permissions || [])]
  permVisible.value = true
}

async function submitPermissions() {
  if (!permMemberId.value) return
  permSubmitting.value = true
  try {
    await setUserMemberPermissions(props.userId, permMemberId.value, permSelection.value)
    permVisible.value = false
    MessagePlugin.success('权限已更新')
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存权限失败')
  } finally {
    permSubmitting.value = false
  }
}

// —— 重置密码 ——
const pwdVisible = ref(false)
const pwdResult = ref<SubAccountResetPasswordResult>({ password: '', revoked_sessions: 0 })

async function openResetPassword(row: SubAccountMemberInfo) {
  const ok = await new Promise<boolean>((resolve) => {
    // 用对话框确认：代成员设密会撤销其登录态，属于会打断成员当前使用的动作。
    const dialog = DialogPlugin.confirm({
      header: '重置成员密码',
      body: `将为成员「${row.username}」生成一次性密码，并撤销其当前全部登录会话。是否继续？`,
      confirmBtn: '生成并重置',
      onConfirm: () => {
        dialog.destroy()
        resolve(true)
      },
      onCancel: () => {
        dialog.destroy()
        resolve(false)
      },
      onClose: () => resolve(false),
    })
  })
  if (!ok) return
  try {
    const result = await resetUserMemberPassword(props.userId, row.id)
    pwdResult.value = result
    pwdVisible.value = true
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重置密码失败')
  }
}

async function copyPassword() {
  try {
    await navigator.clipboard.writeText(pwdResult.value.password)
    MessagePlugin.success('已复制')
  } catch {
    MessagePlugin.warning('复制失败，请手动选择复制')
  }
}

// —— 禁用 / 启用 ——
async function handleDisable(row: SubAccountMemberInfo) {
  const ok = await new Promise<boolean>((resolve) => {
    const dialog = DialogPlugin.confirm({
      header: '禁用成员',
      body: `禁用后成员「${row.username}」将无法登录，其当前会话会被立即撤销。是否继续？`,
      theme: 'warning',
      confirmBtn: '禁用',
      onConfirm: () => {
        dialog.destroy()
        resolve(true)
      },
      onCancel: () => {
        dialog.destroy()
        resolve(false)
      },
      onClose: () => resolve(false),
    })
  })
  if (!ok) return
  try {
    await disableUserMember(props.userId, row.id)
    MessagePlugin.success('成员已禁用')
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '禁用失败')
  }
}

async function handleEnable(row: SubAccountMemberInfo) {
  try {
    await updateUserMember(props.userId, row.id, { status: 'active', remark: row.remark })
    MessagePlugin.success('成员已启用')
    await loadMembers()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '启用失败')
  }
}

// —— 操作日志 ——
const logsVisible = ref(false)
const logsLoading = ref(false)
const logs = ref<SubAccountLogInfo[]>([])

const logColumns: PrimaryTableCol<SubAccountLogInfo>[] = [
  { colKey: 'created_at', title: '时间', width: 160 },
  { colKey: 'actor_name', title: '操作人', width: 120 },
  { colKey: 'module', title: '模块', width: 100 },
  { colKey: 'action', title: '动作', width: 100 },
  { colKey: 'target', title: '对象', ellipsis: true },
]

async function openLogs(row: SubAccountMemberInfo) {
  logsVisible.value = true
  logsLoading.value = true
  try {
    const data = await getUserMemberLogs(props.userId, row.id, { page: 1, page_size: 50 })
    logs.value = data.items || []
  } catch (error) {
    logs.value = []
    MessagePlugin.error((error as Error).message || '加载操作日志失败')
  } finally {
    logsLoading.value = false
  }
}

onMounted(() => {
  void loadPermissionOptions()
})

watch(
  () => [props.userId, props.profile?.is_sub_account] as const,
  () => {
    void loadMembers()
  },
  { immediate: true },
)

defineExpose({ reload: loadMembers })
</script>

<style scoped>
.section-hint {
  margin: 0 0 var(--space-md);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.section-hint--tight {
  margin-top: calc(var(--space-sm) * -1);
}

.member-head {
  display: flex;
  align-items: baseline;
  gap: var(--space-sm);
  min-width: 0;
}

/* 一次性密码：等宽字体便于核对，选中即全选（避免手抖漏字符） */
.pwd-row {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin: var(--space-md) 0 var(--space-sm);
}

.pwd-value {
  flex: 1;
  padding: 10px 12px;
  font-family: var(--font-family-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 14px;
  letter-spacing: 0.5px;
  color: var(--color-foreground);
  background: var(--color-surface-2, var(--td-bg-color-container));
  border: 1px dashed var(--color-border, var(--td-component-stroke));
  border-radius: 6px;
  word-break: break-all;
  user-select: all;
}
</style>
