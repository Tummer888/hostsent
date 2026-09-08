<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ product?.name || '产品详情' }}</h2>
          <p class="page-header__desc">SKU: {{ product?.code || '—' }}</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadDetail">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <t-button theme="primary" @click="goEdit">编辑</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="base" label="基本信息">
          <div class="tabs-section">
            <t-descriptions v-if="product" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="产品名称">{{ product.name }}</t-descriptions-item>
              <t-descriptions-item label="SKU 编码">{{ product.code }}</t-descriptions-item>
              <t-descriptions-item label="分类">{{ categoryName(product.category_id) }}</t-descriptions-item>
              <t-descriptions-item label="产品类型">{{ product.product_type || '—' }}</t-descriptions-item>
              <t-descriptions-item label="价格模型">{{ priceModelLabel(product.price_model) }}</t-descriptions-item>
              <t-descriptions-item label="已售库存">
                <span>{{ product.stock === -1 ? '不限' : product.stock }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="销售价">¥{{ formatPrice(product.price) }}</t-descriptions-item>
              <t-descriptions-item label="成本价">¥{{ formatPrice(product.cost_price) }}</t-descriptions-item>
              <t-descriptions-item label="关联上游商品 ID">
                <span>{{ product.source_product_id || '—' }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="关联上游提供商 ID">
                <span>{{ product.source_provider_id || '—' }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="排序">{{ product.sort_order }}</t-descriptions-item>
              <t-descriptions-item label="状态">
                <t-tag :theme="statusTag(product.status).theme" variant="light" size="small" shape="round">
                  {{ statusTag(product.status).text }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(product.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(product.updated_at) }}</t-descriptions-item>
              <t-descriptions-item label="产品描述" :span="2">{{ product.description || '—' }}</t-descriptions-item>
              <t-descriptions-item label="规格 JSON" :span="2">
                <pre class="spec-pre">{{ product.specs || '—' }}</pre>
              </t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="specs" label="规格变体">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">规格变体</h3>
              <span class="table-card__meta">共 {{ specs.length }} 个规格</span>
            </div>
            <t-table
              row-key="id"
              :data="specs"
              :columns="specColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #status="{ row }">
                <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </t-tag>
              </template>
              <template #empty>
                <t-empty description="暂无规格变体" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="history" label="变更历史">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">变更历史</h3>
              <span class="table-card__meta">共 {{ history.length }} 条记录</span>
            </div>
            <t-table
              row-key="id"
              :data="history"
              :columns="historyColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #change_type="{ row }">
                <t-tag theme="primary" variant="light" size="small" shape="round">
                  {{ changeTypeLabel(row.change_type) }}
                </t-tag>
              </template>
              <template #empty>
                <t-empty description="暂无变更历史" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  getProductCategoryList,
  getProductDetail,
  getProductHistory,
  getProductSpecs,
} from '@/api/product'
import {
  changeTypeLabel,
  formatPrice,
  formatTime,
  priceModelLabel,
  statusTag,
} from '@/pages/product/constants'
import type {
  SaleProductCategoryInfo,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductSpecInfo,
} from '@/types/interface'

defineOptions({ name: 'ProductProductsDetail' })

const route = useRoute()
const router = useRouter()

const product = ref<SaleProductInfo | null>(null)
const specs = ref<SaleProductSpecInfo[]>([])
const history = ref<SaleProductHistoryInfo[]>([])
const loading = ref(false)
const activeTab = ref('base')
const categoryIdMap = ref<Record<number, string>>({})

const specColumns: PrimaryTableCol<SaleProductSpecInfo>[] = [
  { colKey: 'spec_code', title: '规格编码', minWidth: 120 },
  { colKey: 'name', title: '规格名称', minWidth: 120 },
  { colKey: 'price_model', title: '价格模型', width: 100 },
  { colKey: 'price', title: '售价', width: 100 },
  { colKey: 'cost_price', title: '成本价', width: 100 },
  { colKey: 'stock', title: '库存', width: 80 },
  { colKey: 'status', title: '状态', width: 80 },
]

const historyColumns: PrimaryTableCol<SaleProductHistoryInfo>[] = [
  { colKey: 'change_type', title: '操作类型', width: 100 },
  { colKey: 'old_value', title: '变更前', minWidth: 140 },
  { colKey: 'new_value', title: '变更后', minWidth: 140 },
  { colKey: 'operator_name', title: '操作人', width: 110 },
  { colKey: 'remark', title: '备注', minWidth: 120 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

function categoryName(id: number): string {
  return categoryIdMap.value[id] || '—'
}

async function loadCategories() {
  try {
    const data = await getProductCategoryList()
    const map: Record<number, string> = {}
    const flatten = (nodes: SaleProductCategoryInfo[]) => {
      for (const node of nodes) {
        map[node.id] = node.name
        if (node.children?.length) flatten(node.children)
      }
    }
    flatten(data.items)
    categoryIdMap.value = map
  } catch {
    /* 忽略 */
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  loading.value = true
  try {
    product.value = await getProductDetail(id)
    specs.value = await getProductSpecs(id)
    history.value = await getProductHistory(id)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品详情失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/product/products')
}

function goEdit() {
  router.push(`/product/products/${route.params.id}/edit`)
}

onMounted(() => {
  loadCategories()
  loadDetail()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}

.spec-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
  color: #334155;
}
</style>
