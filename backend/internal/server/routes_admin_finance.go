// 管理端路由——Finance域（自 routes_admin.go 拆分，批次三 C2）。

package server

import (
	"github.com/gin-gonic/gin"
)

func registerAdminFinanceRoutes(v1 *gin.RouterGroup, app *App) {
	financeGroup := v1.Group("/finance")
	financeGroup.Use(app.adminAuth())
	{
		// 财务统计与参数（本轮整理）：总览与报表共用同一聚合口径，避免各自前端累加；
		// 参数接口按白名单读写财务键，财务角色无需 system:config:*。
		financeGroup.GET("/stats", app.perm("finance:wallet"), app.financeStatsHandler.Stats)
		financeGroup.GET("/settings", app.perm("finance:config"), app.financeSettingsHandler.List)
		financeGroup.PUT("/settings", app.perm("finance:config"), app.financeSettingsHandler.Update)
		// 钱包/流水
		financeGroup.GET("/wallets/:user_id", app.perm("finance:wallet"), app.walletHandler.Balance)
		financeGroup.GET("/transactions", app.perm("finance:wallet"), app.walletHandler.ListTransactions)
		// 明细导出（CSV）：与列表同筛选条件，条数封顶 20000。
		financeGroup.GET("/transactions/export", app.perm("finance:wallet"), app.walletHandler.ExportTransactions)
		financeGroup.POST("/transactions/adjust", app.perm("finance:adjust"), app.walletHandler.Adjust)
		// 充值
		financeGroup.POST("/recharges", app.perm("finance:recharge"), app.rechargeHandler.Create)
		financeGroup.GET("/recharges", app.perm("finance:recharge"), app.rechargeHandler.List)
		financeGroup.POST("/recharges/:id/approve", app.perm("finance:recharge:approve"), app.rechargeHandler.Approve)
		// 提现
		financeGroup.GET("/withdrawals", app.perm("finance:withdraw"), app.withdrawHandler.List)
		financeGroup.POST("/withdrawals/:id/approve", app.perm("finance:withdraw:audit"), app.withdrawHandler.Approve)
		financeGroup.POST("/withdrawals/:id/reject", app.perm("finance:withdraw:audit"), app.withdrawHandler.Reject)
		// 打款闭环（doc34 F-02）：审核通过后登记打款完成/失败，成功才结算冻结资金。
		financeGroup.POST("/withdrawals/:id/mark-paid", app.perm("finance:withdraw:audit"), app.withdrawHandler.MarkPaid)
		financeGroup.POST("/withdrawals/:id/mark-failed", app.perm("finance:withdraw:audit"), app.withdrawHandler.MarkFailed)
		// 账单/对账
		financeGroup.GET("/bills", app.perm("finance:bill"), app.billHandler.List)
		// 手动生成账单（doc36 §7-2）：按用户 + 账期归集消费/退款并分类。
		financeGroup.POST("/bills/generate", app.perm("finance:bill:close"), app.billHandler.Generate)
		financeGroup.POST("/bills/:id/close", app.perm("finance:bill:close"), app.billHandler.Close)
		financeGroup.POST("/bills/recon", app.perm("finance:bill:recon"), app.reconHandler.Reconcile)
		// 发票管理（doc36 §3.3）：查看申请 → 开票/驳回；渠道与外部单号已预埋。
		financeGroup.GET("/invoices", app.perm("finance:invoice"), app.billHandler.Invoices)
		financeGroup.POST("/invoices/:id/issue", app.perm("finance:invoice:issue"), app.billHandler.IssueInvoice)
		financeGroup.POST("/invoices/:id/reject", app.perm("finance:invoice:issue"), app.billHandler.RejectInvoice)
	}
	costGroup := v1.Group("/cost")
	costGroup.Use(app.adminAuth())
	{
		// 成本总览：月度收入/成本/利润与利润率、成本构成、近 12 月趋势
		costGroup.GET("/overview", app.perm("finance:cost:overview"), app.costHandler.Overview)
		// 成本项配置（母机月费 / 人力 / 机房带宽…）
		costGroup.GET("/items", app.perm("finance:cost:item"), app.costHandler.ListItems)
		costGroup.POST("/items", app.perm("finance:cost:item"), app.costHandler.CreateItem)
		costGroup.PUT("/items/:id", app.perm("finance:cost:item"), app.costHandler.UpdateItem)
		costGroup.DELETE("/items/:id", app.perm("finance:cost:item"), app.costHandler.DeleteItem)
		// 上游余额台账（doc111 §5）：余额=自动抓取快照，消耗/充值=上游账本流水，全部自动取数
		costGroup.GET("/balances", app.perm("finance:cost:balance"), app.costHandler.Ledger)
		// 一键抓取渠道余额（适配器未实现 AccountReader 时返回 30008）
		costGroup.POST("/balances/fetch", app.perm("finance:cost:balance"), app.costHandler.FetchBalance)
		// 上游账本（doc111 §5.2）：消费/充值流水同步与明细、余额水位告警
		costGroup.POST("/balances/sync-ledger", app.perm("finance:cost:balance"), app.costHandler.SyncLedger)
		costGroup.GET("/balances/entries", app.perm("finance:cost:balance"), app.costHandler.ListLedgerEntries)
		costGroup.GET("/balances/alerts", app.perm("finance:cost:balance"), app.costHandler.BalanceAlerts)
	}
	paymentGroup := v1.Group("/payment")
	paymentGroup.Use(app.adminAuth())
	{
		// 渠道类型与渠道实例
		paymentGroup.GET("/channel-types", app.perm("payment:channel"), app.payment.channelHandler.ListTypes)
		paymentGroup.GET("/channels", app.perm("payment:channel"), app.payment.channelHandler.List)
		paymentGroup.POST("/channels", app.perm("payment:channel:manage"), app.payment.channelHandler.Create)
		paymentGroup.GET("/channels/:id", app.perm("payment:channel"), app.payment.channelHandler.Get)
		paymentGroup.PUT("/channels/:id", app.perm("payment:channel:manage"), app.payment.channelHandler.Update)
		paymentGroup.PATCH("/channels/:id/status", app.perm("payment:channel:manage"), app.payment.channelHandler.UpdateStatus)
		paymentGroup.POST("/channels/:id/test", app.perm("payment:channel"), app.payment.channelHandler.Test)
		// 支付单
		paymentGroup.GET("/orders", app.perm("payment:order"), app.payment.orderHandler.List)
		paymentGroup.GET("/orders/:id", app.perm("payment:order"), app.payment.orderHandler.Get)
		paymentGroup.POST("/orders/:id/confirm", app.perm("payment:order:operate"), app.payment.orderHandler.Confirm)
		paymentGroup.POST("/orders/:id/close", app.perm("payment:order:operate"), app.payment.orderHandler.Close)
		paymentGroup.POST("/orders/:id/sync", app.perm("payment:order:operate"), app.payment.orderHandler.Sync)
		// 回调日志
		paymentGroup.GET("/callbacks", app.perm("payment:callback"), app.payment.callbackHandler.List)
		// 渠道退款单
		paymentGroup.GET("/refunds", app.perm("payment:refund"), app.payment.refundHandler.List)
		// 打款单
		paymentGroup.GET("/payouts", app.perm("payment:payout"), app.payment.payoutHandler.List)
		paymentGroup.POST("/payouts/:id/mark-paid", app.perm("payment:payout:operate"), app.payment.payoutHandler.MarkPaid)
		paymentGroup.POST("/payouts/:id/retry", app.perm("payment:payout:operate"), app.payment.payoutHandler.Retry)
		// 渠道对账
		paymentGroup.GET("/recon", app.perm("payment:recon"), app.payment.reconHandler.List)
		paymentGroup.POST("/recon", app.perm("payment:recon"), app.payment.reconHandler.Reconcile)
		// 支付方式路由（场景可用渠道与优先级）
		paymentGroup.GET("/methods", app.perm("payment:method"), app.payment.methodHandler.Options)
	}
	pointGroup := v1.Group("/points")
	pointGroup.Use(app.adminAuth())
	{
		pointGroup.GET("/overview", app.perm("point:account"), app.point.adminHandler.Overview)
		pointGroup.GET("/rules", app.perm("point:rule"), app.point.adminHandler.Rules)
		pointGroup.POST("/rules", app.perm("point:rule:manage"), app.point.adminHandler.CreateRule)
		pointGroup.PUT("/rules/:id", app.perm("point:rule:manage"), app.point.adminHandler.UpdateRule)
		pointGroup.GET("/accounts", app.perm("point:account"), app.point.adminHandler.Accounts)
		pointGroup.GET("/accounts/:user_id", app.perm("point:account"), app.point.adminHandler.Account)
		pointGroup.POST("/accounts/adjust", app.perm("point:account:adjust"), app.point.adminHandler.Adjust)
		pointGroup.GET("/transactions", app.perm("point:transaction"), app.point.adminHandler.Transactions)
	}
	referralGroup := v1.Group("/referral")
	referralGroup.Use(app.adminAuth())
	{
		referralGroup.GET("/cashbacks", app.perm("referral:cashback:list"), app.adminReferralHandler.Cashbacks)
		referralGroup.GET("/invitees", app.perm("referral:cashback:list"), app.adminReferralHandler.Invitees)
		referralGroup.GET("/withdrawals", app.perm("referral:withdraw:list"), app.adminReferralHandler.Withdrawals)
		referralGroup.POST("/withdrawals/:id/approve", app.perm("referral:withdraw:audit"), app.adminReferralHandler.Approve)
		referralGroup.POST("/withdrawals/:id/reject", app.perm("referral:withdraw:audit"), app.adminReferralHandler.Reject)
	}
	salesGroup := v1.Group("/sales")
	salesGroup.Use(app.adminAuth())
	{
		salesGroup.GET("/customers", app.perm("sales:customer:list"), app.salesCustomerHandler.List)
		salesGroup.GET("/customers/unassigned", app.perm("sales:customer:assign"), app.salesCustomerHandler.Unassigned)
		salesGroup.POST("/customers/assign", app.perm("sales:customer:assign"), app.salesCustomerHandler.Assign)
		salesGroup.POST("/customers/release", app.perm("sales:customer:assign"), app.salesCustomerHandler.Release)
		salesGroup.GET("/customers/:userID/relations", app.perm("sales:customer:list"), app.salesCustomerHandler.Relations)
		salesGroup.GET("/sales-candidates", app.perm("sales:customer:assign"), app.salesCustomerHandler.Candidates)

		salesGroup.GET("/commissions", app.perm("sales:commission:list"), app.salesCommissionHandler.List)
		salesGroup.GET("/commissions/summary", app.perm("sales:commission:list"), app.salesCommissionHandler.Summary)
		salesGroup.GET("/withdrawals", app.perm("sales:commission:list"), app.salesCommissionHandler.Withdrawals)
		salesGroup.POST("/withdrawals", app.perm("sales:commission:list"), app.salesCommissionHandler.ApplyWithdrawal)
		salesGroup.POST("/withdrawals/:id/approve", app.perm("sales:commission:audit"), app.salesCommissionHandler.ApproveWithdrawal)
		salesGroup.POST("/withdrawals/:id/reject", app.perm("sales:commission:audit"), app.salesCommissionHandler.RejectWithdrawal)
		salesGroup.POST("/withdrawals/:id/pay", app.perm("sales:commission:settle"), app.adminRequireVerification("payout_apply"), app.salesCommissionHandler.PayWithdrawal)

		salesGroup.GET("/performance/ranking", app.perm("sales:performance:view"), app.salesPerformanceHandler.Ranking)
		salesGroup.GET("/performance/targets", app.perm("sales:performance:view"), app.salesPerformanceHandler.Targets)
		salesGroup.PUT("/performance/targets", app.perm("sales:performance:view"), app.salesPerformanceHandler.UpsertTargets)
		salesGroup.GET("/performance/me", app.perm("sales:commission:list"), app.salesPerformanceHandler.Me)
	}
}
