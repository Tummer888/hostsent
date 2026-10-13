<template>
  <t-dialog
    v-model:visible="visible"
    header="为用户创建订单"
    width="760px"
    :confirm-btn="confirmBtn"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <div class="configure-order">
      <!-- 1. 选择商品：切换商品要重新拉取规格/周期/选配项 -->
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="下单用户">
          <t-input :model-value="username || '—'" disabled />
        </t-form-item>
        <t-form-item label="选择商品" name="product_id">
          <t-select
            v-model="form.product_id"
            :options="productOptions"
            filterable
            placeholder="请选择商品"
            :loading="productsLoading"
            @change="handleProductChange"
          />
        </t-form-item>
      </t-form>
      <p class="configure-order__hint">
        商品选定后下方按该商品实际可售的规格 / 周期 / 配置项渲染；不再支持选择平台上不存在或
        运营已下架的配置，避免开出来的机器与售卖口径不一致。
      </p>

      <t-loading :loading="configLoading" size="small" show-overlay>
        <!-- 2. 规格（SKU）：商品挂了规格就必须选，未选后端会拒绝下单 -->
        <section v-if="config" class="configure-order__section">
          <h4 class="configure-order__title">规格</h4>
          <t-radio-group v-if="specs.length" v-model="form.spec_code" variant="default-filled">
            <t-radio-button v-for="sp in specs" :key="sp.spec_code" :value="sp.spec_code">
              {{ sp.name }}
              <em class="configure-order__price">¥{{ money(sp.price) }}</em>
              <em v-if="sp.stock === 0" class="configure-order__soldout">已售罄</em>
            </t-radio-button>
          </t-radio-group>
          <p v-else class="configure-order__empty">该商品未挂规格，按商品级价格下单。</p>
        </section>

        <!-- 3. 计费周期：来自周期价格矩阵，未维护周期的商品只给默认档 -->
        <section v-if="cycleChoices.length" class="configure-order__section">
          <h4 class="configure-order__title">计费周期</h4>
          <t-radio-group v-model="form.billing_cycle" variant="default-filled">
            <t-radio-button v-for="c in cycleChoices" :key="c" :value="c">{{ cycleLabel(c) }}</t-radio-button>
          </t-radio-group>
        </section>

        <!-- 4. 配置项：控件类型由后端下发（下拉/分组下拉/单选/步进/开关/文本） -->
        <section v-if="options.length" class="configure-order__section">
          <h4 class="configure-order__title">
            配置选项
            <em class="configure-order__title-hint">不改动即按档位默认配置</em>
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

              <!-- 分组下拉：操作系统按镜像家族（CentOS / Debian / Ubuntu） -->
              <t-select
                v-if="opt.widget === 'group_select'"
                :model-value="selection[opt.option_key] || undefined"
                :placeholder="placeholder(opt)"
                :clearable="!opt.required"
                filterable
                @change="(v: SelectValue) => setSelection(opt, v)"
              >
                <t-option-group v-for="g in groupedValues(opt)" :key="g.label" :label="g.label" divider>
                  <t-option
                    v-for="it in g.items"
                    :key="it.value"
                    :value="it.value"
                    :label="itemLabel(opt, it)"
                  />
                </t-option-group>
              </t-select>

              <!-- 普通下拉 -->
              <t-select
                v-else-if="opt.widget === 'select'"
                :model-value="selection[opt.option_key] || undefined"
                :placeholder="placeholder(opt)"
                :clearable="!opt.required"
                filterable
                @change="(v: SelectValue) => setSelection(opt, v)"
              >
                <t-option
                  v-for="it in opt.values"
                  :key="it.value"
                  :value="it.value"
                  :label="itemLabel(opt, it)"
                />
              </t-select>

              <!-- 单选按钮组 -->
              <t-radio-group
                v-else-if="opt.widget === 'radio'"
                :model-value="selection[opt.option_key]"
                variant="default-filled"
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

              <!-- 步进器：超出档位内含部分才加价 -->
              <div v-else-if="opt.widget === 'qty'" class="configure-order__qty">
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

              <!-- 文本：主机名等自由填写项，留空由系统生成 -->
              <div v-else-if="opt.widget === 'text'" class="configure-order__text">
                <t-input
                  :model-value="selection[opt.option_key] || ''"
                  :placeholder="placeholder(opt)"
                  clearable
                  @change="(v: string) => setSelectionValue(opt, v)"
                />
                <t-button v-if="opt.option_key === 'hostname'" size="small" variant="outline" @click="randomHostname(opt)">
                  随机
                </t-button>
              </div>

              <p v-if="optionHelp(opt)" class="configure-order__option-help">{{ optionHelp(opt) }}</p>
            </div>
          </div>
          <p v-if="missingRequired.length" class="configure-order__warn">
            请选择：{{ missingRequired.map((o) => o.name).join('、') }}
          </p>
        </section>

        <!-- 5. 上游固定配置：只读，说明「这些项定死了，改也开不出来」 -->
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

        <section class="configure-order__section">
          <h4 class="configure-order__title">数量与支付</h4>
          <div class="configure-order__row">
            <span class="configure-order__row-label">数量</span>
            <t-input-number v-model="form.quantity" :min="1" :max="99" theme="normal" />
          </div>
          <div class="configure-order__row">
            <span class="configure-order__row-label">
              开通方式
              <em class="configure-order__row-hint">
                仅创建：只落待支付订单，用户自己去付款；<br>
                余额支付并开通：从用户余额扣款后立即投递开通任务。
              </em>
            </span>
            <t-radio-group v-model="form.pay_mode" variant="default-filled">
              <t-radio-button value="create">创建订单（待支付）</t-radio-button>
              <t-radio-button value="balance">余额支付并开通</t-radio-button>
            </t-radio-group>
          </div>
          <div class="configure-order__row">
            <span class="configure-order__row-label">
              自定义价格（元）
              <em class="configure-order__row-hint">
                填了即按此价成交（不参与算价）；留空按商品周期价与折扣算。
                这里是首次开通价，续费价取商品的周期价格矩阵（产品管理 → 定价）。
              </em>
            </span>
            <t-input-number
              v-model="form.price"
              :min="0"
              :precision="2"
              theme="normal"
              placeholder="留空按商品价与折扣算"
            />
          </div>
        </section>
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
 */
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createUserOrder,
  getAdminOrderConfig,
  type AdminOrderConfigOption,
  type AdminOrderConfigOptionVal,
  type AdminOrderConfigResponse,
} from '@/api/user'
import { getProductList as getUcProductList } from '@/api/product'

