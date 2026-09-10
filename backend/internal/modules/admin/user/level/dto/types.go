// Package dto 定义用户等级模块的请求与响应数据结构。
package dto

import "time"

// ListQuery 定义用户等级列表的筛选条件。
type ListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

// CreateRequest 表示创建用户等级时提交的参数。
type CreateRequest struct {
	Name             string  `json:"name" binding:"required"`
	Code             string  `json:"code" binding:"required"`
	Weight           int     `json:"weight"`
	Status           string  `json:"status"`
	FeatureFlags     string  `json:"feature_flags"`
	UpgradeCondition string  `json:"upgrade_condition"`
	UpgradeThreshold float64 `json:"upgrade_threshold"`
	MaxSubAccounts   int     `json:"max_sub_accounts"`
	Benefits         string  `json:"benefits"`
	Description      string  `json:"description"`
}

// UpdateRequest 表示更新用户等级时提交的参数。
type UpdateRequest struct {
	Name             string  `json:"name" binding:"required"`
	Code             string  `json:"code" binding:"required"`
	Weight           int     `json:"weight"`
	Status           string  `json:"status" binding:"required"`
	FeatureFlags     string  `json:"feature_flags"`
	UpgradeCondition string  `json:"upgrade_condition"`
	UpgradeThreshold float64 `json:"upgrade_threshold"`
	MaxSubAccounts   int     `json:"max_sub_accounts"`
	Benefits         string  `json:"benefits"`
	Description      string  `json:"description"`
}

// Info 描述单个用户等级。
type Info struct {
	ID               uint64    `json:"id"`
	Name             string    `json:"name"`
	Code             string    `json:"code"`
	Weight           int       `json:"weight"`
	Status           string    `json:"status"`
	FeatureFlags     string    `json:"feature_flags"`
	UpgradeCondition string    `json:"upgrade_condition"`
	UpgradeThreshold float64   `json:"upgrade_threshold"`
	MaxSubAccounts   int       `json:"max_sub_accounts"`
	Benefits         string    `json:"benefits"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ListMeta 描述分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListResponse 表示用户等级列表返回结果。
type ListResponse struct {
	Items []Info   `json:"items"`
	Meta  ListMeta `json:"meta"`
}

// APIResponse 表示通用接口响应结构（仅用于接口文档标注）。
type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}
