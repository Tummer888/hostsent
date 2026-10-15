<template>
  <!--
    对接商品改价编辑器（列表弹窗与编辑页内嵌共用同一份实现）。

    为什么必须按周期改，而不是"填一个销售价":
      上游把「付款周期 × 价格」绑定在一起下发（月付/季付/年付各是一个价，
      且上游已按周期给了折扣）。本地只改一个单价会让季付/年付档位用旧价
      或算出与上游对不上的金额。因此这里的两个方案都落在周期价格矩阵上。
  -->
  <div class="upstream-price-editor">
    <!-- 上游价格基线：让运营看清"按什么在算"（成本/标价都是上游下发） -->
    <t-alert v-if="upstream" theme="info" class="upe-alert">
      <template #message>
        <div class="upe-baseline">
          <span>
            上游标价
            <b>{{ upstream.list_price > 0 ? `¥${formatPrice(upstream.list_price)}` : '未定价' }}</b>
            <span v-if="upstream.discount_desc" class="upe-muted">（{{ upstream.discount_desc }}）</span>
          </span>
          <span>
            代理价（采购成本）
            <b>{{ upstream.cost_price > 0 ? `¥${formatPrice(upstream.cost_price)}` : '—' }}</b>
          </span>
          <span v-if="!upstream.renewable" class="upe-warn">{{ upstream.renew_note }}</span>
        </div>
      </template>
    </t-alert>

    <t-tabs v-model="activeMode" theme="normal">
      <!-- ============ 方案一：百分比（跟着上游走） ============ -->
      <t-tab-panel value="percent" label="按百分比（跟随上游）">
        <div class="upe-pane">
          <p class="upe-hint">
            设定一个加价规则，<b>上游以后调价时各周期售价自动按规则重算</b>。 适合"永远按上游价加上固定利润卖"。
          </p>

          <div class="upe-field">
            <span class="upe-label">加价口径</span>
            <t-radio-group v-model="markupType" variant="default-filled" class="upe-radio">
              <t-radio-button value="list_percent">按上游标价</t-radio-button>
              <t-radio-button value="percent">按代理价</t-radio-button>
              <t-radio-button value="fixed">代理价 + 固定额</t-radio-button>
            </t-radio-group>
          </div>

          <div class="upe-field">
            <span class="upe-label">{{ valueLabel }}</span>
            <div class="upe-value-row">
              <t-input-number
                v-model="markupValue"
                class="upe-input"
                :min="0"
                :precision="2"
                theme="column"
                :suffix="markupType === 'fixed' ? '元' : '%'"
              />
              <t-space v-if="markupType !== 'fixed'" size="small" class="upe-quick">
                <t-button
                  v-for="preset in percentPresets"
                  :key="preset"
                  size="small"
                  :theme="markupValue === preset ? 'primary' : 'default'"
                  :variant="markupValue === preset ? 'base' : 'outline'"
                  @click="markupValue = preset"
                  >{{ preset }}%</t-button
                >
              </t-space>
            </div>
            <span class="upe-tip">{{ percentHint }}</span>
          </div>

          <!-- 各周期预览：按上游成本 × 规则算出的售价与毛利 -->
          <div class="upe-preview">
            <div class="upe-preview__head">
              <span>各周期售价预览</span>
              <span class="upe-muted">按当前上游价格计算，保存后即为对客价</span>
            </div>
            <t-table
              v-if="previewRows.length"
              row-key="cycle"
              :data="previewRows"
              :columns="previewColumns"
              size="small"
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #cycle="{ row }">
                <div class="upe-cycle">
                  <span class="upe-cycle__name">{{ row.cycle_name }}</span>
                  <span v-if="row.months" class="upe-muted">{{ row.months }} 个月</span>
                </div>
              </template>
              <template #price="{ row }">
                <span class="upe-price">{{ row.price > 0 ? `¥${formatPrice(row.price)}` : '—' }}</span>
              </template>
              <template #margin="{ row }">
                <span v-if="row.cost > 0 && row.price > 0" :class="row.price >= row.cost ? 'upe-up' : 'upe-down'">
                  {{ marginText(row) }}
                </span>
                <span v-else class="upe-muted">—</span>
              </template>
            </t-table>
            <t-empty v-else description="该商品还没有可售周期价格（请先在上游商品页同步，或用第二个方案手工定价）" />
          </div>
        </div>
      </t-tab-panel>

      <!-- ============ 方案二：自定义（本次定价，逐周期） ============ -->
      <t-tab-panel value="custom" label="自定义各周期价">
        <div class="upe-pane">
          <p class="upe-hint">
            逐周期填价，<b>只对改过的档位生效</b>（未改动的档位继续跟随上游）；
            改过的档位会成为<b>人工价</b>，上游以后调价不再覆盖它。
          </p>

          <!-- 快速填入：一次算出所有周期价，再逐格微调 -->
          <div class="upe-gen">
            <div class="upe-gen__head">
              <span class="upe-label">快速填入</span>
              <span class="upe-muted">先按月付价 + 优惠月数算一遍，再逐格微调</span>
            </div>
            <div class="upe-gen__row">
              <label class="upe-gen__field">
                <span>月付价（元）</span>
                <t-input-number v-model="genMonthly" :min="0" :precision="2" theme="column" />
              </label>
              <label class="upe-gen__field">
                <span>季付优惠（月）</span>
                <t-input-number v-model="genQuarterlyOff" :min="0" :max="2" theme="column" />
              </label>
              <label class="upe-gen__field">
                <span>半年付优惠（月）</span>
                <t-input-number v-model="genSemiannuallyOff" :min="0" :max="5" theme="column" />
              </label>
              <label class="upe-gen__field">
                <span>年付优惠（月）</span>
                <t-input-number v-model="genAnnuallyOff" :min="0" :max="11" theme="column" />
              </label>
              <t-button size="small" theme="primary" variant="outline" @click="applyQuickFill">一键填入</t-button>
            </div>
            <div class="upe-gen__row">
              <label class="upe-gen__field">
                <span>或按折扣（%）</span>
                <t-input-number v-model="genDiscount" :min="1" :max="100" theme="column" />
              </label>
              <span class="upe-gen__note">
                按「月付价 × 月数 × 折扣」填（如 90 = 长周期打 9 折）；
                与优惠月数二选一，点「一键填入」时以二者都填则先按优惠月数。
              </span>
              <t-button size="small" variant="outline" @click="applyDiscountFill">按折扣填入</t-button>
            </div>
          </div>

          <t-table
            v-if="rows.length"
            row-key="cycle"
            :data="rows"
            :columns="columns"
            size="small"
            table-layout="fixed"
            cell-empty-content="—"
          >
            <template #cycle="{ row }">
              <div class="upe-cycle">
                <span class="upe-cycle__name">{{ row.cycle_name }}</span>
                <t-tag v-if="row.source === 'manual'" theme="warning" variant="light" size="small" shape="round"
                  >人工</t-tag
                >
                <t-tag v-else-if="row.source" theme="primary" variant="light" size="small" shape="round"
                  >跟随上游</t-tag
                >
              </div>
            </template>
            <template #price="{ row }">
              <t-input-number v-model="row.price" :min="0" :precision="2" theme="normal" size="small" />
            </template>
            <template #cost="{ row }">
              <span class="upe-muted">{{ row.cost_price > 0 ? `¥${formatPrice(row.cost_price)}` : '—' }}</span>
            </template>
            <template #margin="{ row }">
              <span
                v-if="row.cost_price > 0 && row.price > 0"
                :class="row.price >= row.cost_price ? 'upe-up' : 'upe-down'"
              >
                {{ marginText(row) }}
              </span>
              <span v-else class="upe-muted">—</span>
            </template>
            <template #status="{ row }">
              <t-switch v-model="row.enabled" size="small" />
            </template>
          </t-table>
          <t-empty v-else description="加载中或该商品尚未建立周期价格（先同步上游商品目录）" />
        </div>
      </t-tab-panel>
    </t-tabs>

    <!-- 同步到已开通主机：只影响「已绑定续费价」的实例 -->
    <div class="upe-sync">
      <div class="upe-sync__head">
        <t-checkbox v-model="syncRenewal">同时同步到已开通主机</t-checkbox>
        <span class="upe-muted">
          只影响<b>已绑定续费价</b>的实例（它们不会自动跟随商品调价）；未绑定的实例本来就实时跟随。
        </span>
      </div>
      <div v-if="syncRenewal" class="upe-sync__body">
        <t-radio-group v-model="syncMode" variant="default-filled" class="upe-radio">
          <t-radio-button value="update">按新价更新绑定</t-radio-button>
          <t-radio-button value="clear">清除绑定（改为跟随商品价）</t-radio-button>
        </t-radio-group>
        <div class="upe-sync__probe">
          <t-button size="small" variant="outline" :loading="probing" @click="probe"> 先看影响几台 </t-button>
          <span v-if="probeResult" class="upe-sync__result">{{ probeResult }}</span>
        </div>
      </div>
    </div>

    <div v-if="!embedded" class="upe-footer">
      <t-button variant="outline" @click="$emit('cancel')">取消</t-button>
      <t-button theme="primary" :loading="saving" @click="handleSave">{{ saveLabel }}</t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'

