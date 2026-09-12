// Package dto 提供任务队列模块的数据传输结构。
//
// 任务队列（本轮 S3）把平台发往上游的四类动作聚合为一个只读队列：
//   - provision       开通履约（订单支付后异步开通上游实例）
//   - instance_action 实例动作（暂停 / 恢复 / 重启 / 关机 / 开机 / 变配 / 销毁）
//   - renewal         续费（链路 A 走上游续费，链路 B 本地账期顺延）
//   - sync            上游同步（目录 / 资源池 / 实例拉取）
//
// 每行带 UpstreamState，回答运营最关心的问题：「这个动作有没有正确到达上游」。
package dto

import commondto "hostsent/backend/internal/modules/admin/resource/common/dto"

// ListMeta 通用分页元信息（复用 common 包定义）
type ListMeta = commondto.ListMeta

// 任务类别（Category）。
const (
	CategoryProvision      = "provision"
	CategoryInstanceAction = "instance_action"
	CategoryRenewal        = "renewal"
	CategorySync           = "sync"
)

// 上游到达状态（UpstreamState）。
const (
	// UpstreamReached 已到达上游且上游返回成功。
	UpstreamReached = "reached"
	// UpstreamNotReached 未到达上游 / 上游拒绝（失败、熔断、人工队列）。
	UpstreamNotReached = "not_reached"
	// UpstreamPending 尚未执行（排队中或执行中），是否到达上游待定。
	UpstreamPending = "pending"
	// UpstreamNotApplicable 该动作无需上游（如链路 B 本地账期顺延），不参与到达率统计。
	UpstreamNotApplicable = "not_applicable"
	// UpstreamSkipped 渠道不支持该能力，已显式跳过（既非成功也非失败）。
	UpstreamSkipped = "skipped"
)

// TaskQueueListQuery 任务队列列表查询。
type TaskQueueListQuery struct {
	// Category 任务类别：provision / instance_action / renewal / sync，空表示全部。
	Category string `form:"category" json:"category"`
	// Status 原始状态过滤（pending/running/success/failed/manual/skipped/cancelled）。
	Status string `form:"status" json:"status"`
	// UpstreamState 上游到达状态过滤。
	UpstreamState string `form:"upstream_state" json:"upstream_state"`
	// ProviderID 按渠道过滤。
	ProviderID uint64 `form:"provider_id" json:"provider_id"`
	// Keyword 按主体 / 单号 / 实例标识 / 错误信息模糊搜索。
	Keyword string `form:"keyword" json:"keyword"`
	// CreatedFrom / CreatedTo 创建时间区间（RFC3339 或 YYYY-MM-DD）。
	CreatedFrom string `form:"created_from" json:"created_from"`
	CreatedTo   string `form:"created_to" json:"created_to"`
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"page_size" json:"page_size"`
}

// TaskQueueItem 队列行。
type TaskQueueItem struct {
	// ID 复合主键 "category:ref_id"，供前端行 key 使用。
	ID           string `json:"id"`
	Category     string `json:"category"`
	CategoryName string `json:"category_name"`
	// Action 原始动作码；ActionName 为展示名（如「重启」「续费」）。
	Action     string `json:"action"`
	ActionName string `json:"action_name"`
	// Subject 主体（实例名 / 商品名 / 渠道名）。
	Subject string `json:"subject"`
	// RefNo 业务单号（订单号 / 续费单号）。
	RefNo string `json:"ref_no"`
	// InstanceRef 实例标识（上游实例号快照）。
	InstanceRef string `json:"instance_ref"`
	// Status 归一后的原始状态；StatusName 为展示名。
	Status     string `json:"status"`
	StatusName string `json:"status_name"`
	// UpstreamState 是否到达上游；UpstreamStateName 为展示名。
	UpstreamState     string `json:"upstream_state"`
	UpstreamStateName string `json:"upstream_state_name"`
	// UpstreamDetail 上游侧细节（错误信息 / 上游单号）。
	UpstreamDetail string `json:"upstream_detail"`
	ProviderID     uint64 `json:"provider_id"`
	ProviderName   string `json:"provider_name"`
	UserID         uint64 `json:"user_id"`
	Username       string `json:"username"`
	// Attempts / MaxAttempts 仅开通履约任务有意义（重试次数 / 上限）。
	Attempts    int     `json:"attempts"`
	MaxAttempts int     `json:"max_attempts"`
	Amount      float64 `json:"amount"`
	CreatedAt   string  `json:"created_at"`
	FinishedAt  *string `json:"finished_at"`
	// DurationMs 已完成任务的耗时（毫秒）；未完成或瞬时动作为 0。
	DurationMs int64 `json:"duration_ms"`
}

// TaskQueueCategoryCount 按类别的计数（供页签角标）。
type TaskQueueCategoryCount struct {
	Category     string `json:"category"`
	CategoryName string `json:"category_name"`
	Total        int64  `json:"total"`
	NotReached   int64  `json:"not_reached"`
}

// TaskQueueSummary 队列汇总（按当前筛选，不受分页影响）。
type TaskQueueSummary struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	Running    int64 `json:"running"`
	Success    int64 `json:"success"`
	Failed     int64 `json:"failed"`
	Manual     int64 `json:"manual"`
	Reached    int64 `json:"reached"`
	NotReached int64 `json:"not_reached"`
	// ReachedRate 到达率百分比（reached / (reached + not_reached)，0~100）。
	ReachedRate float64 `json:"reached_rate"`
	// Categories 各类别计数（不含 category 过滤，供页签展示）。
	Categories []TaskQueueCategoryCount `json:"categories"`
}

// TaskQueueListResponse 任务队列列表响应。
type TaskQueueListResponse struct {
	Items   []TaskQueueItem  `json:"items"`
	Meta    ListMeta         `json:"meta"`
	Summary TaskQueueSummary `json:"summary"`
}
