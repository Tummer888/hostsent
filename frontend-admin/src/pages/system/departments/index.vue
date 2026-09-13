<template>
  <div class="page-container system-page">
    <div class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UsergroupIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">部门管理</h2>
          <span class="page-header__desc">维护组织架构，部门决定工单派单范围与销售归属候选</span>
        </div>
      </div>
      <t-space>
        <t-button variant="outline" :loading="loading" @click="loadDepartments">
          <template #icon><RefreshIcon /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon /></template>
          新建部门
        </t-button>
      </t-space>
    </div>

    <!-- 筛选 -->
    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="loadDepartments">
        <t-form-item label="关键词">
          <t-input v-model="filters.keyword" placeholder="部门名称 / 编码" clearable @enter="loadDepartments" />
        </t-form-item>
        <t-form-item label="类型">
          <t-select
            v-model="filters.kind"
            :options="departmentKindOptions"
            clearable
            placeholder="全部类型"
            style="width: 140px"
          />
        </t-form-item>
        <t-form-item label="状态">
          <t-select
            v-model="filters.status"
            :options="activeStatusOptions"
            clearable
            placeholder="全部状态"
            style="width: 120px"
          />
        </t-form-item>
      </t-form>
      <div class="filter-form__actions">
        <t-button theme="primary" @click="loadDepartments">查询</t-button>
        <t-button variant="outline" @click="resetFilters">重置</t-button>
      </div>
    </t-card>

    <t-card :bordered="false" class="table-card">
      <div class="table-card__head">
        <h3 class="card-title">部门列表</h3>
        <span class="table-card__meta">共 {{ flatCount }} 个部门</span>
      </div>
      <t-table
        row-key="id"
        :data="treeData"
        :columns="columns"
        :loading="loading"
        :tree="{ childrenKey: 'children', treeNodeColumnIndex: 0 }"
        size="small"
        hover
        vertical-align="middle"
        :pagination="null"
      >
        <template #kind="{ row }">
          <t-tag :theme="departmentKindTheme(row.kind)" variant="light" size="small" shape="round">
            {{ departmentKindLabel(row.kind) }}
          </t-tag>
        </template>
        <template #leader="{ row }">
          <span>{{ row.leader_name || '—' }}</span>
        </template>
        <template #admin_count="{ row }">
          <span class="count-text">{{ row.admin_count }}</span>
        </template>
        <template #category_count="{ row }">
          <span class="count-text">{{ row.category_count }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([
              { content: '新增子部门', value: 'add-child' },
              { content: '编辑', value: 'edit' },
              { content: '删除', value: 'delete', theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="small">
            <t-link v-if="row.parent_id === 0" theme="primary" size="small" @click="openCreate(row)">新增子部门</t-link>
            <t-link theme="primary" size="small" @click="openEdit(row)">编辑</t-link>
            <t-popconfirm
              content="确认删除该部门？有在职员工或工单分类引用时会被拒绝。"
              @confirm="removeDepartment(row)"
            >
              <t-link theme="danger" size="small">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
        <template #empty>
          <t-empty description="暂无部门，点击右上角「新建部门」开始搭建组织架构" />
        </template>
      </t-table>
    </t-card>

    <!-- 新建/编辑弹窗 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      width="520px"
      :confirm-btn="{ content: '保存', loading: submitting }"
      @confirm="submitDepartment"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top" colonless>
        <t-form-item label="部门名称" name="name">
          <t-input v-model="form.name" placeholder="如：华东销售部" :maxlength="64" />
        </t-form-item>
        <t-form-item label="部门编码" name="code">
          <t-input
            v-model="form.code"
            placeholder="小写字母/数字/下划线，如 sales-east"
            :maxlength="64"
            :disabled="editing"
          />
        </t-form-item>
        <t-form-item label="部门类型" name="kind">
          <t-select v-model="form.kind" :options="departmentKindOptions" placeholder="请选择部门类型" />
        </t-form-item>
        <t-form-item label="上级部门" name="parent_id">
          <t-select
            v-model="form.parent_id"
            :options="parentOptions"
            :keys="{ label: 'name', value: 'id' }"
            placeholder="顶级部门（不选）"
            clearable
            filterable
          />
        </t-form-item>
        <t-form-item label="部门主管" name="leader_admin_id">
          <t-select
            v-model="form.leader_admin_id"
            :options="adminOptions"
            :keys="{ label: 'label', value: 'value' }"
            placeholder="可选，用于工单复核兜底与客户交待"
            clearable
            filterable
          />
        </t-form-item>
        <t-form-item label="排序" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" theme="column" style="width: 160px" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="form.remark" placeholder="部门职责说明" :autosize="{ minRows: 2, maxRows: 4 }" :maxlength="255" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio value="active">启用</t-radio>
            <t-radio value="disabled">禁用</t-radio>
          </t-radio-group>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, RefreshIcon, UsergroupIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createDepartment,
  deleteDepartment,
  getAdminList,
  getDepartmentList,
  updateDepartment,
  type DepartmentInfo,
  type DepartmentSaveRequest,
} from '@/api/admin'
import {
  activeStatusOptions,
  departmentKindLabel,
  departmentKindOptions,
  departmentKindTheme,
} from '@/pages/system/constants'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 部门管理（S1 员工体系，doc86 §4.1.1）
 *
 * 树表格与 system/menus 一致：只支持两级，父部门不能有上级。
 * 删除受后端前置校验保护（有在职员工或工单分类引用返回 409）。
 */
defineOptions({ name: 'SystemDepartments' })

const { isMobile } = useIsMobile()

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const currentId = ref(0)
const formRef = ref<FormInstanceFunctions | null>(null)

const departments = ref<DepartmentInfo[]>([])
const adminOptions = ref<{ label: string; value: number }[]>([])

const filters = reactive({ keyword: '', kind: '', status: '' })

const form = reactive<DepartmentSaveRequest & { sort_order: number }>({
  name: '',
  code: '',
  kind: 'general',
  parent_id: 0,
  leader_admin_id: 0,
  remark: '',
  sort_order: 0,
  status: 'active',
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入部门名称', type: 'error' }],
  code: [
    { required: true, message: '请输入部门编码', type: 'error' },
    { pattern: /^[a-z0-9_-]{2,64}$/, message: '编码仅支持小写字母、数字、下划线与短横线', type: 'error' },
  ],
  kind: [{ required: true, message: '请选择部门类型', type: 'error' }],
}

const dialogTitle = computed(() => (editing.value ? '编辑部门' : '新建部门'))

// 树形数据直接由后端组装（tree=true），无需前端再拼
const treeData = computed(() => departments.value)

const flatCount = computed(() => {
  let count = 0
  for (const node of departments.value) {
    count += 1 + (node.children?.length ?? 0)
  }
  return count
})

// 上级可选：仅顶级部门可作为父节点，且编辑时排除自身
const parentOptions = computed(() =>
  departments.value.filter((item) => item.parent_id === 0 && item.id !== currentId.value),
)

const columns = computed<PrimaryTableCol<DepartmentInfo>[]>(() => [
  { colKey: 'name', title: '部门名称', minWidth: 160 },
  { colKey: 'code', title: '编码', minWidth: 150 },
  { colKey: 'kind', title: '类型', width: 90 },
  { colKey: 'leader', title: '主管', width: 110 },
  { colKey: 'admin_count', title: '在职', width: 70, align: 'center' as const },
  { colKey: 'category_count', title: '工单分类', width: 90, align: 'center' as const },
  { colKey: 'sort_order', title: '排序', width: 70, align: 'center' as const },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 220, fixed: 'right' as const, align: 'center' as const },
])

async function loadDepartments() {
  loading.value = true
  try {
    const data = await getDepartmentList({
      keyword: filters.keyword.trim() || undefined,
      kind: filters.kind || undefined,
      status: filters.status || undefined,
    })
    departments.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载部门失败')
  } finally {
    loading.value = false
  }
}

// 主管下拉复用员工列表：取较大页容量覆盖常见规模，避免为选人再开一个接口
async function loadAdminOptions() {
  try {
    const data = await getAdminList({ page: 1, page_size: 200, is_resigned: '0' })
    adminOptions.value = data.items.map((item) => ({
      label: item.real_name ? `${item.real_name}（${item.username}）` : item.username,
      value: item.id,
    }))
  } catch {
    // 主管下拉加载失败不阻断部门列表
  }
}

function resetFilters() {
  filters.keyword = ''
  filters.kind = ''
  filters.status = ''
  loadDepartments()
}

function openCreate(parent?: DepartmentInfo) {
  editing.value = false
  currentId.value = 0
  Object.assign(form, {
    name: '',
    code: '',
    kind: parent?.kind ?? 'general',
    parent_id: parent?.id ?? 0,
    leader_admin_id: 0,
    remark: '',
    sort_order: 0,
    status: 'active',
  })
  dialogVisible.value = true
}

function openEdit(row: DepartmentInfo) {
  editing.value = true
  currentId.value = row.id
  Object.assign(form, {
    name: row.name,
    code: row.code,
    kind: row.kind,
    parent_id: row.parent_id,
    leader_admin_id: row.leader_admin_id,
    remark: row.remark ?? '',
    sort_order: row.sort_order,
    status: row.status,
  })
  dialogVisible.value = true
}

async function submitDepartment() {
  const validateResult = await formRef.value?.validate?.()
  if (validateResult !== true) return
  submitting.value = true
  const payload: DepartmentSaveRequest = {
    name: form.name.trim(),
    code: form.code.trim(),
    kind: form.kind,
    parent_id: form.parent_id || 0,
    leader_admin_id: form.leader_admin_id || 0,
    remark: form.remark,
    sort_order: form.sort_order,
    status: form.status,
  }
  try {
    if (editing.value) {
      await updateDepartment(currentId.value, payload)
      MessagePlugin.success('部门已更新')
    } else {
      await createDepartment(payload)
      MessagePlugin.success('部门已创建')
    }
    dialogVisible.value = false
    await loadDepartments()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存部门失败')
  } finally {
    submitting.value = false
  }
}

async function removeDepartment(row: DepartmentInfo) {
  try {
    await deleteDepartment(row.id)
    MessagePlugin.success('部门已删除')
    await loadDepartments()
  } catch (error) {
    // 后端对「有在职员工/分类引用」返回具体原因，原样展示便于运营处置
    MessagePlugin.error((error as Error).message || '删除失败')
  }
}

onMounted(() => {
  loadDepartments()
  loadAdminOptions()
})

function handleMobileAction(value: string | number | Record<string, any>, row: DepartmentInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'add-child':
      openCreate(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      void removeDepartment(row)
      break
  }
}
</script>

<style scoped>
@import '../shared.css';

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

.filter-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
}

.table-card__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 12px;
}

.card-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.table-card__meta {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.count-text {
  font-variant-numeric: tabular-nums;
}
</style>
