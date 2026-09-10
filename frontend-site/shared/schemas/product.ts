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
  provision_mode: z.string().default('self'),
  config_options: z.string().default(''),
  featured: z.boolean().default(false),
  created_at: z.string().default(''),
})

export const productListWireSchema = z.object({
  items: z.array(z.unknown()).default([]),
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

function normalizeProduct(wire: ProductWire): Product {
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
    provisionMode: wire.provision_mode,
    configOptions: wire.config_options,
    featured: wire.featured,
    createdAt: wire.created_at,
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
