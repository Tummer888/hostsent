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
          <t-form-item label="平台渠道" name="source_provider_id" :rules="platformRules">
            <t-select
              v-model="form.source_provider_id"
              placeholder="请选择自营平台渠道（魔方云等）"
              :options="platformOptions"
              :loading="platformLoading"
              clearable
              @change="onPlatformChange"
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
            <span class="form-hint">规格模板自带参考售价时以模板为准，此价用于「仅本地/虚拟商品」。</span>
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

        <!-- ===== 平台参数：下拉选真实取值，不再手写 JSON ===== -->
        <t-divider>平台参数（商品级开通默认值）</t-divider>
        <t-alert
          theme="info"
          message="此处是商品级默认参数；下面每个 SKU 的绑定优先级更高。选好平台渠道后可下拉选取平台上真实存在的区域/节点/存储/镜像。"
        />
        <div class="form-grid">
          <t-form-item label="区域 area">
            <t-select
              v-model="platformParams.area"
              clearable
              placeholder="平台区域"
              :options="areaOptions"
              :disabled="!form.source_provider_id"
            />
          </t-form-item>
          <t-form-item label="节点 node">
            <t-select
              v-model="platformParams.node"
              clearable
              placeholder="平台节点"
              :options="nodeOptions"
              :disabled="!form.source_provider_id"
            />
          </t-form-item>
          <t-form-item label="存储 store">
            <t-select
              v-model="platformParams.store"
              clearable
              placeholder="系统盘所在存储（可选）"
              :options="storeOptions"
              :disabled="!form.source_provider_id"
            />
          </t-form-item>
          <t-form-item label="镜像 os">
            <t-select
              v-model="platformParams.os"
              clearable
              placeholder="平台镜像"
              :options="imageOptions"
              :disabled="!form.source_provider_id"
            />
          </t-form-item>
        </div>
        <t-form-item label="其他平台参数（高级，JSON）">
          <t-textarea
            v-model="extraParamsText"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder='上面四个下拉之外的键，如 {"network_type":"normal","ip_num":1,"traffic_quota":0}'
          />
        </t-form-item>

        <!-- ===== 规格配置：建品时直接选模板生成 SKU（自营链路） ===== -->
        <template v-if="mode === 'create'">
          <t-divider>规格配置</t-divider>
          <t-form-item label="配置方式">
            <t-radio-group v-model="specMode" variant="default-filled">
              <t-radio-button value="template">按规格模板生成 SKU（推荐）</t-radio-button>
              <t-radio-button value="later">稍后在详情页配置</t-radio-button>
            </t-radio-group>
          </t-form-item>

          <template v-if="specMode === 'template'">
            <t-alert v-if="!templateRows.length" theme="warning" message="还没有规格模板。请先到「产品管理 → 规格管理 → 规格模板」新建模板并配好平台映射。" />
            <div v-else class="tpl-list">
              <div class="tpl-list__head">
                <span>勾选要生成的规格；每行可直接改 CPU/内存/系统盘/带宽与区域/节点/存储/镜像。</span>
                <t-space size="small">
                  <span class="tpl-list__count">已选 {{ selectedTemplateCount }} / {{ templateRows.length }}</span>
                  <t-link theme="primary" hover="color" @click="selectAllTemplates">全选</t-link>
                  <t-link theme="primary" hover="color" @click="clearAllTemplates">清空</t-link>
                </t-space>
              </div>
              <div v-for="row in templateRows" :key="row.tpl.id" class="tpl-row" :class="{ 'tpl-row--on': row.selected }">
                <div class="tpl-row__head">
                  <t-checkbox v-model="row.selected" @change="onRowToggle(row)">
                    <span class="tpl-row__name">{{ row.tpl.name }}</span>
                  </t-checkbox>
                  <span class="tpl-row__meta">
                    {{ row.tpl.provider_type || '未绑定平台' }}
                    <template v-if="row.tpl.platform_params"> · 已配平台映射</template>
                    <template v-else> · <em class="tpl-row__warn">模板缺少平台映射</em></template>
                    <template v-if="row.options.length"> · {{ row.options.length }} 个可选参数</template>
                  </span>
                </div>
                <div v-if="row.selected" class="tpl-row__body">
                  <div class="tpl-row__fields">
                    <label class="tpl-field">CPU（核）<t-input-number v-model="row.cpu" :min="1" theme="column" /></label>
                    <label class="tpl-field">内存（GB）<t-input-number v-model="row.memoryGb" :min="1" theme="column" /></label>
                    <label class="tpl-field">系统盘（GB）<t-input-number v-model="row.disk" :min="1" theme="column" /></label>
                    <label class="tpl-field">带宽（Mbps）<t-input-number v-model="row.bandwidth" :min="0" theme="column" /></label>
                    <label class="tpl-field">销售价（元）<t-input-number v-model="row.price" :min="0" :precision="2" theme="column" /></label>
                  </div>
                  <div class="tpl-row__fields">
                    <label class="tpl-field">区域 area<t-select v-model="row.area" clearable placeholder="沿用模板" :options="areaOptions" /></label>
                    <label class="tpl-field">节点 node<t-select v-model="row.node" clearable placeholder="沿用模板" :options="nodeOptionsFor(row.area)" /></label>
                    <label class="tpl-field">存储 store<t-select v-model="row.store" clearable placeholder="沿用模板" :options="storeOptionsFor(row.area)" /></label>
                    <label class="tpl-field">镜像 os<t-select v-model="row.os" clearable placeholder="沿用模板" :options="imageOptions" /></label>
                  </div>
                  <!-- 客户可选配置项：勾选=开放给客户自选；不勾=只用基线值 -->
                  <div v-if="row.options.length" class="tpl-options">
                    <div class="tpl-options__head">
                      客户可选配置项（勾选即开放，可设加价；不勾选只按基线值开通）
                    </div>
                    <div v-for="opt in row.options" :key="opt.key" class="tpl-opt">
                      <t-checkbox v-model="opt.enabled">
                        <code>{{ opt.key }}</code>
                      </t-checkbox>
                      <template v-if="opt.enabled">
                        <template v-if="opt.isRange">
                          <label class="tpl-field tpl-field--inline">
                            范围
                            <t-input-number v-model="opt.rangeMin" :min="0" theme="column" />
                            ~
                            <t-input-number v-model="opt.rangeMax" :min="0" theme="column" />
                          </label>
                          <label class="tpl-field tpl-field--inline">
                            默认
                            <t-input-number v-model="opt.defaultNumber" theme="column" />
                          </label>
                        </template>
                        <div v-else class="tpl-opt__values">
                          <div v-for="v in opt.values" :key="v.value" class="tpl-opt__value">
                            <t-radio :checked="v.isDefault" @change="setDefaultValue(opt, v.value)" />
                            <span class="tpl-opt__label">{{ v.group ? v.group + ' / ' : '' }}{{ v.label }}</span>
                            <t-input-number v-model="v.price" :min="0" :precision="2" theme="column" class="tpl-opt__price" />
                            <span class="tpl-opt__unit">元/月</span>
                          </div>
                        </div>
                      </template>
                    </div>
                  </div>
                  <div class="tpl-row__gen">
                    SKU 编码 <code>{{ row.tpl.id ? previewSpecCode(row) : '—' }}</code> · 原子取值
                    <code>{{ JSON.stringify(buildSpecValues(row)) }}</code>
                  </div>
                </div>
              </div>
            </div>
          </template>
          <t-alert v-else theme="info" message="商品建好后，到详情页「规格变体 → 按规格模板生成」逐个生成；未生成 SKU 前无法上架。" />
        </template>

        <t-alert v-else theme="info">
          <template #message>
            规格在商品详情页「规格变体」中维护；本页只改基础信息与平台参数。
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

