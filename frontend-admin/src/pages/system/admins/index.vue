<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>管理员列表</h2>
        <p>维护后台账号状态、资料与角色绑定关系。</p>
      </div>
      <t-button theme="primary" @click="openCreate">
        <template #icon><t-icon name="add" /></template>
        新增管理员
      </t-button>
    </div>

    <!-- 筛选区域 -->
    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="loadUsers">
        <t-form-item label="关键词">
          <t-input v-model="filters.keyword" placeholder="用户名 / 姓名 / 邮箱 / 手机号" clearable />
        </t-form-item>
        <t-form-item label="状态">
          <t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部状态" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" type="submit">查询</t-button>
          <t-button variant="outline" @click="resetFilters">重置</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 表格区域 -->
    <t-card :bordered="false" class="table-card">
      <t-table
        row-key="id"
        :data="users.items"
        :columns="columns"
        :loading="loading"
        hover
        size="medium"
      >
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #roles="{ row }">
          <t-space break-line size="small">
            <t-tag v-for="role in row.roles || []" :key="role" size="small" variant="light">
              {{ role }}
            </t-tag>
            <span v-if="!(row.roles && row.roles.length)" class="empty-text">未分配</span>
          </t-space>
        </template>
        <template #operation="{ row }">
          <t-space size="small">
            <t-link theme="primary" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" @click="openRoles(row)">分配角色</t-link>
            <t-link theme="primary" @click="resetPassword(row)">重置密码</t-link>
            <t-popconfirm
              :content="row.status === 'active' ? '确认禁用该账号？' : '确认启用该账号？'"
              @confirm="toggleStatus(row)"
            >
              <t-link :theme="row.status === 'active' ? 'danger' : 'primary'">
                {{ row.status === 'active' ? '禁用' : '启用' }}
              </t-link>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>

      <div class="pagination-wrapper">
        <t-pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="users.meta.total"
          show-jumper
          show-page-size
          :page-size-options="[10, 20, 50]"
          @change="loadUsers"
        />
      </div>
    </t-card>

    <!-- 新增/编辑管理员弹窗 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑管理员' : '新增管理员'"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="600px"
      @confirm="submitUser"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="用户名" name="username">
            <t-input v-model="form.username" :disabled="editing" placeholder="登录账号" />
          </t-form-item>
          <t-form-item label="姓名" name="real_name">
            <t-input v-model="form.real_name" placeholder="真实姓名" />
          </t-form-item>
          <t-form-item label="邮箱" name="email">
            <t-input v-model="form.email" placeholder="example@domain.com" />
          </t-form-item>
          <t-form-item label="手机号" name="phone">
            <t-input v-model="form.phone" placeholder="手机联系方式" />
          </t-form-item>
          <t-form-item v-if="!editing" label="初始密码" name="password">
            <t-input v-model="form.password" type="password" placeholder="建议包含字母与数字，至少8位" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="form.status">
              <t-radio value="active">启用</t-radio>
              <t-radio value="disabled">禁用</t-radio>
            </t-radio-group>
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <!-- 分配角色弹窗 -->
    <t-dialog
      v-model:visible="rolesVisible"
      header="分配角色"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="450px"
      @confirm="submitRoles"
    >
      <div class="roles-dialog-content">
        <p class="dialog-tip">请选择要分配给该管理员的角色：</p>
        <t-checkbox-group v-model="selectedRoleIds" class="roles-checkbox-group">
          <t-checkbox
            v-for="role in roles"
            :key="role.id"
            :value="role.id"
            :disabled="role.status !== 'active'"
          >
            {{ role.name }} <span class="role-code">({{ role.code }})</span>
          </t-checkbox>
        </t-checkbox-group>
      </div>
    </t-dialog>

    <!-- 重置密码弹窗 -->
    <t-dialog
      v-model:visible="passwordVisible"
      header="重置密码"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="400px"
      @confirm="submitPassword"
    >
      <t-form-item label="新密码">
        <t-input v-model="newPassword" type="password" placeholder="请输入新密码，至少 8 位" />
      </t-form-item>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { MessagePlugin } from 'tdesign-vue-next';
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next';
import {
  assignUserRoles,
  createUser,
  getRoleList,
  getUserList,
  resetUserPassword,
  updateUser,
  updateUserStatus,
  type RoleInfo,
  type UserInfo,
  type UserListResponse,
} from '@/api/user';

/**
 * 管理员列表页面
 * 用于管理后台账号、分配角色、重置密码等
 */
defineOptions({ name: 'SystemAdmins' });

const loading = ref(false);
const submitting = ref(false);
const dialogVisible = ref(false);
const rolesVisible = ref(false);
const passwordVisible = ref(false);
const editing = ref(false);
const currentUserId = ref(0);
const page = ref(1);
const pageSize = ref(10);
const formRef = ref<FormInstanceFunctions | null>(null);
const newPassword = ref('');

// 筛选与数据
const filters = reactive({ keyword: '', status: '' });
const users = ref<UserListResponse>({
  items: [],
  meta: { page: 1, page_size: 10, total: 0 },
});
const roles = ref<RoleInfo[]>([]);
const selectedRoleIds = ref<number[]>([]);

