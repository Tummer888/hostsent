<template>
  <!--
    客户可选配置项编辑器（商品详情页「可选配置项」页签，自营与上游转售共用）。

    为什么抽成组件：这段逻辑原来在商品详情页里，与「合并后的详情页」其它页签同级；
    合并后仍只保留这一份实现（历史上两处各写一套，导致保存口径漂移）。

    **无损回写**是本组件的第一原则（实测故障）：一张配置项表里混着两类行 ——
      · source=upstream：上游镜像行，`id` 是向上游下单的 configoption 键，`source_key`
        是上游再上一层的 id，子项的 `qty_minimum/qty_maximum` 是上游的价档边界；
      · source=self：运营手工维护的平台参数行。
    旧实现回写时把所有行**一律标成 source=self**、丢掉 `id`/`upstream_id`，还把每个
    取值的加价压成 `[{monthly}]`。结果是：在上游商品上保存一次配置项，上游映射键被
    换成展示名、转售链路开通时拼不出 `configoption[上游选项id]=取值id`，且年付/季付
    加价被清零（实测：系统盘年付单价 9.60 元会变成 0）。现在改为「以服务端原对象为底、
    只覆盖界面真正编辑过的字段」，界面没碰的字段一个都不丢。
  -->
  <div class="config-options-editor">
    <div class="coe-head">
      <span class="coe-meta">
        {{ rows.length }} 个参数 · {{ subCount }} 个取值
        <template v-if="upstreamCount"> · 其中 {{ upstreamCount }} 个来自上游镜像（映射键只读）</template>
      </span>
      <t-space size="small">
        <t-button size="small" theme="primary" variant="outline" @click="openCatalogPicker">
          从平台配置项目录添加
        </t-button>
        <t-button size="small" variant="outline" @click="addBlankOption">新增自定义参数</t-button>
        <t-button size="small" variant="outline" @click="advancedVisible = !advancedVisible">
          {{ advancedVisible ? '隐藏' : '显示' }}高级 JSON
        </t-button>
      </t-space>
    </div>

    <t-alert
      v-if="isUpstreamChain"
      theme="info"
      class="coe-alert"
      message="对接商品的配置项来自上游镜像：映射键（上游选项/取值 id）与数量区间由上游下发，本地只调「展示名、默认值、开放与否、加价」；「主机名」这类本地补的开通项是自营行，可自由增删。"
    />

    <t-table
      row-key="rowKey"
      :data="rows"
      :columns="columns"
      size="small"
      hover
      cell-empty-content="—"
    >
      <template #opt_key="{ row }">
        <div class="coe-cell">
          <span class="coe-cell__strong">{{ row.option_name || '（未命名）' }}</span>
          <span class="coe-cell__sub">
            <code>{{ row.source_key || row.option_key || '—' }}</code>
            <t-tag v-if="row.source === 'upstream'" theme="warning" variant="light" size="small" shape="round">
              上游
            </t-tag>
            <t-tag v-else theme="primary" variant="light" size="small" shape="round">本地</t-tag>
          </span>
        </div>
      </template>

      <template #opt_widget="{ row }">
        <!-- 上游行的控件类型是上游协议的一部分（数量型必须按数量下发），不给改。 -->
        <t-tag v-if="row.source === 'upstream'" theme="default" variant="light" size="small" shape="round">
          {{ widgetLabel(row.widget) }}
        </t-tag>
        <t-select v-else v-model="row.widget" size="small" :options="widgetOptions" @change="onWidgetChange(row)" />
      </template>

      <template #opt_required="{ row }">
        <t-switch v-model="row.required" size="small" />
      </template>

      <template #opt_default="{ row }">
        <!--
          数量型的默认值按数字编辑，但仍写回 `default_value` 字符串：
          用 model-value + change（而不是 v-model 到另一个数字字段）是为了「没编辑过就不改写」，
          否则原始的空串会被写成 "0"，改变后端「缺省回落最小值」的取价口径。
        -->
        <t-input-number
          v-if="isQuantity(row)"
          :model-value="row.default_value === '' ? undefined : numOf(row.default_value)"
          size="small"
          :min="0"
          theme="normal"
          :placeholder="row.unit || '默认数量'"
          @change="(v: number | string | undefined) => (row.default_value = v == null ? '' : String(Number(v) || 0))"
        />
        <t-input v-else v-model="row.default_value" size="small" placeholder="默认值" />
        <span v-if="isQuantity(row)" class="coe-field-hint">档位内含量（不加价）</span>
      </template>

      <template #opt_values="{ row }">
        <t-link theme="primary" hover="color" @click="toggleExpand(row)">
          {{ isQuantity(row) ? '按量计价' : `${row.sub.length} 个取值` }}
        </t-link>
        <span v-if="isQuantity(row) && row.min_value != null" class="coe-cell__sub">
          {{ row.min_value }}~{{ row.max_value }} {{ row.unit }}
        </span>
      </template>

      <template #opt_action="{ row }">
        <div class="action-cell">
          <t-link theme="primary" hover="color" @click="toggleExpand(row)">
            {{ expandedKey === row.rowKey ? '收起' : isQuantity(row) ? '计价' : '取值' }}
          </t-link>
          <t-link theme="danger" hover="color" @click="removeOption(row)">删除</t-link>
        </div>
      </template>
      <template #empty>
        <t-empty description="暂无选配项：建品勾选规格档位会自动生成，也可点上方按钮手工添加" />
      </template>
    </t-table>

    <!-- 展开区：数量型 = 区间 + 逐周期单价；枚举型 = 取值明细（显示名/下发值/分组/逐周期加价） -->
    <div v-for="row in rows" :key="'detail-' + row.rowKey">
      <div v-if="expandedKey === row.rowKey" class="coe-detail">
        <!-- ── 数量型（系统盘/数据盘/带宽/IP 数量）：客户填数值，超出内含量按单位计价 ── -->
        <template v-if="isQuantity(row)">
          <div class="coe-detail__head">
            <span class="coe-cell__strong">{{ row.option_name }} · 按量计价</span>
            <span class="coe-cell__sub">
              客户填 {{ row.min_value ?? 0 }}~{{ row.max_value ?? '—' }} {{ row.unit }}；
              默认 {{ row.default_value || row.min_value || 0 }}{{ row.unit }} 以内不加价，超出部分按下面的单价加
            </span>
          </div>

          <div class="coe-qty">
            <label class="coe-qty__field">
              <span>可填下限</span>
              <t-input-number
                :model-value="row.min_value ?? undefined"
                :min="0"
                :theme="'normal'"
                size="small"
                @change="(v: number | string | undefined) => (row.min_value = numOrNull(v))"
              />
            </label>
            <label class="coe-qty__field">
              <span>可填上限</span>
              <t-input-number
                :model-value="row.max_value ?? undefined"
                :min="0"
                :theme="'normal'"
                size="small"
                @change="(v: number | string | undefined) => (row.max_value = numOrNull(v))"
              />
            </label>
            <label class="coe-qty__field">
              <span>单位</span>
              <t-input v-model="row.unit" size="small" placeholder="GB / Mbps" />
            </label>
          </div>

          <div class="coe-detail__head coe-detail__head--sub">
            <span class="coe-cell__strong">超出单价（元 / 单位）</span>
            <span class="coe-cell__sub">
              按客户所选计费周期取值；该周期没填就用月价。<b>长周期必须单独定价</b>，
              否则年付客户只被收一个月加配的钱（实测故障）。
            </span>
          </div>
          <div class="coe-qty">
            <label v-for="c in cyclePriceCols" :key="c.key" class="coe-qty__field">
              <span>{{ c.title }}</span>
              <t-input-number
                v-model="row.unitPrices[c.key]"
                :min="0"
                :precision="2"
                :theme="'normal'"
                size="small"
              />
            </label>
          </div>

          <div class="coe-detail__head coe-detail__head--sub">
            <span class="coe-cell__strong">上游价档（只读）</span>
            <span class="coe-cell__sub">上游按区间分段计价，本地按「上限价档单价 × 超量」折算，已逐点核对与上游一致</span>
          </div>
          <t-table
            row-key="subKey"
            :data="row.sub"
            :columns="qtyTierColumns"
            size="small"
            table-layout="fixed"
            cell-empty-content="—"
          >
            <template #tier_range="{ row: sub }">{{ sub.qtyMinimum ?? 0 }}~{{ sub.qtyMaximum ?? '—' }}</template>
            <template #tier_price="{ row: sub }">
              <span class="coe-cell__sub">
                月 {{ fmt(sub.priceMonthly) }} · 季 {{ fmt(sub.priceQuarterly) }} · 年 {{ fmt(sub.priceAnnually) }}
              </span>
            </template>
          </t-table>
        </template>

        <!-- ── 枚举型：逐个取值的展示名 / 下发值 / 分组 / 逐周期加价 ── -->
        <template v-else>
          <div class="coe-detail__head">
            <span class="coe-cell__strong">{{ row.option_name }} 的取值</span>
            <t-button size="small" variant="outline" @click="addSub(row)">添加取值</t-button>
          </div>
          <t-table
            row-key="subKey"
            :data="row.sub"
            :columns="subColumns"
            size="small"
            table-layout="fixed"
            cell-empty-content="—"
          >
            <template #sub_label="{ row: sub }">
              <t-input v-model="sub.option_name" size="small" placeholder="客户看到的名称" />
            </template>
            <template #sub_value="{ row: sub }">
              <t-input
                v-model="sub.source_key"
                size="small"
                :disabled="row.source === 'upstream'"
                placeholder="下发值"
              />
            </template>
            <template #sub_group="{ row: sub }">
              <t-input v-model="sub.group_label" size="small" placeholder="Ubuntu/Windows" />
            </template>
            <template #sub_price_monthly="{ row: sub }">
              <t-input-number v-model="sub.priceMonthly" size="small" :min="0" :precision="2" theme="normal" />
            </template>
            <template #sub_price_quarterly="{ row: sub }">
              <t-input-number v-model="sub.priceQuarterly" size="small" :min="0" :precision="2" theme="normal" />
            </template>
            <template #sub_price_annually="{ row: sub }">
              <t-input-number v-model="sub.priceAnnually" size="small" :min="0" :precision="2" theme="normal" />
            </template>
            <template #sub_price_onetime="{ row: sub }">
              <t-input-number v-model="sub.priceOnetime" size="small" :min="0" :precision="2" theme="normal" />
            </template>
            <template #sub_default="{ row: sub }">
              <t-radio :checked="sub.is_default" @change="setSubDefault(row, sub)" />
            </template>
            <template #sub_open="{ row: sub }">
              <t-switch
                :model-value="sub.hidden === 0"
                size="small"
                @change="(v: boolean) => (sub.hidden = v ? 0 : 1)"
              />
            </template>
            <template #sub_action="{ row: sub }">
              <t-link theme="danger" hover="color" @click="removeSub(row, sub)">删除</t-link>
            </template>
          </t-table>
          <p class="coe-note">
            加价为「元 / 该周期」，客户按所选周期加收一次（未填的周期回落月价）。关闭某个取值后客户看不到它。
          </p>
        </template>
      </div>
    </div>

    <t-textarea
      v-if="advancedVisible"
      v-model="advancedText"
      class="coe-advanced"
      :autosize="{ minRows: 8, maxRows: 24 }"
      placeholder='[{"name":"客户选配","options":[{"option_name":"CPU","source":"self","source_key":"cpu","widget":"radio","sub":[{"option_name":"2核","source":"self","source_key":"2"}]}]}]'
      @blur="applyAdvancedJSON"
    />

    <!-- 平台配置项目录选择器 -->
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
          <div class="coe-cell">
            <span class="coe-cell__strong">{{ row.label || row.option_key }}</span>
            <span class="coe-cell__sub">
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
import { computed, onMounted, ref } from 'vue'

