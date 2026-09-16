<template>
  <t-dialog
    v-model:visible="visible"
    header="为用户创建订单"
    width="560px"
    :confirm-btn="{ content: '创建订单', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
      <t-form-item label="下单用户">
        <t-input :model-value="username || '—'" disabled />
      </t-form-item>
      <t-form-item label="选择商品" name="product_id">
        <t-select v-model="form.product_id" :options="productOptions" filterable placeholder="请选择商品" />
      </t-form-item>
      <t-form-item label="计费周期" name="billing_cycle">
        <t-select v-model="form.billing_cycle" :options="billingCycleOptions" placeholder="请选择计费周期" />
      </t-form-item>
      <t-form-item label="价格（元）" name="price">
        <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" placeholder="留空使用商品默认价" />
      </t-form-item>
      <t-form-item label="支付方式" name="pay_mode">
        <t-radio-group v-model="form.pay_mode" variant="default-filled">
          <t-radio-button value="create">仅创建（待支付）</t-radio-button>
          <t-radio-button value="balance">余额支付并开通</t-radio-button>
        </t-radio-group>
      </t-form-item>
      <p class="form-hint">
        余额支付会立即从用户钱包扣款并投递开通任务；余额不足时下单会失败且不会产生订单。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import { createUserOrder } from '@/api/user'
import { getProductList as getUcProductList } from '@/api/product'
import { billingCycleOptions } from '@/pages/users/constants'

const props = defineProps<{
  modelValue: boolean
  userId: number
  username: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = ref(props.modelValue)
watch(() => props.modelValue, (value) => {
  visible.value = value
  if (value) {
    form.product_id = undefined
    form.billing_cycle = 'monthly'
    form.price = 0
    form.pay_mode = 'create'
    void loadProducts()
  }
})
watch(visible, (value) => emit('update:modelValue', value))

const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const productOptions = ref<{ label: string; value: number }[]>([])
const form = reactive<{
  product_id: number | undefined
  billing_cycle: string
  price: number
  pay_mode: 'create' | 'balance'
}>({
  product_id: undefined,
  billing_cycle: 'monthly',
  price: 0,
  pay_mode: 'create',
})

const rules: Record<string, FormRule[]> = {
  product_id: [{ required: true, message: '请选择商品', type: 'error' }],
  billing_cycle: [{ required: true, message: '请选择计费周期', type: 'error' }],
}

async function loadProducts() {
  if (productOptions.value.length) return
  try {
    const data = await getUcProductList({ page: 1, page_size: 100 })
    productOptions.value = (data?.items || []).map((p) => ({
      label: `${p.name}（¥${p.price}）`,
      value: p.id,
    }))
  } catch {
    MessagePlugin.error('加载商品列表失败')
  }
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  if (!form.product_id) return
  submitting.value = true
  try {
    await createUserOrder(props.userId, {
      product_id: form.product_id,
      billing_cycle: form.billing_cycle,
      price: form.price > 0 ? form.price : undefined,
      pay_mode: form.pay_mode,
    })
    MessagePlugin.success(form.pay_mode === 'balance' ? '已支付并投递开通任务' : '订单已创建（待支付）')
    visible.value = false
    emit('saved')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建订单失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.form-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}
</style>