import { getProviderList, getProviderPlatformResources } from '@/api/admin'
import { getSpecTemplateList } from '@/api/product'
import {
  markupLabel,
  priceModelOptions,
  productStatusOptions,
  productTypeOptions,
} from '@/pages/product/constants'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import type {
  PlatformResourceItem,
  ProviderInfo,
  SaleProductCreateRequest,
  SaleProductInfo,
  SaleProductOptionOverride,
  SaleProductSpecTemplateSelection,
  SpecTemplateInfo,
} from '@/types/interface'

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
    ? '自营商品：选平台渠道 + 勾规格模板，创建后即为可上架商品（规格与平台映射一步到位）。'
    : '自营商品：链路固定为自营，平台参数可在此调整，规格在详情页维护。',
)

const { categoryOptions, loadCategories } = useCategoryOptions()

// ===== 平台渠道与平台资源目录 =====
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

const resources = ref<{
  areas: PlatformResourceItem[]
  nodes: PlatformResourceItem[]
  stores: PlatformResourceItem[]
  images: PlatformResourceItem[]
}>({ areas: [], nodes: [], stores: [], images: [] })

/** 只保留启用项：平台返回 offline 的取值不该再被选进新规格。 */
function toOptions(items: PlatformResourceItem[]): { label: string; value: string }[] {
  return items
    .filter((item) => !item.status || item.status === 'active')
    .map((item) => ({ label: `${item.label}（${item.value}）`, value: item.value }))
}

