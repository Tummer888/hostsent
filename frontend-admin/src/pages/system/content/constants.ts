// 内容中心常量：类型/状态的中文名与主题色集中一处，三个页面共用。

import type { ArticleKind, ArticleStatus, CategoryStatus } from '@/api/content'

export interface LabelTheme {
  label: string
  theme: string
}

/** 内容类型（与后端 model.Kind* 一一对应）。 */
export const KIND_OPTIONS: Array<{ label: string; value: ArticleKind }> = [
  { label: '新闻资讯', value: 'news' },
  { label: '帮助文档', value: 'help' },
  { label: '用户条款', value: 'terms' },
  { label: '隐私政策', value: 'privacy' },
]

const KIND_MAP: Record<string, LabelTheme> = {
  news: { label: '新闻资讯', theme: 'primary' },
  help: { label: '帮助文档', theme: 'success' },
  terms: { label: '用户条款', theme: 'warning' },
  privacy: { label: '隐私政策', theme: 'danger' },
}

export function kindLabel(kind: string): string {
  return KIND_MAP[kind]?.label || kind
}

export function kindTheme(kind: string): string {
  return KIND_MAP[kind]?.theme || 'default'
}

/** 门户落点：运营需要知道内容会出现在哪个页面。 */
const KIND_PORTAL_PATH: Record<string, string> = {
  news: '/news',
  help: '/help',
  terms: '/terms',
  privacy: '/privacy',
}

export function kindPortalPath(kind: string): string {
  return KIND_PORTAL_PATH[kind] || ''
}

export const ARTICLE_STATUS_OPTIONS: Array<{ label: string; value: ArticleStatus }> = [
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已下线', value: 'offline' },
]

const ARTICLE_STATUS_MAP: Record<string, LabelTheme> = {
  draft: { label: '草稿', theme: 'default' },
  published: { label: '已发布', theme: 'success' },
  offline: { label: '已下线', theme: 'warning' },
}

export function articleStatusLabel(status: string): string {
  return ARTICLE_STATUS_MAP[status]?.label || status
}

export function articleStatusTheme(status: string): string {
  return ARTICLE_STATUS_MAP[status]?.theme || 'default'
}

const CATEGORY_STATUS_MAP: Record<string, LabelTheme> = {
  active: { label: '启用', theme: 'success' },
  disabled: { label: '停用', theme: 'default' },
}

export function categoryStatusLabel(status: string): string {
  return CATEGORY_STATUS_MAP[status]?.label || status
}

export function categoryStatusTheme(status: string): string {
  return CATEGORY_STATUS_MAP[status]?.theme || 'default'
}

export function categoryStatusOptions(): Array<{ label: string; value: CategoryStatus }> {
  return [
    { label: '启用', value: 'active' },
    { label: '停用', value: 'disabled' },
  ]
}

/** 该类型是否使用分类（与后端 model.KindNeedsCategory 保持一致）。 */
export function kindNeedsCategory(kind: string): boolean {
  return kind === 'news' || kind === 'help'
}

/** 该类型是否每类型只有一篇（条款 / 隐私政策）。 */
export function kindIsSingleton(kind: string): boolean {
  return kind === 'terms' || kind === 'privacy'
}

/** 统一的时间展示：后端给的是 `2006-01-02 15:04:05`，兼容 ISO 串。 */
export function formatTime(value?: string): string {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}
