<template>
  <div class="shop-page">
    <div class="shop-header">
      <h2 class="shop-title">云主机选购</h2>
      <span class="shop-sub">选择心仪的云主机，余额支付即时开通</span>
    </div>

    <div v-loading="loading" class="shop-grid">
      <t-card v-for="p in products" :key="p.id" class="shop-card" :bordered="true">
        <div class="shop-card__name">{{ p.name }}</div>
        <div class="shop-card__desc">{{ p.description?.slice(0, 80) || '暂无描述' }}</div>
        <div class="shop-card__spec" v-if="p.specs">{{ p.specs }}</div>
        <div class="shop-card__footer">
          <span class="shop-card__price">¥{{ p.price.toFixed(2) }}</span>
          <t-button size="small" theme="primary" :loading="buyingId === p.id" @click="handleBuy(p)">
            购买
          </t-button>
        </div>
      </t-card>

      <t-empty v-if="!loading && !products.length" description="暂无可购买商品" />
    </div>

    <!-- 下单确认：展示原价/优惠/实付明细（P5-06） -->
    <t-dialog
      v-model:visible="confirmVisible"
      header="确认下单"
      :confirm-btn="{ content: '确认支付', loading: submitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="submitOrder"
    >
      <div v-loading="quoteLoading" class="confirm">
        <div class="confirm__product">{{ current?.name }}</div>
        <div class="confirm__spec" v-if="current?.specs">{{ current.specs }}</div>

        <div class="confirm__prices" v-if="quote">
          <div class="confirm__row">
            <span>商品原价</span>
            <span>¥{{ quote.original_amount.toFixed(2) }}</span>
          </div>
          <div class="confirm__row confirm__row--discount" v-if="quote.discount_amount > 0">
            <span>
              优惠金额
              <t-tag v-if="quote.discount_source" size="small" theme="success" variant="light">
                {{ sourceLabel(quote.discount_source) }}
              </t-tag>
            </span>
            <span>-¥{{ quote.discount_amount.toFixed(2) }}</span>
          </div>
          <div class="confirm__row confirm__row--final">
            <span>应付金额</span>
            <span>¥{{ quote.final_amount.toFixed(2) }}</span>
          </div>
        </div>
        <t-empty v-else-if="!quoteLoading" description="价格计算失败，请稍后重试" />
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { createOrder, getProducts, quoteOrder, type ProductInfo, type QuoteInfo } from '@/api/shop'

defineOptions({ name: 'Shop' })

const products = ref<ProductInfo[]>([])
const loading = ref(false)
const buyingId = ref<number | null>(null)

const confirmVisible = ref(false)
const current = ref<ProductInfo | null>(null)
const quote = ref<QuoteInfo | null>(null)
const quoteLoading = ref(false)
const submitting = ref(false)

/** 折扣来源中文标签（P5-06）：折扣仅由用户组价格策略承载。 */
function sourceLabel(source: string) {
  const map: Record<string, string> = {
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source
}

async function loadProducts() {
  loading.value = true
  try {
    const { data } = await getProducts({ page_size: 60 })
    products.value = data?.items || []
  } catch (e) {
    MessagePlugin.error('加载商品失败')
  } finally {
    loading.value = false
  }
}

async function handleBuy(p: ProductInfo) {
  current.value = p
  quote.value = null
  confirmVisible.value = true
  quoteLoading.value = true
  try {
    const { data } = await quoteOrder(p.id)
    quote.value = data
  } catch (e: any) {
    MessagePlugin.error(e?.message || '价格计算失败')
  } finally {
    quoteLoading.value = false
  }
}

async function submitOrder() {
  if (!current.value) return
  submitting.value = true
  buyingId.value = current.value.id
  try {
    await createOrder(current.value.id)
    MessagePlugin.success(`已购买「${current.value.name}」，资源开通中`)
    confirmVisible.value = false
  } catch (e: any) {
    MessagePlugin.error(e?.message || '购买失败')
  } finally {
    submitting.value = false
    buyingId.value = null
  }
}

onMounted(loadProducts)
</script>

<style scoped>
.shop-page { padding: 16px 24px; }
.shop-header { margin-bottom: 20px; }
.shop-title { font-size: 20px; font-weight: 700; margin: 0 0 4px; }
.shop-sub { color: #888; font-size: 13px; }
.shop-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}
.shop-card__name { font-weight: 700; font-size: 15px; margin-bottom: 6px; }
.shop-card__desc { color: #666; font-size: 13px; min-height: 36px; margin-bottom: 8px; }
.shop-card__spec { color: #999; font-size: 12px; margin-bottom: 12px; }
.shop-card__footer { display: flex; align-items: center; justify-content: space-between; }
.shop-card__price { color: #e37318; font-size: 18px; font-weight: 700; }

.confirm__product { font-weight: 700; font-size: 15px; }
.confirm__spec { color: #999; font-size: 12px; margin: 4px 0 12px; }
.confirm__prices { border-top: 1px solid var(--td-border-level-1-color, #eee); padding-top: 12px; }
.confirm__row { display: flex; align-items: center; justify-content: space-between; padding: 6px 0; font-size: 14px; color: #555; }
.confirm__row--discount { color: #e37318; }
.confirm__row--final { font-size: 16px; font-weight: 700; color: #e37318; border-top: 1px dashed var(--td-border-level-1-color, #eee); margin-top: 6px; padding-top: 10px; }
</style>
