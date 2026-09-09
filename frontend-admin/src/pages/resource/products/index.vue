<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">商品列表</h2>
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
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">商品列表</h3>
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
          <div class="product-cell">
            <span class="product-name">{{ row.name }}</span>
            <span class="product-sub">{{ row.region || row.zone ? `${row.region}${row.zone ? ' / ' + row.zone : ''}` : '' }}</span>
          </div>
        </template>

        <template #group="{ row }">
          <t-tag v-if="row.group_name" theme="primary" variant="light" size="small" shape="round">{{ row.group_name }}</t-tag>
          <span v-else class="muted">未分类</span>
        </template>

        <template #specs="{ row }">
          <span class="spec-text">{{ row.cpu }}核 / {{ formatMemory(row.memory) }} / {{ formatDisk(row.disk) }} {{ row.bandwidth }}Mbps</span>
        </template>

        <template #os="{ row }">
          <span>{{ row.os || '—' }}</span>
        </template>

        <template #price="{ row }">
          <div class="price-cell">
            <span class="price-sale">¥{{ formatPrice(row.sale_price) }}</span>
            <span class="price-cost">成本 ¥{{ formatPrice(row.cost_price) }}</span>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 1 ? '上架中' : '已下架' }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
            <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无商品数据，请先执行商品同步" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="detailVisible"
      :header="detailProduct ? `商品详情 · ${detailProduct.name}` : '商品详情'"
      width="600px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailProduct" :column="2" bordered size="small">
        <t-descriptions-item label="商品名称">{{ detailProduct.name }}</t-descriptions-item>
        <t-descriptions-item label="上游 ID">{{ detailProduct.upstream_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="规格">{{ detailProduct.cpu }}核 / {{ formatMemory(detailProduct.memory) }} / {{ formatDisk(detailProduct.disk) }} {{ detailProduct.bandwidth }}Mbps</t-descriptions-item>
        <t-descriptions-item label="磁盘类型">{{ detailProduct.disk_type || '—' }}</t-descriptions-item>
        <t-descriptions-item label="操作系统">{{ detailProduct.os || '—' }}</t-descriptions-item>
        <t-descriptions-item label="区域/可用区">{{ detailProduct.region }}{{ detailProduct.zone ? ' / ' + detailProduct.zone : '' }}</t-descriptions-item>
        <t-descriptions-item label="原始规格">{{ detailProduct.raw_specs || '—' }}</t-descriptions-item>
        <t-descriptions-item label="规格明细">{{ detailProduct.specs || '—' }}</t-descriptions-item>
        <t-descriptions-item label="销售价">¥{{ formatPrice(detailProduct.sale_price) }}</t-descriptions-item>
        <t-descriptions-item label="成本价">¥{{ formatPrice(detailProduct.cost_price) }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailProduct.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ detailProduct.status === 1 ? '上架中' : '已下架' }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="更新时间">{{ formatTime(detailProduct.updated_at) }}</t-descriptions-item>
      </t-descriptions>
    </t-dialog>

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

import { AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductList, getProviderList, updateProductPrice } from '@/api/admin'
import type { ProductInfo, ProviderInfo } from '@/types/interface'

defineOptions({ name: 'ResourceProducts' })

const productList = ref<ProductInfo[]>([])
const loading = ref(false)
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const statusOptions = [
  { label: '上架中', value: 1 },
  { label: '已下架', value: 0 },
]

const filters = reactive<{ keyword: string | undefined; provider_id: number | undefined; status: number | undefined }>({
  keyword: undefined,
  provider_id: undefined,
  status: undefined,
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

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const columns: PrimaryTableCol<ProductInfo>[] = [
  { colKey: 'name', title: '商品', minWidth: 180 },
  { colKey: 'group', title: '分类', width: 130 },
  { colKey: 'specs', title: '规格', minWidth: 200 },
  { colKey: 'os', title: '系统', width: 100 },
  { colKey: 'price', title: '价格', width: 150 },
  { colKey: 'status', title: '状态', width: 100 },
  {
    colKey: 'action',
    title: '操作',
    width: 130,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      provider_id: filters.provider_id,
      status: filters.status,
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
    /* 提供商下拉加载失败不阻塞列表 */
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
  filters.status = undefined
  pagination.current = 1
  loadProducts()
}

const detailVisible = ref(false)
const detailProduct = ref<ProductInfo | null>(null)

function openDetailDialog(row: ProductInfo) {
  detailProduct.value = row
  detailVisible.value = true
}

const priceVisible = ref(false)
const priceForm = reactive<{ cost_price: number; sale_price: number }>({ cost_price: 0, sale_price: 0 })

function openPriceDialog(row: ProductInfo) {
  priceForm.cost_price = row.cost_price
  priceForm.sale_price = row.sale_price
  priceVisible.value = true
}

async function handleSavePrice() {
  if (!detailProduct.value) return
  try {
    await updateProductPrice(detailProduct.value.id, {
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
  --chip-bg: linear-gradient(135deg, #7c3aed, #6d28d9);
  --chip-shadow: 0 4px 10px rgba(124, 58, 237, 0.25);
}

.filter-card__grid {
  grid-template-columns: repeat(3, minmax(200px, 1fr));
}

.product-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.product-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.product-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.price-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.price-sale {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.price-cost {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
