<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">定价管理</h2>
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
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="商品名称 / 规格" clearable />
        </div>
        <div class="field">
          <span class="field__label">所属提供商</span>
          <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">商品定价</h3>
        <span class="table-card__meta">共 {{ total }} 个商品</span>
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
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <span class="cell-strong">{{ row.name }}</span>
        </template>
        <template #specs="{ row }">
          <span class="cell-muted">{{ row.cpu }}核 / {{ formatMemory(row.memory) }} / {{ formatDisk(row.disk) }}</span>
        </template>
        <template #sale_price="{ row }">
          <span class="price-sale">¥{{ formatPrice(row.sale_price) }}</span>
        </template>
        <template #cost_price="{ row }">
          <span class="cell-muted">¥{{ formatPrice(row.cost_price) }}</span>
        </template>
        <template #margin="{ row }">
          <span class="cell-muted">{{ marginPercent(row) }}</span>
        </template>
        <template #action="{ row }">
          <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
        </template>
        <template #empty>
          <t-empty description="暂无商品数据，请先执行商品同步" />
        </template>
      </t-table>
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
        <t-form-item label="销售价（元）" name="sale_price">
          <t-input-number v-model="priceForm.sale_price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
        </t-form-item>
        <t-form-item label="成本价（元）" name="cost_price">
          <t-input-number v-model="priceForm.cost_price" :min="0" :precision="2" theme="column" placeholder="请输入成本价" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductList, getProviderList, updateProductPrice } from '@/api/admin'
import type { ProductInfo, ProviderInfo } from '@/types/interface'

defineOptions({ name: 'ResourcePricing' })

const productList = ref<ProductInfo[]>([])
const loading = ref(false)
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])

const filters = reactive<{ keyword: string | undefined; provider_id: number | undefined }>({
  keyword: undefined,
  provider_id: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

function formatMemory(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatDisk(value: number): string {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}T`
  return `${value}G`
}

function formatPrice(value: number): string {
  return Number(value || 0).toFixed(2)
}

function marginPercent(row: ProductInfo): string {
  if (!row.sale_price) return '—'
  return `${(((row.sale_price - row.cost_price) / row.sale_price) * 100).toFixed(1)}%`
}

const columns: PrimaryTableCol<ProductInfo>[] = [
  { colKey: 'name', title: '商品', minWidth: 170 },
  { colKey: 'specs', title: '规格', minWidth: 160 },
  { colKey: 'sale_price', title: '销售价', width: 110 },
  { colKey: 'cost_price', title: '成本价', width: 110 },
  { colKey: 'margin', title: '毛利率', width: 90 },
  { colKey: 'action', title: '操作', width: 90, fixed: 'right' as const, align: 'center' as const },
]

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      provider_id: filters.provider_id,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    productList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品列表失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 提供商下拉加载失败不阻塞 */
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadProducts()
}

function handleSearch() {
  pagination.current = 1
  loadProducts()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.provider_id = undefined
  pagination.current = 1
  loadProducts()
}

const priceVisible = ref(false)
const editingProduct = ref<ProductInfo | null>(null)
const priceForm = reactive<{ cost_price: number; sale_price: number }>({ cost_price: 0, sale_price: 0 })

function openPriceDialog(row: ProductInfo) {
  editingProduct.value = row
  priceForm.cost_price = row.cost_price
  priceForm.sale_price = row.sale_price
  priceVisible.value = true
}

async function handleSavePrice() {
  if (!editingProduct.value) return
  try {
    await updateProductPrice(editingProduct.value.id, {
      cost_price: priceForm.cost_price,
      sale_price: priceForm.sale_price,
    })
    MessagePlugin.success('价格已更新')
    priceVisible.value = false
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新价格失败')
  }
}

onMounted(() => {
  loadProviders()
  loadProducts()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #d97706, #b45309);
  --chip-shadow: 0 4px 10px rgba(217, 119, 6, 0.25);
}

.filter-card__grid {
  grid-template-columns: repeat(2, minmax(200px, 1fr));
}

.price-sale {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}
</style>