import { applyUpstreamMarkup } from '@/api/admin'
import { getProductPriceMatrix, saveProductPriceMatrix, syncProductRenewalPrice } from '@/api/product'
import { billingCycleLabel } from '@/pages/product/constants'
import type { SaleProductUpstreamInfo } from '@/types/product'

defineOptions({ name: 'UpstreamPriceEditor' })

const props = defineProps<{
  /** 销售商品 ID（写价格矩阵与续费价同步都用它） */
  productId: number
  /** 上游对接信息（含各周期成本/标价与折扣说明）；列表页没有时可为空 */
  upstream?: SaleProductUpstreamInfo | null
  /** 商品名（确认文案用） */
  productName?: string
  /**
   * 上游资源商品 ID（products.source_product_id）。
   *
   * 加价规则接口按**上游资源商品**定位（一个上游商品的加价规则作用于由它导入的
   * 全部销售商品），与商品自身的 id 不是一回事，必须单独传。
   */
  sourceProductId?: number
  /**
   * 内嵌到编辑页：隐藏自带的底部按钮，由父页的「保存」统一触发 save()。
   * 独立弹窗（列表「改价」）保持默认 false，自己带按钮。
   */
  embedded?: boolean
  /**
   * 商品当前已保存的加价规则（回显用）。
   *
   * 必须传：编辑器原来恒以「按上游标价 100%」开局，看不到现状；现在与基础信息合并成
   * 一个保存按钮后，这个默认值还会被当成"用户的选择"提交，把没动过价格的商品重算一遍。
   * 传入真实规则 + 脏检查（见 dirty）才能真正回显且只保存改动。
   */
  initialMarkupType?: string
  initialMarkupValue?: number
}>()

