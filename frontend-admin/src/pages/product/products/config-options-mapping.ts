/**
 * 客户可选配置项的「服务端 ↔ 编辑行」双向映射。
 *
 * 单独成模块的原因：这是全流程里**最容易丢数据**的一段（配置项表里混着
 * source=upstream 的上游镜像行与 source=self 的本地行，两类的键与区间语义不同），
 * 而"有没有丢"只能靠把真实接口数据过一遍来验证。抽成纯函数后可以在 Node 里
 * 直接喂真实响应跑往返比对，不必渲染组件。
 *
 * 类型说明：`SaleProductConfigOption` 这个 TS 类型**不完整** —— 上游下单键 `id`、
 * 价档边界 `qty_minimum/qty_maximum`、`is_quantity_tier`、`options_help` 都不在其中，
 * 但它们必须原样回传（否则上游开通拼不出 configoption[选项id]=取值id）。
 * 因此这里统一用 `Raw`（保留服务端原始对象）作为基底，只在界面真正编辑过的字段上覆盖。
 */

/** 服务端返回的原始对象（含类型定义里没有的字段）。 */
export type RawRecord = Record<string, unknown>

/** 数量型配置项的 option_type 集合（与后端 isQuantityOption / 上游 judgeCommon 同口径）。 */
const QUANTITY_TYPES = [4, 7, 9, 11, 14, 15, 16, 17, 18, 19]

/** 四个可定价周期（与后端 configPrice 结构一致）。 */
export interface CycleUnitPrices {
  monthly: number
  quarterly: number
  annually: number
  onetime: number
}

