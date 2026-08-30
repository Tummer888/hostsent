<template>
  <ProductForm
    mode="edit"
    :initial="product"
    @submit="handleSubmit"
    @cancel="goBack"
  />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { MessagePlugin } from 'tdesign-vue-next'

import { getProductDetail, updateProduct } from '@/api/product'
import ProductForm from './ProductForm.vue'
import type { SaleProductInfo, SaleProductUpdateRequest } from '@/types/interface'

defineOptions({ name: 'ProductProductsEdit' })

const route = useRoute()
const router = useRouter()

const product = ref<SaleProductInfo | null>(null)

function goBack() {
  router.push('/product/products')
}

async function loadProduct() {
  const id = Number(route.params.id)
  try {
    product.value = await getProductDetail(id)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品失败')
    goBack()
  }
}

async function handleSubmit(payload: Record<string, unknown>) {
  const id = Number(route.params.id)
  try {
    await updateProduct(id, payload as unknown as SaleProductUpdateRequest)
    MessagePlugin.success('产品已保存')
    router.push('/product/products')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存产品失败')
  }
}

onMounted(() => {
  loadProduct()
})
</script>
