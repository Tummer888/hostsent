<template>
  <ProductForm
    mode="create"
    :submitting="submitting"
    @submit="handleSubmit"
    @cancel="goBack"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { MessagePlugin } from 'tdesign-vue-next'

import { createProduct } from '@/api/product'
import ProductForm from './ProductForm.vue'
import type { SaleProductCreateRequest } from '@/types/interface'

defineOptions({ name: 'ProductProductsCreate' })

const router = useRouter()

const submitting = ref(false)

function goBack() {
  router.push('/product/products')
}

async function handleSubmit(payload: SaleProductCreateRequest) {
  submitting.value = true
  try {
    await createProduct(payload)
    MessagePlugin.success('产品已创建')
    router.push('/product/products')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建产品失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({ goBack })
</script>
