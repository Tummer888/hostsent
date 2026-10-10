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
                <t-button v-if="!isUpstreamChain" size="small" variant="outline" @click="openGenerateDialog">
                  按规格模板生成
                </t-button>
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

        <t-tab-panel value="config" label="可选配置项">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">可选配置项</h3>
              <span class="table-card__meta">
                客户下单时可选的参数；source=self 时按 option_key 直接下发平台
              </span>
            </div>
            <t-alert theme="info" message="建品勾选档位时已自动生成；此处可改取值、分组与加价。「从平台配置项目录添加」可补参数。" />

            <t-space size="small" class="cfg-toolbar">
              <t-button size="small" theme="primary" variant="outline" @click="openCatalogPicker">
                从平台配置项目录添加
              </t-button>
              <t-button size="small" variant="outline" @click="addBlankOption">新增自定义参数</t-button>
              <t-button size="small" variant="outline" @click="advancedVisible = !advancedVisible">
                {{ advancedVisible ? '隐藏' : '显示' }}高级 JSON
              </t-button>
            </t-space>

            <t-table
              row-key="rowKey"
              :data="configOptionRows"
              :columns="configColumns"
              size="small"
              hover
              cell-empty-content="—"
            >
              <template #opt_key="{ row }">
                <div class="product-cell">
                  <span class="cell-strong">{{ row.option_name || '（未命名）' }}</span>
                  <span class="product-sub"><code>{{ row.source_key || row.option_key || '—' }}</code></span>
                </div>
              </template>
              <template #opt_widget="{ row }">
                <t-select
                  v-model="row.widget"
                  size="small"
                  :options="widgetOptions"
                  @change="onWidgetChange(row)"
                />
              </template>
              <template #opt_required="{ row }">
                <t-switch v-model="row.required" size="small" />
              </template>
              <template #opt_default="{ row }">
                <t-input
                  v-model="row.default_value"
                  size="small"
                  placeholder="默认值"
                />
              </template>
              <template #opt_values="{ row }">
                <t-link theme="primary" hover="color" @click="toggleExpand(row)">
                  {{ (row.sub || []).length }} 个取值
                </t-link>
                <span v-if="row.min_value != null" class="product-sub">
                  {{ row.min_value }}~{{ row.max_value }} {{ row.unit }}
                </span>
              </template>
              <template #opt_action="{ row }">
                <div class="action-cell">
                  <t-link theme="primary" hover="color" @click="toggleExpand(row)">
                    {{ expandedKey === row.rowKey ? '收起' : '取值' }}
                  </t-link>
                  <t-link theme="danger" hover="color" @click="removeConfigOption(row)">删除</t-link>
                </div>
              </template>
              <template #empty><t-empty description="暂无选配项，建品时勾选规格档位会自动生成" /></template>
            </t-table>

            <!-- 取值明细：展开行内编辑（值/显示名/分组/加价） -->
            <div v-for="row in configOptionRows" :key="'detail-' + row.rowKey">
              <div v-if="expandedKey === row.rowKey" class="cfg-detail">
                <div class="cfg-detail__head">
                  <span class="cell-strong">{{ row.option_name }} 的取值</span>
                  <t-button size="small" variant="outline" @click="addSub(row)">添加取值</t-button>
                </div>
                <t-table
                  row-key="subKey"
                  :data="row.sub || []"
                  :columns="subColumns"
                  size="small"
                  cell-empty-content="—"
                >
                  <template #sub_label="{ row: sub }">
                    <t-input v-model="sub.option_name" size="small" @change="syncAdvancedFromRows" />
                  </template>
                  <template #sub_value="{ row: sub }">
                    <t-input v-model="sub.source_key" size="small" placeholder="下发值" @change="syncAdvancedFromRows" />
                  </template>
                  <template #sub_group="{ row: sub }">
                    <t-input v-model="sub.group_label" size="small" placeholder="Ubuntu/Windows" @change="syncAdvancedFromRows" />
                  </template>
                  <template #sub_price="{ row: sub }">
                    <t-input-number
                      :model-value="subPricing(sub).monthly ?? 0"
                      size="small"
                      :min="0"
                      :precision="2"
                      theme="column"
                      @change="(v: string | number) => setSubPrice(sub, v)"
                    />
                  </template>
                  <template #sub_default="{ row: sub }">
                    <t-radio :checked="!!sub.is_default" @change="setSubDefault(row, sub)" />
                  </template>
                  <template #sub_action="{ row: sub }">
                    <t-link theme="danger" hover="color" @click="removeSub(row, sub)">删除</t-link>
                  </template>
                </t-table>
              </div>
            </div>

            <t-textarea
              v-if="advancedVisible"
              v-model="configOptionsText"
              class="cfg-advanced"
              :autosize="{ minRows: 8, maxRows: 24 }"
              placeholder='[{"name":"客户选配","options":[{"option_name":"CPU","source":"self","source_key":"cpu","widget":"radio","sub":[{"option_name":"2核","source":"self","source_key":"2"}]}]}]'
              @blur="applyAdvancedJSON"
            />

            <div class="form-footer">
              <t-button variant="outline" @click="loadConfigOptions">重置</t-button>
              <t-button theme="primary" :loading="configSaving" @click="saveConfigOptions">保存可选配置项</t-button>
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
              placeholder="选模板可自动带出规格编码/名称/取值（仍可改）"
              :options="templateOptions"
              @change="onSpecTemplateChange"
            />
            <span class="form-hint">选中模板会自动填好规格编码、名称与原子取值，你只需微调。</span>
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

    <!-- 按规格模板生成 SKU（自营链路打通：模板含平台写参数 → SKU + 已确认绑定） -->
    <t-dialog
      v-model:visible="generateVisible"
      header="按规格模板生成规格"
      width="600px"
      :confirm-btn="{ content: '生成', theme: 'primary', loading: generateSaving }"
      @confirm="submitGenerate"
    >
      <t-form label-align="top" :data="generateForm">
        <t-form-item label="规格模板" required>
          <t-select
            v-model="generateForm.spec_template_id"
            placeholder="请选择已配置平台参数的规格模板"
            :options="templateOptions"
            @change="onTemplateChange"
          />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="规格编码（留空自动生成）">
            <t-input v-model="generateForm.spec_code" placeholder="如 general-2c4g-50g" />
          </t-form-item>
          <t-form-item label="规格名称（留空取模板名）">
            <t-input v-model="generateForm.name" placeholder="如 通用型 2核4G" />
          </t-form-item>
          <t-form-item label="销售价（元）">
            <t-input-number v-model="generateForm.price" :min="0" :precision="2" theme="column" placeholder="0 = 取模板参考售价" />
          </t-form-item>
          <t-form-item label="成本价（元）">
            <t-input-number v-model="generateForm.cost_price" :min="0" :precision="2" theme="column" />
          </t-form-item>
        </div>
        <t-form-item label="平台参数（可覆盖模板值）">
          <t-textarea
            v-model="generateForm.platform_params"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder='留空即用模板参数，如 {"area":"1","node":"2","os":"12"}'
          />
        </t-form-item>
        <t-alert v-if="selectedTemplateHint" theme="info" :message="selectedTemplateHint" />
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

    <!-- 平台配置项目录选择器：从目录添加可选配置项 -->
    <t-dialog
      v-model:visible="pickerVisible"
      header="从平台配置项目录添加"
      width="720px"
      :footer="false"
      @close="pickerVisible = false"
    >
      <t-alert theme="info" message="目录来自「产品管理 → 规格管理 → 平台配置项」，按商品绑定的平台渠道加载。" />
      <t-table
        row-key="option_key"
        :data="pickerItems"
        :loading="pickerLoading"
        :columns="pickerColumns"
        size="small"
        hover
        max-height="420"
        cell-empty-content="—"
      >
        <template #p_option="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.label || row.option_key }}</span>
            <span class="product-sub">
              <code>{{ row.option_key }}</code>
              <t-tag v-if="row.required" theme="error" variant="light" size="small" shape="round">必选</t-tag>
            </span>
          </div>
        </template>
        <template #p_values="{ row }">
          <span>{{ (row.values || []).length }} 个取值</span>
        </template>
        <template #p_action="{ row }">
          <t-link theme="primary" hover="color" @click="addFromCatalog(row)">添加</t-link>
        </template>
        <template #empty><t-empty description="目录为空，请先到「平台配置项」页同步" /></template>
      </t-table>
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
  generateProductSpecFromTemplate,
  getOptionCatalog,
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
import { getProviderList } from '@/api/admin'
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
  OptionSpecInfo,
  ProviderInfo,
  SaleProductConfigGroup,
  SaleProductConfigOption,
  SaleProductConfigSub,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductSpecInfo,
  SaleProductSpecRequest,
  SpecAtomInfo,
  SpecBindingInfo,
  SpecBindingUpsertRequest,
  SpecTemplateInfo,
} from '@/types/interface'

