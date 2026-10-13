// Package model 定义实例运维管理台的数据模型。
//
// 目录形态说明：instance 域采用「域级共享模型 + 子模块」的混合结构——
// 本包的模型供域内各子模块（如 lifecycle/）共享引用，lifecycle 自带
// 完整五件套（dto/model/repository/service/handler）。依据《CONTRIBUTING》
// 层级规则：域内含多个子模块且需共享模型时，允许此形态。
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
	// ActionProvisionRetry 重新开通：对待开通实例（占位行）重投来源订单的履约任务。
	ActionProvisionRetry = "provision_retry"
	ActionResize         = "resize"
	ActionDestroy        = "destroy"
	ActionRemark         = "remark"
	ActionSuspend        = "suspend"
	ActionUnsuspend      = "unsuspend"
	// 维护类动作（平台接口实测存在，映射见 dcimcloud/provider.go 头部注释）。
	ActionReinstall     = "reinstall"      // 重装系统（可换镜像）
	ActionResetPassword = "reset_password" // 重置登录密码
	ActionRescue        = "rescue"         // 进入救援系统
	ActionExitRescue    = "exit_rescue"    // 退出救援系统
	ActionSnapCreate    = "snapshot_create"
	ActionSnapDelete    = "snapshot_delete"
	ActionSnapRestore   = "snapshot_restore"
	ActionBandwidth     = "bandwidth"
	ActionAddIP         = "add_ip"
	ActionAttachDisk    = "attach_disk"
	// ActionStage 生命周期阶段推进（T5.4 自动扫描落阶段，无上游动作时记录阶段变化）。
	ActionStage = "stage"
	// ActionBillingAdjust 手工调整账期/付款周期（管理端自定义，不产生账单）。
	ActionBillingAdjust = "billing_adjust"
	// ActionRevealCredential 查看实例登录明文密码（只读但属敏感信息访问，必须留痕）。
	ActionRevealCredential = "reveal_credential"
)

// 操作结果。
const (
	ResultSuccess = "success"
	ResultFailed  = "failed"
	// ResultSkipped 未真正下发上游的情况：自动执行总开关关闭、预演、上游缺能力仅标记。
	ResultSkipped = "skipped"
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
