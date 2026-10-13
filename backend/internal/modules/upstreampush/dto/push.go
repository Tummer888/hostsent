// Package dto 提供上游推送接收端的出入参定义。
//
// 字段名与魔方财务（zjmf）推送方的 POST 表单字段逐一对齐，不做重命名——
// 上游用 http_build_query 发的是 flat form，字段名即协议，改名只会让联调时
// 两边对不上。来源见上游源码 app/zjmf.php 的 pushHostInfo / pushTicketReply。
package dto

// HostSyncRequest 主机信息推送（POST {downstream_url}/api/host/sync）。
//
// id 是「下游侧的主机标识」：上游在 pushHostInfo 里把 host.id 换成了
// stream_info.downstream_id，而该值由我方开通时以 DownstreamID=order.ID 下发，
// 所以这里就是 orders.id。host_id 是上游自己的主机 id，仅供留痕。
type HostSyncRequest struct {
	// 验签三要素：id 参与签名；token 上游不发送（接收方用本地存的令牌补上）；
	// rand_str 与 signature 由 createSign 生成。
	ID        string `form:"id"`
	Signature string `form:"signature"`
	RandStr   string `form:"rand_str"`

	HostID string `form:"host_id"`

	// 主机身份与接入信息。
	Domain      string `form:"domain"`
	Username    string `form:"username"`
	Password    string `form:"password"`
	DedicatedIP string `form:"dedicatedip"`
	AssignedIPs string `form:"assignedips"`
	Port        string `form:"port"`
	OS          string `form:"os"`
	OSURL       string `form:"os_url"`
	// IPPassword 面板密码（上游 getPanelPass 取到才带）。
	IPPassword string `form:"ippassword"`

	// 账期与状态。
	NextDueDate   string `form:"nextduedate"` // Unix 秒级时间戳字符串（上游为空时会 unset）
	DomainStatus  string `form:"domainstatus"`
	SuspendReason string `form:"suspendreason"`
	// Type 仅开通场景带 "create"（上游用它触发一次性的开通后流程）。
	Type string `form:"type"`
}

// TicketReplySyncRequest 工单回复推送（POST {downstream_url}/api/ticket_reply/sync）。
//
// tid 是上游工单号（上游 ticket.tid），接收方按本地 tickets.upstream_tid 反查。
// id 是下游主机标识（参与签名），rid 是上游回复号（去重用）。
type TicketReplySyncRequest struct {
	ID        string `form:"id"`
	Signature string `form:"signature"`
	RandStr   string `form:"rand_str"`

	TID string `form:"tid"`
	RID string `form:"rid"`

	Content string `form:"content"`
	// Password 上游附带的宿主密码（上游协议里给下游做凭证同步用），我方不使用。
	Password string `form:"password"`
}

// HostSyncResult 接收结果的回应内容，便于上游侧联调定位（上游只判 HTTP 200）。
type HostSyncResult struct {
	// Applied 是否真的改动了本地实例行（false = 内容与本地一致，幂等命中）。
	Applied bool `json:"applied"`
	// InstanceID 命中的本地实例主键（0 表示未找到实例行，即"推送早于开通落库"）。
	InstanceID uint64 `json:"instance_id"`
	// ChangedFields 本次实际改动的字段名，便于排查"推了但状态没变"。
	ChangedFields []string `json:"changed_fields"`
}

// TicketReplySyncResult 工单回复接收结果。
type TicketReplySyncResult struct {
	Applied  bool   `json:"applied"` // false = 该回复已存在（上游重试的幂等命中）
	TicketID uint64 `json:"ticket_id"`
	ReplyID  uint64 `json:"reply_id"`
}