defineOptions({ name: 'ProductProductsDetail' })

const route = useRoute()
const router = useRouter()

const product = ref<SaleProductInfo | null>(null)
const specs = ref<SaleProductSpecInfo[]>([])
const history = ref<SaleProductHistoryInfo[]>([])
const specAtoms = ref<SpecAtomInfo[]>([])
const templateOptions = ref<{ label: string; value: number }[]>([])
// 模板详情（选模板时带出规格参数用）：只用来回填表单，不作为落库依据。
const specTemplateDetails = ref<SpecTemplateInfo[]>([])
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

/**
 * 选中规格模板时自动带出规格编码/名称/原子取值（可改）——此前模板下拉只是个"引用标记"，
 * 选了之后下面还要手写 JSON，模板等于白配。
 */
function onSpecTemplateChange(value?: number | string) {
  const id = Number(value)
  if (!id) return
  const tpl = specTemplateDetails.value.find((item) => item.id === id)
  if (!tpl) return
  if (!specForm.spec_code.trim()) specForm.spec_code = deriveSpecCodeFromTemplate(tpl)
  if (!specForm.name.trim()) specForm.name = tpl.name
  if (!specForm.price && tpl.price > 0) specForm.price = tpl.price
  if (!specForm.specs.trim()) specForm.specs = JSON.stringify(deriveSpecValuesFromTemplate(tpl), null, 2)
  MessagePlugin.info(`已按模板「${tpl.name}」带出规格参数，可按需修改`)
}

