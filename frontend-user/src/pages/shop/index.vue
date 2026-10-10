<template>
  <div class="page-body console-module shop-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CartIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">云主机选购</h2>
          <p class="page-header__desc">余额支付即时开通；加入购物车后可一并结算</p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" @click="router.push('/cart')">
          购物车
          <template #suffix v-if="cartStore.count > 0">({{ cartStore.count }})</template>
        </t-button>
      </div>
    </header>

    <!-- 站点跳转带来的意图：?product=<id>&spec=<spec_code>&cycle=<cycle> -->
    <section v-if="intentProductName" class="surface-card intent-card">
      <CheckCircleIcon size="18" class="intent-card__icon" />
      <span>来自官网的选购意向：<strong>{{ intentProductName }}</strong></span>
      <t-button size="small" variant="text" @click="clearIntent">知道了</t-button>
    </section>

    <FilterCard>
      <div class="field">
        <label class="field__label" for="shop-keyword">关键词</label>
        <t-input
          id="shop-keyword"
          v-model="keyword"
          placeholder="搜索商品名称"
          clearable
          @enter="search"
          @clear="search"
        />
      </div>
      <div class="field">
        <label class="field__label">只看推荐</label>
        <t-switch v-model="featuredOnly" @change="search" />
      </div>
      <template #actions>
        <t-button theme="primary" :loading="loading" @click="search">查询</t-button>
        <t-button variant="outline" @click="resetFilter">重置</t-button>
      </template>
    </FilterCard>

    <section v-loading="loading" class="shop-grid">
      <article v-for="p in products" :key="p.id" class="surface-card shop-card">
        <div class="shop-card__cover">
          <img v-if="p.cover_image" :src="p.cover_image" :alt="p.name" />
          <component :is="ServerIcon" v-else size="34" />
          <span v-if="p.featured" class="shop-card__flag">推荐</span>
        </div>

        <div class="shop-card__body">
          <h3 class="shop-card__name">{{ p.name }}</h3>
          <p class="shop-card__desc">{{ p.description || '暂无描述' }}</p>

          <div class="shop-card__meta">
            <t-tag size="small" variant="light">{{ sourceModeLabel(p.source_mode) }}</t-tag>
            <t-tag v-if="p.skus.length" size="small" variant="light" theme="primary">
              {{ p.skus.length }} 种规格
            </t-tag>
          </div>

          <div class="shop-card__foot">
            <div class="price-cell">
              <span class="shop-card__price">¥{{ p.price.toFixed(2) }}</span>
              <span class="shop-card__unit">{{ priceModelLabel(p.price_model) }}</span>
            </div>
            <t-button size="small" theme="primary" @click="openBuy(p)">购买</t-button>
          </div>
        </div>
      </article>

      <div v-if="!loading && !products.length" class="empty-state surface-card">
        <t-empty description="暂无可购买商品" />
      </div>
    </section>

    <t-pagination
      v-if="total > pagination.pageSize"
      v-model:current="pagination.current"
      v-model:page-size="pagination.pageSize"
      :total="total"
      :page-size-options="[12, 24, 48]"
      class="shop-pager"
      @change="loadProducts"
    />

    <!-- 购买：规格 + 周期 + 数量 → 实时预结算 -->
    <t-dialog
      v-model:visible="buyVisible"
      header="确认购买"
      width="520px"
    >
      <div v-if="current" class="buy">
        <div class="buy__product">{{ current.name }}</div>
        <p class="buy__desc">{{ current.description }}</p>

        <div v-if="current.skus.length" class="buy__field">
          <span class="buy__label">规格</span>
          <t-radio-group v-model="selectedSpec" variant="default-filled" @change="refreshQuote">
            <t-radio-button v-for="s in current.skus" :key="s.spec_code" :value="s.spec_code">
              {{ s.name }}
              <em class="buy__spec-price">¥{{ s.price.toFixed(2) }}</em>
            </t-radio-button>
          </t-radio-group>
          <p v-if="selectedSku?.stock === 0" class="buy__warn">该规格已售罄</p>
        </div>

        <div v-if="selectableCycles.length" class="buy__field">
          <span class="buy__label">计费周期</span>
          <t-radio-group v-model="selectedCycle" variant="default-filled" @change="refreshQuote">
            <t-radio-button v-for="c in selectableCycles" :key="c" :value="c">
              {{ cycleLabel(c) }}
            </t-radio-button>
          </t-radio-group>
        </div>

        <!--
          客户选配项：档位之外的参数（操作系统/CPU 档/带宽/数据盘…）。
          控件类型由后端按 widget 下发：group_select 分组下拉（OS 按 Ubuntu/Windows）、
          radio 按钮组（CPU 2核/4核/8核）、qty 步进器、bool 开关、select 普通下拉。
          每项旁实时显示加价；最终金额仍以 quoteOrder 的结果为准。
        -->
        <div v-if="allOptions.length" class="buy__field buy__field--options">
          <span class="buy__label">
            配置选项
            <em class="buy__label-hint">不改动即按档位默认配置</em>
          </span>

          <div class="buy__options">
            <div v-for="opt in allOptions" :key="opt.option_key" class="buy__option">
              <div class="buy__option-head">
                <span class="buy__option-name">
                  {{ opt.name }}
                  <em v-if="opt.required" class="buy__option-required">必选</em>
                  <em v-if="opt.unit" class="buy__option-unit">/{{ opt.unit }}</em>
                </span>
                <span v-if="optionSurcharge(opt) > 0" class="buy__option-add">
                  +¥{{ optionSurcharge(opt).toFixed(2) }}
                </span>
              </div>

              <!-- 分组下拉：操作系统按镜像家族（Ubuntu / Windows / CentOS）分组 -->
              <t-select
                v-if="optionWidget(opt) === 'group_select'"
                :model-value="selection[opt.option_key] || undefined"
                :placeholder="optionPlaceholder(opt)"
                :clearable="!opt.required"
                @change="(v: SelectValue) => setSelection(opt, v)"
              >
                <t-option-group
                  v-for="g in groupedValues(opt)"
                  :key="g.label"
                  :label="g.label"
                  divider
                >
                  <t-option
                    v-for="it in g.items"
                    :key="it.value"
                    :value="it.value"
                    :label="optionItemLabel(opt, it)"
                  />
                </t-option-group>
              </t-select>

              <!-- 普通下拉：区域/节点/存储等来源为平台资源列表的取值 -->
              <t-select
                v-else-if="optionWidget(opt) === 'select'"
                :model-value="selection[opt.option_key] || undefined"
                :placeholder="optionPlaceholder(opt)"
                :clearable="!opt.required"
                @change="(v: SelectValue) => setSelection(opt, v)"
              >
                <t-option
                  v-for="it in opt.values"
                  :key="it.value"
                  :value="it.value"
                  :label="optionItemLabel(opt, it)"
                />
              </t-select>

              <!-- 按钮组：CPU 2核/4核/8核、网络类型这类少量离散取值 -->
              <t-radio-group
                v-else-if="optionWidget(opt) === 'radio'"
                :model-value="selection[opt.option_key]"
                variant="default-filled"
                @change="(v: string | number | boolean) => setSelectionValue(opt, v)"
              >
                <t-radio-button v-for="it in opt.values" :key="it.value" :value="it.value">
                  {{ it.label }}
                  <em v-if="itemAddPrice(opt, it) > 0" class="buy__option-add-inline">
                    +¥{{ itemAddPrice(opt, it).toFixed(2) }}
                  </em>
                </t-radio-button>
              </t-radio-group>

              <!-- 开关：二值参数（如 IP-MAC 绑定） -->
              <t-switch
                v-else-if="optionWidget(opt) === 'bool'"
                :model-value="selection[opt.option_key]"
                :custom-value="boolValues(opt)"
                @change="(v: SwitchValue) => setSelectionValue(opt, v)"
              />

              <!-- 步进器：带宽/数据盘/IP 数量这类数量型参数，超出档位内含量才加价 -->
              <div v-else-if="optionWidget(opt) === 'qty'" class="buy__option-qty">
                <t-input-number
                  :model-value="numberValue(opt)"
                  :min="opt.min_value ?? 0"
                  :max="opt.max_value ?? undefined"
                  :step="1"
                  theme="normal"
                  @change="(v: NumericValue) => setSelectionValue(opt, v ?? '')"
                />
                <span v-if="opt.unit" class="buy__option-qty-unit">{{ opt.unit }}</span>
              </div>

              <p v-if="opt.help" class="buy__option-help">{{ opt.help }}</p>
            </div>
          </div>

          <p v-if="missingRequired.length" class="buy__warn">
            请选择：{{ missingRequired.map((o) => o.name).join('、') }}
          </p>
        </div>

        <div class="buy__field">
          <span class="buy__label">数量</span>
          <t-input-number v-model="quantity" :min="1" :max="99" theme="normal" @change="refreshQuote" />
        </div>

        <div class="buy__field">
          <span class="buy__label">支付方式</span>
          <t-radio-group v-model="payMode" variant="default-filled">
            <t-radio-button value="balance">余额支付（即时开通）</t-radio-button>
            <t-radio-button value="channel">在线支付（去收银台）</t-radio-button>
          </t-radio-group>
          <p class="buy__hint">
            {{
              payMode === 'balance'
                ? '从账户余额扣款，下单后立即开通资源，余额不足会提示先充值。'
                : '提交后生成待支付订单，用支付宝/微信/线下转账等方式完成支付；到账后自动开通。'
            }}
          </p>
        </div>

        <div v-loading="quoteLoading" class="buy__prices">
          <div v-if="quote" class="buy__rows">
            <!--
              档位价 = 原价 − 选配加价：后端 original_amount 已含选配，若直接叫「商品原价」
              再单列一行加价，读起来像 119 + 60 = 179。拆开写才能与应付金额对上。
            -->
            <div class="buy__row">
              <span>档位价{{ selectedSku ? ` · ${selectedSku.name}` : '' }}</span>
              <span>¥{{ baseAmount.toFixed(2) }}</span>
            </div>
            <div v-if="(quote.option_amount ?? 0) > 0" class="buy__row buy__row--option">
              <span>
                配置选项加价
                <em class="buy__row-detail">{{ optionSummary }}</em>
              </span>
              <span>+¥{{ (quote.option_amount ?? 0).toFixed(2) }}</span>
            </div>
            <div v-if="quote.discount_amount > 0" class="buy__row buy__row--discount">
              <span>
                优惠金额
                <t-tag size="small" variant="light" theme="success">
                  {{ sourceLabel(quote.discount_source) }}
                </t-tag>
              </span>
              <span>-¥{{ quote.discount_amount.toFixed(2) }}</span>
            </div>
            <div class="buy__row buy__row--final">
              <span>应付金额</span>
              <span>¥{{ quote.final_amount.toFixed(2) }}</span>
            </div>
            <p v-if="quantity > 1" class="buy__row-note">以上为单台价格，共 {{ quantity }} 台</p>
          </div>
          <t-empty v-else-if="!quoteLoading" description="价格计算失败，请稍后重试" />
        </div>
      </div>

      <template #footer>
        <div class="buy__footer">
          <t-button variant="outline" :disabled="!current || !quote" @click="addCurrentToCart">加入购物车</t-button>
          <div class="buy__footer-right">
            <t-button variant="outline" @click="buyVisible = false">取消</t-button>
            <t-button
              theme="primary"
              :loading="submitting"
              :disabled="!quote || soldOut || missingRequired.length > 0"
              @click="submitOrder"
            >
              {{ payMode === 'balance' ? '余额支付并开通' : '提交订单并支付' }}            </t-button>
          </div>
        </div>
      </template>
    </t-dialog>

    <!-- 在线支付收银台：下单后（订单已落待支付）选渠道付款，到账由后端回调自动开通 -->
    <PaymentCashier
      v-model:visible="cashierVisible"
      :amount="cashierAmount"
      :submit="submitCashier"
      @paid="onCashierPaid"
      @closed="onCashierClosed"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { CartIcon, CheckCircleIcon, ServerIcon } from 'tdesign-icons-vue-next'