const emit = defineEmits<{
  (e: 'saved'): void
  (e: 'cancel'): void
}>()

const activeMode = ref<'percent' | 'custom'>('percent')

/** 加价规则的作用对象：上游资源商品 ID（不是本地商品 ID）。 */
const markupResourceProductID = computed(() => Number(props.sourceProductId) || 0)
const saving = ref(false)

// ===== 方案一：百分比（加价规则） =====
const markupType = ref<'list_percent' | 'percent' | 'fixed'>('list_percent')
const markupValue = ref(100)
const percentPresets = [100, 105, 110, 120, 130]

/**
 * 已保存规则的快照（`类型:数值`）。是否改过用签名比对，不挂 watch。
 *
 * 不能用 watch 标记脏：回显时 initMarkupRule 会写这两个 ref，watch 的回调是异步的，
 * 会在初始化之后才触发，于是"打开编辑页什么都没动"也被判成改过 ——
 * 合并保存时就会把这个默认规则提交上去，把没动过价格的商品重算一遍。
 * 签名比对还天然幂等：改了又改回去 = 没改。
 */
const markupSnapshot = ref('list_percent:100')
const markupDirty = computed(() => markupSignature() !== markupSnapshot.value)

function markupSignature(): string {
  return `${markupType.value}:${Number(markupValue.value) || 0}`
}

