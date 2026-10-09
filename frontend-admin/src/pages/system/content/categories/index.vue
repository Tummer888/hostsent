<template>
  <div class="page-body content-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FolderIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">内容分类</h2>
          <p class="page-header__desc">新闻分栏与帮助中心目录树</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="expandAll">全部展开</t-button>
        <t-button variant="outline" @click="collapseAll">全部收起</t-button>
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate(null)">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建分类
        </t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeKind" :space-evenly="false" @change="handleKindChange">
        <t-tab-panel value="news" label="新闻分栏" />
        <t-tab-panel value="help" label="帮助目录" />
      </t-tabs>

      <div class="table-card__head" style="margin-top: 14px">
        <h3 class="card-title">{{ activeKind === 'news' ? '新闻分栏' : '帮助目录树' }}</h3>
        <span class="table-card__meta">
          共 {{ total }} 个分类 ·
          {{ activeKind === 'news' ? '新闻为平铺分栏，一般不建三级' : '帮助文档靠父子关系组成左侧目录树' }}
        </span>
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
                  v-if="node.data.status !== 'active'"
                  theme="default"
                  variant="light"
                  size="small"
                  shape="round"
                >{{ categoryStatusLabel(node.data.status) }}</t-tag>
                <span class="category-node__meta">/{{ node.data.slug }} · 排序 {{ node.data.sort_order }}</span>
                <t-space size="small" class="category-node__actions">
                  <t-link theme="primary" hover="color" @click.stop="openCreate(node.data)">添加子分类</t-link>
                  <t-link theme="primary" hover="color" @click.stop="openEdit(node.data)">编辑</t-link>
                  <t-link theme="danger" hover="color" @click.stop="handleDelete(node.data)">删除</t-link>
                </t-space>
              </div>
            </template>
          </t-tree>
        </div>
        <t-empty v-if="!loading && !treeData.length" :description="`暂无${activeKind === 'news' ? '新闻分栏' : '帮助目录'}`" />
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
          <t-input v-model="form.name" placeholder="请输入分类名称" :maxlength="64" />
        </t-form-item>
        <t-form-item label="URL 标识（slug）" name="slug">
          <t-input v-model="form.slug" placeholder="留空自动生成" :maxlength="64" />
          <p class="field-help">用于门户分类筛选地址，如 /news?category=getting-started。</p>
        </t-form-item>
        <t-form-item label="图标" name="icon">
          <t-input v-model="form.icon" placeholder="可选，图标名（如 rocket）" :maxlength="64" />
        </t-form-item>
        <t-form-item label="描述" name="description">
          <t-textarea
            v-model="form.description"
            placeholder="可选，分类说明"
            :autosize="{ minRows: 2, maxRows: 3 }"
            :maxlength="255"
          />
        </t-form-item>
        <t-form-item label="排序" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" placeholder="数值越小越靠前" />
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio value="active">启用</t-radio>
            <t-radio value="disabled">停用</t-radio>
          </t-radio-group>
          <p class="field-help">停用后该分类不在门户展示；其下内容不会被删除，但入口会消失。</p>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, FolderIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'

import {
  createContentCategory,
  deleteContentCategory,
  getContentCategories,
  updateContentCategory,
  type CategoryItem,
  type CategoryStatus,
} from '@/api/content'

import { categoryStatusLabel } from '../constants'

defineOptions({ name: 'ContentCategories' })

const activeKind = ref<'news' | 'help'>('news')
const treeData = ref<CategoryItem[]>([])
const expandedKeys = ref<Array<string | number>>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)

type FormMode = 'create' | 'edit'
const formMode = ref<FormMode>('create')

const form = reactive<{
  parent_id?: number
  name: string
  slug: string
  icon: string
  description: string
  sort_order: number
  status: CategoryStatus
}>({
  parent_id: undefined,
  name: '',
  slug: '',
  icon: '',
  description: '',
  sort_order: 0,
  status: 'active',
})

let editingId = 0

const total = computed(() => countNodes(treeData.value))

const parentOptions = computed(() => {
  const options: Array<{ label: string; value: number }> = []
  const walk = (nodes: CategoryItem[], depth: number) => {
    for (const node of nodes) {
      // 不能把自己挂到自己下面（会造成自环，整棵子树在树构建时消失）。
      if (node.id !== editingId) {
        options.push({ label: `${'　'.repeat(depth)}${node.name}`, value: node.id })
      }
      if (node.children?.length) walk(node.children, depth + 1)
    }
  }
  walk(treeData.value, 0)
  return options
})

function countNodes(nodes: CategoryItem[]): number {
  let n = 0
  for (const node of nodes) {
    n += 1
    if (node.children?.length) n += countNodes(node.children)
  }
  return n
}

function getAllNodeIds(nodes: CategoryItem[]): Array<string | number> {
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
    const resp = await getContentCategories({ kind: activeKind.value })
    treeData.value = resp.items || []
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载分类失败')
  } finally {
    loading.value = false
  }
}

function handleKindChange() {
  expandedKeys.value = []
  loadData()
}

function openCreate(parent: CategoryItem | null) {
  formMode.value = 'create'
  editingId = 0
  form.parent_id = parent ? parent.id : undefined
  form.name = ''
  form.slug = ''
  form.icon = ''
  form.description = ''
  form.sort_order = 0
  form.status = 'active'
  dialogVisible.value = true
}

function openEdit(node: CategoryItem) {
  formMode.value = 'edit'
  editingId = node.id
  form.parent_id = node.parent_id > 0 ? node.parent_id : undefined
  form.name = node.name
  form.slug = node.slug
  form.icon = node.icon
  form.description = node.description
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
    const payload = {
      kind: activeKind.value,
      parent_id: form.parent_id ?? 0,
      name: form.name.trim(),
      slug: form.slug.trim() || undefined,
      icon: form.icon.trim(),
      description: form.description.trim(),
      sort_order: form.sort_order,
      status: form.status,
    }
    if (formMode.value === 'create') {
      await createContentCategory(payload)
      MessagePlugin.success('分类已创建')
    } else {
      await updateContentCategory(editingId, payload)
      MessagePlugin.success('分类已更新')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存分类失败')
  } finally {
    saving.value = false
  }
}

function handleDelete(node: CategoryItem) {
  const dialog = DialogPlugin.confirm({
    header: '删除分类',
    body: `确认删除「${node.name}」？若其下仍有子分类，删除将失败。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteContentCategory(node.id)
        MessagePlugin.success('分类已删除')
        dialog.hide()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '删除分类失败')
      }
    },
  })
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>
