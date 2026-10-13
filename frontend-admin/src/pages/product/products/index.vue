<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">产品列表</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadProducts">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="handleCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建产品
        </t-button>
        <t-button variant="outline" @click="handleClone">
          <template #icon>
            <CloudIcon aria-hidden="true" />
          </template>
          导入上游商品
        </t-button>
      </t-space>
    </header>

    <div class="products-layout">
      <!-- 分类导航：点节点 = 过滤该分类及其整棵子树（后端已支持子树展开）；移动端隐藏，用筛选卡里的分类下拉 -->
      <aside v-if="!isMobile" class="products-aside surface-card">
        <div class="products-aside__head">
          <h3 class="card-title">分类导航</h3>
          <t-link theme="primary" hover="color" class="products-aside__all" @click="selectAllCategories">全部产品</t-link>
        </div>
        <t-input v-model="treeKeyword" size="small" clearable placeholder="搜索分类">
          <template #prefix-icon>
            <SearchIcon aria-hidden="true" />
          </template>
        </t-input>
        <div class="products-aside__tree">
          <t-tree
            v-model:expanded="asideExpanded"
            :data="filteredAsideTree"
            :keys="{ label: 'name', value: 'id', children: 'children' }"
            activable
            :actived="activeTreeKeys"
            line
            :expand-level="1"
            @active="onTreeActive"
          >
            <template #label="{ node }">
              <span class="aside-node" :class="{ 'aside-node--disabled': node.data.status !== 1 }">
                {{ node.data.name }}{{ node.data.status !== 1 ? '（停用）' : '' }}
              </span>
            </template>
          </t-tree>
        </div>
      </aside>

      <div class="products-main">
        <FilterCard>
          <div class="field">
            <span class="field__label">关键词</span>
            <t-input v-model="filters.keyword" placeholder="产品名称 / SKU 编码" clearable @enter="handleSearch" />
          </div>
          <div v-if="isMobile" class="field">
            <span class="field__label">分类</span>
            <t-select v-model="filters.category_id" clearable placeholder="全部分类" :options="categoryOptions" />
          </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="productStatusOptions" />
      </div>
      <div class="field">
        <span class="field__label">链路</span>
        <t-select v-model="filters.source_mode" clearable placeholder="全部链路" :options="sourceModeOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">产品列表</h3>
        <div class="table-card__meta products-crumb-wrap">
          <span v-if="activeCategoryPath.length" class="products-crumb">
            分类：{{ activeCategoryPath.map((n) => n.name).join(' / ') }}
            <t-link theme="primary" hover="color" @click="selectAllCategories">查看全部</t-link>
          </span>
          <span>共 {{ pagination.total }} 个产品</span>
        </div>
      </div>

      <!-- 批量操作条：勾选后出现，支持跨页累计选择（selectedCount 由选区状态给出） -->
      <div class="batch-bar" :class="{ 'batch-bar--active': selectedRowKeys.length > 0 }">
        <span class="batch-bar__count">
          <template v-if="selectedRowKeys.length">已选 {{ selectedRowKeys.length }} 个产品</template>
          <template v-else>勾选商品后可批量上架 / 下架 / 删除</template>
        </span>
        <t-space size="small">
          <t-button size="small" variant="outline" :disabled="!selectedRowKeys.length || batchLoading" @click="handleBatchPublish">
            批量上架
          </t-button>
          <t-button size="small" variant="outline" :disabled="!selectedRowKeys.length || batchLoading" @click="handleBatchUnpublish">
            批量下架
          </t-button>
          <t-button size="small" theme="danger" variant="outline" :disabled="!selectedRowKeys.length || batchLoading" @click="handleBatchDelete">
            批量删除
          </t-button>
          <t-link v-if="selectedRowKeys.length" theme="default" hover="color" @click="clearSelection">清空选择</t-link>
        </t-space>
      </div>
      <t-table
        row-key="id"
        :data="productList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :row-selection="rowSelection"
        :reserve-selected-row-on-paginate="true"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
        @select-change="handleSelectChange"
      >
        <template #name="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">SKU: {{ row.code }}</span>
          </div>
        </template>

        <template #category="{ row }">
          <span>{{ categoryName(row.category_id) }}</span>
        </template>

        <template #source_mode="{ row }">
          <t-tag :theme="sourceModeTag(row.source_mode).theme" variant="light" size="small" shape="round">
            {{ sourceModeTag(row.source_mode).text }}
          </t-tag>
        </template>

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.price) }}</span>
            <span class="price-sub">{{ priceModelLabel(row.price_model) }} / 成本 ¥{{ formatPrice(row.cost_price) }}</span>
          </div>
        </template>

        <template #stock="{ row }">
          <span>{{ row.stock === -1 ? '不限' : row.stock }}</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '改价', value: 'price', theme: 'default' },
                { content: '上架', value: 'publish', hidden: () => row.status === 1, theme: 'success' },
                { content: '下架', value: 'unpublish', hidden: () => row.status !== 1, theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
              <t-link
                v-if="row.status !== 1"
                theme="primary"
                hover="color"
                @click="handlePublish(row)"
              >上架</t-link>
              <t-link
                v-else
                theme="warning"
                hover="color"
                @click="handleUnpublish(row)"
              >下架</t-link>
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无产品数据，请先新建产品" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>
      </div>
    </div>

    <t-dialog
      v-model:visible="priceVisible"
      header="编辑价格"
      width="460px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSavePrice"
      @close="priceVisible = false"
    >
      <t-form label-align="top" :data="priceForm" @submit.prevent>
        <t-form-item label="销售价（元）" name="price">
          <t-input-number v-model="priceForm.price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
        </t-form-item>
        <t-form-item label="成本价（元）" name="cost_price">
          <t-input-number v-model="priceForm.cost_price" :min="0" :precision="2" theme="column" placeholder="请输入成本价" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="priceForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，记录本次调价说明" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="cloneVisible"
      header="从上游商品导入"
      width="960px"
      :confirm-btn="{ content: `导入选中的 ${selectedProductIds.size} 个商品`, theme: 'primary', disabled: !selectedProductIds.size, loading: cloneSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCloneConfirm"
      @close="cloneVisible = false"
    >
      <t-form label-align="top" :data="cloneForm" @submit.prevent>
        <div class="clone-grid">
          <t-form-item label="上游提供商" name="source_provider_id">
            <t-select v-model="cloneForm.source_provider_id" :options="providerOptions" placeholder="请选择上游提供商" @change="handleProviderChange" />
            <span class="clone-tip">只列可对接商品目录的渠道，自营平台不在列。</span>
          </t-form-item>
          <t-form-item label="归入本地分类" name="category_id">
            <t-select v-model="cloneForm.category_id" :options="categoryOptions" clearable placeholder="不选则落「未分类」" />
            <span class="clone-tip">不选也能导，导入后再逐个改分类比较费事。</span>
          </t-form-item>
          <t-form-item label="定价（按上游标价）" name="price_percent">
            <t-input-number v-model="cloneForm.price_percent" class="clone-percent" :min="1" :step="5" suffix="%" />
            <span class="clone-tip">售价 = 上游标价 × N%；100 按原价，&gt;100 加价，&lt;100 让利。</span>
          </t-form-item>
          <t-form-item label="导入后" name="publish_now">
            <t-radio-group v-model="cloneForm.publish_now" variant="default-filled">
              <t-radio-button :value="true">直接上架</t-radio-button>
              <t-radio-button :value="false">先存草稿</t-radio-button>
            </t-radio-group>
            <span class="clone-tip">
              财务型上游的开通参数由上游自带（原样转发），导入即可上架；选草稿则先编辑再上架。
            </span>
          </t-form-item>
        </div>

        <div class="clone-summary">
          <span>上游商品 {{ providerProducts.length }} 个</span>
          <span class="clone-summary__split">
            在售 {{ onSaleCount }} · 未在售 {{ providerProducts.length - onSaleCount }}
            <template v-if="importedCount"> · 已导入 {{ importedCount }}</template>
          </span>
          <span v-if="discountHint" class="clone-summary__hint">{{ discountHint }}</span>
          <span class="clone-summary__pick">
            <template v-if="selectedProductIds.size">已选 {{ selectedProductIds.size }} 个 · 预计毛利 {{ formatPrice(selectedProfit) }}/月</template>
            <template v-else>共 {{ selectableCount }} 个可导入</template>
          </span>
        </div>

        <p class="clone-hint">
          「隐藏」= 上游商铺未展示（分类被上游隐藏 / 商品隐藏 / 已下架），仍可导入开通；
          「已导入」= 该上游商品已经导入过，不再重复勾选 —— 点击可直接打开本地商品详情。
        </p>

        <div class="clone-groups">
          <div v-if="cloneLoading" class="clone-empty">正在加载上游商品…</div>
          <div v-else-if="!groupList.length" class="clone-empty">请先选择上游提供商，加载商品分组</div>
          <div v-for="group in groupList" :key="group.id" class="clone-group">
            <div class="clone-group__head">
              <t-checkbox
                :checked="isGroupChecked(group)"
                :disabled="!hasSelectable(group)"
                @change="() => toggleGroup(group)"
              >
                {{ group.name }}
                <t-tag v-if="group.derived" size="small" variant="light-outline" theme="warning">派生</t-tag>
              </t-checkbox>
              <span class="clone-group__count">
                {{ group.items.length }} 个
                <template v-if="groupImportedCount(group)"> · 已导入 {{ groupImportedCount(group) }}</template>
              </span>
            </div>
            <div class="clone-table">
              <div class="clone-table__head clone-row">
                <span />
                <span>商品</span>
                <span>规格</span>
                <span class="clone-col--num">上游标价</span>
                <span class="clone-col--num">代理价</span>
                <span class="clone-col--num">售价</span>
                <span class="clone-col--num">毛利</span>
                <span>状态</span>
              </div>
              <label
                v-for="item in group.items"
                :key="item.id"
                class="clone-row clone-row--item"
                :class="{ 'clone-row--off': !isProductOnSale(item), 'clone-row--done': item.imported }"
              >
                <t-checkbox
                  :checked="isProductChecked(item.id)"
                  :disabled="Boolean(item.imported)"
                  @change="() => toggleProduct(item.id)"
                />
                <span class="clone-cell clone-cell--name" :title="item.name">{{ item.name }}</span>
                <span class="clone-cell clone-cell--spec">{{ formatProductSpec(item) }}</span>
                <span class="clone-cell clone-col--num">{{ priceText(item.sale_price) }}</span>
                <span class="clone-cell clone-col--num">{{ priceText(item.cost_price) }}</span>
                <span class="clone-cell clone-col--num clone-cell--sale">{{ priceText(salePriceOf(item)) }}</span>
                <span class="clone-cell clone-col--num" :class="marginClass(item)">{{ marginText(item) }}</span>
                <span class="clone-cell clone-cell--status">
                  <t-tooltip
                    v-if="item.imported"
                    :content="importedTooltip(item)"
                  >
                    <t-link theme="success" hover="color" @click="openImportedProduct(item)">
                      已导入 {{ importedStatusText(item) }}
                    </t-link>
                  </t-tooltip>
                  <t-tooltip v-else :content="statusTooltipOf(item)">
                    <t-tag size="small" variant="light" :theme="statusThemeOf(item)">{{ statusTextOf(item) }}</t-tag>
                  </t-tooltip>
                </span>
              </label>
            </div>
          </div>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AddIcon, AppIcon, CloudIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchCloneProductFromUpstream,
  batchDeleteProducts,
  batchPublishProducts,
  batchUnpublishProducts,
  deleteProduct,
  getProductCategoryList,
  getProductList,
  publishProduct,
  setProductSpecPassthrough,
  unpublishProduct,
  updateProductPrice,
} from '@/api/product'
import { getProductList as getResourceProductList, getProviderList } from '@/api/admin'
import { formatPrice, priceModelLabel, productStatusOptions, sourceModeOptions, sourceModeTag, statusTag } from '@/pages/product/constants'
import type { ProductInfo, ProviderInfo, SaleProductCategoryInfo, SaleProductInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'
import { formatDisk, formatMemory } from '@/utils/spec-format'

defineOptions({ name: 'ProductProducts' })

const router = useRouter()
const route = useRoute()

const productList = ref<SaleProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const { categoryOptions, categoryName, loadCategories } = useCategoryOptions()

const filters = reactive<{ keyword: string | undefined; category_id: number | undefined; status: number | undefined; source_mode: string | undefined }>({
  keyword: undefined,
  category_id: undefined,
  status: undefined,
  source_mode: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadProducts)

const columns: PrimaryTableCol<SaleProductInfo>[] = [
  // 勾选列（批量操作入口）；row-selection 通过 :row-selection 传入
  { colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const },
  { colKey: 'name', title: '产品', minWidth: 180 },
  { colKey: 'source_mode', title: '链路', width: 110 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'price', title: '价格', width: 170 },
  { colKey: 'stock', title: '库存', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 260,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

// ===== 分类导航（左树右表）：树只负责定位，表格保持服务端分页 =====
const asideTree = ref<SaleProductCategoryInfo[]>([])
const asideExpanded = ref<Array<string | number>>([])
const treeKeyword = ref('')

// 按名称过滤导航树：仅后代命中时保留「祖先 → 命中项」链
const filteredAsideTree = computed(() => {
  const kw = treeKeyword.value.trim().toLowerCase()
  if (!kw) return asideTree.value
  const walk = (nodes: SaleProductCategoryInfo[]): SaleProductCategoryInfo[] => {
    const out: SaleProductCategoryInfo[] = []
    for (const node of nodes) {
      const children = node.children?.length ? walk(node.children) : []
      if (node.name.toLowerCase().includes(kw)) out.push(node)
      else if (children.length) out.push({ ...node, children })
    }
    return out
  }
  return walk(asideTree.value)
})


// 从根到当前选中分类的路径（面包屑用）
const activeCategoryPath = computed<SaleProductCategoryInfo[]>(() => {
  const target = filters.category_id
  if (!target) return []
  const path: SaleProductCategoryInfo[] = []
  const walk = (nodes: SaleProductCategoryInfo[]): boolean => {
    for (const node of nodes) {
      path.push(node)
      if (node.id === target) return true
      if (node.children?.length && walk(node.children)) return true
      path.pop()
    }
    return false
  }
  walk(asideTree.value)
  return path
})

const activeTreeKeys = computed(() => (filters.category_id ? [filters.category_id] : []))

async function loadCategoryTree() {
  try {
    const data = await getProductCategoryList()
    asideTree.value = data.items
    // 从 URL 直进时展开选中节点的祖先链，否则深层节点看不见
    if (filters.category_id) {
      asideExpanded.value = activeCategoryPath.value.map((n) => n.id)
    }
  } catch {
    // 导航树加载失败不阻断列表：分类下拉（移动端）与「全部产品」仍可用
  }
}

function onTreeActive(value: Array<string | number>) {
  const id = value.length ? Number(value[value.length - 1]) : undefined
  selectCategory(id && id > 0 ? id : undefined)
}

function selectCategory(id: number | undefined) {
  filters.category_id = id
  resetPage()
  router.replace({ query: { ...route.query, category_id: id ? String(id) : undefined } })
}

function selectAllCategories() {
  selectCategory(undefined)
}

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      category_id: filters.category_id,
      status: filters.status,
      source_mode: filters.source_mode,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    productList.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  resetPage()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.category_id = undefined
  filters.status = undefined
  filters.source_mode = undefined
  resetPage()
}

function handleCreate() {
  router.push('/product/products/create')
}

// ===== 批量操作（多选上架/下架/删除） =====
// 选区由 TDesign 的 row-selection 维护：selectedRowKeys 只含当前勾选的商品 ID，
// 翻页会让未勾选的页离开视图，但已勾选的键保留在集合里（跨页累计，删除前会再确认）。
const selectedRowKeys = ref<Array<string | number>>([])
const batchLoading = ref(false)

// 多选列配置：跨页保留已勾选项（reserve-selected-row-on-paginate 在表格上开启），
// 全选只作用于当前页 —— 提示文案里明确写了"已选 N 个产品"。
const rowSelection = { type: 'multiple' as const, width: 46 }

function handleSelectChange(keys: Array<string | number>) {
  selectedRowKeys.value = keys
}

function clearSelection() {
  selectedRowKeys.value = []
}

function selectedIds(): number[] {
  return selectedRowKeys.value.map((key) => Number(key)).filter((id) => id > 0)
}

/** 批量结果提示：成功数 + 前几条失败原因（逐条回报，不能只报"部分失败"）。 */
function reportBatchResult(kind: string, result: { succeeded?: number[]; failed?: Array<{ name?: string; product_id: number; reason: string }>; message?: string }) {
  const ok = result.succeeded?.length ?? 0
  const failed = result.failed ?? []
  if (ok > 0) MessagePlugin.success(`${kind}成功 ${ok} 个`)
  if (failed.length > 0) {
    const detail = failed
      .slice(0, 3)
      .map((f) => `${f.name || `#${f.product_id}`}：${f.reason}`)
      .join('；')
    MessagePlugin.warning(`${failed.length} 个未${kind}：${detail}${failed.length > 3 ? ' 等' : ''}`)
  }
  if (ok === 0 && failed.length === 0) MessagePlugin.warning(result.message || `没有可${kind}的产品`)
}

async function runBatch(kind: string, action: (ids: number[]) => Promise<any>) {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  batchLoading.value = true
  try {
    const result = await action(ids)
    reportBatchResult(kind, result || {})
    clearSelection()
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `批量${kind}失败`)
  } finally {
    batchLoading.value = false
  }
}

function handleBatchPublish() {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  // 上架走自己的一条路径（而不是通用 runBatch）：门禁拦截时要能从结果里
  // 认出"哪些是仅透传可放行的"，给一次覆盖全选的一键操作，而不是让运营
  // 逐个商品回到单条上架去点确认弹窗。
  batchLoading.value = true
  void (async () => {
    try {
      const result = await batchPublishProducts(ids)
      reportBatchResult('上架', result || {})
      const blocked = (result?.failed ?? []).filter(
        (f) => f.reason.includes('仅透传') || f.reason.includes('平台绑定'),
      )
      if (blocked.length > 0) {
        offerBatchPassthroughAndPublish(blocked.map((f) => f.product_id))
      }
      clearSelection()
      loadProducts()
    } catch (error) {
      MessagePlugin.error((error as Error).message || '批量上架失败')
    } finally {
      batchLoading.value = false
    }
  })()
}

/**
 * 批量上架被门禁拦下时的一键处理：对这批商品统一开启「仅透传上游参数」再上架。
 *
 * 适用于上游给的整机固定配置（参数原样转发）。财务型上游的商品在服务端已按
 * config_groups 自动放行；这里覆盖的是"上游不给配置组、但确实按原样转发"的少数情形。
 */
function offerBatchPassthroughAndPublish(ids: number[]) {
  if (!ids.length) return
  const dialog = DialogPlugin.confirm({
    header: `有 ${ids.length} 个商品需要确认参数来源`,
    body: `这批上游转售商品未完成"本地规格 → 上游开通参数"的平台绑定确认。
若它们都是整机固定配置（参数原样转发、不改写），可一次性开启「仅透传上游参数」放行；
否则请到商品详情 → 规格页签逐个完成绑定确认。`,
    confirmBtn: { content: '全部开启仅透传并上架', theme: 'primary' },
    cancelBtn: { content: '稍后处理' },
    onConfirm: async () => {
      dialog.hide()
      try {
        await Promise.all(ids.map((id) => setProductSpecPassthrough(id, true)))
        const retry = await batchPublishProducts(ids)
        reportBatchResult('上架', retry || {})
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '开启仅透传后仍上架失败')
      }
    },
  })
}

function handleBatchUnpublish() {
  void runBatch('下架', (ids) => batchUnpublishProducts(ids))
}

function handleBatchDelete() {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  const names = productList.value
    .filter((item) => ids.includes(item.id))
    .slice(0, 3)
    .map((item) => item.name)
    .join('、')
  const dialog = DialogPlugin.confirm({
    header: `删除 ${ids.length} 个产品？`,
    body: `删除后不可恢复（含规格与价格矩阵）。${names ? `包含：${names}${ids.length > 3 ? ' 等' : ''}` : ''}`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      dialog.hide()
      await runBatch('删除', (list) => batchDeleteProducts(list))
    },
  })
}

// ===== 上游商品导入（克隆：按上游分组多选 + 本地分类 + 按上游标价百分比定价） =====
// 入口参数：?import=<渠道ID>&preselect=<上游资源商品ID>（上游商品页「去导入」跳转过来时用）
const cloneVisible = ref(false)
const cloneSubmitting = ref(false)
const cloneLoading = ref(false)
const providerOptions = ref<{ label: string; value: number }[]>([])
// 分组结构：group_id → 组名（派生分类带标记）+ 商品列表
const groupList = ref<{ id: number; name: string; derived: boolean; items: ProductInfo[] }[]>([])
const providerProducts = ref<ProductInfo[]>([])
const selectedProductIds = ref<Set<number>>(new Set())
const cloneForm = reactive<{
  source_provider_id: number | undefined
  category_id: number | undefined
  price_percent: number
  /** 导入后是否直接上架（默认是）。财务型上游开通参数自带，无需逐个确认映射。 */
  publish_now: boolean
}>({
  source_provider_id: undefined,
  category_id: undefined,
  price_percent: 100,
  publish_now: true,
})

/** 优惠提示：上游给本账号的代理折扣说明（取自商品镜像的折扣描述）。 */
const discountHint = computed(() => {
  const hit = providerProducts.value.find((item) => item.cost_price > 0 && item.sale_price > item.cost_price)
  if (!hit) return ''
  const off = Math.round((hit.cost_price / hit.sale_price) * 100)
  return `该渠道代理价约为标价的 ${off}%（成本按代理价计）`
})

/** 在售商品数：上游隐藏分组/隐藏商品/已下架都不算在售（status 由同步按真实可见性写入）。 */
const onSaleCount = computed(() => providerProducts.value.filter((item) => isProductOnSale(item)).length)

/** 已导入数：上游商品已导入为本地商品的数量（这些不允许重复导入）。 */
const importedCount = computed(() => providerProducts.value.filter((item) => item.imported).length)

/** 可导入数（未导入的）——汇总行给运营一个"还能导几个"的准数。 */
const selectableCount = computed(() => providerProducts.value.filter((item) => !item.imported).length)

/** 已选商品的预计毛利（售价 − 代理价，按标价可见的商品相加）。 */
const selectedProfit = computed(() => {
  let total = 0
  for (const item of providerProducts.value) {
    if (!selectedProductIds.value.has(item.id)) continue
    const sale = salePriceOf(item)
    if (sale > 0 && item.cost_price > 0) total += sale - item.cost_price
  }
  return Math.round(total * 100) / 100
})

/** 本次定价下的售价：上游标价 × 百分比（未定价商品返回 0）。 */
function salePriceOf(item: ProductInfo): number {
  if (!(item.sale_price > 0)) return 0
  const pct = cloneForm.price_percent > 0 ? cloneForm.price_percent : 100
  return Math.round(item.sale_price * pct) / 100
}

/** 价目单元格：未定价商品显示「未定价」而不是 ¥0.00（活动款价格常只挂在配置项里）。 */
function priceText(v: number): string {
  return v > 0 ? `¥${formatPrice(v)}` : '未定价'
}

/** 毛利单元格：售价与代理价的差额与毛利率；缺价时给「—」。 */
function marginText(item: ProductInfo): string {
  const sale = salePriceOf(item)
  if (sale <= 0 || !(item.cost_price > 0)) return '—'
  const profit = sale - item.cost_price
  const pct = Math.round((profit / sale) * 100)
  return `${formatPrice(profit)} / ${pct}%`
}

function marginClass(item: ProductInfo): string {
  const sale = salePriceOf(item)
  if (sale <= 0 || !(item.cost_price > 0)) return 'clone-cell--muted'
  return sale >= item.cost_price ? 'clone-cell--up' : 'clone-cell--down'
}

/** 是否在售：以同步写入的真实可见性为准（旧数据无 visibility 时退回 status）。 */
function isProductOnSale(item: ProductInfo): boolean {
  if (item.visibility) return item.visibility === 'visible'
  return item.status === 1
}

/** 上游状态：只显示两态（正常 / 隐藏），具体原因放悬停说明。 */
function statusTextOf(item: ProductInfo): string {
  return isProductOnSale(item) ? '正常' : '隐藏'
}

function statusTooltipOf(item: ProductInfo): string {
  switch (item.visibility) {
    case 'group_hidden':
      return '隐藏：上游把整个分类从商铺隐藏（仍可导入开通）'
    case 'product_hidden':
      return '隐藏：上游单独隐藏了这个商品'
    case 'retired':
      return '隐藏：上游已下架该商品（不建议导入）'
    default:
      return '正常：上游商铺在展示'
  }
}

function statusThemeOf(item: ProductInfo): 'success' | 'danger' | 'warning' | 'default' {
  if (isProductOnSale(item)) return 'success'
  return item.visibility === 'retired' ? 'danger' : 'warning'
}

async function handleClone() {
  cloneForm.source_provider_id = undefined
  cloneForm.category_id = undefined
  cloneForm.price_percent = 100
  cloneForm.publish_now = true
  selectedProductIds.value = new Set()
  groupList.value = []
  cloneVisible.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    // 只列出可做商品对接的渠道：自营算力平台（魔方云等）没有商品目录，
    // 之前混在列表里选中后什么也拉不到。
    providerOptions.value = data.items
      .filter((item: ProviderInfo) => item.catalog_supported !== false)
      .map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    MessagePlugin.error('加载上游提供商失败')
  }
}

