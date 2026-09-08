<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">价格变更历史</h2>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadHistory">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">选择商品</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">商品</span>
          <t-select
            v-model="productId"
            :options="productOptions"
            placeholder="请选择商品查看历史记录"
            filterable
            :loading="productLoading"
            @change="handleProductChange"
          />
        </div>
        <div class="field">
          <span class="field__label">变更类型</span>
          <t-select v-model="changeType" clearable placeholder="全部类型" :options="changeTypeOptionsRef" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">变更记录</h3>
        <span class="table-card__meta">{{ currentProduct ? `${currentProduct} 共 ${list.length} 条` : '未选择商品' }}</span>
      </div>
      <t-table
        row-key="id"
        :data="filteredList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #change_type="{ row }">
          <t-tag variant="light" size="small" shape="round" :theme="changeTheme(row.change_type)">
            {{ changeTypeLabel(row.change_type) }}
          </t-tag>
        </template>

        <template #value="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ row.new_value || '—' }}</span>
            <span class="price-sub">原值：{{ row.old_value || '—' }}</span>
          </div>
        </template>

        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>

        <template #empty>
          <t-empty description="暂无变更记录" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'

import { getProductHistory, getProductList } from '@/api/product'
import { changeTypeLabel, changeTypeOptions, formatTime } from '@/pages/product/constants'
import type { SaleProductHistoryInfo, SaleProductInfo } from '@/types/interface'

defineOptions({ name: 'ProductPricingHistory' })

const productOptions = ref<{ label: string; value: number }[]>([])
const productId = ref<number | undefined>(undefined)
const productLoading = ref(false)
const loading = ref(false)
const list = ref<SaleProductHistoryInfo[]>([])
const changeType = ref<string | undefined>(undefined)
const currentProductName = ref('')

const changeTypeOptionsRef = changeTypeOptions

const currentProduct = computed(() => currentProductName.value)

const filteredList = computed(() => {
  if (!changeType.value) return list.value
  return list.value.filter((item) => item.change_type === changeType.value)
})

const columns: PrimaryTableCol<SaleProductHistoryInfo>[] = [
  { colKey: 'change_type', title: '变更类型', width: 100 },
  { colKey: 'value', title: '变更内容', minWidth: 220 },
  { colKey: 'operator_name', title: '操作人', width: 120 },
  { colKey: 'remark', title: '备注', minWidth: 160 },
  { colKey: 'created_at', title: '变更时间', width: 160 },
]

function changeTheme(type: string): 'success' | 'danger' | 'warning' | 'primary' | 'default' {
  switch (type) {
    case 'publish':
    case 'create':
      return 'success'
    case 'unpublish':
    case 'delete':
      return 'danger'
    case 'price':
      return 'warning'
    case 'update':
      return 'primary'
    default:
      return 'default'
  }
}

async function loadProducts() {
  productLoading.value = true
  try {
    const data = await getProductList({ page: 1, page_size: 200 })
    productOptions.value = data.items.map((item: SaleProductInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 商品加载失败不阻塞 */
  } finally {
    productLoading.value = false
  }
}

function handleProductChange() {
  currentProductName.value = productOptions.value.find((item) => item.value === productId.value)?.label || ''
  changeType.value = undefined
  loadHistory()
}

async function loadHistory() {
  if (!productId.value) {
    list.value = []
    return
  }
  loading.value = true
  try {
    const data = await getProductHistory(productId.value)
    list.value = data
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载历史记录失败')
    list.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadProducts()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
