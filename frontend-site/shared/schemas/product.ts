/**
 * 商品展示 schema —— 后端 wire 契约（snake_case）到前端展示模型（camelCase）的单一转换点。
 *
 * 设计要点（见 docs/实施计划/80-官网门户与品牌配置架构设计.md §7、§9.3）：
 *  1. 规范 R4：接口响应必须经 schema 校验后再使用；校验失败的商品被丢弃而不是让整页崩掉。
 *  2. 规范 R7：`specs` / `config_options` 是 JSON 字符串，解析必须容错（try/catch + 降级）。
 *  3. 规范 R6：价格展示按 `price_model` 分别格式化（一口价 / 每小时 / 每月）。
 *
 * 上游契约来源：`backend/internal/modules/uc/product/dto/product.go`。
 */
import { z } from 'zod'

/* ------------------------------------------------------------------ */
/* wire 契约（后端字段，snake_case）                                    */
/* ------------------------------------------------------------------ */

export const productWireSchema = z.object({
  id: z.number().int().nonnegative(),
  name: z.string().min(1),
  description: z.string().default(''),
  cover_image: z.string().default(''),
  category_id: z.number().default(0),
  product_type: z.string().default(''),
  price: z.number().default(0),
  price_model: z.string().default('fixed'),
  specs: z.string().default(''),
  // 后端实际字段名是 source_mode（self 自营 / upstream 上游转售）；
  // provision_mode 是早期契约里的名字，两个都收，避免改名时官网静默丢字段。
  source_mode: z.string().optional(),
  provision_mode: z.string().default('self'),
  config_options: z.string().default(''),
  featured: z.boolean().default(false),
  created_at: z.string().default(''),
  // Go 的 nil 切片序列化成 null（不是 []），而 Zod 的 .default() 只对 undefined 生效，
  // 因此这里必须用 nullish + transform 兜底：否则「未维护周期矩阵」的商品会校验失败，
  // 详情页 404、列表项被静默丢弃。
  cycles: z.array(z.string()).nullish().transform((v) => v ?? []),
  skus: z.array(z.unknown()).nullish().transform((v) => v ?? []),
})

export const productListWireSchema = z.object({
  // 同上：Go 的 nil 切片是 JSON null，用 nullish 兜底而不是 .default()。
  items: z.array(z.unknown()).nullish().transform((v) => v ?? []),
  page: z.number().default(1),
  page_size: z.number().default(12),
  total: z.number().default(0),
})

/* ------------------------------------------------------------------ */
/* 展示模型（前端字段，camelCase）                                      */
/* ------------------------------------------------------------------ */

/** 产品规格项（`specs` JSON 内的结构，见 catalog model.ProductSpecItem）。 */
export interface ProductSpecItem {
  key: string
  label: string
  value: string
}

export interface Product {
  id: number
  name: string
  description: string
  coverImage: string
  categoryId: number
  productType: string
  price: number
  priceModel: string
  specs: ProductSpecItem[]
  provisionMode: string
  configOptions: string
  featured: boolean
  createdAt: string
  /** 商品级可售周期；为空表示按商品级单价下单 */
  cycles: string[]
  /** 可售规格；为空表示未拆 SKU */
  skus: ProductSku[]
}

/** 可售规格（SKU）：官网只做展示与意图传递，下单与算价仍在用户中心完成。 */
export interface ProductSku {
  specCode: string
  name: string
  specs: ProductSpecItem[]
  price: number
  priceModel: string
  stock: number
  cycles: string[]
}

export interface ProductList {
  items: Product[]
  page: number
  pageSize: number
  total: number
}

export const EMPTY_PRODUCT_LIST: ProductList = { items: [], page: 1, pageSize: 12, total: 0 }

/* ------------------------------------------------------------------ */
/* specs 解析（容错）                                                   */
/* ------------------------------------------------------------------ */

const MAX_SPEC_ITEMS = 12

function normalizeSpecValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return ''
  }
}

/**
 * 解析 `specs` JSON 字符串。
 *
 * 兼容两种历史形态：`[{key,label,value}]` 与扁平对象 `{cpu:"2核"}`；
 * 空串、坏 JSON、非预期结构一律返回空数组，由页面降级展示。
 */