import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getOptionCatalog, getProductConfigOptions, saveProductConfigOptions } from '@/api/product'
import type {
  OptionSpecInfo,
  SaleProductConfigGroup,
  SaleProductConfigOption,
  SaleProductConfigSub,
} from '@/types/interface'

defineOptions({ name: 'ConfigOptionsEditor' })

const props = defineProps<{
  /** 商品 ID（读取与保存都按它） */
  productId: number
  /** 商品是否上游转售链路：决定顶部提示与「数量型只读」等口径 */
  upstream?: boolean
  /** 商品绑定的平台渠道（目录选择器据此加载可选参数） */
  providerId?: number
}>()

const emit = defineEmits<{
  (e: 'update:saving', value: boolean): void
}>()

/**
 * 服务端原始对象的保底副本：回写时以它为底，只覆盖界面编辑过的字段。
 *
 * 为什么必须留着原对象：`SaleProductConfigOption` 类型里没有 `id`（上游下单键）、
 * 也没有 `qty_minimum/qty_maximum/is_quantity_tier/options_help`；旧实现按类型逐字段
 * 重建对象，这些字段全部被丢掉，于是上游行的映射键与价档边界被静默改写成展示名。
 * 保留原始对象 + 局部覆盖，才能做到「界面没碰的字段一个不丢」。
 */
