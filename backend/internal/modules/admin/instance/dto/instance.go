// Package dto 定义实例运维管理台的接口出入参。
package dto

// 到期状态筛选项。
const (
	ExpireStateAll      = "all"      // 全部
	ExpireStateExpiring = "expiring" // 临期（expire_within_days 天内）
	ExpireStateExpired  = "expired"  // 已过期
	ExpireStateNone     = "none"     // 未设置到期时间
	ExpireStateNormal   = "normal"   // 到期时间充裕（列表项展示用，非筛选项）
)

// ListMeta 通用分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListQuery 跨用户实例列表查询。
type ListQuery struct {
	Keyword          string `form:"keyword" json:"keyword"`                       // 实例标识 / 实例名
	UserKeyword      string `form:"user_keyword" json:"user_keyword"`             // 用户名 / 邮箱 / 手机号
	UserID           uint64 `form:"user_id" json:"user_id"`                       // 精确用户
	ProviderID       uint64 `form:"provider_id" json:"provider_id"`               // 服务商
	Status           string `form:"status" json:"status"`                         // 服务状态
	SourceMode       string `form:"source_mode" json:"source_mode"`               // self/upstream（链路筛选，T1.3）
	InstanceType     string `form:"instance_type" json:"instance_type"`           // 产品类型筛选（cloud_host/cdn/…，空=全部）
	ExpireState      string `form:"expire_state" json:"expire_state"`             // all/expiring/expired/none
	ExpireWithinDays int    `form:"expire_within_days" json:"expire_within_days"` // 临期天数，默认 7
	Page             int    `form:"page" json:"page"`
	PageSize         int    `form:"page_size" json:"page_size"`
}

