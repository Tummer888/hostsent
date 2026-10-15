<template>
  <!--
    代下单弹窗：内容很长（用户/商品 + 规格 + 周期 + 配置项 + 上游固定配置）。

    尺寸口径（用户反馈"高度超出屏幕、要一直往下滚"）：
      - 宽度拉到 92vw / 上限 1180px：配置项是多列表格型内容，横向有空间就不会每项换行，
        整体高度也跟着降下来；比单纯压高度更有效。
      - 高度交给 TDesign 的 dialog 默认上限（视口内），内容自己在 __body 里滚 —— 
        关键操作（底部按钮）始终可见，不会随内容被推出屏幕。
      - 窄屏回落 94vw（仓库既有写法）。
  -->
  <t-dialog
    v-model:visible="visible"
    header="为用户创建订单"
    :width="isMobile ? '94vw' : 'min(1180px, 92vw)'"
    :confirm-btn="confirmBtn"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <div class="configure-order">
      <!-- 下单用户 + 商品：两项都是「选一次」的入参，并排压缩首屏高度 -->
      <div class="configure-order__head">
        <div class="configure-order__field">
          <span class="configure-order__label">下单用户</span>
          <t-input :model-value="username || '—'" disabled />
        </div>
        <div class="configure-order__field configure-order__field--grow">
          <span class="configure-order__label">选择商品</span>
          <t-select
            v-model="form.product_id"
            :options="productOptions"
            filterable
            placeholder="请选择商品"
            :loading="productsLoading"
            @change="handleProductChange"
          />
        </div>
      </div>

      <t-loading :loading="configLoading" size="small" show-overlay>
        <template v-if="config">
          <!-- 规格：卡片态展示原子取值（2核 4G 50G）与价格，比纯文字单选一眼可辨 -->
          <section class="configure-order__section">
            <h4 class="configure-order__title">
              规格
              <em class="configure-order__title-hint">决定 CPU / 内存 / 硬盘与基准价</em>
            </h4>
            <div v-if="specs.length" class="configure-order__specs">
              <button
                v-for="sp in specs"
                :key="sp.spec_code"
                type="button"
                class="configure-order__spec"
                :class="{ 'is-active': form.spec_code === sp.spec_code }"
                @click="selectSpec(sp)"
              >
                <span class="configure-order__spec-name">{{ sp.name }}</span>
                <span v-if="specAtoms(sp).length" class="configure-order__spec-atoms">
                  <span v-for="a in specAtoms(sp)" :key="a.label" class="configure-order__spec-atom">
                    {{ a.label }}
                  </span>
                </span>
                <span class="configure-order__spec-foot">
                  <em class="configure-order__spec-price">¥{{ money(sp.price) }}</em>
                  <em v-if="sp.stock === 0" class="configure-order__soldout">已售罄</em>
                  <em v-else-if="sp.stock > 0" class="configure-order__stock">剩 {{ sp.stock }}</em>
                </span>
              </button>
            </div>
            <p v-else class="configure-order__empty">该商品未挂规格，按商品级价格下单。</p>
          </section>

          <!-- 计费周期：选项少时用紧凑按钮组，不撑满整行 -->
          <section v-if="cycleChoices.length" class="configure-order__section">
            <h4 class="configure-order__title">计费周期</h4>
            <t-radio-group v-model="form.billing_cycle" variant="default-filled" size="small">
              <t-radio-button v-for="c in cycleChoices" :key="c" :value="c">
                {{ cycleLabel(c) }}
              </t-radio-button>
            </t-radio-group>
          </section>

          <!-- 配置项：一小卡片一项，标题行显示当前取值与加价 -->
          <section v-if="options.length" class="configure-order__section">
            <h4 class="configure-order__title">
              配置选项
              <em class="configure-order__title-hint">已按默认配置预选，需改动再调</em>
            </h4>
            <div class="configure-order__options">
              <div v-for="opt in options" :key="opt.option_key" class="configure-order__option">
                <div class="configure-order__option-head">
                  <span class="configure-order__option-name">
                    {{ opt.name }}
                    <em v-if="opt.required" class="configure-order__required">必选</em>
                    <em v-if="opt.unit" class="configure-order__unit">/{{ opt.unit }}</em>
                  </span>
                  <span v-if="optionSurcharge(opt) > 0" class="configure-order__option-add">
                    +¥{{ money(optionSurcharge(opt)) }}
                  </span>
                </div>

                <!-- 分组下拉：操作系统按镜像家族；选项少时收窄宽度 -->
                <t-select
                  v-if="opt.widget === 'group_select'"
                  :model-value="selection[opt.option_key] || undefined"
                  :style="{ maxWidth: selectMaxWidth(opt) }"
                  :placeholder="placeholder(opt)"
                  :clearable="!opt.required"
                  filterable
                  @change="(v: SelectValue) => setSelection(opt, v)"
                >
                  <t-option-group v-for="g in groupedValues(opt)" :key="g.label" :label="g.label" divider>
                    <t-option v-for="it in g.items" :key="it.value" :value="it.value" :label="itemLabel(opt, it)" />
                  </t-option-group>
                </t-select>

                <!-- 普通下拉 -->
                <t-select
                  v-else-if="opt.widget === 'select'"
                  :model-value="selection[opt.option_key] || undefined"
                  :style="{ maxWidth: selectMaxWidth(opt) }"
                  :placeholder="placeholder(opt)"
                  :clearable="!opt.required"
                  filterable
                  @change="(v: SelectValue) => setSelection(opt, v)"
                >
                  <t-option v-for="it in opt.values" :key="it.value" :value="it.value" :label="itemLabel(opt, it)" />
                </t-select>

                <!-- 单选按钮组 -->
                <t-radio-group
                  v-else-if="opt.widget === 'radio'"
                  :model-value="selection[opt.option_key]"
                  variant="default-filled"
                  size="small"
                  @change="(v: string | number | boolean) => setSelectionValue(opt, v)"
                >
                  <t-radio-button v-for="it in opt.values" :key="it.value" :value="it.value">
                    {{ it.label }}
                    <em v-if="itemAddPrice(opt, it) > 0" class="configure-order__option-add-inline">
                      +¥{{ money(itemAddPrice(opt, it)) }}
                    </em>
                  </t-radio-button>
                </t-radio-group>

                <!-- 开关 -->
                <t-switch
                  v-else-if="opt.widget === 'bool'"
                  :model-value="selection[opt.option_key]"
                  :custom-value="boolValues(opt)"
                  @change="(v: string | number | boolean) => setSelectionValue(opt, v)"
                />

                <!--
                  数量型：滑块 + 数值输入（上游按量计价，如系统盘 80–1024G、带宽 75–500M）。
                  与用户侧同一形态：拉一下就能看到加价怎么变，不用先猜数字再试。
                -->
                <div v-else-if="opt.widget === 'qty'" class="configure-order__qty">
                  <t-slider
                    class="configure-order__slider"
                    :model-value="numberValue(opt)"
                    :min="opt.min_value ?? 0"
                    :max="opt.max_value ?? undefined"
                    :step="1"
                    :label="false"
                    @change="(v: number | number[]) => setSelectionValue(opt, Array.isArray(v) ? v[0] : v)"
                  />
                  <div class="configure-order__qty-input">
                    <t-input-number
                      :model-value="numberValue(opt)"
                      :min="opt.min_value ?? 0"
                      :max="opt.max_value ?? undefined"
                      :step="1"
                      theme="normal"
                      @change="(v: number | string | undefined) => setSelectionValue(opt, v ?? '')"
                    />
                    <span v-if="opt.unit" class="configure-order__qty-unit">{{ opt.unit }}</span>
                  </div>
                  <span v-if="qtyRangeHint(opt)" class="configure-order__qty-range">{{ qtyRangeHint(opt) }}</span>
                </div>

                <!-- 文本：主机名/登录密码等自由填写项；密码用遮蔽输入 -->
                <div v-else-if="opt.widget === 'text'" class="configure-order__text">
                  <t-input
                    :model-value="selection[opt.option_key] || ''"
                    :type="isSecretOption(opt) ? 'password' : 'text'"
                    :placeholder="placeholder(opt)"
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

                <p v-if="optionHelp(opt)" class="configure-order__option-help">{{ optionHelp(opt) }}</p>
              </div>
            </div>
            <p v-if="missingRequired.length" class="configure-order__warn">
              请选择：{{ missingRequired.map((o) => o.name).join('、') }}
            </p>
          </section>

          <!-- 上游固定配置：只读，说明「这些项定死了，改也开不出来」 -->
          <section v-if="lockedUpstream.length" class="configure-order__section">
            <h4 class="configure-order__title">
              上游固定配置
              <em class="configure-order__title-hint">上游已定规格，本地不可修改</em>
            </h4>
            <div class="configure-order__fixed">
              <span v-for="o in lockedUpstream" :key="o.key" class="configure-order__fixed-item">
                {{ o.name }}：<strong>{{ o.value || '—' }}</strong>
              </span>
            </div>
            <p v-if="upstreamRenewNote" class="configure-order__warn">{{ upstreamRenewNote }}</p>
          </section>

          <!-- 数量与开通方式 -->
          <section class="configure-order__section">
            <h4 class="configure-order__title">数量与开通方式</h4>
            <div class="configure-order__row">
              <span class="configure-order__row-label">数量</span>
              <t-input-number v-model="form.quantity" :min="1" :max="99" theme="normal" />
            </div>
            <div class="configure-order__row">
              <span class="configure-order__row-label">
                开通方式
                <em class="configure-order__row-hint">
                  仅创建：只落待支付订单，用户自己去付款；<br />
                  余额支付并开通：从用户余额扣款后立即投递开通任务。
                </em>
              </span>
              <t-radio-group v-model="form.pay_mode" variant="default-filled" size="small">
                <t-radio-button value="create">创建订单（待支付）</t-radio-button>
                <t-radio-button value="balance">余额支付并开通</t-radio-button>
              </t-radio-group>
            </div>
          </section>

          <!-- 价格：先展示默认价（算价管线结果），再让运营按需覆盖首次价与续费价 -->
          <section class="configure-order__section configure-order__section--price">
            <h4 class="configure-order__title">
              价格
              <em class="configure-order__title-hint"> 留空即按默认价下单；填了则以所填价为准，不参与折扣算价 </em>
            </h4>
            <div class="configure-order__price-box">
              <div class="configure-order__price-base">
                <div class="configure-order__price-base-row">
                  <span class="configure-order__price-base-label">默认首次价</span>
                  <span class="configure-order__price-base-value">
                    <template v-if="quoteLoading">计算中…</template>
                    <template v-else-if="quote">¥{{ money(quote.final_amount) }}</template>
                    <template v-else>¥{{ money(defaultUnitPrice) }}</template>
                  </span>
                </div>
                <div v-if="quoteNote" class="configure-order__price-base-note">{{ quoteNote }}</div>
              </div>
              <div class="configure-order__price-fields">
                <label class="configure-order__price-field">
                  <span class="configure-order__price-field-name">首次价（元）</span>
                  <t-input-number
                    v-model="form.price"
                    :min="0"
                    :precision="2"
                    theme="normal"
                    :placeholder="defaultPricePlaceholder"
                    clearable
                  />
                  <em class="configure-order__price-field-hint"
                    >留空 = ¥{{ money(quote?.final_amount ?? defaultUnitPrice) }}</em
                  >
                </label>
                <label class="configure-order__price-field">
                  <span class="configure-order__price-field-name">
                    续费价（元/周期）
                    <em class="configure-order__price-field-tag">长期有效</em>
                  </span>
                  <t-input-number
                    v-model="form.renewal_price"
                    :min="0"
                    :precision="2"
                    theme="normal"
                    placeholder="留空 = 按商品定价续费"
                    clearable
                  />
                  <em class="configure-order__price-field-hint"> 填写后此机器每次续费都按它结算，商品调价不影响 </em>
                </label>
              </div>
            </div>
          </section>
        </template>
      </t-loading>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
