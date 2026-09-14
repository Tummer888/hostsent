/**
 * 内容展示 schema —— 新闻 / 帮助 / 条款 / 隐私 / 友情链接的 wire 契约（snake_case）
 * 到展示模型（camelCase）的单一转换点。
 *
 * 设计要点（见 docs/实施计划/100-内容中心与站点内容管理设计.md §9）：
 *  1. 规范 R12：列表逐条校验、坏数据跳过；详情校验失败等价于「内容不存在」。
 *  2. 正文信任边界：`body` 已由后端 `internal/pkg/sanitize` 净化（doc100 §6.1），
 *     门户直接 v-html 渲染；这里不做二次过滤，避免把合法富文本误伤成纯文本。
 *  3. 分类树是递归结构，深度不设上限但逐层丢弃坏节点。
 *
 * 上游契约来源：`backend/internal/modules/site/dto/site.go`。
 */
import { z } from 'zod'

/* ------------------------------------------------------------------ */
/* 类型常量                                                            */
/* ------------------------------------------------------------------ */

export const ARTICLE_KINDS = ['news', 'help', 'terms', 'privacy'] as const
export type ArticleKind = (typeof ARTICLE_KINDS)[number]

/** 有列表页与详情页的类型（条款 / 隐私是「每类型一篇」，不走列表）。 */
export const LIST_KINDS: ArticleKind[] = ['news', 'help']

/** 有分类树的类型（与后端 model.KindNeedsCategory 保持一致）。 */
export const CATEGORY_KINDS: ArticleKind[] = ['news', 'help']

export function isArticleKind(value: unknown): value is ArticleKind {
  return typeof value === 'string' && (ARTICLE_KINDS as readonly string[]).includes(value)
}

/** 类型 → 门户路径前缀。生成链接与 sitemap 共用，避免两处各写一份映射。 */
export function kindPortalPath(kind: ArticleKind): string {
  switch (kind) {
    case 'news':
      return '/news'
    case 'help':
      return '/help'
    case 'terms':
      return '/terms'
    case 'privacy':
      return '/privacy'
  }
}

/** 类型 → 中文名（页面标题 / 空态文案）。 */
export function kindLabel(kind: ArticleKind): string {
  switch (kind) {
    case 'news':
      return '新闻资讯'
    case 'help':
      return '帮助中心'
    case 'terms':
      return '用户条款'
    case 'privacy':
      return '隐私政策'
  }
}

/* ------------------------------------------------------------------ */
/* wire 契约                                                           */
/* ------------------------------------------------------------------ */

export const articleItemWireSchema = z.object({
  id: z.number().int().nonnegative(),
  kind: z.string().min(1),
  slug: z.string().min(1),
  title: z.string().min(1),
  summary: z.string().default(''),
  cover: z.string().default(''),
  // Go 的 nil 切片序列化成 null，用 nullish 兜底（同 product.ts 的 cycles）
  tags: z.array(z.unknown()).nullish().transform((v) => v ?? []),
  category_id: z.number().default(0),
  category_slug: z.string().default(''),
  category_name: z.string().default(''),
  pinned: z.boolean().default(false),
  version: z.string().default(''),
  publish_at: z.string().default(''),
  updated_at: z.string().default(''),
})

export const articleDetailWireSchema = articleItemWireSchema.extend({
  body: z.string().default(''),
})

export const articleListWireSchema = z.object({
  items: z.array(z.unknown()).nullish().transform((v) => v ?? []),
  total: z.number().default(0),
  page: z.number().default(1),
  page_size: z.number().default(10),
})

/** 分类树递归 schema：children 缺失（omitempty）表示叶子节点。 */
export const categoryWireSchema: z.ZodType<CategoryWire> = z.lazy(() =>
  z.object({
    id: z.number().int().nonnegative(),
    slug: z.string().default(''),
    name: z.string().min(1),
    description: z.string().default(''),
    icon: z.string().default(''),
    children: z.array(z.unknown()).nullish().transform((v) => v ?? []),
  }),
)