// InstanceItem 实例列表项（实例 + 归属用户 + 服务商）。
type InstanceItem struct {
	ID           uint64 `json:"id"`
	InstanceID   string `json:"instance_id"`
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	UserID       uint64 `json:"user_id"`
	Username     string `json:"username"`
	UserEmail    string `json:"user_email"`
	UserPhone    string `json:"user_phone"`
	OrderID      uint64 `json:"order_id"`
	// 双链路语义（P1/T1.4）：来源判据与两条链路各自的产品引用。
	SourceMode         string `json:"source_mode"`
	SellProductID      uint64 `json:"sell_product_id"`
	UpstreamProductID  uint64 `json:"upstream_product_id"`
	ProviderInstanceID string `json:"provider_instance_id"`
	// Pending 待开通（尚未在上游开通的占位行）：列表显示「待开通」并只开放「重新开通」。
	Pending bool `json:"pending"`
	// Resale 是否**转售链路**（source_mode=upstream）：实例对着一个上游资源商品，
	// 因此存在"上游实例号 / 上游镜像 / 上游各周期价"这些概念，也才有换号与换绑需求。
	//
	// 自营链路（self）为 false：机器由本平台开通，实例号由本系统管理 ——
	// 页面不应出现「上游实例号」「重新关联」「更换对接商品」这类措辞与入口
	//（用户实测反馈："虽然是我自己的，但它这里会显示上游实例号，这个一看就是有问题"）。
	//
	// 前端一律用本字段决定措辞与入口，不要自行拼 source_mode 字符串判等：
	// 历史行可能有空值，判等容易漏（后端已把空值按更安全的语义归一）。
	Resale         bool   `json:"resale"`
	LifecycleStage string `json:"lifecycle_stage"`
	// InstanceType 产品类型（cloud_host/cdn/physical/…，迁移 108）：产品线导航与
	// 「这类实例有几个」的计数依据。未知/空值已由仓储归一为 other。
	InstanceType string `json:"instance_type"`
	Name         string `json:"name"`
	CPU          int    `json:"cpu"`
	Memory       int    `json:"memory"`
	Disk         int    `json:"disk"`
	DiskType     string `json:"disk_type"`
	Bandwidth    int    `json:"bandwidth"`
	OS           string `json:"os"`
	Region       string `json:"region"`
	Zone         string `json:"zone"`
	Status       string `json:"status"`
	PowerStatus  string `json:"power_status"`
	// PowerState 机器电源维度（on/off/transitioning/unknown）—— **与账期完全无关**。
	//
	// 为什么单独给一个字段：status 是混合体（上游把 suspended 也映射成 stopped），
	// 前端只凭它分不清"手动关机"与"到期暂停"，于是关机被显示成「已暂停」
	// （用户实测反馈的根因）。电源维度只看机器通没通电；账期维度见 ServiceState。
	PowerState string `json:"power_state"`
	// ServiceState 账期/服务维度（active/expiring/expired/grace/suspended/destroyed）。
	//
	// 判据：到期时间 + 生命周期阶段，**不看电源状态** —— 关机但未到期的实例，
	// 服务维度仍是 active（服务在有效期内，只是机器没开）。
	ServiceState string `json:"service_state"`
	// Transition 进行中的操作（迁移 112）：nil = 空闲。
	// 非空时前端显示「重启中」等过渡文案 + 动画，并禁用冲突操作。
	Transition  *TransitionInfo `json:"transition,omitempty"`
	PrivateIP   string          `json:"private_ip"`
	PublicIP    string          `json:"public_ip"`
	BillingMode string          `json:"billing_mode"`
	ActorUserID uint64          `json:"actor_user_id"`
	ActorName   string          `json:"actor_name"`
	// Remark 管理员内部备注（仅管理端可见）；CustomerRemark 是客户下单填写或自助修改的
	// 备注（客户在自己界面也看得到）。两者严格分开：混用会让内部备注泄漏给客户。
	Remark         string `json:"remark"`
	CustomerRemark string `json:"customer_remark"`
	ExpireAt       string `json:"expire_at"`
	DaysLeft       int    `json:"days_left"`
	ExpireState    string `json:"expire_state"`
	LastSyncedAt   string `json:"last_synced_at"`
	// 到期处置退避状态（doc61 §8.4）：连续失败次数、下次重试时间、最近失败原因。
	EnforceAttempts  int    `json:"enforce_attempts"`
	EnforceNextAt    string `json:"enforce_next_at"`
	LastEnforceError string `json:"last_enforce_error"`
	// RenewalPrice 绑定的续费单价（元/周期）；nil = 未绑定，按商品实时算价。
	// 绑定价压过商品调价与折扣（续费时按它计费），详情页与调整对话框据此回显。
	RenewalPrice *float64 `json:"renewal_price"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// TransitionInfo 进行中的操作（迁移 112）。
//
// 上游下发是异步的：接口回"已受理"后机器还要执行几十秒到几分钟。这段窗口里
// 本地状态不会变，界面无从判断"到底有没有在跑" —— 于是运营反复点同一个按钮。
// 本结构把"正在做什么、从什么时候开始"如实下发给前端。
type TransitionInfo struct {
	// State 落库值：starting/stopping/rebooting/reinstalling/resetting_password/…
	State string `json:"state"`
	// Label 展示文案（"重启中"）。
	Label string `json:"label"`
	// Verb 动作名（"重启"、"重装系统"），用于提示语。
	Verb string `json:"verb"`
	// StartedAt 下发时间（RFC3339，空 = 未知）。
	StartedAt string `json:"started_at"`
	// ExpiresAt 过渡窗口截止时间：超过后视为结果未知（界面提示可重新操作）。
	ExpiresAt string `json:"expires_at"`
	// Overdue 是否已超过窗口仍未确认。
	//
	// 语义不是"失败了"，而是"结果未知"：上游可能还在跑、也可能指令丢了。
	// 界面据此从"进行中（禁止操作）"切换为"结果未确认（可重试，但提示先同步）"。
	Overdue bool `json:"overdue"`
}

// ListResponse 实例列表响应。
type ListResponse struct {
	Items []InstanceItem `json:"items"`
	Meta  ListMeta       `json:"meta"`
}

// StatsQuery 概览统计查询。
type StatsQuery struct {
	ExpireWithinDays int `form:"expire_within_days" json:"expire_within_days"`
}

// StatsResponse 概览统计（全局口径，不受列表筛选影响）。
type StatsResponse struct {
	Total    int64 `json:"total"`
	Running  int64 `json:"running"`
	Stopped  int64 `json:"stopped"`
	Creating int64 `json:"creating"`
	Error    int64 `json:"error"`
	Expiring int64 `json:"expiring"`
	Expired  int64 `json:"expired"`
}

// InstanceTypeOption 产品线导航的一项（实例运维台顶部的类型标签）。
//
// 为什么计数放在这一份响应里而不是让前端逐类型发请求：导航要一次渲染出
// 「云主机 12 · CDN 0 · 物理机 0」的完整形态，N 个类型就是 N 次往返；
// 一次 GROUP BY 就能给全，且计数与列表用的是同一批行（不会出现
// 标签显示 3 条、点进去 2 条的口径差）。
type InstanceTypeOption struct {
	// Value 类型值（cloud_host/cdn/…）；"all" 是虚拟的「全部实例」项。
	Value string `json:"value"`
	Label string `json:"label"`
	// Count 该类型的实例数（按 status 过滤后的口径，与列表一致）。
	Count int64 `json:"count"`
	// Reserved 是否为预埋类型（登记在册但当前**没有任何实例**）。
	//
	// 前端默认不显示预留类型（避免一排永远为 0 的空标签把导航撑长），
	// 但保留该字段供「全部类型」面板或调试查看 —— 预埋的意义是将来加产品线
	// 时无需改后端就能出现，而不是现在就把空标签堆满。
	Reserved bool `json:"reserved"`
}

// InstanceTypeSummary 产品线导航数据（类型 + 计数 + 总量）。
type InstanceTypeSummary struct {
	// Types 按登记顺序排列的类型项（不含 "all"）。
	Types []InstanceTypeOption `json:"types"`
	// Total 全部实例数（"全部"标签的计数，含未登记类型的 other 行）。
	Total int64 `json:"total"`
}

// Capabilities 上游对当前实例支持的操作能力（前端据此置灰按钮）。
//
// 与 upstream.CapabilityDescriptor 的关系：描述符是**渠道级**声明（"这个平台能做什么"），
// 这里是**实例级**结果（当前实例的渠道实际能做什么），由类型断言 + 描述符共同得出。
type Capabilities struct {
	Power     bool `json:"power"`
	Console   bool `json:"console"`
	Resize    bool `json:"resize"`
	Destroy   bool `json:"destroy"`
	Reinstall bool `json:"reinstall"`
	// Suspend 是否支持暂停/恢复（T5.5）：平台暂停态或退化为关机/开机皆算支持。
	Suspend bool `json:"suspend"`
	// 维护类（实测平台接口存在，见 dcimcloud/provider.go 头部映射）。
	ResetPassword bool `json:"reset_password"` // 重置登录密码
	Rescue        bool `json:"rescue"`         // 救援系统（进/出）
	Snapshot      bool `json:"snapshot"`       // 磁盘快照 / 备份
	Bandwidth     bool `json:"bandwidth"`      // 带宽直改
	AddIP         bool `json:"add_ip"`         // 增加 IP / IPv6
	AttachDisk    bool `json:"attach_disk"`    // 挂载数据盘
	// NatAccess NAT 访问地址可读（doc122）：渠道声明了 NatReader 能力时为真，
	// 前端据此决定是否请求/展示 NAT 地址区块。
	NatAccess bool `json:"nat_access"`
}

// DetailInfo 实例详情。
//
// 除实例本体与能力位外，还组装三块「运营一屏看全」的信息：
//   - Order/Finance：来源订单与财务口径（首付、续费价、付款周期、订单状态）；
//   - Product/Upstream：售出商品与（对接商品时的）上游商品价格；
//   - Config/Credential：生效配置项与登录连接信息。
//
// 刻意拆成子结构而非平铺：平铺后再加字段会让这个响应无人敢改，分组后每组可独立演进。
type DetailInfo struct {
	InstanceItem
	Capabilities    Capabilities     `json:"capabilities"`
	CapabilityError string           `json:"capability_error"`
	Order           *SourceOrderInfo `json:"order,omitempty"`
	Product         *ProductInfo     `json:"product,omitempty"`
	Upstream        *UpstreamInfo    `json:"upstream,omitempty"`
	Configs         []ConfigItem     `json:"configs"`
	Credential      *CredentialInfo  `json:"credential,omitempty"`
	// ProvisionStatus/ProvisionError 待开通实例的履约状态（provision_tasks）：
	// pending/running/success/failed/manual；manual=重试无效需人工（如上游余额不足）。
	ProvisionStatus string `json:"provision_status,omitempty"`
	ProvisionError  string `json:"provision_error,omitempty"`
	// NatAccess NAT 机型的访问地址（doc122）。NAT 机型没有独立 IP，客户只能经
	// 节点转发地址访问；独立 IP 机型该字段为 nil，前端不渲染该区块。
	NatAccess *NatAccessInfo `json:"nat_access,omitempty"`
}

// NatAccessInfo NAT 机型的访问地址（实例详情下发）。
//
// 为什么要单独下发而不是让前端拼 public_ip：NAT 机型根本没有独立 IP，
// 连接信息（转发 IP:端口 / 建站域名）要额外请求面板才拿得到。
type NatAccessInfo struct {
	// IsNat 是否走 NAT 链路。判断依据是实例机型而非"是否有转发规则"：
	// 刚开通还没建规则的 NAT 机器同样是 NAT 机型，不能因为列表为空就当独立 IP 展示。
	IsNat bool `json:"is_nat"`
	// Forwards 端口转发地址，已拼成可直接展示/复制给客户的 "IP:端口"。
	Forwards []string `json:"forwards"`
	// Domains 共享建站域名。
	Domains []string `json:"domains"`
	// Error 读取失败原因。与"没有转发规则"区分：读失败时前端提示可重试，
	// 而不是显示成"该实例没有 NAT 配置"（后者会让客户以为机器没配好）。
	Error string `json:"error,omitempty"`
}

// SourceOrderInfo 实例的来源订单财务口径（orders 表）。
//
// 「首付」取来源订单的实付（first_paid_amount）；「续费价」取商品当前周期价
// （product_prices），二者含义不同：前者是历史成交，后者是下次续费应付。
type SourceOrderInfo struct {
	ID            uint64  `json:"id"`
	OrderNo       string  `json:"order_no"`
	Status        string  `json:"status"`
	ProductID     uint64  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	Cycle         string  `json:"cycle"`
	CycleName     string  `json:"cycle_name"`
	Quantity      int     `json:"quantity"`
	TotalAmount   float64 `json:"total_amount"`
	PaidAmount    float64 `json:"paid_amount"`
	DiscountAmt   float64 `json:"discount_amount"`
	FirstPaidAmt  float64 `json:"first_paid_amount"`
	PayMethod     string  `json:"pay_method"`
	PayTime       string  `json:"pay_time"`
	OrderedAt     string  `json:"ordered_at"`
	RenewalID     uint64  `json:"renewal_id"`
	RenewUnitCost float64 `json:"renew_unit_cost"`
	RenewCycle    string  `json:"renew_cycle"`
	// Remark 管理员在订单上写的备注（管理端内部可见）。
	Remark string `json:"remark"`
	// CustomerRemark 客户下单时填写的备注（客户可见，随履约带入实例）。
	CustomerRemark string `json:"customer_remark"`
	// RenewAmount 按实例当前付款周期与数量算出的下次续费金额（售价口径）。
	RenewAmount float64 `json:"renew_amount"`
	// RenewBound RenewAmount 是否来自实例绑定价（instances.renewal_price）而非商品周期价。
	// 前端据此标注"已绑定谈定价"，并可一键取消绑定。
	RenewBound bool `json:"renew_bound"`
	// RenewCost 下次续费的上游成本（成本口径；自营/未绑定为 0）。
	RenewCost float64 `json:"renew_cost"`
	// UpstreamRenewAmount 上游续费金额（对接商品；来自上游商品各周期价，未取到为 0）。
	UpstreamRenewAmount float64 `json:"upstream_renew_amount"`
	// UpstreamRenewCycle 上游续费金额对应的周期（与实例 billing_mode 同口径）。
	UpstreamRenewCycle string `json:"upstream_renew_cycle"`
}

// ProductInfo 售出商品（我方 products 表）与对接关系。
type ProductInfo struct {
	ID               uint64 `json:"id"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	SourceMode       string `json:"source_mode"`
	SourceProviderID uint64 `json:"source_provider_id"`
	SourceProvider   string `json:"source_provider_name"`
	ProviderType     string `json:"provider_type"`
	// SourceProductID 对接的上游**资源商品镜像** id（本地 resource_products.id），
	// 不是上游侧的 pid —— 上游 pid 见 UpstreamInfo.UpstreamID。两者不可混用：
	// 前者是本地镜像行的主键（重新导入后会变号），后者才是上游的稳定标识。
	SourceProductID uint64  `json:"source_product_id"`
	PriceModel      string  `json:"price_model"`
	Price           float64 `json:"price"`
	CostPrice       float64 `json:"cost_price"`
	MarkupType      string  `json:"upstream_markup_type"`
	MarkupValue     float64 `json:"upstream_markup_value"`
	Status          int     `json:"status"`
	// Deleted 商品行已不存在（重新导入/清理后常见）：此时名称取订单快照，
	// 前端据此显示「商品已删除」，而不是把该实例当成"非我方商品开通"。
	Deleted bool `json:"deleted"`
}

