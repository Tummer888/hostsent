<template>
  <div class="level-page">
    <header class="page-header surface-card">
      <div>
        <h2 class="page-title">用户等级管理</h2>
        <p class="page-subtitle">
          等级按累计消费自动升级（只升不降），不参与折扣计算；门槛与子账号上限可直接调整，权益本期仅记录不自动发放。
        </p>
      </div>
      <t-space>
        <t-button v-permission="'level:create'" theme="primary" @click="openCreate">新建等级</t-button>
      </t-space>
    </header>

    <section class="toolbar surface-card">
      <t-space wrap>
        <t-input v-model="filters.keyword" clearable placeholder="搜索等级名称 / 编码" @enter="handleSearch" />
        <t-select v-model="filters.status" clearable placeholder="状态" :options="statusOptions" />
        <t-button theme="primary" @click="handleSearch">查询</t-button>
        <t-button variant="outline" @click="handleReset">重置</t-button>
      </t-space>
    </section>

    <section class="table-panel surface-card">
      <t-table
        row-key="id"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        cell-empty-content="—"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="primary-cell">
            <strong>{{ row.name }}</strong>
            <span>{{ row.code }}</span>
          </div>
        </template>
        <template #upgrade_threshold="{ row }">
          <span v-if="row.upgrade_threshold > 0">¥{{ formatMoney(row.upgrade_threshold) }}</span>
          <span v-else class="muted">注册即获得</span>
        </template>
        <template #max_sub_accounts="{ row }">{{ row.max_sub_accounts }}</template>
        <template #benefits="{ row }">
          <t-space size="4px" break-line>
            <t-tag v-for="item in parseBenefits(row.benefits)" :key="item" theme="primary" variant="light-outline" size="small">
              {{ item }}
            </t-tag>
            <span v-if="parseBenefits(row.benefits).length === 0" class="muted">—</span>
          </t-space>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <t-space size="8px">
            <t-link v-permission="'level:update'" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link v-permission="'level:delete'" theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑用户等级' : '新建用户等级'"
      width="620px"
      :confirm-loading="submitting"
      @confirm="handleSubmit"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="等级名称" name="name">
            <t-input v-model="form.name" placeholder="如：企业用户" />
          </t-form-item>
          <t-form-item label="等级编码" name="code">
            <t-input v-model="form.code" placeholder="如：business" />
          </t-form-item>
          <t-form-item label="权重" name="weight">
            <t-input-number v-model="form.weight" :min="0" :step="1" theme="normal" />
            <p class="form-tip">权重越大等级越高，只升不降按此比较。</p>
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
          <t-form-item label="升级门槛（累计消费，元）" name="upgrade_threshold">
            <t-input-number v-model="form.upgrade_threshold" :min="0" :step="1000" theme="normal" />
            <p class="form-tip">0 表示注册即获得（最低等级）。</p>
          </t-form-item>
          <t-form-item label="子账号上限" name="max_sub_accounts">
            <t-input-number v-model="form.max_sub_accounts" :min="0" :step="1" theme="normal" />
            <p class="form-tip">该等级用户可创建的成员数量。</p>
          </t-form-item>
        </div>
        <t-form-item label="权益（每行一条）" name="benefits">
          <t-textarea v-model="benefitsText" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="高优先级工单&#10;每日自动备份" />
        </t-form-item>
        <t-form-item label="升级条件说明" name="upgrade_condition">
          <t-input v-model="form.upgrade_condition" placeholder="展示用文案，如：累计消费满 10000 元" />
        </t-form-item>
        <t-form-item label="功能标记" name="feature_flags">
          <t-input v-model="form.feature_flags" placeholder="逗号分隔，如：snapshot,backup,ha" />
        </t-form-item>
        <t-form-item label="说明" name="description">
          <t-input v-model="form.description" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  createUserLevel,
  deleteUserLevel,
  getUserLevelList,
  updateUserLevel,
  type UserLevelInfo,
  type UserLevelListQuery,
  type UserLevelRequest,
} from '@/api/user'

defineOptions({ name: 'UserLevels' })

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const editingId = ref<number | null>(null)
const benefitsText = ref('')

