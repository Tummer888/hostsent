<template>
  <ProductForm
    mode="create"
    @submit="handleSubmit"
    @cancel="goBack"
  />
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

import { MessagePlugin } from 'tdesign-vue-next'

import { createProduct } from '@/api/product'
import ProductForm from './ProductForm.vue'
import type { SaleProductCreateRequest } from '@/types/interface'

defineOptions({ name: 'ProductProductsCreate' })

const router = useRouter()

function goBack() {
  router.push('/product/products')
}

async function handleSubmit(payload: Record<string, unknown>) {
  try {
    await createProduct(payload as unknown as SaleProductCreateRequest)
    MessagePlugin.success('产品已创建')
    router.push('/product/products')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建产品失败')
  }
}

defineExpose({ goBack })
</script>