// UpstreamInfo 对接商品的上游信息（source_mode=upstream 时有值）。
//
// 价格全部来自上游镜像快照（resource_products.raw_specs）：
// ListCycles=上游标价、CostCycles=我方采购价（已含代理折扣）。
type UpstreamInfo struct {
	ResourceProductID uint64 `json:"resource_product_id"`
	// ProviderID/ProviderName 该上游资源商品所属渠道；与实例的 provider_id 不一致时
	// ProviderMismatch=true（历史数据/重新导入会留下这种错配，页面需显式提示）。
	ProviderID       uint64             `json:"provider_id"`
	ProviderName     string             `json:"provider_name"`
	ProviderMismatch bool               `json:"provider_mismatch"`
	UpstreamID       string             `json:"upstream_id"`
	Name             string             `json:"name"`
	GroupID          int64              `json:"group_id"`
	GroupName        string             `json:"group_name"`
	ListPrice        float64            `json:"list_price"`
	CostPrice        float64            `json:"cost_price"`
	DiscountDesc     string             `json:"discount_desc"`
	PayType          string             `json:"pay_type"`
	Renewable        bool               `json:"renewable"`
	ListCycles       map[string]float64 `json:"list_cycles"`
	CostCycles       map[string]float64 `json:"cost_cycles"`
}

