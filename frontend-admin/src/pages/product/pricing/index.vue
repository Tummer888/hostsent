<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">价格与上下架</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadProducts">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
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
        <h3 class="card-title">价格与状态</h3>
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

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.price) }}</span>
            <span class="price-sub">{{ priceModelLabel(row.price_model) }}</span>
          </div>
        </template>

        <template #cost="{ row }">
          <span class="cell-muted">¥{{ formatPrice(row.cost_price) }}</span>
        </template>

        <template #margin="{ row }">
          <span class="margin-cell">{{ marginOf(row) }}</span>
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
                { content: '改价', value: 'price', theme: 'default' },
                { content: '上架', value: 'publish', hidden: () => !(row.status !== 1), theme: 'success' },
                { content: '下架', value: 'unpublish', hidden: () => !(row.status === 1), theme: 'warning' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
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
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无产品数据" />
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getProductCategoryList,
  getProductList,
  publishProduct,
  unpublishProduct,
  updateProductPrice,
} from '@/api/product'
import { formatPrice, priceModelLabel, productStatusOptions, statusTag } from '@/pages/product/constants'
import type { SaleProductCategoryInfo, SaleProductInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ProductPricing' })

const productList = ref<SaleProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const categoryOptions = ref<{ label: string; value: number }[]>([])
const categoryIdMap = ref<Record<number, string>>({})

const filters = reactive<{ keyword: string | undefined; category_id: number | undefined; status: number | undefined }>({
  keyword: undefined,
  category_id: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers/loadData）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
const columns: PrimaryTableCol<SaleProductInfo>[] = [
  { colKey: 'name', title: '产品', minWidth: 180 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'price', title: '销售价', width: 140 },
  { colKey: 'cost', title: '成本价', width: 100 },
  { colKey: 'margin', title: '毛利', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 160,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function marginOf(row: SaleProductInfo): string {
  const margin = row.price - row.cost_price
  const ratio = row.price > 0 ? ((margin / row.price) * 100).toFixed(1) : '0.0'
  return `¥${formatPrice(margin)} (${ratio}%)`
}

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
  pagination.current = 1
  loadProducts()
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

<style lang="css" scoped>
.margin-cell {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-primary);
  font-variant-numeric: tabular-nums;
}
</style>
