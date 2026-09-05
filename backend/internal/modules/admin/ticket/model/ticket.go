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

// Ticket 工单主表
type Ticket struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	TicketNo     string     `gorm:"column:ticket_no;size:64;uniqueIndex;not null"` // 工单号
	UserID       uint64     `gorm:"column:user_id;index;not null"`                 // 提交用户
	Title        string     `gorm:"size:255;not null"`                             // 工单标题
	Description  string     `gorm:"type:text"`                                     // 首条描述
	Category     string     `gorm:"size:64;not null"`                              // 分类编码
	CategoryID   uint64     `gorm:"column:category_id;index"`                      // 分类ID
	Priority     string     `gorm:"size:32;not null;default:medium"`               // 优先级
	Status       string     `gorm:"size:32;not null;default:open;index"`           // 工单状态
	AssignedTo   uint64     `gorm:"column:assigned_to;index"`                      // 分配给的管理员ID
	AssignedName string     `gorm:"column:assigned_name;size:64"`                  // 处理人名称快照
	OrderID      uint64     `gorm:"column:order_id;index"`                         // 关联订单（可选）
	InstanceID   uint64     `gorm:"column:instance_id;index"`                      // 关联实例（可选）
	FirstReplyAt *time.Time `gorm:"column:first_reply_at"`                         // 首次响应时间
	ResolvedAt   *time.Time `gorm:"column:resolved_at"`                            // 解决时间
	ClosedAt     *time.Time `gorm:"column:closed_at"`                              // 关闭时间
	CreatedAt    time.Time  `gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Ticket) TableName() string {
	return "tickets"
}