const areaOptions = computed(() => toOptions(resources.value.areas))
const imageOptions = computed(() => toOptions(resources.value.images))
// 节点/存储挂在区域下：给行级过滤用，行未选区域时用商品级区域的过滤结果。
function filterByArea(items: PlatformResourceItem[], area?: string) {
  if (!area) return items
  return items.filter((item) => !item.parent_id || item.parent_id === area)
}
function nodeOptionsFor(area?: string) {
  return toOptions(filterByArea(resources.value.nodes, area || platformParams.area))
}
function storeOptionsFor(area?: string) {
  return toOptions(filterByArea(resources.value.stores, area || platformParams.area))
}
const nodeOptions = computed(() => nodeOptionsFor())
const storeOptions = computed(() => storeOptionsFor())

async function loadPlatformResources(providerId?: number) {
  const id = Number(providerId)
  resources.value = { areas: [], nodes: [], stores: [], images: [] }
  if (!id) return
  try {
    resources.value = await getProviderPlatformResources(id)
  } catch (error) {
    MessagePlugin.warning((error as Error).message || '该渠道未提供平台资源目录，可用「其他平台参数」手工填写')
  }
}

function onPlatformChange(value?: number | string) {
  const id = Number(value)
  // 换渠道后旧参数多半对新平台无效：清空下拉选择，避免跨平台脏值。
  Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
  void loadPlatformResources(id)
}

// ===== 商品级平台参数（后端字段 config_options，这里用下拉 + 高级 JSON 组合编辑）=====
const platformParams = reactive<{ area?: string; node?: string; store?: string; os?: string }>({
  area: undefined, node: undefined, store: undefined, os: undefined,
})
const extraParamsText = ref('')
const PLATFORM_KEYS = ['area', 'node', 'store', 'os'] as const

/** 把 config_options JSON 拆成「四个下拉 + 高级 JSON」；未知键留在高级里不丢。 */
function splitConfigOptions(raw: string) {
  let parsed: Record<string, unknown> = {}
  if (raw && raw.trim()) {
    try {
      const obj = JSON.parse(raw)
      if (obj && typeof obj === 'object' && !Array.isArray(obj)) parsed = obj as Record<string, unknown>
    } catch {
      // 非法 JSON 原样留在高级文本框里，保存时由后端校验报错，不静默丢弃运营输入
      extraParamsText.value = raw
      Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
      return
    }
  }
  const rest: Record<string, unknown> = { ...parsed }
  for (const key of PLATFORM_KEYS) {
    const v = parsed[key]
    platformParams[key] = v == null || v === '' ? undefined : String(v)
    delete rest[key]
  }
  extraParamsText.value = Object.keys(rest).length ? JSON.stringify(rest, null, 2) : ''
}

