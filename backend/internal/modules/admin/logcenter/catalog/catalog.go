// Package catalog 是日志中心的服务端单一真相：日志源注册表（doc92 §4.1）。
//
// 设计要点：
//   - 注册表在 init() 里静态定义，Cleanable 是编译期常量 —— 「哪些表能清」不来自
//     数据库、更不来自前端请求，因此不存在「前端传表名」的注入面（doc92 §0.3 硬约束 1）。
//   - 表名/列名/删除守卫 SQL 片段全部是代码常量，执行期唯一的外部参数是水位线时间，
//     以占位符绑定。
//   - Class 决定默认动作：ops 可清理；audit 只备份不删；账本类干脆不进注册表。
package catalog

import (
	"strings"
)

// Class 日志源分级。
const (
	ClassOps   = "ops"   // 运维流水，可清理
	ClassAudit = "audit" // 审计留痕，只备份不删
)

// Group 前端 Tab 分组。
const (
	GroupOps      = "ops"
	GroupAudit    = "audit"
	GroupSend     = "send"
	GroupSync     = "sync"
	GroupOpen     = "open"
	GroupUpstream = "upstream"
	GroupJob      = "job"
)

// Action 保留策略动作。
const (
	ActionClean       = "clean"        // 先导出后删除
	ActionArchiveOnly = "archive_only" // 仅导出冷备，不删行
	ActionKeep        = "keep"         // 不处理
)

// Column 一列的元数据（驱动前端动态表格渲染，doc92 §8.2）。
type Column struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Type 渲染类型：time/time_ms/text/longtext/number/bool/json/enum。
	Type string `json:"type"`
	// Width 建议列宽（0 = 自适应）。
	Width int `json:"width"`
	// Hidden true 时默认隐藏（列表不展示，详情仍可见）。
	Hidden bool `json:"hidden"`
	// Masked true 时列表与详情都打码，只有导出文件里是明文（doc92 §8.2）。
	Masked bool `json:"masked"`
	// Enum 枚举映射（type=enum 时下发，前端据此渲染中文标签）。
	Enum map[string]string `json:"enum,omitempty"`
}

// Source 一个可查询/可导出/可清理的日志源。
type Source struct {
	Key         string
	DisplayName string
	Group       string
	Table       string
	// TimeColumn 时间列：查询区间与清理水位线都作用于它。
	TimeColumn string
	// SelectColumns 允许查询/导出的列白名单（防 select *）。
	SelectColumns []Column
	// SearchColumns 关键词检索列（ILIKE）。
	SearchColumns []string
	// FilterColumns 允许的等值筛选：URL 参数名 → 列名。
	FilterColumns map[string]string
	// DeleteGuard 额外删除守卫 SQL 片段（如 status NOT IN ('pending','running')）。
	DeleteGuard string
	Class       string
	// Cleanable 仅 Class=ops 且此值为 true 才允许进入清理白名单。
	Cleanable            bool
	DefaultRetentionDays int
	// IndependentPage 该源另有独立业务页面查看（策略页据此加提示，doc92 §8.5）。
	IndependentPage string
	// Remark 展示给运营的一句话说明。
	Remark string
}

// col 简写：构造一个普通列；hidden 为可选参数（默认 false）。
func col(key, label, typ string, width int, hidden ...bool) Column {
	return Column{Key: key, Label: label, Type: typ, Width: width, Hidden: len(hidden) > 0 && hidden[0]}
}

// maskedCol 打码列。
func maskedCol(key, label string, width int) Column {
	return Column{Key: key, Label: label, Type: "text", Width: width, Masked: true}
}

