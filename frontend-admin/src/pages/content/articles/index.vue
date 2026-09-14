<template>
  <div class="page-body content-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <FileIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">内容文章</h2>
          <p class="page-header__desc">新闻 / 帮助 / 条款 / 隐私政策</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建内容
        </t-button>
      </t-space>
    </header>

    <!-- 类型分栏：新闻/帮助/条款/隐私是四种内容形态，混在一张表里无法筛选 -->
    <section class="table-card surface-card">
      <t-tabs v-model="activeKind" :space-evenly="false" @change="handleKindChange">
        <t-tab-panel v-for="tab in kindTabs" :key="tab.value" :value="tab.value" :label="tab.label" />
      </t-tabs>

      <div class="filter-card__grid" style="margin-top: 14px">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="标题关键词" clearable @enter="handleSearch" />
        </div>
        <div class="field" v-if="needsCategory">
          <span class="field__label">分类</span>
          <t-select v-model="filters.category_id" clearable placeholder="全部分类" :options="categoryOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
      <div class="filter-card__actions" style="margin-top: 14px">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">{{ kindLabel(activeKind) }}列表</h3>
        <span class="table-card__meta">
          共 {{ total }} 条 · 门户落点 {{ kindPortalPath(activeKind) || '—' }}
        </span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #title="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.title }}</span>
            <span class="product-sub">/{{ row.slug }}</span>
          </div>
        </template>
        <template #category="{ row }">
          <span class="cell-muted">{{ row.category_name || '未分类' }}</span>
        </template>
        <template #pinned="{ row }">
          <t-tag v-if="row.pinned" theme="primary" variant="light" size="small" shape="round">置顶</t-tag>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #version="{ row }">
          <span class="cell-muted">{{ row.version || '—' }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="articleStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ articleStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #publish_at="{ row }">
          <span class="time-text">{{ formatTime(row.publish_at) }}</span>
        </template>
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '发布', value: 'publish', hidden: () => row.status === 'published', theme: 'success' },
                { content: '下线', value: 'offline', hidden: () => row.status !== 'published', theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link v-if="row.status !== 'published'" theme="primary" hover="color" @click="handlePublish(row)">
                发布
              </t-link>
              <t-link v-if="row.status === 'published'" theme="warning" hover="color" @click="handleOffline(row)">
                下线
              </t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty :description="`暂无${kindLabel(activeKind)}内容`" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="page.current"
        :page-size="page.size"
        :total="total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <ArticleFormDrawer
      v-model:visible="formVisible"
      :article-id="editingId"
      :default-kind="activeKind"
      @saved="handleSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, FileIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  deleteArticle,
  getArticles,
  getContentCategories,
  offlineArticle,
  publishArticle,
  type ArticleItem,
  type ArticleKind,
  type CategoryItem,
} from '@/api/content'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

import ArticleFormDrawer from './edit.vue'
import {
  ARTICLE_STATUS_OPTIONS,
  articleStatusLabel,
  articleStatusTheme,
  formatTime,
  kindIsSingleton,
  kindLabel,
  kindNeedsCategory,
  kindPortalPath,
  KIND_OPTIONS,
} from '../constants'

defineOptions({ name: 'ContentArticles' })

const { isMobile } = useIsMobile()
const loading = ref(false)
const list = ref<ArticleItem[]>([])
const total = ref(0)
const activeKind = ref<ArticleKind>('news')
const filters = reactive<{ keyword: string; category_id?: number; status: string }>({
  keyword: '',
  category_id: undefined,
  status: '',
})
const page = reactive({ current: 1, size: 10 })
const categories = ref<CategoryItem[]>([])

const kindTabs = KIND_OPTIONS
const statusOptions = ARTICLE_STATUS_OPTIONS

const needsCategory = computed(() => kindNeedsCategory(activeKind.value))

const categoryOptions = computed(() => flattenCategories(categories.value))

