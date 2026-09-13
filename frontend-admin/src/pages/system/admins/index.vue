<template>
  <div class="page-container system-page">
    <div class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UserListIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">员工管理</h2>
          <span class="page-header__desc">账号 · 角色 · 部门 · 销售能力与在职状态</span>
        </div>
      </div>
      <t-button theme="primary" @click="openCreate">
        <template #icon>
          <AddIcon />
        </template>
        新增员工
      </t-button>
    </div>

    <!-- 筛选区域 -->
    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="loadAdmins">
        <t-form-item label="关键词">
          <t-input v-model="filters.keyword" placeholder="用户名 / 姓名 / 邮箱 / 手机" clearable @enter="loadAdmins" />
        </t-form-item>
        <t-form-item label="角色">
          <t-select v-model="filters.role" :options="roleCodeOptions" clearable placeholder="全部角色" style="width: 150px" />
        </t-form-item>
        <t-form-item label="部门">
          <t-select
            v-model="filters.department_id"
            :options="departmentOptions"
            :keys="{ label: 'name', value: 'id' }"
            clearable
            filterable
            placeholder="全部部门"
            style="width: 170px"
          />
        </t-form-item>
        <t-form-item label="人员类型">
          <t-select v-model="filters.staff_type" :options="staffTypeOptions" clearable placeholder="全部类型" style="width: 140px" />
        </t-form-item>
        <t-form-item label="销售能力">
          <t-select v-model="filters.sales_enabled" :options="salesEnabledOptions" clearable placeholder="全部" style="width: 110px" />
        </t-form-item>
        <t-form-item label="在职状态">
          <t-select v-model="filters.is_resigned" :options="resignedOptions" clearable placeholder="全部" style="width: 120px" />
        </t-form-item>
      </t-form>
      <div class="filter-form__actions">
        <t-button theme="primary" @click="loadAdmins">查询</t-button>
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
        <template #staff_type="{ row }">
          <t-tag :theme="staffTypeTheme(row.staff_type)" variant="light" size="small" shape="round">
            {{ staffTypeLabel(row.staff_type) }}
          </t-tag>
        </template>
        <template #department_name="{ row }">
          {{ row.department_name || row.department || '—' }}
        </template>
        <template #sales_enabled="{ row }">
          <t-tag v-if="row.sales_enabled" theme="primary" variant="light-outline" size="small">销售</t-tag>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #status="{ row }">
          <!-- 离职优先于启用/禁用展示：离职账号状态必然是 disabled，直接说明原因 -->
          <t-tag v-if="row.is_resigned" theme="default" variant="light-outline">已离职</t-tag>
          <t-tag v-else :theme="row.status === 'active' ? 'success' : 'danger'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #must_change_password="{ row }">
          <t-tag v-if="row.must_change_password" theme="warning" variant="light-outline">待改密</t-tag>
          <span v-else class="cell-muted">—</span>
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
              { content: row.status === 'active' ? '禁用' : '启用', value: 'toggle' },
              { content: '离职', value: 'resign', theme: 'warning' },
              { content: '删除', value: 'delete', theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="small" break-line>
            <t-link theme="primary" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" @click="resetPassword(row)">重置密码</t-link>
            <template v-if="!row.is_resigned">
              <t-popconfirm
                :content="row.status === 'active' ? '确认禁用该账号？' : '确认启用该账号？'"
                @confirm="toggleStatus(row)"
              >
                <t-link :theme="row.status === 'active' ? 'warning' : 'primary'">
                  {{ row.status === 'active' ? '禁用' : '启用' }}
                </t-link>
              </t-popconfirm>
              <t-link theme="warning" @click="openResign(row)">离职</t-link>
            </template>
            <t-popconfirm
              content="确认删除该员工？有在途客户或未结提成时建议改用「离职」。"
              @confirm="removeAdmin(row)"
            >
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

    <!-- 新增/编辑员工弹窗 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑员工' : '新增员工'"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="640px"
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
          <t-form-item label="姓名" name="real_name">
            <t-input v-model="form.real_name" placeholder="真实姓名（工单/销售展示用）" :maxlength="64" />
          </t-form-item>
          <t-form-item label="手机号" name="phone">
            <t-input v-model="form.phone" placeholder="选填" :maxlength="32" />
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
          <t-form-item label="人员类型" name="staff_type">
            <t-select v-model="form.staff_type" :options="staffTypeOptions" placeholder="决定业务身份，与角色独立" />
          </t-form-item>
          <t-form-item label="所属部门" name="department_id">
            <t-select
              v-model="form.department_id"
              :options="departmentOptions"
              :keys="{ label: 'name', value: 'id' }"
              clearable
              filterable
              placeholder="选择部门"
            />
          </t-form-item>
          <t-form-item label="职位" name="position">
            <t-input v-model="form.position" placeholder="如：客服专员" />
          </t-form-item>
          <t-form-item label="销售能力" name="sales_enabled">
            <div class="switch-cell">
              <t-switch v-model="form.sales_enabled" />
              <span class="switch-hint">开启后可被分配客户并产生提成</span>
            </div>
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

    <!-- 离职弹窗（S1）：说明提成与客户交待规则，避免误以为是删除 -->
    <t-dialog
      v-model:visible="resignVisible"
      header="员工离职"
      :confirm-btn="{ content: '确认离职', theme: 'warning', loading: submitting }"
      width="480px"
      @confirm="submitResign"
    >
      <t-alert
        theme="warning"
        message="离职将禁用账号并交待在途客户；已计提的提成仍照常解冻与提现，历史工单保留可追溯。"
        style="margin-bottom: 12px"
      />
      <t-form label-align="top">
        <t-form-item label="在途客户承接人">
          <t-select
            v-model="resignForm.transfer_to_admin_id"
            :options="transferOptions"
            :keys="{ label: 'label', value: 'value' }"
            clearable
            filterable
            placeholder="留空则由部门主管承接"
          />
        </t-form-item>
        <t-form-item label="离职原因">
          <t-textarea
            v-model="resignForm.reason"
            placeholder="选填，写入审计日志"
            :autosize="{ minRows: 2, maxRows: 4 }"
            :maxlength="255"
          />
        </t-form-item>
      </t-form>
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
  getDepartmentList,
  resetAdminPassword,
  resignAdmin,
  setAdminRoles,
  updateAdmin,
  updateAdminStatus,
  type AdminInfo,
  type AdminListResponse,
  type DepartmentInfo,
} from '@/api/admin';
import { getRoleList, type RoleInfo } from '@/api/user';
import { staffTypeLabel, staffTypeOptions, staffTypeTheme } from '@/pages/system/constants';
import MobileAction from '@/components/mobile-action/index.vue';
import { buildMobileActionOptions } from '@/composables/useMobileActions';
import { useIsMobile } from '@/composables/useIsMobile';