// ConfigItem 实例生效配置项（商品配置项 + 实例实际取值）。
//
// Source 说明取值来源，运营据此判断"这个值改得动改不动"：
//
//	instance  实例表字段（CPU/内存/磁盘/带宽/系统/区域）
//	order     下单时客户选配（orders.config_options）
//	product   商品默认（product_config_options）
//	upstream  上游商品固定（对接商品锁定项）
type ConfigItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
	Locked bool   `json:"locked"`
	Unit   string `json:"unit"`
}

// CredentialInfo 实例登录连接信息（管理员排障用）。
//
// 明文密码**不随详情返回**：详情会被高频加载、被日志与浏览器缓存，明文一旦
// 进入响应就等于长期泄露。这里只给用户名/端口/是否有密码，明文走单独的
// reveal 接口（需 instance:password 权限且写操作流水）。
type CredentialInfo struct {
	Username    string `json:"username"`
	Port        string `json:"port"`
	HasPassword bool   `json:"has_password"`
	// PasswordHint 密码来源说明（如「重装后可查看」「上游面板可见」）。
	PasswordHint string `json:"password_hint"`
	// PanelPassword 是否另有一份面板/IPMI 密码（部分平台单独存放）。
	HasPanelPassword bool `json:"has_panel_password"`
}

// CredentialRevealResult 明文凭据（仅 reveal 接口返回，调用方必须提示立即使用勿留存）。
type CredentialRevealResult struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Port     string `json:"port"`
}

// RelinkRequest 按上游实例号重新关联（上游换机/换号后拉取新机器信息）。
//
// 场景（实测需求）：上游把我们的机器换了一台（或重装后上游侧换了实例号），
// 本地留的还是旧号，此后所有上游动作（电源/控制台/续费/销毁）都打在已不存在的机器上。
// 运营需要"填入上游新号 → 拉取新机器信息"，把本地实例指向真正在跑的那台。
//
// 三点安全约束（缺一不可）：
//   - 目标号必须**在上游真实存在**（GetInstance 探活），凭空写一个号等于把实例指向虚空；
//   - 目标号不得已被**另一台本地实例**占用（否则两台实例抢同一个上游号）；
//   - 必须显式确认（ConfirmMark 为当前实例标识），与销毁同口径 —— 改上游号会改变
//     此后所有上游动作的作用对象，属于高影响操作。
type RelinkRequest struct {
	// UpstreamInstanceID 目标上游实例号（必填）。
	UpstreamInstanceID string `json:"upstream_instance_id" binding:"required"`
	// ConfirmMark 当前实例标识（instance_id）或记录 ID，必须精确匹配。
	ConfirmMark string `json:"confirm_mark" binding:"required"`
	// MigrateInstanceID 是否把本地展示号（instances.instance_id）也改成新上游号。
	// 默认 false：instance_id 是本地主键性质的存在，改它会影响引用它的历史流水展示；
	// 只有当本实例的 instance_id 与 provider_instance_id 原本一致（即"一直在用上游号"）时，
	// 才建议同步改，保持两者语义一致。
	MigrateInstanceID bool   `json:"migrate_instance_id"`
	Reason            string `json:"reason"`
}