async function handleProviderChange() {
  groupList.value = []
  providerProducts.value = []
  selectedProductIds.value = new Set()
  if (!cloneForm.source_provider_id) return
  cloneLoading.value = true
  try {
    // 一次拉全量（列表接口上限 500，且不再携带 raw_specs 大字段）：
    // 此前固定 page_size=100 导致上游商品多于 100 时后一半根本选不到。
    const data = await getResourceProductList({ provider_id: cloneForm.source_provider_id, page_size: 500 })
    providerProducts.value = data.items
    groupList.value = buildCloneGroups(data.items)
  } catch {
    MessagePlugin.error('加载上游商品失败')
  } finally {
    cloneLoading.value = false
  }
}

function buildCloneGroups(items: ProductInfo[]): { id: number; name: string; derived: boolean; items: ProductInfo[] }[] {
  const map = new Map<number, { id: number; name: string; derived: boolean; items: ProductInfo[] }>()
  for (const item of items) {
    const gid = item.group_id || 0
    if (!map.has(gid)) {
      map.set(gid, {
        id: gid,
        // 隐藏分组的上游不公示分类名，同步按商品名前缀派生了近似标签（带「派生」标记）；
        // 两者都没有时给「分组 #id」，而不是含糊的「未分类」。
        name: item.group_name || `分组 #${gid}`,
        derived: Boolean(item.group_name_derived),
        items: [],
      })
    }
    map.get(gid)!.items.push(item)
  }
  // 在售组优先、组名稳定排序，避免每次加载顺序乱跳。
  return Array.from(map.values()).sort((a, b) => {
    const aOn = a.items.some((item) => isProductOnSale(item)) ? 0 : 1
    const bOn = b.items.some((item) => isProductOnSale(item)) ? 0 : 1
    if (aOn !== bOn) return aOn - bOn
    return a.name.localeCompare(b.name, 'zh-Hans-CN')
  })
}

