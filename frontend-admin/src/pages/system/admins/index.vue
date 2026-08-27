<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>管理员列表</h2>
        <p>维护后台账号状态、资料与角色。</p>
      </div>
      <t-button theme="primary" @click="openCreate">
        <template #icon>
          <AddIcon />
        </template>
        新增管理员
      </t-button>
    </div>

    <!-- 筛选区域 -->
    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="loadAdmins">
        <t-form-item label="关键词">
          <t-input v-model="filters.keyword" placeholder="用户名 / 邮箱" clearable />
        </t-form-item>
        <t-form-item label="角色">
          <t-select v-model="filters.role" :options="roleOptions" clearable placeholder="全部角色" />
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
        :data="admins.items"
        :columns="columns"
        :loading="loading"
        hover
        size="medium"
      >
        <template #role="{ row }">
          <t-tag :theme="roleTagTheme(row.role)" variant="light-outline">
            {{ roleLabel(row.role) }}
          </t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #department="{ row }">
          {{ row.department || '—' }}
        </template>
        <template #operation="{ row }">
          <t-space size="small">
            <t-link theme="primary" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" @click="resetPassword(row)">重置密码</t-link>
            <t-popconfirm
              :content="row.status === 'active' ? '确认禁用该账号？' : '确认启用该账号？'"
              @confirm="toggleStatus(row)"
            >
              <t-link :theme="row.status === 'active' ? 'danger' : 'primary'">
                {{ row.status === 'active' ? '禁用' : '启用' }}
              </t-link>
            </t-popconfirm>
            <t-popconfirm content="确认删除该管理员？该操作不可恢复" @confirm="removeAdmin(row)">
              <t-link theme="danger">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>

      <div class="pagination-wrapper">
        <t-pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="admins.meta.total"
          show-jumper
          show-page-size
          :page-size-options="[10, 20, 50]"
          @change="loadAdmins"
        />
      </div>
    </t-card>

    <!-- 新增/编辑管理员弹窗 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑管理员' : '新增管理员'"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="560px"
      @confirm="submitAdmin"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="用户名" name="username">
            <t-input v-model="form.username" :disabled="editing" placeholder="登录账号" />
          </t-form-item>
          <t-form-item label="邮箱" name="email">
            <t-input v-model="form.email" placeholder="example@domain.com" />
          </t-form-item>
          <t-form-item label="角色" name="role">
            <t-select v-model="form.role" :options="roleOptions" placeholder="请选择角色" />
          </t-form-item>
          <t-form-item label="部门" name="department">
            <t-input v-model="form.department" placeholder="所属部门" />
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
import { AddIcon } from 'tdesign-icons-vue-next';
import { MessagePlugin } from 'tdesign-vue-next';
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next';
import {
  createAdmin,
  deleteAdmin,
  getAdminList,
  resetAdminPassword,
  updateAdmin,
  updateAdminStatus,
  type AdminInfo,
  type AdminListResponse,
} from '@/api/admin';

/**
 * 管理员列表页面
 * 用于管理后台账号、角色、重置密码等
 */
defineOptions({ name: 'SystemAdmins' });

const ROLE_MAP: Record<string, string> = {
  super_admin: '超级管理员',
  admin: '管理员',
  operator: '运营',
};

const loading = ref(false);
const submitting = ref(false);
const dialogVisible = ref(false);
const passwordVisible = ref(false);
const editing = ref(false);
const currentAdminId = ref(0);
const page = ref(1);
const pageSize = ref(10);
const formRef = ref<FormInstanceFunctions | null>(null);
const newPassword = ref('');

// 筛选与数据
const filters = reactive({ keyword: '', role: '', status: '' });
const admins = ref<AdminListResponse>({
  items: [],
  meta: { page: 1, page_size: 10, total: 0 },
});

// 表单数据
const form = reactive({
  username: '',
  email: '',
  role: 'admin',
  department: '',
  password: '',
  status: 'active',
});

