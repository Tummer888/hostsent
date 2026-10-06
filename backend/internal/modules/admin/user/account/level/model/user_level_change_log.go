// Package model 定义用户等级模块的数据库实体。
package model

import "time"

// 等级变更原因。两条通道都写本表，靠 reason 区分「谁做的决定」：
// 消费升级由累计消费自动触发且只升不降，人工调整由管理员在用户详情页发起、可升可降。
const (
	// ReasonConsumeUpgrade 累计消费达到门槛自动升级。
	ReasonConsumeUpgrade = "consume_upgrade"
	// ReasonManualAdjust 管理员在用户详情页手工调整等级。
	ReasonManualAdjust = "manual_adjust"
)

// UserLevelChangeLog 记录用户等级变更（P3-03）。
//
// 本期权益「只记录不发放」：等级变更时把当时的累计消费与权益快照写入本表，
// 便于运营追溯与后续补发，不触发任何自动发放逻辑。
type UserLevelChangeLog struct {
	ID            uint64  `gorm:"primaryKey;autoIncrement"`
	UserID        uint64  `gorm:"column:user_id;not null;index"`
	FromLevelID   *uint64 `gorm:"column:from_level_id"`
	FromLevelCode string  `gorm:"column:from_level_code;size:64"`
	ToLevelID     uint64  `gorm:"column:to_level_id;not null"`
	ToLevelCode   string  `gorm:"column:to_level_code;size:64;not null"`
	// TotalConsumeAmount 触发变更时的累计消费额。
	TotalConsumeAmount float64 `gorm:"column:total_consume_amount;type:decimal(15,2);not null;default:0"`
	// BenefitsSnapshot 目标等级当时的 benefits 快照（JSON 文本）。
	BenefitsSnapshot string `gorm:"column:benefits_snapshot;type:text"`
	// Reason 变更原因：consume_upgrade（消费升级）/ manual_adjust（人工调整）。
	Reason    string    `gorm:"size:64;not null;default:consume_upgrade"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名。
func (UserLevelChangeLog) TableName() string {
	return "user_level_change_logs"
}
