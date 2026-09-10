package dto

import "time"

type UserGroupListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

type UserGroupCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
	// 折扣策略绑定（P3-01/D3）：用户组是主折扣来源，策略表在 P5 落地，此处先占位。
	PricePolicyID *uint64 `json:"price_policy_id"`
	Priority      int     `json:"priority"`
	IsDefault     bool    `json:"is_default"`
	IsAgentGroup  bool    `json:"is_agent_group"`
}

type UserGroupUpdateRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required"`
	SortOrder   int    `json:"sort_order"`
	// 折扣策略绑定（P3-01/D3）：用户组是主折扣来源，策略表在 P5 落地，此处先占位。
	PricePolicyID *uint64 `json:"price_policy_id"`
	Priority      int     `json:"priority"`
	IsDefault     bool    `json:"is_default"`
	IsAgentGroup  bool    `json:"is_agent_group"`
}

type UserGroupInfo struct {
	ID            uint64    `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	SortOrder     int       `json:"sort_order"`
	PricePolicyID *uint64   `json:"price_policy_id"`
	Priority      int       `json:"priority"`
	IsDefault     bool      `json:"is_default"`
	IsAgentGroup  bool      `json:"is_agent_group"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UserGroupListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type UserGroupListResponse struct {
	Items []UserGroupInfo   `json:"items"`
	Meta  UserGroupListMeta `json:"meta"`
}
