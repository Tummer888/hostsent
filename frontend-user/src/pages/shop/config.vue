<template>
  <div class="page-body console-module shop-config-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">配置选购</h2>
          <p class="page-header__desc">选择规格、计费周期与配置项后加入购物车，统一在购物车结算</p>
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

    <!--
      加载态用骨架屏而不是 v-loading 的转圈遮罩：
      该页首屏内容是固定的几块结构（商品卡 + 配置卡 + 费用明细），骨架屏直接预示
      「这里会有什么」，比一个盖住整页的旋转圈更稳、也不闪；转圈还会随内容高度
      跳动，页面看起来在抖。
    -->
    <section v-if="loading" class="config-layout config-layout--skeleton" aria-busy="true">
      <div class="config-main">
        <article class="surface-card config-card">
          <t-skeleton theme="avatar-text" :row-col="skeletonProductRows" />
        </article>
        <article class="surface-card config-card">
          <t-skeleton :row-col="skeletonOptionRows" />
        </article>
      </div>
      <aside class="config-side">
        <article class="surface-card config-summary">
          <t-skeleton :row-col="skeletonPriceRows" />
        </article>
      </aside>
    </section>

    <section v-else class="config-layout">
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
                <!-- 不显示供货渠道（自营/转售），属内部信息，见 shop/index.vue 同处说明。 -->
                <t-tag v-if="product.skus.length" size="small" variant="light" theme="primary">
                  {{ product.skus.length }} 种规格
                </t-tag>
              </div>
            </div>
          </div>
        </article>

        <article v-if="product" class="surface-card config-card">
          <!-- 固定配置：整机锁定项只读展示（这些配置平台侧定死，改也开不出来） -->
          <div v-if="lockedUpstreamOptions.length" class="config-field">
            <span class="config-label">
              固定配置
              <em class="config-label__hint">平台已定规格，不可修改</em>
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

                <!--
                  数量型：滑块 + 数值输入（上游按量计价，如系统盘 80–1024G、带宽 75–500M）。
                  改成滑块是因为这类参数是"连续区间里的一个值"（不是几个候选档），
                  拉一下就能看到加价怎么变，不用先猜个数字再看价。
                  输入框保留：填精确值 / 键盘操作仍需要。
                -->
                <div v-else-if="optionWidget(opt) === 'qty'" class="config-option__qty">
                  <t-slider
                    class="config-option__slider"
                    :model-value="numberValue(opt)"
                    :min="opt.min_value ?? 0"
                    :max="opt.max_value ?? undefined"
                    :step="1"
                    :label="false"
                    @change="(v: SliderValue) => setSelectionValue(opt, v)"
                  />
                  <div class="config-option__qty-input">
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
                  <span v-if="qtyRangeHint(opt)" class="config-option__qty-range">{{ qtyRangeHint(opt) }}</span>
                </div>

                <!-- 文本：主机名/登录密码等自由填写项；密码用遮蔽输入，避免明文示人 -->
                <div v-else-if="optionWidget(opt) === 'text'" class="config-option__text">
                  <t-input
                    :model-value="selection[opt.option_key] || ''"
                    :type="isSecretOption(opt) ? 'password' : 'text'"
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
                    换一个
                  </t-button>
                </div>

                <p v-if="optionHelpText(opt)" class="config-option__help">{{ optionHelpText(opt) }}</p>
              </div>
            </div>

            <p v-if="missingRequired.length" class="config-warn">
              请选择：{{ missingRequired.map((o) => o.name).join('、') }}
            </p>
            <!-- 商品级售罄（无 SKU 的上游限量商品）：与 SKU 售罄同一提示样式 -->
            <p v-if="!selectedSku && product?.stock === 0" class="config-warn">该商品已售罄，暂时无法购买</p>
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

        <!-- 加载失败/已下架：此时已不在 loading 分支内，直接给空态 -->
        <div v-else class="surface-card">
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
          <!--
            重算价不用转圈遮罩：改周期/选配后价格本来就是「同一块区域换数字」，
            盖一层转圈会让整块明细闪一下。改为在原位给一个轻微的不透明提示，
            内容保持可见，用户能直接对比新旧价格。
          -->
          <div class="config-prices" :class="{ 'is-refreshing': quoteLoading }">
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
            <div v-else-if="quoteLoading" class="config-prices__hint">正在计算价格…</div>
            <t-empty v-else description="价格计算失败，请稍后重试" />
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
            <t-button variant="outline" size="large" block :disabled="!canSubmit" @click="addToCart(false)">
              仅加入购物车
            </t-button>
            <p class="config-actions__note">下单与支付统一在购物车完成：可一次结算多台/多款，也可稍后再买。</p>
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
// 滑块 change 回调：单选模式给数字，双滑块给数组（这里只用单选模式）。
type SliderValue = number | number[]