/**
 * 员工管理页面（P2-01，S1 改版）
 *
 * S1 新增：部门归属（department_id，替代自由文本）、人员类型（staff_type）、
 * 销售能力开关（sales_enabled）、在职/离职状态与「离职」动作。
 * 角色仍走 admin_roles 多角色（与人员类型正交）。
 */
defineOptions({ name: 'SystemAdmins' });

const loading = ref(false);
const submitting = ref(false);
const dialogVisible = ref(false);
const passwordVisible = ref(false);
const resignVisible = ref(false);
const editing = ref(false);
const currentAdminId = ref(0);
const page = ref(1);
const pageSize = ref(10);
const formRef = ref<FormInstanceFunctions | null>(null);
const newPassword = ref('');

// 后端角色（scope=admin），用于多选与名称映射
const roleList = ref<RoleInfo[]>([]);
const roleOptions = ref<{ label: string; value: number }[]>([]);
const roleCodeOptions = ref<{ label: string; value: string }[]>([]);
const roleLabelByCode = ref<Record<string, string>>({});

// 部门下拉（平铺，用于筛选与表单）
const departments = ref<DepartmentInfo[]>([]);
const departmentOptions = computed(() => departments.value);

const salesEnabledOptions = [
  { label: '是', value: '1' },
  { label: '否', value: '0' },
];
const resignedOptions = [
  { label: '在职', value: '0' },
  { label: '已离职', value: '1' },
];

// 筛选与数据
const filters = reactive({
  keyword: '',
  role: '',
  department_id: undefined as number | undefined,
  staff_type: '',
  sales_enabled: '',
  is_resigned: '',
  status: '',
});
const admins = ref<AdminListResponse>({
  items: [],
  meta: { page: 1, page_size: 10, total: 0 },
});

// 表单数据
const form = reactive({
  username: '',
  email: '',
  real_name: '',
  phone: '',
  role_ids: [] as number[],
  staff_type: 'admin',
  department_id: undefined as number | undefined,
  position: '',
  sales_enabled: false,
  password: '',
  status: 'active',
});

// 离职表单
const resignForm = reactive({
  transfer_to_admin_id: undefined as number | undefined,
  reason: '',
});

// 承接人候选：排除离职人员（下拉数据来自当前页会更准，但跨页不全会漏，
// 因此单独维护一份在职员工快照，与列表筛选解耦）。
const transferOptions = ref<{ label: string; value: number }[]>([]);

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
  staff_type: [{ required: true, message: '请选择人员类型', type: 'error' }],
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

