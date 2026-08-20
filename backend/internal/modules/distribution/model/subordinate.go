// Package model 定义分销模块的数据库实体。
package model

import "time"

type Subordinate struct {
	ID                 uint64     `gorm:"primaryKey;autoIncrement"`
	AgentID            uint64     `gorm:"column:agent_id;not null;index"`
	UserID             uint64     `gorm:"column:user_id;not null;index"`
	ParentAgentID      *uint64    `gorm:"column:parent_agent_id;index"`
	LevelDepth         int        `gorm:"column:level_depth;not null;default:1"`
	RelationType       string     `gorm:"column:relation_type;size:32;not null;default:direct"`
	ContributionAmount float64    `gorm:"column:contribution_amount;type:decimal(15,2);not null;default:0"`
	CommissionAmount   float64    `gorm:"column:commission_amount;type:decimal(15,2);not null;default:0"`
	Status             string     `gorm:"size:32;not null;default:active"`
	JoinedAt           *time.Time `gorm:"column:joined_at"`
	CreatedAt          time.Time  `gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime"`
}

// TableName 返回下级关系对应的数据表名。
func (Subordinate) TableName() string {
	return "distribution_subordinates"
}
