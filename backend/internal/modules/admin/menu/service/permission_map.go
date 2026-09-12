package service

import (
	"hostsent/backend/internal/modules/admin/menu/dto"
	appauth "hostsent/backend/internal/pkg/auth"
)

// menuPermissionMap 菜单路径 → 所需权限码。
// 后端按当前管理员权限过滤菜单后再返回，前端不做权限过滤（否则等于把权限表交给客户端，82 P1-13）。
// 未登记的路径视为无需权限（如仪表盘），保证不会误隐藏。
var menuPermissionMap = map[string]string{
	"/dashboard":      "",
	"/dashboard/base": "",

	"/users/overview":        "system:user:list",
	"/users/accounts/list":   "system:user:list",
	"/users/accounts/groups": "user:group:list",

	"/users/security/login-logs": "security:login-log:list",
	"/users/security/audit-logs": "security:audit:list",
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

	"/resource/dashboard":    "resource:provider",
	"/resource/sync-monitor": "resource:sync",
	"/resource/providers":    "resource:provider",
	// 双链路拆页（本轮 S1）：自营平台对接与上游转售渠道同权限口径。
	"/resource/platforms": "resource:provider",
	"/resource/pools":     "resource:provider",
	// 连接测试页已下线（连通性内联到渠道列表行内操作）；旧路径仅作书签兼容。
	"/resource/connectivity": "resource:provider",
	// T3.6 合并页「同步与调度」（调度/任务/日志/差异/待确认调价）。
	"/resource/sync-center":    "resource:sync",
	"/resource/sync":           "resource:sync",
	"/resource/logs":           "sync:log",
	"/resource/reconciliation": "resource:sync",
	"/resource/products":       "resource:product",
	"/resource/product-sync":   "product:sync",
	"/resource/pricing":        "product:update_price",
	"/resource/instances":      "resource:instance",
	"/instances":               "resource:instance",
	"/instances/list":          "resource:instance",
	"/resource/api-test":       "resource:provider",
	"/resource/anomalies":      "resource:instance",
	// 任务队列（本轮 S3）：平台动作是否到达上游，沿用同步权限口径。
	"/resource/task-queue": "resource:sync",
	// 实例对账（本轮 S4）：本地实例与上游成本/账期比对。
	"/resource/reconcile": "resource:instance",
	"/resource/settings":  "system:config:view",

	"/product/products": "product:list",
	// T7.2 商品对接（doc16 §9.3）：组件复用资源侧页面，权限沿用资源商品口径。
	"/product/upstream":             "resource:product",
	"/product/cost-pricing":         "product:update_price",
	"/product/spec/templates":       "spec:template:list",
	"/product/spec/custom":          "spec:template:list",
	"/product/spec/mappings":        "spec:mapping:list",
	"/product/pricing":              "pricing:list",
	"/product/pricing/calculator":   "pricing:list",
	"/product/pricing/history":      "pricing:list",
	"/product/promotion/coupons":    "promotion:coupon:list",
	"/product/promotion/activities": "promotion:activity:list",
	"/product/promotion/bundles":    "promotion:activity:list",
	"/product/promotion/recommends": "promotion:activity:list",
	"/product/categories":           "product:category",
	// T7.4：/product/sync/* 页面已下线（redirect 到资源管理「同步与调度」），映射一并移除。

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
	"/finance/report":           "finance:wallet",
	"/finance/config":           "finance:wallet",

	"/tickets/list":       "ticket:list",
	"/tickets/categories": "ticket:category",
	"/tickets/stats":      "ticket:stats",

	"/system/menus":         "system:menu",
	"/system/roles":         "system:role:list",
	"/system/permissions":   "system:permission:view",
	"/system/admins":        "staff:list",
	"/system/config":        "system:config:view",
	"/system/audit-logs":    "security:audit:list",
	"/system/announcements": "notify:announcement",

	"/lifecycle/expiring": "lifecycle:expiring",
	"/lifecycle/renewals": "lifecycle:renewals",
	"/lifecycle/policy":   "lifecycle:policy",

	"/notification/records":   "notify:record",
	"/notification/templates": "notify:template",
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
