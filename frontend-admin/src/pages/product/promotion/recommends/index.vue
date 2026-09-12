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
        <span class="table-card__meta">共 {{ pagination.total }} 个商品</span>
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
import { onMounted, reactive, ref } from 'vue'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'

import { getProductList, setProductFeatured } from '@/api/product'
import { formatPrice, productStatusOptions, statusTag } from '@/pages/product/constants'
import type { SaleProductInfo } from '@/types/interface'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

defineOptions({ name: 'ProductPromotionRecommends' })

const featuredOptions = [
  { label: '是', value: 1 },
  { label: '否', value: 0 },
]
const statusOptions = productStatusOptions

const loading = ref(false)
const { isMobile } = useIsMobile()
const list = ref<SaleProductInfo[]>([])

const filters = reactive<{ keyword: string | undefined; featured: number | undefined; status: number | undefined }>({
  keyword: undefined,
  featured: undefined,
  status: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadProducts)

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
      // 推荐位筛选交给服务端（此前是取回整页再前端过滤，导致分页总数与实际条数不一致）。
      featured: filters.featured === undefined ? undefined : filters.featured === 1,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  resetPage()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.featured = undefined
  filters.status = undefined
  resetPage()
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
