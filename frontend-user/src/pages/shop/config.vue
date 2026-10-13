<template>
  <div class="page-body console-module shop-config-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">配置选购</h2>
          <p class="page-header__desc">
            选择规格、计费周期与配置项后加入购物车，统一在购物车结算
          </p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="goBack">返回商品列表</t-button>
        <t-button variant="outline" @click="router.push('/cart')">
          购物车
          <template v-if="cartStore.count > 0" #suffix>({{ cartStore.count }})</template>
        </t-button>
      </div>
    </header>

    <section v-loading="loading" class="config-layout">
      <!-- 左：商品与选配 -->
      <div class="config-main">
        <article v-if="product" class="surface-card config-card">
          <div class="config-card__head">
            <div class="config-card__cover">
              <img v-if="product.cover_image" :src="product.cover_image" :alt="product.name" />
              <component :is="ServerIcon" v-else size="30" />
            </div>
            <div class="config-card__info">
              <h3 class="config-card__name">
                {{ product.name }}
                <t-tag v-if="product.featured" size="small" variant="light" theme="primary">推荐</t-tag>
              </h3>
              <p v-if="descriptionText" class="config-card__desc">{{ descriptionText }}</p>
              <div class="config-card__tags">
                <t-tag size="small" variant="light">{{ sourceModeLabel(product.source_mode) }}</t-tag>
                <t-tag v-if="product.skus.length" size="small" variant="light" theme="primary">
                  {{ product.skus.length }} 种规格
                </t-tag>
              </div>
            </div>
          </div>
        </article>

        <article v-if="product" class="surface-card config-card">
          <!-- 对接商品：上游固定配置只读展示（上游已定规格，改也开不出来） -->
          <div v-if="lockedUpstreamOptions.length" class="config-field">
            <span class="config-label">
              固定配置
              <em class="config-label__hint">上游已定规格，不可修改</em>
            </span>
            <div class="config-fixed">
              <span v-for="opt in lockedUpstreamOptions" :key="opt.key" class="config-fixed__item">
                {{ opt.name }}：<strong>{{ opt.value || '—' }}</strong>
              </span>
            </div>
          </div>

          <div v-if="product.skus.length" class="config-field">
            <span class="config-label">规格</span>
            <t-radio-group v-model="selectedSpec" variant="default-filled" @change="handleSpecChange">
              <t-radio-button
                v-for="s in product.skus"
                :key="s.spec_code"
                :value="s.spec_code"
                :disabled="s.stock === 0"
              >
                {{ s.name }}
                <em class="config-spec-price">¥{{ s.price.toFixed(2) }}</em>
              </t-radio-button>
            </t-radio-group>
            <p v-if="selectedSku?.stock === 0" class="config-warn">该规格已售罄</p>
          </div>

          <div v-if="selectableCycles.length" class="config-field">
            <span class="config-label">计费周期</span>
            <t-radio-group v-model="selectedCycle" variant="default-filled" @change="refreshQuote">
              <t-radio-button v-for="c in selectableCycles" :key="c" :value="c">
                {{ cycleLabel(c) }}
              </t-radio-button>
            </t-radio-group>
            <p v-if="renewNote" class="config-hint config-hint--warn">{{ renewNote }}</p>
          </div>

          <!--
            客户选配项：档位之外的参数（操作系统/网络类型/带宽/数据盘…）。
            控件类型由后端按 widget 下发：group_select 分组下拉（操作系统按 CentOS/Ubuntu 家族）、
            radio 按钮组、qty 步进器、bool 开关、select 普通下拉、text 自由填写。
            每项旁实时显示加价；最终金额以服务端算价为准。
          -->
          <div v-if="allOptions.length" class="config-field">
            <span class="config-label">
              配置选项
              <em class="config-label__hint">不改动即按档位默认配置</em>
            </span>

            <div class="config-options">
              <div v-for="opt in allOptions" :key="opt.option_key" class="config-option">
                <div class="config-option__head">
                  <span class="config-option__name">
                    {{ opt.name }}
                    <em v-if="opt.required" class="config-option__required">必选</em>
                    <em v-if="opt.unit" class="config-option__unit">/{{ opt.unit }}</em>
                  </span>
                  <span v-if="optionSurcharge(opt) > 0" class="config-option__add">
                    +¥{{ optionSurcharge(opt).toFixed(2) }}
                  </span>
                </div>

                <!-- 分组下拉：操作系统按镜像家族分组 -->
                <t-select
                  v-if="optionWidget(opt) === 'group_select'"
                  :model-value="selection[opt.option_key] || undefined"
                  :placeholder="optionPlaceholder(opt)"
                  :clearable="!opt.required"
                  filterable
                  @change="(v: SelectValue) => setSelection(opt, v)"
                >
                  <t-option-group v-for="g in groupedValues(opt)" :key="g.label" :label="g.label" divider>
                    <t-option
                      v-for="it in g.items"
                      :key="it.value"
                      :value="it.value"
                      :label="optionItemLabel(opt, it)"
                    />
                  </t-option-group>
                </t-select>

                <!-- 普通下拉：区域/节点/存储等 -->
                <t-select
                  v-else-if="optionWidget(opt) === 'select'"
                  :model-value="selection[opt.option_key] || undefined"
                  :placeholder="optionPlaceholder(opt)"
                  :clearable="!opt.required"
                  filterable
                  @change="(v: SelectValue) => setSelection(opt, v)"
                >
                  <t-option
                    v-for="it in opt.values"
                    :key="it.value"
                    :value="it.value"
                    :label="optionItemLabel(opt, it)"
                  />
                </t-select>

                <!-- 按钮组：少量离散取值 -->
                <t-radio-group
                  v-else-if="optionWidget(opt) === 'radio'"
                  :model-value="selection[opt.option_key]"
                  variant="default-filled"
                  @change="(v: string | number | boolean) => setSelectionValue(opt, v)"
                >
                  <t-radio-button v-for="it in opt.values" :key="it.value" :value="it.value">
                    {{ it.label }}
                    <em v-if="itemAddPrice(opt, it) > 0" class="config-option__add-inline">
                      +¥{{ itemAddPrice(opt, it).toFixed(2) }}
                    </em>
                  </t-radio-button>
                </t-radio-group>

                <!-- 开关：二值参数 -->
                <t-switch
                  v-else-if="optionWidget(opt) === 'bool'"
                  :model-value="selection[opt.option_key]"
                  :custom-value="boolValues(opt)"
                  @change="(v: SwitchValue) => setSelectionValue(opt, v)"
                />

                <!-- 步进器：超出档位内含量才加价 -->
                <div v-else-if="optionWidget(opt) === 'qty'" class="config-option__qty">
                  <t-input-number
                    :model-value="numberValue(opt)"
                    :min="opt.min_value ?? 0"
                    :max="opt.max_value ?? undefined"
                    :step="1"
                    theme="normal"
                    @change="(v: NumericValue) => setSelectionValue(opt, v ?? '')"
                  />
                  <span v-if="opt.unit" class="config-option__qty-unit">{{ opt.unit }}</span>
                </div>

                <!-- 文本：主机名等自由填写项，留空由系统生成 -->
                <div v-else-if="optionWidget(opt) === 'text'" class="config-option__text">
                  <t-input
                    :model-value="selection[opt.option_key] || ''"
                    :placeholder="optionPlaceholder(opt)"
                    clearable
                    @change="(v: string) => setSelectionValue(opt, v)"
                  />
                  <t-button
                    v-if="opt.option_key === 'hostname'"
                    size="small"
                    variant="outline"
                    @click="randomHostname(opt)"
                  >
                    随机
                  </t-button>
                </div>

                <p v-if="optionHelpText(opt)" class="config-option__help">{{ optionHelpText(opt) }}</p>
              </div>
            </div>

            <p v-if="missingRequired.length" class="config-warn">
              请选择：{{ missingRequired.map((o) => o.name).join('、') }}
            </p>
          </div>

          <div class="config-field">
            <span class="config-label">数量</span>
            <t-input-number v-model="quantity" :min="1" :max="99" theme="normal" @change="refreshQuote" />
          </div>

          <!-- 开通说明：主机名/密码/端口这类「下单时看不见但会被问」的事项讲清楚 -->
          <div v-if="upstreamOrderNotes.length" class="config-field">
            <span class="config-label">开通说明</span>
            <ul class="config-notes">
              <li v-for="note in upstreamOrderNotes" :key="note">{{ note }}</li>
            </ul>
          </div>
        </article>

        <div v-else-if="!loading" class="surface-card">
          <t-empty description="商品不存在或已下架">
            <template #action>
              <t-button theme="primary" @click="router.push('/shop')">返回商品列表</t-button>
            </template>
          </t-empty>
        </div>
      </div>

      <!-- 右：价格明细与操作（结算前先加入购物车，链路统一走购物车） -->
      <aside class="config-side">
        <article class="surface-card config-summary">
          <h3 class="card-title">费用明细</h3>
          <div v-loading="quoteLoading" class="config-prices">
            <div v-if="quote" class="config-rows">
              <!--
                档位价 = 原价 − 选配加价：后端 original_amount 已含选配，若直接叫「商品原价」
                再单列一行加价，读起来像 119 + 60 = 179。拆开写才能与应付金额对上。
              -->
              <div class="config-row">
                <span>档位价{{ selectedSku ? ` · ${selectedSku.name}` : '' }}</span>
                <span>¥{{ baseAmount.toFixed(2) }}</span>
              </div>
              <div v-if="(quote.option_amount ?? 0) > 0" class="config-row config-row--option">
                <span>
                  配置选项加价
                  <em class="config-row__detail">{{ optionSummary }}</em>
                </span>
                <span>+¥{{ (quote.option_amount ?? 0).toFixed(2) }}</span>
              </div>
              <div v-if="quote.discount_amount > 0" class="config-row config-row--discount">
                <span>
                  优惠金额
                  <t-tag size="small" variant="light" theme="success">
                    {{ sourceLabel(quote.discount_source) }}
                  </t-tag>
                </span>
                <span>-¥{{ quote.discount_amount.toFixed(2) }}</span>
              </div>
              <div class="config-row config-row--final">
                <span>应付金额</span>
                <span>¥{{ quote.final_amount.toFixed(2) }}</span>
              </div>
              <p v-if="quantity > 1" class="config-row__note">
                以上为单台价格，共 {{ quantity }} 台，加入购物车后按 {{ quantity }} 台计价
              </p>
            </div>
            <t-empty v-else-if="!quoteLoading" description="价格计算失败，请稍后重试" />
          </div>

          <div class="config-actions">
            <t-button
              theme="primary"
              size="large"
              block
              :loading="adding"
              :disabled="!canSubmit"
              @click="addToCart(true)"
            >
              加入购物车并去结算
            </t-button>
            <t-button
              variant="outline"
              size="large"
              block
              :disabled="!canSubmit"
              @click="addToCart(false)"
            >
              仅加入购物车
            </t-button>
            <p class="config-actions__note">
              下单与支付统一在购物车完成：可一次结算多台/多款，也可稍后再买。
            </p>
          </div>
        </article>
      </aside>
    </section>
  </div>
