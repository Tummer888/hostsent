package auth

// 客户侧（用户中心）权限码内置枚举（P4-02）。
//
// 设计要点（见 docs/实施计划/81-账号体系与权限分级重构设计.md §4.6）：
//   - 权限码是固定枚举而非数据库行，避免主账号自行造权；
//   - 子账号只持有「业务操作」类权限，资金进出与实名永不授予；
//   - 主账号（IsSub=false）不走权限码校验，直接拥有全部客户侧权限。
const (
	PermInstanceView     = "instance:view"     // 查看实例
	PermInstanceOperate  = "instance:operate"  // 电源、重装、VNC 等实例操作
	PermOrderView        = "order:view"        // 查看订单
	PermOrderCreate      = "order:create"      // 下单/续费
	PermTicketView       = "ticket:view"       // 查看工单
	PermTicketSubmit     = "ticket:submit"     // 提交/回复工单
	PermBillingView      = "billing:view"      // 查看余额、账单、流水
	PermSubAccountManage = "subaccount:manage" // 管理成员，仅主账号
)

// UserPermissionAll 全部客户侧权限码，供前端渲染开关与后端校验白名单使用。
var UserPermissionAll = []string{
	PermInstanceView,
	PermInstanceOperate,
	PermOrderView,
	PermOrderCreate,
	PermTicketView,
	PermTicketSubmit,
	PermBillingView,
}

// UserPermissionDefault 新建成员时默认勾选的权限码（不含资金与成员管理）。
var UserPermissionDefault = []string{
	PermInstanceView,
	PermInstanceOperate,
	PermOrderView,
	PermOrderCreate,
	PermTicketView,
	PermTicketSubmit,
	PermBillingView,
}

// UserPermissionDenied 永不授予子账号的权限码：资金进出、实名、成员管理。
// 即便请求体伪造了这些码，保存时也会被过滤。
var UserPermissionDenied = []string{
	"billing:recharge",
	"billing:withdraw",
	"billing:refund",
	"realname:submit",
	"realname:update",
	PermSubAccountManage,
}

// IsGrantableUserPermission 判断权限码是否可授予子账号。
func IsGrantableUserPermission(code string) bool {
	for _, denied := range UserPermissionDenied {
		if code == denied {
			return false
		}
	}
	for _, allowed := range UserPermissionAll {
		if code == allowed {
			return true
		}
	}
	return false
}

// UserPermissionLabels 权限码 → 中文标签，供管理端与用户中心展示。
var UserPermissionLabels = map[string]string{
	PermInstanceView:     "查看实例",
	PermInstanceOperate:  "实例操作",
	PermOrderView:        "查看订单",
	PermOrderCreate:      "下单/续费",
	PermTicketView:       "查看工单",
	PermTicketSubmit:     "提交工单",
	PermBillingView:      "查看账单",
	PermSubAccountManage: "成员管理",
}
