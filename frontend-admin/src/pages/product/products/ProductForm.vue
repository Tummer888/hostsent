<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AddIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ pageTitle }}</h2>
          <p class="page-header__desc">{{ headerDesc }}</p>
        </div>
      </div>
    </header>

    <!-- 代理（上游转售）商品的链路与上游绑定由「导入上游商品」决定，此处只读展示，禁止在表单里挑选链路。 -->
    <section v-if="isUpstream" class="form-card surface-card">
      <t-alert theme="warning" message="该商品为上游转售商品，链路与上游绑定在导入时确定，不可在本表单修改。">
        <template #message>
          <div class="upstream-readonly">
            <p>该商品为「上游转售」商品，链路与上游绑定在「导入上游商品」时确定，不可在本表单修改。</p>
            <t-descriptions :column="2" size="small" bordered>
              <t-descriptions-item label="链路">
                <t-tag theme="warning" variant="light" size="small" shape="round">上游转售</t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="上游提供商 ID">{{ initial?.source_provider_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="上游资源商品 ID">{{ initial?.source_product_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="上游加价规则">
                {{ markupLabel(initial?.upstream_markup_type || '', initial?.upstream_markup_value || 0) }}
              </t-descriptions-item>
            </t-descriptions>
            <p class="upstream-readonly__hint">
              如需调整售价，请在「商品调价」或「周期价格」中操作；规格由上游决定，无需在此维护 SKU。
            </p>
          </div>
        </template>
      </t-alert>
      <div class="form-footer">
        <t-button variant="outline" @click="handleCancel">取消</t-button>
        <t-button theme="primary" :loading="submitting" @click="handleSubmitUpstream">保存基础信息</t-button>
      </div>
    </section>

    <section v-else class="form-card surface-card">
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
          <t-form-item
            label="平台渠道"
            name="source_provider_id"
            :rules="platformRules"
          >
            <t-select
              v-model="form.source_provider_id"
              placeholder="请选择自营平台渠道（魔方云等）"
              :options="platformOptions"
              :loading="platformLoading"
              clearable
            />
            <span class="form-hint">云主机必须绑定一个「算力平台」渠道，开通/暂停/销毁将下发到该平台。</span>
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
        </div>

        <t-form-item label="封面图 URL" name="cover_image">
          <t-input v-model="form.cover_image" placeholder="选填，官网产品卡与详情页展示用，如 /branding/logo.svg" clearable />
        </t-form-item>

        <div v-if="form.cover_image" class="cover-preview">
          <span class="cover-preview__label">封面预览</span>
          <img class="cover-preview__img" :src="form.cover_image" alt="封面预览">
        </div>

        <t-form-item label="产品描述" name="description">
          <t-textarea v-model="form.description" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="选填，产品简介" />
        </t-form-item>

        <t-form-item label="平台参数 JSON（开通默认值）" name="config_options">
          <t-textarea
            v-model="form.config_options"
            :autosize="{ minRows: 3, maxRows: 8 }"
            placeholder='选填，如 {"area":"1","node":"2","network_type":"normal","ip_num":1}。留空则由「规格变体」的平台绑定下发。'
          />
          <span class="form-hint">
            此处填写的键会作为开通默认值；具体 SKU 的「平台绑定」优先级更高，可在此商品下逐个规格覆盖。
          </span>
        </t-form-item>

        <t-alert v-if="mode === 'create'" theme="info">
          <template #message>
            创建后请到商品详情页的「规格变体」中，用「规格模板」批量生成 SKU 并确认平台绑定，否则无法上架。
          </template>
        </t-alert>

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

import { getProviderList } from '@/api/admin'
import {
  markupLabel,
  priceModelOptions,
  productStatusOptions,
  productTypeOptions,
} from '@/pages/product/constants'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import type { ProviderInfo, SaleProductCreateRequest, SaleProductInfo } from '@/types/interface'

const props = defineProps<{
  mode: 'create' | 'edit'
  initial?: SaleProductInfo | null
  /** 提交中由父页（真正发请求的一侧）持有，避免子组件 emit 后立即复位 loading。 */
  submitting?: boolean
}>()

const emit = defineEmits<{
  (e: 'submit', payload: SaleProductCreateRequest): void
  (e: 'cancel'): void
}>()

/** 代理（上游转售）商品：链路由导入决定，表单不提供链路选择。 */
const isUpstream = computed(() => props.initial?.source_mode === 'upstream')

const pageTitle = computed(() => (props.mode === 'create' ? '新建自营产品' : '编辑产品'))
const headerDesc = computed(() =>
  props.mode === 'create'
    ? '自营商品：平台渠道在本表单绑定，规格通过「规格模板」生成 SKU 后映射到平台字段。'
    : '自营商品：链路固定为自营，平台渠道与规格映射在详情页维护。',
)

const { categoryOptions, loadCategories } = useCategoryOptions()

// 平台渠道下拉：只列「算力平台」（kind=compute），上游转售渠道不出现在自营商品表单里。
const platformOptions = ref<{ label: string; value: number }[]>([])
const platformLoading = ref(false)

async function loadPlatformOptions() {
  platformLoading.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    platformOptions.value = data.items
      .filter((item: ProviderInfo) => item.kind === 'compute')
      .map((item: ProviderInfo) => ({ label: `${item.name}（${item.provider_type}）`, value: item.id }))
  } catch {
    platformOptions.value = []
  } finally {
    platformLoading.value = false
  }
}

const form = reactive({
  code: '',
  name: '',
  category_id: undefined as number | undefined,
  product_type: 'cloud_host',
  description: '',
  cover_image: '',
  price_model: 'fixed',
  price: 0,
  cost_price: 0,
  source_provider_id: undefined as number | undefined,
  config_options: '',
  stock: -1,
  sort_order: 0,
  status: 0,
})

// 平台渠道仅在「云主机」类商品上必填：它决定开通/暂停/销毁下发到哪个平台；
// 虚拟主机、数据库等纯本地商品不强制绑定渠道（存量演示商品即属此类）。
const platformRules = computed(() =>
  form.product_type === 'cloud_host'
    ? [{ required: true, message: '请选择开通该商品的平台渠道' }]
    : [],
)

watch(
  () => props.initial,
  (initial) => {
    if (initial) {
      form.code = initial.code
      form.name = initial.name
      form.category_id = initial.category_id || undefined
      form.product_type = initial.product_type
      form.description = initial.description || ''
      form.cover_image = initial.cover_image || ''
      form.price_model = initial.price_model
      form.price = initial.price
      form.cost_price = initial.cost_price
      form.source_provider_id = initial.source_provider_id || undefined
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

function buildPayload(): SaleProductCreateRequest | null {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return null
  }
  if (props.mode === 'create' && !form.code.trim()) {
    MessagePlugin.warning('请输入 SKU 编码')
    return null
  }
  // 云主机必须绑定平台渠道才有履约通道；虚拟/服务类商品（如演示虚拟主机）不强制。
  const isCloud = form.product_type === 'cloud_host'
  if (isCloud && !form.source_provider_id) {
    MessagePlugin.warning('请选择自营平台渠道')
    return null
  }
  // 自营专属 payload：链路恒为 self，不携带任何上游转售字段（source_product_id / 加价 / 仅透传）。
  return {
    code: props.mode === 'create' ? form.code.trim() : form.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    source_mode: 'self',
    source_product_id: 0,
    source_provider_id: form.source_provider_id,
    config_options: form.config_options,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    upstream_markup_type: '',
    upstream_markup_value: 0,
    spec_passthrough: false,
  }
}

function handleSubmit() {
  const payload = buildPayload()
  if (payload) emit('submit', payload)
}

/**
 * 代理商品只允许改基础信息：链路与上游绑定字段原样回传（后端 Update 对未传字段保持原值，
 * 这里显式回传 source_mode 是幂等的，避免"编辑一次就被改成自营"的历史缺陷）。
 */
function handleSubmitUpstream() {
  if (!props.initial) return
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return
  }
  const payload: SaleProductCreateRequest = {
    code: props.initial.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    source_mode: 'upstream',
    source_product_id: props.initial.source_product_id,
    source_provider_id: props.initial.source_provider_id,
    config_options: form.config_options,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    upstream_markup_type: props.initial.upstream_markup_type || '',
    upstream_markup_value: props.initial.upstream_markup_value || 0,
    spec_passthrough: !!props.initial.spec_passthrough,
  }
  emit('submit', payload)
}

onMounted(() => {
  loadCategories()
  loadPlatformOptions()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.upstream-readonly {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.upstream-readonly__hint {
  margin: 0;
  color: var(--td-text-color-secondary, #999);
  font-size: 13px;
}
</style>