/** 用商品已保存的规则初始化回显（没有规则时保持默认的「按上游标价 100%」）。 */
function initMarkupRule() {
  const t = (props.initialMarkupType || '').trim()
  const v = Number(props.initialMarkupValue) || 0
  if (t === 'percent' || t === 'fixed' || t === 'list_percent') {
    markupType.value = t
    markupValue.value = v
  } else {
    markupType.value = 'list_percent'
    markupValue.value = 100
  }
  markupSnapshot.value = markupSignature()
}

const valueLabel = computed(() => {
  switch (markupType.value) {
    case 'list_percent':
      return '按上游标价的比例（100 = 按上游原价）'
    case 'percent':
      return '按代理价的比例（%）'
    default:
      return '在代理价上加（元）'
  }
})

const percentHint = computed(() => {
  switch (markupType.value) {
    case 'list_percent':
      return '售价 = 上游标价 × N%。100 即按上游原价卖，>100 加价，<100 让利。'
    case 'percent':
      return '售价 = 代理价（我方采购成本）× N%。'
    default:
      return '售价 = 代理价 + N 元（固定额只加一次，不随周期放大）。'
  }
})

/** 各周期成本（我方采购价）：来自上游镜像的 cost_cycles。 */
const cycleCosts = computed<Record<string, number>>(() => props.upstream?.cost_cycles || {})
/** 各周期上游标价：list_percent 口径的基数。 */
const cycleLists = computed<Record<string, number>>(() => props.upstream?.list_cycles || {})

/** 周期月数（用于展示与"按折扣填入"）。 */
const CYCLE_MONTHS: Record<string, number> = {
  hourly: 0,
  daily: 0,
  monthly: 1,
  quarterly: 3,
  semiannually: 6,
  annually: 12,
  biennially: 24,
  triennially: 36,
  onetime: 0,
}

const CYCLE_ORDER = ['monthly', 'quarterly', 'semiannually', 'annually', 'biennially', 'triennially', 'onetime']

/** 与后端 applyMarkupWithList 同口径算价（预览要和后端算出的一致，否则保存后数字会跳）。 */
function calcPrice(cost: number, list: number): number {
  switch (markupType.value) {
    case 'list_percent': {
      const base = list > 0 ? list : cost
      return round2((base * markupValue.value) / 100)
    }
    case 'percent':
      return round2((cost * markupValue.value) / 100)
    default:
      return round2(cost + markupValue.value)
  }
}

function round2(v: number): number {
  return Math.round(v * 100) / 100
}

const previewRows = computed(() => {
  const cycles = CYCLE_ORDER.filter((c) => (cycleCosts.value[c] ?? 0) > 0 || (cycleLists.value[c] ?? 0) > 0)
  return cycles.map((cycle) => {
    const cost = cycleCosts.value[cycle] ?? 0
    const price = calcPrice(cost, cycleLists.value[cycle] ?? 0)
    return {
      cycle,
      cycle_name: billingCycleLabel(cycle),
      months: CYCLE_MONTHS[cycle] ?? 0,
      cost,
      price,
    }
  })
})

const previewColumns = [
  { colKey: 'cycle', title: '计费周期', width: 150 },
  { colKey: 'price', title: '售价', width: 130 },
  { colKey: 'margin', title: '毛利（售价 − 采购价）', minWidth: 160 },
]

function marginText(row: { price: number; cost?: number; cost_price?: number }): string {
  const cost = row.cost ?? row.cost_price ?? 0
  if (cost <= 0 || row.price <= 0) return '—'
  const diff = round2(row.price - cost)
  const rate = Math.round((diff / row.price) * 100)
  return `¥${formatPrice(diff)}（${rate}%）`
}