export interface ConfigSubRow {
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

export interface ConfigOptionRow {
  rowKey: string
  /** 服务端原始配置项对象（新增行为空） */
  raw?: RawRecord
  option_name: string
  option_type: number
  /** upstream（上游镜像行，映射键只读）/ self（本地平台参数） */
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
  unitPrices: CycleUnitPrices
  sub: ConfigSubRow[]
}

export function numOf(v: unknown): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

export function strOf(v: unknown): string {
  return v == null ? '' : String(v)
}

export function numOrNull(v: number | string | undefined | null): number | null {
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

export function isEmptyUnitPrices(p: CycleUnitPrices): boolean {
  return p.monthly <= 0 && p.quarterly <= 0 && p.annually <= 0 && p.onetime <= 0
}

export function emptyUnitPrices(): CycleUnitPrices {
  return { monthly: 0, quarterly: 0, annually: 0, onetime: 0 }
}

/** 是否数量型（客户填数值）；option_type 是权威判据，widget 只是本地渲染口径。 */
export function isQuantity(row: Pick<ConfigOptionRow, 'option_type' | 'widget'>): boolean {
  return QUANTITY_TYPES.includes(row.option_type) || row.widget === 'qty'
}

/** 数量型选项的「有价价档」：第一个含正价的子项（与后端 quantityUnitPriceForCycle 同规则）。 */
export function pricedTierOf(row: ConfigOptionRow): ConfigSubRow | undefined {
  return row.sub.find((s) => !isEmptyUnitPrices({
    monthly: s.priceMonthly,
    quarterly: s.priceQuarterly,
    annually: s.priceAnnually,
    onetime: s.priceOnetime,
  }))
}

let seq = 0
function nextKey(prefix: string): string {
  seq += 1
  return `${prefix}-${seq}`
}

/**
 * 服务端 config_groups → 编辑行。
 *
 * 每个选项/子项都留一份服务端原始对象（`raw`）：回写时以它为底，界面没建模的字段
 * （上游下单键 id、价档边界、options_help…）才能原样带回去。
 */
export function groupsToRows(groups: unknown[]): ConfigOptionRow[] {
  const out: ConfigOptionRow[] = []
  for (const g of groups || []) {
    const options = ((g as { options?: unknown[] })?.options || []) as RawRecord[]
    for (const opt of options) {
      const raw = opt
      const source = strOf(raw.source) || 'self'
      const optionType = numOf(raw.option_type) || 1
      const rawSubs = (Array.isArray(raw.sub) ? raw.sub : []) as RawRecord[]
      const subs: ConfigSubRow[] = rawSubs.map((s) => {
        const priced = (Array.isArray(s.pricings) ? s.pricings : []) as Record<string, unknown>[]
        const p = priced[0] || {}
        return {
          subKey: nextKey('sub'),
          raw: s,
          option_name: strOf(s.option_name),
          source_key: strOf(s.source_key),
          group_label: strOf(s.group_label),
          is_default: !!s.is_default,
          hidden: numOf(s.hidden),
          priceMonthly: numOf(p.monthly),
          priceQuarterly: numOf(p.quarterly),
          priceAnnually: numOf(p.annually),
          priceOnetime: numOf(p.onetime),
          qtyMinimum: s.qty_minimum == null ? undefined : numOf(s.qty_minimum),
          qtyMaximum: s.qty_maximum == null ? undefined : numOf(s.qty_maximum),
        }
      })
      const row: ConfigOptionRow = {
        rowKey: nextKey('opt'),
        raw,
        option_name: strOf(raw.option_name),
        option_type: optionType,
        source,
        source_key: strOf(raw.source_key),
        option_key: strOf(raw.option_key) || strOf(raw.source_key),
        widget: strOf(raw.widget) || (QUANTITY_TYPES.includes(optionType) ? 'qty' : 'select'),
        required: !!raw.required,
        default_value: strOf(raw.default_value),
        unit: strOf(raw.unit),
        min_value: raw.min_value == null ? null : numOf(raw.min_value),
        max_value: raw.max_value == null ? null : numOf(raw.max_value),
        unitPrices: emptyUnitPrices(),
        sub: subs,
      }
      // 数量型：把「有价价档」的单价提到行级编辑（免费档保持 0，按内含量购买时加价为 0）。
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

/**
 * 编辑行 → 服务端 config_groups（无损合并）。
 *
 * 合并规则，每一条都对应一类「保存一次就丢数据」的历史故障：
 *   · 选项/子项以 `raw` 为底 → `id`(上游下单键)、`upstream_id`、`qty_minimum/qty_maximum`、
 *     `is_quantity_tier`、`options_help`、`provider_type` 等界面没建模的字段原样保留；
 *   · `source` 只在**新增行**取默认 self，已有行恒用服务端的来源标注
 *     （旧实现无条件写 self，会把上游行变成自营行，开通时上游配置整体丢失）；
 *   · `pricings` 保留其余周期，只覆盖界面编辑的四个周期价
 *     （旧实现压成 `[{monthly}]`，把季付/年付单价清零）。
 */
export function buildGroups(rows: ConfigOptionRow[]): unknown[] {
  const options = rows.map((row, idx) => {
    const base: RawRecord = { ...(row.raw || {}) }
    // 上游行不写本地字段：source_key/option_key 是上游下单键，改了开通必错。
    const upstreamRow = row.raw ? strOf(row.raw.source) === 'upstream' : false
    base.option_name = row.option_name
    base.option_type = row.option_type
    base.source = row.raw ? strOf(row.raw.source) || 'self' : 'self'
    if (!upstreamRow) {
      base.source_key = row.source_key
      base.option_key = row.option_key || row.source_key
    }
    base.widget = row.widget
    base.required = !!row.required
    base.default_value = row.default_value || ''
    base.unit = row.unit
    base.sort_order = idx
    if (isQuantity(row)) {
      // 数量型的区间是客户可填范围（滑块）：落到 min_value/max_value。
      // qty_minimum/qty_maximum 同步写回 —— 上游适配器按后两者判定"这是数量变量"
      // （judgeQuantity 只认数量本身）。仅在运营真的给了边界时才覆盖，避免把服务端值改成 0。
      base.min_value = row.min_value
      base.max_value = row.max_value
      if (row.min_value != null) base.qty_minimum = Math.round(row.min_value)
      if (row.max_value != null) base.qty_maximum = Math.round(row.max_value)
    }
    base.sub = row.sub.map((sub, sIdx) => {
      const sbase: RawRecord = { ...(sub.raw || {}) }
      sbase.option_name = sub.option_name
      sbase.source = sub.raw ? strOf(sub.raw.source) || strOf(base.source) : strOf(base.source) || 'self'
      // 上游子项的 source_key 是向上游下单传的取值 id，本地展示名可以改、这个不能改。
      if (!sub.raw || strOf(sub.raw.source) !== 'upstream') {
        sbase.source_key = sub.source_key
      }
      sbase.group_label = sub.group_label
      sbase.is_default = !!sub.is_default
      sbase.hidden = sub.hidden ?? 0
      sbase.sort_order = sIdx
      const pricings =
        Array.isArray(sbase.pricings) && (sbase.pricings as unknown[]).length
          ? [...(sbase.pricings as Record<string, unknown>[])]
          : [{}]
      // 数量型：四个周期价都写到「有价价档」上，免费档保持 0（超出内含量才加价）。
      // 枚举型：每个取值各写自己的四个周期价（旧实现只保留 monthly，把季/年加价清零）。
      const priced = isQuantity(row) ? pricedTierOf(row) : undefined
      const target =
        priced && priced.subKey === sub.subKey
          ? { ...row.unitPrices }
          : isQuantity(row)
            ? emptyUnitPrices()
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
    return base
  })
  if (!options.length) return []
  return [{ name: '客户选配', description: '按配置档生成 / 手工维护', options }]
}

/** 界面行 → 规范化 JSON 串：既用于高级 JSON 展示，也作为「是否改过」的比对基线。 */
export function serializeRows(rows: ConfigOptionRow[]): string {
  return JSON.stringify(buildGroups(rows))
}