/** 把下拉与高级 JSON 合并回 config_options 字符串。 */
function composeConfigOptions(): string | null {
  let extra: Record<string, unknown> = {}
  if (extraParamsText.value.trim()) {
    try {
      const obj = JSON.parse(extraParamsText.value)
      if (!obj || typeof obj !== 'object' || Array.isArray(obj)) {
        MessagePlugin.warning('「其他平台参数」必须是 JSON 对象')
        return null
      }
      extra = obj as Record<string, unknown>
    } catch {
      MessagePlugin.warning('「其他平台参数」JSON 格式非法')
      return null
    }
  }
  const merged: Record<string, unknown> = { ...extra }
  for (const key of PLATFORM_KEYS) {
    if (platformParams[key]) merged[key] = platformParams[key]
  }
  return Object.keys(merged).length ? JSON.stringify(merged) : ''
}

// ===== 规格模板选择（建品即生成 SKU + 客户可选配置项）=====
/** 客户可选配置项：档位勾选的某个参数，及其取值与加价。 */
type OptionRow = {
  key: string
  /** 是否开放给客户选；不开放则只用基线值（默认取第一个取值）。 */
  enabled: boolean
  /** 数量型（带宽/盘/IP 数）用区间表达，没有离散取值。 */
  isRange: boolean
  rangeMin: number
  rangeMax: number
  defaultNumber: number | null
  values: { value: string; label: string; group: string; price: number; isDefault: boolean }[]
}

type TemplateRow = {
  tpl: SpecTemplateInfo
  selected: boolean
  cpu: number
  memoryGb: number
  disk: number
  bandwidth: number
  price: number
  costPrice: number
  area?: string
  node?: string
  store?: string
  os?: string
  /** 客户可选配置项（由档位 option_selections 展开，可改加价与开放与否）。 */
  options: OptionRow[]
  /** 模板里 CPU/内存/磁盘/带宽之外的原子取值（如网络流量），原样保留不丢 */
  extraSpecValues: Record<string, unknown>
  /** 模板里 area/node/store/os 之外的平台参数，原样保留不丢 */
  extraPlatformParams: Record<string, unknown>
}

const specMode = ref<'template' | 'later'>('template')
const templateRows = ref<TemplateRow[]>([])
const selectedTemplateCount = computed(() => templateRows.value.filter((r) => r.selected).length)

/** 由档位 option_selections 展开客户可选配置项（基线取值落到 enabled=false 的行）。 */
function buildOptionRows(tpl: SpecTemplateInfo): OptionRow[] {
  const sel = (tpl.option_selections || {}) as Record<
    string,
    { values?: string[]; range?: number[]; default?: string; group_label?: string }
  >
  const out: OptionRow[] = []
  for (const [key, cfg] of Object.entries(sel)) {
    if (Array.isArray(cfg?.range) && cfg.range.length === 2) {
      const defStr = cfg.default ?? ''
      const defNum = Number(defStr)
      out.push({
        key, enabled: true, isRange: true,
        rangeMin: cfg.range[0], rangeMax: cfg.range[1],
        defaultNumber: defStr !== '' && Number.isFinite(defNum) ? defNum : null,
        values: [],
      })
      continue
    }
    const values = (cfg?.values || []).map((v) => ({
      value: v,
      label: v,
      group: cfg?.group_label || '',
      price: 0,
      isDefault: cfg?.default === v,
    }))
    if (values.length) out.push({ key, enabled: true, isRange: false, rangeMin: 0, rangeMax: 0, defaultNumber: null, values })
  }
  return out
}