type RawRecord = Record<string, unknown>

type SubRow = {
  subKey: string
  /** 服务端原始子项对象（新增行为空） */
  raw?: RawRecord
  option_name: string
  source_key: string
  group_label: string
  is_default: boolean
  hidden: number
  priceMonthly: number
  priceQuarterly: number
  priceAnnually: number
  priceOnetime: number
  qtyMinimum?: number
  qtyMaximum?: number
}

type OptionRow = {
  rowKey: string
  /** 服务端原始配置项对象（新增行为空） */
  raw?: RawRecord
  option_name: string
  option_type: number
  source: string
  source_key: string
  option_key: string
  widget: string
  required: boolean
  default_value: string
  unit: string
  min_value: number | null
  max_value: number | null
  /** 数量型：逐周期超出单价（回写到有价价档上） */
  unitPrices: { monthly: number; quarterly: number; annually: number; onetime: number }
  sub: SubRow[]
}

const rows = ref<OptionRow[]>([])
const expandedKey = ref('')
const advancedVisible = ref(false)
const advancedText = ref('')
const saving = ref(false)
const loading = ref(false)

let rowSeq = 0
function nextKey(prefix: string): string {
  rowSeq += 1
  return `${prefix}-${rowSeq}`
}

const isUpstreamChain = computed(() => !!props.upstream)
const upstreamCount = computed(() => rows.value.filter((r) => r.source === 'upstream').length)
const subCount = computed(() => rows.value.reduce((n, r) => n + r.sub.length, 0))

