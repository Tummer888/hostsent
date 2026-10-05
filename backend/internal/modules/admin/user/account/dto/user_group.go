package dto

import "time"

// UserGroupListQuery 用户组列表查询。
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
	// IsDefault 默认用户组：新用户未指定分组时归入，全库至多一个（置 true 会清掉其他组）。
	IsDefault bool `json:"is_default"`
}

type UserGroupUpdateRequest struct {
	Name        string `json:"name" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required"`
	SortOrder   int    `json:"sort_order"`
	// IsDefault 默认用户组：新用户未指定分组时归入，全库至多一个（置 true 会清掉其他组）。
	IsDefault bool `json:"is_default"`
}

// UserGroupInfo 用户组信息。
//
// 不含任何折扣字段：用户组只做分类（doc108），折扣由代理等级承载。
type UserGroupInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	// MemberCount 组内成员数（未注销客户），列表页展示与下钻用。
	MemberCount int64     `json:"member_count"`
	Status      string    `json:"status"`
	SortOrder   int       `json:"sort_order"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
