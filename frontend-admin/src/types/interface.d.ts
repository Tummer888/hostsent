export interface UserInfo {
  id: number
  name: string
  username: string
  role: string
  roles: string[]
  email: string
  phone: string
  status: string
  avatar?: string
  department?: string
}

// ===== 资源管理 - 上游提供商 =====

export interface ProviderListQuery {
  page?: number
  page_size?: number
  keyword?: string
  provider_type?: string
  status?: number
}

export interface ProviderCreateRequest {
  name: string
  provider_type: string
  api_endpoint: string
  api_key?: string
  api_secret?: string
  region?: string
  status?: number
  sync_enabled?: boolean
  sync_interval?: number
}

export interface ProviderUpdateRequest {
  name?: string
  api_endpoint?: string
  api_key?: string
  api_secret?: string
  region?: string
  status?: number
  sync_enabled?: boolean
  sync_interval?: number
}

export interface ProviderInfo {
  id: number
  name: string
  provider_type: string
  api_endpoint: string
  api_key: string
  api_secret: string
  region: string
  status: number
  sync_enabled: boolean
  sync_interval: number
  last_sync_at: string | null
  total_cpu: number
  total_memory: number
  total_disk: number
  used_cpu: number
  used_memory: number
  used_disk: number
  created_at: string
  updated_at: string
}

export interface ListMeta {
  page: number
  page_size: number
  total: number
}

export interface ProviderListResponse {
  items: ProviderInfo[]
  meta: ListMeta
}

export interface ProviderTypeItem {
  type: string
  name: string
}

export interface TestConnectionResult {
  success: boolean
  message: string
}

// ===== 资源管理 - 资源池 =====

export interface PoolListQuery {
  provider_id?: number
  page?: number
  page_size?: number
}

export interface PoolInfo {
  id: number
  provider_id: number
  upstream_id: string
  name: string
  pool_type: string
  total_cpu: number
  total_memory: number
  total_disk: number
  used_cpu: number
  used_memory: number
  used_disk: number
  status: number
  last_sync_at: string | null
}

export interface PoolListResponse {
  items: PoolInfo[]
  meta: ListMeta
}

// ===== 资源管理 - 上游商品 =====

export interface ProductListQuery {
  keyword?: string
  provider_id?: number
  status?: number
  page?: number
  page_size?: number
}

export interface ProductPriceRequest {
  cost_price: number
  sale_price: number
}

export interface ProductInfo {
  id: number
  provider_id: number
  upstream_id: string
  name: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  os: string
  region: string
  zone: string
  specs: string
  raw_specs: string
  cost_price: number
  sale_price: number
  status: number
  created_at: string
  updated_at: string
}

export interface ProductListResponse {
  items: ProductInfo[]
  meta: ListMeta
}

export interface SyncResult {
  task_id: number
  status: string
  message: string
}

// ===== 资源管理 - 同步任务/日志/实例 =====

export interface SyncTaskListQuery {
  provider_id?: number
  task_type?: string
  status?: string
  page?: number
  page_size?: number
}

