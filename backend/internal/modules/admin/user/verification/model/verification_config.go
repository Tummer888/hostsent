// Package model 提供实名认证模块的数据模型。
package model

import "time"

// 实名配置的键名与默认值（doc104 §5.4）。
//
// 全部落在 config_group='verification'。默认值的取舍原则：
//   - 安全侧默认从严：require_before_order=false 只是不阻断下单，
//     但 real_name_locked=true 与 resubmit_cooldown_hours=24 防止反复提交刷审核队列；
//   - 自动化默认关闭：auto_approve_on_pass=false —— 三方核验通过即自动放行是
//     需要运营明确知情的策略，不能默认打开。
const (
	ConfigGroupVerification = "verification"

	ConfigKeyEnabled           = "verification.enabled"
	ConfigKeyAllowedTypes      = "verification.allowed_types"
	ConfigKeyProvider          = "verification.provider"
	ConfigKeyAutoApproveOnPass = "verification.auto_approve_on_pass"
	ConfigKeyAutoRejectOnFail  = "verification.auto_reject_on_fail"
	ConfigKeyRequireBeforeOrd  = "verification.require_before_order"
	ConfigKeyResubmitCooldown  = "verification.resubmit_cooldown_hours"
	ConfigKeyRealNameLocked    = "verification.real_name_locked"

	// DefaultResubmitCooldownHours 驳回后重新提交的默认冷却小时数。
	DefaultResubmitCooldownHours = 24
)

// ConfigDefault 一条配置的默认定义，供迁移/种子与「配置缺失时的回落」共用。
type ConfigDefault struct {
	Key         string
	Value       string
	ValueType   string
	Description string
	SortOrder   int
}

// VerificationConfigDefaults 全部实名配置的默认值。
//
// 这是唯一的默认值来源：迁移 052 的 seed 与运行时回落都从这里取，
// 避免「库里写 24 小时、代码里回落 48 小时」这类两套口径（doc104 §7）。
func VerificationConfigDefaults() []ConfigDefault {
	return []ConfigDefault{
		{ConfigKeyEnabled, "true", "bool", "是否开放用户端自助提交实名认证", 1},
		{ConfigKeyAllowedTypes, `["personal","enterprise"]`, "json", "允许的认证主体类型", 2},
		{ConfigKeyProvider, "manual", "string", "生效的核验服务商（manual=纯人工审核）", 3},
		{ConfigKeyAutoApproveOnPass, "false", "bool", "三方核验通过即自动通过（跳过人工审核）", 4},
		{ConfigKeyAutoRejectOnFail, "false", "bool", "三方核验明确失败即自动驳回", 5},
		{ConfigKeyRequireBeforeOrd, "false", "bool", "下单前强制实名认证", 6},
		{ConfigKeyResubmitCooldown, "24", "int", "驳回后重新提交的冷却小时数", 7},
		{ConfigKeyRealNameLocked, "true", "bool", "通过后禁止再次提交（改名需走撤销）", 8},
	}
}

// VerificationConfig 表示实名认证配置项。
type VerificationConfig struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `gorm:"column:config_key;size:128;not null;uniqueIndex"`
	ConfigGroup string    `gorm:"column:config_group;size:64;not null;index"`
	ConfigValue string    `gorm:"column:config_value;type:text;not null"`
	ValueType   string    `gorm:"column:value_type;size:32;not null"`
	Status      string    `gorm:"column:status;size:32;not null;index"`
	Description string    `gorm:"column:description;size:255"`
	UpdatedBy   uint64    `gorm:"column:updated_by;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 返回实名认证配置表名。
func (VerificationConfig) TableName() string {
	return "verification_configs"
}
