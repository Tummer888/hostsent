package model

import "time"

// UserGroup 用户组：**纯分类**，只回答"这个客户属于哪一类"（doc108）。
//
// 它不参与算价 —— 折扣的唯一来源是代理等级（agent_levels + agent_level_discounts）。
// 改造前这里是"折扣来源"（price_policy_id），导致改分类会误伤价格；代理域下线时
// 遗留的 is_agent_group 也是全仓零消费的死字段，两者已随 migration 059 删除。
//
// 保持单值语义（users.user_group_id 一个用户只属于一个组）：用户组只承载一个
// 分类维度。需要多维度人群（回访名单、活跃分层）时应另建标签体系，不要往这里加字段。
type UserGroup struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null;uniqueIndex"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Description string `gorm:"size:255"`
	Status      string `gorm:"size:32;not null;default:active"`
	SortOrder   int    `gorm:"column:sort_order;not null;default:0"`
	// IsDefault 默认用户组：新用户未指定分组时归入，全库至多一个。
	IsDefault bool      `gorm:"column:is_default;not null;default:false"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}
