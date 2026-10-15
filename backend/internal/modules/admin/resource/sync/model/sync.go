// Package model 提供资源同步子域的数据模型。
package model

import "time"

// SyncTask 同步任务
type SyncTask struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	ProviderID   uint64     `gorm:"column:provider_id;not null;index"`
	TaskType     string     `gorm:"column:task_type;size:50;not null"`
	Status       string     `gorm:"size:20;default:pending;index"`
	TotalCount   int        `gorm:"column:total_count;default:0"`
	SuccessCount int        `gorm:"column:success_count;default:0"`
	ErrorMessage string     `gorm:"column:error_message;type:text"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SyncTask) TableName() string {
	return "sync_tasks"
}

// SyncLog 同步日志
type SyncLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	TaskID       uint64    `gorm:"column:task_id;index"`
	ProviderID   uint64    `gorm:"column:provider_id;not null;index"`
	SyncType     string    `gorm:"column:sync_type;size:50;not null"`
	Status       string    `gorm:"size:20;default:success;index"`
	TotalCount   int       `gorm:"column:total_count;default:0"`
	SuccessCount int       `gorm:"column:success_count;default:0"`
	ErrorMessage string    `gorm:"column:error_message;type:text"`
	Details      string    `gorm:"type:text"` // JSON 以 text 存储
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SyncLog) TableName() string {
	return "sync_logs"
}

// 实例链路判据（P1/T1.2，D6）：单一字段区分自营与上游，禁止混用。
const (
	// SourceModeSelf 经我方订单/后台开通的自营链路实例。
	SourceModeSelf = "self"
	// SourceModeUpstream 由上游渠道同步发现的上游链路实例。
	SourceModeUpstream = "upstream"
)

// Instance 统一实例主数据
type Instance struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	InstanceID  string `gorm:"column:instance_id;size:64;uniqueIndex;not null"`
	ProviderID  uint64 `gorm:"column:provider_id;not null;index"`
	UserID      uint64 `gorm:"column:user_id;not null;index"`
	Name        string `gorm:"size:100;not null"`
	CPU         int    `gorm:"not null"`
	Memory      int    `gorm:"not null"`
	Disk        int    `gorm:"not null"`
	DiskType    string `gorm:"column:disk_type;size:20"`
	Bandwidth   int    `gorm:"default:0"`
	OS          string `gorm:"size:50"`
	Region      string `gorm:"size:50"`
	Zone        string `gorm:"size:50"`
	Status      string `gorm:"size:30;default:creating;index"`
	PrivateIP   string `gorm:"column:private_ip;size:15"`
	PublicIP    string `gorm:"column:public_ip;size:15"`
	RawData     string `gorm:"column:raw_data;type:text"` // JSON 以 text 存储
	BillingMode string `gorm:"column:billing_mode;size:20;default:hourly"`
	// ActorUserID 开通该实例的真实操作人（子账号下单时为主账号名下的子账号，P4-09）
	ActorUserID uint64 `gorm:"column:actor_user_id;index"`
	// OrderID 开通该实例的来源订单；存量数据为 0（见 61 实施计划 §4.1）。
	OrderID uint64 `gorm:"column:order_id;index"`
	// Remark 管理员内部备注。注意：同步流程（UpsertInstances）不得覆盖此列。
	Remark string `gorm:"size:255"`
	// CustomerRemark 客户备注（下单填写或客户自助修改），与 Remark 严格分开：
	// 前者客户可见、后者仅管理端内部可见（迁移 099）。同步流程同样不得覆盖。
	CustomerRemark string `gorm:"column:customer_remark;size:255"`
	// CredentialPassword 实例登录密码的**密文**（enc:v1: AES-256-GCM，迁移 100）。
	// 开通时密码由本系统生成，此前只发给上游未留存，导致「查看密码」永远取不到。
	// 与 raw_data 的区别：raw_data 会被每次同步整体覆盖，密文不能被同步触碰。
	CredentialPassword string `gorm:"column:credential_password;size:512"`
	// LastSyncedAt 最近一次单实例回源上游的时间。
	LastSyncedAt *time.Time `gorm:"column:last_synced_at"`
	// ---- 双链路语义列（P1/T1.2）----
	// SourceMode 链路判据：self=经我方订单/后台开通，upstream=由上游同步发现。
	SourceMode string `gorm:"column:source_mode;size:16"`
	// SellProductID 自营链路售出商品（products.id）；上游链路为空。
	SellProductID uint64 `gorm:"column:sell_product_id"`
	// UpstreamProductID 上游链路资源商品（resource_products.id）；自营链路为空。
	UpstreamProductID uint64 `gorm:"column:upstream_product_id"`
	// ProviderInstanceID 上游/平台侧的实例号（= instance_id，独立列便于多平台对接）。
	ProviderInstanceID string `gorm:"column:provider_instance_id;size:128"`
	// UpstreamOrderID 上游订单号（链路 A 续费/对账用）。
	UpstreamOrderID string `gorm:"column:upstream_order_id;size:128"`
	// LifecycleStage 生命周期阶段（T5.4 推进器使用）。
	LifecycleStage string `gorm:"column:lifecycle_stage;size:24"`
	// EnforceAttempts 到期阶段强制执行连续失败次数（doc61 §8.4 退避）。
	// 推进器动作失败时自增，成功后归零；用于指数退避与「持续失败」告警。
	EnforceAttempts int `gorm:"column:enforce_attempts;not null;default:0"`
	// EnforceNextAt 下一次允许重试的时间（退避闸门）；为空表示可立即处理。
	EnforceNextAt *time.Time `gorm:"column:enforce_next_at"`
	// LastEnforceError 最近一次强制执行失败原因（截断），供运维在列表/详情直接看到。
	LastEnforceError string `gorm:"column:last_enforce_error;size:500"`
	// RenewalPrice 绑定的续费单价（元/周期），NULL 表示未绑定、按商品定价实时算。
	// 后台代客下单可与首次价分开指定（如首次 30、续费 18），绑定后长期有效：
	// 续费算价优先取本列，商品事后调价不影响这台机器（对单机的价格承诺）。
	RenewalPrice *float64 `gorm:"column:renewal_price;type:decimal(15,2)"`
	// InstanceType 产品类型（迁移 108）：实例属于哪条产品线（cloud_host/cdn/physical/…）。
	//
	// 为什么单独存一列而不是每次联表 products.product_type：实例列表要按产品线筛选并
	// 给出各类型计数，联表在「上游同步发现、没有本地商品」的行上会筛漏（那些行
	// sell_product_id 为空，join 不到商品）。列值在开通/占位时从商品带上；
	// 商品事后改类型不回改已有实例（机器已按当时那条产品线交付并计价）。
	//
	// 取值见 pkg/model/product_kind.go；未知/空值统一归一为 other（不置空，
	// 否则该实例在任何类型标签页里都找不到）。
	InstanceType string `gorm:"column:instance_type;size:32;index"`
	// ---- 过渡态追踪（迁移 112）----
	//
	// 上游的电源/维护指令是**异步**的：接口只回"已受理"，真正生效要等十几秒到几分钟。
	// 这段窗口里 status 还是旧值，界面看不出任何变化 —— 运营以为没生效就反复点，
	// 同一台机器堆了多次重启（用户实测反馈）。落库记录"这台机器上有个操作在进行"，
	// 才能做到刷新后仍显示「重启中」，且新指令被拦截。
	//
	// TransitionState 取值见 pkgmodel.InstanceTransition*；空 = 无进行中操作。
	TransitionState     string     `gorm:"column:transition_state;size:24"`
	TransitionStartedAt *time.Time `gorm:"column:transition_started_at"`
	// TransitionUntil 过渡窗口截止时间。到此仍未回源确认则视为"结果未知"：
	// 不再拦截新指令（避免永久卡死），但界面会提示上次结果未确认。
	TransitionUntil *time.Time `gorm:"column:transition_until"`
	CreatedAt       time.Time  `gorm:"autoCreateTime"`
	ExpireAt        *time.Time `gorm:"column:expire_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Instance) TableName() string {
	return "instances"
}
