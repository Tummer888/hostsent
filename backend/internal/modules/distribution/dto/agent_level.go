package dto

import "time"

type AgentLevelListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

type AgentLevelCreateRequest struct {
	Name                   string  `json:"name" binding:"required"`
	Code                   string  `json:"code" binding:"required"`
	Weight                 int     `json:"weight"`
	DirectCommissionRate   float64 `json:"direct_commission_rate"`
	IndirectCommissionRate float64 `json:"indirect_commission_rate"`
	RenewalCommissionRate  float64 `json:"renewal_commission_rate"`
	UpgradeRewardAmount    float64 `json:"upgrade_reward_amount"`
	SelfPurchaseRebateRate float64 `json:"self_purchase_rebate_rate"`
	AllowManualPrice       bool    `json:"allow_manual_price"`
	AllowSubAgent          bool    `json:"allow_sub_agent"`
	MaxSubAgentDepth       int     `json:"max_sub_agent_depth"`
	Status                 string  `json:"status"`
	Description            string  `json:"description"`
}

type AgentLevelUpdateRequest struct {
	Name                   string  `json:"name" binding:"required"`
	Code                   string  `json:"code" binding:"required"`
	Weight                 int     `json:"weight"`
	DirectCommissionRate   float64 `json:"direct_commission_rate"`
	IndirectCommissionRate float64 `json:"indirect_commission_rate"`
	RenewalCommissionRate  float64 `json:"renewal_commission_rate"`
	UpgradeRewardAmount    float64 `json:"upgrade_reward_amount"`
	SelfPurchaseRebateRate float64 `json:"self_purchase_rebate_rate"`
	AllowManualPrice       bool    `json:"allow_manual_price"`
	AllowSubAgent          bool    `json:"allow_sub_agent"`
	MaxSubAgentDepth       int     `json:"max_sub_agent_depth"`
	Status                 string  `json:"status" binding:"required"`
	Description            string  `json:"description"`
}

type AgentLevelInfo struct {
	ID                     uint64    `json:"id"`
	Name                   string    `json:"name"`
	Code                   string    `json:"code"`
	Weight                 int       `json:"weight"`
	DirectCommissionRate   float64   `json:"direct_commission_rate"`
	IndirectCommissionRate float64   `json:"indirect_commission_rate"`
	RenewalCommissionRate  float64   `json:"renewal_commission_rate"`
	UpgradeRewardAmount    float64   `json:"upgrade_reward_amount"`
	SelfPurchaseRebateRate float64   `json:"self_purchase_rebate_rate"`
	AllowManualPrice       bool      `json:"allow_manual_price"`
	AllowSubAgent          bool      `json:"allow_sub_agent"`
	MaxSubAgentDepth       int       `json:"max_sub_agent_depth"`
	Status                 string    `json:"status"`
	Description            string    `json:"description"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type AgentLevelListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type AgentLevelListResponse struct {
	Items []AgentLevelInfo   `json:"items"`
	Meta  AgentLevelListMeta `json:"meta"`
}