// 选项
const roleOptions = [
  { label: '超级管理员', value: 'super_admin' },
  { label: '管理员', value: 'admin' },
  { label: '运营', value: 'operator' },
];
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
  role: [{ required: true, message: '请选择角色', type: 'error' }],
  password: [
    { required: true, message: '请输入初始密码', type: 'error' },
    { min: 8, message: '密码至少需要 8 位', type: 'error' },
  ],
};

function roleLabel(role: string): string {
  return ROLE_MAP[role] || role;
}

function roleTagTheme(role: string): 'primary' | 'success' | 'warning' | 'default' {
  if (role === 'super_admin') return 'primary';
  if (role === 'admin') return 'success';
  return 'default';
}

// 表格列配置
const columns: PrimaryTableCol<AdminInfo>[] = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'username', title: '用户名', width: 140 },
  { colKey: 'email', title: '邮箱', minWidth: 200 },
  { colKey: 'role', title: '角色', width: 130 },
  { colKey: 'department', title: '部门', width: 120 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'last_login_at', title: '最近登录', width: 180 },
  { colKey: 'operation', title: '操作', width: 240, fixed: 'right' },
];

/**
 * 加载管理员列表
 */
async function loadAdmins() {
  loading.value = true;
  try {
    const data = await getAdminList({
      page: page.value,
      page_size: pageSize.value,
      keyword: filters.keyword,
      role: filters.role,
      status: filters.status,
    });
    admins.value = data;
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载管理员失败');
  } finally {
    loading.value = false;
  }
}

/**
 * 重置筛选
 */
function resetFilters() {
  filters.keyword = '';
  filters.role = '';
  filters.status = '';
  page.value = 1;
  loadAdmins();
}

/**
 * 打开新增弹窗
 */
function openCreate() {
  editing.value = false;
  Object.assign(form, {
    username: '',
    email: '',
    role: 'admin',
    department: '',
    password: '',
    status: 'active',
  });
  dialogVisible.value = true;
}

/**
 * 打开编辑弹窗
 */
function openEdit(row: AdminInfo) {
  currentAdminId.value = row.id;
  editing.value = true;
  Object.assign(form, {
    username: row.username,
    email: row.email,
    role: row.role,
    department: row.department || '',
    password: '',
    status: row.status,
  });
  dialogVisible.value = true;
}

/**
 * 提交管理员表单
 */
async function submitAdmin() {
  const validateResult = await formRef.value?.validate?.();
  if (validateResult !== true) return;

  submitting.value = true;
  try {
    if (editing.value) {
      await updateAdmin(currentAdminId.value, {
        email: form.email.trim(),
        role: form.role,
        department: form.department.trim(),
        status: form.status,
      });
    } else {
      await createAdmin({
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
        role: form.role,
        department: form.department.trim(),
        status: form.status,
      });
    }

    MessagePlugin.success('管理员已保存');
    dialogVisible.value = false;
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存管理员失败');
  } finally {
    submitting.value = false;
  }
}

/**
 * 切换管理员状态
 */
async function toggleStatus(row: AdminInfo) {
  const nextStatus = row.status === 'active' ? 'disabled' : 'active';
  try {
    await updateAdminStatus(row.id, { status: nextStatus });
    MessagePlugin.success(nextStatus === 'active' ? '管理员已启用' : '管理员已禁用');
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败');
  }
}

/**
 * 打开重置密码弹窗
 */
function resetPassword(row: AdminInfo) {
  currentAdminId.value = row.id;
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
    await resetAdminPassword(currentAdminId.value, {
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

/**
 * 删除管理员
 */
async function removeAdmin(row: AdminInfo) {
  try {
    await deleteAdmin(row.id);
    MessagePlugin.success('管理员已删除');
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除管理员失败');
  }
}

onMounted(() => {
  loadAdmins();
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
</style>
