<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">推荐位管理</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadProducts">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
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
          <span class="field__label">推荐状态</span>
          <t-select v-model="filters.featured" clearable placeholder="全部状态" :options="featuredOptions" />
        </div>
        <div class="field">
          <span class="field__label">商品状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
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
        <h3 class="card-title">商品列表</h3>
        <span class="table-card__meta">推荐位商品 {{ featuredCount }} 个 / 共 {{ total }} 个</span>
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
        <template #name="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">SKU: {{ row.code }}</span>
          </div>
        </template>

        <template #featured="{ row }">
          <div class="action-cell">
            <t-switch :value="row.featured" @change="(value: boolean) => handleToggleFeatured(row, value)" />
            <span v-if="row.featured" class="featured-text">推荐中</span>
          </div>
        </template>

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-main">¥{{ formatPrice(row.price) }}</span>
            <span class="price-sub">成本 ¥{{ formatPrice(row.cost_price) }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #empty>
          <t-empty description="暂无商品数据" />
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
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getProductList, setProductFeatured } from '@/api/product'
import { formatPrice, statusTag } from '@/pages/product/constants'
import type { SaleProductInfo } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ProductPromotionRecommends' })

const featuredOptions = [
  { label: '是', value: 1 },
  { label: '否', value: 0 },
]
const statusOptions = [
  { label: '草稿', value: 0 },
  { label: '上架', value: 1 },
  { label: '下架', value: 2 },
]

const loading = ref(false)
const { isMobile } = useIsMobile()
const list = ref<SaleProductInfo[]>([])
const total = ref(0)

const filters = reactive<{ keyword: string | undefined; featured: number | undefined; status: number | undefined }>({
  keyword: undefined,
  featured: undefined,
  status: undefined,
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
const featuredCount = computed(() => list.value.filter((item) => item.featured).length)

const columns: PrimaryTableCol<SaleProductInfo>[] = [
  { colKey: 'name', title: '产品', minWidth: 180 },
  { colKey: 'featured', title: '推荐位', width: 110 },
  { colKey: 'price', title: '价格', width: 150 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'sort_order', title: '排序', width: 70 },
]

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    let items = data.items
    if (filters.featured === 1) items = items.filter((item) => item.featured)
    else if (filters.featured === 0) items = items.filter((item) => !item.featured)
    list.value = items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品失败')
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
  filters.featured = undefined
  filters.status = undefined
  pagination.current = 1
  loadProducts()
}

async function handleToggleFeatured(row: SaleProductInfo, value: boolean) {
  try {
    await setProductFeatured(row.id, value)
    MessagePlugin.success(value ? '已设为推荐' : '已取消推荐')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '操作失败')
    loadProducts()
  }
}

onMounted(() => {
  loadProducts()
})
</script>

<style lang="css">
@import '../../shared.css';

.product-module .featured-text {
  font-size: 12px;
  font-weight: 600;
  color: #5b21b6;
}
</style>