/**
 * 后台代客下单弹窗：按商品实际可售配置渲染规格 / 周期 / 配置项。
 *
 * 为什么做成独立组件：用户列表与用户详情两处入口原来各有一份"商品 + 周期 + 价格"的
 * 简化表单，配置项两边都渲染不出来，代下单开出来的机器与用户自助购买不一致。
 * 统一抽到这里后，配置口径只有一个来源（/users/order-config）。
 *
 * 价格分两栏的原因：首次价与续费价经常是两笔不同的谈定价
 * （如首开 30、之后每次续费 18）。首次价走 orders.final_amount，
 * 续费价落 instances.renewal_price 长期绑定，二者互不影响。
 */
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createUserOrder,
  getAdminOrderConfig,
  quoteUserOrder,
  type AdminOrderConfigOption,
  type AdminOrderConfigOptionVal,
  type AdminOrderConfigResponse,
  type AdminOrderConfigSpec,
  type AdminOrderQuoteResponse,
} from '@/api/user'
import { getProductList as getUcProductList } from '@/api/product'
import { useIsMobile } from '@/composables/useIsMobile'

type SelectValue = string | number | boolean

// 弹窗宽度按视口切：窄屏用 94vw（仓库既有写法），桌面端拉到 min(1180px, 92vw)，
// 让配置项能多列铺开、整体高度降下来。
const { isMobile } = useIsMobile()

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
const submitting = ref(false)
const productsLoading = ref(false)
const configLoading = ref(false)
const quoteLoading = ref(false)
const productOptions = ref<{ label: string; value: number }[]>([])
const config = ref<AdminOrderConfigResponse | null>(null)
/** 默认价算价结果（原价/优惠/实付）；null 表示还没算出来或算价失败。 */
const quote = ref<AdminOrderQuoteResponse | null>(null)
/** 配置项选择：option_key → 取值（下发参数统一按字符串存）。 */
const selection = ref<Record<string, string>>({})