</template>

<script setup lang="ts">
/**
 * 配置选购页（/shop/:id）。
 *
 * 为什么从弹窗改成独立页：原来「确认购买」是一个弹窗卡片，选配项一多就要在弹窗里
 * 滚动，且无法分享/刷新保留选择；同时「加入购物车」与「立即下单」两条链路混在一个
 * 弹窗里，容易出现「到底是下单了还是只加购了」的困惑。
 *
 * 现在链路唯一且清晰：
 *   商品列表 →（本页）选规格/周期/配置项/数量 → 加入购物车 → 购物车结算
 * 本页不直接下单，下单与支付都发生在购物车，避免两条并行的下单入口。
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { ServerIcon } from 'tdesign-icons-vue-next'

import {
  getProductDetail,
  quoteOrder,
  type OptionItem,
  type ProductInfo,
  type ProductOption,
  type QuoteInfo,
  type SkuInfo,
} from '@/api/shop'
import { useCartStore } from '@/store/modules/cart'
import { htmlToPlainText } from '@/utils/rich-text'

// TDesign 回调参数类型不都在包根导出，按契约就地声明，避免深引内部路径。
type SelectValue = string | number | boolean
type SwitchValue = string | number | boolean
type NumericValue = number | string | undefined

defineOptions({ name: 'ShopConfig' })

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()

const loading = ref(false)
const product = ref<ProductInfo | null>(null)

const selectedSpec = ref('')
const selectedCycle = ref('')
const quantity = ref(1)
const quote = ref<QuoteInfo | null>(null)
const quoteLoading = ref(false)
const adding = ref(false)

/** 选配项选择：平台参数名/上游选项 id → 取值（加入购物车与算价原样带上）。 */
const selection = ref<Record<string, string>>({})