// RelinkResult 重新关联结果（回显新旧号与从上游拉到的关键信息）。
type RelinkResult struct {
	OldProviderInstanceID string `json:"old_provider_instance_id"`
	NewProviderInstanceID string `json:"new_provider_instance_id"`
	InstanceID            string `json:"instance_id"`
	Status                string `json:"status"`
	PublicIP              string `json:"public_ip"`
	PrivateIP             string `json:"private_ip"`
	Name                  string `json:"name"`
	OS                    string `json:"os"`
	Region                string `json:"region"`
	// InstanceIDMigrated 本地展示号是否一并改成了新上游号。
	InstanceIDMigrated bool `json:"instance_id_migrated"`
}

// RebindProductRequest 更换实例绑定的本地售出商品 + 上游对接商品。
//
// 场景（实测需求）：上游把我们卖的五区商品换成了六区的商品，本地商品却仍是五区那条 ——
// 此后续费会按五区价算（与上游账单对不上），推给上游的配置也还是五区的。
// 运营需要"对着上游实际在用的商品，把本地的也改过来"。
//
// 三个可独立调整的绑定：
//   - SellProductID：本地售出商品（决定续费算价与展示口径）；
//   - UpstreamProductID：本地镜像行（resource_products.id，决定上游价与配置来源）；
//   - RenewalPrice：顺带把续费价钉到新口径（不传即不动）。
type RebindProductRequest struct {
	// SellProductID 目标本地售出商品（products.id）；0 表示不改。
	SellProductID uint64 `json:"sell_product_id"`
	// UpstreamProductID 目标本地镜像（resource_products.id）；nil 表示不改，
	// 指向 0 表示清空（实例不再关联任何上游镜像）。
	UpstreamProductID *uint64 `json:"upstream_product_id"`
	// RenewalPrice 顺带更新绑定续费价；nil 表示不改，-1 表示清除绑定。
	RenewalPrice *float64 `json:"renewal_price"`
	Reason       string   `json:"reason"`
}

// RebindProductResult 更换结果（回显新旧绑定，便于运营核对）。
type RebindProductResult struct {
	OldSellProductID     uint64 `json:"old_sell_product_id"`
	NewSellProductID     uint64 `json:"new_sell_product_id"`
	OldUpstreamProductID uint64 `json:"old_upstream_product_id"`
	NewUpstreamProductID uint64 `json:"new_upstream_product_id"`
	// SellProductName / UpstreamProductName 新绑定的名称（回显用，读不到为空）。
	SellProductName     string `json:"sell_product_name"`
	UpstreamProductName string `json:"upstream_product_name"`
	// UpstreamChanged 上游镜像确实变了（前端据此提示"上游价与配置已切换"）。
	UpstreamChanged bool `json:"upstream_changed"`
}

// RebindProductCandidate 更换对接商品的可选项（每个下拉一行）。
type RebindProductCandidate struct {
	// ID 提交时回传的值：本地商品为 products.id，上游镜像为 resource_products.id。
	ID uint64 `json:"id"`
	// Name 展示名。
	Name string `json:"name"`
	// Code 商品编码（本地商品有；上游镜像为空）。
	Code string `json:"code"`
	// ProviderName 所属渠道名（上游镜像项展示用，避免跨渠道误选）。
	ProviderName string `json:"provider_name"`
	// Price / CostPrice 该候选的价格与成本（可选，用于比价）。
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	// Cycles 已配置价格的周期档（本地商品展示用）。
	Cycles []string `json:"cycles"`
	// Current 是否是当前绑定项（前端禁用/标注"当前"）。
	Current bool `json:"current"`
	// Disabled 不可选（如跨渠道、已下架），DisabledReason 给出原因。
	Disabled       bool   `json:"disabled"`
	DisabledReason string `json:"disabled_reason"`
}

