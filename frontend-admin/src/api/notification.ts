import { request } from '@/utils/request'

import type { ListMeta, PaymentField } from '@/types/interface'

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
  user_id: number
  target_name: string
  event: string
  title: string
  content: string
  channel: 'inbox' | 'mail' | 'sms'
  send_status: 'sent' | 'failed' | 'pending'
  fail_reason: string
  source_module: string
  source_id: string
  /** text / html：html 时按富文本渲染（doc90 §8.6） */
  content_format: 'text' | 'html'
  /** 关联的外发投递记录 ID；0 = 纯站内信，不显示重发入口 */
  delivery_id: number
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

// 重发通知：内部转发到投递记录重投（doc90 §5.3 保留旧接口不 400）
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
  sms_on: boolean
  sms_template_id: number
  mail_format: 'text' | 'html'
  title_show: boolean
  status: 'active' | 'disabled'
  updated_at: string
}

export interface NotificationTemplateSaveRequest {
  title_tpl?: string
  content_tpl?: string
  inbox_on?: boolean
  mail_on?: boolean
  sms_on?: boolean
  sms_template_id?: number
  mail_format?: 'text' | 'html'
  title_show?: boolean
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

// ===== 消息中心多渠道（doc90）=====
// 全部挂在 /notification/* 前缀，与既有的 /notifications、/notification-templates 互不影响。

// ---------- 渠道类型（descriptor 驱动动态凭证表单）----------

/** 后端 notifier.CapabilityDescriptor：凭证字段声明驱动前端表单 */
export interface NotifyChannelCapability {
  type: string
  name: string
  category: 'mail' | 'sms'
  mode: string
  credential_schema: PaymentField[] | null
  endpoint_schema: PaymentField[] | null
  icon: string
  doc_url: string
  adapter_version: string
  implemented: boolean
}

export interface NotifyChannelTypeItem {
  type: string
  name: string
  category: 'mail' | 'sms'
  mode: string
  icon: string
  doc_url: string
  adapter_version: string
  /** false = 适配器待接入，可创建渠道但测试发送返回 pending */
  implemented: boolean
  capabilities: NotifyChannelCapability
}

// 渠道类型列表（可按 category=mail/sms 过滤）
export function getNotifyChannelTypes(category?: string): Promise<NotifyChannelTypeItem[]> {
  return request
    .get<{ items: NotifyChannelTypeItem[] }>({
      url: '/notification/channel-types',
      params: { category },
    })
    .then((resp) => resp?.items ?? [])
}

// ---------- 渠道实例 ----------

export interface NotifyChannelInfo {
  id: number
  channel_code: string
  name: string
  category: 'mail' | 'sms'
  type: string
  type_name: string
  /** 脱敏回显：secret 字段为掩码，原样回传表示不修改 */
  credentials: Record<string, string>
  endpoint: string
  sign_name: string
  sender: string
  template_code: string
  scenes: string[]
  priority: number
  weight: number
  daily_limit: number
  health_status: string
  last_error: string
  last_check_at: string
  status: number
  is_default: boolean
  remark: string
  implemented: boolean
  capabilities: NotifyChannelCapability
  created_at: string
  updated_at: string
}

export interface NotifyChannelListResponse {
  items: NotifyChannelInfo[]
  meta: ListMeta
}

export interface NotifyChannelSaveRequest {
  channel_code?: string
  name: string
  type?: string
  credentials?: Record<string, string>
  endpoint?: string
  sign_name?: string
  sender?: string
  template_code?: string
  scenes?: string[]
  priority?: number
  weight?: number
  daily_limit?: number
  is_default?: boolean
  remark?: string
}

export function getNotifyChannels(params: {
  category?: string
  type?: string
  status?: number
  keyword?: string
  page?: number
  page_size?: number
}): Promise<NotifyChannelListResponse> {
  return request.get<NotifyChannelListResponse>({ url: '/notification/channels', params })
}

export function createNotifyChannel(data: NotifyChannelSaveRequest): Promise<NotifyChannelInfo> {
  return request.post<NotifyChannelInfo>({ url: '/notification/channels', data })
}

export function updateNotifyChannel(id: number, data: NotifyChannelSaveRequest): Promise<NotifyChannelInfo> {
  return request.put<NotifyChannelInfo>({ url: `/notification/channels/${id}`, data })
}

export function updateNotifyChannelStatus(id: number, status: number): Promise<void> {
  return request.patch<void>({ url: `/notification/channels/${id}/status`, data: { status } })
}

export interface NotifyChannelTestResponse {
  ok: boolean
  /** true = 该服务商适配器待接入（配置已保存，不算失败） */
  pending: boolean
  message: string
}

export function testNotifyChannel(id: number, target?: string): Promise<NotifyChannelTestResponse> {
  return request.post<NotifyChannelTestResponse>({
    url: `/notification/channels/${id}/test`,
    data: { target: target || '' },
  })
}

export interface NotifyTestSendResponse extends NotifyChannelTestResponse {
  channel_name: string
  channel_id: number
  provider_code: string
  provider_msg_id: string
  cost_fen: number
  raw: string
}

// 统一测试发送（频控 10 次/分钟/管理员）
export function notifyTestSend(data: {
  category: 'mail' | 'sms'
  channel_id?: number
  template_code?: string
  target: string
  vars?: Record<string, string>
  content?: string
}): Promise<NotifyTestSendResponse> {
  return request.post<NotifyTestSendResponse>({ url: '/notification/test-send', data })
}

// ---------- 短信模板 ----------

export interface SmsTemplateItem {
  id: number
  code: string
  name: string
  scene: string
  content: string
  upstream_code: string
  var_names: string[]
  status: string
  remark: string
  char_count: number
  segments: number
  created_at: string
  updated_at: string
}

export interface SmsTemplateListResponse {
  items: SmsTemplateItem[]
  meta: ListMeta
}

export interface SmsTemplateSaveRequest {
  code?: string
  name: string
  scene?: string
  content: string
  upstream_code?: string
  status?: string
  remark?: string
}

export function getSmsTemplates(params: {
  scene?: string
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}): Promise<SmsTemplateListResponse> {
  return request.get<SmsTemplateListResponse>({ url: '/notification/sms-templates', params })
}

export function createSmsTemplate(data: SmsTemplateSaveRequest): Promise<SmsTemplateItem> {
  return request.post<SmsTemplateItem>({ url: '/notification/sms-templates', data })
}

export function updateSmsTemplate(id: number, data: SmsTemplateSaveRequest): Promise<SmsTemplateItem> {
  return request.put<SmsTemplateItem>({ url: `/notification/sms-templates/${id}`, data })
}

export function deleteSmsTemplate(id: number): Promise<void> {
  return request.delete<void>({ url: `/notification/sms-templates/${id}` })
}

export interface SmsTemplatePreviewResponse {
  rendered: string
  char_count: number
  segments: number
}

export function previewSmsTemplate(data: {
  content: string
  vars?: Record<string, string>
}): Promise<SmsTemplatePreviewResponse> {
  return request.post<SmsTemplatePreviewResponse>({ url: '/notification/sms-templates/preview', data })
}

// ---------- 模板变量注册表（D7）----------

export interface TemplateVarItem {
  id: number
  var_key: string
  label: string
  category: string
  value_type: string
  sample: string
  description: string
  scenes: string[]
  sort_order: number
  status: string
}

export function getTemplateVars(): Promise<TemplateVarItem[]> {
  return request
    .get<{ items: TemplateVarItem[] }>({ url: '/notification/template-vars' })
    .then((resp) => resp?.items ?? [])
}

export function createTemplateVar(data: Partial<TemplateVarItem>): Promise<TemplateVarItem> {
  return request.post<TemplateVarItem>({ url: '/notification/template-vars', data })
}

export function updateTemplateVar(id: number, data: Partial<TemplateVarItem>): Promise<TemplateVarItem> {
  return request.put<TemplateVarItem>({ url: `/notification/template-vars/${id}`, data })
}

export function deleteTemplateVar(id: number): Promise<void> {
  return request.delete<void>({ url: `/notification/template-vars/${id}` })
}

// ---------- 消息群发 ----------

export interface BroadcastGroupItem {
  id: number
  name: string
  member_count: number
}

export interface BroadcastUserItem {
  id: number
  username: string
  email_masked: string
  phone_masked: string
}

export interface BroadcastTargetResponse {
  total: number
  items?: BroadcastUserItem[]
  groups?: BroadcastGroupItem[]
}

export interface BroadcastFilter {
  registered_after?: string
  registered_before?: string
  tier?: string
  has_instance?: boolean
  min_balance?: string
  max_balance?: string
  status?: string
}

export interface BroadcastTarget {
  mode: 'all' | 'group' | 'users' | 'filter'
  user_group_id?: number
  user_ids?: number[]
  filter?: BroadcastFilter
}

export function getBroadcastTargets(params: {
  /** 检索模式：groups 用户组列表 / users 用户搜索 / filter 条件计数（注意与发送 target.mode 不同名） */
  mode: 'groups' | 'users' | 'filter'
  keyword?: string
  page?: number
  page_size?: number
} & BroadcastFilter): Promise<BroadcastTargetResponse> {
  return request.get<BroadcastTargetResponse>({
    url: '/notification/broadcast/targets',
    // request.get 的 params 要求可索引对象；这里显式断言（键均为后端 form tag）。
    params: params as unknown as Record<string, unknown>,
  })
}

export interface BroadcastPreviewResponse {
  total: number
  sample: BroadcastUserItem[]
  rendered: { title: string; content: string }
  estimated_sms_cost_fen: number
  estimated_mail_count: number
  exceeded: boolean
  max_targets: number
  message: string
}

export interface BroadcastPayload {
  title: string
  content: string
  format?: string
  channels: string[]
  sms_template_code?: string
  vars?: Record<string, string>
  per_user_vars?: boolean
  target: BroadcastTarget
  schedule_at?: string
}

export function previewBroadcast(data: BroadcastPayload): Promise<BroadcastPreviewResponse> {
  return request.post<BroadcastPreviewResponse>({ url: '/notification/broadcast/preview', data })
}

export interface BroadcastResponse {
  batch_id: string
  total: number
  queued: number
  message: string
}

export function sendBroadcast(data: BroadcastPayload): Promise<BroadcastResponse> {
  return request.post<BroadcastResponse>({ url: '/notification/broadcast', data })
}

// ---------- 发送日志（投递记录）----------

export interface DeliveryItem {
  id: number
  batch_id: string
  event: string
  channel: 'inbox' | 'mail' | 'sms'
  target_type: string
  target_id: number
  target_name: string
  recipient: string
  channel_id: number
  title: string
  content: string
  content_format: string
  vars: Record<string, string> | null
  send_status: 'pending' | 'sending' | 'sent' | 'failed' | 'skipped' | 'dead'
  attempts: number
  max_attempts: number
  next_retry_at: string
  provider_msg_id: string
  provider_code: string
  cost_fen: number
  fail_reason: string
  source_module: string
  source_id: string
  sent_at: string
  created_at: string
}

export interface DeliveryListResponse {
  items: DeliveryItem[]
  meta: ListMeta
}

export interface DeliveryListQuery extends Record<string, unknown> {
  event?: string
  channel?: string
  send_status?: string
  target_type?: string
  batch_id?: string
  keyword?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export function getDeliveries(params: DeliveryListQuery): Promise<DeliveryListResponse> {
  return request.get<DeliveryListResponse>({ url: '/notification/deliveries', params })
}

export function getDelivery(id: number): Promise<DeliveryItem> {
  return request.get<DeliveryItem>({ url: `/notification/deliveries/${id}` })
}

export function retryDelivery(id: number): Promise<string> {
  return request.post<string>({ url: `/notification/deliveries/${id}/retry` })
}

export interface DeliveryRetryResponse {
  retried: number
  skipped: string[]
}

export function batchRetryDeliveries(ids: number[]): Promise<DeliveryRetryResponse> {
  return request.post<DeliveryRetryResponse>({ url: '/notification/deliveries/batch-retry', data: { ids } })
}
