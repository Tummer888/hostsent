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
    const created = await createProduct(payload)
    const generated = created.generated_specs?.length || 0
    if (created.spec_template_notice) {
      // 商品已建但规格没全成：留在列表页让运营进详情页补，不要假装成功。
      MessagePlugin.warning(`产品已创建，但规格生成有问题：${created.spec_template_notice}`)
    } else if (generated > 0) {
      MessagePlugin.success(`产品已创建，并生成 ${generated} 个规格（平台绑定已确认）`)
    } else {
      MessagePlugin.success('产品已创建')
    }
    router.push('/product/products')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建产品失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({ goBack })
</script>
