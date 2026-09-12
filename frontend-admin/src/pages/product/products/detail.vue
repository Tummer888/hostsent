<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ product?.name || '产品详情' }}</h2>
          <p class="page-header__desc">SKU: {{ product?.code || '—' }}</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadDetail">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <t-button theme="primary" @click="goEdit">编辑</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="base" label="基本信息">
          <div class="tabs-section">
            <t-descriptions v-if="product" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="产品名称">{{ product.name }}</t-descriptions-item>
              <t-descriptions-item label="SKU 编码">{{ product.code }}</t-descriptions-item>
              <t-descriptions-item label="分类">{{ categoryName(product.category_id) }}</t-descriptions-item>
              <t-descriptions-item label="产品类型">{{ product.product_type || '—' }}</t-descriptions-item>
              <t-descriptions-item label="链路">
                <t-tag :theme="sourceModeTag(product.source_mode).theme" variant="light" size="small" shape="round">
                  {{ sourceModeTag(product.source_mode).text }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="价格模型">{{ priceModelLabel(product.price_model) }}</t-descriptions-item>
              <t-descriptions-item label="已售库存">
                <span>{{ product.stock === -1 ? '不限' : product.stock }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="销售价">¥{{ formatPrice(product.price) }}</t-descriptions-item>
              <t-descriptions-item label="成本价">¥{{ formatPrice(product.cost_price) }}</t-descriptions-item>
              <t-descriptions-item label="上游加价规则">
                <span>{{ markupLabel(product.upstream_markup_type, product.upstream_markup_value) }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="仅透传上游参数">
                <t-tag :theme="product.spec_passthrough ? 'warning' : 'default'" variant="light" size="small" shape="round">
                  {{ product.spec_passthrough ? '是' : '否' }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="关联上游商品 ID">
                <span>{{ product.source_product_id || '—' }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="关联上游提供商 ID">
                <span>{{ product.source_provider_id || '—' }}</span>
              </t-descriptions-item>
              <t-descriptions-item label="排序">{{ product.sort_order }}</t-descriptions-item>
              <t-descriptions-item label="状态">
                <t-tag :theme="statusTag(product.status).theme" variant="light" size="small" shape="round">
                  {{ statusTag(product.status).text }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(product.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(product.updated_at) }}</t-descriptions-item>
              <t-descriptions-item label="产品描述" :span="2">{{ product.description || '—' }}</t-descriptions-item>
              <t-descriptions-item label="封面图" :span="2">
                <img v-if="product.cover_image" class="detail-cover" :src="product.cover_image" alt="产品封面图">
                <span v-else>—</span>
              </t-descriptions-item>
              <t-descriptions-item label="规格 JSON" :span="2">
                <pre class="spec-pre">{{ product.specs || '—' }}</pre>
              </t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="specs" label="规格变体（SKU）">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">规格变体</h3>
              <t-space size="small">
                <span class="table-card__meta">共 {{ specs.length }} 个规格</span>
                <t-button size="small" theme="primary" @click="openSpecDialog()">新增规格</t-button>
              </t-space>
            </div>
            <t-alert
              theme="info"
              :message="isUpstreamChain
                ? '代理商品按上游规格开通；此处规格仅供展示。上架门禁要求上游规格完成平台绑定或开启「仅透传」。'
                : '自营商品上架前，每个启用 SKU 都需满足必填原子且完成平台绑定（绑定状态为「已确认」）。'"
            />
            <t-table
              row-key="id"
              :data="specs"
              :columns="specColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #binding_status="{ row }">
                <t-tag :theme="bindingStatusTag(row.binding_status).theme" variant="light" size="small" shape="round">
                  {{ bindingStatusTag(row.binding_status).text }}
                </t-tag>
              </template>
              <template #platform_params="{ row }">
                <span class="cell-muted">{{ row.platform_params || '—' }}</span>
              </template>
              <template #status="{ row }">
                <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </t-tag>
              </template>
              <template #op="{ row }">
                <t-space size="small">
                  <t-link theme="primary" @click="openSpecDialog(row)">编辑</t-link>
                  <t-link theme="primary" @click="openBindingDialog(row)">平台绑定</t-link>
                  <t-link theme="danger" @click="removeSpec(row)">删除</t-link>
                </t-space>
              </template>
              <template #empty>
                <t-empty description="暂无规格变体" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="config" label="可配置项">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">可配置项</h3>
              <span class="table-card__meta">
                source=upstream 走上游配置 id；source=self 直接作为平台写参数下发
              </span>
            </div>
            <t-alert
              theme="info"
              message="配置组 JSON 结构与上游 config_groups 对齐；group.options[].source/source_key 可标注来源。保存为覆盖式写入。"
            />
            <t-textarea
              v-model="configOptionsText"
              :autosize="{ minRows: 8, maxRows: 24 }"
              placeholder='[{"name":"平台参数","options":[{"option_name":"node","source":"self","source_key":"node","sub":[{"option_name":"一区","source":"self","source_key":"zone-a"}]}]}]'
            />
            <div class="form-footer">
              <t-button variant="outline" @click="loadConfigOptions">重置</t-button>
              <t-button theme="primary" :loading="configSaving" @click="saveConfigOptions">保存可配置项</t-button>
            </div>
          </div>
        </t-tab-panel>

        <t-tab-panel value="history" label="变更历史">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">变更历史</h3>
              <span class="table-card__meta">共 {{ history.length }} 条记录</span>
            </div>
            <t-table
              row-key="id"
              :data="history"
              :columns="historyColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #change_type="{ row }">
                <t-tag theme="primary" variant="light" size="small" shape="round">
                  {{ changeTypeLabel(row.change_type) }}
                </t-tag>
              </template>
              <template #created_at="{ row }">
                <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无变更历史" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- SKU 新增/编辑 -->
    <t-dialog
      v-model:visible="specDialogVisible"
      :header="specForm.id ? '编辑规格变体' : '新增规格变体'"
      width="640px"
      :confirm-btn="{ loading: specSaving }"
      @confirm="submitSpec"
    >
      <t-form label-align="top" :data="specForm">
        <div class="form-grid">
          <t-form-item label="规格编码" required>
            <t-input v-model="specForm.spec_code" placeholder="如 2c4g-30m" />
          </t-form-item>
          <t-form-item label="规格名称" required>
            <t-input v-model="specForm.name" placeholder="如 2核4G 30M" />
          </t-form-item>
          <t-form-item label="价格模型">
            <t-select v-model="specForm.price_model" :options="priceModelOptions" />
          </t-form-item>
          <t-form-item label="销售价（元）">
            <t-input-number v-model="specForm.price" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="成本价（元）">
            <t-input-number v-model="specForm.cost_price" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="库存（-1 不限）">
            <t-input-number v-model="specForm.stock" :min="-1" theme="column" />
          </t-form-item>
          <t-form-item label="规格模板">
            <t-select
              v-model="specForm.spec_template_id"
              clearable
              placeholder="可选，引用标准规格模板"
              :options="templateOptions"
            />
          </t-form-item>
          <t-form-item label="排序">
            <t-input-number v-model="specForm.sort_order" :min="0" theme="column" />
          </t-form-item>
          <t-form-item label="状态">
            <t-select v-model="specForm.status" :options="specStatusOptions" />
          </t-form-item>
        </div>
        <t-form-item label="规格取值 JSON（原子 key → 取值，如 compute.cpu / placement.region）">
          <t-textarea
            v-model="specForm.specs"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder='{"compute.cpu":2,"compute.memory":4096,"storage.system.size":60,"placement.region":"hk"}'
          />
        </t-form-item>
        <t-alert v-if="specAtoms.length" theme="info">
          <template #message>
            <div class="atom-hints">
              <span v-for="atom in specAtoms" :key="atom.key" class="atom-hint">
                {{ atom.key }}<em v-if="atom.required">*</em>({{ atom.name }}{{ atom.unit ? '/' + atom.unit : '' }})
              </span>
            </div>
          </template>
        </t-alert>
      </t-form>
    </t-dialog>

    <!-- 平台绑定 -->
    <t-dialog
      v-model:visible="bindingDialogVisible"
      :header="`平台绑定 · ${bindingSpec?.spec_code || ''}`"
      width="680px"
      :footer="false"
    >
      <div class="binding-panel">
        <div class="binding-panel__head">
          <span>已确认的出站参数将在开通时覆盖推导值（优先级最高）。</span>
          <t-button size="small" variant="outline" @click="newBinding">新建绑定</t-button>
        </div>
        <t-table
          row-key="id"
          :data="bindings"
          :columns="bindingColumns"
          :loading="bindingLoading"
          size="small"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #status="{ row }">
            <t-tag :theme="bindingStatusTag(row.status).theme" variant="light" size="small" shape="round">
              {{ bindingStatusTag(row.status).text }}
            </t-tag>
          </template>
          <template #op="{ row }">
            <t-space size="small">
              <t-link theme="primary" @click="editBinding(row)">编辑</t-link>
              <t-link v-if="row.status !== 'confirmed'" theme="primary" @click="confirmBinding(row)">确认</t-link>
            </t-space>
          </template>
          <template #empty>
            <t-empty description="暂无绑定，点击「新建绑定」建立平台参数映射" />
          </template>
        </t-table>

        <t-divider>绑定编辑</t-divider>
        <t-form label-align="top" :data="bindingForm">
          <div class="form-grid">
            <t-form-item label="方向">
              <t-select v-model="bindingForm.direction" :options="bindingDirectionOptions" />
            </t-form-item>
            <t-form-item label="匹配方式">
              <t-select v-model="bindingForm.match_type" :options="bindingMatchOptions" />
            </t-form-item>
            <t-form-item label="状态">
              <t-select v-model="bindingForm.status" :options="bindingStatusOptions" />
            </t-form-item>
            <t-form-item label="优先级">
              <t-input-number v-model="bindingForm.priority" :min="0" theme="column" />
            </t-form-item>
            <t-form-item label="置信度">
              <t-input-number v-model="bindingForm.confidence" :min="0" :max="100" theme="column" />
            </t-form-item>
            <t-form-item label="规格模板 ID">
              <t-input-number v-model="bindingForm.spec_template_id" :min="0" theme="column" />
            </t-form-item>
          </div>
          <t-form-item label="平台参数 JSON（area/node/store/pid/gid…）">
            <t-textarea
              v-model="bindingForm.platform_params"
              :autosize="{ minRows: 3, maxRows: 8 }"
              placeholder='{"area":"10|CN^襄阳电信","node":"1","store":"1"}'
            />
          </t-form-item>
          <t-form-item label="备注">
            <t-input v-model="bindingForm.remark" placeholder="选填" />
          </t-form-item>
          <div class="form-footer">
            <t-button theme="primary" :loading="bindingSaving" @click="submitBinding">
              {{ bindingForm.id ? '保存绑定' : '创建绑定' }}
            </t-button>
          </div>
        </t-form>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  confirmSpecBinding,
  createProductSpec,
  deleteProductSpec,
  getProductConfigOptions,
  getProductDetail,
  getProductHistory,
  getProductSpecs,
  getSpecAtomList,
  getSpecBindingList,
  getSpecTemplateList,
  saveProductConfigOptions,
  updateProductSpec,
  upsertSpecBinding,
} from '@/api/product'
import {
  bindingStatusTag,
  changeTypeLabel,
  formatPrice,
  formatTime,
  markupLabel,
  priceModelLabel,
  priceModelOptions,
  sourceModeTag,
  statusTag,
} from '@/pages/product/constants'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import type {
  SaleProductConfigGroup,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductSpecInfo,
  SaleProductSpecRequest,
  SpecAtomInfo,
  SpecBindingInfo,
  SpecBindingUpsertRequest,
} from '@/types/interface'

defineOptions({ name: 'ProductProductsDetail' })

const route = useRoute()
const router = useRouter()

const product = ref<SaleProductInfo | null>(null)
const specs = ref<SaleProductSpecInfo[]>([])
const history = ref<SaleProductHistoryInfo[]>([])
const specAtoms = ref<SpecAtomInfo[]>([])
const templateOptions = ref<{ label: string; value: number }[]>([])
const loading = ref(false)
const activeTab = ref('base')

const { categoryName, loadCategories } = useCategoryOptions()

const isUpstreamChain = computed(() => product.value?.source_mode === 'upstream')

const specColumns: PrimaryTableCol<SaleProductSpecInfo>[] = [
  { colKey: 'spec_code', title: '规格编码', minWidth: 120 },
  { colKey: 'name', title: '规格名称', minWidth: 120 },
  { colKey: 'price', title: '售价', width: 90 },
  { colKey: 'cost_price', title: '成本价', width: 90 },
  { colKey: 'stock', title: '库存', width: 70 },
  { colKey: 'binding_status', title: '平台绑定', width: 100 },
  { colKey: 'platform_params', title: '已确认参数', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 70 },
  { colKey: 'op', title: '操作', width: 190 },
]

const bindingColumns: PrimaryTableCol<SpecBindingInfo>[] = [
  { colKey: 'direction', title: '方向', width: 80 },
  { colKey: 'platform_params', title: '平台参数', minWidth: 220 },
  { colKey: 'match_type', title: '匹配', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'priority', title: '优先级', width: 80 },
  { colKey: 'op', title: '操作', width: 130 },
]

const historyColumns: PrimaryTableCol<SaleProductHistoryInfo>[] = [
  { colKey: 'change_type', title: '操作类型', width: 100 },
  { colKey: 'old_value', title: '变更前', minWidth: 140 },
  { colKey: 'new_value', title: '变更后', minWidth: 140 },
  { colKey: 'operator_name', title: '操作人', width: 110 },
  { colKey: 'remark', title: '备注', minWidth: 120 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

const specStatusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const bindingDirectionOptions = [
  { label: '出站（开通参数）', value: 'outbound' },
  { label: '入站（归一展示）', value: 'inbound' },
]

const bindingMatchOptions = [
  { label: '人工', value: 'manual' },
  { label: '精确匹配', value: 'auto_exact' },
  { label: '区间匹配', value: 'auto_range' },
]

const bindingStatusOptions = [
  { label: '未映射', value: 'unmapped' },
  { label: '自动映射', value: 'auto_mapped' },
  { label: '已确认', value: 'confirmed' },
  { label: '已失效', value: 'stale' },
]

// ---------- SKU 编辑 ----------

const specDialogVisible = ref(false)
const specSaving = ref(false)
const specForm = reactive({
  id: 0,
  spec_code: '',
  name: '',
  specs: '',
  price_model: 'fixed',
  price: 0,
  cost_price: 0,
  stock: -1,
  spec_template_id: undefined as number | undefined,
  sort_order: 0,
  status: 1,
})

function openSpecDialog(row?: SaleProductSpecInfo) {
  specForm.id = row?.id || 0
  specForm.spec_code = row?.spec_code || ''
  specForm.name = row?.name || ''
  specForm.specs = row?.specs || ''
  specForm.price_model = row?.price_model || 'fixed'
  specForm.price = row?.price || 0
  specForm.cost_price = row?.cost_price || 0
  specForm.stock = row?.stock ?? -1
  specForm.spec_template_id = row?.spec_template_id || undefined
  specForm.sort_order = row?.sort_order || 0
  specForm.status = row?.status ?? 1
  specDialogVisible.value = true
}

async function submitSpec() {
  if (!specForm.spec_code.trim() || !specForm.name.trim()) {
    MessagePlugin.warning('请填写规格编码与名称')
    return
  }
  if (specForm.specs.trim()) {
    try {
      JSON.parse(specForm.specs)
    } catch {
      MessagePlugin.warning('规格取值必须是合法 JSON')
      return
    }
  }
  const payload: SaleProductSpecRequest = {
    spec_code: specForm.spec_code.trim(),
    name: specForm.name.trim(),
    specs: specForm.specs,
    price_model: specForm.price_model,
    price: specForm.price,
    cost_price: specForm.cost_price,
    stock: specForm.stock,
    spec_template_id: specForm.spec_template_id || 0,
    sort_order: specForm.sort_order,
    status: specForm.status,
  }
  specSaving.value = true
  const id = Number(route.params.id)
  try {
    if (specForm.id) {
      await updateProductSpec(id, specForm.id, payload)
      MessagePlugin.success('规格已更新')
    } else {
      await createProductSpec(id, payload)
      MessagePlugin.success('规格已新增')
    }
    specDialogVisible.value = false
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存规格失败')
  } finally {
    specSaving.value = false
  }
}

async function removeSpec(row: SaleProductSpecInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除规格',
    body: `确认删除规格「${row.name}」？此操作不影响历史订单。`,
    theme: 'warning',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteProductSpec(Number(route.params.id), row.id)
        MessagePlugin.success('规格已删除')
        dialog.hide()
        await loadSpecs()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

// ---------- 平台绑定 ----------

const bindingDialogVisible = ref(false)
const bindingLoading = ref(false)
const bindingSaving = ref(false)
const bindingSpec = ref<SaleProductSpecInfo | null>(null)
const bindings = ref<SpecBindingInfo[]>([])
const bindingForm = reactive({
  id: 0,
  direction: 'outbound',
  match_type: 'manual',
  status: 'unmapped',
  priority: 0,
  confidence: 0,
  spec_template_id: 0,
  platform_params: '',
  remark: '',
})

async function openBindingDialog(row: SaleProductSpecInfo) {
  bindingSpec.value = row
  bindingDialogVisible.value = true
  resetBindingForm()
  await loadBindings()
}

function resetBindingForm() {
  bindingForm.id = 0
  bindingForm.direction = 'outbound'
  bindingForm.match_type = 'manual'
  bindingForm.status = 'unmapped'
  bindingForm.priority = 0
  bindingForm.confidence = 0
  bindingForm.spec_template_id = 0
  bindingForm.platform_params = ''
  bindingForm.remark = ''
}

function newBinding() {
  resetBindingForm()
}

function editBinding(row: SpecBindingInfo) {
  bindingForm.id = row.id
  bindingForm.direction = row.direction || 'outbound'
  bindingForm.match_type = row.match_type || 'manual'
  bindingForm.status = row.status || 'unmapped'
  bindingForm.priority = row.priority || 0
  bindingForm.confidence = row.confidence || 0
  bindingForm.spec_template_id = row.spec_template_id || 0
  bindingForm.platform_params = stringifyParams(row.platform_params)
  bindingForm.remark = row.remark || ''
}

function stringifyParams(params: SpecBindingInfo['platform_params']): string {
  if (!params) return ''
  if (typeof params === 'string') return params
  return JSON.stringify(params, null, 2)
}

async function loadBindings() {
  if (!bindingSpec.value) return
  bindingLoading.value = true
  try {
    bindings.value = await getSpecBindingList({ product_spec_id: bindingSpec.value.id })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载绑定失败')
  } finally {
    bindingLoading.value = false
  }
}

async function submitBinding() {
  if (!bindingSpec.value) return
  let params: Record<string, unknown> | undefined
  if (bindingForm.platform_params.trim()) {
    try {
      params = JSON.parse(bindingForm.platform_params)
    } catch {
      MessagePlugin.warning('平台参数必须是合法 JSON')
      return
    }
  }
  const payload: SpecBindingUpsertRequest = {
    product_spec_id: bindingSpec.value.id,
    direction: bindingForm.direction,
    match_type: bindingForm.match_type,
    status: bindingForm.status,
    priority: bindingForm.priority,
    confidence: bindingForm.confidence,
    spec_template_id: bindingForm.spec_template_id || 0,
    platform_params: params,
    remark: bindingForm.remark,
  }
  bindingSaving.value = true
  try {
    await upsertSpecBinding(payload)
    MessagePlugin.success('绑定已保存')
    resetBindingForm()
    await loadBindings()
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存绑定失败')
  } finally {
    bindingSaving.value = false
  }
}

async function confirmBinding(row: SpecBindingInfo) {
  try {
    await confirmSpecBinding(row.id, { remark: '人工确认（后台）' })
    MessagePlugin.success('绑定已确认')
    await loadBindings()
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '确认失败')
  }
}

// ---------- 可配置项 ----------

const configOptionsText = ref('')
const configSaving = ref(false)

async function loadConfigOptions() {
  try {
    const groups = await getProductConfigOptions(Number(route.params.id))
    configOptionsText.value = groups && groups.length ? JSON.stringify(groups, null, 2) : ''
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载可配置项失败')
  }
}

async function saveConfigOptions() {
  let groups: SaleProductConfigGroup[] = []
  if (configOptionsText.value.trim()) {
    try {
      const parsed = JSON.parse(configOptionsText.value)
      if (!Array.isArray(parsed)) throw new Error('not array')
      groups = parsed as SaleProductConfigGroup[]
    } catch {
      MessagePlugin.warning('可配置项必须是 JSON 数组（config_groups 结构）')
      return
    }
  }
  configSaving.value = true
  try {
    await saveProductConfigOptions(Number(route.params.id), groups)
    MessagePlugin.success('可配置项已保存')
    await loadConfigOptions()
    await loadHistory()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存可配置项失败')
  } finally {
    configSaving.value = false
  }
}

// ---------- 基础数据 ----------

async function loadSpecs() {
  specs.value = await getProductSpecs(Number(route.params.id))
}

async function loadHistory() {
  history.value = await getProductHistory(Number(route.params.id))
}

async function loadAtomsAndTemplates() {
  try {
    specAtoms.value = (await getSpecAtomList()).filter((atom) => atom.status === 1)
  } catch {
    specAtoms.value = []
  }
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100 })
    templateOptions.value = data.items.map((item) => ({ label: item.name, value: item.id }))
  } catch {
    templateOptions.value = []
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  loading.value = true
  try {
    product.value = await getProductDetail(id)
    await Promise.all([loadSpecs(), loadHistory(), loadConfigOptions()])
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品详情失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/product/products')
}

function goEdit() {
  router.push(`/product/products/${route.params.id}/edit`)
}

onMounted(() => {
  loadCategories()
  loadAtomsAndTemplates()
  loadDetail()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}

.spec-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
  color: #334155;
}

.detail-cover {
  width: 160px;
  height: 106px;
  object-fit: cover;
  border-radius: var(--hs-radius-lg);
  border: 1px solid #edf3ef;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.atom-hints {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.atom-hint {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
}

.atom-hint em {
  color: var(--td-error-color, #d54941);
  font-style: normal;
}

.binding-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.tabs-section :deep(.t-alert) {
  margin-bottom: 12px;
}

.tabs-section > .form-footer {
  margin-top: 12px;
}
</style>
