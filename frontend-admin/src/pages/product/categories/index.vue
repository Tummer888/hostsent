<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">分类管理</h2>
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
        <div class="tree-toolbar">
          <span class="table-card__meta">
            {{ keyword ? `匹配 ${matchCount} / 共 ${total} 个分类` : `共 ${total} 个分类` }}
            <template v-if="levelStats.length && !keyword">
              <span v-for="stat in levelStats" :key="stat.level" class="category-lv-stat">
                <i class="category-lv-dot" :class="`category-lv-dot--${stat.level}`" />{{ stat.label }} {{ stat.count }}
              </span>
            </template>
          </span>
          <t-input
            v-model="keyword"
            class="tree-toolbar__search"
            clearable
            placeholder="搜索分类名称"
            @clear="onFilterCleared"
          >
            <template #prefix-icon>
              <SearchIcon aria-hidden="true" />
            </template>
          </t-input>
        </div>
      </div>

      <t-enhanced-table
        ref="tableRef"
        row-key="id"
        :data="filteredTree"
        :columns="columns"
        :tree="{ childrenKey: 'children', defaultExpandAll: true, indent: 28 }"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        :min-width="720"
        cell-empty-content="—"
        :pagination="null"
        :empty="keyword ? '没有匹配的分类，换个关键词试试' : '暂无分类，点击右上角「新建分类」创建'"
        :row-class-name="rowLevelClass"
      >
        <template #name="{ row }">
          <div class="category-name">
            <span class="category-lv" :class="`category-lv--${levelOf(row)}`">{{ levelLabel(levelOf(row)) }}</span>
            <span class="category-name__text" :class="`category-name__text--l${levelOf(row)}`">{{ row.name }}</span>
            <t-tag v-if="row.children?.length" size="small" variant="outline" theme="default" class="category-name__count">
              {{ row.children.length }} 个子分类
            </t-tag>
          </div>
        </template>
        <template #cost_rate="{ row }">
          <span v-if="row.cost_rate" class="category-rate">{{ rateLabel(row.cost_rate) }}</span>
          <span v-else class="cell-muted">未配置</span>
        </template>
        <template #status="{ row }">
          <t-tag
            :theme="row.status === 1 ? 'success' : row.status === 0 ? 'default' : 'danger'"
            variant="light"
            size="small"
          >{{ statusTag(row.status).text }}</t-tag>
        </template>
        <template #op="{ row }">
          <t-space size="small" class="category-actions">
            <t-link theme="primary" hover="color" @click="openCreate(row)">添加子分类</t-link>
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </t-space>
        </template>
      </t-enhanced-table>
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
        <t-form-item label="拿货折扣率" name="cost_rate">
          <t-input-number
            v-model="form.cost_rate"
            :min="0"
            :max="1"
            :step="0.05"
            :decimal-places="4"
            placeholder="如 0.6 = 六折进货；0 = 未配置"
          />
          <p class="form-hint">用于代理折扣的毛利校验：等级折扣率不得低于此值，否则判为亏本。0 = 不校验。</p>
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
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

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
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)

// 树表实例：全部展开/收起走 EnhancedTable 的实例方法
const tableRef = ref<{ expandAll: () => void; foldAll: () => void } | null>(null)

const keyword = ref('')

type FormMode = 'create' | 'edit'
const formMode = ref<FormMode>('create')

const form = reactive<{
  parent_id: number | undefined
  name: string
  sort_order: number
  status: number
  cost_rate: number
}>({
  parent_id: undefined,
  name: '',
  sort_order: 0,
  status: 1,
  cost_rate: 0,
})

let editingId = 0

const total = computed(() => countNodes(treeData.value))

// ===== 层级视觉：颜色按「深度」区分（一级主题色，二级蓝，三级紫，四级青，更深用灰） =====
// chip 底/字色与左侧色点共用一套色板；一级走品牌色变量，随主题切换联动。
const LEVEL_COUNT = 4

const depthMap = computed(() => {
  const map = new Map<number, number>()
  const walk = (nodes: SaleProductCategoryInfo[], depth: number) => {
    for (const node of nodes) {
      map.set(node.id, depth)
      if (node.children?.length) walk(node.children, depth + 1)
    }
  }
  walk(treeData.value, 0)
  return map
})

function levelOf(row: SaleProductCategoryInfo): number {
  return Math.min((depthMap.value.get(row.id) ?? 0) + 1, 5)
}

function levelLabel(level: number): string {
  return level <= LEVEL_COUNT ? `${['一', '二', '三', '四'][level - 1]}级` : `${level}级`
}

const rowLevelClass = ({ row }: { row: SaleProductCategoryInfo }): string =>
  levelOf(row) === 1 ? 'cat-row--l1' : ''

const levelStats = computed(() => {
  const counts = new Map<number, number>()
  for (const depth of depthMap.value.values()) {
    counts.set(depth + 1, (counts.get(depth + 1) ?? 0) + 1)
  }
  return [...counts.entries()]
    .sort(([a], [b]) => a - b)
    .map(([level, count]) => ({ level, count, label: levelLabel(level) }))
})