/** 由模板派生 SKU 编码：规格族-核数/内存/磁盘（与后端 deriveSpecCode 同口径）。 */
function deriveSpecCodeFromTemplate(tpl: SpecTemplateInfo): string {
  const parts: string[] = []
  if (tpl.spec_family) parts.push(tpl.spec_family)
  parts.push(`${tpl.cpu || 1}c${Math.round(tpl.memory || 1)}g`)
  if (tpl.disk > 0) parts.push(`${tpl.disk}g`)
  return parts.join('-')
}

/** 模板未填 spec_values 时按结构化字段推导原子取值（内存 GB→MB，与字典口径一致）。 */
function deriveSpecValuesFromTemplate(tpl: SpecTemplateInfo): Record<string, unknown> {
  if (tpl.spec_values && Object.keys(tpl.spec_values).length) return tpl.spec_values
  const out: Record<string, unknown> = {}
  if (tpl.cpu > 0) out['compute.cpu'] = tpl.cpu
  if (tpl.memory > 0) out['compute.memory'] = Math.round(tpl.memory * 1024)
  if (tpl.disk > 0) out['storage.system.size'] = tpl.disk
  if (tpl.bandwidth > 0) out['network.bandwidth'] = tpl.bandwidth
  if (tpl.disk_type) out['storage.system.type'] = tpl.disk_type
  if (tpl.os) out['image.os'] = tpl.os
  return out
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

// ---------- 按规格模板生成 SKU（自营链路打通） ----------

const generateVisible = ref(false)
const generateSaving = ref(false)
const generateForm = reactive<{
  spec_template_id: number | undefined
  spec_code: string
  name: string
  price: number
  cost_price: number
  platform_params: string
}>({ spec_template_id: undefined, spec_code: '', name: '', price: 0, cost_price: 0, platform_params: '' })
// 模板列表（生成下拉用）：带平台参数与参考售价，便于选择时给出提示。
const generateTemplates = ref<SpecTemplateInfo[]>([])

const selectedTemplateHint = computed(() => {
  const tpl = generateTemplates.value.find((item) => item.id === generateForm.spec_template_id)
  if (!tpl) return ''
  const platform = tpl.platform_params ? JSON.stringify(tpl.platform_params) : '未配置'
  return `模板「${tpl.name}」：原子取值 ${tpl.spec_values ? JSON.stringify(tpl.spec_values) : '按 CPU/内存/磁盘推导'}；平台参数 ${platform}。`
})

async function openGenerateDialog() {
  generateForm.spec_template_id = undefined
  generateForm.spec_code = ''
  generateForm.name = ''
  generateForm.price = 0
  generateForm.cost_price = 0
  generateForm.platform_params = ''
  generateVisible.value = true
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100, status: 1 })
    generateTemplates.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载规格模板失败')
  }
}

