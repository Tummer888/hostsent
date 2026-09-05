<template>
  <div class="page-container">
    <!-- 页头：标题 + 新增入口 -->
    <div class="page-header">
      <div>
        <h2>系统配置</h2>
        <p>管理全局参数与功能开关</p>
      </div>
      <t-button theme="primary" @click="openCreate">
        <template #icon>
          <AddIcon />
        </template>
        新增配置
      </t-button>
    </div>

    <!-- 筛选卡片：分组 + 关键字 -->
    <t-card :bordered="false" class="filter-card">
      <t-form layout="inline" :data="filters" @submit="onSearch">
        <t-form-item label="分组">
          <t-select
            v-model="filters.group"
            :options="groupOptions"
            clearable
            placeholder="全部分组"
            class="filter-select"
          />
        </t-form-item>
        <t-form-item label="关键字">
          <t-input
            v-model="filters.keyword"
            placeholder="配置键 / 描述模糊匹配"
            clearable
            @enter="onSearch"
          />
        </t-form-item>
        <t-form-item>
          <t-space>
            <t-button theme="primary" type="submit">查询</t-button>
            <t-button variant="outline" @click="resetFilters">重置</t-button>
          </t-space>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 表格卡片 -->
    <t-card :bordered="false" class="table-card">
      <div v-if="errorMessage" class="table-error">
        <span>{{ errorMessage }}</span>
        <t-link theme="primary" hover="color" @click="loadData">重试</t-link>
      </div>

      <t-table
        row-key="id"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        hover
        size="medium"
        cell-empty-content="—"
        @page-change="handlePageChange"
      >
        <!-- 配置键：等宽字体展示 -->
        <template #config_key="{ row }">
          <span class="mono-text">{{ row.config_key }}</span>
        </template>
        <!-- 类型：不同主题 tag 区分 -->
        <template #value_type="{ row }">
          <t-tag :theme="valueTypeTagTheme[row.value_type] || 'default'" variant="light-outline">
            {{ row.value_type }}
          </t-tag>
        </template>
        <!-- 分组：中文映射标签，未收录分组展示原值 -->
        <template #config_group="{ row }">
          <t-tag theme="primary" variant="light">
            {{ groupLabelMap[row.config_group] || row.config_group }}
          </t-tag>
        </template>
        <template #description="{ row }">
          <span>{{ row.description || '-' }}</span>
        </template>
        <!-- 状态：启用/禁用 -->
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light-outline">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <t-space size="small">
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-popconfirm
              content="删除后不可恢复，确认删除该配置项？"
              @confirm="removeConfig(row.id)"
            >
              <t-link theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
        <template #empty>
          <t-empty description="暂无配置项" />
        </template>
      </t-table>
    </t-card>

    <!-- 新增 / 编辑共用弹窗 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑配置' : '新增配置'"
      :confirm-btn="{ content: '保存', loading: submitting }"
      width="520px"
      @confirm="submitConfig"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <t-form-item label="配置键" name="config_key">
          <t-input
            v-model="form.config_key"
            :disabled="editing"
            maxlength="64"
            placeholder="小写字母开头，仅小写字母数字下划线"
          />
        </t-form-item>
        <t-form-item label="配置值" name="config_value">
          <t-textarea
            v-model="form.config_value"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder="请输入配置值，JSON 类型请填写合法 JSON 文本"
          />
        </t-form-item>
        <t-form-item label="类型" name="value_type">
          <t-select v-model="form.value_type" :options="valueTypeOptions" placeholder="请选择类型" />
        </t-form-item>
        <t-form-item label="分组" name="config_group">
          <t-select
            v-model="form.config_group"
            :options="groupOptions"
            filterable
            creatable
            placeholder="请选择分组，支持输入创建新分组"
          />
        </t-form-item>
        <t-form-item label="描述" name="description">
          <t-textarea
            v-model="form.description"
            :autosize="{ minRows: 2, maxRows: 4 }"
            :maxlength="255"
            placeholder="请输入配置说明"
          />
        </t-form-item>
        <t-form-item label="排序" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" :max="9999" :step="1" theme="normal" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-switch v-model="form.status" :custom-value="['active', 'disabled']">
            <template #label="{ value }">
              {{ value === 'active' ? '启用' : '禁用' }}
            </template>
          </t-switch>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  createConfig,
  deleteConfig,
  getConfigList,
  updateConfig,
  type SystemConfigInfo,
} from '@/api/system'

defineOptions({ name: 'SystemConfig' })

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const formRef = ref<FormInstanceFunctions | null>(null)
const tableData = ref<SystemConfigInfo[]>([])
const errorMessage = ref('')

const filters = reactive({
  group: '',
  keyword: '',
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50],
})