function formatPrice(v: number): string {
  return Number(v || 0).toFixed(2)
}

// ===== 方案二：自定义各周期价 =====
type PriceRow = {
  cycle: string
  cycle_name: string
  /** 当前售价（落库值） */
  price: number
  /** 采购成本（只读展示） */
  cost_price: number
  /** 值来源：upstream/markup=跟随上游，manual=人工价 */
  source: string
  /** 是否启用该档 */
  enabled: boolean
  setup_fee: number
  cost_setup_fee: number
}

const rows = ref<PriceRow[]>([])
const loading = ref(false)

/**
 * 逐档基线：记录加载时每一档的价格与启停，用于判断"运营真正改过哪几档"。
 *
 * 为什么不能只跟整体快照比：两个页签都在同一份数据上，而保存加价规则会
 * **让后端重算派生行的价格**。若运营同时改了规则与某档价格，先存的规则会把该档
 * 也重算一遍；此时再整表提交，前端手里那份（规则重算前读到的）旧值就会被当成
 * "运营显式改价"写进人工行，把规则刚算出的价覆盖掉 —— 运营的改动看着生效、
 * 实际被自己覆盖。逐档比对后只提交**确实被手改过**的档位，就不会误伤。
 */
const priceBaseline = ref<Map<string, { price: number; enabled: boolean }>>(new Map())

/** 真正被手改过的档位（价格或启停与基线不同）。 */
const editedCycles = computed(() => {
  const out: PriceRow[] = []
  for (const row of rows.value) {
    const base = priceBaseline.value.get(row.cycle)
    if (!base) continue
    if ((Number(row.price) || 0) !== base.price || row.enabled !== base.enabled) out.push(row)
  }
  return out
})

const matrixDirty = computed(() => editedCycles.value.length > 0)

const columns = [
  { colKey: 'cycle', title: '计费周期', width: 150 },
  { colKey: 'price', title: '售价（元）', width: 150 },
  { colKey: 'cost', title: '采购成本', width: 110 },
  { colKey: 'margin', title: '毛利', minWidth: 130 },
  { colKey: 'status', title: '启用', width: 80 },
]

/** 加载周期价格矩阵。 */
async function loadMatrix() {
  if (!props.productId) return
  loading.value = true
  try {
    const matrix = await getProductPriceMatrix({ product_id: props.productId })
    rows.value = (matrix.items || [])
      .slice()
      .sort((a, b) => CYCLE_ORDER.indexOf(a.cycle) - CYCLE_ORDER.indexOf(b.cycle))
      .map((item) => ({
        cycle: item.cycle,
        cycle_name: item.cycle_name || billingCycleLabel(item.cycle),
        price: Number(item.price) || 0,
        cost_price: Number(item.cost_price) || 0,
        source: item.source || '',
        enabled: item.status === 1,
        setup_fee: Number(item.setup_fee) || 0,
        cost_setup_fee: Number(item.cost_setup_fee) || 0,
      }))
    priceBaseline.value = new Map(rows.value.map((r) => [r.cycle, { price: Number(r.price) || 0, enabled: r.enabled }]))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载周期价格失败')
  } finally {
    loading.value = false
  }
}

// ===== 快速填入 =====
const genMonthly = ref(0)
const genQuarterlyOff = ref(0)
const genSemiannuallyOff = ref(1)
const genAnnuallyOff = ref(2)
const genDiscount = ref(100)

/** 优惠月数 → 该周期价：月价 × 月数 − 优惠月数 × 月价（优惠月数上限 = 月数 − 1，避免 0 元价）。 */
function fillByOffMonths() {
  const monthly = Number(genMonthly.value) || 0
  if (monthly <= 0) {
    MessagePlugin.warning('请先填入大于 0 的月付价')
    return
  }
  const offMap: Record<string, number> = {
    monthly: 0,
    quarterly: Number(genQuarterlyOff.value) || 0,
    semiannually: Number(genSemiannuallyOff.value) || 0,
    annually: Number(genAnnuallyOff.value) || 0,
  }
  let filled = 0
  for (const row of rows.value) {
    const months = CYCLE_MONTHS[row.cycle] ?? 0
    if (months <= 0) continue
    const off = Math.max(0, Math.min(offMap[row.cycle] ?? 0, months - 1))
    row.price = round2(monthly * months - monthly * off)
    filled++
  }
  if (filled === 0) MessagePlugin.warning('没有可填入的周期档位')
}

