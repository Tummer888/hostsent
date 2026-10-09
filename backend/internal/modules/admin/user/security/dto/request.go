package dto

type LoginLogListQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	UserID    uint64 `form:"user_id"`
	Username  string `form:"username"`
	Result    string `form:"result"`
	LoginType string `form:"login_type"`
	IP        string `form:"ip"`
	RiskFlag  string `form:"risk_flag"`
	// SubjectType 登录主体域（user/admin）。留空表示不限 —— 全局安全页需要
	// 同时看到两类；客户详情面板必须显式传 user，否则会串到 ID 相同的员工记录上。
	SubjectType string `form:"subject_type"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}

type AuditLogListQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"page_size"`
	Operator     string `form:"operator"`
	Module       string `form:"module"`
	Action       string `form:"action"`
	Result       string `form:"result"`
	ResourceType string `form:"resource_type"`
	ResourceID   string `form:"resource_id"`
	StartTime    string `form:"start_time"`
	EndTime      string `form:"end_time"`
}

type RiskEventListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
	// UserID 按用户过滤；缺这一项时 /security/risk-events?user_id=N 会被静默忽略，
	// 返回全量数据 —— 详情页「安全」Tab 因此拿到的是别人的风险事件。
	UserID    uint64 `form:"user_id"`
	RiskType  string `form:"risk_type"`
	RiskLevel string `form:"risk_level"`
	Status    string `form:"status"`
	// Action 按已执行过的处置动作筛选（blacklist / revoke_sessions / handle / ignore / level）。
	//
	// 与 Status 互补：状态筛「待办关掉没有」，动作筛「做没做过某类管控」。
	// 运营的两个高频问题分别是「哪些还没处理」和「哪些已经封过了」。
	Action    string `form:"action"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type BlacklistListQuery struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Type      string `form:"type"`
	Status    string `form:"status"`
	Source    string `form:"source"`
	Keyword   string `form:"keyword"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type SessionListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	UserID   uint64 `form:"user_id"`
	Username string `form:"username"`
	Status   string `form:"status"`
	Platform string `form:"platform"`
	IP       string `form:"ip"`
	RiskFlag string `form:"risk_flag"`
	// SubjectType 会话主体域（user/admin），语义同 LoginLogListQuery。
	SubjectType string `form:"subject_type"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}

// BlacklistHitListQuery 黑名单命中记录的分页参数。
//
// 命中记录是「该黑名单在登录日志里命中的行」，与登录日志同构，
// 因此分页参数独立成一个小结构，而不是硬编码在仓储里（旧实现写死 1,10，
// 前端翻页翻不动、总数也不对）。
type BlacklistHitListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// RiskEventHandleRequest 关闭一条风险事件（处置 / 忽略）。
//
// 处置与忽略只回答「这条待办关掉没有」，与「要不要同时做管控」是两件事：
// 大部分事件看一眼就知道没事（忽略即可），但确认有问题的那些，运营希望
// 一次点完「处置 + 拉黑 + 踢会话」，而不是关掉待办后再去列表里重新找到它、
// 再点两次。因此这里允许随关单一起带上管控动作。
type RiskEventHandleRequest struct {
	// Note 处置说明（写进事件的 handle_note，页面「处置说明」可见）。
	Note string `json:"note"`
	// Blacklist 关闭待办的同时把该事件的来源加入黑名单。
	Blacklist bool `json:"blacklist"`
	// BlacklistType 拉黑维度：ip / user / device / phone / email；
	// 留空时按事件可用字段自动推断（优先 IP，其次设备，最后账号）。
	BlacklistType string `json:"blacklist_type"`
	// RevokeSessions 关闭待办的同时强制下线该主体（仅客户域支持，见服务层说明）。
	RevokeSessions bool `json:"revoke_sessions"`
}

// RiskEventLevelRequest 手动调整风险等级（doc06 §4.3「手动升级风险等级」）。
type RiskEventLevelRequest struct {
	// RiskLevel 目标等级：low / medium / high / critical。
	RiskLevel string `json:"risk_level" binding:"required"`
	// Note 调整原因（写进 handle_note，页面「处置说明」列可见）。
	Note string `json:"note"`
	// CloseEvent 调级的同时关掉待办（默认 false）。
	//
	// 默认不关：提级是「这条要重点看」的标记，顺手关掉会让刚提级的事件从
	// 「待处理」里消失，运营反而丢了待办。但反过来也成立 —— 运营把一条误报
	// 降级为低级时，心愿就是「标记完就结案」，不该强制他再点一次忽略。
	CloseEvent bool `json:"close_event"`
	// CloseAs 配合 CloseEvent：handled（已处置）或 ignored（已忽略）。
	CloseAs string `json:"close_as"`
}

// RiskEventBlacklistRequest 从风险事件拉黑。
//
// 单独一个结构体（而不是复用 Handle 请求）是因为它多一个必填语义：
// 拉黑是管控动作，不改变待办状态 —— 运营可以既保留这条待办继续观察、
// 又先把来源封掉。
type RiskEventBlacklistRequest struct {
	// Type 拉黑维度；留空按事件可用字段推断。
	Type string `json:"type"`
	// Note 拉黑原因（写进黑名单 reason，同时记进处置流水）。
	Note string `json:"note"`
	// CloseEvent 是否同时把这条待办关成「已处置」（默认 false）。
	CloseEvent bool `json:"close_event"`
}

// RiskEventRevokeRequest 从风险事件强制下线。
type RiskEventRevokeRequest struct {
	Note string `json:"note"`
	// CloseEvent 是否同时把这条待办关成「已处置」（默认 false）。
	CloseEvent bool `json:"close_event"`
}

// BlacklistCreateRequest 新增黑名单。
//
// 永久生效与限时生效靠 ExpiredAt 区分（空 = 永久），doc06 §4.4 关键规则 2。
type BlacklistCreateRequest struct {
	Type        string `json:"type" binding:"required"`
	TargetValue string `json:"target_value" binding:"required"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Reason      string `json:"reason"`
	// ExpiredAt 失效时间（RFC3339 或 "2006-01-02 15:04:05"）；空 = 永久生效。
	ExpiredAt string `json:"expired_at"`
}

type BlacklistUpdateRequest struct {
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	ExpiredAt string `json:"expired_at"`
}

type BlacklistStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type SessionRevokeRequest struct {
	Reason string `json:"reason"`
}

type SessionBatchRevokeRequest struct {
	IDs    []uint64 `json:"ids" binding:"required"`
	Reason string   `json:"reason"`
}

type SessionRevokeUserAllRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Reason string `json:"reason"`
}
