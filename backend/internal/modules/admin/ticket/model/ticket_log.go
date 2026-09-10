package model

import "time"

// 工单日志动作
const (
	LogActionCreate   string = "create"   // 创建
	LogActionAssign   string = "assign"   // 分配
	LogActionClaim    string = "claim"    // 认领
	LogActionTransfer string = "transfer" // 转派
	LogActionReply    string = "reply"    // 回复
	LogActionStatus   string = "status"   // 状态变更
	LogActionClose    string = "close"    // 关闭
	LogActionCancel   string = "cancel"   // 取消
)

// TicketLog 工单操作日志（详情页时间线）。
type TicketLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	TicketID     uint64    `gorm:"column:ticket_id;index;not null"`
	OperatorID   uint64    `gorm:"column:operator_id;not null;default:0"`
	OperatorName string    `gorm:"column:operator_name;size:64;not null;default:''"`
	Action       string    `gorm:"size:32;not null"`
	FromValue    string    `gorm:"column:from_value;size:128"`
	ToValue      string    `gorm:"column:to_value;size:128"`
	Note         string    `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (TicketLog) TableName() string {
	return "ticket_logs"
}
