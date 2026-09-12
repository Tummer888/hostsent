<template>
  <div class="page-container system-page">
    <div class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UserListIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">管理员列表</h2>
        </div>
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
      </t-form>
      <div class="filter-form__actions">
        <t-button theme="primary" type="submit" @click.prevent="loadAdmins">查询</t-button>
        <t-button variant="outline" @click="resetFilters">重置</t-button>
      </div>
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
          <t-space size="small" break-line>
            <t-tag v-for="code in roleCodesOf(row)" :key="code" :theme="roleTagTheme(code)" variant="light-outline">
              {{ roleLabel(code) }}
            </t-tag>
            <span v-if="!roleCodesOf(row).length" class="cell-muted">未分配角色</span>
          </t-space>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #must_change_password="{ row }">
          <t-tag v-if="row.must_change_password" theme="warning" variant="light-outline">待改密</t-tag>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #department="{ row }">
          {{ row.department || '—' }}
        </template>
        <template #position="{ row }">
          {{ row.position || '—' }}
        </template>
        <template #operation="{ row }">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([
              { content: '编辑', value: 'edit' },
              { content: '重置密码', value: 'reset-password' },
              { content: row.status === 'active' ? '禁用' : '启用', value: 'toggle', theme: 'error' },
              { content: '删除', value: 'delete', theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="small">
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
          <t-form-item label="角色（可多选）" name="role_ids">
            <t-select
              v-model="form.role_ids"
              :options="roleOptions"
              multiple
              clearable
              filterable
              placeholder="请选择角色"
            />
          </t-form-item>
          <t-form-item label="部门" name="department">
            <t-input v-model="form.department" placeholder="所属部门" />
          </t-form-item>
          <t-form-item label="职位" name="position">
            <t-input v-model="form.position" placeholder="如：客服专员" />
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
        <t-alert
          v-if="!editing"
          theme="info"
          message="新建员工首次登录将被强制修改密码。"
          style="margin-top: 8px"
        />
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
import { computed, onMounted, reactive, ref } from 'vue';
import { AddIcon, UserListIcon } from 'tdesign-icons-vue-next';
import { MessagePlugin } from 'tdesign-vue-next';
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next';
import {
  createAdmin,
  deleteAdmin,
  getAdminList,
  resetAdminPassword,
  setAdminRoles,
  updateAdmin,
  updateAdminStatus,
  type AdminInfo,
  type AdminListResponse,
} from '@/api/admin';
import { getRoleList, type RoleInfo } from '@/api/user';
import MobileAction from '@/components/mobile-action/index.vue';
import { buildMobileActionOptions } from '@/composables/useMobileActions';
import { useIsMobile } from '@/composables/useIsMobile';

/**
 * 员工管理页面（P2-01）
 * 支持多角色（admin_roles）、部门/职位、状态切换、重置密码（重置后强制改密）
 */
defineOptions({ name: 'SystemAdmins' });

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

// 后端角色（scope=admin），用于多选与名称映射
const roleList = ref<RoleInfo[]>([]);
const roleOptions = ref<{ label: string; value: number }[]>([]);
const roleLabelByCode = ref<Record<string, string>>({});

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
  role_ids: [] as number[],
  department: '',
  position: '',
  password: '',
  status: 'active',
});

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
  role_ids: [{ required: true, message: '请至少选择一个角色', type: 'error' }],
  password: [
    { required: true, message: '请输入初始密码', type: 'error' },
    { min: 8, message: '密码至少需要 8 位', type: 'error' },
  ],
};

function roleLabel(code: string): string {
  return roleLabelByCode.value[code] || code;
}

// 兼容：优先展示 roles 数组，回退到单个 role 字符串
function roleCodesOf(row: AdminInfo): string[] {
  if (row.roles?.length) return row.roles;
  return row.role ? [row.role] : [];
}

function roleTagTheme(role: string): 'primary' | 'success' | 'warning' | 'default' {
  if (role === 'super_admin') return 'primary';
  if (role === 'admin') return 'success';
  return 'default';
}

function roleIdOf(code: string): number | undefined {
  return roleList.value.find((item) => item.code === code)?.id;
}

// 表格列配置
const { isMobile } = useIsMobile();

const columns = computed<PrimaryTableCol<AdminInfo>[]>(() => [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'username', title: '用户名', width: 140 },
  { colKey: 'email', title: '邮箱', minWidth: 200 },
  { colKey: 'role', title: '角色', minWidth: 160 },
  { colKey: 'department', title: '部门', width: 110 },
  { colKey: 'position', title: '职位', width: 110 },
  { colKey: 'must_change_password', title: '改密', width: 90 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'last_login_at', title: '最近登录', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 240, fixed: 'right' },
]);

/**
 * 加载角色列表（用于多选）
 */
async function loadRoles() {
  try {
    const items = await getRoleList();
    roleList.value = items;
    roleOptions.value = items
      .filter((item) => item.status === 'active')
      .map((item) => ({ label: item.name, value: item.id }));
    roleLabelByCode.value = Object.fromEntries(items.map((item) => [item.code, item.name]));
  } catch {
    // 角色列表加载失败不阻断页面主体
  }
}

/**
 * 加载员工列表
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
    MessagePlugin.error((error as Error).message || '加载员工失败');
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
    role_ids: [],
    department: '',
    position: '',
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
    // 后端返回的是角色 code，这里映射回 role_id 供多选使用
    role_ids: roleCodesOf(row)
      .map((code) => roleIdOf(code))
      .filter((id): id is number => typeof id === 'number'),
    department: row.department || '',
    position: row.position || '',
    password: '',
    status: row.status,
  });
  dialogVisible.value = true;
}

/**
 * 提交员工表单
 */
async function submitAdmin() {
  const validateResult = await formRef.value?.validate?.();
  if (validateResult !== true) return;

  submitting.value = true;
  try {
    if (editing.value) {
      await updateAdmin(currentAdminId.value, {
        email: form.email.trim(),
        // role 为兼容字段：取首个角色 code；真实多角色由 setAdminRoles 覆盖式写入
        role: roleCodeOf(form.role_ids[0]) || '',
        role_ids: form.role_ids,
        department: form.department.trim(),
        position: form.position.trim(),
        status: form.status,
      });
      // 覆盖式设置多角色（写 admin_roles 并清权限缓存）
      await setAdminRoles(currentAdminId.value, form.role_ids);
    } else {
      await createAdmin({
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
        role: roleCodeOf(form.role_ids[0]) || '',
        role_ids: form.role_ids,
        department: form.department.trim(),
        position: form.position.trim(),
        status: form.status,
      });
    }

    MessagePlugin.success('员工已保存');
    dialogVisible.value = false;
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存员工失败');
  } finally {
    submitting.value = false;
  }
}

function roleCodeOf(id?: number): string {
  if (!id) return '';
  return roleList.value.find((item) => item.id === id)?.code || '';
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
    MessagePlugin.success('员工已删除');
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除员工失败');
  }
}

onMounted(() => {
  loadRoles();
  loadAdmins();
});
function handleMobileAction(value: string | number | Record<string, any>, row: AdminInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '');
  switch (action) {
    case 'edit':
      openEdit(row);
      break;
    case 'reset-password':
      resetPassword(row);
      break;
    case 'toggle':
      void toggleStatus(row);
      break;
    case 'delete':
      void removeAdmin(row);
      break;
  }
}
</script>

<style scoped>
@import '../shared.css';

.filter-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
}

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
  margin: 0;
  font-size: 20px;
  font-weight: 600;
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
