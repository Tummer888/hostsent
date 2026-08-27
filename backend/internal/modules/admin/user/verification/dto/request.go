// Package dto 提供实名认证模块的请求与响应数据结构。
package dto

type VerificationListQuery struct {
	Page             int    `form:"page"`
	PageSize         int    `form:"page_size"`
	UserID           uint64 `form:"user_id"`
	Username         string `form:"username"`
	VerificationType string `form:"verification_type"`
	ReviewerName     string `form:"reviewer_name"`
	Keyword          string `form:"keyword"`
	StartTime        string `form:"start_time"`
	EndTime          string `form:"end_time"`
}

// VerificationConfigListQuery 表示实名认证配置列表查询条件。
type VerificationConfigListQuery struct {
	ConfigGroup string `form:"config_group"`
	Status      string `form:"status"`
}
