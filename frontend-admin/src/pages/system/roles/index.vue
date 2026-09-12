<template>
  <div class="page-container system-page">
    <div class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <UsergroupIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2>角色列表</h2>
        </div>
      </div>
      <t-button theme="primary" @click="openCreate">
        <template #icon>
          <AddIcon />
        </template>
        新增角色
      </t-button>
    </div>

    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="onSearch">
        <t-form-item label="关键词">
          <t-input v-model="filters.keyword" placeholder="角色名称 / 编码" clearable />
        </t-form-item>
        <t-form-item label="状态">
          <t-select v-model="filters.status" :options="statusOptions" clearable placeholder="全部状态" />
        </t-form-item>
      </t-form>
      <div class="filter-form__actions">
        <t-space>
          <t-button theme="primary" type="submit" @click.prevent="onSearch">查询</t-button>
          <t-button variant="outline" @click="resetFilters">重置</t-button>
        </t-space>
      </div>
    </t-card>

    <t-card :bordered="false" class="table-card">
      <t-table row-key="id" :data="pagedRoles" :columns="columns" :loading="loading" hover size="medium">
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #description="{ row }">
          <span>{{ row.description || '-' }}</span>
        </template>
        <template #operation="{ row }">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([
              { content: '编辑', value: 'edit' },
              { content: '权限分配', value: 'permissions' },
              { content: '删除', value: 'delete', theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="small">
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" hover="color" @click="goPermissions(row.id)">权限分配</t-link>
            <t-popconfirm content="确认删除该角色？已绑定管理员的角色应先解除绑定。" @confirm="removeRole(row.id)">
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>

      <div class="pagination-wrapper">
        <t-pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="filteredRoles.length"
          show-jumper
          show-page-size
          :page-size-options="[10, 20, 50]"
        />
      </div>
    </t-card>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑角色' : '新增角色'"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="500px"
      @confirm="submitRole"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <t-form-item label="角色名称" name="name">
          <t-input v-model="form.name" maxlength="64" placeholder="请输入角色名称" />
        </t-form-item>
        <t-form-item label="角色编码" name="code">
          <t-input v-model="form.code" maxlength="64" :disabled="editing" placeholder="请输入角色编码（如 super_admin）" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio value="active">启用</t-radio>
            <t-radio value="disabled">禁用</t-radio>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="角色描述" name="description">
          <t-textarea v-model="form.description" placeholder="请输入角色描述信息" :maxlength="255" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, UsergroupIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PrimaryTableCol, SubmitContext } from 'tdesign-vue-next'
import { useRouter } from 'vue-router'

import { createRole, deleteRole, getRoleList, updateRole, type RoleInfo } from '@/api/user'
import MobileAction from '@/components/mobile-action/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'SystemRoles' })

const router = useRouter()
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const formRef = ref<FormInstanceFunctions | null>(null)
const page = ref(1)
const pageSize = ref(10)

const roles = ref<RoleInfo[]>([])
const filters = reactive({
  keyword: '',
  status: '',
})
const form = reactive({
  id: 0,
  name: '',
  code: '',
  status: 'active',
  description: '',
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入角色名称', type: 'error' }],
  code: [{ required: true, message: '请输入角色编码', type: 'error' }],
  status: [{ required: true, message: '请选择状态', type: 'error' }],
}

const { isMobile } = useIsMobile()

const columns = computed<PrimaryTableCol<RoleInfo>[]>(() => [
  { colKey: 'id', title: 'ID', width: 80 },
  { colKey: 'name', title: '角色名称', minWidth: 160 },
  { colKey: 'code', title: '角色编码', minWidth: 200 },
  { colKey: 'description', title: '描述', minWidth: 220 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'updated_at', title: '更新时间', width: 180 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 220, fixed: 'right' },
])

const filteredRoles = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase()
  return roles.value.filter((role) => {
    const matchStatus = !filters.status || role.status === filters.status
    const matchKeyword =
      !keyword || `${role.name} ${role.code} ${role.description || ''}`.toLowerCase().includes(keyword)
    return matchStatus && matchKeyword
  })
})

const pagedRoles = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredRoles.value.slice(start, start + pageSize.value)
})

async function loadRoles() {
  loading.value = true
  try {
    roles.value = await getRoleList()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载角色失败')
  } finally {
    loading.value = false
  }
}

function onSearch(_context?: SubmitContext) {
  page.value = 1
}

function resetFilters() {
  filters.keyword = ''
  filters.status = ''
  page.value = 1
}

function openCreate() {
  editing.value = false
  Object.assign(form, {
    id: 0,
    name: '',
    code: '',
    status: 'active',
    description: '',
  })
  dialogVisible.value = true
}

function openEdit(row: RoleInfo) {
  editing.value = true
  Object.assign(form, {
    id: row.id,
    name: row.name,
    code: row.code,
    status: row.status,
    description: row.description || '',
  })
  dialogVisible.value = true
}

async function submitRole() {
  const validateResult = await formRef.value?.validate?.()
  if (validateResult !== true) return

  submitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      code: form.code.trim(),
      status: form.status,
      description: form.description.trim(),
    }

    if (editing.value) {
      await updateRole(form.id, payload)
    } else {
      await createRole(payload)
    }

    MessagePlugin.success('角色已保存')
    dialogVisible.value = false
    await loadRoles()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存角色失败')
  } finally {
    submitting.value = false
  }
}

async function removeRole(id: number) {
  try {
    await deleteRole(id)
    MessagePlugin.success('角色已删除')
    await loadRoles()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除角色失败')
  }
}

function goPermissions(id: number) {
  router.push({
    name: 'SystemPermissions',
    query: { role_id: String(id) },
  })
}

onMounted(loadRoles)
function handleMobileAction(value: string | number | Record<string, any>, row: RoleInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '');
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'permissions':
      goPermissions(row.id)
      break
    case 'delete':
      void removeRole(row.id)
      break
  }
}
</script>

<style scoped>
@import '../shared.css';

.filter-form__actions {
  display: flex;
  justify-content: flex-end;
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
  align-items: center;
  gap: 16px;
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
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
