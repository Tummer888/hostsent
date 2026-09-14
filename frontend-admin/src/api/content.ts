// 内容中心（doc100）：文章（新闻/帮助/条款/隐私）/ 分类 / 友情链接。
//
// 与公告 API（@/api/notification.ts）分开的原因：公告带「平台定向 / 等级 / 弹窗」的
// 站内消息语义，走 notify:* 权限；这里是纯展示型内容，走 content:* 权限。
import { request } from '@/utils/request'

export type ArticleKind = 'news' | 'help' | 'terms' | 'privacy'
export type ArticleStatus = 'draft' | 'published' | 'offline'
export type CategoryStatus = 'active' | 'disabled'

export interface ArticleItem {
  id: number
  kind: ArticleKind
  category_id: number
  category_name: string
  slug: string
  title: string
  summary: string
  body: string
  body_format: 'html' | 'text'
  cover: string
  tags: string
  status: ArticleStatus
  pinned: boolean
  sort_order: number
  version: string
  publish_at: string
  offline_at: string
  operator_id: number
  created_at: string
  updated_at: string
}

export interface ArticleListResponse {
  total: number
  list: ArticleItem[]
  page: number
  page_size: number
}

export interface ArticleListQuery {
  kind?: string
  category_id?: number
  status?: string
  keyword?: string
  page?: number
  page_size?: number
  // request.get 的 params 形参是 Record<string, unknown>，命名接口不带隐式索引签名，
  // 这里显式声明（与 api/logcenter.ts 的 LogQueryParams 同做法）才能直接传对象。
  [key: string]: string | number | undefined
}

export interface ArticleSaveRequest {
  kind: ArticleKind
  category_id?: number
  slug?: string
  title: string
  summary?: string
  body: string
  body_format?: 'html' | 'text'
  cover?: string
  tags?: string
  pinned?: boolean
  sort_order?: number
  version?: string
  publish_at?: string
}

export interface CategoryItem {
  id: number
  kind: 'news' | 'help'
  parent_id: number
  name: string
  slug: string
  description: string
  icon: string
  sort_order: number
  status: CategoryStatus
  children?: CategoryItem[]
}

export interface CategorySaveRequest {
  kind: 'news' | 'help'
  parent_id?: number
  name: string
  slug?: string
  description?: string
  icon?: string
  sort_order?: number
  status?: CategoryStatus
}

export interface FriendlyLinkItem {
  id: number
  name: string
  url: string
  logo: string
  description: string
  sort_order: number
  open_in_new: boolean
  status: CategoryStatus
  created_at: string
  updated_at: string
}

export interface FriendlyLinkListResponse {
  total: number
  list: FriendlyLinkItem[]
  page: number
  page_size: number
}

export interface LinkSaveRequest {
  name: string
  url: string
  logo?: string
  description?: string
  sort_order?: number
  open_in_new?: boolean
  status?: CategoryStatus
}

// ---------- 文章 ----------

export function getArticles(params: ArticleListQuery): Promise<ArticleListResponse> {
  return request.get<ArticleListResponse>({ url: '/content/articles', params })
}

export function getArticle(id: number): Promise<ArticleItem> {
  return request.get<ArticleItem>({ url: `/content/articles/${id}` })
}

export function createArticle(data: ArticleSaveRequest): Promise<ArticleItem> {
  return request.post<ArticleItem>({ url: '/content/articles', data })
}

export function updateArticle(id: number, data: ArticleSaveRequest): Promise<ArticleItem> {
  return request.put<ArticleItem>({ url: `/content/articles/${id}`, data })
}

export function publishArticle(id: number): Promise<ArticleItem> {
  return request.post<ArticleItem>({ url: `/content/articles/${id}/publish` })
}

export function offlineArticle(id: number): Promise<ArticleItem> {
  return request.post<ArticleItem>({ url: `/content/articles/${id}/offline` })
}

export function deleteArticle(id: number): Promise<string> {
  return request.delete<string>({ url: `/content/articles/${id}` })
}

// ---------- 分类 ----------

export interface CategoryListResponse {
  items: CategoryItem[]
}

export function getContentCategories(params: { kind?: string; status?: string }): Promise<CategoryListResponse> {
  return request.get<CategoryListResponse>({ url: '/content/categories', params })
}

export function createContentCategory(data: CategorySaveRequest): Promise<CategoryItem> {
  return request.post<CategoryItem>({ url: '/content/categories', data })
}

export function updateContentCategory(id: number, data: CategorySaveRequest): Promise<CategoryItem> {
  return request.put<CategoryItem>({ url: `/content/categories/${id}`, data })
}

export function deleteContentCategory(id: number): Promise<string> {
  return request.delete<string>({ url: `/content/categories/${id}` })
}

// ---------- 友情链接 ----------

export function getFriendlyLinks(params: {
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}): Promise<FriendlyLinkListResponse> {
  return request.get<FriendlyLinkListResponse>({ url: '/content/links', params })
}

export function createFriendlyLink(data: LinkSaveRequest): Promise<FriendlyLinkItem> {
  return request.post<FriendlyLinkItem>({ url: '/content/links', data })
}

export function updateFriendlyLink(id: number, data: LinkSaveRequest): Promise<FriendlyLinkItem> {
  return request.put<FriendlyLinkItem>({ url: `/content/links/${id}`, data })
}

export function deleteFriendlyLink(id: number): Promise<string> {
  return request.delete<string>({ url: `/content/links/${id}` })
}
