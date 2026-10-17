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
	// KindStatusMismatch 服务状态两侧对不上号（上游暂停/开通中，本地口径不一致）。
	//
	// 注意：**不含机器电源状态**。财务型上游（zjmf）没有电源字段，客户关机后
	// domainstatus 仍是 Active，那是"机器没通电"而非"服务状态不同步"——
	// 混进来会每次关机都报一条（第二十一轮修复的误报）。
	KindStatusMismatch = "status_mismatch"
	KindStatusStale    = "status_stale" // 上游已终止，本地服务仍视为在用
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
//
// 文案只描述**服务**维度，不写机器开关机：上游侧是 domainstatus（服务状态），
// 本地侧是 lifecycle_stage 归一后的服务口径，两侧都比不到"机器通没通电"。
// 用"已暂停/未同步"这种词会让人以为"我关了机、系统没同步"（第二十一轮用户反馈）。
var KindNames = map[string]string{
	KindPriceBelowCost:  "售价低于成本",
	KindPriceThinMargin: "毛利过薄",
	KindExpireEarly:     "本地到期过早",
	KindExpireLate:      "本地到期过晚",
	KindStatusMismatch:  "服务状态两侧不一致",
	KindStatusStale:     "上游已终止本地仍在服务",
}

// KindLabel 取展示名，未知类型回落原码（避免新类型在页面上显示空白）。
func KindLabel(kind string) string {
	if name, ok := KindNames[kind]; ok {
		return name
	}
	return kind
}

// serviceStateLabels 服务维度取值的中文名（台账两侧的展示口径）。
//
// 台账存的是归一值（本地侧 lifecycle_stage 归一后的服务口径、上游侧 domainstatus 原文），
// 直接展示会露出 "active"/"Active" 这样的内部码 —— 台账页"本地/上游"两行并排，
// 一边中文一边英文码既难读、也让人以为两侧是两回事。
var serviceStateLabels = map[string]string{
	// 本地侧（归一值）
	"active":       "服务中",
	"grace":        "宽限期",
	"provisioning": "开通中",
	"suspended":    "已暂停",
	"destroyed":    "已终止",
	"cancelled":    "已取消",
	"unknown":      "未知",
	// 上游侧（domainstatus 原文）
	"Active":     "服务中",
	"Pending":    "开通中",
	"Suspended":  "已暂停",
	"Deleted":    "已终止",
	"Terminated": "已终止",
	"Cancelled":  "已取消",
	"Fraud":      "风控冻结",
}

// ServiceStateLabel 服务维度取值的中文展示；未登记原样回显（便于发现新状态）。
func ServiceStateLabel(v string) string {
	if name, ok := serviceStateLabels[v]; ok {
		return name
	}
	return v
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
