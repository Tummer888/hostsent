/**
 * 站点配置 schema —— 前后端共用的单一真相。
 *
 * 设计要点（见 docs/实施计划/80-官网门户与品牌配置架构设计.md 第 5 节）：
 *  1. 默认值写在代码里，数据库只存覆盖值；字段缺失/类型不符/接口异常时回落到默认值，页面永不白屏。
 *  2. 每个配置项都有类型与校验规则，保存前校验，避免运营填坏数据把首页搞崩。
 *  3. 数据库中的扁平键（`site.name`）通过 fromFlatConfig 还原成这里的嵌套结构。
 */
import { z } from 'zod'

/* ------------------------------------------------------------------ */
/* 各区块 schema                                                       */
/* ------------------------------------------------------------------ */

export const featureSchema = z.object({
  icon: z.string().max(30),
  title: z.string().min(1).max(30),
  desc: z.string().max(120),
})

export const siteSectionSchema = z.object({
  name: z.string().min(1).max(50),
  slogan: z.string().max(100),
  logo: z.string().max(255),
  favicon: z.string().max(255),
  icp: z.string().max(100),
  contactPhone: z.string().max(50),
  contactEmail: z.string().max(100),
  copyright: z.string().max(200),
})

export const themeSectionSchema = z.object({
  primaryColor: z.string().regex(/^#[0-9a-fA-F]{6}$/, '主题色需为 #RRGGBB 格式'),
  radius: z.string().regex(/^\d{1,3}px$/, '圆角需为像素值，例如 10px'),
})

export const homeSectionSchema = z.object({
  heroTitle: z.string().max(60),
  heroSubtitle: z.string().max(200),
  heroImage: z.string().max(255),
  heroPrimaryCta: z.string().max(20),
  heroPrimaryLink: z.string().max(255),
  heroSecondaryCta: z.string().max(20),
  heroSecondaryLink: z.string().max(255),
  featuresTitle: z.string().max(40),
  features: z.array(featureSchema).max(8),
  ctaTitle: z.string().max(60),
  ctaDesc: z.string().max(160),
  featuredTitle: z.string().max(40),
  featuredLimit: z.number().int().min(1).max(12),
  announceTitle: z.string().max(40),
  announceLimit: z.number().int().min(1).max(10),
})

export const siteContentSchema = z.object({
  site: siteSectionSchema,
  theme: themeSectionSchema,
  home: homeSectionSchema,
})

/* ------------------------------------------------------------------ */
/* 类型与默认值                                                        */
/* ------------------------------------------------------------------ */

export type SiteContent = z.infer<typeof siteContentSchema>
export type SiteSection = z.infer<typeof siteSectionSchema>
export type ThemeSection = z.infer<typeof themeSectionSchema>
export type HomeSection = z.infer<typeof homeSectionSchema>
export type HomeFeature = z.infer<typeof featureSchema>

/** 厂商默认值：未配置或配置损坏时使用，保证页面始终可用。 */
export const DEFAULT_SITE_CONTENT: SiteContent = {
  site: {
    name: '宿派云控',
    slogan: '高性能、可弹性伸缩的云计算服务',
    logo: '/branding/logo.svg',
    favicon: '/branding/favicon.svg',
    icp: '',
    contactPhone: '400-800-1234',
    contactEmail: 'support@hostsent.com',
    copyright: '© 2026 Hostsent.com 版权所有',
  },
  theme: {
    primaryColor: '#2b5cff',
    radius: '10px',
  },
  home: {
    heroTitle: '更强大的一站式云资源平台',
    heroSubtitle:
      '从产品上架、在线下单到自动开通，全流程在一个后台闭环；已对接魔方财务与魔方云，支持白标转售，几分钟上线你自己的云品牌。',
    heroImage: '',
    heroPrimaryCta: '立即选购',
    heroPrimaryLink: '/products',
    heroSecondaryCta: '了解产品优势',
    heroSecondaryLink: '/#features',
    featuresTitle: '为什么选择我们',
    features: [
      { icon: 'server', title: '高性能云主机', desc: '全闪存存储与多线 BGP 接入，计算性能稳定可靠。' },
      { icon: 'shield', title: '安全与合规', desc: 'DDoS 防护、快照备份与细粒度访问控制，数据更安心。' },
      { icon: 'bolt', title: '弹性伸缩', desc: '按量付费、秒级开通，业务增长时随时扩容。' },
      { icon: 'support', title: '7×24 技术支持', desc: '工单与智能助手全天候响应，保障业务平稳运行。' },
    ],
    ctaTitle: '准备好开始了吗？',
    ctaDesc: '注册即可享受新用户优惠，几分钟内完成你的第一台云主机部署。',
    featuredTitle: '热门产品',
    featuredLimit: 8,
    announceTitle: '最新公告',
    announceLimit: 5,
  },
}

/* ------------------------------------------------------------------ */
/* 解析与合并                                                          */
/* ------------------------------------------------------------------ */

type PlainObject = Record<string, unknown>

function isPlainObject(value: unknown): value is PlainObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** 深合并：对象递归合并，数组与标量整体覆盖（数组不做元素级合并）。 */
function deepMerge<T>(base: T, override: unknown): T {
  if (!isPlainObject(base) || !isPlainObject(override)) {
    return (override === undefined ? base : (override as T))
  }
  const out: PlainObject = { ...base }
  for (const [key, value] of Object.entries(override)) {
    if (value === undefined) continue
    const current = out[key]
    out[key] = isPlainObject(current) && isPlainObject(value) ? deepMerge(current, value) : value
  }
  return out as T
}

/** 按区块校验：坏区块单独回落默认值，不影响其它区块。 */
function parseSection<T>(schema: z.ZodType<T>, defaults: T, raw: unknown): T {
  if (raw === undefined || raw === null) return defaults
  const merged = deepMerge(defaults, raw)
  const parsed = schema.safeParse(merged)
  return parsed.success ? parsed.data : defaults
}

/**
 * 把任意来源的原始配置解析为合法 SiteContent。
 * 接口异常、字段缺失、类型不符都会回落到默认值，绝不抛错。
 */
export function resolveSiteContent(raw: unknown): SiteContent {
  const src: PlainObject = isPlainObject(raw) ? raw : {}
  return {
    site: parseSection(siteSectionSchema, DEFAULT_SITE_CONTENT.site, src.site),
    theme: parseSection(themeSectionSchema, DEFAULT_SITE_CONTENT.theme, src.theme),
    home: parseSection(homeSectionSchema, DEFAULT_SITE_CONTENT.home, src.home),
  }
}

/* ------------------------------------------------------------------ */
/* 扁平配置还原（对接 system_configs）                                 */
/* ------------------------------------------------------------------ */

/** 按目标默认值的类型做标量转换：字符串 "8" → 数字，JSON 文本 → 对象/数组。 */
function coerce(value: unknown, template: unknown): unknown {
  if (value === undefined || value === null) return undefined
  if (typeof template === 'number') {
    const n = typeof value === 'number' ? value : Number(String(value).trim())
    return Number.isFinite(n) ? n : undefined
  }
  if (typeof template === 'boolean') {
    if (typeof value === 'boolean') return value
    return String(value).trim().toLowerCase() === 'true'
  }
  if (typeof template === 'object') {
    if (typeof value !== 'string') return value
    try {
      return JSON.parse(value)
    } catch {
      return undefined // 填坏的 JSON 直接丢弃，交由默认值兜底
    }
  }
  return typeof value === 'string' ? value : String(value)
}

function setByPath(target: PlainObject, path: string, value: unknown, template: unknown): void {
  const keys = path.split('.').filter(Boolean)
  if (keys.length === 0) return
  const coerced = coerce(value, template)
  if (coerced === undefined) return

  let cursor: PlainObject = target
  for (let i = 0; i < keys.length - 1; i += 1) {
    const key = keys[i] as string
    if (!isPlainObject(cursor[key])) cursor[key] = {}
    cursor = cursor[key] as PlainObject
  }
  cursor[keys[keys.length - 1] as string] = coerced
}

/** 读取默认值中某个路径的值，用于类型推断。 */
function getByPath(source: unknown, path: string): unknown {
  let cursor: unknown = source
  for (const key of path.split('.')) {
    if (!isPlainObject(cursor)) return undefined
    cursor = cursor[key]
  }
  return cursor
}

/** 数据库扁平键用 snake_case（`home.hero_title`），代码内用 camelCase。 */
function toCamelPath(path: string): string {
  return path
    .split('.')
    .filter(Boolean)
    .map((segment) => segment.replace(/_([a-z0-9])/g, (_, chr: string) => chr.toUpperCase()))
    .join('.')
}

/**
 * 扁平键值对（`{ "site.name": "宿派云控", "home.featured_limit": "8" }`）
 * 还原为嵌套结构。键不存在或类型不符时忽略该项，由默认值兜底。
 */
export function fromFlatConfig(flat: unknown): PlainObject {
  if (!isPlainObject(flat)) return {}
  const out: PlainObject = {}
  for (const [path, value] of Object.entries(flat)) {
    const camelPath = toCamelPath(path)
    setByPath(out, camelPath, value, getByPath(DEFAULT_SITE_CONTENT, camelPath))
  }
  return out
}