const widgetOptions = [
  { label: '下拉（可分组）', value: 'select' },
  { label: '单选（按钮组）', value: 'radio' },
  { label: '数量（步进器/滑块）', value: 'qty' },
  { label: '开关', value: 'bool' },
]

function widgetLabel(widget: string): string {
  return widgetOptions.find((o) => o.value === widget)?.label || widget || '下拉'
}

const cyclePriceCols = [
  { key: 'monthly' as const, title: '月付（元/单位）' },
  { key: 'quarterly' as const, title: '季付' },
  { key: 'annually' as const, title: '年付' },
  { key: 'onetime' as const, title: '一次性' },
]

const columns: PrimaryTableCol<OptionRow>[] = [
  { colKey: 'opt_key', title: '参数', minWidth: 190 },
  { colKey: 'opt_widget', title: '控件', width: 150 },
  { colKey: 'opt_required', title: '必选', width: 70 },
  { colKey: 'opt_default', title: '默认值', width: 150 },
  { colKey: 'opt_values', title: '取值 / 区间', minWidth: 140 },
  { colKey: 'opt_action', title: '操作', width: 120, align: 'center' as const },
]

const subColumns: PrimaryTableCol<SubRow>[] = [
  { colKey: 'sub_label', title: '显示名', minWidth: 150 },
  { colKey: 'sub_value', title: '下发值', width: 110 },
  { colKey: 'sub_group', title: '分组', width: 120 },
  { colKey: 'sub_price_monthly', title: '月加价', width: 110 },
  { colKey: 'sub_price_quarterly', title: '季加价', width: 110 },
  { colKey: 'sub_price_annually', title: '年加价', width: 110 },
  { colKey: 'sub_price_onetime', title: '一次性', width: 110 },
  { colKey: 'sub_default', title: '默认', width: 60 },
  { colKey: 'sub_open', title: '开放', width: 60 },
  { colKey: 'sub_action', title: '操作', width: 70, align: 'center' as const },
]