import {
  createOrder,
  getProductDetail,
  getProducts,
  payOrder,
  quoteOrder,
  type OptionItem,
  type ProductInfo,
  type ProductOption,
  type QuoteInfo,
  type SkuInfo,
} from '@/api/shop'
import PaymentCashier, { type CashierPayment } from '@/components/payment-cashier/index.vue'
import FilterCard from '@/components/filter-card/index.vue'
import { useCartStore, cartItemKey } from '@/store/modules/cart'

/**
 * 控件回调的取值类型。
 *
 * TDesign 回调参数类型不都在包根导出（InputNumberValue 只在 es/input-number/type
 * 里），这里按契约就地声明，避免为几个别名深引内部路径。
 */
type SelectValue = string | number | boolean
type SwitchValue = string | number | boolean
type NumericValue = number | string | undefined

defineOptions({ name: 'Shop' })

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()

// ========== 列表 ==========
const products = ref<ProductInfo[]>([])
const loading = ref(false)
const keyword = ref('')
const featuredOnly = ref(false)
const total = ref(0)
const pagination = ref({ current: 1, pageSize: 12 })

/**
 * 官网跳转带来的选购意图（doc87 §2.2）：
 * /shop?product=<id>&spec=<spec_code>&cycle=<cycle>。
 * 意图只用来「预填 + 自动拉起购买弹窗」，不直接下单 —— 支付金额必须由用户确认。
 */