// RebindProductCandidates 更换对接商品的可选清单。
//
// 此前前端要求运营手填商品 ID —— 运营得先去商品列表查号再回来填，跨渠道/已下架
// 的商品只有提交后才会被后端拒绝（用户实测反馈"还要在这里输入 ID 查找，有点麻烦"）。
// 这里把候选与可用性判定一起给出，前端直接渲染成选框。
type RebindProductCandidates struct {
	// SellProducts 本地售出商品候选项（仅含未删除的）。
	SellProducts []RebindProductCandidate `json:"sell_products"`
	// UpstreamProducts 上游镜像候选项：**仅本实例同渠道**的（跨渠道选中必被拒）。
	UpstreamProducts []RebindProductCandidate `json:"upstream_products"`
	// ProviderID 本实例所属渠道（前端说明用）。
	ProviderID   uint64 `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	// Hint 供运营阅读的说明（候选被过滤掉时的原因）。
	Hint string `json:"hint"`
}

// ReinstallImage 重装可选的一个镜像。
type ReinstallImage struct {
	// Value 下发给上游的镜像值：魔方云为镜像 ID；魔方财务为**上游配置子项 ID**。
	Value string `json:"value"`
	// Label 展示名（已清洗上游的 "id|名称^完整名" 形态）。
	Label string `json:"label"`
	// Group 分组名（部分平台按系列分组，如 "CentOS"）。
	Group string `json:"group"`
	// Status 可用状态（上游给出时展示，如「可用/维护中」）。
	Status string `json:"status"`
}

// ReinstallImages 重装镜像列表（重装对话框的下拉数据源）。
//
// 为什么需要单独接口：重装镜像的来源随链路不同 ——
//   - 自营平台（魔方云）：渠道的镜像目录（/resources 的 images）；
//   - 对接上游（魔方财务）：os 是**上游商品的配置子项 ID**，没有独立镜像目录接口。
//
// 此前前端只读渠道镜像目录，对接链路必然拿到空列表（实测：重装对话框镜像为空），
// 运营只能手填 ID —— 而 ID 从哪来没有任何提示。这里统一成一处数据源。
type ReinstallImages struct {
	Images []ReinstallImage `json:"images"`
	// Source 列表来源：platform（渠道镜像目录）/ product（上游商品配置子项）/ none。
	// 前端据此给不同提示（"从上游商品配置读取" vs "手填"）。
	Source string `json:"source"`
	// Hint 供运营阅读的说明（读不到时的原因与替代做法）。
	Hint string `json:"hint"`
}

// ReinstallImagesSource 镜像列表来源取值。
const (
	ReinstallSourcePlatform = "platform"
	ReinstallSourceProduct  = "product"
	ReinstallSourceNone     = "none"
)

// AdjustRequest 实例账期/计费周期调整请求（管理端自定义）。
//
// 与「续费」的语义差别：续费是**上账**（付钱买一段时间，向上游下单）；
// 调整是**改口径**（把到期时间或付款周期改成运维认为正确的值，不产生账单）。
// 因此本接口不收费、不生成订单，但必须留审计，且对接商品需要显式 force。
type AdjustRequest struct {
	// ExpireAt 目标到期时间（RFC3339 或 "2006-01-02 15:04:05"）；空表示不改。
	ExpireAt string `json:"expire_at"`
	// ClearExpire 是否把到期时间清空（一次性/买断商品）。
	ClearExpire bool `json:"clear_expire"`
	// BillingMode 目标付款周期（monthly/quarterly/annually/hourly/daily/onetime）；空表示不改。
	BillingMode string `json:"billing_mode"`
	// RenewalPrice 绑定续费单价（元/周期）。>0 时写入 instances.renewal_price，
	// 0 表示本次不改动，负值（-1）表示清除绑定（回到按商品实时算价）。
	//
	// 用「负值 = 清除」而不是指针：JSON 里缺字段与显式 null 都不好区分，
	// 而运营确实需要"取消我上次钉死的价"这个动作。
	RenewalPrice *float64 `json:"renewal_price"`
	// Force 对接商品（source_mode=upstream）下强制本地调整。
	// 不加 force 时后端拒绝：上游账期才是权威，本地改了会被回源覆盖，
	// 而生命周期推进器可能按这个本地值误停服。
	Force  bool   `json:"force"`
	Reason string `json:"reason"`
}

// BillingCycleOption 账期调整对话框里的一个可选项。
//
// 只列**上游确实支持**的周期（读渠道能力描述符 BillingCycles）：
// 前端此前硬编码了 9 档（含小时/按日/两年/三年），选一个上游不卖的周期
// 会得到一个必然失败的下发请求 —— 账单周期与上游对不上，钱按错的周期扣。
type BillingCycleOption struct {
	Cycle string `json:"cycle"`
	Name  string `json:"name"`
	// Upstream 该周期是否在**上游**支持（自营实例此项恒为 true：周期由本系统管理）。
	Upstream bool `json:"upstream"`
	// UpstreamList/Cost 上游该周期的标价/采购价（对接商品；上游未报价为 0）。
	UpstreamList float64 `json:"upstream_list"`
	UpstreamCost float64 `json:"upstream_cost"`
	// Sale/Cost 我方商品该周期的售价/成本（product_prices 周期矩阵；未配置为 0）。
	Sale float64 `json:"sale"`
	Cost float64 `json:"cost"`
	// HasProductPrice 该周期在本系统商品价格矩阵里是否已配置。
	HasProductPrice bool `json:"has_product_price"`
}

// AdjustContext 账期调整所需的上下文（可选周期 + 当前绑定续费价）。
//
// 单独下发而不是让前端从上游商品里猜：可选周期以**上游能力**为准（渠道能卖什么），
// 价格以**上游镜像快照 + 我方价格矩阵**为准，两者都不是前端能推导出来的。
type AdjustContext struct {
	// Cycles 可选周期（含上游价与我方价，供运营比价后定价）。
	Cycles []BillingCycleOption `json:"cycles"`
	// CurrentCycle 实例当前付款周期（预选值）。
	CurrentCycle string `json:"current_cycle"`
	// CurrentRenewalPrice 当前绑定的续费单价；nil 表示未绑定（按商品实时算价）。
	CurrentRenewalPrice *float64 `json:"current_renewal_price"`
	// RenewalPriceHint 绑定价的语义说明（前端直接展示，避免运营误解成"本次收费"）。
	RenewalPriceHint string `json:"renewal_price_hint"`
	// UpstreamCyclesUnavailable 未能读到上游支持周期（渠道不可用/未声明）：
	// 此时 Cycles 回落到平台全集，前端需提示"周期列表未经上游核对"。
	UpstreamCyclesUnavailable bool `json:"upstream_cycles_unavailable"`
}

// PowerRequest 电源操作请求。
type PowerRequest struct {
	Action string `json:"action" binding:"required"`
}

// VNCResult 远程控制台结果。
type VNCResult struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	External bool   `json:"external"`
}

// ReinstallRequest 重装系统请求（可换镜像；FormatDataDisk 危险需显式传 true）。
type ReinstallRequest struct {
	OS string `json:"os" binding:"required"`
	// Port 重装后的自定义端口（>0 生效；SSH/RDP）。
	Port int `json:"port"`
	// FormatDataDisk 是否同时格式化数据盘（数据将丢失，默认 false）。
	FormatDataDisk bool `json:"format_data_disk"`
	// SystemDiskSize 目标系统盘大小（>0 生效）。
	SystemDiskSize int    `json:"system_disk_size"`
	Reason         string `json:"reason"`
}

// ReinstallResult 重装结果：平台为 新系统 生成的初始凭据。
//
// 只在此处一次性回传，不落库（本库 instances 表没有凭据列，开通时也不落盘）；
// 前端必须提示运营/客户立即保存，否则只能去平台面板重置。
type ReinstallResult struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResetPasswordRequest 重置实例登录密码请求。
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// RescueRequest 进入救援系统请求。
type RescueRequest struct {
	// System 救援系统类型（平台口径：魔方云 1/2）。
	System int `json:"system" binding:"required"`
	// TempPassword 救援系统的临时密码。
	TempPassword string `json:"temp_password" binding:"required"`
}

// SnapshotCreateRequest 创建快照/备份请求。
type SnapshotCreateRequest struct {
	// Type snap（快照）| backup（备份）；空视为 snap。
	Type string `json:"type"`
	// Name 快照名称；空由适配器生成。
	Name string `json:"name"`
	// DiskID 目标磁盘 ID（平台侧）；空则由服务层取实例的系统盘。
	DiskID string `json:"disk_id"`
}

// SnapshotRestoreRequest 用快照/备份恢复请求（高危：会覆盖当前系统盘）。
type SnapshotRestoreRequest struct {
	// SnapshotID 快照 ID。
	SnapshotID string `json:"snapshot_id" binding:"required"`
	// ConfirmMark 二次确认（须等于实例标识或记录 ID）。
	ConfirmMark string `json:"confirm_mark" binding:"required"`
}

// SnapshotInfo 快照/备份条目（下发给前端展示）。
type SnapshotInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	Status     int    `json:"status"`
	DiskID     string `json:"disk_id"`
	DiskName   string `json:"disk_name"`
	CreateTime string `json:"create_time"`
	Remarks    string `json:"remarks"`
}

// BandwidthRequest 带宽直改请求（Mbps；0 表示不改该方向）。
type BandwidthRequest struct {
	InBw   int    `json:"in_bw"`
	OutBw  int    `json:"out_bw"`
	Reason string `json:"reason"`
}

// AddIPRequest 增加 IP / IPv6 请求。
type AddIPRequest struct {
	// Version 4 或 6；空视为 4。
	Version int `json:"version"`
	// Num 增加数量（>0）。
	Num int `json:"num" binding:"required"`
	// IPGroup 平台 IP 分组 ID（可选，空由平台决定）。
	IPGroup string `json:"ip_group"`
}

// AttachDiskRequest 挂载数据盘请求。
type AttachDiskRequest struct {
	SizeGB int    `json:"size_gb" binding:"required"`
	Store  string `json:"store"`
	Reason string `json:"reason"`
}

// ResizeRequest 变配请求（仅提交需变更的字段）。
type ResizeRequest struct {
	CPU      int    `json:"cpu"`
	Memory   int    `json:"memory"`
	Disk     int    `json:"disk"`
	DiskType string `json:"disk_type"`
	Reason   string `json:"reason"`
}

// DestroyRequest 销毁请求；ConfirmMark 必须与实例标识完全一致才算二次确认。
type DestroyRequest struct {
	ConfirmMark string `json:"confirm_mark" binding:"required"`
	Reason      string `json:"reason"`
}

// SuspendRequest 暂停请求（T5.5；reason 透传上游暂停原因并落审计）。
type SuspendRequest struct {
	Reason string `json:"reason"`
}

// BatchActionRequest 批量运维请求（doc61 P1：列表页多选后一次性下发）。
//
// Action 取值与单实例接口一致：on/off/hard_off/reboot/hard_reboot（电源）、
// sync（回源刷新）、suspend/unsuspend（平台暂停态）。
type BatchActionRequest struct {
	Action string   `json:"action" binding:"required"`
	IDs    []uint64 `json:"ids" binding:"required"`
	Reason string   `json:"reason"`
}

// BatchActionItem 批量操作中的单实例结果。
//
// Status 三态：success（已下发）/ skipped（无需或不可能执行，如已在目标状态、上游缺能力）/
// failed（真实失败，如上游不可达）。skipped 不计入失败，避免运维被"已在运行中"这类
// 无害结果误导为事故。
type BatchActionItem struct {
	ID           uint64 `json:"id"`
	InstanceMark string `json:"instance_mark"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
}