defineOptions({ name: 'ShopConfig' })

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()

const loading = ref(false)
const product = ref<ProductInfo | null>(null)

/**
 * 首屏骨架的行结构：形状对齐真实内容，加载完成时视觉上不会"跳一下"。
 * 金额列留窄条、配置行留两列，与右侧费用明细的排布大致对应。
 */
const skeletonProductRows = [
  [{ type: 'rect', width: '56px', height: '56px' }],
  [
    { type: 'text', width: '60%' },
    { type: 'text', width: '160px' },
  ],
]
const skeletonOptionRows = [
  [{ type: 'text', width: '30%' }],
  [{ type: 'rect', width: '100%', height: '32px' }],
  [{ type: 'text', width: '30%' }],
  [{ type: 'rect', width: '100%', height: '32px' }],
  [{ type: 'text', width: '30%' }],
  [{ type: 'rect', width: '100%', height: '32px' }],
]
const skeletonPriceRows = [
  [
    { type: 'text', width: '40%' },
    { type: 'text', width: '30%' },
  ],
  [
    { type: 'text', width: '40%' },
    { type: 'text', width: '30%' },
  ],
  [
    { type: 'text', width: '40%' },
    { type: 'text', width: '30%' },
  ],
  [{ type: 'rect', width: '100%', height: '40px' }],
]

const selectedSpec = ref('')
const selectedCycle = ref('')
const quantity = ref(1)
const quote = ref<QuoteInfo | null>(null)
const quoteLoading = ref(false)
const adding = ref(false)

/** 选配项选择：平台参数名/上游选项 id → 取值（加入购物车与算价原样带上）。 */
const selection = ref<Record<string, string>>({})

const productId = computed(() => Number(route.params.id))

/**
 * 商品下所有可选项（当前后端只给一个分组，展平后逐项渲染）。
 *
 * values 兜底成空数组：文本型项（主机名）没有候选取值，后端会下发 null；
 * 而模板对每个选项都会求值加价摘要（内部 opt.values.find），漏兜底会让整个
 * 组件渲染抛异常 —— 表现就是页面一直转圈、什么都不出来。
 */
const allOptions = computed<ProductOption[]>(() =>
  (product.value?.option_groups ?? [])
    .flatMap((g) => g.options)
    .map((o) => ({ ...o, values: Array.isArray(o.values) ? o.values : [] })),
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
    // 单位加价必须按当前周期取：上游每个周期各有单价（系统盘 0.8/月、2.4/季、9.6/年），
    // 只按月度算会让年付订单的预览价远低于实付（用户反馈的"加配硬盘不随年付叠加"）。
    const unit = qtyUnitPrice(opt)
    if (unit <= 0) return 0
    const def = opt.default_value !== '' ? Number(opt.default_value) : (opt.min_value ?? 0)
    const base = Number.isNaN(def) ? (opt.min_value ?? 0) : def
    const billable = numberValue(opt) - base
    return billable > 0 ? billable * unit : 0
  }
  const chosen = selection.value[opt.option_key] || opt.default_value
  const item = opt.values.find((v) => v.value === chosen)
  return item ? itemAddPrice(opt, item) : 0
}

/**
 * 数量型选项在当前周期的每单位加价。
 * 该周期没有独立单价时回落月度价（与后端 quantityUnitPriceForCycle 同口径），
 * 保证"前端估算 = 后端实付"，避免预览与下单金额不一致。
 */