function buildRow(tpl: SpecTemplateInfo): TemplateRow {
  const specValues = (tpl.spec_values || {}) as Record<string, unknown>
  const platformParamsOfTpl = (tpl.platform_params || {}) as Record<string, unknown>

  const pickNumber = (keys: string[], fallback: number): number => {
    for (const key of keys) {
      const v = specValues[key]
      if (v != null && v !== '' && Number(v) > 0) return Number(v)
    }
    return fallback
  }
  // 原子字典里内存单位是 MB，模板结构化字段是 GB —— 两边都归一到 GB 展示。
  const memoryMb = pickNumber(['compute.memory'], 0)
  const row: TemplateRow = {
    tpl,
    selected: false,
    cpu: pickNumber(['compute.cpu'], tpl.cpu || 1),
    memoryGb: memoryMb > 0 ? Math.round(memoryMb / 1024) : Math.round(tpl.memory || 1),
    disk: pickNumber(['storage.system.size'], tpl.disk || 40),
    bandwidth: pickNumber(['network.bandwidth'], tpl.bandwidth || 0),
    price: tpl.price || 0,
    costPrice: 0,
    area: platformParamsOfTpl.area != null ? String(platformParamsOfTpl.area) : undefined,
    node: platformParamsOfTpl.node != null ? String(platformParamsOfTpl.node) : undefined,
    store: platformParamsOfTpl.store != null ? String(platformParamsOfTpl.store) : undefined,
    os: platformParamsOfTpl.os != null ? String(platformParamsOfTpl.os) : undefined,
    options: buildOptionRows(tpl),
    extraSpecValues: {},
    extraPlatformParams: {},
  }
  const consumedSpecKeys = new Set(['compute.cpu', 'compute.memory', 'storage.system.size', 'network.bandwidth'])
  for (const [k, v] of Object.entries(specValues)) {
    if (!consumedSpecKeys.has(k) && v != null && v !== '') row.extraSpecValues[k] = v
  }
  for (const [k, v] of Object.entries(platformParamsOfTpl)) {
    if (!(PLATFORM_KEYS as readonly string[]).includes(k) && v != null && v !== '') row.extraPlatformParams[k] = v
  }
  return row
}

/** 组装请求里的 option_overrides（只带开放给客户的参数与其取值加价）。 */
function buildOptionOverrides(row: TemplateRow): SaleProductOptionOverride[] | undefined {
  const out: SaleProductOptionOverride[] = []
  for (const opt of row.options) {
    if (!opt.enabled) continue
    if (opt.isRange) {
      out.push({
        option_key: opt.key,
        min_value: opt.rangeMin,
        max_value: opt.rangeMax,
        default: opt.defaultNumber != null ? String(opt.defaultNumber) : undefined,
        values: [],
      })
      continue
    }
    const values = opt.values.map((v) => ({
      value: v.value,
      label: v.label,
      group_label: v.group || undefined,
      is_default: v.isDefault,
      price_monthly: v.price,
    }))
    out.push({
      option_key: opt.key,
      default: (opt.values.find((v) => v.isDefault) || opt.values[0])?.value,
      values,
    })
  }
  return out.length ? out : undefined
}

/** 单选默认值：同一参数只能有一个默认（TDesign 没有原生 radio，用点击切换）。 */
function setDefaultValue(opt: OptionRow, value: string) {
  for (const v of opt.values) v.isDefault = v.value === value
}

async function loadTemplates() {
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100, status: 1 })
    templateRows.value = data.items.map(buildRow)
  } catch {
    templateRows.value = []
  }
}

/** 勾选时把区域补成商品级区域，省得运营每行都选一遍。 */
function onRowToggle(row: TemplateRow) {
  if (row.selected && !row.area && platformParams.area) row.area = platformParams.area
}

function selectAllTemplates() {
  for (const row of templateRows.value) {
    row.selected = true
    onRowToggle(row)
  }
}
function clearAllTemplates() {
  for (const row of templateRows.value) row.selected = false
}

/** 由行内参数组装原子取值 JSON（内存 GB→MB 与字典口径对齐）。 */
function buildSpecValues(row: TemplateRow): Record<string, unknown> {
  const out: Record<string, unknown> = { ...row.extraSpecValues }
  if (row.cpu > 0) out['compute.cpu'] = row.cpu
  if (row.memoryGb > 0) out['compute.memory'] = row.memoryGb * 1024
  if (row.disk > 0) out['storage.system.size'] = row.disk
  if (row.bandwidth > 0) out['network.bandwidth'] = row.bandwidth
  if (row.area) out['placement.region'] = row.area
  return out
}

function buildPlatformParams(row: TemplateRow): Record<string, unknown> {
  const out: Record<string, unknown> = { ...row.extraPlatformParams }
  if (row.area) out.area = row.area
  if (row.node) out.node = row.node
  if (row.store) out.store = row.store
  if (row.os) out.os = row.os
  return out
}