const productId = computed(() => Number(route.params.id))

const allOptions = computed<ProductOption[]>(() =>
  (product.value?.option_groups ?? []).flatMap((g) => g.options),
)

/** 归一控件类型：后端已判好，这里只兜底（老数据可能没有 widget）。 */
function optionWidget(opt: ProductOption): string {
  if (opt.widget) return opt.widget
  if (opt.min_value !== null || opt.max_value !== null) return 'qty'
  return 'select'
}

/** 分组下拉的取值分组；无 group_label 的取值归入「其他」，保证不丢项。 */
const groupedValues = (opt: ProductOption) => {
  const groups: Array<{ label: string; items: OptionItem[] }> = []
  for (const it of opt.values) {
    const label = it.group_label || '其他'
    let group = groups.find((g) => g.label === label)
    if (!group) {
      group = { label, items: [] }
      groups.push(group)
    }
    group.items.push(it)
  }
  return groups
}

/** 开关型的两端取值：后端按 [关, 开] 顺序给枚举值。 */
function boolValues(opt: ProductOption): SwitchValue[] {
  const values = opt.values.map((v) => v.value)
  if (values.length >= 2) return [values[0], values[1]]
  return [0, 1]
}

/** 数量型输入框的回显值：空值时回落默认值，使「未改动」也显示档位内含量。 */
function numberValue(opt: ProductOption): number {
  const raw = selection.value[opt.option_key]
  if (raw !== undefined && raw !== '') {
    const n = Number(raw)
    if (!Number.isNaN(n)) return n
  }
  const def = Number(opt.default_value)
  if (opt.default_value !== '' && !Number.isNaN(def)) return def
  return opt.min_value ?? 0
}

