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
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { createOrder, getProducts, type ProductInfo } from '@/api/shop'

defineOptions({ name: 'Shop' })

const products = ref<ProductInfo[]>([])
const loading = ref(false)
const buyingId = ref<number | null>(null)

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
  buyingId.value = p.id
  try {
    await createOrder(p.id)
    MessagePlugin.success(`已购买「${p.name}」，资源开通中`)
  } catch (e: any) {
    MessagePlugin.error(e?.message || '购买失败')
  } finally {
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
</style>
