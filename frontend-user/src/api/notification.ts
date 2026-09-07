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
export interface AnnouncementInfo {
  id: number
  title: string
  content: string
  level: string // info / warning / critical
  published_at: string
  created_at: string
}

// 通知偏好项
export interface NotificationPreference {
  event: string
  inbox_on: boolean
  mail_on: boolean
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
  request.get<any, { data: AnnouncementInfo[] }>('/uc/announcements')

// 查询我的通知偏好
export const getMyPreferences = () =>
  request.get<any, { data: NotificationPreferenceListResponse }>('/uc/notification-preferences')

// 批量更新通知偏好
export const updateMyPreferences = (data: { list: NotificationPreference[] }) =>
  request.put<any, void>('/uc/notification-preferences', data)