function applyQuickFill() {
  // 折扣填了且优惠月数全为 0 时按折扣；否则优先优惠月数（运营更常用"省几个月"的表达）。
  const hasOff = [genQuarterlyOff.value, genSemiannuallyOff.value, genAnnuallyOff.value].some((v) => Number(v) > 0)
  if (!hasOff && Number(genDiscount.value) > 0 && Number(genDiscount.value) !== 100) {
    applyDiscountFill()
    return
  }
  fillByOffMonths()
}

/** 折扣填法：月价 × 月数 × 折扣%。 */
function applyDiscountFill() {
  const monthly = Number(genMonthly.value) || 0
  if (monthly <= 0) {
    MessagePlugin.warning('请先填入大于 0 的月付价')
    return
  }
  const pct = Number(genDiscount.value) || 100
  if (pct <= 0 || pct > 100) {
    MessagePlugin.warning('折扣需在 1~100 之间（100 = 不打折）')
    return
  }
  let filled = 0
  for (const row of rows.value) {
    const months = CYCLE_MONTHS[row.cycle] ?? 0
    if (months <= 0) continue
    row.price = round2((monthly * months * pct) / 100)
    filled++
  }
  if (filled === 0) MessagePlugin.warning('没有可填入的周期档位')
}

// ===== 同步到已开通主机 =====
const syncRenewal = ref(false)
const syncMode = ref<'update' | 'clear'>('update')
const probing = ref(false)
const probeResult = ref('')

/** 试算：先问后端会影响几台，避免"确认后才发现动了 200 台"。 */
async function probe() {
  if (!props.productId) return
  probing.value = true
  probeResult.value = ''
  try {
    const res = await syncProductRenewalPrice(props.productId, {
      clear: syncMode.value === 'clear',
      dry_run: true,
    })
    probeResult.value = res.message || `将影响 ${res.affected} 台`
  } catch (error) {
    probeResult.value = ''
    MessagePlugin.error((error as Error).message || '试算失败')
  } finally {
    probing.value = false
  }
}

/** 执行同步（保存之后调用；失败不回滚价格，只提示同步失败）。 */
async function runRenewalSync(): Promise<boolean> {
  try {
    const res = await syncProductRenewalPrice(props.productId, {
      clear: syncMode.value === 'clear',
      dry_run: false,
    })
    MessagePlugin.success(res.message || '续费价已同步')
    return true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '续费价同步失败（价格本身已保存）')
    return false
  }
}

// ===== 保存 =====
const saveLabel = computed(() => (activeMode.value === 'percent' ? '保存加价规则' : '保存周期价格'))

/**
 * 独立使用（列表「改价」弹窗）时的保存入口：成功后通知父页刷新。
 * 内嵌到编辑页时父页直接调 save()，与基础信息合成一个保存按钮（见 embedded）。
 */
async function handleSave() {
  const ok = await save()
  if (ok) emit('saved')
}

/**
 * 保存当前方案，返回是否成功。
 *
 * 返回值是必须的：编辑页把「基础信息 + 价格」合并成一个保存按钮，价格存失败时
 * 不能再往下走（否则会出现"提示保存成功、价格其实没存"的错觉）。
 * 具体失败原因由各方法内部提示，这里只归集成败。
 *
 * 内嵌（编辑页）时**只提交真正改过的方案**：两个页签都在同一份数据上，
 * 合并保存不该因为"当前停在哪一屏"就跳过或重复提交。独立弹窗（列表「改价」）里
 * 点按钮本身就是明确动作，照旧提交当前页签。
 */
