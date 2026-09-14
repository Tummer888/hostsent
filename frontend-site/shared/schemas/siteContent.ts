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

/** 页脚服务保障条（`site.footer_promises`）。 */
export const footerPromiseSchema = z.object({
  icon: z.string().max(30),
  title: z.string().min(1).max(20),
  desc: z.string().max(60),
})

/**
 * 页脚社交按钮（`site.footer_socials`）。icon 需是 SiteIcon 内置名，认不出时按钮仍渲染但无图标。
 *
 * `url` 是必填语义：没有地址的社交入口在页脚就是「点了没反应」的假按钮
 *（改造前点一下弹「微信开发中」），组件会把缺 url 的项直接丢掉不渲染。
 */
export const footerSocialSchema = z.object({
  label: z.string().min(1).max(20),
  icon: z.string().max(30),
  url: z.string().max(255).catch(''),
})

/** 页脚栏目（`site.footer_columns`）。to 支持站内路径与站点锚点（`/#contact`）。 */
export const footerColumnSchema = z.object({
  title: z.string().min(1).max(30),
  links: z
    .array(
      z.object({
        label: z.string().min(1).max(40),
        to: z.string().min(1).max(255),
      }),
    )
    .max(12),
})

export const siteSectionSchema = z.object({
  name: z.string().min(1).max(50),
  slogan: z.string().max(100),
  logo: z.string().max(255),
  favicon: z.string().max(255),
  icp: z.string().max(100),
  contactPhone: z.string().max(50),
  contactEmail: z.string().max(100),
  contactAddress: z.string().max(200),
  copyright: z.string().max(200),
  /** 增值电信业务经营许可证号（页脚展示，运营在管理端配置） */
  licenseNo: z.string().max(100),
  /** 代理域名注册服务机构 */
  licenseOrg: z.string().max(100),
  /** 公网安备号 */
  publicSecurity: z.string().max(100),
  /** 微信公众号名称 */
  wechat: z.string().max(100),
  /**
   * 以下四项在数据库里的键是 `site.footer_*`（分组 site，doc100 §8.2），
   * fromFlatConfig 按点号路径还原后自然落在 site 区块下，因此字段名带 footer 前缀。
   */
  /** 页脚服务保障条 */
  footerPromises: tolerantArray(footerPromiseSchema, 6),
  /** 页脚社交按钮 */
  footerSocials: tolerantArray(footerSocialSchema, 8),
  /** 页脚栏目（标题 + 链接组） */
  footerColumns: tolerantArray(footerColumnSchema, 6),
  /** 页脚法律行补充文案（许可证/备案组合），留空不渲染该行 */
  footerLegalLine: z.string().max(200),
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
    // 联系方式默认留空：代码里放 400-800-1234 / support@hostsent.com 这类示例值，
    // 上线后会变成官网首页的「售前咨询热线」——一个打不通的号码比不显示更糟，
    // 用户打不通会直接认定平台是假的。运营在「系统配置 → 站点品牌」里填真实号码。
    contactPhone: '',
    contactEmail: '',
    contactAddress: '',
    copyright: '© 2026 Hostsent.com 版权所有',
    // 法务信息默认留空：运营未配置时页脚不渲染该行，
    // 而不是显示示例证号 —— 伪造的备案/许可证号比缺失更糟。
    licenseNo: '',
    licenseOrg: '',
    publicSecurity: '',
    wechat: '',
    // 页脚服务保障条默认值：每一条都对应平台真实机制（工单优先级、余额扣费台账、
    // 实例与订单的操作留痕、生命周期到期提醒与自动续费）。
    // 此前这里是「7×24 / 免费备案 / 无忧退订 / 建议反馈」——平台不提供备案与
    // 无理由退订，「建议反馈」也没有后端入口，属于把做不到的事写成服务承诺。
    footerPromises: [
      { icon: 'service', title: '工单支持', desc: '按优先级响应，进度可见' },
      { icon: 'secured', title: '余额支付', desc: '扣费走账户余额，明细可查' },
      { icon: 'edit', title: '操作留痕', desc: '订单与实例动作可回溯' },
      { icon: 'time', title: '到期提醒', desc: '到期前提醒，可自助续费' },
      { icon: 'rollback', title: '自动续费', desc: '可开启自动续费避免中断' },
    ],
    // 社交按钮默认留空：平台没有公众号/QQ 群/开源仓库的公开地址时，
    // 摆一排点了没反应的图标不如不摆（AppFooter 只渲染带 url 的项）。
    // 运营在「系统配置 → 站点品牌 → 社交按钮」按 {"label","icon","url"} 填。
    footerSocials: [],
    // 栏目文案留空数组：栏目内容与品牌名、真实产品线有关（「关于宿派云控」），
    // 写进默认值会在运营改名后留下一段对不上的旧品牌名。空数组时组件用
    // 品牌名动态拼装（见 AppFooter.vue 的 defaultColumns），效果与改造前一致。
    footerColumns: [],
    footerLegalLine: '',
  },
  theme: {
    primaryColor: '#2b5cff',
    radius: '10px',
  },
  home: {
    heroTitle: '更强大的一站式云资源平台',
    heroSubtitle:
      '从产品上架、在线下单到自动开通，全流程在一个后台闭环；已对接魔方云与魔方财务，分钟级交付你自己的云服务。',
    heroImage: '',
    heroPrimaryCta: '立即选购',
    heroPrimaryLink: '/products',
    heroSecondaryCta: '了解产品优势',
    heroSecondaryLink: '/#features',
    featuresTitle: '为什么选择我们',
    // 四张卡只写平台确实提供的能力：多上游渠道（资源来自魔方云/魔方财务等已接入渠道）、
    // 支付后自动开通（开通任务队列 + 失败重试与转人工）、周期计费与续费（周期价格矩阵、
    // 到期提醒、自动续费）、工单支持（四档优先级）。
    // 此前写的「全闪存/多线 BGP」「DDoS 防护、快照备份」「按量付费、秒级开通」
    // 都不是本平台能承诺的：资源规格由上游决定，平台没有安全防护与快照模块，
    // 计费按周期定价而非按量，开通耗时取决于上游接口。
    features: [
      { icon: 'server', title: '多云上游统一交付', desc: '已接入魔方云、魔方财务等上游渠道，产品、订单与实例在同一后台收敛。' },
      { icon: 'bolt', title: '支付后自动开通', desc: '支付成功即投递开通任务，由上游接口创建实例；异常自动重试并转人工处理。' },
      { icon: 'refresh', title: '周期计费与续费', desc: '按周期制定价格，支持自助续费与自动续费，到期前有提醒。' },
      { icon: 'support', title: '工单与全链路留痕', desc: '工单按优先级响应；订单、开通与实例操作全程记录，问题可回溯。' },
    ],
    ctaTitle: '准备好开始了吗？',
    ctaDesc: '注册后即可浏览产品与周期价格，几分钟内完成你的第一台云主机下单。',
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
 * 元素级容错的数组校验：坏元素被丢弃，好元素保留。
 *
 * 为什么不用裸 `z.array(item).max(n)`：数组里只要有一项填坏（比如运营在页脚栏目
 * JSON 里漏了 title），整段 safeParse 失败 → 整个 `site` 区块回落默认值，
 * 连同一区块里填得好好的品牌名、备案号一起丢掉。这类「一处手误全盘失效」
 * 的失败模式在配置场景里代价太高（doc80 R11：单个坏数据不应影响其它数据）。
 */
function tolerantArray<T>(itemSchema: z.ZodType<T>, max: number) {
  return z.preprocess((raw) => {
    if (!Array.isArray(raw)) return raw
    const valid: unknown[] = []
    for (const entry of raw) {
      if (itemSchema.safeParse(entry).success) valid.push(entry)
    }
    return valid.slice(0, max)
  }, z.array(itemSchema).max(max))
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
 * 管理端「系统配置」页历史扁平键 → 本 schema 路径的别名表。
 *
 * 为什么保留这张表：`site_name` 这类扁平键是建库时的历史键名，后端白名单仍原样返回，
 * 迁移 045 只把它们置为 disabled 而未删除。保留映射，是为了在尚未执行迁移的库上仍能
 * 读到运营填过的值，不至于把品牌信息整个丢掉。
 */
const FLAT_KEY_ALIASES: Record<string, string> = {
  site_name: 'site.name',
  site_slogan: 'site.slogan',
  site_logo: 'site.logo',
  site_favicon: 'site.favicon',
  site_icp: 'site.icp',
  site_copyright: 'site.copyright',
  site_license_no: 'site.licenseNo',
  site_license_org: 'site.licenseOrg',
  site_public_security: 'site.publicSecurity',
  contact_phone: 'site.contactPhone',
  contact_email: 'site.contactEmail',
  contact_address: 'site.contactAddress',
  contact_wechat: 'site.wechat',
}

/**
 * 扁平键值对（`{ "site.name": "宿派云控", "home.featured_limit": "8" }`）
 * 还原为嵌套结构。键不存在或类型不符时忽略该项，由默认值兜底。
 *
 * 两套键名同时存在时，**规范点号键（`site.name`）覆盖历史扁平键（`site_name`）**：
 * 点号键是管理端「站点品牌」分组（以及迁移 045 的数据补齐）维护的当前值，扁平键是
 * 建库时的旧占位值。这里曾写反过顺序 —— 门户首页因此长期显示旧站点名，而不是运营
 * 在后台改的新名。
 */
export function fromFlatConfig(flat: unknown): PlainObject {
  if (!isPlainObject(flat)) return {}
  const out: PlainObject = {}

  const write = (path: string, value: unknown): void => {
    const camelPath = toCamelPath(path)
    // 只接受本 schema 里存在的路径：否则 system_timezone 之类的无关配置也会被写进中间对象。
    if (getByPath(DEFAULT_SITE_CONTENT, camelPath) === undefined) return
    setByPath(out, camelPath, value, getByPath(DEFAULT_SITE_CONTENT, camelPath))
  }

  // 第一轮：历史扁平键（不含点号，不会被下一轮的点号扫描碰到）。
  // 未命中别名的键（如 system_timezone / currency_unit）与本 schema 无关，跳过。
  for (const [rawPath, value] of Object.entries(flat)) {
    const alias = FLAT_KEY_ALIASES[rawPath]
    if (!alias) continue
    write(alias, value)
  }
  // 第二轮：规范点号键，后写以覆盖同义的扁平键。
  for (const [rawPath, value] of Object.entries(flat)) {
    if (!rawPath.includes('.')) continue
    write(rawPath, value)
  }
  return out
}