function onTemplateChange(value?: number | string) {
  const tpl = generateTemplates.value.find((item) => item.id === Number(value))
  if (!tpl) return
  if (!generateForm.name) generateForm.name = tpl.name
  if (!generateForm.price && tpl.price > 0) generateForm.price = tpl.price
}

async function submitGenerate() {
  if (!generateForm.spec_template_id) {
    MessagePlugin.warning('请选择规格模板')
    return
  }
  let platformParams: Record<string, unknown> | undefined
  if (generateForm.platform_params.trim()) {
    try {
      platformParams = JSON.parse(generateForm.platform_params)
    } catch {
      MessagePlugin.warning('平台参数必须是合法 JSON')
      return
    }
  }
  generateSaving.value = true
  try {
    const result = await generateProductSpecFromTemplate(Number(route.params.id), {
      spec_template_id: generateForm.spec_template_id,
      spec_code: generateForm.spec_code.trim() || undefined,
      name: generateForm.name.trim() || undefined,
      price: generateForm.price,
      cost_price: generateForm.cost_price,
      platform_params: platformParams,
    })
    if (result.notice) {
      MessagePlugin.warning(result.notice)
    } else {
      MessagePlugin.success('规格已生成并完成平台绑定')
    }
    generateVisible.value = false
    await loadSpecs()
    await loadHistory()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '生成规格失败')
  } finally {
    generateSaving.value = false
  }
}

// ---------- 可选配置项（结构化编辑器）----------
//
// 后端仍以 config_groups 数组收发（与上游克隆链路共用），但这里给运营的是表格：
// 每个参数一行，展开行内改取值/分组/加价。JSON 文本框降级为「高级」入口，
// 两边通过 rowsToGroups / groupsToRows 双向同步，避免运营手写整段 JSON。

type ConfigSubRow = SaleProductConfigSub & { subKey: string; pricings?: { monthly?: number }[] }
// 用 Omit 换掉 sub 的类型：表格行需要 subKey 做 row-key，直接用交叉会与基类型里的 sub 冲突。
type ConfigOptionRow = Omit<SaleProductConfigOption, 'sub'> & { rowKey: string; sub: ConfigSubRow[] }

const configOptionsText = ref('')
const configSaving = ref(false)
const advancedVisible = ref(false)
const expandedKey = ref('')
const configOptionRows = ref<ConfigOptionRow[]>([])

let rowSeq = 0
function nextKey(prefix: string): string {
  rowSeq += 1
  return `${prefix}-${rowSeq}`
}

const widgetOptions = [
  { label: '下拉（可分组）', value: 'select' },
  { label: '单选（按钮组）', value: 'radio' },
  { label: '数量（步进器）', value: 'qty' },
  { label: '开关', value: 'bool' },
]

const configColumns: PrimaryTableCol<ConfigOptionRow>[] = [
  { colKey: 'opt_key', title: '参数', minWidth: 180 },
  { colKey: 'opt_widget', title: '控件', width: 150 },
  { colKey: 'opt_required', title: '必选', width: 70 },
  { colKey: 'opt_default', title: '默认值', width: 130 },
  { colKey: 'opt_values', title: '取值', minWidth: 130 },
  { colKey: 'opt_action', title: '操作', width: 130, align: 'center' as const },
]

const subColumns: PrimaryTableCol<ConfigSubRow>[] = [
  { colKey: 'sub_label', title: '显示名', minWidth: 160 },
  { colKey: 'sub_value', title: '下发值', width: 120 },
  { colKey: 'sub_group', title: '分组', width: 130 },
  { colKey: 'sub_price', title: '加价(元/月)', width: 140 },
  { colKey: 'sub_default', title: '默认', width: 70 },
  { colKey: 'sub_action', title: '操作', width: 70, align: 'center' as const },
]

