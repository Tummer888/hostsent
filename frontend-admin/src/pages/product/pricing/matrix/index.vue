<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">周期价格</h2>
          <p class="page-header__desc">
            商品 × 规格 × 计费周期的价格矩阵。价格直接存折后价，用户组/代理折扣由「折扣策略」在下单时二次叠加。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" :disabled="!productId" @click="reload">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" :loading="saving" :disabled="!productId || !items.length" @click="handleSave">
          保存矩阵
        </t-button>
      </t-space>
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
            filterable
            clearable
            placeholder="选择要维护周期价格的商品"
            :loading="productLoading"
            @change="handleProductChange"
          />
        </div>
        <div class="field">
          <span class="field__label">规格（SKU）</span>
          <t-select
            v-model="specId"
            :options="specOptions"
            clearable
            placeholder="商品级（不区分规格）"
            :disabled="!productId || !specOptions.length"
            @change="reload"
          />
        </div>
        <div class="field">
          <span class="field__label">币种</span>
          <t-input v-model="currency" disabled />
        </div>
      </div>
    </section>

    <section v-if="productId" class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">
          {{ matrix?.product_name || '周期价格矩阵' }}
          <t-tag v-if="isUpstream" theme="warning" variant="light" size="small" shape="round" class="head-tag">
            上游转售
          </t-tag>
        </h3>
        <span class="table-card__meta">{{ cycleHint }}</span>
      </div>

      <t-alert
        v-if="isUpstream"
        theme="info"
        class="matrix-alert"
        message="上游商品的价格由上游成本 + 加价规则推导，此处只能启停档位；要改价格请调整上游加价规则或等同步重算。"
      />

      <t-table
        row-key="cycle"
        :data="items"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #cycle="{ row }">
          <div class="matrix-cell">
            <span class="cell-strong">{{ row.cycle_name }}</span>
            <span class="cell-muted">{{ row.cycle }}</span>
          </div>
        </template>

        <template #cost_setup_fee="{ row }">
          <t-input-number
            v-model="row.cost_setup_fee"
            :min="0"
            :precision="2"
            :disabled="!row.editable"
            theme="normal"
            size="small"
          />
        </template>

        <template #setup_fee="{ row }">
          <t-input-number
            v-model="row.setup_fee"
            :min="0"
            :precision="2"
            :disabled="!row.editable"
            theme="normal"
            size="small"
          />
        </template>

        <template #cost_price="{ row }">
          <t-input-number
            v-model="row.cost_price"
            :min="0"
            :precision="2"
            :disabled="!row.editable"
            theme="normal"
            size="small"
          />
        </template>

        <template #price="{ row }">
          <t-input-number
            v-model="row.price"
            :min="0"
            :precision="2"
            :disabled="!row.editable"
            theme="normal"
            size="small"
          />
        </template>

        <template #source="{ row }">
          <t-tag :theme="sourceTheme(row.source)" variant="light" size="small" shape="round">
            {{ sourceLabel(row.source) }}
          </t-tag>
        </template>

        <template #enabled="{ row }">
          <t-switch v-model="row.status" :custom-value="[1, 0]" @change="() => onToggle(row)" />
        </template>
      </t-table>

      <p class="matrix-footnote">
        停用的档位不会出现在用户端/开放平台的可选周期里，下单时会直接拒绝该周期。
      </p>
    </section>

    <section v-else class="table-card surface-card">
      <t-empty description="请先选择商品，再维护其周期价格矩阵" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { MoneyIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductPriceMatrix, getProductList, getProductSpecs, saveProductPriceMatrix } from '@/api/product'
import type { ProductPriceItem, ProductPriceMatrix } from '@/types/interface'

defineOptions({ name: 'ProductPricingMatrix' })

const productOptions = ref<{ label: string; value: number }[]>([])
const specOptions = ref<{ label: string; value: number }[]>([])
const productLoading = ref(false)
const loading = ref(false)
const saving = ref(false)

