package server

import (
	"github.com/gin-gonic/gin"

	appauth "hostsent/backend/internal/pkg/auth"
)

// registerUCRoutes 注册用户中心（uc）全部路由：认证/实名/OAuth 授权/菜单/财务/积分/
// 支付回调/推广/商品/订单/实例/工单/子账号/生命周期/消息中心，
// 以及 /api/v1/auth、/api/v1/menus 两个历史兼容前缀。
// 自 router.go 原样拆出（批次四 4a），组内注册顺序与拆分前保持一致。
func registerUCRoutes(r *gin.Engine, app *App) {
	// 用户中心（普通用户自助）：独立模块 internal/modules/uc/auth
	ucAuth := r.Group("/api/v1/uc/auth")
	{
		ucAuth.POST("/login", app.userCenterAuthHandler.Login)                                                                            // 登录
		ucAuth.POST("/login/verify-otp", app.userCenterAuthHandler.VerifyLoginOTP)                                                        // 登录二次验证
		ucAuth.POST("/register", app.userCenterAuthHandler.Register)                                                                      // 注册
		ucAuth.POST("/forgot-password", app.userCenterAuthHandler.ForgotPassword)                                                         // 忘记密码（下发验证码）
		ucAuth.POST("/reset-password", app.userCenterAuthHandler.ResetPassword)                                                           // 重置密码
		ucAuth.POST("/logout", app.userAuth(), app.userCenterAuthHandler.Logout)                                                          // 登出
		ucAuth.GET("/userinfo", app.userAuth(), app.userCenterAuthHandler.UserInfo)                                                       // 用户信息
		ucAuth.PUT("/profile", app.userAuth(), app.userCenterAuthHandler.UpdateProfile)                                                   // 更新资料（改手机/邮箱时按字段动态要求验证）
		ucAuth.PUT("/password", app.userAuth(), app.userRequireVerification("password_change"), app.userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 用户端实名认证（doc104 §5.7）：状态查询、提交、本人申请历史与三方核验跳转。
	// 实名绑定的是自然人主体，子账号不具备独立实名资格，故整体挂 rejectSub 硬拒绝；
	// 服务层再以 real_name_verified_at 为唯一信任信号，不受展示名影响。
	ucVerification := r.Group("/api/v1/uc/verification")
	ucVerification.Use(app.userAuth(), app.rejectSub())
	{
		ucVerification.GET("", app.verification.userHandler.Status)
		ucVerification.POST("", app.verification.userHandler.Submit)
		ucVerification.GET("/applications", app.verification.userHandler.ListMine)
		ucVerification.GET("/applications/:id", app.verification.userHandler.Detail)
		ucVerification.POST("/applications/:id/authorize", app.verification.userHandler.Authorize)
		// 材料上传/下载（doc104 §5.7）：走鉴权端点而不是暴露存储路径 ——
		// 证件影像属敏感个人信息，只有申请本人与其审核员可取。
		ucVerification.POST("/applications/:id/documents", app.verification.userHandler.UploadDocument)
		ucVerification.GET("/documents/:id/download", app.verification.userHandler.DownloadDocument)
	}

	// 实名三方核验回调：用户在支付宝侧完成核验后被重定向回来，此时还没有平台令牌，
	// 因此这条必须在免登录路径上；信任来源是 provider 侧的 certify_id + 申请单上的
	// ProviderTxnNo 比对（服务层校验），不依赖任何客户端参数。
	ucVerificationCallback := r.Group("/api/v1/uc/verification")
	{
		ucVerificationCallback.GET("/:provider/callback", app.verification.userHandler.ProviderCallback)
	}

	// 用户中心菜单：普通用户控制台侧边栏（platform=user）
	ucMenu := r.Group("/api/v1/uc/menus")
	ucMenu.Use(app.userAuth())
	{
		ucMenu.GET("/tree", app.userMenuHandler.Tree) // 菜单树
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /auth 路径（同处理器，避免前端改动）
	ucAuthCompat := r.Group("/api/v1/auth")
	{
		ucAuthCompat.POST("/login", app.userCenterAuthHandler.Login)
		ucAuthCompat.POST("/login/verify-otp", app.userCenterAuthHandler.VerifyLoginOTP) // 登录二次验证
		ucAuthCompat.POST("/register", app.userCenterAuthHandler.Register)
		ucAuthCompat.POST("/forgot-password", app.userCenterAuthHandler.ForgotPassword) // 忘记密码
		ucAuthCompat.POST("/reset-password", app.userCenterAuthHandler.ResetPassword)   // 重置密码
		ucAuthCompat.POST("/logout", app.userAuth(), app.userCenterAuthHandler.Logout)
		ucAuthCompat.GET("/userinfo", app.userAuth(), app.userCenterAuthHandler.UserInfo)
		ucAuthCompat.PUT("/profile", app.userAuth(), app.userCenterAuthHandler.UpdateProfile)                                                   // 更新资料
		ucAuthCompat.PUT("/password", app.userAuth(), app.userRequireVerification("password_change"), app.userCenterAuthHandler.ChangePassword) // 修改密码
	}

	// 兼容 frontend-user 项目 baseURL=/api/v1 时的 /menus/tree 路径（同处理器）
	ucMenuCompat := r.Group("/api/v1/menus")
	ucMenuCompat.Use(app.userAuth())
	{
		ucMenuCompat.GET("/tree", app.userMenuHandler.Tree) // 菜单树
	}

	// 用户中心财务：用户自助查看余额 / 流水 / 账单并发起充值
	// 子账号可看账单（billing:view），但充值属资金入口，硬编码拒绝（P4-06）。
	ucFinance := r.Group("/api/v1/uc/finance")
	{
		ucFinance.GET("/balance", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Balance)           // 我的余额
		ucFinance.GET("/transactions", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Transactions) // 我的资金流水
		ucFinance.POST("/recharge", app.userAuth(), app.rejectSub(), app.userFinanceHandler.CreateRecharge)                        // 发起充值（子账号拒绝）
		ucFinance.GET("/recharges", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Recharges)       // 我的充值单（doc34 F-07）
		ucFinance.GET("/bills", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.Bills)               // 我的账单
		// 发票（doc36 §3.3）：查看与申请均属账单域，子账号可读可申请（不涉及资金出入）。
		ucFinance.GET("/invoices", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.MyInvoices) // 我的发票申请
		ucFinance.POST("/invoices", app.userAuth(), app.rejectSub(), app.userFinanceHandler.ApplyInvoice)                    // 申请开票（子账号拒绝）
		// 发票取件与邮件下发（doc36 §3.3 预埋）：下载已可用（取文件地址），邮件下发本轮返回明确未接入。
		ucFinance.GET("/invoices/:id/download", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.InvoiceDownload)
		ucFinance.POST("/invoices/:id/email", app.userAuth(), app.userPerm(appauth.PermBillingView), app.userFinanceHandler.InvoiceEmail)
		// 注：原未鉴权充值回调 POST /uc/finance/recharge/callback 已下线（doc34 F-01）。
		// 充值到账改由支付中心统一回调 /api/v1/payment/notify/:channel_code 验签后驱动。
	}

	// 用户中心支付：收银台 / 支付方式偏好 / 收款账户 / 提现申请（子账号拒绝资金入口）。
	ucPayment := r.Group("/api/v1/uc/payment")
	{
		// 查看类（账单域权限即可）
		ucPayment.GET("/methods", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Methods)
		ucPayment.GET("/preferences", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Preferences)
		ucPayment.PUT("/preferences", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.SavePreferences)
		ucPayment.GET("/accounts", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Accounts)
		ucPayment.GET("/withdrawals", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Withdrawals)
		ucPayment.GET("/orders/:payment_no", app.userAuth(), app.userPerm(appauth.PermBillingView), app.payment.userPaymentHandler.Order)
		// 资金入口（子账号硬拒绝，P4-06）
		ucPayment.POST("/accounts", app.userAuth(), app.rejectSub(), app.payment.userPaymentHandler.CreateAccount)
		ucPayment.PUT("/accounts/:id/default", app.userAuth(), app.rejectSub(), app.payment.userPaymentHandler.SetDefaultAccount)
		ucPayment.POST("/recharge", app.userAuth(), app.rejectSub(), app.payment.userPaymentHandler.Recharge)
		ucPayment.POST("/bills/pay", app.userAuth(), app.rejectSub(), app.payment.userPaymentHandler.PayBill)
		ucPayment.POST("/withdrawals", app.userAuth(), app.rejectSub(), app.userRequireVerification("withdraw_apply"), app.payment.userPaymentHandler.Withdraw)
	}

	// 用户中心积分（doc36）：只读展示，积分不可抵扣、不可提现、不可转入余额。
	// 复用账单域权限（billing:view），子账号可读（积分归主账号）。
	ucPoints := r.Group("/api/v1/uc/points")
	{
		ucPoints.GET("", app.userAuth(), app.userPerm(appauth.PermBillingView), app.point.userHandler.Overview)                  // 我的积分
		ucPoints.GET("/transactions", app.userAuth(), app.userPerm(appauth.PermBillingView), app.point.userHandler.Transactions) // 我的积分流水
	}

	// 支付渠道异步回调（免登录，唯一信任来源为渠道验签；取代原未鉴权充值回调）。
	paymentNotify := r.Group("/api/v1/payment/notify")
	{
		paymentNotify.POST("/:channel_code", app.payment.notifyHandler.Notify)
	}

	// 用户中心推广返现：查看邀请码/邀请人/返现明细属账单域，申请提现与转入余额为资金入口（子账号拒绝）。
	ucReferral := r.Group("/api/v1/uc/referral")
	{
		ucReferral.GET("/profile", app.userAuth(), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Profile)                                       // 我的推广概览
		ucReferral.GET("/invitees", app.userAuth(), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Invitees)                                     // 我的邀请
		ucReferral.GET("/cashbacks", app.userAuth(), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Cashbacks)                                   // 返现明细
		ucReferral.GET("/withdrawals", app.userAuth(), app.userPerm(appauth.PermBillingView), app.ucReferralHandler.Withdrawals)                               // 我的提现记录
		ucReferral.POST("/withdrawals", app.userAuth(), app.rejectSub(), app.userRequireVerification("withdraw_apply"), app.ucReferralHandler.ApplyWithdrawal) // 申请提现
		ucReferral.POST("/transfer", app.userAuth(), app.rejectSub(), app.ucReferralHandler.Transfer)                                                          // 转入现金余额
	}

	// 用户中心商品：上架商品公开可浏览（无需登录）
	ucProducts := r.Group("/api/v1/uc/products")
	{
		ucProducts.GET("", app.ucProductHandler.List)
		ucProducts.GET("/:id", app.ucProductHandler.Get)
	}

	// 用户端安全设置（doc91 §4.4/§4.5）：需登录；子账号也能看自己的设置。
	ucSecurity := r.Group("/api/v1/uc/security")
	ucSecurity.Use(app.userAuth(), app.userAudit())
	{
		ucSecurity.GET("/settings", app.captcha.userHandler.GetSettings)
		ucSecurity.PUT("/settings", app.captcha.userHandler.UpdateSettings)
		ucSecurity.POST("/verification/send", app.captcha.userHandler.SendVerification)
		ucSecurity.POST("/verification/verify", app.captcha.userHandler.VerifyCode)
		ucSecurity.GET("/verification/requirement", app.captcha.userHandler.OpenVerification)
	}

	// 用户中心订单：下单（余额支付即时开通 / 渠道支付落待支付单）+ 我的订单（需登录）
	// 子账号下单需 order:create，查看需 order:view（P4-06）。
	ucOrders := r.Group("/api/v1/uc/orders")
	ucOrders.Use(app.userAuth(), app.userAudit())
	{
		ucOrders.POST("", app.userPerm(appauth.PermOrderCreate), app.ucOrderHandler.Create)
		ucOrders.GET("", app.userPerm(appauth.PermOrderView), app.ucOrderHandler.List)
		// 预结算只读算价，子账号可看（P5-05，需求 order:view）。
		ucOrders.POST("/quote", app.userPerm(appauth.PermOrderView), app.ucOrderHandler.Quote)
		// 取消待支付订单：状态作废而非资金进出，与下单同权限（子账号可操作）。
		ucOrders.POST("/:id/cancel", app.userPerm(appauth.PermOrderCreate), app.ucOrderHandler.Cancel)
		// 发起收银台支付属资金支出（rejectSub）：子账号硬拒绝，与充值/提现同口径（P4-06）。
		ucOrders.POST("/:id/pay", app.userPerm(appauth.PermOrderCreate), app.rejectSub(), app.ucOrderHandler.Pay)
		// 订单详情：仅返回订单归属账号下的订单，他人订单按不存在处理。
		ucOrders.GET("/:id", app.userPerm(appauth.PermOrderView), app.ucOrderHandler.Detail)
	}

	// 用户中心主机管理：列表/详情/电源操作/VNC（需登录）
	// 查看需 instance:view，电源/VNC 需 instance:operate（P4-06）。
	ucInstances := r.Group("/api/v1/uc/instances")
	ucInstances.Use(app.userAuth(), app.userAudit())
	{
		ucInstances.GET("", app.userPerm(appauth.PermInstanceView), app.ucInstanceHandler.List)                // 我的主机列表
		ucInstances.GET("/:id", app.userPerm(appauth.PermInstanceView), app.ucInstanceHandler.Detail)          // 主机详情
		ucInstances.POST("/:id/power", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.Power) // 电源操作
		ucInstances.POST("/:id/vnc", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.VNC)     // 远程控制台
		// 维护类自助操作（重装/重置密码/救援/快照）：写入操作，需 instance:operate。
		// 重装允许格式化数据盘、快照恢复会覆盖系统盘 —— 两者按管理端口径加二次验证。
		ucInstances.POST("/:id/reinstall", app.userPerm(appauth.PermInstanceOperate), app.userRequireVerification("instance_reinstall"), app.ucInstanceHandler.Reinstall)
		ucInstances.POST("/:id/reset-password", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.ResetPassword)
		ucInstances.POST("/:id/rescue", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.Rescue)
		ucInstances.POST("/:id/exit-rescue", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.ExitRescue)
		ucInstances.GET("/:id/snapshots", app.userPerm(appauth.PermInstanceView), app.ucInstanceHandler.Snapshots)
		ucInstances.POST("/:id/snapshots", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.CreateSnapshot)
		ucInstances.DELETE("/:id/snapshots/:snapshotId", app.userPerm(appauth.PermInstanceOperate), app.ucInstanceHandler.DeleteSnapshot)
		ucInstances.POST("/:id/snapshots/restore", app.userPerm(appauth.PermInstanceOperate), app.userRequireVerification("instance_snapshot_restore"), app.ucInstanceHandler.RestoreSnapshot)
		// 自助销毁（doc91 §6.4）：不可逆，子账号硬拒绝 + instance_destroy 二次验证票据。
		// 默认 otp_required=false，升级当天这条新路由不阻断任何人。
		ucInstances.DELETE("/:id", app.rejectSub(), app.userPerm(appauth.PermInstanceOperate), app.userRequireVerification("instance_destroy"), app.ucInstanceHandler.Destroy)
	}

	// 用户中心工单支持：我的工单自助管理（doc50）
	// 查看需 ticket:view，提交/回复需 ticket:submit（P4-06）。
	ucSupport := r.Group("/api/v1/uc/support")
	ucSupport.Use(app.userAudit())
	{
		ucSupport.GET("/tickets", app.userAuth(), app.userPerm(appauth.PermTicketView), app.userTicketHandler.List)                     // 我的工单列表
		ucSupport.POST("/tickets", app.userAuth(), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Create)                // 提交工单
		ucSupport.GET("/ticket-categories", app.userAuth(), app.userPerm(appauth.PermTicketView), app.userTicketHandler.ListCategories) // 可用工单分类
		ucSupport.GET("/tickets/:id", app.userAuth(), app.userPerm(appauth.PermTicketView), app.userTicketHandler.Get)                  // 工单详情（含回复）
		ucSupport.POST("/tickets/:id/replies", app.userAuth(), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Reply)     // 追加工单回复
		ucSupport.POST("/tickets/:id/cancel", app.userAuth(), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.Cancel)     // 取消工单
		// S2 附件：上传需提交权，下载需查看权；服务端再按工单归属与内部标记鉴权。
		ucSupport.POST("/tickets/:id/attachments", app.userAuth(), app.userPerm(appauth.PermTicketSubmit), app.userTicketHandler.UploadAttachment)
		ucSupport.GET("/attachments/:id/download", app.userAuth(), app.userPerm(appauth.PermTicketView), app.userTicketHandler.DownloadAttachment)
	}

	// 用户中心成员（子账号，P4-07）：仅主账号可管理，服务层再校验一次 IsSub。
	ucMembers := r.Group("/api/v1/uc/members")
	ucMembers.Use(app.userAuth(), app.userAudit())
	{
		ucMembers.GET("", app.memberHandler.List)                           // 我的成员列表
		ucMembers.POST("", app.memberHandler.Create)                        // 新建成员
		ucMembers.PUT("/:id", app.memberHandler.Update)                     // 改备注/状态
		ucMembers.PUT("/:id/permissions", app.memberHandler.SetPermissions) // 覆盖式设置权限
		ucMembers.DELETE("/:id", app.memberHandler.Delete)                  // 删除（禁用）成员
		ucMembers.GET("/:id/logs", app.memberHandler.ListLogs)              // 成员操作日志
	}

	// 用户中心生命周期与续费（doc60 §7.2）
	// 聚合视图 /uc/instances/renewals 为固定路径，需先于 /instances/:id 注册
	ucLifecycle := r.Group("/api/v1/uc")
	ucLifecycle.Use(app.userAudit())
	{
		ucLifecycle.GET("/instances/renewals", app.userAuth(), app.lifecycleUserHandler.RenewalsView)          // 续费管理聚合视图
		ucLifecycle.POST("/instances/:id/renew", app.userAuth(), app.lifecycleUserHandler.Renew)               // 手动续费
		ucLifecycle.PUT("/instances/:id/auto-renew", app.userAuth(), app.lifecycleUserHandler.ToggleAutoRenew) // 自动续费开关
		ucLifecycle.GET("/renewals", app.userAuth(), app.lifecycleUserHandler.Records)                         // 我的续费记录
		ucLifecycle.GET("/renewals/:id", app.userAuth(), app.lifecycleUserHandler.Detail)                      // 续费记录详情
	}

	// 用户中心消息中心（doc70 §7.2）
	ucNotify := r.Group("/api/v1/uc")
	{
		ucNotify.GET("/notifications/unread-count", app.userAuth(), app.notifyUserHandler.UserUnreadCount)
		ucNotify.GET("/notifications", app.userAuth(), app.notifyUserHandler.UserList)
		ucNotify.GET("/notifications/:id", app.userAuth(), app.notifyUserHandler.UserDetail)
		ucNotify.POST("/notifications/read-all", app.userAuth(), app.notifyUserHandler.UserReadAll)
		ucNotify.GET("/announcements", app.userAuth(), app.notifyUserHandler.UserAnnouncements)
		ucNotify.GET("/notification-preferences", app.userAuth(), app.notifyUserHandler.UserGetPrefs)
		ucNotify.PUT("/notification-preferences", app.userAuth(), app.notifyUserHandler.UserUpdatePrefs)
	}
}