/** 分组下拉选项（筛选与表单共用，表单允许自定义创建） */
const groupOptions = [
  { label: '站点配置', value: 'site' },
  { label: '计费配置', value: 'billing' },
  { label: '功能开关', value: 'feature' },
  { label: '安全', value: 'security' },
  { label: '订单', value: 'order' },
]

const groupLabelMap: Record<string, string> = Object.fromEntries(
  groupOptions.map((option) => [option.value, option.label]),
)

/** 类型下拉选项 */
const valueTypeOptions = [
  { label: 'string（字符串）', value: 'string' },
  { label: 'bool（布尔）', value: 'bool' },
  { label: 'int（整数）', value: 'int' },
  { label: 'json（JSON）', value: 'json' },
]

/** 类型对应的 tag 主题 */
const valueTypeTagTheme: Record<string, string> = {
  string: 'default',
  bool: 'warning',
  int: 'primary',
  json: 'success',
}

const rules: Record<string, FormRule[]> = {
  config_key: [
    { required: true, message: '请输入配置键', type: 'error' },
    { pattern: /^[a-z][a-z0-9_]*$/, message: '小写字母开头，仅小写字母数字下划线', type: 'error' },
  ],
  value_type: [{ required: true, message: '请选择类型', type: 'error' }],
  sort_order: [
    {
      validator: (value) => Number.isInteger(Number(value)),
      message: '排序必须为整数',
      type: 'error',
    },
  ],
}

const columns: PrimaryTableCol<SystemConfigInfo>[] = [
  { colKey: 'config_key', title: '配置键', minWidth: 200 },
  { colKey: 'config_value', title: '配置值', minWidth: 200, ellipsis: true },
  { colKey: 'value_type', title: '类型', width: 110 },
  { colKey: 'config_group', title: '分组', width: 130 },
  { colKey: 'description', title: '描述', minWidth: 200, ellipsis: true },
  { colKey: 'sort_order', title: '排序', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'operation', title: '操作', width: 130, fixed: 'right' },
]

const initForm = () => ({
  id: 0,
  config_key: '',
  config_value: '',
  value_type: 'string' as SystemConfigInfo['value_type'],
  config_group: 'site',
  description: '',
  sort_order: 0,
  status: 'active' as SystemConfigInfo['status'],
})

const form = reactive(initForm())

/** 加载配置列表（服务端分页 + 筛选） */
async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getConfigList({
      group: filters.group || undefined,
      keyword: filters.keyword.trim() || undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    tableData.value = response.items
    pagination.total = response.meta.total
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载系统配置失败'
  } finally {
    loading.value = false
  }
}

function onSearch() {
  pagination.current = 1
  void loadData()
}

function resetFilters() {
  filters.group = ''
  filters.keyword = ''
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

function openCreate() {
  editing.value = false
  Object.assign(form, initForm())
  dialogVisible.value = true
}

function openEdit(row: SystemConfigInfo) {
  editing.value = true
  Object.assign(form, {
    id: row.id,
    config_key: row.config_key,
    config_value: row.config_value,
    value_type: row.value_type,
    config_group: row.config_group,
    description: row.description || '',
    sort_order: row.sort_order,
    status: row.status,
  })
  dialogVisible.value = true
}

async function submitConfig() {
  const validateResult = await formRef.value?.validate?.()
  if (validateResult !== true) return

  submitting.value = true
  try {
    if (editing.value) {
      await updateConfig(form.id, {
        config_value: form.config_value,
        value_type: form.value_type,
        config_group: form.config_group,
        description: form.description.trim(),
        sort_order: form.sort_order,
        status: form.status,
      })
    } else {
      await createConfig({
        config_key: form.config_key.trim(),
        config_value: form.config_value,
        value_type: form.value_type,
        config_group: form.config_group,
        description: form.description.trim(),
        sort_order: form.sort_order,
        status: form.status,
      })
    }

    MessagePlugin.success('配置已保存')
    dialogVisible.value = false
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存配置失败')
  } finally {
    submitting.value = false
  }
}

async function removeConfig(id: number) {
  try {
    await deleteConfig(id)
    MessagePlugin.success('配置已删除')
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除配置失败')
  }
}

onMounted(() => {
  void loadData()
})
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

.filter-select {
  min-width: 180px;
}

.mono-text {
  font-family: var(--hs-font-mono, ui-monospace, Menlo, Consolas, monospace);
  font-size: 13px;
}

.table-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  padding: 10px 12px;
  border: 1px solid rgba(239, 68, 68, 0.18);
  border-radius: var(--td-radius-medium);
  background: rgba(239, 68, 68, 0.06);
  color: var(--td-error-color);
}
</style>