const tableData = ref<UserLevelInfo[]>([])
const filters = reactive<UserLevelListQuery>({ page: 1, page_size: 10, keyword: '', status: '' })
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50, 100],
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const columns: PrimaryTableCol<UserLevelInfo>[] = [
  { colKey: 'name', title: '等级信息', minWidth: 180 },
  { colKey: 'weight', title: '权重', width: 90 },
  { colKey: 'upgrade_threshold', title: '升级门槛', width: 140 },
  { colKey: 'max_sub_accounts', title: '子账号上限', width: 120 },
  { colKey: 'benefits', title: '权益', minWidth: 240 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'description', title: '说明', minWidth: 160 },
  { colKey: 'operation', title: '操作', width: 130, fixed: 'right' },
]

const formRef = ref<FormInstanceFunctions>()

function emptyForm(): UserLevelRequest {
  return {
    name: '',
    code: '',
    weight: 0,
    status: 'active',
    feature_flags: '',
    upgrade_condition: '',
    upgrade_threshold: 0,
    max_sub_accounts: 0,
    benefits: '',
    description: '',
  }
}

const form = reactive<UserLevelRequest>(emptyForm())

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入等级名称', type: 'error' }],
  code: [{ required: true, message: '请输入等级编码', type: 'error' }],
  status: [{ required: true, message: '请选择状态', type: 'error' }],
}

async function loadData() {
  loading.value = true
  try {
    const data = await getUserLevelList(filters)
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载用户等级失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  filters.page = 1
  pagination.current = 1
  void loadData()
}

function handleReset() {
  Object.assign(filters, { page: 1, page_size: 10, keyword: '', status: '' })
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  filters.page = pageInfo.current
  filters.page_size = pageInfo.pageSize
  void loadData()
}

function openCreate() {
  editing.value = false
  editingId.value = null
  Object.assign(form, emptyForm())
  benefitsText.value = ''
  dialogVisible.value = true
}

function openEdit(row: UserLevelInfo) {
  editing.value = true
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    code: row.code,
    weight: row.weight,
    status: row.status,
    feature_flags: row.feature_flags || '',
    upgrade_condition: row.upgrade_condition || '',
    upgrade_threshold: row.upgrade_threshold || 0,
    max_sub_accounts: row.max_sub_accounts || 0,
    benefits: row.benefits || '',
    description: row.description || '',
  })
  benefitsText.value = parseBenefits(row.benefits).join('\n')
  dialogVisible.value = true
}

function buildBenefitsPayload(lines: string[]): string {
  const items = lines.map((line) => line.trim()).filter(Boolean)
  if (items.length === 0) return ''
  return JSON.stringify({ benefits: items })
}

async function handleSubmit() {
  const result = await formRef.value?.validate()
  if (result !== true) return
  submitting.value = true
  try {
    const payload: UserLevelRequest = {
      ...form,
      benefits: buildBenefitsPayload(benefitsText.value.split('\n')),
    }
    if (editing.value && editingId.value != null) {
      await updateUserLevel(editingId.value, payload)
      MessagePlugin.success('等级已更新')
    } else {
      await createUserLevel(payload)
      MessagePlugin.success('等级已创建')
    }
    dialogVisible.value = false
    void loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

function handleDelete(row: UserLevelInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除等级',
    body: `确认删除等级「${row.name}」？已属于该等级的用户不会被自动降级。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteUserLevel(row.id)
        MessagePlugin.success('已删除')
        dialog.destroy()
        void loadData()
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

/** benefits 存 JSON 文本，兼容空值与历史脏数据 */
function parseBenefits(raw?: string): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) return parsed.map((item) => String(item))
    if (Array.isArray(parsed?.benefits)) return parsed.benefits.map((item: unknown) => String(item))
    return []
  } catch {
    return raw
      .split(/[,，\n]/)
      .map((item) => item.trim())
      .filter(Boolean)
  }
}

function formatMoney(value: number): string {
  return value.toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 2 })
}

onMounted(() => {
  void loadData()
})
</script>

<style scoped>
.level-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-header,
.toolbar,
.table-panel {
  padding: 16px 20px;
}

.page-title {
  margin: 0;
  font-size: 22px;
}

.page-subtitle {
  margin: 8px 0 0;
  max-width: 720px;
  color: var(--color-muted-foreground);
  line-height: 1.6;
}

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.muted {
  color: var(--color-muted-foreground);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

.form-tip {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