async function save(): Promise<boolean> {
  if (!props.productId) return false
  // 独立弹窗（列表「改价」）：点按钮本身就是明确动作，照旧提交当前页签。
  if (!props.embedded) {
    saving.value = true
    try {
      const ok = activeMode.value === 'percent' ? await saveMarkupRule() : await saveMatrix()
      if (!ok) return false
      if (syncRenewal.value) await runRenewalSync()
      return true
    } finally {
      saving.value = false
    }
  }
  // 内嵌（编辑页）：只提交真正改过的方案，两个页签都在同一份数据上，
  // 合并保存不该因为"当前停在哪一屏"就跳过或重复提交。
  const hasRuleChange = markupDirty.value
  const hasMatrixChange = matrixDirty.value
  if (!hasRuleChange && !hasMatrixChange) {
    // 价格没动过：不算失败，也不发请求，直接把控制权交回父页存基础信息。
    if (syncRenewal.value) await runRenewalSync()
    return true
  }
  saving.value = true
  try {
    // 两块都改过时：先落规则，再把矩阵重新拉一遍、把运营手改的档位盖回去，最后整表提交。
    //
    // 为什么必须重拉：保存规则会让后端按新规则重算派生行的价格。若直接拿页面上
    // 规则重算**之前**读到的旧值整表提交，那些未手改的档位会因为"与库中现值不同"
    // 被判成显式改价、写成人工行，把规则刚算出的价覆盖掉（看起来规则没生效）。
    // 重拉后未改档位与库一致（保持派生），手改档位仍按运营填的值提交（转人工），两边都对。
    if (hasRuleChange) {
      const ok = await saveMarkupRule()
      if (!ok) return false
      if (hasMatrixChange) {
        const edits = editedCycles.value.map((r) => ({
          cycle: r.cycle,
          price: Number(r.price) || 0,
          enabled: r.enabled,
        }))
        await loadMatrix()
        applyEdits(edits)
      }
    }
    if (hasMatrixChange) {
      const ok = await saveMatrix()
      if (!ok) return false
    }
    if (syncRenewal.value) {
      await runRenewalSync()
    }
    return true
  } finally {
    saving.value = false
  }
}

/** 把运营手改的档位盖回当前表格（规则重算后重拉矩阵时用，避免手改被丢掉）。 */
function applyEdits(edits: { cycle: string; price: number; enabled: boolean }[]) {
  const byCycle = new Map(edits.map((e) => [e.cycle, e]))
  for (const row of rows.value) {
    const hit = byCycle.get(row.cycle)
    if (!hit) continue
    row.price = hit.price
    row.enabled = hit.enabled
  }
}

/**
 * 保存加价规则：走后端的「上游商品加价规则」接口。
 *
 * 注意该接口按**上游资源商品**定位（一个上游商品可对应多个本地商品），
 * 这里用商品详情里的 upstream_id 反查到的资源商品 ID 不适配；
 * 因此改为用 resource_product_id = 商品的 source_product_id。
 */
async function saveMarkupRule() {
  const rpID = markupResourceProductID.value
  if (!rpID) {
    MessagePlugin.warning('取不到该商品绑定的上游商品，无法设置加价规则；请改用「自定义各周期价」')
    return false
  }
  if (markupType.value !== 'fixed' && !(Number(markupValue.value) > 0)) {
    MessagePlugin.warning('请填写大于 0 的加价数值')
    return false
  }
  try {
    const res = await applyUpstreamMarkup({
      resource_product_id: rpID,
      markup_type: markupType.value,
      markup_value: Number(markupValue.value),
      remark: '商品改价（百分比方案）',
    })
    const affected = res?.affected ?? 0
    MessagePlugin.success(affected > 0 ? res.message || `已按规则重算 ${affected} 个商品售价` : '加价规则已保存')
    return true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存加价规则失败')
    return false
  }
}