function qtyUnitPrice(opt: ProductOption): number {
  const cycle = selectedCycle.value
  if (cycle === 'quarterly') return opt.unit_price_quarterly || opt.unit_price || 0
  if (cycle === 'annually') return opt.unit_price_annually || opt.unit_price || 0
  if (cycle === 'onetime') return opt.unit_price_onetime || opt.unit_price || 0
  return opt.unit_price || 0
}

/**
 * 默认选择：与后台代下单弹窗同一口径，开页即把生效值显示出来。
 *
 * 为什么不留空：
 *   1) 履约侧对"没选的项"取首个可见值（上游 pickConfigSub、自营 pickEnumValue 都是），
 *      留空并不等于"不配置"，只等于用户看不见会开出什么 —— 上游商品的网络类型/
 *      操作系统候选值都没有声明默认，留空时下拉是空白的，用户得逐个点开才知道默认值。
 *   2) 数量型回填档位内含量，文本型（主机名）随机填一个：主机名留空时后端会按商品名
 *      兜底生成，用户既看不到也记不住；先给一个可改的值，比让用户面对空白更省事。
 * 用户看到不喜欢的值可以直接切换，这比从空白里"想起来要填什么"快得多。
 */
function applyDefaultSelection() {
  const next: Record<string, string> = {}
  for (const opt of allOptions.value) {
    const widget = optionWidget(opt)
    if (widget === 'qty') {
      next[opt.option_key] = numberValue(opt).toString()
      continue
    }
    if (widget === 'text') {
      if (opt.option_key === 'hostname') next[opt.option_key] = randomHostnameValue()
      continue
    }
    // 优先运营声明的默认值，其次首个可见值（与后端履约口径一致）。
    const preferred =
      opt.values.find((v) => v.value === opt.default_value) ?? opt.values.find((v) => v.is_default) ?? opt.values[0]
    if (preferred) next[opt.option_key] = preferred.value
  }
  selection.value = next
}

/**
 * 是否为敏感项（密码类）：渲染成遮蔽输入框。
 * 密码明文显示在屏幕上（并可能被旁人/录屏看到），对客户没有任何好处。
 */
function isSecretOption(opt: ProductOption): boolean {
  return opt.option_key === 'root_password' || opt.option_key === 'password'
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
  selectedSku.value?.cycles?.length ? selectedSku.value.cycles : (product.value?.cycles ?? []),
)

const descriptionText = computed(() => htmlToPlainText(product.value?.description))

/** 上游固定配置（只读展示）：锁定项表示上游商品已定死，本地不可改。 */
const lockedUpstreamOptions = computed(() => (product.value?.upstream_chain?.options ?? []).filter((o) => o.locked))

/** 不可续费提示（上游一次性/免费商品）；可续费时为空。 */
const renewNote = computed(() => {
  const chain = product.value?.upstream_chain
  if (!chain || chain.renewable) return ''
  return chain.renew_note || '该商品不支持续费'
})

/** 开通说明：主机名可自定义 / 密码随机生成 / 端口由上游分配。 */
const upstreamOrderNotes = computed(() => product.value?.upstream_chain?.order_notes ?? [])

/**
 * 售罄判定：商品挂了 SKU 时看所选 SKU，没挂 SKU（上游转售商品）时看商品级库存。
 * 上游限量商品的余量在商品行上（stock=0 即售罄），此前这里只看 SKU，
 * 导致上游售罄的商品前台仍可下单（后端会拦，但用户是先扣款入口前才看到报错）。
 */
const soldOut = computed(() => {
  if (selectedSku.value) return selectedSku.value.stock === 0
  return product.value?.stock === 0
})

const canSubmit = computed(
  () => !!product.value && !!quote.value && !soldOut.value && missingRequired.value.length === 0,
)

/**
 * 生成一个随机主机名：与后端 safeHostname 规则一致（字母开头 + 小写字母数字连字符）。
 *
 * 后缀取 8 位而不是 6 位：带 hs- 前缀共 11 位，满足上游「主机名至少 10 位」的校验；
 * 6 位时总长只有 9 位，提交后会被上游以「主机名长度至少10位」拒绝，白扣一次款。
 * 抽成独立函数供「开页预填」与「点换一个」两处共用，避免两边规则漂移。
 */
