// Package dto 定义商品分组与折扣组的数据传输结构（doc108 §8I）。
package dto

// ProductGroupItemRequest 商品分组成员：一个分类或一个商品。
type ProductGroupItemRequest struct {
	TargetType string `json:"target_type" binding:"required,oneof=category product"`
	TargetID   uint64 `json:"target_id"`
}

// ProductGroupRequest 商品分组创建/更新。
type ProductGroupRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=64"`
	Code        string `json:"code" binding:"required,min=1,max=64"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// Items 为 nil 表示不改成员；非 nil（含空数组）整体覆盖。
	Items []ProductGroupItemRequest `json:"items"`
}

// ProductGroupTargetInfo 成员明细（带名称，前端免二次查询）。
type ProductGroupTargetInfo struct {
	TargetType string `json:"target_type"`
	TargetID   uint64 `json:"target_id"`
	TargetName string `json:"target_name"`
}

// ProductGroupInfo 商品分组信息。
type ProductGroupInfo struct {
	ID          uint64                   `json:"id"`
	Name        string                   `json:"name"`
	Code        string                   `json:"code"`
	Description string                   `json:"description"`
	Status      string                   `json:"status"`
	ItemCount   int64                    `json:"item_count"`
	Items       []ProductGroupTargetInfo `json:"items"`
	CreatedAt   string                   `json:"created_at"`
	UpdatedAt   string                   `json:"updated_at"`
}

// ProductGroupListResponse 商品分组列表。
type ProductGroupListResponse struct {
	Items []ProductGroupInfo `json:"items"`
}

// SchemeItemRequest 折扣组的等级费率。
type SchemeItemRequest struct {
	AgentLevelID uint64  `json:"agent_level_id" binding:"required"`
	DiscountRate float64 `json:"discount_rate"`
}

// SchemeRequest 折扣组创建/更新。
type SchemeRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=64"`
	Code        string `json:"code" binding:"required,min=1,max=64"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// ProductGroupIDs 绑定的商品分组（可多个，共用同一套费率阶梯）。
	// 为 nil 表示不改绑定；非 nil（含空数组）整体覆盖。
	// 一个分组至多被一个折扣组绑定（服务层校验）。
	ProductGroupIDs []uint64 `json:"product_group_ids"`
	// Items 为 nil 表示不改费率；非 nil（含空数组）整体覆盖。
	Items []SchemeItemRequest `json:"items"`
}

// SchemeGroupInfo 折扣组绑定的商品分组（带名称与成员数，前端免二次查询）。
type SchemeGroupInfo struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	ItemCount int64  `json:"item_count"`
}

// SchemeItemInfo 折扣组的等级费率明细。
type SchemeItemInfo struct {
	AgentLevelID     uint64  `json:"agent_level_id"`
	AgentLevelName   string  `json:"agent_level_name"`
	AgentLevelWeight int     `json:"agent_level_weight"`
	DiscountRate     float64 `json:"discount_rate"`
}

// SchemeInfo 折扣组信息。
type SchemeInfo struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// ProductGroupIDs / ProductGroups 绑定的商品分组（可多个）。
	ProductGroupIDs []uint64          `json:"product_group_ids"`
	ProductGroups   []SchemeGroupInfo `json:"product_groups"`
	// TargetCount 绑定分组展开后的去重目标数（应用时会写入的目标个数）。
	TargetCount int64            `json:"target_count"`
	ItemCount   int64            `json:"item_count"`
	Items       []SchemeItemInfo `json:"items"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
}

// SchemeListResponse 折扣组列表。
type SchemeListResponse struct {
	Items []SchemeInfo `json:"items"`
}
