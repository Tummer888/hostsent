package model

// TicketStatusCount 工单状态计数（统计聚合结果）
type TicketStatusCount struct {
	Status string `gorm:"column:status"`
	Count  int64  `gorm:"column:count"`
}

// TicketTrendPoint 工单趋势点（按日聚合：工单量）
type TicketTrendPoint struct {
	Date   string `gorm:"column:date"`
	Count  int64  `gorm:"column:count"`
	NewCnt int64  `gorm:"column:new_cnt"`
	Closed int64  `gorm:"column:closed"`
}

// TicketCategoryStat 分类工单分布（统计聚合结果）
type TicketCategoryStat struct {
	Category string `gorm:"column:category"`
	Count    int64  `gorm:"column:count"`
}
