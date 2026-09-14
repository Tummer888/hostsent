import request from '@/utils/request'

// 用户中心 · 通知与公告接口（baseURL=/api/v1，路径拼接为 /api/v1/uc/notifications/*）
// 注意：本项目 request 拦截器返回的是完整信封 { code, message, data }，因此各函数返回 .data。

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

// 通知信息
export interface NotificationInfo {
  id: number
  user_id: number
  event: string
  title: string
  content: string
  is_read: boolean
  read_at: string
  created_at: string
}

// 通知列表响应：data = { total, list, page, page_size }
export interface NotificationListResponse {
  list: NotificationInfo[]
  page: number
  page_size: number
  total: number
}

// 公告信息
//
// 契约来源：后端 GET /api/v1/uc/announcements 返回 data = { list: [...] }，
// 时间字段是 publish_at（与公告 DTO 一致）。此前这里写成裸数组 + published_at，
// 于是 pages/profile/messages.vue 取 data ?? [] 恒为空、发布时间恒为空 —— 公告 tab 一直白着。
export interface AnnouncementInfo {
  id: number
  title: string
  content: string
  /** text 为存量纯文本（按文本节点渲染）；html 为服务端已净化的富文本 */
  body_format: string
  level: string // info / warning / critical
  popup: boolean
  pinned: boolean
  publish_at: string
}

// 公告列表响应：data = { list: [...] }
export interface AnnouncementListResponse {
  list: AnnouncementInfo[]
}

// 通知偏好项
export interface NotificationPreference {
  event: string
  inbox_on: boolean
  mail_on: boolean
  /** 短信通知（doc90 渠道体系新增；模板未配短信时该值无实际作用） */
  sms_on: boolean
  /** 强制送达事件（OTP/验证码类）：开关不可关闭，前端置灰并提示 */
  mandatory: boolean
}

// 通知偏好列表响应：data = { list: [...] }
export interface NotificationPreferenceListResponse {
  list: NotificationPreference[]
}

// 未读数量响应
export interface UnreadCountResponse {
  count: number
}

// 查询未读通知数量
export const getUnreadCount = () =>
  request.get<any, { data: UnreadCountResponse }>('/uc/notifications/unread-count')

// 查询我的通知列表
export const getMyNotifications = (params: {
  page?: number
  page_size?: number
  event?: string
  is_read?: boolean
} = {}) => request.get<any, { data: NotificationListResponse }>('/uc/notifications', { params })

// 查询通知详情
export const getNotificationDetail = (id: number) =>
  request.get<any, { data: NotificationInfo }>(`/uc/notifications/${id}`)

// 全部标记为已读
export const readAll = () => request.post<any, void>('/uc/notifications/read-all')

// 查询我的公告列表
export const getMyAnnouncements = () =>
  request.get<any, { data: AnnouncementListResponse }>('/uc/announcements')

// 查询我的通知偏好
export const getMyPreferences = () =>
  request.get<any, { data: NotificationPreferenceListResponse }>('/uc/notification-preferences')

// 批量更新通知偏好
export const updateMyPreferences = (data: { list: NotificationPreference[] }) =>
  request.put<any, void>('/uc/notification-preferences', data)