// 按名称过滤：命中节点保留其完整子树；仅后代命中时保留「祖先 → 命中后代」的链
const filteredTree = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return treeData.value
  const walk = (nodes: SaleProductCategoryInfo[]): SaleProductCategoryInfo[] => {
    const out: SaleProductCategoryInfo[] = []
    for (const node of nodes) {
      const children = node.children?.length ? walk(node.children) : []
      if (node.name.toLowerCase().includes(kw)) {
        out.push(node)
      } else if (children.length) {
        out.push({ ...node, children })
      }
    }
    return out
  }
  return walk(treeData.value)
})

const matchCount = computed(() => countNodes(filteredTree.value))

// 关键词变化时自动全展开：过滤结果必须是完整可见的，否则“匹配 3 个”却只看得见 1 个
watch(keyword, () => {
  void nextTick(() => tableRef.value?.expandAll())
})

function onFilterCleared() {
  keyword.value = ''
}

const columns: PrimaryTableCol<SaleProductCategoryInfo>[] = [
  {
    colKey: 'name',
    title: '分类',
    minWidth: 280,
    cell: 'name',
  },
  { colKey: 'sort_order', title: '排序', width: 90, align: 'center' },
  { colKey: 'cost_rate', title: '拿货折扣率', width: 120, cell: 'cost_rate' },
  { colKey: 'status', title: '状态', width: 90, cell: 'status' },
  { colKey: 'op', title: '操作', width: 210, cell: 'op' },
]

function rateLabel(rate: number): string {
  const zhe = rate * 10
  return `${Number.isInteger(zhe) ? zhe : zhe.toFixed(1)} 折`
}

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

function expandAll() {
  tableRef.value?.expandAll()
}

function collapseAll() {
  tableRef.value?.foldAll()
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
  form.parent_id = parent ? parent.id : undefined
  form.name = ''
  form.sort_order = 0
  form.status = 1
  form.cost_rate = 0
  dialogVisible.value = true
}

function openEdit(node: SaleProductCategoryInfo) {
  formMode.value = 'edit'
  editingId = node.id
  form.parent_id = node.parent_id
  form.name = node.name
  form.sort_order = node.sort_order
  form.status = node.status
  form.cost_rate = node.cost_rate || 0
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
        cost_rate: form.cost_rate || 0,
      })
      MessagePlugin.success('分类已创建')
    } else {
      await updateProductCategory(editingId, {
        parent_id: form.parent_id,
        name: form.name.trim(),
        sort_order: form.sort_order,
        status: form.status,
        cost_rate: form.cost_rate || 0,
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

function handleDelete(node: SaleProductCategoryInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除分类',
    body: `确认删除「${node.name}」？若其下仍有子分类或已绑定商品，删除将失败。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteProductCategory(node.id)
        MessagePlugin.success('分类已删除')
        dialog.hide()
        loadData()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除分类失败')
      }
    },
  })
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style lang="css" scoped>
.tree-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  flex-wrap: wrap;
}

.tree-toolbar__search {
  width: 220px;
}

.category-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

/* 层级徽标：一级绿（品牌色）、二级蓝、三级紫、四级青、更深灰 */
.category-lv {
  flex-shrink: 0;
  min-width: 34px;
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  text-align: center;
  line-height: 16px;
}

.category-lv--1 {
  background: var(--td-brand-color-1);
  color: var(--td-brand-color-8);
}

.category-lv--2 {
  background: #eff6ff;
  color: #1d4ed8;
}

.category-lv--3 {
  background: #f5f3ff;
  color: #6d28d9;
}

.category-lv--4 {
  background: #ecfeff;
  color: #0e7490;
}

.category-lv--5 {
  background: var(--hs-surface-3);
  color: var(--color-muted-foreground);
}

/* 名称字重/颜色随层级递减，形成第二重视觉层级 */
.category-name__text--l1 {
  font-weight: 700;
  color: var(--td-brand-color-8);
}

.category-name__text--l2 {
  font-weight: 600;
  color: var(--color-foreground);
}

.category-name__text--l3,
.category-name__text--l4,
.category-name__text--l5 {
  font-weight: 500;
  color: var(--color-foreground);
}

/* 一级行整行铺极浅品牌色底，子分类保持白底 —— 组与组的分界一眼可见 */
:deep(.cat-row--l1 > td) {
  background: rgba(var(--color-primary-rgb), 0.035);
}

:deep(.cat-row--l1:hover > td) {
  background: var(--td-brand-color-1);
}

/* 卡片头的分级统计 */
.category-lv-stat {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 10px;
}

.category-lv-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.category-lv-dot--1 {
  background: var(--td-brand-color-6);
}

.category-lv-dot--2 {
  background: #3b82f6;
}

.category-lv-dot--3 {
  background: #8b5cf6;
}

.category-lv-dot--4 {
  background: #06b6d4;
}

.category-lv-dot--5 {
  background: var(--color-muted-foreground);
}

.category-name__text {
  font-weight: 600;
  color: var(--color-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.category-name__count {
  flex-shrink: 0;
  color: var(--color-muted-foreground);
}

.category-rate {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--color-foreground);
}

.cell-muted {
  color: var(--color-muted-foreground);
}

.category-actions {
  white-space: nowrap;
}
</style>
