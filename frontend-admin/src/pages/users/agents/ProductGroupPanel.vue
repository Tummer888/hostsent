<template>
  <div class="pg-panel">
    <section class="pg-card surface-card">
      <div class="pg-head">
        <div>
          <h3 class="pg-title">② 商品分组（管商品，不管人）</h3>
          <p class="pg-desc">
            把分类和/或单个商品划成命名集合；到「③ 折扣组」把折扣组绑定到商品分组后一键应用。
          </p>
        </div>
        <t-button v-permission="'agent_level:create'" theme="primary" @click="openCreate">新建商品分组</t-button>
      </div>

      <t-table
        row-key="id"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        cell-empty-content="—"
        size="small"
        hover
      >
        <template #name="{ row }">
          <div class="pg-cell">
            <strong>{{ row.name }}</strong>
            <span class="muted">{{ row.code }}</span>
          </div>
        </template>
        <template #items="{ row }">
          <div class="pg-items">
            <t-tag
              v-for="item in (row.items || []).slice(0, 6)"
              :key="`${item.target_type}:${item.target_id}`"
              size="small"
              variant="light"
              :theme="item.target_type === 'category' ? 'primary' : 'warning'"
              shape="round"
            >
              {{ item.target_type === 'category' ? '分类' : '商品' }}·{{ item.target_name }}
            </t-tag>
            <span v-if="(row.items || []).length > 6" class="muted">等 {{ row.items.length }} 项</span>
            <span v-if="!(row.items || []).length" class="muted">空分组</span>
          </div>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <t-space size="8px">
            <t-link v-permission="'agent_level:update'" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-popconfirm content="确认删除该商品分组？（被折扣组绑定时会被拒绝）" @confirm="handleDelete(row)">
              <t-link v-permission="'agent_level:delete'" theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑商品分组' : '新建商品分组'"
      width="640px"
      :confirm-btn="{ content: '保存', loading: submitting }"
      @confirm="handleSubmit"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="分组名称" name="name">
            <t-input v-model="form.name" placeholder="如：云主机主力款" />
          </t-form-item>
          <t-form-item label="分组编码" name="code">
            <t-input v-model="form.code" placeholder="如：vhost_main" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
        </div>
        <t-form-item label="说明" name="description">
          <t-input v-model="form.description" placeholder="选填" />
        </t-form-item>
        <t-form-item label="包含的分类" name="categories">
          <t-select
            v-model="form.categoryIds"
            :options="categoryOptions"
            multiple
            filterable
            clearable
            placeholder="可多选分类（含子分类）"
          />
        </t-form-item>
        <t-form-item label="包含的商品" name="products">
          <t-select
            v-model="form.productIds"
            :options="productOptions"
            multiple
            filterable
            clearable
            placeholder="可多选单个商品（未上架的带标注）"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PrimaryTableCol } from 'tdesign-vue-next'

import {
  createProductGroup,
  deleteProductGroup,
  getProductGroupList,
  updateProductGroup,
  type ProductGroupInfo,
  type ProductGroupRequest,
} from '@/api/agent-level'
import { getProductCategoryList, getProductList } from '@/api/product'
import type { SaleProductInfo } from '@/types/product'

defineOptions({ name: 'ProductGroupPanel' })

const emit = defineEmits<{ changed: [] }>()

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const editingId = ref<number | null>(null)
const tableData = ref<ProductGroupInfo[]>([])

const categoryOptions = ref<{ label: string; value: number }[]>([])
const productOptions = ref<{ label: string; value: number }[]>([])

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

const columns = computed<PrimaryTableCol<ProductGroupInfo>[]>(() => [
  { colKey: 'name', title: '商品分组', minWidth: 180 },
  { colKey: 'items', title: '成员', minWidth: 380 },
  { colKey: 'item_count', title: '成员数', width: 90 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'operation', title: '操作', width: 120 },
])