function isProductChecked(id: number): boolean {
  return selectedProductIds.value.has(id)
}

function toggleProduct(id: number) {
  const item = providerProducts.value.find((p) => p.id === id)
  // 已导入的上游商品不允许再选：重复导入只会拿到"商品编码重复"的失败。
  if (item?.imported) return
  const next = new Set(selectedProductIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedProductIds.value = next
}

/** 组内未导入的商品（勾选操作只在这些上生效）。 */
function selectableItems(group: { items: ProductInfo[] }): ProductInfo[] {
  return group.items.filter((item) => !item.imported)
}

function groupImportedCount(group: { items: ProductInfo[] }): number {
  return group.items.length - selectableItems(group).length
}

function hasSelectable(group: { items: ProductInfo[] }): boolean {
  return selectableItems(group).length > 0
}

function isGroupChecked(group: { items: ProductInfo[] }): boolean {
  const selectable = selectableItems(group)
  return selectable.length > 0 && selectable.every((item) => selectedProductIds.value.has(item.id))
}

function toggleGroup(group: { items: ProductInfo[] }) {
  const selectable = selectableItems(group)
  if (selectable.length === 0) return
  const next = new Set(selectedProductIds.value)
  const allChecked = isGroupChecked(group)
  if (allChecked) {
    selectable.forEach((item) => next.delete(item.id))
  } else {
    selectable.forEach((item) => next.add(item.id))
  }
  selectedProductIds.value = next
}

/** 已导入商品在本地商品表的去向与状态（弹窗里直接说明"导到哪儿了"）。 */
function importedStatusText(item: ProductInfo): string {
  const suffix = item.imported_product_id ? ` #${item.imported_product_id}` : ''
  switch (item.imported_status) {
    case 1:
      return `已上架${suffix}`
    case 2:
      return `已下架${suffix}`
    default:
      return `草稿${suffix}`
  }
}

/**
 * 「草稿」是什么、在哪儿改：导入默认落草稿（前台不展示、不售卖）。
 * 悬停直接说清入口，避免"显示草稿但不知道去哪看"。
 */
function importedTooltip(item: ProductInfo): string {
  const target = item.imported_product_id ? `本地商品 #${item.imported_product_id}` : '本地商品'
  const how = '点击打开该商品详情：可改分类/描述/价格，改完在详情页或列表页「上架」'
  switch (item.imported_status) {
    case 1:
      return `已导入为${target}（已上架）。${how}`
    case 2:
      return `已导入为${target}（已下架，前台不可购买）。${how}`
    default:
      return `已导入为${target}（草稿：未上架，前台不展示也不售卖）。${how}`
  }
}

/** 跳到已导入的本地商品详情（草稿的编辑/上架入口就在那里）。 */
function openImportedProduct(item: ProductInfo) {
  if (!item.imported_product_id) {
    MessagePlugin.warning('未取到本地商品 ID，请到「产品管理 → 产品列表」按名称查找')
    return
  }
  router.push(`/product/products/${item.imported_product_id}`)
}

function formatProductSpec(item: ProductInfo): string {
  // 规格单位统一走 utils/spec-format（此前直接拼 `${memory}M` 会显示「8192M」）。
  const parts: string[] = []
  if (item.cpu) parts.push(`${item.cpu}核`)
  const mem = formatMemory(item.memory || 0)
  if (mem !== '—') parts.push(mem)
  const disk = formatDisk(item.disk || 0)
  if (disk !== '—') parts.push(disk)
  if (item.os) parts.push(item.os)
  return parts.length ? parts.join(' / ') : '规格未同步'
}

async function handleCloneConfirm() {
  if (!cloneForm.source_provider_id || selectedProductIds.value.size === 0) {
    MessagePlugin.warning('请选择上游提供商并勾选至少一个上游商品')
    return
  }
  // 上游未定价的商品（价格只挂在配置项、上游还没配活动价）仍可导入，
  // 但要在导入前说清楚：导入后本地售价为 0，需人工补价。
  const unpriced = providerProducts.value.filter(
    (item) => selectedProductIds.value.has(item.id) && !(item.sale_price > 0) && !(item.cost_price > 0),
  )
  // 提交前剔除已导入的（弹窗可能开着很久，期间这些可能已被别处导入）：
  // 带上只会得到"商品编码重复"的失败，直接在客户端挡掉。
  const selectedItems = providerProducts.value.filter((item) => selectedProductIds.value.has(item.id))
  const importable = selectedItems.filter((item) => !item.imported)
  if (importable.length === 0) {
    MessagePlugin.warning('选中的商品都已导入过，无需重复导入')
    return
  }
  cloneSubmitting.value = true
  try {
    const result = await batchCloneProductFromUpstream({
      source_provider_id: cloneForm.source_provider_id,
      source_product_ids: importable.map((item) => item.id),
      category_id: cloneForm.category_id,
      price_percent: cloneForm.price_percent,
      // 1=导入即上架（默认），0=草稿（先编辑再上架）
      status: cloneForm.publish_now ? 1 : 0,
    })
    const imported = result?.items?.length ?? 0
    const failed = result?.failed ?? []
    if (imported > 0) {
      MessagePlugin.success(`已导入 ${imported} 个上游商品`)
    }
    if (unpriced.length > 0) {
      MessagePlugin.warning(
        `${unpriced.length} 个商品上游未定价（如 ${unpriced[0].name}），导入后售价为 0，请到商品详情页补价`,
      )
    }
    if (failed.length > 0) {
      // 逐条失败此前被静默吞掉（接口一律返回成功），这里把原因摊开：
      // 最常见的是「该上游商品已导入过」（商品编码 cpy-<渠道>-<上游商品> 已存在）。
      const detail = failed
        .slice(0, 3)
        .map((f) => `${f.name || `#${f.source_product_id}`}：${f.reason}`)
        .join('；')
      MessagePlugin.warning(
        imported > 0
          ? `${failed.length} 个未导入：${detail}${failed.length > 3 ? ' 等' : ''}`
          : `导入失败：${detail}${failed.length > 3 ? ' 等' : ''}`,
      )
    }
    if (imported === 0 && failed.length === 0) {
      MessagePlugin.warning('没有可导入的商品，请检查选择')
    }
    if (imported > 0) {
      cloneVisible.value = false
      loadProducts()
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '导入失败')
  } finally {
    cloneSubmitting.value = false
  }
}

function openDetail(row: SaleProductInfo) {
  router.push(`/product/products/${row.id}`)
}

function openEdit(row: SaleProductInfo) {
  router.push(`/product/products/${row.id}/edit`)
}

async function handlePublish(row: SaleProductInfo) {
  try {
    await publishProduct(row.id)
    MessagePlugin.success('已上架')
    loadProducts()
  } catch (error) {
    const message = (error as Error).message || '上架失败'
    // 上架门禁（上游转售商品）：把"为什么被拦"和"怎么放行"讲清楚，并给一键操作，
    // 避免运营看到一句报错就卡住（此前只能去猜「仅透传」在哪里开）。
    if (message.includes('仅透传') || message.includes('平台绑定')) {
      offerPassthroughAndPublish(row)
      return
    }
    MessagePlugin.error(message)
  }
}

/**
 * 上架被门禁拦下时的引导：说明原理（本地规格→上游开通参数没确认过），
 * 若该上游商品是整机固定配置，可直接开启「仅透传」再上架。
 */
function offerPassthroughAndPublish(row: SaleProductInfo) {
  const dialog = DialogPlugin.confirm({
    header: '上架前需要确认参数来源',
    body: `「${row.name}」是上游转售商品：上架后客户下单会把参数发给上游开通，所以系统先要求确认"本地规格 → 上游开通参数"的对应关系。
若该上游商品是整机固定配置（参数原样转发、不改写），可开启「仅透传上游参数」直接放行；否则请到商品详情 → 规格页签完成平台绑定确认。`,
    confirmBtn: { content: '开启仅透传并上架', theme: 'primary' },
    cancelBtn: { content: '稍后处理' },
    onConfirm: async () => {
      dialog.hide()
      try {
        await setProductSpecPassthrough(row.id, true)
        await publishProduct(row.id)
        MessagePlugin.success('已开启仅透传并上架')
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '开启仅透传后仍上架失败')
      }
    },
  })
}

async function handleUnpublish(row: SaleProductInfo) {
  try {
    await unpublishProduct(row.id)
    MessagePlugin.success('已下架')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下架失败')
  }
}

async function handleDelete(row: SaleProductInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除产品',
    body: `确认删除「${row.name}」？删除后不可恢复。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteProduct(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

const priceVisible = ref(false)
const priceForm = reactive<{ price: number; cost_price: number; remark: string }>({
  price: 0,
  cost_price: 0,
  remark: '',
})
let priceProductId = 0

function openPriceDialog(row: SaleProductInfo) {
  priceProductId = row.id
  priceForm.price = row.price
  priceForm.cost_price = row.cost_price
  priceForm.remark = ''
  priceVisible.value = true
}

async function handleSavePrice() {
  try {
    await updateProductPrice(priceProductId, {
      price: priceForm.price,
      cost_price: priceForm.cost_price,
      remark: priceForm.remark || undefined,
    })
    MessagePlugin.success('价格已更新')
    priceVisible.value = false
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新价格失败')
  }
}

onMounted(() => {
  const q = Number(route.query.category_id)
  if (q > 0) filters.category_id = q
  loadCategories()
  loadCategoryTree()
  loadProducts()
  void openCloneFromQuery()
})

/**
 * 从上游商品页「去导入」跳转时自动开弹窗并预选：
 *   ?import=<渠道ID>（&preselect=<上游商品ID>）
 * 处理完把参数从地址栏清掉，避免刷新或返回时反复弹窗。
 */
async function openCloneFromQuery() {
  const providerId = Number(route.query.import)
  if (!providerId) return
  const preselectId = Number(route.query.preselect)
  const query = { ...route.query }
  delete query.import
  delete query.preselect
  router.replace({ query })
  await handleClone()
  cloneForm.source_provider_id = providerId
  await handleProviderChange()
  if (preselectId > 0) {
    const target = providerProducts.value.find((item) => item.id === preselectId)
    if (target?.imported) {
      MessagePlugin.warning(`${target.name} 已导入过，无需重复导入`)
    } else if (target) {
      selectedProductIds.value = new Set([preselectId])
    }
  }
}

// 浏览器前进/后退时同步分类过滤（selectCategory 的 replace 会触发本 watch，但值相同直接跳过）
watch(
  () => route.query.category_id,
  (value) => {
    const id = Number(value)
    const next = value && id > 0 ? id : undefined
    if (next !== filters.category_id) {
      filters.category_id = next
      resetPage()
    }
  },
)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: SaleProductInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'price':
      openPriceDialog(row)
      break
    case 'publish':
      void handlePublish(row)
      break
    case 'unpublish':
      void handleUnpublish(row)
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
/* 表头三列：等宽 + 等高（提示两行高度固定），避免加载数据/切换选项时格子跳动 */
.clone-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  align-items: start;
}
.clone-grid :deep(.t-form__item) {
  min-width: 0;
  margin-bottom: 8px;
}
.clone-grid :deep(.t-form__label) {
  min-height: 22px;
  line-height: 22px;
}
/* 百分比输入框默认宽度很窄，拉满一列才不会与下拉框参差 */
.clone-percent {
  width: 100%;
}
.clone-tip {
  display: block;
  min-height: 36px;
  margin-top: 4px;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-secondary, #999);
}
.clone-hint {
  margin: 0 0 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary, #999);
}
/* 商品面板固定高度：加载/切换服务商时弹窗不再忽高忽低 */
.clone-groups {
  height: 420px;
  overflow: auto;
  border: 1px solid var(--td-component-border, #ddd);
  border-radius: 6px;
  padding: 8px;
}
.clone-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--td-text-color-secondary, #999);
  font-size: 13px;
}
.clone-group {
  margin-bottom: 12px;
}
.clone-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  border-radius: 4px;
  font-weight: 600;
}
.clone-group__count {
  color: var(--td-text-color-secondary, #999);
  font-weight: 400;
  font-size: 12px;
}
/* 商品表：固定列宽对齐，价格各占一列，避免长名挤在一起看不清 */
.clone-table {
  padding: 4px 8px 0 8px;
}
.clone-row {
  display: grid;
  grid-template-columns: 28px minmax(200px, 1.6fr) 150px 84px 84px 84px 96px 76px;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
}
.clone-table__head.clone-row {
  color: var(--td-text-color-placeholder, #bbb);
  border-bottom: 1px solid var(--td-component-border, #eee);
  padding-bottom: 6px;
}
.clone-row--item {
  border-bottom: 1px dashed var(--td-component-border, #f0f0f0);
  cursor: pointer;
}
.clone-row--item:hover {
  background: var(--td-bg-color-container-hover, #fafafa);
}
.clone-row--off .clone-cell--name,
.clone-row--off .clone-cell--spec {
  color: var(--td-text-color-placeholder, #bbb);
}
/* 已导入：整行压暗 + 勾选框禁用，一眼区分"还能导的"和"已经导过的" */
.clone-row--done {
  opacity: 0.72;
  cursor: not-allowed;
}
.clone-row--done:hover {
  background: transparent;
}
.clone-cell--status {
  overflow: visible;
}
.clone-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.clone-cell--spec {
  color: var(--td-text-color-secondary, #999);
}
.clone-cell--sale {
  color: var(--td-brand-color, #0052d9);
  font-weight: 600;
}
.clone-col--num {
  text-align: right;
}
.clone-cell--up {
  color: var(--td-success-color, #2ba471);
}
.clone-cell--down {
  color: var(--td-error-color, #d54941);
}
.clone-cell--muted {
  color: var(--td-text-color-placeholder, #bbb);
}
.clone-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  min-height: 24px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  white-space: nowrap;
  overflow: hidden;
}
.clone-summary__split {
  color: var(--td-text-color-primary, #333);
}
.clone-summary__hint {
  padding: 1px 8px;
  border-radius: 10px;
  background: var(--td-success-color-1, #e8f8f2);
  color: var(--td-success-color, #2ba471);
}
.clone-summary__pick {
  margin-left: auto;
  color: var(--td-brand-color, #0052d9);
}
/* 批量操作条：常态是灰色提示，勾选后高亮，避免误以为"没选也能点" */
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  margin-bottom: 8px;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.batch-bar--active {
  background: rgba(0, 82, 217, 0.06);
  color: var(--td-text-color-primary, #333);
}

.batch-bar__count {
  font-weight: 500;
}

.products-layout {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.products-aside {
  flex: 0 0 248px;
  position: sticky;
  top: 76px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 14px;
  max-height: calc(100vh - 100px);
}

.products-aside__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.products-aside__all {
  font-size: 13px;
}

.products-aside__tree {
  overflow: auto;
  min-height: 120px;
}

.aside-node {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aside-node--disabled {
  color: var(--color-muted-foreground);
}

.products-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-lg, 16px);
}

.products-crumb-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.products-crumb {
  font-weight: 600;
  color: var(--color-foreground);
}

@media (max-width: 900px) {
  .products-layout {
    display: block;
  }

  .products-aside {
    display: none;
  }
}
</style>