// BatchActionResponse 批量运维响应（逐台结果，供前端按行回显）。
type BatchActionResponse struct {
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Skipped   int               `json:"skipped"`
	Items     []BatchActionItem `json:"items"`
}

// BatchDestroyRequest 批量销毁请求。
//
// 与单实例销毁同样的二次确认口径：逐个要求实例标识被显式列出（ConfirmMarks 是
// 「实例标识 → 用户照抄确认的文本」的映射），不做"全选即销毁"。
// 批量销毁不可逆且会真删上游机器，因此这里刻意**不给**"一个确认文本覆盖全部"的捷径。
type BatchDestroyRequest struct {
	IDs []uint64 `json:"ids" binding:"required"`
	// ConfirmMarks 实例标识 → 该实例的确认文本（须等于实例标识或记录 ID）。
	// 缺失的实例按"未确认"跳过，不静默执行。
	ConfirmMarks map[string]string `json:"confirm_marks" binding:"required"`
	Reason       string            `json:"reason"`
}

// BatchDestroyItem 批量销毁中的单实例结果。
type BatchDestroyItem struct {
	ID           uint64 `json:"id"`
	InstanceMark string `json:"instance_mark"`
	Name         string `json:"name"`
	// Status 复用批量运维三态：success / skipped / failed。
	Status       string `json:"status"`
	Message      string `json:"message"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
}

// BatchDestroyResponse 批量销毁响应（逐台结果，与批量运维同一形态便于前端共用回显）。
type BatchDestroyResponse struct {
	Total     int                `json:"total"`
	Succeeded int                `json:"succeeded"`
	Failed    int                `json:"failed"`
	Skipped   int                `json:"skipped"`
	Items     []BatchDestroyItem `json:"items"`
}

// RemarkRequest 管理员备注请求。
type RemarkRequest struct {
	Remark string `json:"remark"`
}

// RefundInstanceRequest 退款联动处置请求（订单侧钩子 → 实例运维台内部调用）。
type RefundInstanceRequest struct {
	OrderID      uint64  `json:"order_id"`
	OrderNo      string  `json:"order_no"`
	UserID       uint64  `json:"user_id"`
	RefundNo     string  `json:"refund_no"`
	RefundAmount float64 `json:"refund_amount"`
	PaidAmount   float64 `json:"paid_amount"`
	// FullRefund 本次退款是否已覆盖订单全部实付：只有全额退款才谈得上处置实例。
	FullRefund bool `json:"full_refund"`
}

// OperationItem 操作流水项。
type OperationItem struct {
	ID           uint64 `json:"id"`
	InstanceID   uint64 `json:"instance_id"`
	InstanceMark string `json:"instance_mark"`
	UserID       uint64 `json:"user_id"`
	OperatorType string `json:"operator_type"`
	OperatorID   uint64 `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Action       string `json:"action"`
	Params       string `json:"params"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
	Result       string `json:"result"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}