const qtyTierColumns: PrimaryTableCol<SubRow>[] = [
  { colKey: 'tier_range', title: '数量区间', width: 140 },
  { colKey: 'tier_price', title: '上游单价', minWidth: 240 },
]

// ---------- 工具 ----------

/** 数量型配置项的 option_type 集合（与后端 isQuantityOption / 上游 judgeQuantity 同口径）。 */
const QUANTITY_TYPES = [4, 7, 9, 11, 14, 15, 16, 17, 18, 19]
function isQuantity(row: OptionRow): boolean {
  return QUANTITY_TYPES.includes(row.option_type) || row.widget === 'qty'
}

function numOrNull(v: number | string | undefined): number | null {
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}
function numOf(v: unknown): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}
function strOf(v: unknown): string {
  return v == null ? '' : String(v)
}
function fmt(v: number): string {
  return v > 0 ? `¥${Number(v).toFixed(2)}` : '免费'
}

/** 数量型选项的「有价价档」：第一个含正价的子项（与后端 quantityUnitPriceForCycle 同规则）。 */
function pricedTierOf(row: OptionRow): SubRow | undefined {
  return row.sub.find(
    (s) => s.priceMonthly > 0 || s.priceQuarterly > 0 || s.priceAnnually > 0 || s.priceOnetime > 0,
  )
}

function emptyUnitPrices() {
  return { monthly: 0, quarterly: 0, annually: 0, onetime: 0 }
}

// ---------- 读：服务端 → 行（保留原始对象） ----------

function groupsToRows(groups: SaleProductConfigGroup[]): OptionRow[] {
  const out: OptionRow[] = []
  for (const g of groups || []) {
    for (const opt of (g.options || []) as (SaleProductConfigOption & RawRecord)[]) {
      const raw = opt as RawRecord
      const source = strOf(raw.source) || 'self'
      const subs: SubRow[] = ((opt.sub || []) as (SaleProductConfigSub & RawRecord)[]).map((s) => {
        const sraw = s as RawRecord
        const p = ((s.pricings || [])[0] || {}) as Record<string, unknown>
        return {
          subKey: nextKey('sub'),
          raw: sraw,
          option_name: strOf(s.option_name),
          source_key: strOf(s.source_key),
          group_label: strOf(s.group_label),
          is_default: !!s.is_default,
          hidden: numOf(s.hidden),
          priceMonthly: numOf(p.monthly),
          priceQuarterly: numOf(p.quarterly),
          priceAnnually: numOf(p.annually),
          priceOnetime: numOf(p.onetime),
          qtyMinimum: sraw.qty_minimum == null ? undefined : numOf(sraw.qty_minimum),
          qtyMaximum: sraw.qty_maximum == null ? undefined : numOf(sraw.qty_maximum),
        }
      })
      const row: OptionRow = {
        rowKey: nextKey('opt'),
        raw,
        option_name: strOf(opt.option_name),
        option_type: numOf(opt.option_type) || 1,
        source,
        source_key: strOf(raw.source_key),
        option_key: strOf(raw.option_key),
        widget: strOf(raw.widget) || (QUANTITY_TYPES.includes(numOf(opt.option_type)) ? 'qty' : 'select'),
        required: !!opt.required,
        default_value: strOf(opt.default_value),
        unit: strOf(raw.unit),
        min_value: raw.min_value == null ? null : numOf(raw.min_value),
        max_value: raw.max_value == null ? null : numOf(raw.max_value),
        unitPrices: emptyUnitPrices(),
        sub: subs,
      }
      // 数量型的「超出单价」= 有价价档的单价（免费档保持 0，客户按内含量购买时加价为 0）。
      const priced = pricedTierOf(row)
      if (priced) {
        row.unitPrices = {
          monthly: priced.priceMonthly,
          quarterly: priced.priceQuarterly,
          annually: priced.priceAnnually,
          onetime: priced.priceOnetime,
        }
      }
      out.push(row)
    }
  }
  return out
}