function roleTagTheme(role: string): 'danger' | 'primary' | 'success' | 'warning' | 'default' {
  if (role === 'super_admin') return 'danger';
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
  { colKey: 'username', title: '用户名', width: 130 },
  { colKey: 'real_name', title: '姓名', width: 100, cell: (h, { row }) => row.real_name || '—' },
  { colKey: 'email', title: '邮箱', minWidth: 190 },
  { colKey: 'role', title: '角色', minWidth: 160 },
  { colKey: 'staff_type', title: '类型', width: 90 },
  { colKey: 'department_name', title: '部门', width: 120 },
  { colKey: 'position', title: '职位', width: 110 },
  { colKey: 'sales_enabled', title: '销售', width: 80 },
  { colKey: 'must_change_password', title: '改密', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'last_login_at', title: '最近登录', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 260, fixed: 'right' },
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
    // 角色筛选按 code 过滤（后端 role 走 admin_roles 联表比对 code）
    roleCodeOptions.value = items
      .filter((item) => item.status === 'active')
      .map((item) => ({ label: item.name, value: item.code }));
    roleLabelByCode.value = Object.fromEntries(items.map((item) => [item.code, item.name]));
  } catch {
    // 角色列表加载失败不阻断页面主体
  }
}

/**
 * 加载部门（平铺）与在职员工快照
 */
async function loadDepartments() {
  try {
    const data = await getDepartmentList({ flat: 1, status: 'active' });
    departments.value = data.items;
  } catch {
    // 部门加载失败不阻断主表
  }
}

async function loadTransferOptions() {
  try {
    const data = await getAdminList({ page: 1, page_size: 200, is_resigned: '0' });
    transferOptions.value = data.items.map((item) => ({
      label: item.real_name ? `${item.real_name}（${item.username}）` : item.username,
      value: item.id,
    }));
  } catch {
    // 承接人下拉失败时仍可提交（留空由部门主管承接）
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
      keyword: filters.keyword.trim() || undefined,
      role: filters.role || undefined,
      status: filters.status,
      department_id: filters.department_id,
      staff_type: filters.staff_type || undefined,
      sales_enabled: filters.sales_enabled || undefined,
      is_resigned: filters.is_resigned || undefined,
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
  filters.department_id = undefined;
  filters.staff_type = '';
  filters.sales_enabled = '';
  filters.is_resigned = '';
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
    real_name: '',
    phone: '',
    role_ids: [],
    staff_type: 'admin',
    department_id: undefined,
    position: '',
    sales_enabled: false,
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
    real_name: row.real_name || '',
    phone: row.phone || '',
    staff_type: row.staff_type || 'admin',
    department_id: row.department_id || undefined,
    position: row.position || '',
    sales_enabled: !!row.sales_enabled,
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
    const common = {
      email: form.email.trim(),
      real_name: form.real_name.trim(),
      phone: form.phone.trim(),
      staff_type: form.staff_type,
      department_id: form.department_id || 0,
      sales_enabled: form.sales_enabled,
      position: form.position.trim(),
      // 部门自由文本不再由前端填写：编辑时留空，展示改读 department_id 对应名称；
      // 为兼容存量数据与旧接口，仍回传原值。
      department: '',
      status: form.status,
    };
    if (editing.value) {
      await updateAdmin(currentAdminId.value, {
        ...common,
        // role 为兼容字段：取首个角色 code；真实多角色由 setAdminRoles 覆盖式写入
        role: roleCodeOf(form.role_ids[0]) || '',
      });
      // 覆盖式设置多角色（写 admin_roles 并清权限缓存）
      await setAdminRoles(currentAdminId.value, form.role_ids);
    } else {
      await createAdmin({
        ...common,
        username: form.username.trim(),
        password: form.password,
        role: roleCodeOf(form.role_ids[0]) || '',
        role_ids: form.role_ids,
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
 * 打开离职弹窗
 */
function openResign(row: AdminInfo) {
  currentAdminId.value = row.id;
  resignForm.transfer_to_admin_id = undefined;
  resignForm.reason = '';
  resignVisible.value = true;
  loadTransferOptions();
}

/**
 * 提交离职
 */
async function submitResign() {
  submitting.value = true;
  try {
    await resignAdmin(currentAdminId.value, {
      reason: resignForm.reason.trim(),
      transfer_to_admin_id: resignForm.transfer_to_admin_id || 0,
    });
    MessagePlugin.success('员工已办理离职');
    resignVisible.value = false;
    await loadAdmins();
  } catch (error) {
    MessagePlugin.error((error as Error).message || '离职办理失败');
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
  loadDepartments();
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
    case 'resign':
      openResign(row);
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

.page-header__desc {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
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

.switch-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.switch-hint {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
</style>
