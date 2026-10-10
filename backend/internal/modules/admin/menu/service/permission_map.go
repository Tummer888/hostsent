package service

import (
	"hostsent/backend/internal/modules/admin/menu/dto"
	appauth "hostsent/backend/internal/pkg/auth"
)

// menuPermissionMap 菜单路径 → 所需权限码。
// 后端按当前管理员权限过滤菜单后再返回，前端不做权限过滤（否则等于把权限表交给客户端，82 P1-13）。
// 未登记的路径视为无需权限（如仪表盘），保证不会误隐藏。
//
// 键只对 admin 平台生效：FilterByPermissions 仅在 platform == "admin" 时被调用
// （见 MenuHandler.Tree）。/dashboard、/points、/referral 等 6 条路径被 admin 与 user
// 两个平台复用，本表的值只描述 admin 侧口径，由 menu_align_test.go 断言 2 锁定。
var menuPermissionMap = map[string]string{
	"/dashboard":      "",
	"/dashboard/base": "",

	"/users/overview":        "system:user:list",
	"/users/accounts/list":   "system:user:list",
	"/users/accounts/groups": "user:group:list",

	"/users/security/login-logs": "security:login-log:list",
	"/users/security/risk":       "security:risk:list",
	"/users/security/blacklist":  "security:blacklist:manage",
	"/users/security/sessions":   "security:session:manage",

	"/users/accounts/levels": "level:list",
	// 代理等级（doc108）：折扣的唯一来源，与用户等级是两套阶梯。

	"/users/verification/list":   "verification:list",
	"/users/verification/config": "verification:list",

	"/resource/providers": "resource:provider",
	// 双链路拆页（本轮 S1）：自营平台对接与上游转售渠道同权限口径。
	"/resource/platforms": "resource:provider",
	"/resource/pools":     "resource:provider",
	// T3.6 合并页「同步与调度」（调度/任务/日志/差异/待确认调价）。
	"/resource/sync-center": "resource:sync",
	"/resource/anomalies":   "resource:instance",
	// 任务队列（本轮 S3）：平台动作是否到达上游，沿用同步权限口径。
	"/resource/task-queue": "resource:sync",
	// 实例对账（本轮 S4）：本地实例与上游成本/账期比对。
	"/resource/reconcile": "resource:instance",

	// 实例管理域：运维台与只读资产视图（原 /resource/instances，doc102 §3.3 迁入本域）。
	"/instances":           "resource:instance",
	"/instances/list":      "resource:instance",
	"/instances/inventory": "resource:instance",
	// 全局操作流水（跨实例审计视图，doc61 P1）。
	"/instances/operations": "resource:instance",
	// 生命周期与续费（doc60）整域并入本域（迁移 066）：路径 /lifecycle/* → /instances/lifecycle/*，
	// 权限码不变。
	"/instances/lifecycle/expiring": "lifecycle:expiring",
	"/instances/lifecycle/renewals": "lifecycle:renewals",
	"/instances/lifecycle/policy":   "lifecycle:policy",
	// 到期处置（doc61 §8.4）：预演报告与手动单实例执行。
	"/instances/lifecycle/enforcement": "lifecycle:enforce",

	"/product/products": "product:list",
	// T7.2 商品对接（doc16 §9.3）：组件复用资源侧页面，权限沿用资源商品口径。
	"/product/upstream":       "resource:product",
	"/product/cost-pricing":   "product:update_price",
	"/product/spec/templates": "spec:template:list",
	// /product/spec/custom 已随 077 下线（菜单与页面一并移除），此处不再登记。
	"/product/spec/mappings": "spec:mapping:list",
	// 平台配置项目录（T4.5）：按对接平台维护可配置项与取值库。
	"/product/spec/option-catalog": "spec:option:list",
	"/product/pricing":             "pricing:list",
	"/product/pricing/calculator":  "pricing:list",
	"/product/pricing/history":     "pricing:list",
	// 周期价格矩阵（doc25）：与折扣策略同权限口径（定价查看/维护）。
	"/product/pricing/matrix": "pricing:list",
	// 折扣策略此前漏登记 → 菜单对所有角色可见（doc23 问题清单）。
	"/product/pricing/policies":     "pricing:list",
	"/product/promotion/coupons":    "promotion:coupon:list",
	"/product/promotion/activities": "promotion:activity:list",
	"/product/promotion/bundles":    "promotion:activity:list",
	"/product/promotion/recommends": "promotion:activity:list",
	"/product/categories":           "product:category",

	"/orders/list":    "order:list",
	"/orders/refunds": "order:refunds",
	"/orders/stats":   "order:stats",

	"/finance/overview":        "finance:wallet",
	"/finance/accounts/adjust": "finance:adjust",
	"/finance/transactions":    "finance:wallet",
	// 充值管理/提现管理（迁移 084 改挂资金管理/推广返现，路径与权限码均不变）。
	"/finance/recharges":   "finance:recharge",
	"/finance/withdrawals": "finance:withdraw",
	"/finance/bills":           "finance:bill",
	"/finance/recon":           "finance:bill",
	// 发票管理（doc36 §3.3）：与账单同域，开票动作由 finance:invoice:issue 细分。
	"/finance/invoices": "finance:invoice",
	"/finance/report":   "finance:wallet",
	// 成本管理（doc111）：总览/成本项/上游余额台账三页三码。
	"/finance/cost/overview":  "finance:cost:overview",
	"/finance/cost/items":     "finance:cost:item",
	"/finance/cost/upstreams": "finance:cost:balance",
	// 财务参数页独立权限码（本轮整理）：财务角色可自行调参，不必被授 system:config:*。
	// 资金流水改挂 /finance/accounts 目录（迁移 073）不改路径，因此本表键不变。
	"/finance/config": "finance:config",
	// 推广返现（doc84）整域并入财务管理（迁移 070）：路径 /referral/* → /finance/referral/*，
	// 权限码不变。
	"/finance/referral/cashbacks":   "referral:cashback:list",
	"/finance/referral/invitees":    "referral:cashback:list",
	"/finance/referral/withdrawals": "referral:withdraw:list",
	// 积分中心（doc36）整域并入财务管理（迁移 070）：路径 /points/* → /finance/points/*，
	// 权限码不变。用户端同名的 /points 菜单不查本表（FilterByPermissions 仅 admin 平台）。
	"/finance/points/overview":     "point:account",
	"/finance/points/rules":        "point:rule",
	"/finance/points/accounts":     "point:account",
	"/finance/points/transactions": "point:transaction",

	"/tickets/list":       "ticket:list",
	"/tickets/categories": "ticket:category",
	"/tickets/stats":      "ticket:stats",
	// 复核中心（S3 双人复核）：与回复复核同一权限码。
	"/tickets/reviews": "ticket:review",

	"/system/roles":       "system:role:list",
	"/system/permissions": "system:permission:view",
	"/system/admins":      "staff:list",
	"/system/departments": "department:list",
	"/system/menus":       "system:menu",
	// 系统配置组（迁移 071/072）：原 /system/config、/system/captcha、/system/oauth 收进同一目录，
	// 路径改为 /system/config/*，权限码不变。操作审计（security:audit:list）同期收进本组，
	// 迁移 072 又改挂「日志中心」，并恢复原路径 /system/audit-logs（见日志中心分组）。
	"/system/config/basic":   "system:config:view",
	"/system/config/captcha": "captcha:config",
	"/system/config/oauth":   "oauth:config",
	// 日志中心（doc92 §9.1）：日志含手机号/邮箱/上游请求体，仅超管与运维可见。
	"/system/logs":         "log:center",
	"/system/logs/cleanup": "log:cleanup",
	"/system/logs/policy":  "log:policy",
	// 操作审计页与「用户管理 → 安全与风控」共用 security:audit:list（同一件事的两个入口）。
	"/system/audit-logs": "security:audit:list",
	// 支付中心（doc35）整域并入系统管理（迁移 067）：路径 /payment/* → /system/payment/*，
	// 权限码不变。支付概览聚合 5 个接口（channels/orders/callbacks/payouts/refunds），
	// 口径与 router.meta.permission 统一为 payment:order（doc102 M3-1）。
	"/system/payment/overview":  "payment:order",
	"/system/payment/channels":  "payment:channel",
	"/system/payment/methods":   "payment:method",
	"/system/payment/orders":    "payment:order",
	"/system/payment/callbacks": "payment:callback",
	"/system/payment/refunds":   "payment:refund",
	"/system/payment/payouts":   "payment:payout",
	"/system/payment/recon":     "payment:recon",
	// 消息中心（doc70/doc90）整域并入系统管理（迁移 068）：路径 /notification/* →
	// /system/notification/*，权限码不变。
	"/system/notification/records":       "notify:record",
	"/system/notification/templates":     "notify:template",
	"/system/notification/channels":      "notify:channel",
	"/system/notification/sms-templates": "notify:sms-template",
	"/system/notification/broadcast":     "notify:broadcast",
	"/system/notification/deliveries":    "notify:delivery",
	// 销售中心（S1/S4–S6）整域并入系统管理（迁移 068）：路径 /sales/* → /system/sales/*。
	// 提成审核页同时服务主管审核，菜单权限取审核码；销售的自助入口在台账页按钮级，
	// 后端 GET /sales/withdrawals 仍用 sales:commission:list 放行本人查询（doc86 §5 注 2）。
	"/system/sales/customers":   "sales:customer:list",
	"/system/sales/commissions": "sales:commission:list",
	"/system/sales/withdrawals": "sales:commission:audit",
	"/system/sales/performance": "sales:performance:view",
	// 内容管理（doc100 §7.1）整域并入系统管理（迁移 069）：路径 /content/* → /system/content/*，
	// 权限码不变。公告管理沿用 notify:announcement（doc102 §4.1 M2-4 的 R5 例外）。
	// 门户展示型内容漏登记过一次 —— 未登记的路径在 FilterByPermissions 里被视为
	// 「无需权限」原样返回，等于把内容管理菜单对所有角色（含只读客服）敞开，
	// 所以这四个路径必须显式登记。页脚配置没有独立页面（在「系统管理 → 系统配置」里编辑），
	// 故不在此登记。
	"/system/content/articles":      "content:article:list",
	"/system/content/categories":    "content:category:list",
	"/system/content/links":         "content:link:list",
	"/system/content/announcements": "notify:announcement",
}