// ---------- 写：行 → config_groups（无损合并） ----------

/**
 * 把界面上的行合并回服务端结构。
 *
 * 合并规则（每一条都对应一类「保存一次就丢数据」的历史故障）：
 *   · 选项/子项对象以 `raw` 为底 → `id`(上游下单键)、`upstream_id`、`qty_minimum/qty_maximum`、
 *     `is_quantity_tier`、`options_help`、`provider_type` 等界面没建模的字段原样保留；
 *   · `source` 只在**新增行**上取默认值 self，已有行恒用服务端的来源标注
 *     （旧实现无条件写 self，会把上游行变成自营行，开通时上游配置整体丢失）；
 *   · `pricings` 保留其余周期，只覆盖界面编辑的四个周期价（旧实现压成 [{monthly}]，
 *     把年付/季付单价清零）。
 */
function buildGroups(): SaleProductConfigGroup[] {
  const options = rows.value.map((row, idx) => {
    const base: RawRecord = { ...(row.raw || {}) }
    base.option_name = row.option_name
    base.option_type = row.option_type
    // 已有行沿用服务端来源；新增行是运营手工建的平台参数（self）。
    base.source = row.raw ? strOf(row.raw.source) || 'self' : 'self'
    base.source_key = row.source_key
    base.option_key = row.option_key || row.source_key
    base.widget = row.widget
    base.required = !!row.required
    base.default_value = row.default_value || ''
    base.unit = row.unit
    base.sort_order = idx
    if (isQuantity(row)) {
      // 数量型的区间是客户可填范围（滑块），落到 min_value/max_value；
      // 同时回写 qty_minimum/qty_maximum —— 上游适配器按后两者判定"这是数量变量"。
      base.min_value = row.min_value
      base.max_value = row.max_value
      base.qty_minimum = row.min_value == null ? 0 : Math.round(row.min_value)
      if (row.max_value != null) base.qty_maximum = Math.round(row.max_value)
    }
    base.sub = row.sub.map((sub, sIdx) => {
      const sbase: RawRecord = { ...(sub.raw || {}) }
      sbase.option_name = sub.option_name
      sbase.source = sub.raw ? strOf(sub.raw.source) || strOf(base.source) : strOf(base.source) || 'self'
      sbase.source_key = sub.source_key
      sbase.group_label = sub.group_label
      sbase.is_default = !!sub.is_default
      sbase.hidden = sub.hidden ?? 0
      sbase.sort_order = sIdx
      const pricings = Array.isArray(sbase.pricings) && (sbase.pricings as unknown[]).length
        ? [...(sbase.pricings as Record<string, unknown>[])]
        : [{}]
      // 数量型：四个周期价都写到「有价价档」上，免费档保持 0（超出内含量才加价）。
      // 枚举型：每个取值各写自己的四个周期价（旧实现只保留 monthly，把季/年单价清零）。
      const priced = isQuantity(row) ? pricedTierOf(row) : undefined
      const target =
        priced && priced.subKey === sub.subKey
          ? { ...row.unitPrices }
          : isQuantity(row)
            ? { monthly: 0, quarterly: 0, annually: 0, onetime: 0 }
            : {
                monthly: sub.priceMonthly,
                quarterly: sub.priceQuarterly,
                annually: sub.priceAnnually,
                onetime: sub.priceOnetime,
              }
      pricings[0] = {
        ...pricings[0],
        monthly: target.monthly,
        quarterly: target.quarterly,
        annually: target.annually,
        onetime: target.onetime,
      }
      sbase.pricings = pricings
      return sbase
    })
    return base as unknown as SaleProductConfigOption
  })
  if (!options.length) return []
  return [{ name: '客户选配', description: '按配置档生成 / 手工维护', options }]
}

