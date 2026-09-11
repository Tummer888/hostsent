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

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="产品名称 / SKU 编码" clearable @enter="handleSearch" />
        </div>
        <div class="field">
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
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">产品列表</h3>
        <span class="table-card__meta">共 {{ total }} 个产品</span>
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
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
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
          <t-tag :theme="sourceModeTag(row.source_mode || (row.provision_mode === 'clone' ? 'upstream' : 'self')).theme" variant="light" size="small" shape="round">
            {{ sourceModeTag(row.source_mode || (row.provision_mode === 'clone' ? 'upstream' : 'self')).text }}
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
                { content: '上架', value: 'publish', hidden: () => !(row.status !== 1), theme: 'success' },
                { content: '下架', value: 'unpublish', hidden: () => !(row.status === 1), theme: 'warning' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
              <t-link
                v-if="row.status !== 1"
                theme="success"
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
      width="720px"
      :confirm-btn="{ content: '导入选中的 N 个商品', theme: 'primary', loading: cloneSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCloneConfirm"
      @close="cloneVisible = false"
    >
      <t-form label-align="top" :data="cloneForm" @submit.prevent>
        <div class="clone-grid">
          <t-form-item label="上游提供商" name="source_provider_id">
            <t-select v-model="cloneForm.source_provider_id" :options="providerOptions" placeholder="请选择上游提供商" @change="handleProviderChange" />
          </t-form-item>
          <t-form-item label="定价百分比 (%)" name="price_percent">
            <t-input-number v-model="cloneForm.price_percent" :min="100" :step="10" theme="column" placeholder="售价 = 上游售价 × 百分比" />
            <span class="clone-tip">销售价 = 上游售价 × N%</span>
          </t-form-item>
        </div>

        <div class="clone-groups">
          <div v-if="!groupList.length" class="clone-empty">请先选择上游提供商，加载商品分组</div>
          <div v-for="group in groupList" :key="group.id" class="clone-group">
            <div class="clone-group__head">
              <t-checkbox :checked="isGroupChecked(group)" @change="() => toggleGroup(group)">
                {{ group.name || '未分类' }}
              </t-checkbox>
              <span class="clone-group__count">{{ group.items.length }} 个</span>
            </div>
            <div class="clone-group__items">
              <t-checkbox
                v-for="item in group.items"
                :key="item.id"
                :checked="isProductChecked(item.id)"
                @change="() => toggleProduct(item.id)"
              >
                <span class="clone-item">
                  <span class="clone-item__name">{{ item.name }}</span>
                  <span class="clone-item__spec">{{ formatProductSpec(item) }}</span>
                  <span class="clone-item__price">¥{{ formatPrice(item.sale_price) }}</span>
                </span>
              </t-checkbox>
            </div>
          </div>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { AddIcon, AppIcon, CloudIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchCloneProductFromUpstream,
  deleteProduct,
  getProductCategoryList,
  getProductList,
  publishProduct,
  unpublishProduct,
  updateProductPrice,
} from '@/api/product'
import { getProductList as getResourceProductList, getProviderList } from '@/api/admin'
import { formatPrice, priceModelLabel, productStatusOptions, sourceModeOptions, sourceModeTag, statusTag } from '@/pages/product/constants'
import type { ProductInfo, ProviderInfo, SaleProductCategoryInfo, SaleProductInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ProductProducts' })

const router = useRouter()

const productList = ref<SaleProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const categoryOptions = ref<{ label: string; value: number }[]>([])
const categoryIdMap = ref<Record<number, string>>({})

const filters = reactive<{ keyword: string | undefined; category_id: number | undefined; status: number | undefined; source_mode: string | undefined }>({
  keyword: undefined,
  category_id: undefined,
  status: undefined,
  source_mode: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<SaleProductInfo>[] = [
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

function categoryName(id: number): string {
  return categoryIdMap.value[id] || '—'
}

async function loadCategories() {
  try {
    const data = await getProductCategoryList()
    const options: { label: string; value: number }[] = []
    const map: Record<number, string> = {}
    const flatten = (nodes: SaleProductCategoryInfo[]) => {
      for (const node of nodes) {
        options.push({ label: node.name, value: node.id })
        map[node.id] = node.name
        if (node.children?.length) flatten(node.children)
      }
    }
    flatten(data.items)
    categoryOptions.value = options
    categoryIdMap.value = map
  } catch {
    /* 分类加载失败不阻塞列表 */
  }
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
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadProducts()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}


function handleSearch() {
  pagination.current = 1
  loadProducts()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.category_id = undefined
  filters.status = undefined
  filters.source_mode = undefined
  pagination.current = 1
  loadProducts()
}

function handleCreate() {
  router.push('/product/products/create')
}

// ===== 上游商品导入（克隆，按分组多选 + 百分设定价） =====
const cloneVisible = ref(false)
const cloneSubmitting = ref(false)
const providerOptions = ref<{ label: string; value: number }[]>([])
// 分组结构：group_id → 组名 + 商品列表
const groupList = ref<{ id: number; name: string; items: ProductInfo[] }[]>([])
const providerProducts = ref<ProductInfo[]>([])
const selectedProductIds = ref<Set<number>>(new Set())
const cloneForm = reactive<{ source_provider_id: number | undefined; price_percent: number }>({
  source_provider_id: undefined,
  price_percent: 120,
})

async function handleClone() {
  cloneForm.source_provider_id = undefined
  cloneForm.price_percent = 120
  selectedProductIds.value = new Set()
  groupList.value = []
  cloneVisible.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    MessagePlugin.error('加载上游提供商失败')
  }
}

async function handleProviderChange() {
  groupList.value = []
  providerProducts.value = []
  selectedProductIds.value = new Set()
  if (!cloneForm.source_provider_id) return
  try {
    const data = await getResourceProductList({ provider_id: cloneForm.source_provider_id, page_size: 100 })
    providerProducts.value = data.items
    groupList.value = buildCloneGroups(data.items)
  } catch {
    MessagePlugin.error('加载上游商品失败')
  }
}

function buildCloneGroups(items: ProductInfo[]): { id: number; name: string; items: ProductInfo[] }[] {
  const map = new Map<number, { id: number; name: string; items: ProductInfo[] }>()
  for (const item of items) {
    const gid = item.group_id || 0
    if (!map.has(gid)) {
      map.set(gid, { id: gid, name: item.group_name || '未分类', items: [] })
    }
    map.get(gid)!.items.push(item)
  }
  return Array.from(map.values())
}

function isProductChecked(id: number): boolean {
  return selectedProductIds.value.has(id)
}

function toggleProduct(id: number) {
  const next = new Set(selectedProductIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedProductIds.value = next
}

function isGroupChecked(group: { id: number; items: ProductInfo[] }): boolean {
  return group.items.length > 0 && group.items.every((item) => selectedProductIds.value.has(item.id))
}

function toggleGroup(group: { id: number; items: ProductInfo[] }) {
  const next = new Set(selectedProductIds.value)
  const allChecked = isGroupChecked(group)
  if (allChecked) {
    group.items.forEach((item) => next.delete(item.id))
  } else {
    group.items.forEach((item) => next.add(item.id))
  }
  selectedProductIds.value = next
}

function formatProductSpec(item: ProductInfo): string {
  const parts: string[] = []
  if (item.cpu) parts.push(`${item.cpu}核`)
  if (item.memory) parts.push(`${item.memory}M`)
  if (item.disk) parts.push(`${item.disk}G`)
  if (item.os) parts.push(item.os)
  return parts.length ? parts.join(' / ') : '规格未同步'
}

async function handleCloneConfirm() {
  if (!cloneForm.source_provider_id || selectedProductIds.value.size === 0) {
    MessagePlugin.warning('请选择上游提供商并勾选至少一个上游商品')
    return
  }
  cloneSubmitting.value = true
  try {
    await batchCloneProductFromUpstream({
      source_provider_id: cloneForm.source_provider_id,
      source_product_ids: Array.from(selectedProductIds.value),
      price_percent: cloneForm.price_percent,
    })
    MessagePlugin.success(`已导入 ${selectedProductIds.value.size} 个上游商品`)
    cloneVisible.value = false
    loadProducts()
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
    MessagePlugin.error((error as Error).message || '上架失败')
  }
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
  try {
    await deleteProduct(row.id)
    MessagePlugin.success('已删除')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除失败')
  }
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
  loadCategories()
  loadProducts()
})

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
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.clone-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.clone-tip {
  display: block;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}
.clone-groups {
  max-height: 380px;
  overflow: auto;
  border: 1px solid var(--td-component-border, #ddd);
  border-radius: 6px;
  padding: 8px;
}
.clone-empty {
  color: var(--td-text-color-secondary, #999);
  padding: 24px;
  text-align: center;
  font-size: 13px;
}
.clone-group {
  margin-bottom: 8px;
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
.clone-group__items {
  padding-left: 16px;
  display: flex;
  flex-direction: column;
}
.clone-item {
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
}
.clone-item__spec {
  color: var(--td-text-color-secondary, #999);
  font-size: 12px;
}
.clone-item__price {
  color: var(--td-warning-color, #e37318);
  font-size: 12px;
}
</style>