function groupsToRows(groups: SaleProductConfigGroup[]): ConfigOptionRow[] {
  const out: ConfigOptionRow[] = []
  for (const g of groups || []) {
    for (const opt of g.options || []) {
      out.push({
        ...opt,
        rowKey: nextKey('opt'),
        sub: (opt.sub || []).map((s) => ({ ...s, subKey: nextKey('sub') })),
      })
    }
  }
  return out
}

function subPricing(sub: ConfigSubRow): { monthly?: number } {
  const p = (sub.pricings || [])[0]
  return p || {}
}

function setSubPrice(sub: ConfigSubRow, value: string | number) {
  const monthly = Number(value) || 0
  sub.pricings = [{ monthly }]
  syncAdvancedFromRows()
}

function setSubDefault(row: ConfigOptionRow, target: ConfigSubRow) {
  // 每参数只保留一个默认（用户侧预选靠它）。
  for (const sub of row.sub) sub.is_default = sub.subKey === target.subKey
  const hit = row.sub.find((s) => s.subKey === target.subKey)
  row.default_value = hit ? (hit.source_key || '') : ''
  syncAdvancedFromRows()
}

function addSub(row: ConfigOptionRow) {
  row.sub = row.sub || []
  row.sub.push({ option_name: '', source_key: '', source: 'self', group_label: '', hidden: 0, subKey: nextKey('sub') })
  expandedKey.value = row.rowKey
  syncAdvancedFromRows()
}

function removeSub(row: ConfigOptionRow, target: ConfigSubRow) {
  row.sub = (row.sub || []).filter((s) => s.subKey !== target.subKey)
  syncAdvancedFromRows()
}

function addBlankOption() {
  configOptionRows.value.push({
    rowKey: nextKey('opt'),
    option_name: '',
    option_type: 2,
    source: 'self',
    source_key: '',
    option_key: '',
    widget: 'radio',
    required: false,
    default_value: '',
    hidden: 0,
    sort_order: configOptionRows.value.length,
    sub: [],
  })
}

function removeConfigOption(row: ConfigOptionRow) {
  configOptionRows.value = configOptionRows.value.filter((r) => r.rowKey !== row.rowKey)
  syncAdvancedFromRows()
}

/** 先选控件再定 option_type：用户侧按 widget 渲染，option_type 只作兼容口径。 */
function onWidgetChange(row: ConfigOptionRow) {
  const map: Record<string, number> = { select: 1, radio: 2, bool: 3, qty: 4 }
  row.option_type = map[row.widget || 'select'] ?? 1
  syncAdvancedFromRows()
}

function toggleExpand(row: ConfigOptionRow) {
  expandedKey.value = expandedKey.value === row.rowKey ? '' : row.rowKey
}

/** 把表格行回写成 config_groups（保存与高级 JSON 同步共用）。 */
function rowsToGroups(): SaleProductConfigGroup[] {
  const options: SaleProductConfigOption[] = configOptionRows.value.map((row, idx) => ({
    option_name: row.option_name,
    option_type: row.option_type ?? 1,
    source: 'self',
    source_key: row.source_key || row.option_key || '',
    provider_type: row.provider_type,
    option_key: row.option_key || row.source_key || '',
    widget: row.widget || 'select',
    required: !!row.required,
    default_value: row.default_value || '',
    widget_group: row.widget_group,
    min_value: row.min_value ?? null,
    max_value: row.max_value ?? null,
    unit: row.unit,
    help: row.help,
    hidden: row.hidden ?? 0,
    sort_order: row.sort_order ?? idx,
    sub: (row.sub || []).map((s, sIdx) => ({
      option_name: s.option_name,
      source: 'self',
      source_key: s.source_key,
      group_label: s.group_label,
      is_default: !!s.is_default,
      hidden: s.hidden ?? 0,
      sort_order: sIdx,
      pricings: [{ monthly: subPricing(s).monthly ?? 0 }],
    })),
  }))
  if (!options.length) return []
  return [{ name: '客户选配', description: '按配置档生成 / 手工维护', options }]
}

function syncAdvancedFromRows() {
  const groups = rowsToGroups()
  configOptionsText.value = groups.length ? JSON.stringify(groups, null, 2) : ''
}

