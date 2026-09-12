<template>
  <ProductForm
    mode="edit"
    :initial="product"
    :submitting="submitting"
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
import type { SaleProductCreateRequest, SaleProductInfo } from '@/types/interface'

defineOptions({ name: 'ProductProductsEdit' })

const route = useRoute()
const router = useRouter()

const product = ref<SaleProductInfo | null>(null)
const submitting = ref(false)

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

async function handleSubmit(payload: SaleProductCreateRequest) {
  const id = Number(route.params.id)
  submitting.value = true
  try {
    // 更新接口不接受 code（SKU 编码不可变），其余字段与创建同构。
    const { code: _code, ...rest } = payload
    void _code
    await updateProduct(id, rest)
    MessagePlugin.success('产品已保存')
    router.push('/product/products')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存产品失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadProduct()
})
</script>
