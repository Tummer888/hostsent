// Package model 提供成本管理子域的数据模型。
//
// 三个口径来源（见 service 层口径说明与 doc111）：
//   - 上游成本：上游账本流水（消费/充值，适配器能读账本的渠道全自动同步）；
//     余额快照只用于展示当前余额与「余额水位告警」，不再参与成本推算；
//   - 自营/固定成本：成本项配置（母机月费、员工工资、机房带宽…按需增删）；
//   - 收入：复用财务统计的服务收入口径（消费 − 退款），不在此重复定义 SQL。
package model

import "time"

// 成本项分类。分类只影响展示分组，不影响合计口径。
const (
	CategorySelfHosted  string = "self_hosted"  // 自营宿主机/母机
	CategoryUpstreamOps string = "upstream_ops" // 上游运营（备用线路、代维）
	CategoryLabor       string = "labor"        // 人力（工资、社保、外包）
	CategoryInfra       string = "infra"        // 基础设施（机房、带宽、域名、证书）
	CategoryOther       string = "other"        // 其他
)

// CategoryLabels 分类中文名（后端也给出，避免前后端两套文案漂移）。
var CategoryLabels = map[string]string{
	CategorySelfHosted:  "自营宿主机",
	CategoryUpstreamOps: "上游运营",
	CategoryLabor:       "人力成本",
	CategoryInfra:       "基础设施",
	CategoryOther:       "其他",
}

// 计费周期。
const (
	CycleMonthly string = "monthly" // 按月固定（按自然月计入）
	CycleOnce    string = "once"    // 一次性（计入 occurred_on 所在自然月）
)

// 状态。
const (
	StatusActive   string = "active"
	StatusDisabled string = "disabled"
)

// 账本流水方向（与 pkg/upstream 的 LedgerKind* 同值；成本模块不 import 协议包，
// 由装配层做同名映射，避免领域模型被协议类型渗透）。
const (
	LedgerKindConsume string = "consume" // 余额支付消费（订购/续费）
	LedgerKindTopup   string = "topup"   // 充值/入账
)

// 快照来源（当前只可能是自动抓取：手工录入已下线，见 doc111 §5.4）。
const (
	SnapshotSourceAuto string = "auto" // 定时/手动触发抓取
)

// CostItem 成本项配置。
//
// 设计取舍：成本项刻意做成「配置项」而不是「资产台账」—— 平台当前没有母机/员工实体，
// 硬造一套资产表只会让运营在两张表之间来回跳；subject 字段用于写清成本对象
// （如「华南母机-01」「客服小王」），等真有资产台账时再挂外键即可。
type CostItem struct {
	ID       uint64  `gorm:"primaryKey;autoIncrement"`
	Name     string  `gorm:"size:100;not null"`
	Category string  `gorm:"size:32;not null;index"`
	Amount   float64 `gorm:"type:decimal(15,2);not null;default:0"`
	// Cycle monthly=按月计入；once=一次性，计入 OccurredOn 所在自然月。
	Cycle string `gorm:"size:16;not null;default:monthly"`
	// OccurredOn 一次性成本的发生日期（Cycle=once 时必填）。
	OccurredOn *time.Time `gorm:"column:occurred_on;type:date"`
	// EffectiveFrom/To 生效区间（含端点）；EffectiveTo 为空表示长期有效。
	// 按月成本项在区间与自然月相交时计入（月中开始/结束按整月计入，见口径说明）。
	EffectiveFrom time.Time  `gorm:"column:effective_from;type:date;not null"`
	EffectiveTo   *time.Time `gorm:"column:effective_to;type:date"`
	Subject       string     `gorm:"size:120;not null;default:''"` // 成本对象（母机名/员工/线路）
	Remark        string     `gorm:"size:255;not null;default:''"`
	Status        string     `gorm:"size:16;not null;default:active;index"`
	OperatorID    uint64     `gorm:"column:operator_id;not null;default:0"` // 最近操作人
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (CostItem) TableName() string { return "cost_items" }

// UpstreamBalanceSnapshot 上游渠道余额快照。
//
// 唯一用途是「当前余额」：台账页展示水位、低余额告警取数。
// 同一渠道同一天只保留一条（唯一索引）：重复抓取直接覆盖，避免同一天多值。
// 每日由调度器自动抓一次（finance.cost_snapshot_hour），页面也可手动触发抓取。
type UpstreamBalanceSnapshot struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	ProviderID uint64 `gorm:"column:provider_id;not null;index:idx_ub_snapshot_provider_date,priority:1"`
	// SnapshotDate 快照日期（只到天）；与 ProviderID 组成唯一键。
	SnapshotDate time.Time `gorm:"column:snapshot_date;type:date;not null;uniqueIndex:uk_ub_snapshot_provider_date,priority:2;index:idx_ub_snapshot_provider_date,priority:2"`
	Balance      float64   `gorm:"type:decimal(15,2);not null;default:0"`
	Currency     string    `gorm:"size:8;not null;default:'CNY'"`
	Source       string    `gorm:"size:16;not null;default:auto"`
	Remark       string    `gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (UpstreamBalanceSnapshot) TableName() string { return "upstream_balance_snapshots" }

// UpstreamLedgerEntry 上游账本流水（消费 / 充值）。
//
// 上游成本的唯一自动来源：每一笔扣款/充值的时间、金额、类型与上游账单号都落库，
// 月度成本直接按流水归集，可逐笔核对、可回填历史（首次全量、之后按 ID 断点增量）。
//
// 幂等：以 (provider_id, kind, external_id) 唯一，重复同步只更新金额/类型/时间，
// 不会产生重复行；因此「每天同步一次」与「全量回填」是同一个写入口。
type UpstreamLedgerEntry struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	ProviderID uint64 `gorm:"column:provider_id;not null;uniqueIndex:uk_up_ledger_ext,priority:1;index:idx_up_ledger_provider_time,priority:1"`
	// Kind consume=消费（余额支付）/ topup=充值入账。
	Kind string `gorm:"column:kind;size:16;not null;uniqueIndex:uk_up_ledger_ext,priority:2"`
	// ExternalID 上游记录 ID（字符串化；上游自增，可用于增量同步的断点）。
	ExternalID string    `gorm:"column:external_id;size:64;not null;uniqueIndex:uk_up_ledger_ext,priority:3"`
	OccurredAt time.Time `gorm:"column:occurred_at;not null;index:idx_up_ledger_provider_time,priority:2"`
	// Amount 金额（正数；消费为正、充值也为正，方向由 Kind 决定）。
	Amount float64 `gorm:"type:decimal(15,2);not null;default:0"`
	// RefundAmount 该笔消费对应的退款金额（正数=已冲回），消费净额 = Amount − RefundAmount。
	RefundAmount float64 `gorm:"column:refund_amount;type:decimal(15,2);not null;default:0"`
	// Category 上游类型原文（订购产品 / 续费 / 用户充值 / 人工入账），用于分组展示。
	Category    string    `gorm:"size:64;not null;default:''"`
	RefNo       string    `gorm:"column:ref_no;size:64;not null;default:''"`
	Description string    `gorm:"size:255;not null;default:''"`
	Currency    string    `gorm:"size:8;not null;default:'CNY'"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名。
func (UpstreamLedgerEntry) TableName() string { return "upstream_ledger_entries" }