const intentProductName = ref('')

// ========== 购买弹窗 ==========
const buyVisible = ref(false)
const current = ref<ProductInfo | null>(null)
const selectedSpec = ref('')
const selectedCycle = ref('')
const quantity = ref(1)
const quote = ref<QuoteInfo | null>(null)
const quoteLoading = ref(false)
const submitting = ref(false)
/** 支付方式：balance 余额支付（默认，下单即开通）/ channel 在线支付（下单后去收银台）。 */
const payMode = ref<'balance' | 'channel'>('balance')

// ========== 客户选配项 ==========
/** 选配项选择：平台参数名 → 取值（下单时原样传给 config_selections）。 */
const selection = ref<Record<string, string>>({})

/** 商品下所有可选项（当前后端只给一个分组，展平后逐项渲染）。 */
const allOptions = computed<ProductOption[]>(() =>
  (current.value?.option_groups ?? []).flatMap((g) => g.options),
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

/** 开关型的两端取值：后端按 [关, 开] 顺序给枚举值（如 0/1、不开启/开启绑定）。 */
function boolValues(opt: ProductOption): SwitchValue[] {
  const values = opt.values.map((v) => v.value)
  if (values.length >= 2) return [values[0], values[1]]
  return [0, 1]
}

/**
 * 数量型输入框的回显值（数字）。
 *
 * 输入框要求 number，而 selection 里存的是字符串；空值时回落到默认值，
 * 使「未改动配置」也显示档位内含的默认数量（如带宽默认 10M）。
 */
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
  return opt.required ? `请选择${opt.name}` : `按平台默认（可选）`
}

