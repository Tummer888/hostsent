// Package model 定义实例运维管理台的数据模型。
package model

import "time"

// 操作人类型。
const (
	OperatorTypeAdmin  = "admin"
	OperatorTypeUser   = "user"
	OperatorTypeSystem = "system"
)

// 运维动作（写入 instance_operations.action）。
const (
	ActionPowerOn    = "power_on"
	ActionPowerOff   = "power_off"
	ActionHardOff    = "hard_off"
	ActionReboot     = "reboot"
	ActionHardReboot = "hard_reboot"
	ActionVNC        = "vnc"
	ActionSync       = "sync"
	ActionResize     = "resize"
	ActionDestroy    = "destroy"
	ActionRemark     = "remark"
	ActionSuspend    = "suspend"
)

// 操作结果。
const (
	ResultSuccess = "success"
	ResultFailed  = "failed"
)

// Operation 实例运维操作流水（管理端/用户端/系统自动操作统一落库）。
//
// 与 admin_audit_logs 的分工：后者记录管理端 HTTP 写请求，粒度是"谁请求了哪个路径"；
// 本表记录实例语义操作，且系统/定时任务发起的动作不经 HTTP，审计中间件录不到。
type Operation struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	InstanceID   uint64    `gorm:"column:instance_id;not null;index"`
	InstanceMark string    `gorm:"column:instance_mark;size:64"`
	UserID       uint64    `gorm:"column:user_id;not null;index"`
	OperatorType string    `gorm:"column:operator_type;size:20;not null;default:admin"`
	OperatorID   uint64    `gorm:"column:operator_id"`
	OperatorName string    `gorm:"column:operator_name;size:64"`
	Action       string    `gorm:"size:32;not null;index"`
	Params       string    `gorm:"type:text"`
	BeforeStatus string    `gorm:"column:before_status;size:30"`
	AfterStatus  string    `gorm:"column:after_status;size:30"`
	Result       string    `gorm:"size:20;not null;default:success"`
	ErrorMessage string    `gorm:"column:error_message;size:500"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (Operation) TableName() string { return "instance_operations" }
