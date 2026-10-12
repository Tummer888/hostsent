// 用户中心认证域装配（批次四 4b）。

package server

import (
	"errors"
	finaccountservice "hostsent/backend/internal/modules/admin/finance/account/service"
	finbillservice "hostsent/backend/internal/modules/admin/finance/bill/service"
	usercenterhandler "hostsent/backend/internal/modules/uc/auth/handler"
	usercenterrepo "hostsent/backend/internal/modules/uc/auth/repository"
	usercenterservice "hostsent/backend/internal/modules/uc/auth/service"
	userfinancehandler "hostsent/backend/internal/modules/uc/finance/handler"
	usermenuhandler "hostsent/backend/internal/modules/uc/menu/handler"
	usermenurepo "hostsent/backend/internal/modules/uc/menu/repository"
	usermenuservice "hostsent/backend/internal/modules/uc/menu/service"
	apperrors "hostsent/backend/internal/pkg/errors"
)

// buildUCAuth 构造用户中心认证/菜单与用户端财务处理器。
func (a *asm) buildUCAuth() {
	// 用户中心模块：独立的数据访问、认证服务与处理器（与后台管理模块解耦）
	a.userCenterRepo = usercenterrepo.NewUserRepository(a.database)
	a.userCenterService = usercenterservice.NewAuthService(a.userCenterRepo, a.jwtIssuer, a.logger)
	// 用户侧撤销（登出/改密）同样要失效校验缓存，与管理员踢人走同一条失效通道。
	a.userCenterService.SetSessionInvalidator(a.sessionGuard)
	// 默认用户组兜底：用户自助注册未指定分组时归入 is_default 组。
	a.userCenterService.SetDefaultGroupResolver(a.userGroupService)
	a.userCenterAuthHandler = usercenterhandler.NewAuthHandler(a.userCenterService)
	// 用户中心菜单：复用 menus 表（platform=user），独立 DTO 对齐前端驼峰字段
	a.userMenuRepo = usermenurepo.NewMenuRepository(a.database)
	a.userMenuService = usermenuservice.NewMenuService(a.userMenuRepo)
	a.userMenuHandler = usermenuhandler.NewMenuHandler(a.userMenuService)
	// 用户中心财务：复用账务核心/充值/账单服务，仅暴露用户自助接口；
	// 错误码映射由本装配层提供（uc 不依赖 admin 的错误变量）。
	a.userFinanceHandler = userfinancehandler.NewFinanceHandler(a.walletService, a.rechargeService, a.billService,
		func(err error) *apperrors.AppError {
			switch {
			case errors.Is(err, finaccountservice.ErrWalletNotFound):
				return apperrors.New(20002, err.Error())
			case errors.Is(err, finaccountservice.ErrInsufficientBalance):
				return apperrors.New(30001, err.Error())
			case errors.Is(err, finaccountservice.ErrStatusConflict):
				return apperrors.New(20003, err.Error())
			// 开票（doc36 §3.3）：不可开票/已开票/已有待处理申请都是可预期的用户操作反馈，
			// 归为业务错误（20001）让前端按提示语展示，而不是落到 50001 通用失败。
			case errors.Is(err, finbillservice.ErrBillNotFound):
				return apperrors.New(20002, err.Error())
			case errors.Is(err, finbillservice.ErrBillNotInvoicable),
				errors.Is(err, finbillservice.ErrBillZeroAmountNotInvoicable),
				errors.Is(err, finbillservice.ErrAlreadyInvoiced),
				errors.Is(err, finbillservice.ErrInvoicePending),
				errors.Is(err, finbillservice.ErrInvoiceNotFound),
				errors.Is(err, finbillservice.ErrInvoiceFileNotReady),
				errors.Is(err, finbillservice.ErrInvoiceStatusConflict):
				return apperrors.New(20001, err.Error())
			// 邮件下发为预埋能力：本轮明确返回「未接入」，前端按提示语提示人工处理。
			case errors.Is(err, finbillservice.ErrInvoiceEmailNotReady):
				return apperrors.New(40001, err.Error())
			default:
				return apperrors.New(50001, err.Error())
			}
		},
	)
}
