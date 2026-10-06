<template>
  <div class="scheme-panel">
    <section class="scheme-card surface-card">
      <div class="scheme-head">
        <div>
          <h3 class="scheme-title">折扣组</h3>
          <p class="scheme-desc">
            一行 = 一个折扣组（如「6折组」），列是<strong>代理分组</strong>；绑定一个商品分组后点「应用」，
            折扣率即展开写入该分组内全部分类/商品（受成本线与代理分组权重单调性约束）。
          </p>
        </div>
        <t-button v-permission="'agent_level:create'" theme="primary" @click="openCreate">新建折扣组</t-button>
      </div>

      <t-loading :loading="loading" size="small">
        <div v-if="rows.length === 0" class="empty-hint">
          还没有折扣组，点右上角「新建折扣组」开始（建议按档位命名，如 6折组、5折组）。
        </div>
        <div v-else class="scheme-scroll">
          <table class="scheme-table">
            <thead>
              <tr>
                <th class="scheme-table__name">折扣组</th>
                <th class="scheme-table__group">绑定商品分组</th>
                <th v-for="level in activeLevels" :key="level.id" class="scheme-table__level">
                  <div class="level-head">
                    <span>{{ level.name }}</span>
                    <span class="level-head__meta">权重 {{ level.weight }}</span>
                  </div>
                </th>
                <th class="scheme-table__op">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="row.id">
                <td class="scheme-table__name">
                  <t-input v-model="row.draft.name" size="small" placeholder="组名称" class="scheme-name-input" />
                </td>
                <td class="scheme-table__group">
                  <t-select
                    v-model="row.draft.product_group_id"
                    :options="groupOptions"
                    size="small"
                    clearable
                    filterable
                    placeholder="无"
                    class="scheme-group-select"
                  />
                </td>
                <td v-for="level in activeLevels" :key="level.id" class="scheme-table__level">
                  <t-input-number
                    v-model="row.draft.rates[level.id]"
                    size="small"
                    :min="0"
                    :max="1"
                    :step="0.01"
                    :precision="2"
                    theme="normal"
                    class="scheme-rate"
                  />
                </td>
                <td class="scheme-table__op">
                  <t-space size="6px">
                    <t-link
                      v-permission="'agent_level:update'"
                      theme="primary"
                      hover="color"
                      :disabled="row.saving"
                      @click="saveRow(row)"
                    >
                      保存
                    </t-link>
                    <t-popconfirm content="把该组的折扣率写入绑定商品分组下的全部分类/商品？" @confirm="applyRow(row)">
                      <t-link v-permission="'agent_level:update'" theme="primary" hover="color" :disabled="row.applying">
                        应用
                      </t-link>
                    </t-popconfirm>
                    <t-popconfirm content="确认删除该折扣组？（不影响已写入的折扣矩阵）" @confirm="deleteRow(row)">
                      <t-link v-permission="'agent_level:delete'" theme="danger" hover="color">删除</t-link>
                    </t-popconfirm>
                  </t-space>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </t-loading>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      header="新建折扣组"
      width="480px"
      :confirm-btn="{ content: '创建', loading: submitting }"
      @confirm="handleCreate"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <t-form-item label="折扣组名称" name="name">
          <t-input v-model="form.name" placeholder="如：6折组（建议按档位命名）" />
        </t-form-item>
        <t-form-item label="编码" name="code">
          <t-input v-model="form.code" placeholder="如：scheme_60" />
        </t-form-item>
        <t-form-item label="绑定商品分组" name="product_group_id">
          <t-select v-model="form.product_group_id" :options="groupOptions" clearable filterable placeholder="可先不绑，应用前必须绑定" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import {
  applyScheme,
  createScheme,
  deleteScheme,
  getAgentLevelList,
  getProductGroupList,
  getSchemeList,
  updateScheme,
  type ProductGroupInfo,
  type SchemeInfo,
} from '@/api/agent-level'

defineOptions({ name: 'SchemePanel' })

const emit = defineEmits<{ changed: [] }>()