// sources 26 个源的完整清单（doc92 §4.2）。
var sources = []Source{
	// —— audit：审计留痕，只备份不删 ——
	{
		Key: "admin_audit", DisplayName: "管理端操作审计", Group: GroupAudit,
		Table: "admin_audit_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		IndependentPage: "/system/audit-logs",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("admin_id", "管理员ID", "number", 100),
			col("admin_name", "管理员", "text", 120),
			col("action", "动作", "text", 120),
			col("resource_type", "资源类型", "text", 120),
			col("resource_id", "资源ID", "text", 100),
			col("module", "模块", "text", 100),
			col("request_method", "方法", "text", 80),
			col("request_path", "请求路径", "text", 240),
			col("response_code", "状态码", "number", 90),
			col("detail", "详情", "json", 0, true),
			maskedCol("ip", "IP", 130),
			col("user_agent", "UserAgent", "text", 0, true),
			col("trace_id", "TraceID", "text", 160),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"admin_name", "action", "resource_type", "request_path", "trace_id"},
		FilterColumns: map[string]string{"admin_id": "admin_id", "action": "action", "resource_type": "resource_type"},
	},
	{
		Key: "user_audit", DisplayName: "用户操作审计", Group: GroupAudit,
		Table: "user_operation_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("actor_user_id", "操作人ID", "number", 100),
			col("actor_name", "操作人", "text", 120),
			col("account_user_id", "账号ID", "number", 100),
			col("module", "模块", "text", 100),
			col("action", "动作", "text", 120),
			col("target", "目标", "text", 160),
			col("detail", "详情", "longtext", 0, true),
			maskedCol("ip", "IP", 130),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"actor_name", "module", "action", "target"},
		// account_user_id 是数据归属账号（成员在谁名下操作），actor_user_id 是操作人本人。
		// 用户详情页的「操作日志」Tab 按归属账号过滤，缺这一项时过滤器被静默忽略。
		FilterColumns: map[string]string{"actor_user_id": "actor_user_id", "account_user_id": "account_user_id", "module": "module", "action": "action"},
	},
	{
		Key: "security_audit", DisplayName: "安全审计日志", Group: GroupAudit,
		Table: "audit_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		// 原指向 /users/security/audit-logs，该页与「系统管理 → 操作审计」重复（同一张
		// audit_logs 表、同一权限码），已删除（doc102 §4.1 M2-5），这里跟随合并页。
		IndependentPage: "/system/audit-logs",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("operator_id", "操作人ID", "number", 100),
			col("operator_name", "操作人", "text", 120),
			col("module", "模块", "text", 100),
			col("resource_type", "资源类型", "text", 120),
			col("resource_id", "资源ID", "text", 100),
			col("action", "动作", "text", 120),
			col("request_method", "方法", "text", 80),
			col("request_path", "请求路径", "text", 240),
			col("request_payload", "请求载荷", "longtext", 0, true),
			col("response_code", "状态码", "number", 90),
			col("response_message", "响应信息", "text", 0, true),
			maskedCol("ip", "IP", 130),
			col("trace_id", "TraceID", "text", 160),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"operator_name", "module", "action", "request_path", "trace_id"},
		FilterColumns: map[string]string{"operator_id": "operator_id", "module": "module", "action": "action"},
	},
	{
		Key: "login", DisplayName: "登录日志", Group: GroupAudit,
		Table: "login_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		IndependentPage: "/users/security/login-logs",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("user_id", "用户ID", "number", 100),
			maskedCol("username", "账号", 140),
			col("login_type", "登录方式", "text", 120),
			col("result", "结果", "enum", 90, false),
			col("failure_reason", "失败原因", "text", 0, true),
			maskedCol("ip", "IP", 130),
			col("ip_region", "归属地", "text", 140),
			col("user_agent", "UserAgent", "text", 0, true),
			col("platform", "平台", "text", 110),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"username", "ip", "failure_reason"},
		FilterColumns: map[string]string{"user_id": "user_id", "login_type": "login_type", "result": "result"},
		Remark:        "登录结果 success/failed",
	},
	{
		Key: "ticket", DisplayName: "工单操作日志", Group: GroupAudit,
		Table: "ticket_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("ticket_id", "工单ID", "number", 100),
			col("operator_id", "操作人ID", "number", 100),
			col("operator_name", "操作人", "text", 120),
			col("action", "动作", "text", 120),
			col("from_value", "变更前", "text", 140),
			col("to_value", "变更后", "text", 140),
			col("note", "备注", "longtext", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"operator_name", "action", "note"},
		FilterColumns: map[string]string{"ticket_id": "ticket_id", "action": "action"},
	},
	{
		Key: "instance_ops", DisplayName: "实例操作流水", Group: GroupAudit,
		Table: "instance_operations", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		IndependentPage: "/instances/list",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("instance_id", "实例ID", "number", 100),
			col("instance_mark", "实例标识", "text", 150),
			col("user_id", "用户ID", "number", 100),
			col("operator_type", "操作者类型", "text", 110),
			col("operator_id", "操作者ID", "number", 100),
			col("operator_name", "操作者", "text", 120),
			col("action", "动作", "text", 120),
			col("params", "参数", "longtext", 0, true),
			col("before_status", "变更前状态", "text", 120),
			col("after_status", "变更后状态", "text", 120),
			col("result", "结果", "text", 100),
			col("error_message", "错误信息", "text", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"instance_mark", "operator_name", "action"},
		FilterColumns: map[string]string{"instance_id": "instance_id", "user_id": "user_id", "action": "action"},
	},
	{
		Key: "verification_review", DisplayName: "实名审核日志", Group: GroupAudit,
		Table: "verification_review_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("application_id", "申请ID", "number", 100),
			col("from_status", "变更前", "text", 110),
			col("to_status", "变更后", "text", 110),
			col("action", "动作", "text", 120),
			col("operator_id", "操作人ID", "number", 100),
			col("operator_name", "操作人", "text", 120),
			col("note", "备注", "longtext", 0, true),
			col("reject_reason", "驳回原因", "text", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"operator_name", "action", "note"},
		FilterColumns: map[string]string{"application_id": "application_id", "action": "action"},
	},
	{
		Key: "level_change", DisplayName: "用户等级变更", Group: GroupAudit,
		Table: "user_level_change_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("user_id", "用户ID", "number", 100),
			col("from_level_code", "原等级", "text", 110),
			col("to_level_code", "新等级", "text", 110),
			col("total_consume_amount", "累计消费", "number", 120),
			col("benefits_snapshot", "权益快照", "longtext", 0, true),
			col("reason", "原因", "text", 200),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"from_level_code", "to_level_code", "reason"},
		FilterColumns: map[string]string{"user_id": "user_id"},
	},
	{
		Key: "payment_callback", DisplayName: "支付回调日志", Group: GroupAudit,
		Table: "payment_callback_logs", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		IndependentPage: "/payment/callbacks",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("channel_id", "渠道ID", "number", 100),
			col("channel_code", "渠道编码", "text", 120),
			col("notify_id", "通知单号", "text", 180),
			col("payment_no", "支付单号", "text", 180),
			col("raw_body", "原始报文", "longtext", 0, true),
			col("headers", "请求头", "longtext", 0, true),
			col("verify_ok", "验签通过", "bool", 100),
			col("amount_fen", "金额(分)", "number", 110),
			col("handle_status", "处理状态", "text", 110),
			col("handle_msg", "处理信息", "text", 0, true),
			maskedCol("source_ip", "来源IP", 130),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"notify_id", "payment_no", "channel_code"},
		FilterColumns: map[string]string{"channel_id": "channel_id", "handle_status": "handle_status"},
	},
	{
		Key: "payment_recon", DisplayName: "支付对账记录", Group: GroupAudit,
		Table: "payment_recon_records", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("recon_no", "对账单号", "text", 180),
			col("channel_id", "渠道ID", "number", 100),
			col("channel_code", "渠道编码", "text", 120),
			col("period", "账期", "text", 110),
			col("local_amount_fen", "本地金额(分)", "number", 130),
			col("local_count", "本地笔数", "number", 100),
			col("channel_amount_fen", "渠道金额(分)", "number", 130),
			col("channel_count", "渠道笔数", "number", 100),
			col("diff_fen", "差额(分)", "number", 110),
			col("status", "状态", "text", 100),
			col("detail", "详情", "json", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"recon_no", "channel_code", "period"},
		FilterColumns: map[string]string{"channel_id": "channel_id", "status": "status"},
	},
	{
		Key: "risk_event", DisplayName: "风控事件", Group: GroupAudit,
		Table: "risk_events", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("risk_type", "风险类型", "text", 130),
			col("risk_level", "风险等级", "text", 110),
			col("user_id", "用户ID", "number", 100),
			maskedCol("username", "账号", 140),
			maskedCol("ip", "IP", 130),
			col("rule_code", "规则码", "text", 110),
			col("summary", "摘要", "text", 240),
			col("detail_payload", "详情", "longtext", 0, true),
			col("occur_count", "次数", "number", 90),
			col("status", "状态", "text", 100),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"risk_type", "username", "ip", "rule_code", "summary"},
		FilterColumns: map[string]string{"user_id": "user_id", "risk_level": "risk_level", "status": "status"},
	},
	{
		Key: "product_history", DisplayName: "商品变更历史", Group: GroupAudit,
		Table: "product_history", TimeColumn: "created_at",
		Class: ClassAudit, Cleanable: false, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("product_id", "商品ID", "number", 100),
			col("change_type", "变更类型", "text", 120),
			col("old_value", "变更前", "longtext", 0, true),
			col("new_value", "变更后", "longtext", 0, true),
			col("operator_id", "操作人ID", "number", 100),
			col("operator_name", "操作人", "text", 120),
			col("remark", "备注", "text", 200),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"change_type", "operator_name", "remark"},
		FilterColumns: map[string]string{"product_id": "product_id", "change_type": "change_type"},
	},

	// —— send：通知与验证码（doc90/doc91 产生） ——
	{
		Key: "notify_inbox", DisplayName: "站内信记录", Group: GroupSend,
		Table: "notifications", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 365,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("user_id", "用户ID", "number", 100),
			col("target_type", "目标类型", "text", 110),
			col("event", "事件", "text", 160),
			col("title", "标题", "text", 240),
			col("content", "内容", "longtext", 0, true),
			col("channel", "通道", "text", 100),
			col("send_status", "发送状态", "text", 110),
			col("fail_reason", "失败原因", "text", 0, true),
			col("source_module", "来源模块", "text", 120),
			col("read_at", "已读时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"title", "event", "source_module"},
		FilterColumns: map[string]string{"user_id": "user_id", "event": "event", "channel": "channel", "send_status": "send_status"},
		Remark:        "站内信是用户可见内容，保留期单独定为 365 天（doc92 §4.2 例外 1）",
	},
	{
		Key: "notify_delivery", DisplayName: "通知投递日志", Group: GroupSend,
		Table: "notification_deliveries", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("batch_id", "批次号", "text", 150),
			col("event", "事件", "text", 160),
			col("channel", "通道", "text", 90),
			col("target_type", "目标类型", "text", 100),
			col("target_id", "目标ID", "number", 100),
			col("target_name", "目标名称", "text", 130),
			maskedCol("recipient", "收件地址", 170),
			col("channel_id", "渠道ID", "number", 90),
			col("title", "标题", "text", 200),
			col("content", "内容", "longtext", 0, true),
			col("send_status", "状态", "text", 100),
			col("attempts", "尝试次数", "number", 100),
			col("provider_msg_id", "上游消息ID", "text", 0, true),
			col("provider_code", "上游码", "text", 100),
			col("fail_reason", "失败原因", "text", 0, true),
			col("source_module", "来源模块", "text", 110),
			col("sent_at", "发送时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"event", "title", "recipient"},
		FilterColumns: map[string]string{
			"event": "event", "channel": "channel", "send_status": "send_status",
			"target_id": "target_id", "batch_id": "batch_id",
		},
		Remark: "skipped=配置问题（不重试）；failed=上游/网络问题（按退避重试）",
	},
	{
		Key: "notify_read", DisplayName: "通知已读记录", Group: GroupSend,
		Table: "notification_reads", TimeColumn: "read_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 365,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("notification_id", "通知ID", "number", 100),
			col("user_id", "用户ID", "number", 100),
			col("read_at", "已读时间", "time", 170),
		},
		SearchColumns: []string{},
		FilterColumns: map[string]string{"user_id": "user_id", "notification_id": "notification_id"},
		Remark:        "跟随站内信保留期，避免「消息还在、已读态没了」的错位",
	},
	{
		Key: "verify_code", DisplayName: "验证码记录", Group: GroupSend,
		Table: "verification_codes", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("scene", "场景", "text", 170),
			col("channel", "通道", "text", 90),
			maskedCol("target", "接收目标", 170),
			col("status", "状态", "text", 100),
			col("attempts", "校验次数", "number", 100),
			col("max_attempts", "次数上限", "number", 100),
			col("provider_id", "服务商ID", "number", 100),
			col("provider_msg_id", "上游消息ID", "text", 0, true),
			col("cost_fen", "费用(分)", "number", 100),
			maskedCol("request_ip", "请求IP", 130),
			col("user_agent", "UserAgent", "text", 0, true),
			col("operator_id", "操作人ID", "number", 100),
			col("expire_at", "过期时间", "time", 170),
			col("used_at", "使用时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"scene", "target", "target_hash"},
		FilterColumns: map[string]string{"scene": "scene", "channel": "channel", "status": "status"},
		Remark:        "验证码只存哈希（code_hash）与盐，明文永不落库",
	},

	// —— sync：同步框架（既有清理先例的收编对象） ——
	{
		Key: "sync_log", DisplayName: "同步日志", Group: GroupSync,
		Table: "sync_logs", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		IndependentPage: "/resource/sync-center",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("task_id", "任务ID", "number", 100),
			col("provider_id", "渠道ID", "number", 100),
			col("sync_type", "同步类型", "text", 130),
			col("status", "状态", "enum", 100),
			col("total_count", "总数", "number", 90),
			col("success_count", "成功数", "number", 90),
			col("error_message", "错误信息", "text", 0, true),
			col("details", "明细", "longtext", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"sync_type", "error_message"},
		FilterColumns: map[string]string{"task_id": "task_id", "provider_id": "provider_id", "status": "status"},
	},
	{
		Key: "sync_task", DisplayName: "同步任务", Group: GroupSync,
		Table: "sync_tasks", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		// pending/running 的行是「还没结束的任务」，删掉会让调度器与运维都失去线索。
		DeleteGuard: "status NOT IN ('pending','running')",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("provider_id", "渠道ID", "number", 100),
			col("task_type", "任务类型", "text", 140),
			col("status", "状态", "enum", 110),
			col("total_count", "总数", "number", 90),
			col("success_count", "成功数", "number", 90),
			col("error_message", "错误信息", "text", 0, true),
			col("started_at", "开始时间", "time", 170),
			col("completed_at", "完成时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"task_type", "error_message"},
		FilterColumns: map[string]string{"provider_id": "provider_id", "status": "status", "task_type": "task_type"},
		Remark:        "pending/running 的行永不进入清理候选（状态终态保护）",
	},
	{
		Key: "sync_diff", DisplayName: "同步差异", Group: GroupSync,
		Table: "sync_diffs", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("task_id", "任务ID", "number", 100),
			col("provider_id", "渠道ID", "number", 100),
			col("scope", "范围", "text", 110),
			col("action", "动作", "text", 110),
			col("local_id", "本地ID", "number", 100),
			col("external_id", "外部ID", "text", 160),
			col("field", "字段", "text", 110),
			col("old_value", "原值", "longtext", 0, true),
			col("new_value", "新值", "longtext", 0, true),
			col("disposition", "处置", "text", 110),
			col("remark", "备注", "text", 200),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"external_id", "field", "remark"},
		FilterColumns: map[string]string{"task_id": "task_id", "provider_id": "provider_id", "scope": "scope", "action": "action"},
	},
	{
		Key: "price_change", DisplayName: "价格变更事件", Group: GroupSync,
		Table: "price_change_events", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		// 未处置的调价事件仍在运营待办里，不能因为超期就被清掉。
		DeleteGuard: "status <> 'pending'",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("provider_id", "渠道ID", "number", 100),
			col("scope", "范围", "text", 110),
			col("upstream_id", "上游ID", "text", 150),
			col("product_id", "商品ID", "number", 100),
			col("field", "字段", "text", 110),
			col("old_value", "原值", "number", 110),
			col("new_value", "新值", "number", 110),
			col("change_ratio", "变化比例", "number", 110),
			col("threshold", "阈值", "number", 100),
			col("status", "状态", "text", 100),
			col("applied", "已应用", "bool", 90),
			col("remark", "备注", "text", 200),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"upstream_id", "field", "remark"},
		FilterColumns: map[string]string{"provider_id": "provider_id", "scope": "scope", "status": "status"},
		Remark:        "status=pending 的待处置事件不清理",
	},
	{
		Key: "provision_task", DisplayName: "开通任务", Group: GroupSync,
		Table: "provision_tasks", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		// 白名单式守卫：只有明确的终态才允许删。
		DeleteGuard: "status IN ('success','failed','dead','cancelled')",
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("order_id", "订单ID", "number", 100),
			col("user_id", "用户ID", "number", 100),
			col("product_id", "商品ID", "number", 100),
			col("source_mode", "来源", "text", 110),
			col("status", "状态", "text", 100),
			col("attempts", "尝试次数", "number", 100),
			col("last_error", "最近错误", "text", 0, true),
			col("started_at", "开始时间", "time", 170),
			col("completed_at", "完成时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"last_error"},
		FilterColumns: map[string]string{"order_id": "order_id", "user_id": "user_id", "status": "status"},
		Remark:        "仅终态（success/failed/dead/cancelled）可清理",
	},

	// —— open：开放平台 ——
	{
		Key: "open_api", DisplayName: "开放平台调用日志", Group: GroupOpen,
		Table: "open_api_logs", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("app_id", "应用ID", "number", 100),
			col("method", "方法", "text", 80),
			col("path", "路径", "text", 220),
			col("query", "查询串", "longtext", 0, true),
			col("client_request_id", "客户端请求ID", "text", 200),
			col("status_code", "状态码", "number", 90),
			col("error_code", "错误码", "number", 90),
			col("duration_ms", "耗时(ms)", "number", 100),
			col("request_digest", "请求摘要", "text", 0, true),
			col("response_digest", "响应摘要", "text", 0, true),
			maskedCol("ip", "IP", 130),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"path", "client_request_id"},
		FilterColumns: map[string]string{"app_id": "app_id", "status_code": "status_code"},
	},
	{
		Key: "open_request", DisplayName: "开放平台幂等请求", Group: GroupOpen,
		Table: "open_requests", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("app_id", "应用ID", "number", 100),
			col("client_request_id", "客户端请求ID", "text", 200),
			col("method", "方法", "text", 80),
			col("path", "路径", "text", 220),
			col("status", "状态", "text", 110),
			col("response_status", "响应码", "number", 100),
			col("response_body", "响应体", "longtext", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"client_request_id", "path"},
		FilterColumns: map[string]string{"app_id": "app_id", "status": "status"},
	},
	{
		Key: "open_notify", DisplayName: "开放平台回调投递", Group: GroupOpen,
		Table: "open_notify_deliveries", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("app_id", "应用ID", "number", 100),
			col("event", "事件", "text", 150),
			col("payload", "载荷", "json", 0, true),
			col("status", "状态", "text", 100),
			col("attempts", "尝试次数", "number", 100),
			col("max_attempts", "最大次数", "number", 100),
			col("last_error", "最近错误", "text", 0, true),
			col("next_retry_at", "下次重试", "time", 170),
			col("delivered_at", "投递时间", "time", 170),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"event", "last_error"},
		FilterColumns: map[string]string{"app_id": "app_id", "status": "status", "event": "event"},
	},

	// —— upstream / job：本模块新建 ——
	{
		Key: "upstream_api", DisplayName: "上游厂商接口日志", Group: GroupUpstream,
		Table: "upstream_api_logs", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("provider_id", "渠道ID", "number", 100),
			col("provider_name", "渠道名称", "text", 160),
			col("provider_type", "渠道类型", "text", 130),
			col("op", "操作", "text", 180),
			col("method", "方法", "text", 80),
			col("url", "URL", "text", 0, true),
			col("status_code", "状态码", "number", 90),
			col("success", "成功", "bool", 90),
			col("error_code", "错误码", "text", 110),
			col("error_message", "错误信息", "text", 240),
			col("duration_ms", "耗时(ms)", "number", 100),
			col("retry_index", "重试序号", "number", 100),
			col("request_bytes", "请求字节", "number", 110),
			col("request_truncated", "请求截断", "bool", 100),
			col("request_body", "请求体", "longtext", 0, true),
			col("response_bytes", "响应字节", "number", 110),
			col("response_truncated", "响应截断", "bool", 100),
			col("response_body", "响应体", "longtext", 0, true),
			col("request_digest", "请求摘要", "text", 0, true),
			col("response_digest", "响应摘要", "text", 0, true),
			col("trace_id", "TraceID", "text", 160),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"op", "error_message", "trace_id"},
		FilterColumns: map[string]string{
			"provider_id": "provider_id", "op": "op", "method": "method",
			"status_code": "status_code", "success": "success",
		},
		Remark: "摘要（sha256）恒写，请求/响应正文受采样与截断开关控制且已脱敏",
	},
	{
		Key: "job_run", DisplayName: "定时任务日志", Group: GroupJob,
		Table: "job_run_logs", TimeColumn: "created_at",
		Class: ClassOps, Cleanable: true, DefaultRetentionDays: 180,
		SelectColumns: []Column{
			col("id", "ID", "number", 80),
			col("job_name", "任务名", "text", 180),
			col("job_group", "分组", "text", 110),
			col("trigger_type", "触发方式", "text", 110),
			col("status", "状态", "text", 100),
			col("started_at", "开始时间", "time", 170),
			col("finished_at", "结束时间", "time", 170),
			col("duration_ms", "耗时(ms)", "number", 100),
			col("items_scanned", "扫描数", "number", 100),
			col("items_affected", "处理数", "number", 100),
			col("summary", "摘要", "json", 0, true),
			col("error_message", "错误信息", "text", 0, true),
			col("created_at", "时间", "time", 170),
		},
		SearchColumns: []string{"job_name", "error_message"},
		FilterColumns: map[string]string{"job_name": "job_name", "job_group": "job_group", "status": "status"},
		Remark:        "高频轮询任务空轮不写日志，避免日志量爆炸",
	},
}