export function parseSpecs(raw: unknown): ProductSpecItem[] {
  if (typeof raw !== 'string' || raw.trim() === '') return []

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }

  const items: ProductSpecItem[] = []
  if (Array.isArray(parsed)) {
    for (const entry of parsed) {
      if (typeof entry !== 'object' || entry === null) continue
      const record = entry as Record<string, unknown>
      const label = typeof record.label === 'string' ? record.label : String(record.key ?? '')
      if (!label) continue
      items.push({ key: String(record.key ?? label), label, value: normalizeSpecValue(record.value) })
    }
  } else if (typeof parsed === 'object') {
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      items.push({ key, label: key, value: normalizeSpecValue(value) })
    }
  }

  return items.slice(0, MAX_SPEC_ITEMS)
}

/* ------------------------------------------------------------------ */
/* 价格格式化（规范 R6）                                                */
/* ------------------------------------------------------------------ */

/** 价格模型 → 计价单位后缀。未知模型按一口价处理。 */
export function priceUnit(priceModel: string): string {
  switch (priceModel) {
    case 'hourly':
      return '/小时'
    case 'monthly':
      return '/月'
    default:
      return ''
  }
}

/** 格式化价格，例如 `¥129.00 /月`；价格为 0 时返回「免费」以免展示成 ¥0.00 误导。 */
export function formatPrice(price: number, priceModel: string): string {
  if (!Number.isFinite(price) || price <= 0) return '免费'
  return `¥${price.toFixed(2)}${priceUnit(priceModel) ? ` ${priceUnit(priceModel)}` : ''}`
}

/** 仅取金额部分（用于需要把单位单独排版的场景）。 */
export function formatPriceAmount(price: number): string {
  if (!Number.isFinite(price) || price <= 0) return '免费'
  return price.toFixed(2)
}

/* ------------------------------------------------------------------ */
/* 归一化与解析入口                                                     */
/* ------------------------------------------------------------------ */

type ProductWire = z.infer<typeof productWireSchema>

/**
 * 解析 SKU 数组。
 *
 * 每一项都按「坏的丢掉、好的保留」处理：官网是展示层，一个字段缺失的 SKU
 * 不该把整个商品详情页打掉（规范 R4）。
 */
function parseSkus(raw: unknown, fallbackCycles: string[]): ProductSku[] {
  if (!Array.isArray(raw)) return []
  const out: ProductSku[] = []
  for (const entry of raw) {
    if (typeof entry !== 'object' || entry === null) continue
    const record = entry as Record<string, unknown>
    const code = typeof record.spec_code === 'string' ? record.spec_code : ''
    const name = typeof record.name === 'string' ? record.name : ''
    if (!code && !name) continue
    const cycles = Array.isArray(record.cycles)
      ? record.cycles.filter((c): c is string => typeof c === 'string' && c !== '')
      : []
    out.push({
      specCode: code,
      name: name || code,
      specs: parseSpecs(record.specs),
      price: typeof record.price === 'number' ? record.price : 0,
      priceModel: typeof record.price_model === 'string' ? record.price_model : '',
      stock: typeof record.stock === 'number' ? record.stock : -1,
      // SKU 无自有周期时回落商品级周期，与后端 Get 的回落口径一致。
      cycles: cycles.length ? cycles : fallbackCycles,
    })
  }
  return out
}

function normalizeProduct(wire: ProductWire): Product {
  const cycles = wire.cycles.filter((c) => c !== '')
  return {
    id: wire.id,
    name: wire.name,
    description: wire.description,
    coverImage: wire.cover_image,
    categoryId: wire.category_id,
    productType: wire.product_type,
    price: wire.price,
    priceModel: wire.price_model,
    specs: parseSpecs(wire.specs),
    provisionMode: wire.source_mode || wire.provision_mode,
    configOptions: wire.config_options,
    featured: wire.featured,
    createdAt: wire.created_at,
    cycles,
    skus: parseSkus(wire.skus, cycles),
  }
}

/** 解析单个商品；校验失败返回 null（由调用方决定跳过还是 404）。 */
export function parseProduct(raw: unknown): Product | null {
  const parsed = productWireSchema.safeParse(raw)
  return parsed.success ? normalizeProduct(parsed.data) : null
}

/** 解析商品列表；逐条校验，坏数据被跳过而不影响整页。 */
export function parseProductList(raw: unknown): ProductList {
  const parsed = productListWireSchema.safeParse(raw)
  if (!parsed.success) return EMPTY_PRODUCT_LIST

  const items: Product[] = []
  for (const entry of parsed.data.items) {
    const product = parseProduct(entry)
    if (product) items.push(product)
  }
  return {
    items,
    page: parsed.data.page,
    pageSize: parsed.data.page_size,
    total: parsed.data.total,
  }
}
