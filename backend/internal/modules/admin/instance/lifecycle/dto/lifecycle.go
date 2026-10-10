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
	Stage    string `form:"stage" json:"stage"`     // expiring/grace/suspended/destroyed/active，空=全部
	Keyword  string `form:"keyword" json:"keyword"` // 实例标识 / 实例名 / 用户名
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
	UnitPrice    float64 `json:"unit_price"` // 单周期续费价（元）
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
	PeriodCount int     `json:"period_count"` // 续费周期数，默认 1
	Amount      float64 `json:"amount"`       // 自定义金额（元），0=按产品价计算
	Remark      string  `json:"remark"`       // 备注
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

// EnforceRequest 手动单实例到期处置请求。
type EnforceRequest struct {
	Reason string `json:"reason"`
}

// PolicyResponse 生命周期策略
type PolicyResponse struct {
	RemindDays       string `json:"remind_days"`
	AutoRenewDefault bool   `json:"auto_renew_default"`
	GraceDays        int    `json:"grace_days"`
	DestroyKeepDays  int    `json:"destroy_keep_days"`
	// AutoEnforce 到期阶段自动执行总开关（默认 false：只派生阶段、不动上游）。
	AutoEnforce bool `json:"auto_enforce"`
	// EnforceDryRun 预演开关（默认 true）：即使总开关开启，也只计算将要执行的动作而不落上游。
	EnforceDryRun bool `json:"enforce_dry_run"`
	// RefundAction 退款审核通过后对关联实例的动作：none（默认）/ suspend / destroy。
	// 仅对**全额退款**生效（部分退款属补偿，不动机器）。
	RefundAction string `json:"refund_action"`
	UpdatedAt    string `json:"updated_at"`
}

// PolicyUpdateRequest 更新生命周期策略请求
type PolicyUpdateRequest struct {
	RemindDays       string `json:"remind_days"`
	AutoRenewDefault *bool  `json:"auto_renew_default"`
	GraceDays        *int   `json:"grace_days"`
	DestroyKeepDays  *int   `json:"destroy_keep_days"`
	AutoEnforce      *bool  `json:"auto_enforce"`
	EnforceDryRun    *bool  `json:"enforce_dry_run"`
	RefundAction     string `json:"refund_action"`
}

// EnforcementPreviewItem 到期处置预演单条结果（dry-run 报告）。
type EnforcementPreviewItem struct {
	InstanceID   uint64 `json:"instance_id"`
	InstanceMark string `json:"instance_mark"`
	Name         string `json:"name"`
	UserID       uint64 `json:"user_id"`
	Username     string `json:"username"`
	ProviderID   uint64 `json:"provider_id"`
	Stage        string `json:"stage"`        // 当前生命周期阶段
	TargetStage  string `json:"target_stage"` // 本次将推进到的阶段
	Action       string `json:"action"`       // suspend / destroy / unsuspend / mark
	Reason       string `json:"reason"`       // 判定依据（人话）
	ExpireAt     string `json:"expire_at"`
	DaysLeft     int    `json:"days_left"`
	// CapabilityMissing 上游缺该能力时置位，提示「仅标记、需人工跟进」。
	CapabilityMissing bool `json:"capability_missing"`
}

// EnforcementPreviewResponse 到期处置预演报告。
type EnforcementPreviewResponse struct {
	// Enabled/DryRun 回显当前策略闸门状态，前端据此提示「这是预演还是真执行」。
	Enabled bool `json:"enabled"`
	DryRun  bool `json:"dry_run"`
	Total   int  `json:"total"`
	// StageCounts 按目标阶段计数（grace/suspended/destroyed/active）。
	StageCounts map[string]int           `json:"stage_counts"`
	Items       []EnforcementPreviewItem `json:"items"`
	GeneratedAt string                   `json:"generated_at"`
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
	Items  []UserInstanceRenewalItem `json:"items"`
	Policy PolicyResponse            `json:"policy"` // 策略摘要（宽限期等展示用）
}

// RenewalCreatedResponse 续费订单创建结果（待支付）
type RenewalCreatedResponse struct {
	Renewal RenewalInfo `json:"renewal"`
}

// UserRenewalListQuery 我的续费记录查询
type UserRenewalListQuery struct {
	Status   string `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}
