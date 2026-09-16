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

	"/users/levels": "level:list",

	"/users/verification/pending":  "verification:list",
	"/users/verification/approved": "verification:list",
	"/users/verification/rejected": "verification:list",
	"/users/verification/config":   "verification:list",

	"/referral/cashbacks":   "referral:cashback:list",
	"/referral/invitees":    "referral:cashback:list",
	"/referral/withdrawals": "referral:withdraw:list",

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

	"/product/products": "product:list",
	// T7.2 商品对接（doc16 §9.3）：组件复用资源侧页面，权限沿用资源商品口径。
	"/product/upstream":           "resource:product",
	"/product/cost-pricing":       "product:update_price",
	"/product/spec/templates":     "spec:template:list",
	"/product/spec/custom":        "spec:template:list",
	"/product/spec/mappings":      "spec:mapping:list",
	"/product/pricing":            "pricing:list",
	"/product/pricing/calculator": "pricing:list",
	"/product/pricing/history":    "pricing:list",
	// 周期价格矩阵（doc25）：与折扣策略同权限口径（定价查看/维护）。
	"/product/pricing/matrix": "pricing:list",
	// 折扣策略此前漏登记 → 菜单对所有角色可见（doc23 问题清单）。
	"/product/pricing/policies":     "pricing:list",
	"/product/promotion/coupons":    "promotion:coupon:list",
	"/product/promotion/activities": "promotion:activity:list",
	"/product/promotion/bundles":    "promotion:activity:list",
	"/product/promotion/recommends": "promotion:activity:list",
	"/product/categories":           "product:category",

	// 内容中心（doc100 §7.1）：门户展示型内容。此处漏登记过一次 —— 未登记的路径
	// 在 FilterByPermissions 里被视为「无需权限」原样返回，等于把内容管理菜单
	// 对所有角色（含只读客服）敞开，所以这四个路径必须显式登记。
	// 公告管理（doc102 §4.1 M2-4）由系统管理迁入本域，权限码仍是 notify:announcement。
	// 页脚配置没有独立页面（在「系统管理 → 系统配置」里编辑），故不在此登记。
	"/content/articles":      "content:article:list",
	"/content/categories":    "content:category:list",
	"/content/links":         "content:link:list",
	"/content/announcements": "notify:announcement",

	"/orders/list":    "order:list",
	"/orders/refunds": "order:refunds",
	"/orders/stats":   "order:stats",

	"/finance/overview":         "finance:wallet",
	"/finance/accounts/wallets": "finance:wallet",
	"/finance/accounts/adjust":  "finance:adjust",
	"/finance/transactions":     "finance:wallet",
	"/finance/recharges":        "finance:recharge",
	"/finance/withdrawals":      "finance:withdraw",
	"/finance/bills":            "finance:bill",
	"/finance/recon":            "finance:bill",
	// 发票管理（doc36 §3.3）：与账单同域，开票动作由 finance:invoice:issue 细分。
	"/finance/invoices": "finance:invoice",
	"/finance/report":   "finance:wallet",
	"/finance/config":   "finance:wallet",

	// 支付中心（doc35）：渠道/支付方式/支付单/回调/退款/打款/对账独立模块。
	// 支付概览聚合 5 个接口（channels/orders/callbacks/payouts/refunds），
	// 口径与 router.meta.permission 统一为 payment:order（doc102 M3-1）。
	"/payment/overview":  "payment:order",
	"/payment/channels":  "payment:channel",
	"/payment/methods":   "payment:method",
	"/payment/orders":    "payment:order",
	"/payment/callbacks": "payment:callback",
	"/payment/refunds":   "payment:refund",
	"/payment/payouts":   "payment:payout",
	"/payment/recon":     "payment:recon",

	"/tickets/list":       "ticket:list",
	"/tickets/categories": "ticket:category",
	"/tickets/stats":      "ticket:stats",
	// 复核中心（S3 双人复核）：与回复复核同一权限码。
	"/tickets/reviews": "ticket:review",

	// 销售中心（S1/S4–S6）：客户归属 / 提成台账 / 提成审核 / 业绩排行。
	// 提成审核页同时服务主管审核，菜单权限取审核码；销售的自助入口在台账页按钮级，
	// 后端 GET /sales/withdrawals 仍用 sales:commission:list 放行本人查询（doc86 §5 注 2）。
	"/sales/customers":   "sales:customer:list",
	"/sales/commissions": "sales:commission:list",
	"/sales/withdrawals": "sales:commission:audit",
	"/sales/performance": "sales:performance:view",

	"/system/roles":       "system:role:list",
	"/system/permissions": "system:permission:view",
	"/system/admins":      "staff:list",
	"/system/departments": "department:list",
	"/system/menus":       "system:menu",
	"/system/config":      "system:config:view",
	"/system/captcha":     "captcha:config",
	"/system/audit-logs":  "security:audit:list",
	// 日志中心（doc92 §9.1）：日志含手机号/邮箱/上游请求体，仅超管与运维可见。
	"/system/logs":         "log:center",
	"/system/logs/cleanup": "log:cleanup",
	"/system/logs/policy":  "log:policy",

	"/lifecycle/expiring": "lifecycle:expiring",
	"/lifecycle/renewals": "lifecycle:renewals",
	"/lifecycle/policy":   "lifecycle:policy",

	"/notification/records":       "notify:record",
	"/notification/templates":     "notify:template",
	"/notification/channels":      "notify:channel",
	"/notification/sms-templates": "notify:sms-template",
	"/notification/broadcast":     "notify:broadcast",
	"/notification/deliveries":    "notify:delivery",

	// 积分中心（doc36）：独立账本，规则/账户/流水三个叶子。
	"/points/overview":     "point:account",
	"/points/rules":        "point:rule",
	"/points/accounts":     "point:account",
	"/points/transactions": "point:transaction",
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