/** 下拉展示名：带加价时拼在名称后，避免选中后看不出多花了多少。 */
function optionItemLabel(opt: ProductOption, it: OptionItem): string {
  const add = itemAddPrice(opt, it)
  return add > 0 ? `${it.label}（+¥${add.toFixed(2)}）` : it.label
}

function optionPlaceholder(opt: ProductOption): string {
  return opt.required ? `请选择${opt.name}` : '按平台默认（可选）'
}

/** 配置项下方的说明：固定说明 + 当前所选取值的含义。 */
function optionHelpText(opt: ProductOption): string {
  const parts: string[] = []
  if (opt.help) parts.push(opt.help)
  const raw = selection.value[opt.option_key]
  if (raw) {
    const hit = opt.values.find((v) => v.value === raw)
    if (hit?.help) parts.push(`${hit.label}：${hit.help}`)
  }
  return parts.join(' ')
}

/** 某取值在当前计费周期下的加价（与后端 priceForCycle 同口径）。 */
function itemAddPrice(opt: ProductOption, it: OptionItem): number {
  const cycle = selectedCycle.value
  if (cycle === 'quarterly' && it.price_quarterly > 0) return it.price_quarterly
  if (cycle === 'annually' && it.price_annually > 0) return it.price_annually
  if (cycle === 'onetime' && it.price_onetime > 0) return it.price_onetime
  void opt
  return it.price_monthly
}

/** 某项当前选择的加价（界面展示用，口径与后端 ResolveOptionPricing 一致）。 */
function optionSurcharge(opt: ProductOption): number {
  const widget = optionWidget(opt)
  if (widget === 'qty') {
    const unit = opt.unit_price ?? 0
    if (unit <= 0) return 0
    const def = opt.default_value !== '' ? Number(opt.default_value) : opt.min_value ?? 0
    const base = Number.isNaN(def) ? opt.min_value ?? 0 : def
    const billable = numberValue(opt) - base
    return billable > 0 ? billable * unit : 0
  }
  const chosen = selection.value[opt.option_key] || opt.default_value
  const item = opt.values.find((v) => v.value === chosen)
  return item ? itemAddPrice(opt, item) : 0
}

/** 默认选择：必选项回填默认值/首个可选值，数量型填默认数量。 */
function applyDefaultSelection() {
  const next: Record<string, string> = {}
  for (const opt of allOptions.value) {
    const widget = optionWidget(opt)
    if (widget === 'qty') {
      next[opt.option_key] = numberValue(opt).toString()
      continue
    }
    if (opt.required) {
      const preferred =
        opt.values.find((v) => v.value === opt.default_value) ??
        opt.values.find((v) => v.is_default) ??
        opt.values[0]
      if (preferred) next[opt.option_key] = preferred.value
      continue
    }
    if (opt.default_value) next[opt.option_key] = opt.default_value
  }
  selection.value = next
}