const columns = computed<PrimaryTableCol[]>(() => [
  { colKey: 'title', title: '标题', minWidth: 220 },
  ...(needsCategory.value ? [{ colKey: 'category', title: '分类', width: 120 }] : []),
  { colKey: 'pinned', title: '置顶', width: 80, align: 'center' },
  ...(kindIsSingleton(activeKind.value) ? [{ colKey: 'version', title: '版本', width: 90 }] : []),
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'publish_at', title: '发布时间', width: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 200, fixed: 'right', align: 'center' },
])

const pagination = computed(() => ({
  current: page.current,
  pageSize: page.size,
  total: total.value,
  showJumper: true,
}))

function flattenCategories(items: CategoryItem[]): Array<{ label: string; value: number }> {
  const out: Array<{ label: string; value: number }> = []
  const walk = (nodes: CategoryItem[], depth: number) => {
    for (const node of nodes) {
      out.push({ label: `${'　'.repeat(depth)}${node.name}`, value: node.id })
      if (node.children?.length) walk(node.children, depth + 1)
    }
  }
  walk(items, 0)
  return out
}

async function loadCategories() {
  if (!needsCategory.value) {
    categories.value = []
    return
  }
  try {
    const resp = await getContentCategories({ kind: activeKind.value })
    categories.value = resp.items || []
  } catch {
    // 分类读取失败不该阻塞文章列表：这里静默降级成「无分类筛选」，
    // 否则一个分类接口故障会让整个内容页打不开。
    categories.value = []
  }
}

async function loadData() {
  loading.value = true
  try {
    const resp = await getArticles({
      kind: activeKind.value,
      category_id: filters.category_id,
      status: filters.status || undefined,
      keyword: filters.keyword || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.list || []
    total.value = resp.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载内容列表失败')
  } finally {
    loading.value = false
  }
}

function handleKindChange() {
  page.current = 1
  filters.category_id = undefined
  loadCategories()
  loadData()
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  filters.category_id = undefined
  page.current = 1
  loadData()
}

function handlePageChange(info: PageInfo) {
  page.current = info.current
  page.size = info.pageSize
  loadData()
}

function goMobilePage(target: number) {
  const totalPages = Math.max(1, Math.ceil(total.value / page.size))
  const clamped = Math.min(Math.max(target, 1), totalPages)
  if (clamped === page.current) return
  void applyMobilePage(clamped, page.size)
}

async function applyMobilePage(current: number, pageSize: number) {
  page.current = current
  page.size = pageSize
  await loadData()
}

function handleMobilePageSizeChange(pageSize: number) {
  void applyMobilePage(1, pageSize)
}

// —— 新增 / 编辑 ——
const formVisible = ref(false)
const editingId = ref<number | null>(null)

function openCreate() {
  editingId.value = null
  formVisible.value = true
}

function openEdit(row: ArticleItem) {
  editingId.value = row.id
  formVisible.value = true
}

function handleSaved() {
  formVisible.value = false
  loadData()
}

// —— 发布 / 下线 / 删除 ——
async function handlePublish(row: ArticleItem) {
  try {
    await publishArticle(row.id)
    MessagePlugin.success('内容已发布')
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '发布失败')
  }
}

function handleOffline(row: ArticleItem) {
  const dialog = DialogPlugin.confirm({
    header: '下线内容',
    body: `确认下线「${row.title}」吗？下线后门户将不再展示。`,
    confirmBtn: { content: '确认下线', theme: 'warning' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await offlineArticle(row.id)
        MessagePlugin.success('内容已下线')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '下线失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function handleDelete(row: ArticleItem) {
  const dialog = DialogPlugin.confirm({
    header: '删除内容',
    body: `确认删除「${row.title}」吗？此操作不可恢复，门户上该内容会立即消失。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteArticle(row.id)
        MessagePlugin.success('内容已删除')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function handleMobileAction(value: string | number | Record<string, unknown>, row: ArticleItem) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'publish':
      void handlePublish(row)
      break
    case 'offline':
      void handleOffline(row)
      break
    case 'delete':
      void handleDelete(row)
      break
  }
}

onMounted(async () => {
  await loadCategories()
  await loadData()
})
</script>

<style lang="css">
@import '../shared.css';
</style>
