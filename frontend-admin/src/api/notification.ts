import { request } from '@/utils/request'

// ===== 消息中心（公告 / 通知记录 / 通知模板）=====

// ---------- 公告 ----------
export interface AnnouncementItem {
  id: number
  title: string
  content: string
  platform: 'user' | 'admin' | 'both'
  level: 'info' | 'warning' | 'critical'
  popup: boolean
  status: 'draft' | 'published' | 'offline'
  publish_at: string
  created_at: string
  updated_at: string
}

export interface AnnouncementListResponse {
  total: number
  list: AnnouncementItem[]
  page: number
  page_size: number
}

export interface AnnouncementListQuery {
  keyword?: string
  status?: string
  platform?: string
  page?: number
  page_size?: number
}

export interface AnnouncementSaveRequest {
  title: string
  content: string
  platform: 'user' | 'admin' | 'both'
  level: 'info' | 'warning' | 'critical'
  popup: boolean
  publish_at?: string
}

// 查询公告列表（分页）
export function getAnnouncements(params: AnnouncementListQuery): Promise<AnnouncementListResponse> {
  return request.get<AnnouncementListResponse>({
    url: '/announcements',
    params: {
      keyword: params.keyword,
      status: params.status,
      platform: params.platform,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// 创建公告
export function createAnnouncement(data: AnnouncementSaveRequest): Promise<AnnouncementItem> {
  return request.post<AnnouncementItem>({ url: '/announcements', data })
}

// 更新公告
export function updateAnnouncement(id: number, data: AnnouncementSaveRequest): Promise<AnnouncementItem> {
  return request.put<AnnouncementItem>({ url: `/announcements/${id}`, data })
}

// 发布公告
export function publishAnnouncement(id: number): Promise<AnnouncementItem> {
  return request.post<AnnouncementItem>({ url: `/announcements/${id}/publish` })
}

// 下线公告
export function offlineAnnouncement(id: number): Promise<AnnouncementItem> {
  return request.post<AnnouncementItem>({ url: `/announcements/${id}/offline` })
}

// 删除公告
export function deleteAnnouncement(id: number): Promise<string> {
  return request.delete<string>({ url: `/announcements/${id}` })
}

// ---------- 通知记录 ----------
export interface NotifyRecordItem {
  id: number
  target_type: 'user' | 'admin'
  target_id: number
  target_name: string
  event: string
  title: string
  content: string
  channel: 'inbox' | 'mail'
  send_status: 'sent' | 'failed' | 'pending'
  fail_reason: string
  source_module: string
  created_at: string
  sent_at: string
}

export interface NotifyRecordListResponse {
  total: number
  list: NotifyRecordItem[]
  page: number
  page_size: number
}

export interface NotifyRecordListQuery {
  event?: string
  channel?: string
  send_status?: string
  target_type?: string
  keyword?: string
  page?: number
  page_size?: number
}

// 查询通知记录列表（分页）
export function getNotifyRecords(params: NotifyRecordListQuery): Promise<NotifyRecordListResponse> {
  return request.get<NotifyRecordListResponse>({
    url: '/notifications',
    params: {
      event: params.event,
      channel: params.channel,
      send_status: params.send_status,
      target_type: params.target_type,
      keyword: params.keyword,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

// 重发通知（仅失败邮件）
export function resendNotify(id: number): Promise<string> {
  return request.post<string>({ url: `/notifications/${id}/resend` })
}

// 管理员未读消息数
export function getAdminUnreadCount(): Promise<{ count: number }> {
  return request.get<{ count: number }>({ url: '/notifications/unread-count' })
}

// 发送测试邮件
export function sendMailTest(data: { to: string }): Promise<string> {
  return request.post<string>({ url: '/notifications/mail-test', data })
}

// ---------- 通知模板 ----------
export interface NotificationTemplateItem {
  id: number
  event: string
  title_tpl: string
  content_tpl: string
  inbox_on: boolean
  mail_on: boolean
  status: 'active' | 'disabled'
  updated_at: string
}

export interface NotificationTemplateSaveRequest {
  title_tpl?: string
  content_tpl?: string
  inbox_on?: boolean
  mail_on?: boolean
  status?: 'active' | 'disabled'
}

// 查询通知模板列表
export async function getTemplates(): Promise<NotificationTemplateItem[]> {
  const resp = await request.get<{ list: NotificationTemplateItem[] }>({ url: '/notification-templates' })
  return resp?.list ?? []
}

// 更新通知模板
export function updateTemplate(
  id: number,
  data: NotificationTemplateSaveRequest,
): Promise<NotificationTemplateItem> {
  return request.put<NotificationTemplateItem>({ url: `/notification-templates/${id}`, data })
}