/** 未选中的必选项（数量型有默认值，不会缺）。 */
const missingRequired = computed<ProductOption[]>(() =>
  allOptions.value.filter((opt) => {
    if (!opt.required || optionWidget(opt) === 'qty') return false
    return !selection.value[opt.option_key]
  }),
)

/** 提交给后端的选配：丢掉空值，避免把「未选」当作显式空值下发。 */
const cleanSelections = computed<Record<string, string>>(() => {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(selection.value)) {
    if (v !== undefined && v !== '') out[k] = String(v)
  }
  return out
})

/** 档位价 = 原价 − 选配加价（后端 original_amount 已含选配）。 */
const baseAmount = computed(() => {
  const original = quote.value?.original_amount ?? 0
  const optionAmount = quote.value?.option_amount ?? 0
  return Math.max(0, Math.round((original - optionAmount) * 100) / 100)
})

/** 已选配置的摘要（「8核 · 带宽 20Mbps」），挂在加价行后说明加价从哪来。 */
const optionSummary = computed(() => {
  const parts = selectionLabels.value.filter((label) => {
    const opt = allOptions.value.find((o) => label.startsWith(o.name))
    return !!opt && optionSurcharge(opt) > 0
  })
  return parts.length ? `（${parts.join(' · ')}）` : ''
})

/** 选配的文字摘要，用于购物车展示（不必回查商品详情）。 */
const selectionLabels = computed<string[]>(() =>
  allOptions.value
    .map((opt) => {
      const raw = selection.value[opt.option_key]
      if (!raw) return ''
      if (optionWidget(opt) === 'qty') return `${opt.name} ${raw}${opt.unit || ''}`
      const item = opt.values.find((v) => v.value === raw)
      return item ? `${opt.name} ${item.label}` : `${opt.name} ${raw}`
    })
    .filter((s) => s !== ''),
)

const selectedSku = computed<SkuInfo | undefined>(() =>
  product.value?.skus.find((s) => s.spec_code === selectedSpec.value),
)

/** 周期来源：选中规格的自有周期优先，否则回落商品级周期。 */
const selectableCycles = computed(() =>
  selectedSku.value?.cycles?.length ? selectedSku.value.cycles : product.value?.cycles ?? [],
)

const descriptionText = computed(() => htmlToPlainText(product.value?.description))

/** 上游固定配置（只读展示）：锁定项表示上游商品已定死，本地不可改。 */
const lockedUpstreamOptions = computed(() =>
  (product.value?.upstream_chain?.options ?? []).filter((o) => o.locked),
)

/** 不可续费提示（上游一次性/免费商品）；可续费时为空。 */
const renewNote = computed(() => {
  const chain = product.value?.upstream_chain
  if (!chain || chain.renewable) return ''
  return chain.renew_note || '该商品不支持续费'
})

/** 开通说明：主机名可自定义 / 密码随机生成 / 端口由上游分配。 */
const upstreamOrderNotes = computed(() => product.value?.upstream_chain?.order_notes ?? [])

const soldOut = computed(() => selectedSku.value?.stock === 0)

const canSubmit = computed(
  () => !!product.value && !!quote.value && !soldOut.value && missingRequired.value.length === 0,
)

/**
 * 随机主机名：与后端 safeHostname 规则一致（字母开头 + 小写字母数字连字符）。
 *
 * 后缀取 8 位而不是 6 位：带 hs- 前缀共 11 位，满足上游「主机名至少 10 位」的校验；
 * 6 位时总长只有 9 位，提交后会被上游以「主机名长度至少10位」拒绝，白扣一次款。
 */
function randomHostname(opt: ProductOption) {
  const suffix = Math.random().toString(36).slice(2, 10)
  setSelectionValue(opt, `hs-${suffix}`)
}

function setSelection(opt: ProductOption, value: SelectValue) {
  setSelectionValue(opt, value)
}

/** 统一写入选择：控件回调可能给字符串/数字/布尔，一律按字符串存。 */
function setSelectionValue(opt: ProductOption, value: SelectValue | NumericValue | null) {
  selection.value = {
    ...selection.value,
    [opt.option_key]: value === null || value === undefined ? '' : String(value),
  }
  refreshQuote()
}