function previewSpecCode(row: TemplateRow): string {
  // 编码不再带「规格族」前缀（用户要求去掉 通用型/计算型 这类分组）：
  // 用平台类型 + 核/内存/盘 表达，与档位实际取值一致。
  const parts: string[] = []
  if (row.tpl.provider_type) parts.push(row.tpl.provider_type)
  parts.push(`${row.cpu || 1}c${row.memoryGb || 1}g`)
  if (row.disk > 0) parts.push(`${row.disk}g`)
  return parts.join('-')
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
  async (initial) => {
    if (!initial) return
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
    form.stock = initial.stock
    form.sort_order = initial.sort_order
    form.status = initial.status
    splitConfigOptions(initial.config_options || '')
    // 编辑既有商品时把平台资源目录也拉起来，让下拉能回显与改选。
    if (form.source_provider_id) await loadPlatformResources(form.source_provider_id)
  },
  { immediate: true },
)

function handleCancel() {
  emit('cancel')
}

function buildSpecTemplatePayload(): SaleProductSpecTemplateSelection[] | null {
  if (props.mode !== 'create' || specMode.value !== 'template') return []
  const rows = templateRows.value.filter((r) => r.selected)
  if (!rows.length) {
    MessagePlugin.warning('请勾选至少一个规格模板，或改选「稍后在详情页配置」')
    return null
  }
  const payload: SaleProductSpecTemplateSelection[] = []
  for (const row of rows) {
    const platformParamsOfRow = buildPlatformParams(row)
    if (!Object.keys(platformParamsOfRow).length) {
      MessagePlugin.warning(`规格「${row.tpl.name}」没有平台参数（模板未配映射且未就地选择），生成后无法上架`)
      return null
    }
    payload.push({
      spec_template_id: row.tpl.id,
      // 带上预览编码：后端在"请求未给编码"时会按模板结构派生，就地改过的配置会生成
      // 名不副实的编码（如模板 2C4G 改成 4C8G 却叫 general-2c4g-30g）。
      spec_code: previewSpecCode(row),
      name: `${row.tpl.name}（${row.cpu || 1}核${row.memoryGb || 1}G）`,
      spec_values: buildSpecValues(row),
      platform_params: platformParamsOfRow,
      // 客户可选配置项：只带开放给客户的参数（含取值加价），后端按档位目录补齐控件元数据。
      option_overrides: buildOptionOverrides(row),
      price: row.price,
      cost_price: row.costPrice,
      stock: form.stock,
    })
  }
  return payload
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
  const configOptions = composeConfigOptions()
  if (configOptions === null) return null
  const specTemplates = buildSpecTemplatePayload()
  if (specTemplates === null) return null
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
    config_options: configOptions,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    spec_templates: specTemplates.length ? specTemplates : undefined,
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
  const configOptions = composeConfigOptions()
  if (configOptions === null) return
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
    config_options: configOptions,
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
  if (props.mode === 'create') void loadTemplates()
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

.tpl-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}

.tpl-list__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--td-text-color-secondary, #666);
}

.tpl-list__count {
  font-weight: 600;
  color: var(--td-text-color-primary, #333);
}

.tpl-row {
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--td-bg-color-container, #fff);
}

.tpl-row--on {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: 0 0 0 1px rgba(0, 82, 217, 0.12);
}

.tpl-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tpl-row__name {
  font-weight: 600;
}

.tpl-row__meta {
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.tpl-row__warn {
  color: var(--td-warning-color, #e37318);
  font-style: normal;
}

.tpl-row__body {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpl-row__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px 12px;
}

.tpl-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #666);
}

.tpl-row__gen {
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  word-break: break-all;
}

.tpl-row__gen code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  color: var(--td-text-color-primary, #333);
}

.tpl-options {
  border-top: 1px dashed var(--td-component-stroke, #e7e7e7);
  padding-top: 8px;
}

.tpl-options__head {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
  margin-bottom: 6px;
}

.tpl-opt {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}

.tpl-opt code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.tpl-opt__values {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.tpl-opt__value {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.tpl-opt__price {
  width: 110px;
}

.tpl-opt__unit {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.tpl-field--inline {
  flex-direction: row;
  align-items: center;
  gap: 6px;
}
</style>
