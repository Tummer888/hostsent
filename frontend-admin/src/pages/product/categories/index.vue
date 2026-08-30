<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">分类管理</h2>
          <p class="page-header__desc">维护产品管理的分类树，支持多级子分类与排序。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="expandAll">全部展开</t-button>
        <t-button variant="outline" @click="collapseAll">全部收起</t-button>
        <t-button theme="primary" :loading="loading" @click="loadData">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate(null)">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建分类
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">分类树</h3>
        <span class="table-card__meta">共 {{ total }} 个分类</span>
      </div>
      <t-loading :loading="loading">
        <div class="tree-wrapper">
          <t-tree
            v-model:expanded="expandedKeys"
            :data="treeData"
            line
            row-key="id"
            :keys="{ label: 'name', value: 'id', children: 'children' }"
            :default-expand-all="true"
          >
            <template #label="{ node }">
              <div class="category-node">
                <span class="category-node__name">{{ node.data.name }}</span>
                <t-tag
                  v-if="node.data.status !== 1"
                  :theme="node.data.status === 0 ? 'default' : 'danger'"
                  variant="light"
                  size="small"
                >{{ statusTag(node.data.status).text }}</t-tag>
                <span class="category-node__meta">排序 {{ node.data.sort_order }}</span>
                <t-space size="small" class="category-node__actions">
                  <t-link theme="primary" hover="color" @click.stop="openCreate(node.data)">添加子分类</t-link>
                  <t-link theme="primary" hover="color" @click.stop="openEdit(node.data)">编辑</t-link>
                  <t-link theme="danger" hover="color" @click.stop="handleDelete(node.data)">删除</t-link>
                </t-space>
              </div>
            </template>
          </t-tree>
        </div>
      </t-loading>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="formMode === 'create' ? '新建分类' : '编辑分类'"
      width="520px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      :loading="saving"
      @confirm="handleSubmit"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="上级分类" name="parent_id">
          <t-select v-model="form.parent_id" clearable placeholder="不选则为顶级分类" :options="parentOptions" />
        </t-form-item>
        <t-form-item label="分类名称" name="name" :rules="[{ required: true, message: '请输入分类名称' }]">
          <t-input v-model="form.name" placeholder="请输入分类名称" />
        </t-form-item>
        <t-form-item label="展示顺序" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" placeholder="数值越小越靠前" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio :value="1">启用</t-radio>
            <t-radio :value="0">停用</t-radio>
          </t-radio-group>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type TreeOptionData } from 'tdesign-vue-next'

import {
  createProductCategory,
  deleteProductCategory,
  getProductCategoryList,
  updateProductCategory,
} from '@/api/product'
import { statusTag } from '@/pages/product/constants'
import type { SaleProductCategoryInfo } from '@/types/interface'

defineOptions({ name: 'ProductCategories' })

const treeData = ref<SaleProductCategoryInfo[]>([])
const expandedKeys = ref<Array<string | number>>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)

type FormMode = 'create' | 'edit'
const formMode = ref<FormMode>('create')

const form = reactive<{ parent_id: number | undefined; name: string; sort_order: number; status: number }>({
  parent_id: undefined,
  name: '',
  sort_order: 0,
  status: 1,
})

let editingId = 0
let originalParentId = 0

const total = computed(() => countNodes(treeData.value))

const parentOptions = computed(() => {
  const options: { label: string; value: number }[] = []
  const walk = (nodes: SaleProductCategoryInfo[], depth: number) => {
    for (const node of nodes) {
      if (node.id !== editingId) {
        options.push({ label: `${'　'.repeat(depth)}${node.name}`, value: node.id })
      }
      if (node.children?.length) walk(node.children, depth + 1)
    }
  }
  walk(treeData.value, 0)
  return options
})

function countNodes(nodes: SaleProductCategoryInfo[]): number {
  let n = 0
  for (const node of nodes) {
    n += 1
    if (node.children?.length) n += countNodes(node.children)
  }
  return n
}

function getAllNodeIds(nodes: SaleProductCategoryInfo[]): Array<string | number> {
  const ids: Array<string | number> = []
  for (const node of nodes) {
    ids.push(node.id)
    if (node.children?.length) ids.push(...getAllNodeIds(node.children))
  }
  return ids
}

function expandAll() {
  expandedKeys.value = getAllNodeIds(treeData.value)
}

function collapseAll() {
  expandedKeys.value = []
}

async function loadData() {
  loading.value = true
  try {
    const data = await getProductCategoryList()
    treeData.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载分类失败')
  } finally {
    loading.value = false
  }
}

function openCreate(parent: { id: number; name: string } | null) {
  formMode.value = 'create'
  editingId = 0
  originalParentId = 0
  form.parent_id = parent ? parent.id : undefined
  form.name = ''
  form.sort_order = 0
  form.status = 1
  dialogVisible.value = true
}

function openEdit(node: SaleProductCategoryInfo) {
  formMode.value = 'edit'
  editingId = node.id
  originalParentId = node.parent_id
  form.parent_id = node.parent_id
  form.name = node.name
  form.sort_order = node.sort_order
  form.status = node.status
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入分类名称')
    return
  }
  saving.value = true
  try {
    if (formMode.value === 'create') {
      await createProductCategory({
        parent_id: form.parent_id,
        name: form.name.trim(),
        sort_order: form.sort_order,
        status: form.status,
      })
      MessagePlugin.success('分类已创建')
    } else {
      await updateProductCategory(editingId, {
        parent_id: form.parent_id,
        name: form.name.trim(),
        sort_order: form.sort_order,
        status: form.status,
      })
      MessagePlugin.success('分类已更新')
    }
    dialogVisible.value = false
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存分类失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete(node: SaleProductCategoryInfo) {
  try {
    await deleteProductCategory(node.id)
    MessagePlugin.success('分类已删除')
    loadData()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除分类失败')
  }
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style lang="css" scoped>
.tree-wrapper {
  max-height: 560px;
  overflow: auto;
}

.category-node {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.category-node__name {
  font-weight: 600;
  color: #334155;
}

.category-node__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.category-node__actions {
  margin-left: auto;
  opacity: 0;
  transition: opacity var(--hs-duration-fast);
}

.category-node:hover .category-node__actions {
  opacity: 1;
}

@media (max-width: 768px) {
  .category-node__meta {
    display: none;
  }
}
</style>
