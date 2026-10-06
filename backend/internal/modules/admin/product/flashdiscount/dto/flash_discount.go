// Package dto 提供限时活动折扣的传输结构（doc108 §8J）。
package dto

// ListMeta 通用分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Query 活动折扣列表查询。
type Query struct {
	Keyword string `form:"keyword" json:"keyword"`
	// Status 运营启停意图：active / disabled；空 = 全部。
	Status string `form:"status" json:"status"`
	// Running 只看「此刻正在进行」的活动（按时间窗实时判断）；空字符串 = 不筛。
	// 界面上的「进行中」筛选走这个，而不是 status —— status 只表达启停意图。
	Running  string `form:"running" json:"running"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ItemRequest 定向条目：一个分类或一个商品。
type ItemRequest struct {
	TargetType string `json:"target_type" binding:"omitempty,oneof=category product"`
	TargetID   uint64 `json:"target_id"`
}

// Request 创建/更新活动折扣。
//
// StartAt / EndAt 用字符串传（"2026-11-01 00:00:00"），空串表示不限；
// 与仓库既有 DTO 的时间口径一致（见 promotion/discount 子域的 parseTime）。
type Request struct {
	Name          string  `json:"name" binding:"required,min=1,max=64"`
	Code          string  `json:"code" binding:"required,min=1,max=64"`
	Description   string  `json:"description"`
	DiscountType  string  `json:"discount_type" binding:"omitempty,oneof=rate amount"`
	DiscountValue float64 `json:"discount_value"`
	Scope         string  `json:"scope" binding:"omitempty,oneof=all category product"`
	StartAt       string  `json:"start_at"`
	EndAt         string  `json:"end_at"`
	Status        string  `json:"status"`
	Remark        string  `json:"remark"`
	// Items 为 nil 表示不改条目；非 nil（含空数组）整体覆盖。
	Items []ItemRequest `json:"items"`
}

// ItemInfo 定向条目明细（带目标名，前端免二次查询）。
type ItemInfo struct {
	TargetType string `json:"target_type"`
	TargetID   uint64 `json:"target_id"`
	TargetName string `json:"target_name"`
}

// Info 活动折扣信息。
type Info struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	Description   string  `json:"description"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue float64 `json:"discount_value"`
	Scope         string  `json:"scope"`
	StartAt       string  `json:"start_at"`
	EndAt         string  `json:"end_at"`
	// Status 运营启停意图。
	Status string `json:"status"`
	// RuntimeStatus 按当前时间推导的运行态：pending / running / expired / disabled。
	RuntimeStatus string     `json:"runtime_status"`
	Remark        string     `json:"remark"`
	ItemCount     int64      `json:"item_count"`
	Items         []ItemInfo `json:"items"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
}

// ListResponse 活动折扣列表响应。
type ListResponse struct {
	Items []Info   `json:"items"`
	Meta  ListMeta `json:"meta"`
}
