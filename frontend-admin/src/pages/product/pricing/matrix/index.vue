<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">周期价格</h2>
          <p class="page-header__desc">
            商品 × 规格 × 计费周期的价格矩阵。格子里存的是折后价，代理折扣与限时活动折扣在其上按身份二选一叠加。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="editorLoading" :disabled="!productId" @click="reload">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" :loading="editorSaving" :disabled="!productId" @click="handleSave">
          保存矩阵
        </t-button>
      </t-space>
    </header>

    <FilterCard>
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
          @change="onSpecChange"
        />
      </div>
      <div class="field">
        <span class="field__label">币种</span>
        <t-input v-model="currency" disabled />
      </div>
    </FilterCard>

    <section v-if="productId" class="table-card surface-card">
      <!-- 矩阵表本体与商品详情页「价格」页签共用同一组件（口径只有一份）。 -->
      <PriceMatrixEditor
        ref="editorRef"
        :auto-load="false"
        :product-id="productId || 0"
        :spec-id="specId"
        @update:loading="editorLoading = $event"
        @update:saving="editorSaving = $event"
      />
    </section>

    <section v-else class="table-card surface-card">
      <t-empty description="请先选择商品，再维护其周期价格矩阵" />
    </section>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { onMounted, ref } from 'vue'

import { MoneyIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getProductList, getProductSpecs } from '@/api/product'
import PriceMatrixEditor from '@/pages/product/products/PriceMatrixEditor.vue'

defineOptions({ name: 'ProductPricingMatrix' })

const productOptions = ref<{ label: string; value: number }[]>([])
const specOptions = ref<{ label: string; value: number }[]>([])
const productLoading = ref(false)
// 加载/保存状态由矩阵组件回调上来，页头按钮的 loading 才跟得上。
const editorLoading = ref(false)
const editorSaving = ref(false)

const productId = ref<number | undefined>(undefined)
const specId = ref<number | undefined>(undefined)
const currency = ref('CNY')

const editorRef = ref<{ reload: () => Promise<void>; save: () => Promise<boolean> } | null>(null)

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
  if (!id) return
  await loadSpecs(id)
  await editorRef.value?.reload()
}

async function onSpecChange() {
  await editorRef.value?.reload()
}

async function reload() {
  await editorRef.value?.reload()
}

async function handleSave() {
  await editorRef.value?.save()
}

onMounted(loadProducts)
</script>

<style scoped>
@import '../../shared.css';
</style>
