<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AddIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ pageTitle }}</h2>
        </div>
      </div>
    </header>

    <section class="form-card surface-card">
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
            <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
          </t-form-item>
          <t-form-item label="SKU 编码" name="code" :rules="[{ required: mode === 'create', message: 'SKU 编码不能为空' }]">
            <t-input v-model="form.code" placeholder="如 cloud-host-basic" :disabled="mode === 'edit'" clearable />
          </t-form-item>
          <t-form-item label="分类" name="category_id">
            <t-select v-model="form.category_id" clearable placeholder="请选择分类" :options="categoryOptions" />
          </t-form-item>
          <t-form-item label="产品类型" name="product_type">
            <t-select v-model="form.product_type" clearable placeholder="请选择类型" :options="productTypeOptions" />
          </t-form-item>
          <t-form-item label="供货模式" name="provision_mode">
            <t-select v-model="form.provision_mode" placeholder="请选择供货模式" :options="provisionModeOptions" />
          </t-form-item>
          <t-form-item label="价格模型" name="price_model">
            <t-select v-model="form.price_model" placeholder="请选择价格模型" :options="priceModelOptions" />
          </t-form-item>
          <t-form-item label="库存" name="stock">
            <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
          </t-form-item>
          <t-form-item label="销售价（元）" name="price">
            <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
          </t-form-item>
          <t-form-item label="成本价（元）" name="cost_price">
            <t-input-number v-model="form.cost_price" :min="0" :precision="2" theme="column" placeholder="请输入成本价" />
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="数值越小越靠前" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" placeholder="请选择状态" :options="productStatusOptions" />
          </t-form-item>
          <t-form-item label="关联上游资源商品 ID" name="source_product_id">
            <t-input-number v-model="form.source_product_id" :min="0" theme="column" placeholder="选填" />
          </t-form-item>
          <t-form-item label="关联上游提供商 ID" name="source_provider_id">
            <t-input-number v-model="form.source_provider_id" :min="0" theme="column" placeholder="选填" />
          </t-form-item>
        </div>

        <t-form-item label="产品描述" name="description">
          <t-textarea v-model="form.description" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="选填，产品简介" />
        </t-form-item>

        <t-form-item label="规格 JSON" name="specs">
          <t-textarea
            v-model="form.specs"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder='选填，如 {"cpu":4,"memory":8,"disk":80}'
          />
        </t-form-item>

        <t-form-item label="上游配置选项 JSON（自营映射 /clouds 参数）" name="config_options">
          <t-textarea
            v-model="form.config_options"
            :autosize="{ minRows: 3, maxRows: 8 }"
            placeholder='选填，如 {"area":1,"os":"centos7","cpu":2,"memory":4096,"bw":10,"ip_num":1,"network_type":"normal"}'
          />
        </t-form-item>

        <div class="form-footer">
          <t-button variant="outline" @click="handleCancel">取消</t-button>
          <t-button theme="primary" :loading="submitting" @click="handleSubmit">{{ mode === 'create' ? '创建' : '保存' }}</t-button>
        </div>
      </t-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'

import { AddIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getProductCategoryList } from '@/api/product'
import { priceModelOptions, productStatusOptions, productTypeOptions, provisionModeOptions } from '@/pages/product/constants'
import type { SaleProductCategoryInfo, SaleProductInfo } from '@/types/interface'

const props = defineProps<{
  mode: 'create' | 'edit'
  initial?: SaleProductInfo | null
}>()

const emit = defineEmits<{
  (e: 'submit', payload: Record<string, unknown>): void
  (e: 'cancel'): void
}>()

const pageTitle = computed(() => (props.mode === 'create' ? '新建产品' : '编辑产品'))

const categoryOptions = ref<{ label: string; value: number }[]>([])
const submitting = ref(false)

const form = reactive({
  code: '',
  name: '',
  category_id: undefined as number | undefined,
  product_type: 'cloud_host',
  description: '',
  specs: '',
  price_model: 'fixed',
  price: 0,
  cost_price: 0,
  source_product_id: undefined as number | undefined,
  source_provider_id: undefined as number | undefined,
  provision_mode: 'self',
  config_options: '',
  stock: -1,
  sort_order: 0,
  status: 0,
})

async function loadCategories() {
  try {
    const data = await getProductCategoryList()
    const options: { label: string; value: number }[] = []
    const flatten = (nodes: SaleProductCategoryInfo[]) => {
      for (const node of nodes) {
        options.push({ label: node.name, value: node.id })
        if (node.children?.length) flatten(node.children)
      }
    }
    flatten(data.items)
    categoryOptions.value = options
  } catch {
    /* 分类加载失败不阻塞表单 */
  }
}

watch(
  () => props.initial,
  (initial) => {
    if (initial) {
      form.code = initial.code
      form.name = initial.name
      form.category_id = initial.category_id || undefined
      form.product_type = initial.product_type
      form.description = initial.description || ''
      form.specs = initial.specs || ''
      form.price_model = initial.price_model
      form.price = initial.price
      form.cost_price = initial.cost_price
      form.source_product_id = initial.source_product_id || undefined
      form.source_provider_id = initial.source_provider_id || undefined
      form.provision_mode = initial.provision_mode || 'self'
      form.config_options = initial.config_options || ''
      form.stock = initial.stock
      form.sort_order = initial.sort_order
      form.status = initial.status
    }
  },
  { immediate: true },
)

function handleCancel() {
  emit('cancel')
}

async function handleSubmit() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return
  }
  if (props.mode === 'create' && !form.code.trim()) {
    MessagePlugin.warning('请输入 SKU 编码')
    return
  }
  submitting.value = true
  try {
    const payload: Record<string, unknown> = {
      name: form.name.trim(),
      category_id: form.category_id || 0,
      product_type: form.product_type,
      description: form.description,
      specs: form.specs,
      price_model: form.price_model,
      price: form.price,
      cost_price: form.cost_price,
      source_product_id: form.source_product_id || 0,
      source_provider_id: form.source_provider_id || 0,
      provision_mode: form.provision_mode,
      config_options: form.config_options,
      stock: form.stock,
      sort_order: form.sort_order,
      status: form.status,
    }
    if (props.mode === 'create') {
      payload.code = form.code.trim()
    }
    emit('submit', payload)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadCategories()
})
</script>

<style lang="css">
@import '../shared.css';
</style>