const form = reactive<{
  product_id: number | undefined
  spec_code: string
  billing_cycle: string
  quantity: number
  /** 首次开通的自定义价；undefined/0 = 用默认价 */
  price: number | undefined
  /** 绑定续费价；undefined/0 = 不绑定 */
  renewal_price: number | undefined
  pay_mode: 'create' | 'balance'
}>({
  product_id: undefined,
  spec_code: '',
  billing_cycle: '',
  quantity: 1,
  price: undefined,
  renewal_price: undefined,
  pay_mode: 'create',
})

const confirmBtn = computed(() => ({
  content: '创建订单',
  theme: 'primary' as const,
  loading: submitting.value,
  disabled: missingRequired.value.length > 0,
}))

const specs = computed(() => (config.value?.specs ?? []).filter((s) => s.stock !== 0))

/** 周期选项：优先所选规格自有周期，否则回落商品级周期。 */
const cycleChoices = computed(() => {
  const spec = config.value?.specs.find((s) => s.spec_code === form.spec_code)
  if (spec?.cycles?.length) return spec.cycles
  return config.value?.cycles ?? []
})

const options = computed(() => config.value?.options ?? [])
const lockedUpstream = computed(() => (config.value?.upstream?.options ?? []).filter((o) => o.locked))
const upstreamRenewNote = computed(() => {
  const up = config.value?.upstream
  if (!up || up.renewable) return ''
  return up.renew_note || '该商品不支持续费'
})