type SelectValue = string | number | boolean

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
const productOptions = ref<{ label: string; value: number }[]>([])
const config = ref<AdminOrderConfigResponse | null>(null)
/** 配置项选择：option_key → 取值（下发参数统一按字符串存）。 */
const selection = ref<Record<string, string>>({})

const form = reactive<{
  product_id: number | undefined
  spec_code: string
  billing_cycle: string
  quantity: number
  price: number
  pay_mode: 'create' | 'balance'
}>({
  product_id: undefined,
  spec_code: '',
  billing_cycle: '',
  quantity: 1,
  price: 0,
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
const lockedUpstream = computed(() =>
  (config.value?.upstream?.options ?? []).filter((o) => o.locked),
)
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
  form.product_id = undefined
  form.spec_code = ''
  form.billing_cycle = ''
  form.quantity = 1
  form.price = 0
  form.pay_mode = 'create'
  config.value = null
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

watch(() => form.spec_code, () => syncCycleForSpec())

/** 默认选择：必选项回填默认值/首个取值，数量型填默认数量。 */
function applyDefaultSelection() {
  const next: Record<string, string> = {}
  for (const opt of options.value) {
    if (opt.widget === 'qty') {
      next[opt.option_key] = String(numberValue(opt))
      continue
    }
    if (opt.widget === 'text') continue
    if (!opt.required) continue
    const preferred =
      opt.values.find((v) => v.value === opt.default_value) ??
      opt.values.find((v) => v.is_default) ??
      opt.values[0]
    if (preferred) next[opt.option_key] = preferred.value
  }
  selection.value = next
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

function boolValues(opt: AdminOrderConfigOption): SelectValue[] {
  const values = opt.values.map((v) => v.value)
  if (values.length >= 2) return [values[0], values[1]]
  return [0, 1]
}

function numberValue(opt: AdminOrderConfigOption): number {
  const raw = selection.value[opt.option_key]
  if (raw !== undefined && raw !== '') {
    const n = Number(raw)
    if (!Number.isNaN(n)) return n
  }
  const def = Number(opt.default_value)
  if (opt.default_value !== '' && !Number.isNaN(def)) return def
  return opt.min_value ?? 0
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
 * 随机主机名：与后端 safeHostname 规则一致（字母开头 + 小写字母数字连字符）。
 *
 * 后缀取 8 位而不是 6 位：带 hs- 前缀共 11 位，满足上游「主机名至少 10 位」的校验；
 * 6 位时总长只有 9 位，提交后会被上游以「主机名长度至少10位」拒绝，白扣一次款。
 */
function randomHostname(opt: AdminOrderConfigOption) {
  const suffix = Math.random().toString(36).slice(2, 10)
  setSelectionValue(opt, `hs-${suffix}`)
}

/** 提交给后端的选配：丢掉空值，避免把「未选」当作显式空值下发。 */
function cleanSelections(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(selection.value)) {
    if (v !== undefined && v !== '') out[k] = String(v)
  }
  return out
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
      price: form.price > 0 ? form.price : undefined,
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
.configure-order {
  display: flex;
  flex-direction: column;
  gap: var(--space-md, 12px);
}

.configure-order__hint {
  margin: 0 0 4px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__section {
  margin-bottom: 18px;
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

.configure-order__price {
  margin-left: 6px;
  font-style: normal;
  font-weight: 600;
}

.configure-order__soldout {
  margin-left: 6px;
  font-style: normal;
  font-size: 12px;
  color: var(--td-error-color-6);
}

.configure-order__empty {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
}

.configure-order__options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}

.configure-order__option {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.configure-order__option-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.configure-order__option-name {
  font-size: 13px;
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

.configure-order__qty,
.configure-order__text {
  display: flex;
  align-items: center;
  gap: 8px;
}

.configure-order__qty-unit {
  font-size: 12px;
  color: var(--color-muted-foreground, #6b7280);
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

.configure-order__warn {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--td-error-color-6);
}
</style>
