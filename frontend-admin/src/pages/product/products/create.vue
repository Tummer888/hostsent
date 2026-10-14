<template>
  <ProductForm mode="create" :submitting="submitting" @submit="handleSubmit" @cancel="goBack" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { MessagePlugin } from 'tdesign-vue-next'

import { createProduct, saveProductPriceMatrix } from '@/api/product'
import ProductForm from './ProductForm.vue'
import type { SaleProductCreateRequest } from '@/types/interface'

defineOptions({ name: 'ProductProductsCreate' })

const router = useRouter()

const submitting = ref(false)

function goBack() {
  router.push('/product/products')
}

/**
 * saveCyclePrices 创建商品后落周期价格矩阵。
 *
 * 为什么不在创建接口里一并提交：周期价走独立的价格矩阵接口（按商品/规格维度维护），
 * 创建接口只认商品本体字段。分开也更安全——矩阵写失败不该让整个建品回滚，
 * 商品已建好，运营可在详情页补价。
 */
async function saveCyclePrices(productId: number, prices?: Record<string, number>) {
  if (!productId || !prices || !Object.keys(prices).length) return
  try {
    await saveProductPriceMatrix({
      product_id: productId,
      items: Object.entries(prices).map(([cycle, price]) => ({
        cycle,
        price,
        cost_price: 0,
        setup_fee: 0,
        cost_setup_fee: 0,
        status: 1,
      })),
    })
  } catch (error) {
    // 只提示不阻断：商品已创建成功，价格可在详情页补。
    MessagePlugin.warning(`周期价格写入失败，请到商品详情页补：${(error as Error).message || '未知原因'}`)
  }
}

async function handleSubmit(payload: SaleProductCreateRequest, cyclePrices?: Record<string, number>) {
  submitting.value = true
  try {
    const created = await createProduct(payload)
    await saveCyclePrices(created.id, cyclePrices)
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