/** 未选中的必选枚举项（数量型/文本型有兜底，不拦提交）。 */
const missingRequired = computed(() =>
  options.value.filter((opt) => {
    if (!opt.required) return false
    if (opt.widget === 'qty' || opt.widget === 'text') return false
    return !selection.value[opt.option_key]
  }),
)

/** 兜底默认价：算价未返回时用所选规格价，再退商品基础价，避免价格栏空着。 */
const defaultUnitPrice = computed(() => {
  const spec = config.value?.specs.find((s) => s.spec_code === form.spec_code)
  if (spec && spec.price > 0) return spec.price
  return config.value?.price ?? 0
})

/** 首次价输入框占位符：把默认价写进去，客服一眼看到"不填就是这么多"。 */
const defaultPricePlaceholder = computed(() => {
  const amount = quote.value?.final_amount ?? defaultUnitPrice.value
  return amount > 0 ? `默认 ¥${money(amount)}` : '留空按商品价'
})

/** 默认价的构成说明：把选配加价与折扣拆出来，避免只有一个数字看不懂。 */
const quoteNote = computed(() => {
  const q = quote.value
  if (!q) return ''
  const parts: string[] = []
  if (q.option_amount > 0) parts.push(`含选配加价 ¥${money(q.option_amount)}`)
  if (q.discount_amount > 0) {
    parts.push(`优惠 -¥${money(q.discount_amount)}${q.discount_source ? `（${sourceLabel(q.discount_source)}）` : ''}`)
  }
  if (q.discount_amount > 0 && q.original_amount > 0) {
    parts.push(`原价 ¥${money(q.original_amount)}`)
  }
  return parts.join(' · ')
})

watch(
  () => props.modelValue,
  (value) => {
    visible.value = value
    if (value) {
      reset()
      void loadProducts()
    }
  },
)
watch(visible, (value) => emit('update:modelValue', value))

function reset() {
  if (quoteTimer) clearTimeout(quoteTimer)
  form.product_id = undefined
  form.spec_code = ''
  form.billing_cycle = ''
  form.quantity = 1
  form.price = undefined
  form.renewal_price = undefined
  form.pay_mode = 'create'
  config.value = null
  quote.value = null
  selection.value = {}
}

async function loadProducts() {
  if (productOptions.value.length) return
  productsLoading.value = true
  try {
    const data = await getUcProductList({ page: 1, page_size: 100 })
    productOptions.value = (data?.items || []).map((p) => ({
      label: `${p.name}（¥${p.price}）`,
      value: p.id,
    }))
  } catch {
    MessagePlugin.error('加载商品列表失败')
  } finally {
    productsLoading.value = false
  }
}

async function handleProductChange(productId: number | undefined) {
  config.value = null
  quote.value = null
  selection.value = {}
  form.spec_code = ''
  form.billing_cycle = ''
  if (!productId) return
  configLoading.value = true
  try {
    config.value = await getAdminOrderConfig(productId)
    // 只有一个规格时直接选中，省掉一次点击（多规格交给客服显式选择）。
    if (config.value.specs.length === 1 && config.value.specs[0].stock !== 0) {
      form.spec_code = config.value.specs[0].spec_code
    }
    if (cycleChoices.value.length) form.billing_cycle = cycleChoices.value[0]
    applyDefaultSelection()
    void refreshQuote()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载商品配置失败')
  } finally {
    configLoading.value = false
  }
}

/** 切换规格时同步周期选项（规格自有周期可能只有部分档位）。 */
function syncCycleForSpec() {
  const choices = cycleChoices.value
  if (!choices.length) return
  if (!form.billing_cycle || !choices.includes(form.billing_cycle)) {
    form.billing_cycle = choices[0]
  }
}