const productId = ref<number | undefined>(undefined)
const specId = ref<number | undefined>(undefined)
const currency = ref('CNY')

const matrix = ref<ProductPriceMatrix | null>(null)
const items = ref<ProductPriceItem[]>([])

const isUpstream = computed(() => matrix.value?.source_mode === 'upstream')

const cycleHint = computed(() => {
  if (!matrix.value) return ''
  if (isUpstream.value) {
    return `上游可提供 ${matrix.value.upstream_cycles.length} 档周期`
  }
  return `平台全部 ${matrix.value.upstream_cycles.length} 档周期可维护`
})

const columns: PrimaryTableCol<ProductPriceItem>[] = [
  { colKey: 'cycle', title: '周期', width: 150 },
  { colKey: 'cost_setup_fee', title: '初装费成本', width: 140 },
  { colKey: 'setup_fee', title: '初装费售价', width: 140 },
  { colKey: 'cost_price', title: '价格成本', width: 140 },
  { colKey: 'price', title: '售价（折后）', width: 150 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'enabled', title: '启用', width: 90, align: 'center' as const },
]

function sourceLabel(source: string): string {
  return { upstream: '上游同步', markup: '加价推导', manual: '人工填写' }[source] || '人工填写'
}
function sourceTheme(source: string): 'success' | 'warning' | 'primary' | 'default' {
  switch (source) {
    case 'upstream':
      return 'primary'
    case 'markup':
      return 'warning'
    case 'manual':
      return 'success'
    default:
      return 'default'
  }
}

async function loadProducts() {
  productLoading.value = true
  try {
    const data = await getProductList({ page: 1, page_size: 200 })
    productOptions.value = (data.items || []).map((item) => ({ label: item.name, value: item.id }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载商品列表失败')
  } finally {
    productLoading.value = false
  }
}

async function loadSpecs(id: number) {
  specOptions.value = []
  specId.value = undefined
  try {
    const specs = await getProductSpecs(id)
    specOptions.value = (specs || [])
      .filter((spec) => spec.status === 1)
      .map((spec) => ({ label: spec.name || spec.spec_code, value: spec.id }))
  } catch {
    /* 未拆 SKU 的商品没有规格，属正常情况 */
  }
}

async function handleProductChange(value: unknown) {
  const id = typeof value === 'number' ? value : undefined
  matrix.value = null
  items.value = []
  if (!id) return
  await loadSpecs(id)
  await reload()
}

async function reload() {
  if (!productId.value) return
  loading.value = true
  try {
    const data = await getProductPriceMatrix({
      product_id: productId.value,
      spec_id: specId.value,
      currency: currency.value,
    })
    matrix.value = data
    items.value = data.items || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载周期价格矩阵失败')
  } finally {
    loading.value = false
  }
}

// t-switch 绑定 1/0 数值时组件内部会把布尔语义转换一次，这里统一回写为 1/0 保证提交口径稳定。
function onToggle(row: ProductPriceItem) {
  row.status = row.status ? 1 : 0
}

async function handleSave() {
  if (!productId.value) return
  saving.value = true
  try {
    const data = await saveProductPriceMatrix({
      product_id: productId.value,
      spec_id: specId.value,
      currency: currency.value,
      items: items.value.map((item) => ({
        cycle: item.cycle,
        price: Number(item.price) || 0,
        cost_price: Number(item.cost_price) || 0,
        setup_fee: Number(item.setup_fee) || 0,
        cost_setup_fee: Number(item.cost_setup_fee) || 0,
        status: item.status ? 1 : 0,
      })),
    })
    matrix.value = data
    items.value = data.items || []
    MessagePlugin.success('周期价格已保存')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadProducts)
</script>

<style scoped>
@import '../../shared.css';

.head-tag {
  margin-left: 8px;
}

.matrix-alert {
  margin: 0 16px 12px;
}

.matrix-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.matrix-footnote {
  margin: 12px 16px 4px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>