/** 切换规格：周期选项可能收窄，当前周期不在可选内时回落首个。 */
function handleSpecChange() {
  const available = selectableCycles.value
  if (!selectedCycle.value || !available.includes(selectedCycle.value)) {
    selectedCycle.value = available[0] || ''
  }
  refreshQuote()
}

async function refreshQuote() {
  if (!product.value) return
  quoteLoading.value = true
  try {
    const { data } = await quoteOrder({
      productId: product.value.id,
      specCode: selectedSpec.value,
      cycle: selectedCycle.value,
      quantity: quantity.value,
      configSelections: cleanSelections.value,
    })
    quote.value = data
  } catch {
    quote.value = null
  } finally {
    quoteLoading.value = false
  }
}

/**
 * 加入购物车；`goCart` 为 true 时直接去结算页。
 *
 * 两条路径都先进购物车：结算入口唯一，避免「这次是下单还是加购」的歧义，
 * 也让用户能在结算页统一核对多台/多款的最终价格。
 */
function addToCart(goCart: boolean) {
  if (!product.value || !quote.value) return
  if (soldOut.value) {
    MessagePlugin.warning('所选规格已售罄')
    return
  }
  if (missingRequired.value.length) {
    MessagePlugin.warning(`请先选择：${missingRequired.value.map((o) => o.name).join('、')}`)
    return
  }
  adding.value = true
  try {
    const sku = selectedSku.value
    cartStore.add({
      productId: product.value.id,
      productName: product.value.name,
      coverImage: product.value.cover_image,
      specCode: selectedSpec.value,
      specName: sku?.name || '默认规格',
      cycle: selectedCycle.value,
      quantity: quantity.value,
      unitPrice:
        quote.value.final_amount && quantity.value
          ? quote.value.final_amount / quantity.value
          : sku?.price || product.value.price,
      priceModel: product.value.price_model,
      selections: cleanSelections.value,
      selectionLabels: selectionLabels.value,
    })
    MessagePlugin.success('已加入购物车')
    if (goCart) router.push('/cart')
  } finally {
    adding.value = false
  }
}

function cycleLabel(c: string): string {
  const map: Record<string, string> = {
    hourly: '按小时',
    daily: '按天',
    monthly: '按月',
    quarterly: '按季',
    semiannually: '半年',
    annually: '按年',
    biennially: '两年',
    triennially: '三年',
    onetime: '一次性',
  }
  return map[c] || c
}

function sourceModeLabel(m: string): string {
  return m === 'upstream' ? '上游直供' : '自营'
}

/** 折扣来源中文标签（agent=代理等级折扣，是当前唯一折扣来源）。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '代理折扣',
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

function goBack() {
  router.push('/shop')
}

/** 预填来自官网的选购意图（?spec=&cycle=&config=），参数名与取值都是平台口径。 */
function applyIntent() {
  const spec = String(route.query.spec ?? '')
  const cycle = String(route.query.cycle ?? '')
  const preselect = parseIntentConfig(route.query.config)
  if (spec) selectedSpec.value = spec
  if (cycle) selectedCycle.value = cycle
  if (Object.keys(preselect).length) {
    const known = new Set(allOptions.value.map((o) => o.option_key))
    const merged = { ...selection.value }
    for (const [k, v] of Object.entries(preselect)) {
      if (v !== '' && known.has(k)) merged[k] = v
    }
    selection.value = merged
  }
}

function parseIntentConfig(raw: unknown): Record<string, string> {
  const text = String(Array.isArray(raw) ? raw[0] : raw ?? '')
  const out: Record<string, string> = {}
  if (!text) return out
  for (const pair of text.split(',')) {
    const idx = pair.indexOf(':')
    if (idx <= 0) continue
    const key = pair.slice(0, idx).trim()
    const value = pair.slice(idx + 1).trim()
    if (key && value) out[key] = value
  }
  return out
}

onMounted(async () => {
  if (!productId.value || Number.isNaN(productId.value)) {
    MessagePlugin.warning('商品参数不合法')
    router.replace('/shop')
    return
  }
  loading.value = true
  try {
    const { data } = await getProductDetail(productId.value)
    product.value = data
  } catch {
    product.value = null
    MessagePlugin.warning('商品不存在或已下架')
    return
  } finally {
    loading.value = false
  }

  // 默认规格/周期：可与官网带来的意图合并，意图里的值优先。
  const skuList = product.value?.skus ?? []
  const firstAvailable = skuList.find((s) => s.stock !== 0) ?? skuList[0]
  selectedSpec.value = firstAvailable?.spec_code || ''
  const cycles = selectableCycles.value
  selectedCycle.value = cycles[0] || ''

  applyDefaultSelection()
  applyIntent()
  // 意图里的周期可能不在所选规格的可售周期内，回落到首个可售档。
  if (selectedCycle.value && !selectableCycles.value.includes(selectedCycle.value)) {
    selectedCycle.value = selectableCycles.value[0] || ''
  }
  await refreshQuote()
})
</script>

