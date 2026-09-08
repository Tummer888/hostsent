<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">价格计算器</h2>
        </div>
      </div>
    </header>

    <section class="form-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">计算参数</h3>
      </div>
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="商品" name="product_id">
          <t-select
            v-model="form.product_id"
            :options="productOptions"
            placeholder="请选择商品"
            filterable
            :loading="productLoading"
            @change="handleProductChange"
          />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="计费模式" name="billing_mode">
            <t-select v-model="form.billing_mode" :options="billingModeOptions" placeholder="请选择计费模式" @change="applyUnitPrice" />
          </t-form-item>
          <t-form-item label="单价（元）" name="unit_price">
            <t-input-number v-model="form.unit_price" :min="0" :precision="2" theme="column" placeholder="请输入单价" />
          </t-form-item>
          <t-form-item label="数量" name="quantity">
            <t-input-number v-model="form.quantity" :min="1" :max="9999" theme="column" placeholder="请输入数量" />
          </t-form-item>
          <t-form-item :label="durationLabel" name="duration">
            <t-input-number v-model="form.duration" :min="1" :max="1200" theme="column" placeholder="请输入时长" />
          </t-form-item>
        </div>
        <div class="field" style="margin-top: var(--space-lg)">
          <span class="field__label">价格明细</span>
        </div>
      </t-form>

      <t-table
        row-key="key"
        :data="priceDetailRows"
        :columns="detailColumns"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #empty>
          <t-empty description="请选择商品并填写参数，此处将展示价格明细" />
        </template>
      </t-table>

      <div class="result-bar">
        <div class="result-bar__meta">
          {{ form.product_id ? productName(form.product_id) : '未选择商品' }}
          <span class="result-bar__hint">{{ summaryHint }}</span>
        </div>
        <div class="result-bar__total">预估总价 <strong>¥{{ formatPrice(totalAmount) }}</strong></div>
      </div>

      <div class="form-footer">
        <t-button variant="outline" @click="resetForm">重置</t-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { AppIcon } from 'tdesign-icons-vue-next'

import { getProductList } from '@/api/product'
import { formatPrice } from '@/pages/product/constants'
import type { SaleProductInfo } from '@/types/interface'

defineOptions({ name: 'ProductPricingCalculator' })

const billingModeOptions = [
  { label: '固定价', value: 'fixed' },
  { label: '按小时', value: 'hourly' },
  { label: '按月', value: 'monthly' },
]

const productOptions = ref<{ label: string; value: number }[]>([])
const productMap = ref<Record<number, SaleProductInfo>>({})
const productLoading = ref(false)

const form = reactive<{
  product_id: number | undefined
  billing_mode: string | undefined
  unit_price: number
  quantity: number
  duration: number
}>({
  product_id: undefined,
  billing_mode: 'monthly',
  unit_price: 0,
  quantity: 1,
  duration: 1,
})

const durationLabel = computed(() => {
  if (form.billing_mode === 'hourly') return '时长（小时）'
  if (form.billing_mode === 'monthly') return '时长（月）'
  return '数量系数'
})

const summaryHint = computed(() => {
  const mode = billingModeOptions.find((item) => item.value === form.billing_mode)?.label || ''
  if (form.billing_mode === 'hourly') {
    return `${mode} · ${form.quantity} 台 × ${form.duration} 小时`
  }
  if (form.billing_mode === 'monthly') {
    return `${mode} · ${form.quantity} 台 × ${form.duration} 月`
  }
  return `${mode} · ${form.quantity} 台`
})

const totalAmount = computed(() => {
  const base = form.unit_price * form.quantity
  if (form.billing_mode === 'hourly') return base * form.duration
  if (form.billing_mode === 'monthly') return base * form.duration
  return base
})

const detailColumns: PrimaryTableCol[] = [
  { colKey: 'item', title: '明细项', width: 180 },
  { colKey: 'value', title: '说明', minWidth: 220 },
  { colKey: 'amount', title: '金额（元）', width: 140 },
]

const priceDetailRows = computed(() => {
  if (!form.product_id) return []
  return [
    { key: 'quantity', item: '数量', value: `${form.quantity} 台`, amount: `¥${formatPrice(form.unit_price * form.quantity)}` },
    ...(form.billing_mode === 'hourly'
      ? [{ key: 'duration', item: '时长', value: `${form.duration} 小时`, amount: '—' }]
      : form.billing_mode === 'monthly'
        ? [{ key: 'duration', item: '时长', value: `${form.duration} 个月`, amount: '—' }]
        : []),
    { key: 'total', item: '合计', value: '预估总价', amount: `¥${formatPrice(totalAmount.value)}` },
  ]
})

function productName(id: number): string {
  return productMap.value[id]?.name || '—'
}

function applyUnitPrice() {
  if (!form.product_id) return
  const product = productMap.value[form.product_id]
  if (!product) return
  if (form.billing_mode === product.price_model) {
    form.unit_price = product.price
    return
  }
  const price = product.price
  if (form.billing_mode === 'hourly') {
    form.unit_price = Number((price / 720).toFixed(4))
  } else if (form.billing_mode === 'monthly') {
    form.unit_price = Number((price * 720).toFixed(2))
  }
}

function handleProductChange() {
  if (!form.product_id) return
  const product = productMap.value[form.product_id]
  if (!product) return
  form.billing_mode = product.price_model || 'fixed'
  form.unit_price = product.price
}

function resetForm() {
  Object.assign(form, {
    product_id: undefined,
    billing_mode: 'monthly',
    unit_price: 0,
    quantity: 1,
    duration: 1,
  })
}

async function loadProducts() {
  productLoading.value = true
  try {
    const data = await getProductList({ page: 1, page_size: 200 })
    productOptions.value = data.items.map((item: SaleProductInfo) => ({ label: item.name, value: item.id }))
    const map: Record<number, SaleProductInfo> = {}
    for (const item of data.items) map[item.id] = item
    productMap.value = map
  } catch {
    /* 商品加载失败不阻塞 */
  } finally {
    productLoading.value = false
  }
}

onMounted(() => {
  loadProducts()
})
</script>

<style lang="css">
@import '../../shared.css';

.product-module .result-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-top: var(--space-lg);
  padding: var(--space-lg);
  border-radius: var(--hs-radius-lg);
  background: linear-gradient(135deg, #f5f3ff, #eef2ff);
  border: 1px solid #e0e7ff;
  flex-wrap: wrap;
}

.product-module .result-bar__meta {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.product-module .result-bar__hint {
  margin-left: var(--space-sm);
  font-size: 12px;
  font-weight: 400;
  color: var(--color-muted-foreground);
}

.product-module .result-bar__total {
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.product-module .result-bar__total strong {
  font-size: 22px;
  font-weight: 700;
  color: #5b21b6;
  margin-left: var(--space-sm);
}
</style>