export interface SyncLogListQuery {
  provider_id?: number
  task_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface InstanceListQuery {
  provider_id?: number
  user_id?: number
  status?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface CreateSyncTaskRequest {
  provider_id: number
  task_type: string
}

export interface SyncTaskInfo {
  id: number
  provider_id: number
  task_type: string
  status: string
  total_count: number
  success_count: number
  error_message: string
  started_at: string | null
  completed_at: string | null
  created_at: string
}

export interface SyncTaskListResponse {
  items: SyncTaskInfo[]
  meta: ListMeta
}

export interface SyncLogInfo {
  id: number
  task_id: number
  provider_id: number
  sync_type: string
  status: string
  total_count: number
  success_count: number
  error_message: string
  details: string
  created_at: string
}

export interface SyncLogListResponse {
  items: SyncLogInfo[]
  meta: ListMeta
}

export interface InstanceInfo {
  id: number
  instance_id: string
  provider_id: number
  user_id: number
  product_id: number
  name: string
  cpu: number
  memory: number
  disk: number
  disk_type: string
  bandwidth: number
  os: string
  region: string
  zone: string
  status: string
  private_ip: string
  public_ip: string
  billing_mode: string
  expire_at: string | null
  created_at: string
}

export interface InstanceListResponse {
  items: InstanceInfo[]
  meta: ListMeta
}

// ===== 产品管理 - 产品 =====

export interface SaleProductListQuery {
  keyword?: string
  category_id?: number
  status?: number
  page?: number
  page_size?: number
}

export interface SaleProductCreateRequest {
  code: string
  name: string
  category_id?: number
  product_type?: string
  description?: string
  specs?: string
  price_model?: string
  price?: number
  cost_price?: number
  source_product_id?: number
  source_provider_id?: number
  stock?: number
  sort_order?: number
  status?: number
}

export interface SaleProductUpdateRequest {
  name: string
  category_id?: number
  product_type?: string
  description?: string
  specs?: string
  price_model?: string
  price?: number
  cost_price?: number
  stock?: number
  sort_order?: number
  status?: number
}

export interface SaleProductPriceRequest {
  price: number
  cost_price: number
  remark?: string
}

export interface SaleProductInfo {
  id: number
  code: string
  name: string
  category_id: number
  product_type: string
  description: string
  specs: string
  price_model: string
  price: number
  cost_price: number
  source_product_id: number
  source_provider_id: number
  stock: number
  sort_order: number
  status: number
  created_at: string
  updated_at: string
}

export interface SaleProductListResponse {
  items: SaleProductInfo[]
  meta: ListMeta
}

export interface SaleProductSpecInfo {
  id: number
  product_id: number
  spec_code: string
  name: string
  specs: string
  price_model: string
  price: number
  cost_price: number
  stock: number
  sort_order: number
  status: number
}

export interface SaleProductHistoryInfo {
  id: number
  product_id: number
  change_type: string
  old_value: string
  new_value: string
  operator_name: string
  remark: string
  created_at: string
}

// ===== 产品管理 - 分类 =====

export interface SaleProductCategoryCreateRequest {
  parent_id?: number
  name: string
  icon?: string
  sort_order?: number
  status?: number
}

export type SaleProductCategoryUpdateRequest = SaleProductCategoryCreateRequest

export interface SaleProductCategoryInfo {
  id: number
  parent_id: number
  name: string
  icon: string
  sort_order: number
  status: number
  children?: SaleProductCategoryInfo[]
}

export interface SaleProductCategoryListResponse {
  items: SaleProductCategoryInfo[]
}

// ===== 订单管理 =====

export interface OrderListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  product_id?: number
  status?: string
  pay_method?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface OrderInfo {
  id: number
  order_no: string
  user_id: number
  product_id: number
  product_name: string
  specs: string
  quantity: number
  price_model: string
  total_amount: number
  paid_amount: number
  status: string
  pay_method: string
  pay_time: string
  expire_time: string
  remark: string
  created_at: string
  updated_at: string
}

export interface OrderListResponse {
  items: OrderInfo[]
  meta: ListMeta
}

export interface OrderItemInfo {
  id: number
  order_id: number
  product_id: number
  product_name: string
  spec_code: string
  specs: string
  price: number
  quantity: number
  amount: number
  created_at: string
}

export interface OrderDetail extends OrderInfo {
  items: OrderItemInfo[]
  refunds: RefundInfo[]
}

export interface OrderRemarkUpdateRequest {
  remark: string
}

export interface RefundCreateRequest {
  amount: number
  reason?: string
}

export interface OrderStatusStat {
  status: string
  count: number
}

export interface OrderTrendPoint {
  date: string
  sales: number
  orders: number
}

export interface OrderStatsResponse {
  today_sales: number
  today_orders: number
  avg_order_value: number
  refund_rate: number
  status_distribution: OrderStatusStat[]
  trend: OrderTrendPoint[]
}

// ===== 退款管理 =====

export interface RefundListQuery {
  keyword?: string
  order_id?: number
  status?: string
  page?: number
  page_size?: number
}

export interface RefundInfo {
  id: number
  refund_no: string
  order_id: number
  order_no: string
  user_id: number
  amount: number
  reason: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  created_at: string
  updated_at: string
}

export interface RefundListResponse {
  items: RefundInfo[]
  meta: ListMeta
}

export interface RefundApproveRequest {
  remark?: string
}

// ===== 财务管理 =====

export interface WalletInfo {
  user_id: number
  balance: number
  frozen: number
  total_income: number
  total_expense: number
  version: number
}

export interface TransactionListQuery {
  user_id?: number
  type?: string
  direction?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface TransactionInfo {
  id: number
  tx_no: string
  user_id: number
  type: string
  direction: number // 1=收入 -1=支出
  amount: number
  balance_before: number
  balance_after: number
  order_id: number
  order_no: string
  ref_no: string
  remark: string
  operator_id: number
  created_at: string
}

export interface TransactionListResponse {
  items: TransactionInfo[]
  meta: ListMeta
}

export interface AdjustRequest {
  user_id: number
  type?: string
  direction: number
  amount: number
  biz_key: string
  remark?: string
}

export interface RechargeCreateRequest {
  user_id: number
  amount: number
  method: string
  remark?: string
}

export interface RechargeApproveRequest {
  channel_tx?: string
  remark?: string
}

export interface RechargeInfo {
  id: number
  recharge_no: string
  user_id: number
  amount: number
  method: string
  status: string
  channel_tx: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface RechargeListQuery {
  user_id?: number
  status?: string
  method?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface RechargeListResponse {
  items: RechargeInfo[]
  meta: ListMeta
}

export interface WithdrawListQuery {
  user_id?: number
  status?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface WithdrawInfo {
  id: number
  withdraw_no: string
  user_id: number
  amount: number
  channel: string
  account: string
  status: string
  audit_by: number
  audit_by_name: string
  audited_at: string
  paid_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface WithdrawListResponse {
  items: WithdrawInfo[]
  meta: ListMeta
}

export interface WithdrawAuditRequest {
  remark?: string
}

export interface BillListQuery {
  user_id?: number
  period?: string
  status?: string
  page?: number
  page_size?: number
}

export interface BillInfo {
  id: number
  bill_no: string
  user_id: number
  period: string
  total_amount: number
  refund_amount: number
  status: string
  created_at: string
  updated_at: string
}

export interface BillListResponse {
  items: BillInfo[]
  meta: ListMeta
}

export interface ReconcileResponse {
  period: string
  income_total: number
  expense_total: number
  tx_count: number
  wallet_balance: number
  diff: number
  status: string
}

// ===== 工单支持（doc50） =====

export interface TicketListQuery {
  keyword?: string
  user_keyword?: string
  user_id?: number
  category?: string
  priority?: string
  status?: string
  assigned_to?: number
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}

export interface TicketInfo {
  id: number
  ticket_no: string
  user_id: number
  username: string
  title: string
  category: string
  category_name: string
  priority: string
  status: string
  assigned_to: number
  assigned_name: string
  order_id: number
  instance_id: number
  reply_count: number
  created_at: string
  updated_at: string
}

export interface TicketListResponse {
  items: TicketInfo[]
  meta: ListMeta
}

export interface TicketReplyInfo {
  id: number
  ticket_id: number
  sender_type: string // user / admin
  sender_id: number
  sender_name: string
  content: string
  created_at: string
}

export interface TicketDetail extends TicketInfo {
  description: string
  first_reply_at: string
  resolved_at: string
  closed_at: string
  replies: TicketReplyInfo[]
}

export interface TicketReplyRequest {
  content: string
}

export interface TicketAssignRequest {
  assigned_to: number
}

export interface TicketStatusRequest {
  status: string
}

export interface TicketCategorySaveRequest {
  name: string
  code: string
  description?: string
  sort_order?: number
  status?: string
}

export interface TicketCategoryInfo {
  id: number
  name: string
  code: string
  description: string
  sort_order: number
  status: string
  created_at: string
  updated_at: string
}

export interface TicketStatusStat {
  status: string
  count: number
}

export interface TicketTrendPoint {
  date: string
  count: number
}

export interface TicketCategoryStat {
  category: string
  count: number
}

export interface TicketStatsResponse {
  total: number
  open: number
  in_progress: number
  waiting_user: number
  resolved: number
  closed: number
  avg_first_reply_seconds: number
  status_distribution: TicketStatusStat[]
  trend: TicketTrendPoint[]
  category_distribution: TicketCategoryStat[]
}
