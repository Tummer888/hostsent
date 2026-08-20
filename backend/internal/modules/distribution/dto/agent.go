package dto

import "time"

type AgentListQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Status       string `form:"status"`
	AgentLevelID uint64 `form:"agent_level_id"`
	Keyword      string `form:"keyword"`
}

type AgentCreateRequest struct {
	UserID           uint64     `json:"user_id" binding:"required"`
	AgentLevelID     uint64     `json:"agent_level_id" binding:"required"`
	InviterAgentID   *uint64    `json:"inviter_agent_id"`
	InviteCode       string     `json:"invite_code" binding:"required"`
	Status           string     `json:"status"`
	DirectSubCount   int        `json:"direct_sub_count"`
	TeamSubCount     int        `json:"team_sub_count"`
	TotalCommission  float64    `json:"total_commission"`
	AvailableBalance float64    `json:"available_balance"`
	LastSettledAt    *time.Time `json:"last_settled_at"`
	Remark           string     `json:"remark"`
}

type AgentUpdateRequest struct {
	AgentLevelID     uint64     `json:"agent_level_id" binding:"required"`
	InviterAgentID   *uint64    `json:"inviter_agent_id"`
	InviteCode       string     `json:"invite_code" binding:"required"`
	Status           string     `json:"status" binding:"required"`
	DirectSubCount   int        `json:"direct_sub_count"`
	TeamSubCount     int        `json:"team_sub_count"`
	TotalCommission  float64    `json:"total_commission"`
	AvailableBalance float64    `json:"available_balance"`
	LastSettledAt    *time.Time `json:"last_settled_at"`
	Remark           string     `json:"remark"`
}

type AgentInfo struct {
	ID               uint64     `json:"id"`
	UserID           uint64     `json:"user_id"`
	Username         string     `json:"username"`
	RealName         string     `json:"real_name"`
	Email            string     `json:"email"`
	Phone            string     `json:"phone"`
	AgentLevelID     uint64     `json:"agent_level_id"`
	AgentLevelName   string     `json:"agent_level_name"`
	InviterAgentID   *uint64    `json:"inviter_agent_id"`
	InviterAgentName string     `json:"inviter_agent_name"`
	InviteCode       string     `json:"invite_code"`
	Status           string     `json:"status"`
	DirectSubCount   int        `json:"direct_sub_count"`
	TeamSubCount     int        `json:"team_sub_count"`
	TotalCommission  float64    `json:"total_commission"`
	AvailableBalance float64    `json:"available_balance"`
	LastSettledAt    *time.Time `json:"last_settled_at"`
	Remark           string     `json:"remark"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type AgentListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type AgentListResponse struct {
	Items []AgentInfo   `json:"items"`
	Meta  AgentListMeta `json:"meta"`
}
