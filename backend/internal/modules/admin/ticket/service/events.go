package service

// 工单通知事件常量。与 notification 模块的 notifymodel.Event* 保持一致，
// 这里在工单包内重复声明，避免 ticket → notification 的编译期耦合（P2-04 用窄接口解耦）。
const (
	notifEventTicketReplied       = "ticket_replied"
	notifEventTicketAssigned      = "ticket_assigned"
	notifEventTicketStatus        = "ticket_status"
	notifEventTicketReviewPending = "ticket_review_pending" // 有待复核回复（S3，通知复核人）
	notifEventTicketReviewResult  = "ticket_review_result"  // 复核结果（S3，通知原提交客服）
	notifEventTicketTransferred   = "ticket_transferred"    // 工单转交专人（S2，通知用户）
)
