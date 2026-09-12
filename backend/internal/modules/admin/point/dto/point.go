// Package dto 提供积分体系的数据传输结构。
package dto

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `form:"page" json:"page"`
	PageSize int   `form:"page_size" json:"page_size"`
	Total    int64 `json:"total"`
}

// —— 规则 ——

// RuleListQuery 规则列表查询。
type RuleListQuery struct {
	Scene    string `form:"scene" json:"scene"`
	Status   *int   `form:"status" json:"status"`
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
}

// RuleInfo 规则信息。
type RuleInfo struct {
	ID                uint64  `json:"id"`
	Code              string  `json:"code"`
	Name              string  `json:"name"`
	Scene             string  `json:"scene"`
	EarnMode          string  `json:"earn_mode"`
	FixedPoints       int64   `json:"fixed_points"`
	PointsPerYuan     float64 `json:"points_per_yuan"`
	MinAmount         float64 `json:"min_amount"`
	MaxPointsPerOrder int64   `json:"max_points_per_order"`
	ValidDays         int     `json:"valid_days"`
	Status            int     `json:"status"`
	SortOrder         int     `json:"sort_order"`
	Remark            string  `json:"remark"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// RuleListResponse 规则列表响应。
type RuleListResponse struct {
	Items []RuleInfo `json:"items"`
	Meta  ListMeta   `json:"meta"`
}

// RuleSaveRequest 新建/更新规则。
type RuleSaveRequest struct {
	Code              string  `json:"code"`
	Name              string  `json:"name" binding:"required"`
	Scene             string  `json:"scene" binding:"required"`
	EarnMode          string  `json:"earn_mode" binding:"required"`
	FixedPoints       int64   `json:"fixed_points"`
	PointsPerYuan     float64 `json:"points_per_yuan"`
	MinAmount         float64 `json:"min_amount"`
	MaxPointsPerOrder int64   `json:"max_points_per_order"`
	ValidDays         int     `json:"valid_days"`
	Status            *int    `json:"status"`
	SortOrder         int     `json:"sort_order"`
	Remark            string  `json:"remark"`
}

// —— 账户 ——

// AccountListQuery 积分账户列表查询。
type AccountListQuery struct {
	UserID      uint64 `form:"user_id" json:"user_id"`
	UserKeyword string `form:"user_keyword" json:"user_keyword"` // 用户名/邮箱模糊
	MinPoints   int64  `form:"min_points" json:"min_points"`     // 最低可用积分
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
}

// AccountInfo 积分账户信息。
type AccountInfo struct {
	ID          uint64 `json:"id"`
	UserID      uint64 `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Balance     int64  `json:"balance"`
	Frozen      int64  `json:"frozen"`
	TotalEarned int64  `json:"total_earned"`
	TotalSpent  int64  `json:"total_spent"`
	UpdatedAt   string `json:"updated_at"`
}

// AccountListResponse 积分账户列表响应。
type AccountListResponse struct {
	Items []AccountInfo `json:"items"`
	Meta  ListMeta      `json:"meta"`
}

// AdjustRequest 人工调整积分。
type AdjustRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Points int64  `json:"points" binding:"required"` // 正数=发放，负数=扣减
	Remark string `json:"remark" binding:"required"`
}

// —— 流水 ——

// TransactionListQuery 积分流水查询。
type TransactionListQuery struct {
	UserID    uint64 `form:"user_id" json:"user_id"`
	Type      string `form:"type" json:"type"`
	Direction int    `form:"direction" json:"direction"`
	StartTime string `form:"start_time" json:"start_time"`
	EndTime   string `form:"end_time" json:"end_time"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
}

// TransactionInfo 积分流水信息。
type TransactionInfo struct {
	ID            uint64 `json:"id"`
	TxNo          string `json:"tx_no"`
	UserID        uint64 `json:"user_id"`
	Username      string `json:"username"`
	Type          string `json:"type"`
	Direction     int    `json:"direction"`
	Points        int64  `json:"points"`
	BalanceBefore int64  `json:"balance_before"`
	BalanceAfter  int64  `json:"balance_after"`
	BizType       string `json:"biz_type"`
	RefNo         string `json:"ref_no"`
	Remark        string `json:"remark"`
	OperatorID    uint64 `json:"operator_id"`
	ExpireAt      string `json:"expire_at"`
	CreatedAt     string `json:"created_at"`
}

// TransactionListResponse 积分流水列表响应。
type TransactionListResponse struct {
	Items []TransactionInfo `json:"items"`
	Meta  ListMeta          `json:"meta"`
}

// —— 用户端 ——

// MyOverview 我的积分概览（用户端）。
type MyOverview struct {
	UserID      uint64            `json:"user_id"`
	Balance     int64             `json:"balance"`
	Frozen      int64             `json:"frozen"`
	TotalEarned int64             `json:"total_earned"`
	TotalSpent  int64             `json:"total_spent"`
	Rules       []RuleInfo        `json:"rules"`  // 生效规则（说明积分怎么来）
	Recent      []TransactionInfo `json:"recent"` // 最近流水（首页摘要）
}

// OverviewResponse 管理端积分概览（积分中心首页）。
type OverviewResponse struct {
	TotalBalance int64             `json:"total_balance"` // 全平台可用积分
	TotalEarned  int64             `json:"total_earned"`  // 累计发放
	TotalSpent   int64             `json:"total_spent"`   // 累计消耗
	AccountCount int64             `json:"account_count"` // 积分账户数
	Rules        []RuleInfo        `json:"rules"`         // 生效规则
	Recent       []TransactionInfo `json:"recent"`        // 最近流水
}