/** 保存周期矩阵：整表提交（后端按周期 upsert，未提交档位删除，所以必须带上全部档位）。 */
async function saveMatrix(): Promise<boolean> {
  // 内嵌（编辑页）合并保存时，表格没动过就不提交 —— 空提交等于把"当前屏"当成用户意图。
  if (props.embedded && !matrixDirty.value) return true
  const items = rows.value.map((row) => ({
    cycle: row.cycle,
    price: Number(row.price) || 0,
    cost_price: Number(row.cost_price) || 0,
    setup_fee: Number(row.setup_fee) || 0,
    cost_setup_fee: Number(row.cost_setup_fee) || 0,
    status: row.enabled ? 1 : 0,
  }))
  if (items.some((i) => i.status === 1 && i.price <= 0)) {
    MessagePlugin.warning('启用中的周期必须填写大于 0 的售价（0 元价上架后客户真能 0 元买）')
    return false
  }
  try {
    await saveProductPriceMatrix({ product_id: props.productId, items })
    MessagePlugin.success('周期价格已保存')
    await loadMatrix()
    return true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存周期价格失败')
    return false
  }
}

onMounted(() => {
  initMarkupRule()
  void loadMatrix()
})

defineExpose({ reload: loadMatrix, save })
</script>

<style scoped>
.upstream-price-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.upe-alert {
  margin-bottom: 0;
}
.upe-baseline {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  font-size: 13px;
  line-height: 1.7;
}
.upe-muted {
  color: var(--td-text-color-secondary, #999);
}
.upe-warn {
  color: var(--td-warning-color, #e37318);
}
.upe-pane {
  padding-top: 4px;
}
.upe-hint {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #999);
}
/* 字段纵向排列：控件与说明各占一行（避免说明被 flex 行挤成竖排单字） */
.upe-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
  min-width: 0;
}
.upe-label {
  font-size: 13px;
  font-weight: 600;
}
.upe-radio {
  flex-wrap: wrap;
  gap: 8px;
}
.upe-value-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
}
.upe-input {
  width: 160px;
  flex: none;
}
.upe-quick {
  flex-wrap: wrap;
}
.upe-tip {
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #999);
}
.upe-preview {
  margin-top: 4px;
}
.upe-preview__head {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 600;
}
.upe-preview__head .upe-muted {
  font-size: 12px;
  font-weight: 400;
}
/* 周期价格表：窄屏横向滚动而不是把列挤成一字一行 */
.upe-cycle {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.upe-cycle__name {
  font-weight: 600;
}
.upe-price {
  font-weight: 600;
  color: var(--td-brand-color, #0052d9);
}
.upe-up {
  color: var(--td-success-color, #2ba471);
}
.upe-down {
  color: var(--td-error-color, #d54941);
}
.upe-gen {
  padding: 12px;
  margin-bottom: 12px;
  border: 1px solid var(--td-component-border, #e5e5e5);
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer, #fafafa);
}
.upe-gen__head {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}
.upe-gen__head .upe-muted {
  font-size: 12px;
}
.upe-gen__row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 12px;
}
.upe-gen__row + .upe-gen__row {
  margin-top: 10px;
}
.upe-gen__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  width: 130px;
}
.upe-gen__note {
  flex: 1 1 260px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary, #999);
}
.upe-sync {
  padding: 12px;
  border: 1px dashed var(--td-component-border, #e5e5e5);
  border-radius: 8px;
}
.upe-sync__head {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 4px;
}
.upe-sync__head .upe-muted {
  font-size: 12px;
  line-height: 1.6;
}
.upe-sync__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}
.upe-sync__probe {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.upe-sync__result {
  font-size: 12px;
  color: var(--td-brand-color, #0052d9);
}
.upe-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 4px;
}

/* 窄屏：快速填入的字段与按钮各占一行，表格交给 TDesign 自身横向滚动 */
@media (max-width: 768px) {
  .upe-gen__field {
    width: 100%;
  }
  .upe-gen__row {
    flex-direction: column;
    align-items: stretch;
  }
  .upe-input {
    width: 100%;
  }
}
</style>
