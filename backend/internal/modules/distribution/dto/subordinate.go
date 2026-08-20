package dto

import "time"

type SubordinateListQuery struct {
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
	AgentID    uint64 `form:"agent_id"`
	Status     string `form:"status"`
	LevelDepth int    `form:"level_depth"`
	Keyword    string `form:"keyword"`
}

type SubordinateCreateRequest struct {
	AgentID            uint64     `json:"agent_id" binding:"required"`
	UserID             uint64     `json:"user_id" binding:"required"`
	ParentAgentID      *uint64    `json:"parent_agent_id"`
	LevelDepth         int        `json:"level_depth"`
	RelationType       string     `json:"relation_type"`
	ContributionAmount float64    `json:"contribution_amount"`
	CommissionAmount   float64    `json:"commission_amount"`
	Status             string     `json:"status"`
	JoinedAt           *time.Time `json:"joined_at"`
}

type SubordinateUpdateRequest struct {
	ParentAgentID      *uint64    `json:"parent_agent_id"`
	LevelDepth         int        `json:"level_depth"`
	RelationType       string     `json:"relation_type" binding:"required"`
	ContributionAmount float64    `json:"contribution_amount"`
	CommissionAmount   float64    `json:"commission_amount"`
	Status             string     `json:"status" binding:"required"`
	JoinedAt           *time.Time `json:"joined_at"`
}

type SubordinateInfo struct {
	ID                 uint64     `json:"id"`
	AgentID            uint64     `json:"agent_id"`
	AgentName          string     `json:"agent_name"`
	UserID             uint64     `json:"user_id"`
	Username           string     `json:"username"`
	RealName           string     `json:"real_name"`
	Phone              string     `json:"phone"`
	ParentAgentID      *uint64    `json:"parent_agent_id"`
	ParentAgentName    string     `json:"parent_agent_name"`
	LevelDepth         int        `json:"level_depth"`
	RelationType       string     `json:"relation_type"`
	ContributionAmount float64    `json:"contribution_amount"`
	CommissionAmount   float64    `json:"commission_amount"`
	Status             string     `json:"status"`
	JoinedAt           *time.Time `json:"joined_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type SubordinateListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type SubordinateListResponse struct {
	Items []SubordinateInfo   `json:"items"`
	Meta  SubordinateListMeta `json:"meta"`
}
