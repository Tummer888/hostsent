package model

import "time"

// StandardInstanceStatus 统一实例状态
type StandardInstanceStatus string

const (
	InstanceStatusCreating   StandardInstanceStatus = "creating"
	InstanceStatusRunning    StandardInstanceStatus = "running"
	InstanceStatusStopped    StandardInstanceStatus = "stopped"
	InstanceStatusRestarting StandardInstanceStatus = "restarting"
	InstanceStatusError      StandardInstanceStatus = "error"
	InstanceStatusDeleting   StandardInstanceStatus = "deleting"
	InstanceStatusDeleted    StandardInstanceStatus = "deleted"
	// InstanceStatusCancelled 新购订单未付款、超时或被取消导致这台机器从未开通。
	//
	// 与 deleted 的区别（两者都不是"机器没了"的终态，但来路完全不同）：
	//   · deleted  —— 机器**开通过**，后来被销毁（客户退订/到期未续费/管理员终止）。
	//                 渠道侧曾有一台真机器在跑，销毁时通知了渠道。
	//   · cancelled—— 机器**从未存在**：订单没付款就过期（或客户主动取消），
	//                 履约从未执行。渠道侧从来没有任何资源，因此不涉及"终止"。
	// 分开显示的价值：运营看列表能区分"这台跑过又没了"与"这单根本没成交"，
	// 前者要查退订/到期链路，后者要去订单侧看流失原因。
	InstanceStatusCancelled StandardInstanceStatus = "cancelled"
)

// 暂停原因分类（instances.suspend_reason，迁移 114）。
//
// 为什么需要分类而不是只有自由文本：界面要按原因给**不同的下一步**——
// 到期暂停说"续费即恢复"，违规暂停说"联系客服处理"，两者的动作完全不同。
// 靠匹配自由文本来分类既脆弱又会被上游/文案改写破坏。
const (
	// SuspendReasonExpired 到期未续费，被生命周期推进器自动暂停。
	SuspendReasonExpired = "expired"
	// SuspendReasonViolation 管理员因违规暂停（客户需联系客服，不是续费能解决的）。
	SuspendReasonViolation = "violation"
	// SuspendReasonManual 管理员因其他原因暂停（工单、风控复核、客户申请等）。
	SuspendReasonManual = "manual"
)

// IsValidSuspendReason 校验暂停原因分类取值。
func IsValidSuspendReason(r string) bool {
	switch r {
	case SuspendReasonExpired, SuspendReasonViolation, SuspendReasonManual:
		return true
	}
	return false
}

// StandardInstance 统一实例
type StandardInstance struct {
	ID           uint                   `json:"id"`
	ProviderID   uint                   `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	UpstreamID   string                 `json:"upstream_id"`
	Name         string                 `json:"name"`
	UserID       uint                   `json:"user_id"`
	ProductID    uint                   `json:"product_id"`
	Specs        StandardProductSpec    `json:"specs"`
	Status       StandardInstanceStatus `json:"status"`
	PrivateIP    string                 `json:"private_ip"`
	PublicIP     string                 `json:"public_ip"`
	Region       string                 `json:"region"`
	Zone         string                 `json:"zone"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpireAt     time.Time              `json:"expire_at"`
	RawData      map[string]interface{} `json:"raw_data"`
}
