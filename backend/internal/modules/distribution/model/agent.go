package model

import "time"

type Agent struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	UserID           uint64     `gorm:"column:user_id;not null;uniqueIndex"`
	AgentLevelID     uint64     `gorm:"column:agent_level_id;not null;index"`
	InviterAgentID   *uint64    `gorm:"column:inviter_agent_id;index"`
	InviteCode       string     `gorm:"column:invite_code;size:64;not null;uniqueIndex"`
	Status           string     `gorm:"size:32;not null;default:active"`
	DirectSubCount   int        `gorm:"column:direct_sub_count;not null;default:0"`
	TeamSubCount     int        `gorm:"column:team_sub_count;not null;default:0"`
	TotalCommission  float64    `gorm:"column:total_commission;type:decimal(15,2);not null;default:0"`
	AvailableBalance float64    `gorm:"column:available_balance;type:decimal(15,2);not null;default:0"`
	LastSettledAt    *time.Time `gorm:"column:last_settled_at"`
	Remark           string     `gorm:"size:255"`
	CreatedAt        time.Time  `gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime"`
}

func (Agent) TableName() string {
	return "distribution_agents"
}
