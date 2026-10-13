// Package model 提供实例对账的数据模型。
package model

import "time"

// 差异类型（reconcile_findings.kind）。
//
// 命名与 reconcile/dto 的 anomaly 码保持一致，便于前端把两类视图对上：
// 列表页展示"当前时刻"的比对结论，台账页展示"这条差异存在了多久、谁处理了"。
const (
	KindPriceBelowCost  = "price_below_cost"  // 本地售价低于上游成本（资金直接受损）
	KindPriceThinMargin = "price_thin_margin" // 毛利率低于阈值
	KindExpireEarly     = "expire_early"      // 本地到期早于上游（用户已付费区间被截短）
	KindExpireLate      = "expire_late"       // 本地到期晚于上游（上游可能已停服而本地仍计费）
	KindStatusMismatch  = "status_mismatch"   // 上游已暂停，本地仍视为运行中
	KindStatusStale     = "status_stale"      // 上游已删除/取消，本地仍视为有效
)

// 差异等级。
const (
	SeverityDanger  = "danger"
	SeverityWarning = "warning"
)

// 处置状态。
const (
	StatusOpen     = "open"     // 未处理
	StatusAck      = "ack"      // 已确认，暂不处理
	StatusIgnored  = "ignored"  // 忽略（不再提醒）
	StatusResolved = "resolved" // 差异条件已消失（自动收敛）
)

// KindSeverity 差异类型的固定等级。
//
// 为什么等级写死在类型上而不是每次比对时算：预警的语义是"这类问题有多严重"，
// 不随单次数值浮动。低于成本和上游已删除本地仍计费会直接亏钱，必须是 danger。
var KindSeverity = map[string]string{
	KindPriceBelowCost:  SeverityDanger,
	KindExpireEarly:     SeverityDanger,
	KindExpireLate:      SeverityDanger,
	KindStatusStale:     SeverityDanger,
	KindPriceThinMargin: SeverityWarning,
	KindStatusMismatch:  SeverityWarning,
}

// KindNames 差异类型展示名。
var KindNames = map[string]string{
	KindPriceBelowCost:  "售价低于成本",
	KindPriceThinMargin: "毛利过薄",
	KindExpireEarly:     "本地到期过早",
	KindExpireLate:      "本地到期过晚",
	KindStatusMismatch:  "上游已暂停本地未同步",
	KindStatusStale:     "上游已删除本地仍有效",
}

// KindLabel 取展示名，未知类型回落原码（避免新类型在页面上显示空白）。
func KindLabel(kind string) string {
	if name, ok := KindNames[kind]; ok {
		return name
	}
	return kind
}

// SeverityOf 取类型对应的等级，未登记按预警处理（宁可多提醒也不要漏掉资金风险）。
func SeverityOf(kind string) string {
	if s, ok := KindSeverity[kind]; ok {
		return s
	}
	return SeverityWarning
}

// ReconcileFinding 对账差异台账。
//
// 一行 = 某实例的某一类差异。UNIQUE(instance_id, kind) 保证同类差异只留一行：
// 重复检出只刷新 last_seen_at，条件消失由扫描置 resolved。这样"这条差异存在了
// 多久"是真实数据，而不是每轮扫描重新造一条。
type ReconcileFinding struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	ProviderID uint64 `gorm:"column:provider_id;not null;default:0;index"`
	// InstanceID 指向 instances.id（本地主键，不是上游实例号）：
	// 对账的比对对象是本地行，用本地主键关联最稳。
	InstanceID uint64 `gorm:"column:instance_id;not null;uniqueIndex:uk_reconcile_findings_instance_kind"`
	Kind       string `gorm:"column:kind;size:32;not null;uniqueIndex:uk_reconcile_findings_instance_kind"`
	Severity   string `gorm:"column:severity;size:16;not null"`
	// LocalValue / UpstreamValue 是差异两侧的展示快照（如 "38.00" / "42.50"），
	// 存字符串而非数值：价格与时间两类值形态不同，且台账只需可读回溯。
	LocalValue    string   `gorm:"column:local_value;type:text;not null;default:''"`
	UpstreamValue string   `gorm:"column:upstream_value;type:text;not null;default:''"`
	Delta         *float64 `gorm:"column:delta;type:decimal(18,4)"`
	// FirstSeenAt 首次检出：预警文案里的"已持续 N 天"以它为起点。
	FirstSeenAt time.Time `gorm:"column:first_seen_at;not null"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at;not null"`
	Status      string    `gorm:"column:status;size:16;not null;default:open"`
	// NoticeSeq 已发出的预警通知序号。通知的幂等键是 (source_module, source_id)，
	// 同一差异重开必须换 source_id 才能再提醒，故每次发出即自增。
	NoticeSeq  int        `gorm:"column:notice_seq;not null;default:0"`
	NotifiedAt *time.Time `gorm:"column:notified_at"`
	AckBy      *uint64    `gorm:"column:ack_by"`
	AckAt      *time.Time `gorm:"column:ack_at"`
	Note       string     `gorm:"column:note;size:255;not null;default:''"`
	CreatedAt  time.Time  `gorm:"autoCreateTime"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (ReconcileFinding) TableName() string { return "reconcile_findings" }
