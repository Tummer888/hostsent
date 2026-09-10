import request from '@/utils/request'

/** 代理概览（P6-03）。 */
export interface AgentProfile {
  agent_id: number
  level_name: string
  level_code: string
  invite_code: string
  invite_link: string
  status: string
  direct_sub_count: number
  team_sub_count: number
  total_commission: number
  available_balance: number
}

export interface TeamMemberInfo {
  id: number
  user_id: number
  username: string
  name: string
  email: string
  level_depth: number
  relation_type: string
  contribution_amount: number
  commission_amount: number
  status: string
  joined_at: string
}

export interface CommissionInfo {
  id: number
  order_no: string
  commission_type: string
  source_type: string
  base_amount: number
  rate: number
  amount: number
  status: string
  remark: string
  created_at: string
}

export interface SettlementInfo {
  id: number
  settlement_no: string
  period_start: string
  period_end: string
  commission_total: number
  deduction_total: number
  payable_total: number
  status: string
  paid_at: string
  created_at: string
}

export interface AgentListQuery {
  page?: number
  page_size?: number
  status?: string
  /** 下级层级筛选（P7，0/空为全部）。 */
  level_depth?: number
}

/** 代理看板汇总（P7）。 */
export interface AgentStats {
  direct_sub_count: number
  team_sub_count: number
  pending_commission: number
  settled_commission: number
  paid_settlement: number
  pending_settlement: number
}

interface ListResponse<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export function getAgentProfile() {
  return request.get<any, { data: AgentProfile }>('/uc/agent/profile')
}

export function getAgentStats() {
  return request.get<any, { data: AgentStats }>('/uc/agent/stats')
}

export function getAgentTeam(params: AgentListQuery = {}) {
  return request.get<any, { data: ListResponse<TeamMemberInfo> }>('/uc/agent/team', { params })
}

export function getAgentCommissions(params: AgentListQuery = {}) {
  return request.get<any, { data: ListResponse<CommissionInfo> }>('/uc/agent/commissions', { params })
}

export function getAgentSettlements(params: AgentListQuery = {}) {
  return request.get<any, { data: ListResponse<SettlementInfo> }>('/uc/agent/settlements', { params })
}