function selectSpec(sp: AdminOrderConfigSpec) {
  if (sp.stock === 0) return
  form.spec_code = sp.spec_code
}

watch(
  () => form.spec_code,
  () => {
    syncCycleForSpec()
    void refreshQuote()
  },
)
watch(
  () => form.billing_cycle,
  () => void refreshQuote(),
)
// 数量与选配改动同样影响默认价（选项加价并入基数后再打折）。
// 选配用 300ms 防抖：数量型是输入框，逐键触发会打出一串算价请求。
watch([() => form.quantity, selection], debounceQuote, { deep: true })

let quoteTimer: ReturnType<typeof setTimeout> | undefined

function debounceQuote() {
  if (quoteTimer) clearTimeout(quoteTimer)
  quoteTimer = setTimeout(() => void refreshQuote(), 300)
}

/**
 * 刷新默认价：调后端算价预览，与真正下单同一条管线。
 * 失败时静默清空（前端退回用规格价展示），不弹错误打断客服操作 ——
 * 算价失败不阻止下单，下单时后端还会再算一次并以那次为准。
 */
async function refreshQuote() {
  if (!form.product_id) return
  const token = ++quoteSeq
  quoteLoading.value = true
  try {
    const selections = cleanSelections()
    const resp = await quoteUserOrder(props.userId, {
      product_id: form.product_id,
      billing_cycle: form.billing_cycle || undefined,
      spec_code: form.spec_code || undefined,
      quantity: form.quantity,
      config_selections: Object.keys(selections).length ? selections : undefined,
    })
    // 慢响应回来时若已经发起了更新的算价，就丢弃这一份，避免价格栏倒退回旧值。
    if (token === quoteSeq) quote.value = resp
  } catch {
    if (token === quoteSeq) quote.value = null
  } finally {
    if (token === quoteSeq) quoteLoading.value = false
  }
}

/** 算价请求序号：只认最后一次发出的响应。 */
let quoteSeq = 0

/**
 * 默认选择：枚举项预选一个值，数量型填默认数量。
 *
 * 为什么非必选项也要预选：履约侧对"没选的项"就是取首个可见值
 * （上游 config_groups 的 pickConfigSub、自营的 pickEnumValue 都是这个口径）。
 * 界面上留空并不等于"不配置"，只等于"客服看不见会开出什么"——
 * 实测商品（网络类型/操作系统）的候选值都没有声明默认，留空时下拉是空白的，
 * 客服必须逐个点开才知道默认开的是 VPC 还是经典网络。
 * 这里把生效值显式预选出来，并以 selection 提交，界面所见即最终下发值。
 */