// OperationListQuery 操作流水分页查询。
type OperationListQuery struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// OperationListResponse 操作流水响应。
type OperationListResponse struct {
	Items []OperationItem `json:"items"`
	Meta  ListMeta        `json:"meta"`
}

// OperationLogQuery 全局操作流水查询（跨实例审计视图）。
type OperationLogQuery struct {
	Keyword      string `form:"keyword" json:"keyword"`             // 实例标识 / 操作人 / 用户名
	Action       string `form:"action" json:"action"`               // 动作：power_on/power_off/.../suspend/destroy/stage
	OperatorType string `form:"operator_type" json:"operator_type"` // admin/user/system
	Result       string `form:"result" json:"result"`               // success/failed/skipped
	UserID       uint64 `form:"user_id" json:"user_id"`
	StartTime    string `form:"start_time" json:"start_time"`
	EndTime      string `form:"end_time" json:"end_time"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"page_size" json:"page_size"`
}

// OperationLogItem 全局操作流水项。
type OperationLogItem struct {
	OperationItem
	InstanceName string `json:"instance_name"`
	Username     string `json:"username"`
}

// OperationLogListResponse 全局操作流水响应。
type OperationLogListResponse struct {
	Items []OperationLogItem `json:"items"`
	Meta  ListMeta           `json:"meta"`
}

// RelatedOrderItem 关联订单项。
type RelatedOrderItem struct {
	ID          uint64  `json:"id"`
	OrderNo     string  `json:"order_no"`
	Status      string  `json:"status"`
	ProductName string  `json:"product_name"`
	TotalAmount float64 `json:"total_amount"`
	PaidAmount  float64 `json:"paid_amount"`
	PayMethod   string  `json:"pay_method"`
	CreatedAt   string  `json:"created_at"`
}

// RelatedRenewalItem 关联续费记录项。
type RelatedRenewalItem struct {
	ID           uint64  `json:"id"`
	RenewalNo    string  `json:"renewal_no"`
	Source       string  `json:"source"`
	Status       string  `json:"status"`
	PeriodCount  int     `json:"period_count"`
	Amount       float64 `json:"amount"`
	ExpireBefore string  `json:"expire_before"`
	ExpireAfter  string  `json:"expire_after"`
	CreatedAt    string  `json:"created_at"`
}

// RelatedTicketItem 关联工单项。
type RelatedTicketItem struct {
	ID           uint64 `json:"id"`
	TicketNo     string `json:"ticket_no"`
	Title        string `json:"title"`
	Priority     string `json:"priority"`
	Status       string `json:"status"`
	AssignedName string `json:"assigned_name"`
	CreatedAt    string `json:"created_at"`
}

// RelatedInfo 实例关联的业务记录（订单/续费/工单）。
type RelatedInfo struct {
	Orders   []RelatedOrderItem   `json:"orders"`
	Renewals []RelatedRenewalItem `json:"renewals"`
	Tickets  []RelatedTicketItem  `json:"tickets"`
}