interface CategoryWire {
  id: number
  slug: string
  name: string
  description: string
  icon: string
  children: unknown[]
}

export const categoryListWireSchema = z.object({
  items: z.array(z.unknown()).nullish().transform((v) => v ?? []),
})

export const friendlyLinkWireSchema = z.object({
  id: z.number().int().nonnegative(),
  name: z.string().min(1),
  url: z.string().min(1),
  logo: z.string().default(''),
  description: z.string().default(''),
  open_in_new: z.boolean().default(true),
})

export const friendlyLinkListWireSchema = z.object({
  items: z.array(z.unknown()).nullish().transform((v) => v ?? []),
})

/* ------------------------------------------------------------------ */
/* 展示模型                                                            */
/* ------------------------------------------------------------------ */

export interface ArticleSummary {
  id: number
  kind: ArticleKind
  slug: string
  title: string
  summary: string
  cover: string
  tags: string[]
  categoryId: number
  categorySlug: string
  categoryName: string
  pinned: boolean
  version: string
  /** 原始时间串（`2006-01-02 15:04:05`），展示格式化见 formatArticleDate。 */
  publishAt: string
  updatedAt: string
}

export interface ArticleDetail extends ArticleSummary {
  /** 服务端已净化的 HTML 正文，可直接 v-html。 */
  body: string
}

export interface ArticleCategory {
  id: number
  slug: string
  name: string
  description: string
  icon: string
  children: ArticleCategory[]
}

export interface FriendlyLink {
  id: number
  name: string
  url: string
  logo: string
  description: string
  openInNew: boolean
}

export interface ArticleList {
  items: ArticleSummary[]
  page: number
  pageSize: number
  total: number
}

export const EMPTY_ARTICLE_LIST: ArticleList = { items: [], page: 1, pageSize: 10, total: 0 }
export const EMPTY_ARTICLE_CATEGORIES: ArticleCategory[] = []
export const EMPTY_FRIENDLY_LINKS: FriendlyLink[] = []

/* ------------------------------------------------------------------ */
/* 展示辅助                                                            */
/* ------------------------------------------------------------------ */

/** 只保留日期部分（`2026-09-10 12:00:00` → `2026-09-10`），无法识别则原样返回。 */
export function formatArticleDate(value: string): string {
  const match = value.match(/^(\d{4}-\d{2}-\d{2})/)
  return match?.[1] ?? value
}

/**
 * 正文 → 摘要素材。
 *
 * 列表接口已经返回 summary，这里只作为兜底：运营没填摘要时列表会显示空白摘要，
 * 用正文剥标签得到一段可读文字比留空好。剥标签是尽力而为，不参与 XSS 防护。
 */
export function articleExcerpt(summary: string, body: string, maxLength = 120): string {
  const source = summary.trim() || body.replace(/<[^>]*>/g, ' ')
  const plain = source.replace(/\s+/g, ' ').trim()
  return plain.length > maxLength ? `${plain.slice(0, maxLength)}…` : plain
}

/** 在分类树中按 slug 找节点（用于面包屑与筛选回显），找不到返回 null。 */
export function findCategoryBySlug(
  nodes: ArticleCategory[],
  slug: string,
): ArticleCategory | null {
  if (!slug) return null
  for (const node of nodes) {
    if (node.slug === slug) return node
    const hit = findCategoryBySlug(node.children, slug)
    if (hit) return hit
  }
  return null
}

/** 展平分类树，保留层级信息以便侧栏缩进渲染。 */
export function flattenCategoryTree(
  nodes: ArticleCategory[],
  depth = 0,
): { node: ArticleCategory; depth: number }[] {
  const out: { node: ArticleCategory; depth: number }[] = []
  for (const node of nodes) {
    out.push({ node, depth })
    out.push(...flattenCategoryTree(node.children, depth + 1))
  }
  return out
}

