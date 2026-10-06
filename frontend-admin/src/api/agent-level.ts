// 代理等级与折扣矩阵 API（doc108）。
//
// 代理等级是「拿货折扣」的载体，也是当前算价的**唯一**折扣来源；
// 用户组已不再参与算价（只做客户分类），因此折扣相关的读写都在这里，
// 不在 user.ts 的用户组接口里。
import { request } from '@/utils/request'

/** 折扣目标类型：全站兜底 / 某分类 / 某商品（命中优先级 product > category > all）。 */
export type AgentDiscountTargetType = 'all' | 'category' | 'product'

export interface AgentLevelListQuery {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

/** 折扣矩阵的一格（等级 × 目标）。 */
export interface AgentDiscountItem {
  target_type: AgentDiscountTargetType
  target_id: number
  /** 目标名（分类名/商品名；all 固定为"全站兜底"）。 */
  target_name?: string
  /** 折扣率，0.85 = 八五折（越小越优惠）；0 = 未配置（不打折）。 */
  discount_rate: number
}

export interface AgentLevelInfo {
  id: number
  name: string
  code: string
  /** 权重，越大越优先（折扣最优）。矩阵列按此降序。 */
  weight: number
  status: string
  description?: string
  /** 该等级下的代理用户数。 */
  member_count: number
  /** 已配置的折扣格数。 */
  discount_count: number
  discounts?: AgentDiscountItem[]
  created_at: string
  updated_at: string
}

export interface AgentLevelRequest {
  name: string
  code: string
  weight: number
  status: string
  description?: string
  /** nil（不传）表示不修改矩阵；传（含空数组）= 整体覆盖。 */
  discounts?: AgentDiscountItem[]
}

export interface AgentLevelListResponse {
  items: AgentLevelInfo[]
  meta: { page: number; page_size: number; total: number }
}

/** 矩阵的一列：一个代理等级。 */
export interface AgentMatrixColumn {
  agent_level_id: number
  name: string
  code: string
  weight: number
  status: string
}

/** 矩阵的一格。 */
export interface AgentMatrixCell {
  agent_level_id: number
  discount_rate: number
  configured: boolean
}

/** 矩阵的一行：全站兜底或某个分类。 */
export interface AgentMatrixRow {
  target_type: AgentDiscountTargetType
  target_id: number
  target_name: string
  /** 分类成本率（0 = 未配置，此时跳过毛利校验）；全站行固定 0。 */
  cost_rate: number
  cells: AgentMatrixCell[]
}

export interface AgentMatrixResponse {
  columns: AgentMatrixColumn[]
  rows: AgentMatrixRow[]
  /** 商品例外行：只列配置过商品级折扣的商品（商品级阶梯的直接证据）。 */
  product_rows: AgentMatrixRow[]
}

export interface AgentLadderPreviewRequest {
  /** 最优等级（权重最高）的折扣率，如 0.7 = 七折。 */
  anchor_rate: number
  /** 每往下一级增加的折扣率（让利变少）。 */
  step: number
  /** 阶梯作用的目标：all / category / product（商品级阶梯）。空 = all。 */
  target_type?: AgentDiscountTargetType
  target_id?: number
  /** 可选成本率覆盖；一般不传，服务端按目标解析（预览与落库同口径）。 */
  cost_rate?: number
}

export interface AgentLadderPreviewCell {
  agent_level_id: number
  name: string
  weight: number
  discount_rate: number
  /** 毛利率 = 折扣率 − 成本率；成本率未配置时为 0。 */
  gross_margin: number
  feasible: boolean
}

export interface AgentLadderPreviewResponse {
  cells: AgentLadderPreviewCell[]
  warnings: string[]
}

export interface AgentLadderApplyRequest {
  target_type: AgentDiscountTargetType
  target_id: number
  anchor_rate: number
  step: number
}

export function getAgentLevelList(params: AgentLevelListQuery): Promise<AgentLevelListResponse> {
  return request.get<AgentLevelListResponse>({
    url: '/agent-levels',
    params: {
      page: params.page,
      page_size: params.page_size,
      status: params.status,
      keyword: params.keyword,
    },
  })
}

export function getAgentLevelDetail(id: string | number): Promise<AgentLevelInfo> {
  return request.get<AgentLevelInfo>({
    url: `/agent-levels/${id}`,
  })
}

export function createAgentLevel(data: AgentLevelRequest): Promise<AgentLevelInfo> {
  return request.post<AgentLevelInfo>({
    url: '/agent-levels',
    data,
  })
}

export function updateAgentLevel(id: string | number, data: AgentLevelRequest): Promise<AgentLevelInfo> {
  return request.put<AgentLevelInfo>({
    url: `/agent-levels/${id}`,
    data,
  })
}

export function deleteAgentLevel(id: string | number): Promise<string> {
  return request.delete<string>({
    url: `/agent-levels/${id}`,
  })
}

/** 单格折扣更新参数：rate=0 表示清除该格（不打折）。 */
export interface AgentCellUpdateRequest {
  agent_level_id: number
  target_type: AgentDiscountTargetType
  target_id: number
  discount_rate: number
}

/** 矩阵上直接改一格：保存或清除。返回更新后的整张矩阵。 */
export function updateAgentCell(data: AgentCellUpdateRequest): Promise<AgentMatrixResponse> {
  return request.put<AgentMatrixResponse>({
    url: '/agent-levels/discount',
    data,
  })
}

export function getAgentMatrix(): Promise<AgentMatrixResponse> {
  return request.get<AgentMatrixResponse>({
    url: '/agent-levels/matrix',
  })
}

export function previewAgentLadder(data: AgentLadderPreviewRequest): Promise<AgentLadderPreviewResponse> {
  return request.post<AgentLadderPreviewResponse>({
    url: '/agent-levels/ladder/preview',
    data,
  })
}

export function applyAgentLadder(data: AgentLadderApplyRequest): Promise<AgentMatrixResponse> {
  return request.post<AgentMatrixResponse>({
    url: '/agent-levels/ladder/apply',
    data,
  })
}

// —— 商品分组 × 折扣组（doc108 §8I）——

export type ProductGroupTargetType = 'category' | 'product'

export interface ProductGroupItem {
  target_type: ProductGroupTargetType
  target_id: number
  target_name?: string
}

export interface ProductGroupInfo {
  id: number
  name: string
  code: string
  description?: string
  status: string
  item_count: number
  items: ProductGroupItem[]
  created_at: string
  updated_at: string
}

export interface ProductGroupRequest {
  name: string
  code: string
  description?: string
  status?: string
  /** nil（不传）= 不改成员；传（含空数组）= 整体覆盖。 */
  items?: Array<{ target_type: ProductGroupTargetType; target_id: number }>
}

export interface SchemeItem {
  agent_level_id: number
  agent_level_name?: string
  agent_level_weight?: number
  discount_rate: number
}

export interface SchemeInfo {
  id: number
  name: string
  code: string
  description?: string
  status: string
  product_group_id: number
  product_group_name?: string
  item_count: number
  items: SchemeItem[]
  created_at: string
  updated_at: string
}

export interface SchemeRequest {
  name: string
  code: string
  description?: string
  status?: string
  product_group_id?: number
  /** nil（不传）= 不改费率；传（含空数组）= 整体覆盖。 */
  items?: Array<{ agent_level_id: number; discount_rate: number }>
}

export function getProductGroupList(): Promise<{ items: ProductGroupInfo[] }> {
  return request.get({ url: '/agent-levels/product-groups' })
}

export function createProductGroup(data: ProductGroupRequest): Promise<ProductGroupInfo> {
  return request.post<ProductGroupInfo>({ url: '/agent-levels/product-groups', data })
}

export function updateProductGroup(id: number, data: ProductGroupRequest): Promise<ProductGroupInfo> {
  return request.put<ProductGroupInfo>({ url: `/agent-levels/product-groups/${id}`, data })
}

export function deleteProductGroup(id: number): Promise<string> {
  return request.delete<string>({ url: `/agent-levels/product-groups/${id}` })
}

export function getSchemeList(): Promise<{ items: SchemeInfo[] }> {
  return request.get({ url: '/agent-levels/discount-schemes' })
}

export function createScheme(data: SchemeRequest): Promise<SchemeInfo> {
  return request.post<SchemeInfo>({ url: '/agent-levels/discount-schemes', data })
}

export function updateScheme(id: number, data: SchemeRequest): Promise<SchemeInfo> {
  return request.put<SchemeInfo>({ url: `/agent-levels/discount-schemes/${id}`, data })
}

export function deleteScheme(id: number): Promise<string> {
  return request.delete<string>({ url: `/agent-levels/discount-schemes/${id}` })
}

/** 应用折扣组：展开写入绑定商品分组下的全部目标，返回生效矩阵。 */
export function applyScheme(id: number): Promise<AgentMatrixResponse> {
  return request.post<AgentMatrixResponse>({ url: `/agent-levels/discount-schemes/${id}/apply` })
}

// —— 代理分组成员（归属管理）：代理分组 = 代理等级，归属 = users.agent_level_id ——

export interface AgentMemberInfo {
  id: number
  username: string
  real_name?: string
  email?: string
  phone?: string
  status: string
  /** null = 尚未归属任何代理分组。 */
  agent_level_id?: number | null
  agent_level_name?: string
  created_at: string
}

export interface AgentMemberListQuery {
  page?: number
  page_size?: number
  keyword?: string
  /** true = 列「尚未归属任何分组」的账号（跨分组候选池，忽略 ID）。 */
  unassigned?: boolean
}

export interface AgentMemberListResponse {
  items: AgentMemberInfo[]
  meta: { page: number; page_size: number; total: number }
  /** 未归属任何代理分组的账号总数。 */
  unassigned_total: number
}

export interface AgentMemberAssignResponse {
  changed: number
  skipped: Array<{ user_id: number; reason: string }>
}

export function getAgentGroupMembers(
  id: number,
  params: AgentMemberListQuery = {},
): Promise<AgentMemberListResponse> {
  return request.get<AgentMemberListResponse>({
    url: `/agent-levels/${id}/members`,
    params: {
      page: params.page,
      page_size: params.page_size,
      keyword: params.keyword,
      unassigned: params.unassigned ? 'true' : undefined,
    },
  })
}

/** 批量纳入 / 移出本代理分组；action=remove 表示取消这些账号的代理身份。 */
export function assignAgentGroupMembers(
  id: number,
  userIDs: number[],
  action: 'assign' | 'remove',
): Promise<AgentMemberAssignResponse> {
  return request.post<AgentMemberAssignResponse>({
    url: `/agent-levels/${id}/members`,
    data: { user_ids: userIDs, action },
  })
}