interface SchemeRow {
  id: number
  code: string
  description: string
  status: string
  saving: boolean
  applying: boolean
  draft: {
    name: string
    product_group_id: number | undefined
    rates: Record<number, number>
  }
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const rows = ref<SchemeRow[]>([])
const groups = ref<ProductGroupInfo[]>([])
const activeLevels = ref<Array<{ id: number; name: string; weight: number }>>([])

const groupOptions = computed(() =>
  groups.value.map((group) => ({
    label: `${group.name}（${group.item_count} 项）`,
    value: group.id,
  })),
)

const formRef = ref<FormInstanceFunctions>()
const form = reactive<{ name: string; code: string; product_group_id: number | undefined }>({
  name: '',
  code: '',
  product_group_id: undefined,
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入折扣组名称', type: 'error' }],
  code: [{ required: true, message: '请输入编码', type: 'error' }],
}

async function loadData() {
  loading.value = true
  try {
    const [schemeData, groupData, levelData] = await Promise.all([
      getSchemeList(),
      getProductGroupList(),
      getAgentLevelList({ page: 1, page_size: 100, status: 'active' }),
    ])
    groups.value = groupData.items || []
    activeLevels.value = (levelData.items || []).map((item) => ({
      id: item.id,
      name: item.name,
      weight: item.weight,
    }))
    // 行内可编辑草稿：费率列以「当前启用等级」为准；方案里已停用等级的费率不展示但保留在库。
    rows.value = (schemeData.items || []).map((scheme) => toRow(scheme))
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载折扣组失败')
  } finally {
    loading.value = false
  }
}

function toRow(scheme: SchemeInfo): SchemeRow {
  const rates: Record<number, number> = {}
  for (const item of scheme.items || []) rates[item.agent_level_id] = item.discount_rate
  return {
    id: scheme.id,
    code: scheme.code,
    description: scheme.description || '',
    status: scheme.status,
    saving: false,
    applying: false,
    draft: {
      name: scheme.name,
      product_group_id: scheme.product_group_id || undefined,
      rates,
    },
  }
}

function payloadOf(row: SchemeRow) {
  const items = activeLevels.value
    .map((level) => ({ agent_level_id: level.id, discount_rate: Number(row.draft.rates[level.id] || 0) }))
  return {
    name: row.draft.name,
    code: row.code,
    description: row.description,
    status: row.status,
    product_group_id: row.draft.product_group_id || 0,
    // items 非空数组 = 整体覆盖（含停用等级的历史费率，避免保存把库里的清掉）
    items,
  }
}

async function saveRow(row: SchemeRow) {
  if (!row.draft.name.trim()) {
    MessagePlugin.error('请填写折扣组名称')
    return
  }
  row.saving = true
  try {
    await updateScheme(row.id, payloadOf(row))
    MessagePlugin.success('折扣组已保存（尚未生效，点「应用」写入折扣矩阵）')
    await loadData()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    row.saving = false
  }
}

async function applyRow(row: SchemeRow) {
  // 应用前先保存草稿，避免"改了没存就应用"
  if (!row.draft.name.trim()) {
    MessagePlugin.error('请填写折扣组名称')
    return
  }
  row.applying = true
  try {
    await updateScheme(row.id, payloadOf(row))
    await applyScheme(row.id)
    MessagePlugin.success('已应用：折扣率已写入绑定商品分组下的全部分类/商品')
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '应用失败')
  } finally {
    row.applying = false
  }
}

async function deleteRow(row: SchemeRow) {
  try {
    await deleteScheme(row.id)
    MessagePlugin.success('已删除')
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除失败')
  }
}

function openCreate() {
  Object.assign(form, { name: '', code: '', product_group_id: undefined })
  dialogVisible.value = true
}

async function handleCreate() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    await createScheme({
      name: form.name,
      code: form.code,
      product_group_id: form.product_group_id || 0,
      items: activeLevels.value.map((level) => ({ agent_level_id: level.id, discount_rate: 0 })),
    })
    MessagePlugin.success('折扣组已创建，请在表格中填写各等级折扣率')
    dialogVisible.value = false
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

onMounted(loadData)

defineExpose({ reload: loadData })
</script>

<style scoped>
.scheme-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.surface-card {
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  box-shadow: none;
}

.scheme-card {
  padding: 16px 20px;
}

.scheme-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.scheme-title {
  margin: 0;
  font-size: 16px;
}

.scheme-desc {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.empty-hint {
  padding: 20px 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.scheme-scroll {
  overflow-x: auto;
}

.scheme-table {
  border-collapse: collapse;
  width: 100%;
  min-width: 720px;
  font-size: 13px;
}

.scheme-table th,
.scheme-table td {
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 8px 10px;
  text-align: center;
  white-space: nowrap;
}

.scheme-table__name {
  min-width: 160px;
  text-align: left;
}

.scheme-table__group {
  min-width: 200px;
}

.scheme-name-input {
  width: 150px;
}

.scheme-group-select {
  width: 190px;
}

.scheme-rate {
  width: 96px;
}

.level-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.level-head__meta {
  font-size: 11px;
  color: var(--color-muted-foreground);
}
</style>