const formRef = ref<FormInstanceFunctions>()
const form = reactive<{
  name: string
  code: string
  description: string
  status: string
  categoryIds: number[]
  productIds: number[]
}>({
  name: '',
  code: '',
  description: '',
  status: 'active',
  categoryIds: [],
  productIds: [],
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入分组名称', type: 'error' }],
  code: [{ required: true, message: '请输入分组编码', type: 'error' }],
  status: [{ required: true, message: '请选择状态', type: 'error' }],
}

interface CategoryNode {
  id: number
  name: string
  status?: number
  children?: unknown[]
}

async function loadOptions() {
  try {
    const data = await getProductCategoryList()
    const flat: { label: string; value: number }[] = []
    const walk = (items: CategoryNode[], depth: number) => {
      for (const item of items) {
        if (item.status !== undefined && item.status !== 1) continue
        const prefix = depth === 0 ? '' : `${'　'.repeat(depth - 1)}└ `
        flat.push({ label: `${prefix}${item.name}`, value: item.id })
        if (Array.isArray(item.children) && item.children.length) walk(item.children as CategoryNode[], depth + 1)
      }
    }
    walk((data.items || []) as unknown as CategoryNode[], 0)
    categoryOptions.value = flat
  } catch {
    categoryOptions.value = []
  }
  try {
    // 全量商品（未上架标注后缀排后）：预配分组时商品可能还没上架
    const data = await getProductList({ page: 1, page_size: 500 } as never)
    const items = (data.items || []) as Array<SaleProductInfo>
    const onSale: { label: string; value: number }[] = []
    const offShelf: { label: string; value: number }[] = []
    for (const item of items) {
      const entry = { label: item.status === 1 ? item.name : `${item.name}（已下架）`, value: item.id }
      ;(item.status === 1 ? onSale : offShelf).push(entry)
    }
    productOptions.value = [...onSale, ...offShelf]
  } catch {
    productOptions.value = []
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await getProductGroupList()
    tableData.value = data.items || []
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载商品分组失败')
  } finally {
    loading.value = false
  }
}

function emptyForm() {
  return {
    name: '',
    code: '',
    description: '',
    status: 'active',
    categoryIds: [] as number[],
    productIds: [] as number[],
  }
}

function openCreate() {
  editing.value = false
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function openEdit(row: ProductGroupInfo) {
  editing.value = true
  editingId.value = row.id
  const categoryIds: number[] = []
  const productIds: number[] = []
  for (const item of row.items || []) {
    if (item.target_type === 'category') categoryIds.push(item.target_id)
    else productIds.push(item.target_id)
  }
  Object.assign(form, {
    name: row.name,
    code: row.code,
    description: row.description || '',
    status: row.status,
    categoryIds,
    productIds,
  })
  dialogVisible.value = true
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    const items = [
      ...form.categoryIds.map((id) => ({ target_type: 'category' as const, target_id: id })),
      ...form.productIds.map((id) => ({ target_type: 'product' as const, target_id: id })),
    ]
    const payload: ProductGroupRequest = {
      name: form.name,
      code: form.code,
      description: form.description,
      status: form.status,
      items,
    }
    if (editing.value && editingId.value != null) {
      await updateProductGroup(editingId.value, payload)
      MessagePlugin.success('商品分组已更新')
    } else {
      await createProductGroup(payload)
      MessagePlugin.success('商品分组已创建')
    }
    dialogVisible.value = false
    await loadData()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

function handleDelete(row: ProductGroupInfo) {
  void (async () => {
    try {
      await deleteProductGroup(row.id)
      MessagePlugin.success('已删除')
      await loadData()
      emit('changed')
    } catch (error) {
      MessagePlugin.error((error as Error)?.message || '删除失败')
    }
  })()
}

onMounted(async () => {
  await Promise.all([loadData(), loadOptions()])
})

defineExpose({ openCreate })
</script>

<style scoped>
.pg-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.surface-card {
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  box-shadow: none;
}

.pg-card {
  padding: 16px 20px;
}

.pg-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.pg-title {
  margin: 0;
  font-size: 16px;
}

.pg-desc {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.pg-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pg-items {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.muted {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