/**
 * 某取值在当前计费周期下的加价。
 *
 * 与后端 priceForCycle 同口径：季/年/一次性优先取该列，未维护时回落月度。
 * 仅用于界面上的「+¥x」，真实金额以 quoteOrder 为准。
 */
function itemAddPrice(opt: ProductOption, it: OptionItem): number {
  const cycle = selectedCycle.value
  if (cycle === 'quarterly' && it.price_quarterly > 0) return it.price_quarterly
  if (cycle === 'annually' && it.price_annually > 0) return it.price_annually
  if (cycle === 'onetime' && it.price_onetime > 0) return it.price_onetime
  void opt
  return it.price_monthly
}

/**
 * 某项当前选择的加价（界面展示用）。
 *
 * 枚举型取所选取值；数量型按「超出档位内含量的部分 × 每单位加价」估，
 * 与后端 ResolveOptionPricing 的口径一致（默认值以内不加价）。
 */
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

/**
 * 默认选择：必选项回填默认值/首个可选值，非必选项留空表示「按平台默认」。
 * 回填后立即参与预结算，界面上的加价与应付金额从一开始就与下单一致。
 */
function applyDefaultSelection() {
  const next: Record<string, string> = {}
  for (const opt of allOptions.value) {
    const widget = optionWidget(opt)
    if (widget === 'qty') {
      // 数量型：填默认值，便于用户看到「当前是几」并在此基础上下调/上调。
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

/** 未选中的必选项（枚举型才算；数量型有默认值，不会缺）。 */
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

/**
 * 档位价 = 原价 − 选配加价。
 *
 * 后端 original_amount 已经把选配加价算进去了（先加价再打折），所以单列一行
 * 加价时必须把它从原价里扣掉，否则「档位价 + 加价」会比应付金额大出一截。
 */
const baseAmount = computed(() => {
  const original = quote.value?.original_amount ?? 0
  const optionAmount = quote.value?.option_amount ?? 0
  return Math.max(0, Math.round((original - optionAmount) * 100) / 100)
})

/** 已选配置的摘要（「8核 · 带宽 20Mbps」），挂在加价行后说明加价从哪来。 */
const optionSummary = computed(() => {
  const parts = selectionLabels.value.filter((label) => {
    // 只有真正产生加价的项才值得列出来，避免把默认配置也念一遍。
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

function setSelection(opt: ProductOption, value: SelectValue) {
  setSelectionValue(opt, value)
}

/** 统一写入选择：控件回调可能给字符串/数字/布尔，一律按字符串存（下发参数是字符串）。 */
function setSelectionValue(opt: ProductOption, value: SelectValue | NumericValue | null) {
  selection.value = {
    ...selection.value,
    [opt.option_key]: value === null || value === undefined ? '' : String(value),
  }
  refreshQuote()
}

// ========== 收银台（在线支付） ==========
const cashierVisible = ref(false)
/** 已提交的待支付订单 ID：关闭收银台不删单，由用户在列表继续支付或后端超时关单。 */
const pendingOrderId = ref(0)
const cashierAmount = ref(0)

function submitCashier(channelCode: string, scene: string): Promise<CashierPayment> {
  return payOrder(pendingOrderId.value, { channel_code: channelCode, scene }).then(({ data }) => ({
    payment_no: data.payment_no,
    amount: data.amount,
    pay_url: data.pay_url,
    qrcode: data.qrcode,
    instructions: data.instructions,
    status: data.status,
  }))
}

async function onCashierPaid() {
  const orderId = pendingOrderId.value
  pendingOrderId.value = 0
  if (current.value) {
    cartStore.remove(cartItemKey(current.value.id, selectedSpec.value, selectedCycle.value, cleanSelections.value))
  }
  MessagePlugin.success('支付完成，资源开通中')
  if (orderId) router.push(`/order/${orderId}`)
}

/**
 * 关闭收银台未付款：订单保留为待支付。
 *
 * 收银台上的按钮是「稍后支付」，删单会跟这句承诺自相矛盾（用户回头去订单列表找不到单）。
 * 订单先放着，等用户自己在列表里继续支付或取消；超时也会被调度器关掉。
 */
function onCashierClosed() {
  const orderId = pendingOrderId.value
  pendingOrderId.value = 0
  if (!orderId) return
  MessagePlugin.info('订单已生成并保留为待支付，可在「我的订单」继续支付')
}

const selectedSku = computed<SkuInfo | undefined>(() =>
  current.value?.skus.find((s) => s.spec_code === selectedSpec.value),
)

/** 周期来源：选中规格的自有周期优先，否则回落商品级周期。 */
const selectableCycles = computed(() => selectedSku.value?.cycles?.length ? selectedSku.value.cycles : current.value?.cycles ?? [])

const soldOut = computed(() => selectedSku.value?.stock === 0)

function cycleLabel(c: string): string {
  const map: Record<string, string> = {
    hourly: '按小时',
    monthly: '按月',
    quarterly: '按季',
    semiannually: '半年',
    annually: '按年',
    biennially: '两年',
    onetime: '一次性',
  }
  return map[c] || c
}

function priceModelLabel(m: string): string {
  const map: Record<string, string> = {
    monthly: '/月',
    quarterly: '/季',
    annually: '/年',
    hourly: '/小时',
    fixed: '一次性',
    onetime: '一次性',
  }
  return map[m] || ''
}

function sourceModeLabel(m: string): string {
  return m === 'upstream' ? '上游直供' : '自营'
}

/** 折扣来源中文标签：agent=代理等级折扣（doc108 起唯一折扣来源）；
 *  group 保留是为了历史订单——doc108 之前用户组价格策略产生的订单仍写着 group。 */
function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '代理折扣',
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
  }
  return map[source] || source || '优惠'
}

// ========== 数据加载 ==========
async function loadProducts() {
  loading.value = true
  try {
    const { data } = await getProducts({
      keyword: keyword.value || undefined,
      featured: featuredOnly.value || undefined,
      page: pagination.value.current,
      page_size: pagination.value.pageSize,
    })
    products.value = data?.items || []
    total.value = data?.total || 0
  } catch {
    products.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  pagination.value.current = 1
  loadProducts()
}

function resetFilter() {
  keyword.value = ''
  featuredOnly.value = false
  search()
}

// ========== 购买流程 ==========
async function openBuy(product: ProductInfo, specCode = '', cycle = '', preselect: Record<string, string> = {}) {
  // 列表里的商品不带 skus/cycles/option_groups（列表接口不下发），拉一次详情再开弹窗。
  current.value = product
  buyVisible.value = true
  quote.value = null
  quantity.value = 1
  selection.value = {}

  try {
    const { data } = await getProductDetail(product.id)
    if (data) current.value = data
  } catch {
    // 详情拿不到就按商品级价格下单，不阻断（存量商品本就没有 SKU）
  }

  selectedSpec.value = specCode || current.value?.skus[0]?.spec_code || ''
  const cycles = selectedSpec.value
    ? current.value?.skus.find((s) => s.spec_code === selectedSpec.value)?.cycles ?? []
    : []
  const available = cycles.length ? cycles : current.value?.cycles ?? []
  selectedCycle.value = cycle && available.includes(cycle) ? cycle : available[0] || ''

  // 用默认值预填选配项；来自官网的意图里带的选择覆盖在默认值之上。
  applyDefaultSelection()
  if (Object.keys(preselect).length) {
    const known = new Set(allOptions.value.map((o) => o.option_key))
    const merged = { ...selection.value }
    for (const [k, v] of Object.entries(preselect)) {
      if (v !== '' && known.has(k)) merged[k] = v
    }
    selection.value = merged
  }

  await refreshQuote()
}

async function refreshQuote() {
  if (!current.value) return
  quoteLoading.value = true
  try {
    const { data } = await quoteOrder({
      productId: current.value.id,
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

async function submitOrder() {
  if (!current.value || !quote.value) return
  if (soldOut.value) {
    MessagePlugin.warning('所选规格已售罄')
    return
  }
  if (missingRequired.value.length) {
    MessagePlugin.warning(`请先选择：${missingRequired.value.map((o) => o.name).join('、')}`)
    return
  }
  submitting.value = true
  try {
    const { data } = await createOrder({
      productId: current.value.id,
      specCode: selectedSpec.value,
      cycle: selectedCycle.value,
      quantity: quantity.value,
      configSelections: cleanSelections.value,
      payMode: payMode.value,
    })
    buyVisible.value = false
    cartStore.remove(cartItemKey(current.value.id, selectedSpec.value, selectedCycle.value, cleanSelections.value))

    // 渠道支付：订单已落待支付，接着拉起收银台；付款成功由后端回调开通。
    // 这里不跳转到详情页 —— 用户还没付钱，跳走会让他找不到收银台。
    if (payMode.value === 'channel') {
      pendingOrderId.value = data?.id || 0
      cashierAmount.value = quote.value.final_amount
      cashierVisible.value = true
      return
    }

    MessagePlugin.success(`已购买「${current.value.name}」，资源开通中`)
    if (data?.id) router.push(`/order/${data.id}`)
  } catch {
    // 请求拦截器已提示错误原因（余额不足/库存不足等）
  } finally {
    submitting.value = false
  }
}

function addCurrentToCart() {
  if (!current.value) return
  if (missingRequired.value.length) {
    MessagePlugin.warning(`请先选择：${missingRequired.value.map((o) => o.name).join('、')}`)
    return
  }
  const sku = selectedSku.value
  cartStore.add({
    productId: current.value.id,
    productName: current.value.name,
    coverImage: current.value.cover_image,
    specCode: selectedSpec.value,
    specName: sku?.name || '默认规格',
    cycle: selectedCycle.value,
    quantity: quantity.value,
    unitPrice: quote.value?.final_amount && quantity.value
      ? quote.value.final_amount / quantity.value
      : sku?.price || current.value.price,
    priceModel: current.value.price_model,
    selections: cleanSelections.value,
    selectionLabels: selectionLabels.value,
  })
  MessagePlugin.success('已加入购物车')
  buyVisible.value = false
}

// ========== 官网跳转意图 ==========
function clearIntent() {
  intentProductName.value = ''
  router.replace({ path: '/shop', query: {} })
}

async function consumeIntent() {
  const raw = route.query.product
  const productId = Number(Array.isArray(raw) ? raw[0] : raw)
  if (!productId || Number.isNaN(productId)) return

  const spec = String(route.query.spec ?? '')
  const cycle = String(route.query.cycle ?? '')
  const preselect = parseIntentConfig(route.query.config)

  try {
    const { data } = await getProductDetail(productId)
    if (!data) return
    intentProductName.value = data.name
    // 商品已下架时详情接口直接报错，这里自然走不到
    await openBuy(data, spec, cycle, preselect)
  } catch {
    MessagePlugin.warning('官网带来的商品不可购买，可能已下架')
    clearIntent()
  }
}

/**
 * 解析官网带来的选配意图（`config=cpu:4,os:62`）。
 *
 * 官网页面对配置项只做「选择 + 传递」，参数名与取值仍是平台口径；
 * 这里只是把意图交回购买弹窗做预填，真正的校验与算价仍在后端。
 * 无法识别的片段直接丢弃（不阻断整次跳转）。
 */
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
  await loadProducts()
  await consumeIntent()
})
</script>

<style scoped>
/* ============ 来自官网的选购意图提示条 ============ */
.intent-card {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 12px var(--space-xl);
  font-size: 13px;
  color: var(--color-foreground);
}

.intent-card__icon {
  color: var(--color-primary);
  flex-shrink: 0;
}

.intent-card :deep(.t-button) {
  margin-left: auto;
}

/* ============ 商品网格 ============ */
.shop-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(272px, 1fr));
  gap: var(--space-lg);
  min-height: 160px;
}

.shop-grid .empty-state {
  grid-column: 1 / -1;
}

.shop-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: transform var(--hs-duration-base) var(--hs-ease-out),
    box-shadow var(--hs-duration-base) var(--hs-ease-out);
}

.shop-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--hs-shadow-md);
}

.shop-card__cover {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 116px;
  background: linear-gradient(135deg, var(--hs-surface-3) 0%, var(--hs-surface-2) 100%);
  color: var(--color-muted-foreground);
  overflow: hidden;
}

.shop-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.shop-card__flag {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--color-primary);
  color: #fff;
  font-size: 11px;
}

.shop-card__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-lg);
  flex: 1;
}