/** 界面行 → 规范化 JSON：既用于高级 JSON 展示，也用作「是否改过」的比对基线。 */
function serialize(): string {
  return JSON.stringify(buildGroups())
}

function syncAdvancedFromRows() {
  const groups = buildGroups()
  advancedText.value = groups.length ? JSON.stringify(groups, null, 2) : ''
}

/** 高级 JSON 编辑后回灌表格：走同一套无损读取，未知字段仍由 raw 保底。 */
function applyAdvancedJSON() {
  const raw = advancedText.value.trim()
  if (!raw) {
    rows.value = []
    return
  }
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) throw new Error('not array')
    rows.value = groupsToRows(parsed as SaleProductConfigGroup[])
    MessagePlugin.success('已按 JSON 刷新表格')
  } catch {
    MessagePlugin.warning('JSON 结构非法（应为 config_groups 数组），表格保持原样')
  }
}

// ---------- 行编辑 ----------

function onWidgetChange(row: OptionRow) {
  const map: Record<string, number> = { select: 1, radio: 2, bool: 3, qty: 4 }
  row.option_type = map[row.widget || 'select'] ?? 1
  syncAdvancedFromRows()
}

function toggleExpand(row: OptionRow) {
  expandedKey.value = expandedKey.value === row.rowKey ? '' : row.rowKey
}

function addSub(row: OptionRow) {
  row.sub.push({
    subKey: nextKey('sub'),
    option_name: '',
    source_key: '',
    group_label: '',
    is_default: false,
    hidden: 0,
    priceMonthly: 0,
    priceQuarterly: 0,
    priceAnnually: 0,
    priceOnetime: 0,
  })
  expandedKey.value = row.rowKey
}

function removeSub(row: OptionRow, target: SubRow) {
  row.sub = row.sub.filter((s) => s.subKey !== target.subKey)
}

function setSubDefault(row: OptionRow, target: SubRow) {
  // 每个参数只保留一个默认：用户侧预选靠它，多个默认会导致"看到的默认"与"实际开通"不一致。
  for (const sub of row.sub) sub.is_default = sub.subKey === target.subKey
  if (!isQuantity(row)) {
    const hit = row.sub.find((s) => s.subKey === target.subKey)
    row.default_value = hit ? hit.source_key || '' : ''
  }
}

function addBlankOption() {
  rows.value.push({
    rowKey: nextKey('opt'),
    option_name: '',
    option_type: 2,
    source: 'self',
    source_key: '',
    option_key: '',
    widget: 'radio',
    required: false,
    default_value: '',
    unit: '',
    min_value: null,
    max_value: null,
    unitPrices: emptyUnitPrices(),
    sub: [],
  })
}

function removeOption(row: OptionRow) {
  rows.value = rows.value.filter((r) => r.rowKey !== row.rowKey)
}