// 表单数据
const form = reactive({
  username: '',
  real_name: '',
  email: '',
  phone: '',
  password: '',
  status: 'active',
});

// 状态选项
const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
];

// 表单校验规则
const rules: Record<string, FormRule[]> = {
  username: [{ required: true, message: '请输入用户名', type: 'error' }],
  email: [
    { required: true, message: '请输入邮箱', type: 'error' },
    { email: true, message: '请输入正确的邮箱格式', type: 'error' },
  ],
  phone: [{ required: true, message: '请输入手机号', type: 'error' }],
  password: [
    { required: true, message: '请输入初始密码', type: 'error' },
    { min: 8, message: '密码至少需要 8 位', type: 'error' },
  ],
};

// 表格列配置
const columns: PrimaryTableCol<UserInfo>[] = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'username', title: '用户名', width: 140 },
  { colKey: 'real_name', title: '姓名', width: 120 },
  { colKey: 'phone', title: '手机号', width: 150 },
  { colKey: 'email', title: '邮箱', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'roles', title: '角色', minWidth: 180 },
  { colKey: 'created_at', title: '创建时间', width: 180 },
  { colKey: 'operation', title: '操作', width: 280, fixed: 'right' },
];

/**
 * 加载管理员列表
 */
async function loadUsers() {
  loading.value = true;
  try {
    const data = await getUserList({
      page: page.value,
      page_size: pageSize.value,
      keyword: filters.keyword,
      status: filters.status,
    });
    users.value = data;
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载管理员失败');
  } finally {
    loading.value = false;
  }
}

/**
 * 加载角色列表（用于分配角色）
 */
async function loadRoles() {
  try {
    roles.value = await getRoleList();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载角色失败');
  }
}

/**
 * 重置筛选
 */
function resetFilters() {
  filters.keyword = '';
  filters.status = '';
  loadUsers();
}

/**
 * 打开新增弹窗
 */
function openCreate() {
  editing.value = false;
  Object.assign(form, {
    username: '',
    real_name: '',
    email: '',
    phone: '',
    password: '',
    status: 'active',
  });
  dialogVisible.value = true;
}

/**
 * 打开编辑弹窗
 */
function openEdit(row: UserInfo) {
  currentUserId.value = row.id;
  editing.value = true;
  Object.assign(form, { ...row, password: '' });
  dialogVisible.value = true;
}

/**
 * 提交管理员表单
 */
async function submitUser() {
  const validateResult = await formRef.value?.validate?.();
  if (validateResult !== true) return;

  submitting.value = true;
  try {
    const payload = {
      username: form.username.trim(),
      real_name: form.real_name.trim(),
      email: form.email.trim(),
      phone: form.phone.trim(),
      status: form.status,
    };

    if (editing.value) {
      await updateUser(currentUserId.value, payload);
    } else {
      await createUser({
        ...payload,
        password: form.password,
        role_ids: [],
      });
    }

    MessagePlugin.success('管理员已保存');
    dialogVisible.value = false;
    await loadUsers();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存管理员失败');
  } finally {
    submitting.value = false;
  }
}

/**
 * 打开分配角色弹窗
 */
function openRoles(row: UserInfo) {
  currentUserId.value = row.id;
  // 根据 role codes 映射回 role IDs
  selectedRoleIds.value = roles.value
    .filter((role) => (row.roles || []).includes(role.code))
    .map((role) => role.id);
  rolesVisible.value = true;
}

/**
 * 提交角色分配
 */
async function submitRoles() {
  submitting.value = true;
  try {
    await assignUserRoles(currentUserId.value, {
      role_ids: selectedRoleIds.value,
    });
    MessagePlugin.success('角色分配成功');
    rolesVisible.value = false;
    await loadUsers();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存角色失败');
  } finally {
    submitting.value = false;
  }
}

/**
 * 切换管理员状态
 */
async function toggleStatus(row: UserInfo) {
  const nextStatus = row.status === 'active' ? 'disabled' : 'active';
  try {
    await updateUserStatus(row.id, { status: nextStatus });
    MessagePlugin.success(nextStatus === 'active' ? '管理员已启用' : '管理员已禁用');
    await loadUsers();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败');
  }
}

/**
 * 打开重置密码弹窗
 */
function resetPassword(row: UserInfo) {
  currentUserId.value = row.id;
  newPassword.value = '';
  passwordVisible.value = true;
}

/**
 * 提交重置密码
 */
async function submitPassword() {
  if (newPassword.value.length < 8) {
    MessagePlugin.warning('密码至少需要 8 位');
    return;
  }
  submitting.value = true;
  try {
    await resetUserPassword(currentUserId.value, {
      password: newPassword.value,
    });
    MessagePlugin.success('密码已重置');
    passwordVisible.value = false;
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重置密码失败');
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadRoles()]);
});
</script>

<style scoped>
.page-container {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.page-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 500;
}

.page-header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}

.filter-card,
.table-card {
  background: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px 24px;
}

.empty-text {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.dialog-tip {
  margin-bottom: 16px;
  color: var(--td-text-color-secondary);
}

.roles-checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.role-code {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>