/* ------------------------------------------------------------------ */
/* 解析入口                                                            */
/* ------------------------------------------------------------------ */

function normalizeTags(raw: unknown[]): string[] {
  return raw.filter((tag): tag is string => typeof tag === 'string' && tag.trim() !== '')
}

type ArticleItemWire = z.infer<typeof articleItemWireSchema>

function normalizeArticle(wire: ArticleItemWire): ArticleSummary | null {
  // kind 是后端枚举，出现未知值时丢弃该条：门户没有对应的展示形态，
  // 硬塞进列表只会得到点进去 404 的死链。
  if (!isArticleKind(wire.kind)) return null
  return {
    id: wire.id,
    kind: wire.kind,
    slug: wire.slug,
    title: wire.title,
    summary: wire.summary,
    cover: wire.cover,
    tags: normalizeTags(wire.tags),
    categoryId: wire.category_id,
    categorySlug: wire.category_slug,
    categoryName: wire.category_name,
    pinned: wire.pinned,
    version: wire.version,
    publishAt: wire.publish_at,
    updatedAt: wire.updated_at,
  }
}

/** 解析单条文章摘要；校验失败返回 null。 */
export function parseArticleSummary(raw: unknown): ArticleSummary | null {
  const parsed = articleItemWireSchema.safeParse(raw)
  return parsed.success ? normalizeArticle(parsed.data) : null
}

/** 解析文章详情；校验失败返回 null（由调用方判 404）。 */
export function parseArticleDetail(raw: unknown): ArticleDetail | null {
  const parsed = articleDetailWireSchema.safeParse(raw)
  if (!parsed.success) return null
  const base = normalizeArticle(parsed.data)
  return base ? { ...base, body: parsed.data.body } : null
}

/** 解析文章列表；逐条校验，坏数据被跳过。 */
export function parseArticleList(raw: unknown): ArticleList {
  const parsed = articleListWireSchema.safeParse(raw)
  if (!parsed.success) return EMPTY_ARTICLE_LIST

  const items: ArticleSummary[] = []
  for (const entry of parsed.data.items) {
    const article = parseArticleSummary(entry)
    if (article) items.push(article)
  }
  return {
    items,
    page: parsed.data.page,
    pageSize: parsed.data.page_size,
    total: parsed.data.total,
  }
}

function normalizeCategoryNode(raw: CategoryWire): ArticleCategory {
  const children: ArticleCategory[] = []
  for (const child of raw.children) {
    const parsed = categoryWireSchema.safeParse(child)
    if (parsed.success) children.push(normalizeCategoryNode(parsed.data))
  }
  return {
    id: raw.id,
    slug: raw.slug,
    name: raw.name,
    description: raw.description,
    icon: raw.icon,
    children,
  }
}

/** 解析分类树；坏节点被跳过而不是让整棵树消失。 */
export function parseArticleCategories(raw: unknown): ArticleCategory[] {
  const parsed = categoryListWireSchema.safeParse(raw)
  if (!parsed.success) return EMPTY_ARTICLE_CATEGORIES

  const out: ArticleCategory[] = []
  for (const entry of parsed.data.items) {
    const node = categoryWireSchema.safeParse(entry)
    if (node.success) out.push(normalizeCategoryNode(node.data))
  }
  return out
}

/** 解析友情链接；坏条目被跳过。 */
export function parseFriendlyLinks(raw: unknown): FriendlyLink[] {
  const parsed = friendlyLinkListWireSchema.safeParse(raw)
  if (!parsed.success) return EMPTY_FRIENDLY_LINKS

  const out: FriendlyLink[] = []
  for (const entry of parsed.data.items) {
    const link = friendlyLinkWireSchema.safeParse(entry)
    if (!link.success) continue
    out.push({
      id: link.data.id,
      name: link.data.name,
      url: link.data.url,
      logo: link.data.logo,
      description: link.data.description,
      openInNew: link.data.open_in_new,
    })
  }
  return out
}