// ---------- 从平台配置项目录添加 ----------

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
    pickerItems.value = await getOptionCatalog({
      provider_type: 'dcimcloud',
      provider_id: props.providerId,
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载平台配置项目录失败')
    pickerItems.value = []
  } finally {
    pickerLoading.value = false
  }
}

function addFromCatalog(spec: OptionSpecInfo) {
  if (rows.value.some((r) => (r.option_key || r.source_key) === spec.option_key)) {
    MessagePlugin.warning(`已存在参数 ${spec.option_key}`)
    return
  }
  const isQty = spec.widget === 'qty' || spec.min_value != null
  rows.value.push({
    rowKey: nextKey('opt'),
    option_name: spec.label || spec.option_key,
    option_type: isQty ? 4 : spec.widget === 'radio' ? 2 : 1,
    source: 'self',
    source_key: spec.option_key,
    option_key: spec.option_key,
    widget: isQty ? 'qty' : spec.widget || 'select',
    required: !!spec.required,
    default_value: spec.default_value || '',
    unit: spec.unit || '',
    min_value: spec.min_value ?? null,
    max_value: spec.max_value ?? null,
    unitPrices: emptyUnitPrices(),
    sub: (spec.values || []).map((v) => ({
      subKey: nextKey('sub'),
      option_name: v.label || v.value,
      source_key: v.value,
      group_label: v.group_label || '',
      is_default: false,
      hidden: v.status === 'offline' ? 1 : 0,
      priceMonthly: 0,
      priceQuarterly: 0,
      priceAnnually: 0,
      priceOnetime: 0,
    })),
  })
  MessagePlugin.success(`已添加参数 ${spec.label || spec.option_key}`)
}

// ---------- 保存 ----------

const baseline = ref('')

async function reload() {
  if (!props.productId) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    const groups = await getProductConfigOptions(props.productId)
    rows.value = groupsToRows(groups || [])
    baseline.value = serialize()
    syncAdvancedFromRows()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载可选配置项失败')
  } finally {
    loading.value = false
  }
}

/**
 * 保存，返回是否成功。
 *
 * `dirty` 为假时直接返回成功、不发请求：合并后的详情页只有一个「保存」，
 * 没碰过配置项也提交一次等于让"别的页签的保存"顺带整表覆盖一遍配置项 ——
 * 幂等虽由服务端保证，但无谓的覆盖会刷新 updated_at 并留下误导性的变更历史。
 */
async function save(): Promise<boolean> {
  if (!props.productId) return true
  if (serialize() === baseline.value) return true
  saving.value = true
  emit('update:saving', true)
  try {
    await saveProductConfigOptions(props.productId, buildGroups())
    MessagePlugin.success('可选配置项已保存')
    await reload()
    return true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存可选配置项失败')
    return false
  } finally {
    saving.value = false
    emit('update:saving', false)
  }
}

onMounted(() => {
  void reload()
})

defineExpose({ reload, save })
</script>

<style scoped>
.config-options-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.coe-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.coe-meta {
  font-size: 13px;
  color: var(--td-text-color-secondary, #6b7280);
}

.coe-alert {
  margin: 0;
}

.coe-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.coe-cell__strong {
  font-weight: 600;
}

.coe-cell__sub {
  font-size: 12px;
  color: var(--td-text-color-secondary, #6b7280);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.coe-field-hint {
  display: block;
  font-size: 12px;
  color: var(--td-text-color-placeholder, #999);
}

.coe-detail {
  margin-top: 8px;
  padding: 12px;
  border: 1px solid var(--td-component-border, #e5e5e5);
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer, #fafafa);
}

.coe-detail__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.coe-detail__head--sub {
  margin-top: 12px;
}

/* 数量型的字段块：窄屏一列、宽屏自适应，避免把中文说明挤成竖排单字 */
.coe-qty {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px 12px;
}

.coe-qty__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #6b7280);
  min-width: 0;
}

.coe-note {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #6b7280);
}

.coe-advanced {
  margin-top: 4px;
}
</style>
