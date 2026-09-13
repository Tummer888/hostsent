<template>
  <div class="page-body ticket-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <TagIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">工单分类管理</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadCategories">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新增分类
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">分类列表</h3>
        <span class="table-card__meta">共 {{ categories.length }} 个分类</span>
      </div>
      <t-table
        row-key="id"
        :data="categories"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="null"
      >
        <template #status="{ row }">
          <t-tag :theme="categoryStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ categoryStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #department_name="{ row }">
          <span v-if="row.department_name">{{ row.department_name }}</span>
          <span v-else class="cell-muted">未指定</span>
        </template>
        <template #preconditions="{ row }">
          <t-space size="small" break-line>
            <t-tag v-if="row.require_realname" theme="warning" variant="light" size="small" shape="round">需实名</t-tag>
            <t-tag v-if="row.require_binding" theme="warning" variant="light" size="small" shape="round">需关联订单</t-tag>
            <t-tag v-if="row.need_review" theme="primary" variant="light" size="small" shape="round">双人复核</t-tag>
            <t-tag v-if="row.visible_role_codes?.length" variant="light" size="small" shape="round">
              限 {{ row.visible_role_codes.length }} 个角色
            </t-tag>
            <span
              v-if="!row.require_realname && !row.require_binding && !row.need_review && !row.visible_role_codes?.length"
              class="cell-muted"
            >
              无
            </span>
          </t-space>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '禁用/启用', value: 'toggle', theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="handleToggleStatus(row)">
                {{ row.status === 'active' ? '禁用' : '启用' }}
              </t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无分类数据" />
        </template>
      </t-table>
    </section>

    <!-- 新增/编辑分类对话框 -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editingId ? '编辑分类' : '新增分类'"
      width="640px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="分类名称" name="name">
          <t-input v-model="form.name" placeholder="如：技术支持" :maxlength="64" />
        </t-form-item>
        <t-form-item label="分类编码" name="code">
          <t-input v-model="form.code" placeholder="如：technical（唯一，保存后不可与现有重复）" :maxlength="64" :disabled="!!editingId" />
        </t-form-item>
        <t-form-item label="描述" name="description">
          <t-textarea v-model="form.description" placeholder="分类用途说明" :autosize="{ minRows: 2, maxRows: 4 }" :maxlength="255" />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="排序值" name="sort_order">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" style="width: 100%" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="categoryStatusOptions" />
          </t-form-item>
          <t-form-item label="首次响应时限（小时）" name="sla_hours">
            <t-input-number v-model="form.sla_hours" :min="0" theme="column" style="width: 100%" />
          </t-form-item>
          <t-form-item label="归属部门" name="department_id">
            <t-select
              v-model="form.department_id"
              clearable
              filterable
              placeholder="不指定则不限部门"
              :options="departmentOptions"
              :loading="departmentLoading"
            />
          </t-form-item>
        </div>

        <t-form-item label="提交前置条件" name="preconditions">
          <t-space direction="vertical" size="small">
            <t-checkbox v-model="form.require_realname">要求提交人已完成实名认证</t-checkbox>
            <t-checkbox v-model="form.require_binding">要求关联本人的订单或实例</t-checkbox>
            <span class="form-hint">勾选后，用户在用户中心提交该分类工单时会被前置校验拦截并给出对应提示。</span>
          </t-space>
        </t-form-item>

        <t-form-item label="回复复核" name="need_review">
          <t-space direction="vertical" size="small">
            <t-checkbox v-model="form.need_review">管理员回复需双人复核（复核通过后才对用户可见）</t-checkbox>
            <span class="form-hint">开启后需保证本部门存在两名以上具备「工单复核」权限的员工。</span>
          </t-space>
        </t-form-item>

        <t-form-item label="可提交角色（空=不限）" name="visible_role_codes">
          <t-select
            v-model="form.visible_role_codes"
            multiple
            clearable
            filterable
            placeholder="不限"
            :options="roleOptions"
            :loading="roleLoading"
            :max-tag-count="2"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { AddIcon, RefreshIcon, TagIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createTicketCategory, deleteTicketCategory, getTicketCategories, updateTicketCategory } from '@/api/ticket'
import { getDepartmentList, type DepartmentInfo } from '@/api/admin'
import { getRoleList, type RoleInfo } from '@/api/user'
import { categoryStatusLabel, categoryStatusOptions, categoryStatusTheme, formatTime } from '@/pages/ticket/constants'
import type { TicketCategoryInfo, TicketCategorySaveRequest } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'TicketCategories' })

const categories = ref<TicketCategoryInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const saving = ref(false)
const formVisible = ref(false)
const editingId = ref<number | null>(null)

// 部门下拉（S2 归属部门）与角色下拉（可提交角色）
const departmentOptions = ref<{ label: string; value: number }[]>([])
const departmentLoading = ref(false)
const roleOptions = ref<{ label: string; value: string }[]>([])
const roleLoading = ref(false)

