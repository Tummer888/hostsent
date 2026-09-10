package model

import "time"

type UserGroup struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null;uniqueIndex"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Description string `gorm:"size:255"`
	Status      string `gorm:"size:32;not null;default:active"`
	SortOrder   int    `gorm:"column:sort_order;not null;default:0"`
	// 折扣来源绑定（D3：用户组是主折扣来源，绑定 price_policies）
	PricePolicyID *uint64   `gorm:"column:price_policy_id"`
	Priority      int       `gorm:"column:priority;not null;default:0"` // 组间优先级，兜底用
	IsDefault     bool      `gorm:"column:is_default;not null;default:false"`
	IsAgentGroup  bool      `gorm:"column:is_agent_group;not null;default:false"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}