// byKey 索引化注册表。
var byKey = func() map[string]*Source {
	m := make(map[string]*Source, len(sources))
	for i := range sources {
		m[sources[i].Key] = &sources[i]
	}
	return m
}()

// All 返回全部日志源（副本，调用方不能篡改注册表）。
func All() []Source {
	out := make([]Source, len(sources))
	copy(out, sources)
	return out
}

// Get 按 key 取源；不存在返回 nil。
//
// 返回值判空是执行期的一道硬防线：key 拼错时拒绝执行，而不是静默「清理 0 行」。
func Get(key string) *Source {
	return byKey[strings.TrimSpace(key)]
}

// Keys 返回全部 key（顺序与 All 一致）。
func Keys() []string {
	out := make([]string, 0, len(sources))
	for i := range sources {
		out = append(out, sources[i].Key)
	}
	return out
}

// CleanableKeys 返回可清理源的 key。
func CleanableKeys() []string {
	out := make([]string, 0, len(sources))
	for i := range sources {
		if sources[i].Cleanable {
			out = append(out, sources[i].Key)
		}
	}
	return out
}

// GroupOrder 前端分组展示顺序。
func GroupOrder() []string {
	return []string{GroupOps, GroupAudit, GroupSend, GroupSync, GroupOpen, GroupUpstream, GroupJob}
}