const form = reactive<{
  name: string
  code: string
  description: string
  sort_order: number
  status: string
  department_id: number | undefined
  require_realname: boolean
  require_binding: boolean
  need_review: boolean
  visible_role_codes: string[]
  sla_hours: number
}>({
  name: '',
  code: '',
  description: '',
  sort_order: 0,
  status: 'active',
  department_id: undefined,
  require_realname: false,
  require_binding: false,
  need_review: false,
  visible_role_codes: [],
  sla_hours: 0,
})

const columns: PrimaryTableCol<TicketCategoryInfo>[] = [
  { colKey: 'name', title: '分类名称', minWidth: 140 },
  { colKey: 'code', title: '编码', minWidth: 120 },
  { colKey: 'department_name', title: '归属部门', width: 120 },
  { colKey: 'preconditions', title: '规则', minWidth: 200 },
  { colKey: 'description', title: '描述', minWidth: 200, ellipsis: true },
  { colKey: 'sort_order', title: '排序', width: 80, align: 'center' as const },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 170, fixed: 'right' as const, align: 'center' as const },
]

async function loadCategories() {
  loading.value = true
  try {
    categories.value = await getTicketCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载分类列表失败')
  } finally {
    loading.value = false
  }
}

// 加载部门下拉（平铺，只取启用部门）
async function loadDepartments() {
  departmentLoading.value = true
  try {
    const data = await getDepartmentList({ status: 'active', flat: 1 })
    departmentOptions.value = (data.items ?? []).map((item: DepartmentInfo) => ({ label: item.name, value: item.id }))
  } catch {
    // 部门加载失败不阻断分类页主体
  } finally {
    departmentLoading.value = false
  }
}

// 加载角色下拉：仅后台（admin scope）角色可被选为可提交角色
async function loadRoles() {
  roleLoading.value = true
  try {
    const roles = await getRoleList()
    roleOptions.value = (roles ?? [])
      .filter((role: RoleInfo) => role.scope !== 'user')
      .map((role: RoleInfo) => ({ label: role.name, value: role.code }))
  } catch {
    // 角色加载失败不阻断分类页主体
  } finally {
    roleLoading.value = false
  }
}

function resetForm() {
  form.name = ''
  form.code = ''
  form.description = ''
  form.sort_order = 0
  form.status = 'active'
  form.department_id = undefined
  form.require_realname = false
  form.require_binding = false
  form.need_review = false
  form.visible_role_codes = []
  form.sla_hours = 0
}

function openCreate() {
  editingId.value = null
  resetForm()
  formVisible.value = true
}

function openEdit(row: TicketCategoryInfo) {
  editingId.value = row.id
  form.name = row.name
  form.code = row.code
  form.description = row.description
  form.sort_order = row.sort_order
  form.status = row.status
  form.department_id = row.department_id || undefined
  form.require_realname = !!row.require_realname
  form.require_binding = !!row.require_binding
  form.need_review = !!row.need_review
  form.visible_role_codes = [...(row.visible_role_codes ?? [])]
  form.sla_hours = row.sla_hours ?? 0
  formVisible.value = true
}

function buildPayload(): TicketCategorySaveRequest {
  return {
    name: form.name.trim(),
    code: form.code.trim(),
    description: form.description,
    sort_order: form.sort_order,
    status: form.status,
    department_id: form.department_id,
    require_realname: form.require_realname,
    require_binding: form.require_binding,
    need_review: form.need_review,
    visible_role_codes: form.visible_role_codes,
    sla_hours: form.sla_hours,
  }
}

async function handleSave() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入分类名称')
    return
  }
  if (!form.code.trim()) {
    MessagePlugin.warning('请输入分类编码')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateTicketCategory(editingId.value, buildPayload())
      MessagePlugin.success('分类已更新')
    } else {
      await createTicketCategory(buildPayload())
      MessagePlugin.success('分类已创建')
    }
    formVisible.value = false
    loadCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存分类失败')
  } finally {
    saving.value = false
  }
}

// 启用/禁用切换：直接复用当前行数据组装完整请求，避免覆盖 S2 新字段
async function handleToggleStatus(row: TicketCategoryInfo) {
  const target = row.status === 'active' ? 'disabled' : 'active'
  try {
    await updateTicketCategory(row.id, {
      name: row.name,
      code: row.code,
      description: row.description,
      sort_order: row.sort_order,
      status: target,
      department_id: row.department_id || undefined,
      require_realname: row.require_realname,
      require_binding: row.require_binding,
      need_review: row.need_review,
      visible_role_codes: row.visible_role_codes ?? [],
      sla_hours: row.sla_hours ?? 0,
    })
    MessagePlugin.success(target === 'active' ? '分类已启用' : '分类已禁用')
    loadCategories()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '状态更新失败')
  }
}

function handleDelete(row: TicketCategoryInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除分类',
    body: `确认删除分类「${row.name}」吗？删除后用户端不可再选择该分类。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteTicketCategory(row.id)
        MessagePlugin.success('分类已删除')
        dialog.destroy()
        loadCategories()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(() => {
  loadCategories()
  loadDepartments()
  loadRoles()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: TicketCategoryInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'toggle':
      void handleToggleStatus(row)
      break
    case 'delete':
      void handleDelete(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
/* 表单两列栅格：窄屏自动堆叠 */
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
  line-height: 1.6;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
