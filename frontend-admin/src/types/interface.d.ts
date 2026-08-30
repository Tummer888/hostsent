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
