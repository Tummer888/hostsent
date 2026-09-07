// Package dto 提供生命周期模块的接口出入参。
package dto

// ListMeta 通用分页元信息（对齐订单模块 dto.ListMeta）
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ExpiringListQuery 到期实例分页查询（管理端）
type ExpiringListQuery struct {
	Stage    string `form:"stage" json:"stage"`       // expiring/grace/suspended/destroyed/active，空=全部
	Keyword  string `form:"keyword" json:"keyword"`   // 实例标识 / 实例名 / 用户名
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// ExpiringInstanceItem 到期实例列表项
type ExpiringInstanceItem struct {
	ID           uint64  `json:"id"`
	InstanceMark string  `json:"instance_mark"`
	Name         string  `json:"name"`
	UserID       uint64  `json:"user_id"`
	Username     string  `json:"username"`
	ProductID    uint64  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	UnitPrice    float64 `json:"unit_price"`        // 单周期续费价（元）
	BillingMode  string  `json:"billing_mode"`
	Status       string  `json:"status"` // 上游同步状态（running/stopped...）
	ExpireAt     string  `json:"expire_at"`
	DaysLeft     int     `json:"days_left"` // 剩余天数（负数=已过期天数）
	Stage        string  `json:"stage"`     // 派生生命周期阶段
	AutoRenew    bool    `json:"auto_renew"`
}

// ExpiringListResponse 到期实例列表响应
type ExpiringListResponse struct {
	Items []ExpiringInstanceItem `json:"items"`
	Meta  ListMeta               `json:"meta"`
}

// RenewalListQuery 续费记录分页查询
type RenewalListQuery struct {
	Keyword   string `form:"keyword" json:"keyword"` // 续费单号 / 订单号 / 实例标识
	UserID    uint64 `form:"user_id" json:"user_id"`
	Status    string `form:"status" json:"status"`
	Source    string `form:"source" json:"source"`
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// RenewalInfo 续费记录
type RenewalInfo struct {
	ID           uint64  `json:"id"`
	RenewalNo    string  `json:"renewal_no"`
	InstanceID   uint64  `json:"instance_id"`
	InstanceMark string  `json:"instance_mark"`
	UserID       uint64  `json:"user_id"`
	Username     string  `json:"username"`
	ProductID    uint64  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	BillingMode  string  `json:"billing_mode"`
	PeriodCount  int     `json:"period_count"`
	Amount       float64 `json:"amount"`
	Source       string  `json:"source"`
	Status       string  `json:"status"`
	OrderID      uint64  `json:"order_id"`
	OrderNo      string  `json:"order_no"`
	Remark       string  `json:"remark"`
	PayTime      string  `json:"pay_time"`
	ExpireBefore string  `json:"expire_before"`
	ExpireAfter  string  `json:"expire_after"`
	FailReason   string  `json:"fail_reason"`
	CreatedAt    string  `json:"created_at"`
}

// RenewalListResponse 续费记录列表响应
type RenewalListResponse struct {
	Items []RenewalInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// AdminRenewRequest 管理员代续费请求
type AdminRenewRequest struct {
	PeriodCount int     `json:"period_count"`             // 续费周期数，默认 1
	Amount      float64 `json:"amount"`                   // 自定义金额（元），0=按产品价计算
	Remark      string  `json:"remark"`                   // 备注
}

// UserRenewRequest 用户发起续费请求
type UserRenewRequest struct {
	PeriodCount int `json:"period_count"` // 续费周期数，默认 1
}

// AutoRenewToggleRequest 自动续费开关请求
type AutoRenewToggleRequest struct {
	Enabled     bool `json:"enabled"`
	PeriodCount int  `json:"period_count"` // 自动续费周期数，默认 1
}

// PolicyResponse 生命周期策略
type PolicyResponse struct {
	RemindDays       string `json:"remind_days"`
	AutoRenewDefault bool   `json:"auto_renew_default"`
	GraceDays        int    `json:"grace_days"`
	DestroyKeepDays  int    `json:"destroy_keep_days"`
	UpdatedAt        string `json:"updated_at"`
}

// PolicyUpdateRequest 更新生命周期策略请求
type PolicyUpdateRequest struct {
	RemindDays       string `json:"remind_days"`
	AutoRenewDefault *bool  `json:"auto_renew_default"`
	GraceDays        *int   `json:"grace_days"`
	DestroyKeepDays  *int   `json:"destroy_keep_days"`
}

// UserInstanceRenewalItem 用户续费管理聚合视图项
type UserInstanceRenewalItem struct {
	ID           uint64  `json:"id"`
	InstanceMark string  `json:"instance_mark"`
	Name         string  `json:"name"`
	ProductID    uint64  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	UnitPrice    float64 `json:"unit_price"` // 单周期续费价（元）
	BillingMode  string  `json:"billing_mode"`
	Status       string  `json:"status"`
	ExpireAt     string  `json:"expire_at"`
	DaysLeft     int     `json:"days_left"`
	Stage        string  `json:"stage"`
	AutoRenew    bool    `json:"auto_renew"`
	AutoPeriod   int     `json:"auto_period"` // 自动续费周期数
}

// UserRenewalsViewResponse 用户续费管理聚合视图
type UserRenewalsViewResponse struct {
	Items   []UserInstanceRenewalItem `json:"items"`
	Policy  PolicyResponse            `json:"policy"` // 策略摘要（宽限期等展示用）
}

// RenewalCreatedResponse 续费订单创建结果（待支付）
type RenewalCreatedResponse struct {
	Renewal RenewalInfo `json:"renewal"`
}

// UserRenewalListQuery 我的续费记录查询
type UserRenewalListQuery struct {
	Status string `form:"status" json:"status"`
	Page   int    `form:"page" json:"page"`
	PageSize int  `form:"page_size" json:"page_size"`
}
