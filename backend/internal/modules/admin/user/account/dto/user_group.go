package dto

import "time"

// UserGroupListQuery 用户组列表查询。
// IsAgentGroup 是字符串三态（沿用与 IsSubAccount 一致的约定）：
// "" 全部，"true" 仅代理组，"false" 仅普通组。
type UserGroupListQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Status       string `form:"status"`
	Keyword      string `form:"keyword"`
	IsAgentGroup string `form:"is_agent_group"`
}

type UserGroupCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
	// 折扣策略绑定（P3-01/D3）：用户组是主折扣来源，绑定 price_policies。
	PricePolicyID *uint64 `json:"price_policy_id"`
	// IsDefault 默认用户组：新用户未指定分组时归入，全库至多一个（置 true 会清掉其他组）。
	IsDefault bool `json:"is_default"`
	// IsAgentGroup 代理组标记：仅用于区分组类型（普通组/代理组），不参与折扣解析。
	IsAgentGroup bool `json:"is_agent_group"`
}

type UserGroupUpdateRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required"`
	SortOrder   int    `json:"sort_order"`
	// 折扣策略绑定（P3-01/D3）：用户组是主折扣来源，绑定 price_policies。
	PricePolicyID *uint64 `json:"price_policy_id"`
	// IsDefault 默认用户组：新用户未指定分组时归入，全库至多一个（置 true 会清掉其他组）。
	IsDefault bool `json:"is_default"`
	// IsAgentGroup 代理组标记：仅用于区分组类型（普通组/代理组），不参与折扣解析。
	IsAgentGroup bool `json:"is_agent_group"`
}

type UserGroupInfo struct {
	ID            uint64    `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	SortOrder     int       `json:"sort_order"`
	PricePolicyID *uint64   `json:"price_policy_id"`
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
