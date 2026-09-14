/**
 * 公告展示 schema —— 后端公开公告接口的 wire 契约（snake_case）到展示模型（camelCase）。
 *
 * 上游契约来源：`backend/internal/modules/uc/site/dto/site.go`
 * （仅返回已发布且未下线的公告，不含 operator_id 等内部字段）。
 */
import { z } from 'zod'

export const announcementWireSchema = z.object({
  id: z.number().int().nonnegative(),
  title: z.string().min(1),
  content: z.string().default(''),
  // html 表示服务端已净化的富文本，可直接 v-html；text 是存量纯文本，必须按文本节点输出。
  // 早期公告没有这个字段，缺失时按 text 处理 —— 那些内容确实是纯文本。
  body_format: z.string().default('text'),
  level: z.string().default('info'),
  popup: z.boolean().default(false),
  pinned: z.boolean().default(false),
  publish_at: z.string().default(''),
})

export const announcementListWireSchema = z.object({
  items: z.array(z.unknown()).nullish().transform((v) => v ?? []),
})

export type AnnouncementLevel = 'info' | 'warning' | 'critical'

export interface Announcement {
  id: number
  title: string
  content: string
  /** true 表示 content 是已净化的 HTML；false 表示纯文本，渲染时不做 HTML 解析。 */
  isHtml: boolean
  level: AnnouncementLevel
  popup: boolean
  pinned: boolean
  /** 原始时间串（`2006-01-02 15:04:05`），展示格式化见 `formatAnnouncementDate`。 */
  publishedAt: string
}

export const EMPTY_ANNOUNCEMENT_LIST: Announcement[] = []

/** 公告等级标签文案与样式键；未知等级按普通处理。 */
export function announcementLevelMeta(level: string): { label: string; tone: AnnouncementLevel } {
  switch (level) {
    case 'warning':
      return { label: '预警', tone: 'warning' }
    case 'critical':
      return { label: '重要', tone: 'critical' }
    default:
      return { label: '公告', tone: 'info' }
  }
}

/** 只保留日期部分（`2026-09-10 12:00:00` → `2026-09-10`），无法识别则原样返回。 */
export function formatAnnouncementDate(publishedAt: string): string {
  const match = publishedAt.match(/^(\d{4}-\d{2}-\d{2})/)
  return match?.[1] ?? publishedAt
}

/** 纯文本摘要，避免公告内容中的 HTML 直接渲染。 */
export function announcementExcerpt(content: string, maxLength = 80): string {
  const plain = content.replace(/<[^>]*>/g, '').replace(/\s+/g, ' ').trim()
  return plain.length > maxLength ? `${plain.slice(0, maxLength)}…` : plain
}

function normalizeLevel(level: string): AnnouncementLevel {
  return level === 'warning' || level === 'critical' ? level : 'info'
}

/** 解析单条公告；校验失败返回 null。 */
export function parseAnnouncement(raw: unknown): Announcement | null {
  const parsed = announcementWireSchema.safeParse(raw)
  if (!parsed.success) return null
  const wire = parsed.data
  return {
    id: wire.id,
    title: wire.title,
    content: wire.content,
    isHtml: wire.body_format === 'html',
    level: normalizeLevel(wire.level),
    popup: wire.popup,
    pinned: wire.pinned,
    publishedAt: wire.publish_at,
  }
}

/** 解析公告列表；逐条校验，坏数据被跳过。 */
export function parseAnnouncementList(raw: unknown): Announcement[] {
  const parsed = announcementListWireSchema.safeParse(raw)
  if (!parsed.success) return EMPTY_ANNOUNCEMENT_LIST

  const items: Announcement[] = []
  for (const entry of parsed.data.items) {
    const announcement = parseAnnouncement(entry)
    if (announcement) items.push(announcement)
  }
  return items
}