// PermissionMap 返回菜单路径 → 权限码的只读副本。导出供对齐门禁测试
// （menu_align_test.go）与「菜单体检」使用；不要用它做运行时鉴权。
func PermissionMap() map[string]string {
	out := make(map[string]string, len(menuPermissionMap))
	for k, v := range menuPermissionMap {
		out[k] = v
	}
	return out
}

// FilterByPermissions 按权限集合过滤菜单树：
//   - 叶子节点命中所需权限（或路径未登记）才保留；
//   - 目录节点只要有一个可见子节点就保留，全部子节点被过滤则整目录移除；
//   - 权限集合含通配 "*"（超管）时原样返回。
func FilterByPermissions(nodes []dto.MenuNode, perms appauth.PermissionSet) []dto.MenuNode {
	if perms.IsSuper() {
		return nodes
	}
	filtered := make([]dto.MenuNode, 0, len(nodes))
	for _, node := range nodes {
		children := FilterByPermissions(node.Children, perms)
		isDir := node.Type == "directory" || len(node.Children) > 0
		if isDir {
			if len(children) == 0 {
				continue
			}
			node.Children = children
			filtered = append(filtered, node)
			continue
		}
		code, mapped := menuPermissionMap[node.Path]
		if mapped && code != "" && !perms.Has(code) {
			continue
		}
		node.Children = nil
		filtered = append(filtered, node)
	}
	return filtered
}