function applyDefaultSelection() {
  const next: Record<string, string> = {}
  for (const opt of options.value) {
    if (opt.widget === 'qty') {
      next[opt.option_key] = String(numberValue(opt))
      continue
    }
    // 文本型（主机名）没有候选值，但留空会让客服看不到最终会开成什么名字：
    // 后端 safeHostname 会按商品名/订单号兜底生成，名字不可预测。
    // 这里开窗即随机填一个，客服看着合适就留、不合适再改。
    if (opt.widget === 'text') {
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

/** 是否为敏感项（密码类）：渲染成遮蔽输入框（客服代下单时也不该明文示人）。 */
function isSecretOption(opt: AdminOrderConfigOption): boolean {
  return opt.option_key === 'root_password' || opt.option_key === 'password'
}

function setSelection(opt: AdminOrderConfigOption, value: SelectValue) {
  setSelectionValue(opt, value)
}

function setSelectionValue(opt: AdminOrderConfigOption, value: SelectValue | number | null | undefined) {
  selection.value = {
    ...selection.value,
    [opt.option_key]: value === null || value === undefined ? '' : String(value),
  }
}

function groupedValues(opt: AdminOrderConfigOption) {
  const groups: Array<{ label: string; items: AdminOrderConfigOptionVal[] }> = []
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

/**
 * 下拉框最大宽度：选项多/标签长时才给足宽度，选项少就按内容走。
 *
 * 原来固定满宽会把「网络类型」（三五个选项）撑成一整行的空框，
 * 一屏塞不下几个配置项，客服要反复滚动。
 */
function selectMaxWidth(opt: AdminOrderConfigOption): string {
  if (opt.widget === 'group_select' || opt.values.length > 8) return '100%'
  const longest = opt.values.reduce((max, v) => Math.max(max, v.label.length), 0)
  const px = Math.min(360, Math.max(140, longest * 14 + 64))
  return `${px}px`
}

function boolValues(opt: AdminOrderConfigOption): SelectValue[] {
  const values = opt.values.map((v) => v.value)
  if (values.length >= 2) return [values[0], values[1]]
  return [0, 1]
}

function numberValue(opt: AdminOrderConfigOption): number {
  const min = opt.min_value ?? undefined
  const max = opt.max_value ?? undefined
  let n: number | undefined
  const raw = selection.value[opt.option_key]
  if (raw !== undefined && raw !== '') {
    const parsed = Number(raw)
    if (!Number.isNaN(parsed)) n = parsed
  }
  if (n === undefined) {
    const def = Number(opt.default_value)
    if (opt.default_value !== '' && !Number.isNaN(def)) n = def
  }
  if (n === undefined) n = min ?? 0
  // 越界兜底：存量脏值已在数据层修正（migrations/111 + 启动期回填），
  // 这里再夹一次，任何来源的默认值都不会被原样预填给客户。
  if (min !== undefined && n < min) n = min
  if (max !== undefined && n > max) n = max
  return n
}

/**
 * 数量型选项的可填区间提示（如「可填 80 – 1024 GB，80 GB 以内不加价」）。
 *
 * 为什么显式写出来：这类参数按量计价，代下单的人得知道
 * 「多少以内不加价、最多能给到多少」，否则滑块两端没有依据。
 */
function qtyRangeHint(opt: AdminOrderConfigOption): string {
  if (opt.widget !== 'qty') return ''
  const unit = opt.unit ? ` ${opt.unit}` : ''
  const parts: string[] = []
  if (opt.min_value !== null && opt.min_value !== undefined && opt.max_value !== null && opt.max_value !== undefined) {
    parts.push(`可填 ${opt.min_value} – ${opt.max_value}${unit}`)
  } else if (opt.min_value !== null && opt.min_value !== undefined) {
    parts.push(`最小 ${opt.min_value}${unit}`)
  } else if (opt.max_value !== null && opt.max_value !== undefined) {
    parts.push(`最大 ${opt.max_value}${unit}`)
  }
  const def = opt.default_value !== '' ? Number(opt.default_value) : NaN
  if (!Number.isNaN(def)) parts.push(`${def}${unit} 以内不加价`)
  // 超出部分的单价随周期变化（上游每周期各一价），写明当前周期的价格，
  // 代下单时才知道"多出来的这部分按什么价算"。
  const unitPrice = qtyUnitPrice(opt)
  if (unitPrice > 0) {
    const label = form.billing_cycle ? cycleLabel(form.billing_cycle) : ''
    parts.push(`超出部分 ¥${unitPrice.toFixed(2)}/${opt.unit || '单位'}${label ? `（${label}）` : ''}`)
  }
  return parts.join('，')
}

function itemLabel(opt: AdminOrderConfigOption, it: AdminOrderConfigOptionVal): string {
  const add = itemAddPrice(opt, it)
  return add > 0 ? `${it.label}（+¥${money(add)}）` : it.label
}

function placeholder(opt: AdminOrderConfigOption): string {
  if (opt.help) return opt.help
  return opt.required ? `请选择${opt.name}` : '按平台默认（可选）'
}

function optionHelp(opt: AdminOrderConfigOption): string {
  const parts: string[] = []
  if (opt.help) parts.push(opt.help)
  const raw = selection.value[opt.option_key]
  if (raw) {
    const hit = opt.values.find((v) => v.value === raw)
    if (hit?.help) parts.push(`${hit.label}：${hit.help}`)
  }
  return parts.join(' ')
}

/** 某取值在当前周期下的加价（界面估算；真实金额以服务端算价为准）。 */
function itemAddPrice(opt: AdminOrderConfigOption, it: AdminOrderConfigOptionVal): number {
  switch (form.billing_cycle) {
    case 'quarterly':
      if (it.price_quarterly > 0) return it.price_quarterly
      break
    case 'annually':
      if (it.price_annually > 0) return it.price_annually
      break
    case 'onetime':
      if (it.price_onetime > 0) return it.price_onetime
      break
  }
  void opt
  return it.price_monthly
}

/** 某项当前选择的加价估算（口径与后端 ResolveOptionPricing 一致）。 */
function optionSurcharge(opt: AdminOrderConfigOption): number {
  if (opt.widget === 'qty') {
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
 * 该周期无独立单价时回落月度价（与后端 quantityUnitPriceForCycle 同口径），
 * 保证弹窗估算与实付一致。
 */
function qtyUnitPrice(opt: AdminOrderConfigOption): number {
  const current = form.billing_cycle
  if (current === 'quarterly') return opt.unit_price_quarterly || opt.unit_price || 0
  if (current === 'annually') return opt.unit_price_annually || opt.unit_price || 0
  if (current === 'onetime') return opt.unit_price_onetime || opt.unit_price || 0
  return opt.unit_price || 0
}

/**
 * 把 SKU 的原子取值 JSON 拍平成展示标签（「2核」「4G」「50G SSD」）。
 *
 * 只取几个常用原子：未知 key 原样丢弃而不是显示成 compute.foo=bar，
 * 否则界面会混进技术字段名，反而不如 SKU 名称直观。
 */
function specAtoms(sp: AdminOrderConfigSpec): Array<{ label: string }> {
  if (!sp.specs) return []
  let raw: Record<string, unknown>
  try {
    raw = JSON.parse(sp.specs)
  } catch {
    return []
  }
  if (!raw || typeof raw !== 'object') return []
  const out: Array<{ label: string }> = []
  const cpu = Number(raw['compute.cpu'])
  const memoryMB = Number(raw['compute.memory'])
  const diskGB = Number(raw['storage.system.size'])
  const diskType = String(raw['storage.system.type'] ?? '')
  const bandwidth = Number(raw['network.bandwidth'])
  if (cpu > 0) out.push({ label: `${cpu}核` })
  if (memoryMB > 0) {
    // compute.memory 以 MB 存（见 specatom 注释），小于 1G 才按 MB 展示
    out.push({ label: memoryMB >= 1024 ? `${Math.round(memoryMB / 1024)}G` : `${memoryMB}M` })
  }
  if (diskGB > 0) out.push({ label: diskType ? `${diskGB}G ${diskType.toUpperCase()}` : `${diskGB}G` })
  if (bandwidth > 0) out.push({ label: `${bandwidth}M 带宽` })
  return out
}

/**
 * 生成一个随机主机名：与后端 safeHostname 规则一致（字母开头 + 小写字母数字连字符）。
 *
 * 后缀取 8 位而不是 6 位：带 hs- 前缀共 11 位，满足上游「主机名至少 10 位」的校验；
 * 6 位时总长只有 9 位，提交后会被上游以「主机名长度至少10位」拒绝，白扣一次款。
 * 抽成独立函数供「开窗预填」与「点随机」两处共用，避免两边规则漂移。
 */
function randomHostnameValue(): string {
  return `hs-${Math.random().toString(36).slice(2, 10)}`
}

/** 换一个随机主机名（客服对预填的名字不满意时点「换一个」）。 */
function randomHostname(opt: AdminOrderConfigOption) {
  setSelectionValue(opt, randomHostnameValue())
}

/** 提交给后端的选配：丢掉空值，避免把「未选」当作显式空值下发。 */
function cleanSelections(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(selection.value)) {
    if (v !== undefined && v !== '') out[k] = String(v)
  }
  return out
}

function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '代理折扣',
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
    bound: '绑定续费价',
  }
  return map[source] || source || '优惠'
}

function cycleLabel(c: string): string {
  const map: Record<string, string> = {
    hourly: '按小时',
    daily: '按天',
    monthly: '月付',
    quarterly: '季付',
    semiannually: '半年付',
    annually: '年付',
    biennially: '两年付',
    triennially: '三年付',
    onetime: '一次性',
  }
  return map[c] || c
}

function money(v: number): string {
  return (Math.round(v * 100) / 100).toFixed(2)
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  if (!form.product_id) {
    MessagePlugin.warning('请选择商品')
    return
  }
  if (config.value && config.value.specs.length && !form.spec_code) {
    MessagePlugin.warning('请选择规格')
    return
  }
  if (missingRequired.value.length) {
    MessagePlugin.warning(`请选择：${missingRequired.value.map((o) => o.name).join('、')}`)
    return
  }
  submitting.value = true
  try {
    const selections = cleanSelections()
    await createUserOrder(props.userId, {
      product_id: form.product_id,
      billing_cycle: form.billing_cycle || undefined,
      spec_code: form.spec_code || undefined,
      quantity: form.quantity,
      price: form.price && form.price > 0 ? form.price : undefined,
      renewal_price: form.renewal_price && form.renewal_price > 0 ? form.renewal_price : undefined,
      pay_mode: form.pay_mode,
      config_selections: Object.keys(selections).length ? selections : undefined,
    })
    MessagePlugin.success(form.pay_mode === 'balance' ? '已支付并投递开通任务' : '订单已创建（待支付）')
    visible.value = false
    emit('saved')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '创建订单失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
/*
  内容很长时由 dialog 的 body 滚动，底部按钮不被推出屏幕。
  TDesign 的 dialog 默认把整块放进视口并按内容撑高，这里给内容区一个上限，
  超出就在内部滚 —— 弹窗本体的高度因此不会再超过视口。
*/
:deep(.t-dialog__body) {
  max-height: calc(100vh - 220px);
  overflow-y: auto;
  overscroll-behavior: contain;
}

.configure-order {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* 顶部两项并排：都不是「一次要选很多」的字段，各占一半即可，少占一屏高度 */
.configure-order__head {
  display: flex;
  gap: 16px;
  margin-bottom: 14px;
}

.configure-order__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 220px;
}

.configure-order__field--grow {
  flex: 1;
  width: auto;
}

.configure-order__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.configure-order__section {
  margin-bottom: 16px;
}

.configure-order__title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.configure-order__title-hint {
  font-size: 12px;
  font-style: normal;
  font-weight: 400;
  color: var(--color-muted-foreground, #6b7280);
}

/* 规格卡片：选中态用主色描边，原子取值用小圆点分列 */
.configure-order__specs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(196px, 1fr));
  gap: 10px;
}

.configure-order__spec {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke, #dcdcdc);
  border-radius: 8px;
  background: var(--td-bg-color-container, #fff);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.2s,
    box-shadow 0.2s;
}

.configure-order__spec:hover {
  border-color: var(--td-brand-color-light, #a6c8ff);
}

.configure-order__spec.is-active {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: inset 0 0 0 1px var(--td-brand-color, #0052d9);
}

.configure-order__spec-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.configure-order__spec-atoms {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 6px;
}

.configure-order__spec-atom {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--td-bg-color-container-hover, #f3f3f3);
  font-size: 11px;
  color: var(--td-text-color-secondary);
}

.configure-order__spec-foot {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.configure-order__spec-price {
  font-style: normal;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-error-color-6, #d54941);
}

.configure-order__stock {
  font-style: normal;
  font-size: 11px;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__soldout {
  font-style: normal;
  font-size: 11px;
  color: var(--td-error-color-6);
}

.configure-order__empty {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}

/*
  配置项多列铺开：弹窗加宽后每列至少 300px，桌面端 1180px 可并 3 列，
  比单列纵向堆叠矮得多（用户诉求是"别一直往下滚"）。
  窄屏回落单列（见文件末尾 media query）。
*/
.configure-order__options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}

.configure-order__option {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke, #dcdcdc);
  border-radius: 8px;
  background: var(--td-bg-color-container, #fff);
}

.configure-order__option-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.configure-order__option-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.configure-order__required {
  margin-left: 6px;
  font-style: normal;
  font-size: 11px;
  color: var(--td-error-color-6);
}

.configure-order__unit {
  margin-left: 4px;
  font-style: normal;
  font-size: 11px;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__option-add {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-warning-color-6, #d97706);
}

.configure-order__option-add-inline {
  margin-left: 4px;
  font-style: normal;
  font-size: 11px;
  color: var(--td-warning-color-6, #d97706);
}

.configure-order__option-help {
  margin: 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground, #6b7280);
}

/*
  数量型控件：滑块 + 数值输入上下排布（与用户侧同形态）。
*/
.configure-order__qty {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.configure-order__slider {
  width: 100%;
}
.configure-order__qty-input {
  display: flex;
  align-items: center;
  gap: 8px;
}
.configure-order__qty-input :deep(.t-input-number) {
  width: 150px;
}
.configure-order__qty-unit {
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}
.configure-order__qty-range {
  font-size: 11.5px;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__text {
  display: flex;
  align-items: center;
  gap: 8px;
}

.configure-order__fixed {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--td-bg-color-container-hover, #f5f6f8);
  font-size: 12px;
}

.configure-order__fixed-item strong {
  color: var(--td-text-color-primary);
}

.configure-order__row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.configure-order__row-label {
  min-width: 170px;
  font-size: 13px;
  color: var(--td-text-color-primary);
}

.configure-order__row-hint {
  display: block;
  font-style: normal;
  font-size: 11px;
  color: var(--color-muted-foreground, #6b7280);
}

/* 价格区：左侧默认价、右侧两个输入，用底色与其它区块分开 */
.configure-order__section--price {
  margin-bottom: 4px;
}

.configure-order__price-box {
  display: flex;
  gap: 16px;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--td-bg-color-container-hover, #f5f6f8);
}

.configure-order__price-base {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 190px;
}

.configure-order__price-base-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.configure-order__price-base-label {
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__price-base-value {
  font-size: 20px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.configure-order__price-base-note {
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__price-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  flex: 1;
}

.configure-order__price-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.configure-order__price-field-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.configure-order__price-field-tag {
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 3px;
  background: var(--td-brand-color-light, #e3f0ff);
  font-style: normal;
  font-size: 10px;
  font-weight: 400;
  color: var(--td-brand-color, #0052d9);
}

.configure-order__price-field-hint {
  font-style: normal;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__warn {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--td-error-color-6);
}

/* 窄屏：配置项回落单列，弹窗内容区继续内部滚动（弹窗宽度已由 isMobile 控制）。 */
@media (max-width: 768px) {
  .configure-order__options {
    grid-template-columns: 1fr;
  }
  .configure-order__head {
    flex-direction: column;
    align-items: stretch;
  }
  .configure-order__price-field {
    flex-wrap: wrap;
  }
  :deep(.t-dialog__body) {
    max-height: calc(100vh - 180px);
  }
}
</style>
