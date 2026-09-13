// Package model 提供工单域的数据模型。
package model

import "time"

// 工单状态
const (
	TicketStatusOpen        string = "open"         // 新建/待响应
	TicketStatusInProgress  string = "in_progress"  // 处理中
	TicketStatusWaitingUser string = "waiting_user" // 等待用户回复
	TicketStatusResolved    string = "resolved"     // 已解决
	TicketStatusClosed      string = "closed"       // 已关闭
	TicketStatusCancelled   string = "cancelled"    // 已取消
)

// 工单优先级
const (
	PriorityLow    string = "low"    // 低
	PriorityMedium string = "medium" // 中
	PriorityHigh   string = "high"   // 高
	PriorityUrgent string = "urgent" // 紧急
)

// 发送人类型
const (
	SenderTypeUser  string = "user"  // 用户发送
	SenderTypeAdmin string = "admin" // 管理员发送
)

// 分类状态
const (
	CategoryStatusActive   string = "active"   // 启用
	CategoryStatusDisabled string = "disabled" // 禁用
)

// 回复/工单复核状态（S3 双人复核）：仅 need_review 分类的管理员回复进入 pending。
const (
	ReviewStatusNone     string = ""         // 无需复核（分类未开启，或为内部备注）
	ReviewStatusPending  string = "pending"  // 待复核，用户不可见
	ReviewStatusApproved string = "approved" // 已通过，用户可见
	ReviewStatusRejected string = "rejected" // 已驳回，用户不可见（需改写后重发）
)

// 复核动作
const (
	ReviewActionApprove string = "approve"
	ReviewActionReject  string = "reject"
)

// Ticket 工单主表
type Ticket struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	TicketNo     string `gorm:"column:ticket_no;size:64;uniqueIndex;not null"` // 工单号
	UserID       uint64 `gorm:"column:user_id;index;not null"`                 // 提交用户
	Title        string `gorm:"size:255;not null"`                             // 工单标题
	Description  string `gorm:"type:text"`                                     // 首条描述
	Category     string `gorm:"size:64;not null"`                              // 分类编码
	CategoryID   uint64 `gorm:"column:category_id;index"`                      // 分类ID
	Priority     string `gorm:"size:32;not null;default:medium"`               // 优先级
	Status       string `gorm:"size:32;not null;default:open;index"`           // 工单状态
	AssignedTo   uint64 `gorm:"column:assigned_to;index"`                      // 分配给的管理员ID
	AssignedName string `gorm:"column:assigned_name;size:64"`                  // 处理人名称快照
	// DepartmentID 归属部门（创建时取分类的 department_id 快照，S2）：
	// 派单与数据范围隔离据此判定，分类事后改部门不改变历史工单归属。
	DepartmentID uint64 `gorm:"column:department_id;index"`
	OrderID      uint64 `gorm:"column:order_id;index"`    // 关联订单（可选）
	InstanceID   uint64 `gorm:"column:instance_id;index"` // 关联实例（可选）
	// —— S3 双人复核：工单级复核状态（回复级 pending 时同步置位，便于列表筛选）——
	ReviewStatus      string     `gorm:"column:review_status;size:20;index"` // pending/approved/rejected/空
	ReviewerID        uint64     `gorm:"column:reviewer_id"`                 // 最近复核人
	ReviewRequestedBy uint64     `gorm:"column:review_requested_by"`         // 待复核回复的提交人（用于禁止自审）
	ReviewedAt        *time.Time `gorm:"column:reviewed_at"`
	ReviewNote        string     `gorm:"column:review_note;size:255"` // 复核意见（驳回原因）
	FirstReplyAt      *time.Time `gorm:"column:first_reply_at"`       // 首次响应时间
	ResolvedAt        *time.Time `gorm:"column:resolved_at"`          // 解决时间
	ClosedAt          *time.Time `gorm:"column:closed_at"`            // 关闭时间
	CreatedAt         time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Ticket) TableName() string {
	return "tickets"
}