/** 高级 JSON 编辑后回灌表格（运营手改 JSON 时表格要跟着变）。 */
function applyAdvancedJSON() {
  const raw = configOptionsText.value.trim()
  if (!raw) {
    configOptionRows.value = []
    return
  }
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) throw new Error('not array')
    configOptionRows.value = groupsToRows(parsed as SaleProductConfigGroup[])
    MessagePlugin.success('已按 JSON 刷新表格')
  } catch {
    MessagePlugin.warning('JSON 结构非法（应为 config_groups 数组），表格保持原样')
  }
}

async function loadConfigOptions() {
  try {
    const groups = await getProductConfigOptions(Number(route.params.id))
    configOptionRows.value = groupsToRows(groups || [])
    syncAdvancedFromRows()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载可选配置项失败')
  }
}

async function saveConfigOptions() {
  const groups = rowsToGroups()
  configSaving.value = true
  try {
    await saveProductConfigOptions(Number(route.params.id), groups)
    MessagePlugin.success('可选配置项已保存')
    await loadConfigOptions()
    await loadHistory()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存可选配置项失败')
  } finally {
    configSaving.value = false
  }
}

// ---------- 从平台配置项目录添加参数 ----------

const pickerVisible = ref(false)
const pickerLoading = ref(false)
const pickerItems = ref<OptionSpecInfo[]>([])

const pickerColumns: PrimaryTableCol<OptionSpecInfo>[] = [
  { colKey: 'p_option', title: '配置项', minWidth: 200 },
  { colKey: 'p_values', title: '可选值', width: 110 },
  { colKey: 'unit', title: '单位', width: 80 },
  { colKey: 'p_action', title: '操作', width: 80, align: 'center' as const },
]

async function openCatalogPicker() {
  pickerVisible.value = true
  pickerLoading.value = true
  try {
    // 自营商品用商品绑定的平台渠道反查平台类型；未绑定则列全部平台类型（按 mofangyun 兜底）。
    let providerType = ''
    if (product.value?.source_provider_id) {
      const data = await getProviderList({ page_size: 100 })
      const hit = data.items.find((p: ProviderInfo) => p.id === product.value?.source_provider_id)
      providerType = hit?.provider_type || ''
    }
    pickerItems.value = await getOptionCatalog({
      provider_type: providerType || 'mofangyun',
      provider_id: product.value?.source_provider_id,
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载平台配置项目录失败')
    pickerItems.value = []
  } finally {
    pickerLoading.value = false
  }
}

function addFromCatalog(spec: OptionSpecInfo) {
  if (configOptionRows.value.some((r) => (r.option_key || r.source_key) === spec.option_key)) {
    MessagePlugin.warning(`已存在参数 ${spec.option_key}`)
    return
  }
  const isQty = spec.widget === 'qty' || spec.min_value != null
  configOptionRows.value.push({
    rowKey: nextKey('opt'),
    option_name: spec.label || spec.option_key,
    option_type: isQty ? 4 : spec.widget === 'radio' ? 2 : 1,
    source: 'self',
    source_key: spec.option_key,
    provider_type: spec.provider_type,
    option_key: spec.option_key,
    widget: isQty ? 'qty' : (spec.widget || 'select'),
    required: !!spec.required,
    default_value: spec.default_value || '',
    widget_group: spec.group_name,
    min_value: spec.min_value ?? null,
    max_value: spec.max_value ?? null,
    unit: spec.unit,
    help: spec.help,
    hidden: 0,
    sort_order: spec.sort_order ?? configOptionRows.value.length,
    sub: (spec.values || []).map((v) => ({
      option_name: v.label || v.value,
      source: 'self',
      source_key: v.value,
      group_label: v.group_label || '',
      is_default: false,
      hidden: v.status === 'offline' ? 1 : 0,
      pricings: [{ monthly: 0 }],
      subKey: nextKey('sub'),
    })),
  })
  syncAdvancedFromRows()
  MessagePlugin.success(`已添加参数 ${spec.label || spec.option_key}`)
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
    // 只列启用中的模板：停用模板不该再被选来生成新规格。
    specTemplateDetails.value = data.items.filter((item) => item.status === 1)
    templateOptions.value = specTemplateDetails.value.map((item) => ({ label: item.name, value: item.id }))
  } catch {
    specTemplateDetails.value = []
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
