// Package dto 提供促销管理（promotion）子域的数据传输结构。
package dto

// ListMeta 通用分页元信息
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// CouponQuery 优惠券列表查询
type CouponQuery struct {
	Keyword  string `form:"keyword" json:"keyword"`
	CouponType string `form:"coupon_type" json:"coupon_type"`
	Status   int    `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// CouponRequest 创建/更新优惠券
type CouponRequest struct {
	CouponCode   string  `json:"coupon_code"`
	Name         string  `json:"name" binding:"required"`
	CouponType   string  `json:"coupon_type" binding:"required"`
	Amount       float64 `json:"amount"`
	MinAmount    float64 `json:"min_amount"`
	Discount     float64 `json:"discount"`
	TotalStock   int     `json:"total_stock"`
	PerUserLimit int     `json:"per_user_limit"`
	Scope        string  `json:"scope"`
	ValidFrom    string  `json:"valid_from"`
	ValidTo      string  `json:"valid_to"`
	Status       int     `json:"status"`
}

// CouponInfo 优惠券信息
type CouponInfo struct {
	ID           uint64  `json:"id"`
	CouponCode   string  `json:"coupon_code"`
	Name         string  `json:"name"`
	CouponType   string  `json:"coupon_type"`
	Amount       float64 `json:"amount"`
	MinAmount    float64 `json:"min_amount"`
	Discount     float64 `json:"discount"`
	TotalStock   int     `json:"total_stock"`
	ClaimedCount int     `json:"claimed_count"`
	PerUserLimit int     `json:"per_user_limit"`
	Scope        string  `json:"scope"`
	ValidFrom    string  `json:"valid_from"`
	ValidTo      string  `json:"valid_to"`
	Status       int     `json:"status"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// CouponListResponse 优惠券列表响应
type CouponListResponse struct {
	Items []CouponInfo `json:"items"`
	Meta  ListMeta     `json:"meta"`
}

// CouponGrantQuery 发放记录查询
type CouponGrantQuery struct {
	CouponID uint64 `form:"coupon_id" json:"coupon_id"`
	Status   string `form:"status" json:"status"`
	Keyword  string `form:"keyword" json:"keyword"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// CouponGrantCreateRequest 发放优惠券（单个/批量）
type CouponGrantCreateRequest struct {
	CouponID uint64   `json:"coupon_id" binding:"required"`
	UserIDs  []uint64 `json:"user_ids" binding:"required"`
}

// CouponGrantInfo 发放记录信息
type CouponGrantInfo struct {
	ID        uint64 `json:"id"`
	CouponID  uint64 `json:"coupon_id"`
	UserID    uint64 `json:"user_id"`
	UserName  string `json:"user_name"`
	Status    string `json:"status"`
	ValidFrom string `json:"valid_from"`
	ValidTo   string `json:"valid_to"`
	UsedAt    string `json:"used_at"`
	CreatedAt string `json:"created_at"`
}

// CouponGrantListResponse 发放记录列表响应
type CouponGrantListResponse struct {
	Items []CouponGrantInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// PromotionQuery 促销活动列表查询
type PromotionQuery struct {
	Keyword       string `form:"keyword" json:"keyword"`
	PromotionType string `form:"promotion_type" json:"promotion_type"`
	Status        int    `form:"status" json:"status"`
	Page          int    `form:"page" json:"page"`
	PageSize      int    `form:"page_size" json:"page_size"`
}

// PromotionRequest 创建/更新促销活动
type PromotionRequest struct {
	Name          string `json:"name" binding:"required"`
	PromotionType string `json:"promotion_type" binding:"required"`
	Rule          string `json:"rule"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	SortOrder     int    `json:"sort_order"`
	Status        int    `json:"status"`
}

// PromotionInfo 促销活动信息
type PromotionInfo struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	PromotionType string `json:"promotion_type"`
	Rule          string `json:"rule"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	SortOrder     int    `json:"sort_order"`
	Status        int    `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// PromotionListResponse 促销活动列表响应
type PromotionListResponse struct {
	Items []PromotionInfo `json:"items"`
	Meta  ListMeta        `json:"meta"`
}