// GroupLabel 分组中文名。
func GroupLabel(group string) string {
	switch group {
	case GroupOps:
		return "运维流水"
	case GroupAudit:
		return "审计留痕（仅备份）"
	case GroupSend:
		return "通知与验证码"
	case GroupSync:
		return "同步框架"
	case GroupOpen:
		return "开放平台"
	case GroupUpstream:
		return "上游接口"
	case GroupJob:
		return "定时任务"
	default:
		return group
	}
}

// ActionLabel 动作中文名。
func ActionLabel(action string) string {
	switch action {
	case ActionClean:
		return "清理"
	case ActionArchiveOnly:
		return "仅备份不清理"
	case ActionKeep:
		return "保留不处理"
	default:
		return action
	}
}

// DefaultRetention 源的默认保留天数（>=7）。
func (s *Source) DefaultRetention() int {
	if s == nil {
		return 0
	}
	if s.DefaultRetentionDays <= 0 {
		return DefaultRetentionDays
	}
	return s.DefaultRetentionDays
}

// DefaultRetentionDays 全局默认保留期（doc89 §9.1 log_retention_days 默认值）。
const DefaultRetentionDays = 180

// MinRetentionDays 代码硬地板：任何来源（前端传参、配置、策略行）都不能突破。
//
// 防止误操作把保留期设成 0 导致「删光所有日志」（doc92 §6.2）。
const MinRetentionDays = 7

// EffectiveAction 源的默认动作（audit 类只能 archive_only）。
func (s *Source) EffectiveAction() string {
	if s == nil {
		return ActionKeep
	}
	if !s.Cleanable {
		return ActionArchiveOnly
	}
	return ActionClean
}

// Sources 全局唯一的源清单（顺序即前端展示顺序，按 group 归并后保持稳定）。
func Sources(group string) []Source {
	if group == "" {
		return All()
	}
	out := make([]Source, 0, len(sources))
	for i := range sources {
		if sources[i].Group == group {
			out = append(out, sources[i])
		}
	}
	return out
}