function randomHostnameValue(): string {
  return `hs-${Math.random().toString(36).slice(2, 10)}`
}

/** 换一个随机主机名（用户对预填的名字不满意时点）。 */
function randomHostname(opt: ProductOption) {
  setSelectionValue(opt, randomHostnameValue())
}

function setSelection(opt: ProductOption, value: SelectValue) {
  setSelectionValue(opt, value)
}

/** 统一写入选择：控件回调可能给字符串/数字/布尔，一律按字符串存。 */
function setSelectionValue(opt: ProductOption, value: SelectValue | NumericValue | SliderValue | null) {
  // 滑块在双点位模式下给数组；本页只用单选模式，取首值兜底，避免把 "80,100" 写进选择。
  const scalar = Array.isArray(value) ? value[0] : value
  selection.value = {
    ...selection.value,
    [opt.option_key]: scalar === null || scalar === undefined ? '' : String(scalar),
  }
  refreshQuote()
}

/**
 * 数量型选项的可填区间提示（如「可填 80 – 1024 GB，默认 80 GB」）。
 *
 * 为什么要把默认值和区间都写出来：这类参数按量计价，客户得知道
 * 「多少是不加价的、最多能加到多少」，否则看到滑块两端不知道该往哪拉。
 */
function qtyRangeHint(opt: ProductOption): string {
  if (optionWidget(opt) !== 'qty') return ''
  const unit = opt.unit ? ` ${opt.unit}` : ''
  const min = opt.min_value
  const max = opt.max_value
  const def = opt.default_value !== '' ? Number(opt.default_value) : NaN
  const parts: string[] = []
  if (min !== null && min !== undefined && max !== null && max !== undefined) {
    parts.push(`可填 ${min} – ${max}${unit}`)
  } else if (min !== null && min !== undefined) {
    parts.push(`最小 ${min}${unit}`)
  } else if (max !== null && max !== undefined) {
    parts.push(`最大 ${max}${unit}`)
  }
  if (!Number.isNaN(def)) {
    parts.push(`${def}${unit} 以内不加价`)
  }
  // 超出部分的单价随周期变化（上游每周期各一价），把当前周期的价格写出来，
  // 客户拉滑块时才知道"多出来的这部分按什么价算"。
  const unitPrice = qtyUnitPrice(opt)
  if (unitPrice > 0) {
    const label = selectedCycle.value ? cycleLabel(selectedCycle.value) : ''
    parts.push(`超出部分 ¥${unitPrice.toFixed(2)}/${opt.unit || '单位'}${label ? `（${label}）` : ''}`)
  }
  return parts.filter(Boolean).join('，')
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

/** 折扣来源中文标签（agent=代理等级折扣，是当前唯一折扣来源）。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '优惠',
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
  const text = String(Array.isArray(raw) ? raw[0] : (raw ?? ''))
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

/* 骨架屏容器：与真实布局同栅格，加载完成时列宽不跳变 */
.config-layout--skeleton .config-card,
.config-layout--skeleton .config-summary {
  min-height: 120px;
}

/* 重算价时整块轻微淡出，提示"正在更新"但不遮挡内容 */
.config-prices.is-refreshing {
  opacity: 0.55;
  transition: opacity 0.15s ease;
}

.config-prices__hint {
  padding: var(--space-lg) 0;
  text-align: center;
  font-size: 13px;
  color: var(--color-muted-foreground, #6b7280);
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

/*
  数量型控件：滑块 + 数值输入并排。
  滑块占满剩余宽度、输入框固定宽度 —— 让"往右拉 = 加价"这件事一眼可见，
  同时保留精确输入的入口。
*/
.config-option__qty {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.config-option__qty-row {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.config-option__slider {
  flex: 1;
  min-width: 0;
}
.config-option__qty-input {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}
.config-option__qty-input :deep(.t-input-number) {
  width: 128px;
}
.config-option__qty-unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
.config-option__qty-range {
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.config-option__text {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
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