<style scoped>
.config-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: var(--space-lg);
  align-items: start;
}

.config-main {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  min-width: 0;
}

.config-card {
  padding: var(--space-lg);
}

.config-card__head {
  display: flex;
  gap: var(--space-lg);
  padding-bottom: var(--space-lg);
  margin-bottom: var(--space-lg);
  border-bottom: 1px solid var(--color-border);
}

.config-card__cover {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 88px;
  height: 88px;
  flex-shrink: 0;
  border-radius: var(--hs-radius-md, 8px);
  background: linear-gradient(135deg, var(--hs-surface-3) 0%, var(--hs-surface-2) 100%);
  color: var(--color-muted-foreground);
  overflow: hidden;
}

.config-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.config-card__info {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  min-width: 0;
}

.config-card__name {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.config-card__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.config-card__tags {
  display: flex;
  gap: var(--space-xs);
  flex-wrap: wrap;
}

.config-field {
  margin-bottom: var(--space-xl);
}

.config-field:last-child {
  margin-bottom: 0;
}

.config-label {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  margin-bottom: var(--space-sm);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.config-label__hint {
  font-size: 11.5px;
  font-style: normal;
  font-weight: 400;
  color: var(--color-muted-foreground);
}

.config-spec-price {
  margin-left: 6px;
  font-style: normal;
  font-weight: 600;
}

.config-fixed {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
  padding: 10px 12px;
  border-radius: var(--hs-radius-md, 8px);
  background: var(--hs-surface-2, #f5f6f8);
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.config-fixed__item strong {
  color: var(--color-foreground);
}

.config-options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: var(--space-lg);
}

.config-option {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  min-width: 0;
}

.config-option__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.config-option__name {
  font-size: 13px;
  color: var(--color-foreground);
}

.config-option__required {
  margin-left: 6px;
  font-style: normal;
  font-size: 11px;
  color: var(--td-error-color-6);
}

.config-option__unit {
  margin-left: 4px;
  font-style: normal;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.config-option__add {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-warning-color-6, #d97706);
}

.config-option__add-inline {
  margin-left: 4px;
  font-style: normal;
  font-size: 11px;
  color: var(--td-warning-color-6, #d97706);
}

.config-option__qty,
.config-option__text {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.config-option__qty-unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.config-option__help {
  margin: 0;
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.config-notes {
  margin: 0;
  padding-left: 18px;
  font-size: 12.5px;
  line-height: 1.8;
  color: var(--color-muted-foreground);
}

.config-warn {
  margin: var(--space-sm) 0 0;
  font-size: 12px;
  color: var(--td-error-color-6);
}

.config-hint {
  margin: var(--space-sm) 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.config-hint--warn {
  color: var(--td-warning-color-6, #d97706);
}

/* ---------- 右侧价格与操作 ---------- */
.config-side {
  position: sticky;
  top: var(--space-lg);
}

.config-summary {
  padding: var(--space-lg);
}

.config-prices {
  margin: var(--space-md) 0 var(--space-lg);
  min-height: 96px;
}

.config-rows {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.config-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-sm);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.config-row--option {
  color: var(--color-foreground);
}

.config-row--discount {
  color: var(--td-success-color-6, #16a34a);
}

.config-row--final {
  padding-top: var(--space-sm);
  border-top: 1px dashed var(--color-border);
  font-size: 15px;
  font-weight: 700;
  color: var(--color-accent);
}

.config-row--final span:last-child {
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}

.config-row__detail {
  font-style: normal;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.config-row__note {
  margin: 0;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.config-actions {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.config-actions__note {
  margin: 0;
  font-size: 11.5px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}

@media (max-width: 1024px) {
  .config-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .config-side {
    position: static;
  }
}

@media (max-width: 640px) {
  .config-card__head {
    flex-direction: column;
  }

  .config-options {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