.shop-card__name {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: var(--color-foreground);
}

.shop-card__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 40px;
}

.shop-card__meta {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  flex-wrap: wrap;
}

.shop-card__foot {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-top: auto;
  padding-top: var(--space-sm);
}

.shop-card__price {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.shop-card__unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.shop-pager {
  justify-content: flex-end;
}

/* ============ 购买弹窗 ============ */
.buy {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.buy__product {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.buy__desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.buy__field {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.buy__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

.buy__spec-price {
  margin-left: 4px;
  font-style: normal;
  opacity: 0.75;
  font-size: 12px;
}

/* ============ 选配项 ============ */
.buy__label-hint {
  margin-left: 6px;
  font-style: normal;
  font-weight: 400;
  font-size: 11px;
  opacity: 0.7;
}

.buy__options {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 320px;
  padding-right: 4px;
  overflow-y: auto;
}

.buy__option {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.buy__option-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.buy__option-name {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.buy__option-required {
  padding: 0 4px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--color-danger) 12%, transparent);
  color: var(--color-danger);
  font-style: normal;
  font-size: 10px;
  font-weight: 500;
}

.buy__option-unit {
  font-style: normal;
  font-weight: 400;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.buy__option-add {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--color-accent);
  font-variant-numeric: tabular-nums;
}

.buy__option-add-inline {
  margin-left: 4px;
  font-style: normal;
  font-size: 11px;
  opacity: 0.8;
}

.buy__option-qty {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
}

.buy__option-qty-unit {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.buy__option-help {
  margin: 0;
  font-size: 11.5px;
  line-height: 1.5;
  color: var(--color-muted-foreground);
}

/* 选配项里的按钮组允许换行：CPU 档位较多时窄屏不至于横向溢出 */
.buy__option :deep(.t-radio-group) {
  flex-wrap: wrap;
}

.buy__option :deep(.t-select) {
  width: 100%;
}

.buy__row--option {
  color: var(--color-foreground);
}

.buy__row-detail {
  font-style: normal;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.buy__row-note {
  margin: 2px 0 0;
  font-size: 11.5px;
  color: var(--color-muted-foreground);
  text-align: right;
}

.buy__warn {
  margin: 0;
  font-size: 12px;
  color: var(--color-danger);
}

/* 支付方式说明：随所选方式切换文案，解释「钱什么时候扣、资源什么时候开」 */
.buy__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.buy__prices {
  min-height: 96px;
  border-top: 1px solid var(--color-border);
  padding-top: var(--space-md);
}

.buy__rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.buy__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
  color: var(--color-muted-foreground);
  font-variant-numeric: tabular-nums;
}

.buy__row--discount {
  color: var(--color-accent);
}

.buy__row--final {
  margin-top: 4px;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border);
  font-size: 15px;
  font-weight: 700;
  color: var(--color-accent);
}

.buy__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  width: 100%;
}

.buy__footer-right {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

/* ============ 响应式 ============ */
@media (max-width: 768px) {
  .shop-grid {
    grid-template-columns: 1fr;
    gap: var(--space-md);
  }

  .intent-card {
    padding: 12px var(--space-md);
  }

  .buy__footer {
    flex-direction: column-reverse;
    align-items: stretch;
  }

  .buy__footer-right {
    flex-direction: column-reverse;
    align-items: stretch;
  }
}
</style>
