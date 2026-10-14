// 用户域装配（批次四 4b，合并原 assembly_level.go 与 assembly_agent_level.go）。

package server

import (
	"context"
	"errors"
	"fmt"
	accountdto "hostsent/backend/internal/modules/admin/finance/account/dto"
	finrechargedto "hostsent/backend/internal/modules/admin/finance/account/recharge/dto"
	finrechargemodel "hostsent/backend/internal/modules/admin/finance/account/recharge/model"
	menuhandler "hostsent/backend/internal/modules/admin/menu/handler"
	menuservice "hostsent/backend/internal/modules/admin/menu/service"
	"hostsent/backend/internal/modules/admin/user/account/handler"
	levelhandler "hostsent/backend/internal/modules/admin/user/account/level/handler"
	levelrepo "hostsent/backend/internal/modules/admin/user/account/level/repository"
	levelservice "hostsent/backend/internal/modules/admin/user/account/level/service"
	"hostsent/backend/internal/modules/admin/user/account/service"
	agentlevelhandler "hostsent/backend/internal/modules/admin/user/agentlevel/handler"
	agentlevelrepo "hostsent/backend/internal/modules/admin/user/agentlevel/repository"
	agentlevelservice "hostsent/backend/internal/modules/admin/user/agentlevel/service"
	securityhandler "hostsent/backend/internal/modules/admin/user/security/handler"
	securityrepo "hostsent/backend/internal/modules/admin/user/security/repository"
	securityservice "hostsent/backend/internal/modules/admin/user/security/service"
	"time"

	"gorm.io/gorm"
)

// buildUserDomain 构造员工/用户/角色/权限/菜单/安全/等级/代理等级域及其处理器。
func (a *asm) buildUserDomain() {
	a.levelRepo = levelrepo.NewUserLevelRepository(a.database)
	a.userService = service.NewUserService(
		a.userRepo,
		a.jwtIssuer,
		func(ctx context.Context, userID uint64, amount float64, remark string, operatorID uint64) (*service.RechargeResult, error) {
			// 正数走充值单链路：登记 → 立即确认到账。这样一笔人工代充值会同时
			// 留下充值单（财务「充值管理」可查、用户端「我的充值单」可见）与
			// 资金流水，并由充值服务在到账后归集当期账单。
			//
			// 此前正数也走 Adjust 人工调账，只写一条 type=adjust 的流水：
			// 财务查不到充值单、账单里也没有充值额，同一个人工操作在三个页面
			// 各说各话。Adjust 保留给负数（追回/纠正）——那才是真正的调账。
			if amount > 0 {
				adjusted := remark
				if adjusted == "" {
					adjusted = "人工充值"
				}
				info, err := a.rechargeService.CreateAndApprove(ctx, finrechargedto.RechargeCreateRequest{
					UserID: userID,
					Amount: amount,
					Method: finrechargemodel.RechargeMethodManual,
					Remark: adjusted,
				}, operatorID)
				if err != nil {
					return nil, err
				}
				return &service.RechargeResult{RechargeNo: info.RechargeNo}, nil
			}
			// 负数：扣减（追回/纠正），走人工调账。
			direction := -1
			amount = -amount
			adjusted := remark
			if adjusted == "" {
				adjusted = "人工扣减"
			}
			tr, err := a.walletService.Adjust(ctx, accountdto.AdjustRequest{
				UserID:    userID,
				Type:      "adjust",
				Direction: direction,
				Amount:    amount,
				BizKey:    fmt.Sprintf("admin-recharge-%d", userID) + fmt.Sprintf("-%d", time.Now().UnixNano()),
				Remark:    adjusted,
			}, operatorID)
			if err != nil {
				return nil, err
			}
			return &service.RechargeResult{TxNo: tr.TxNo}, nil
		},
	)
	a.userDetailService = service.NewUserDetailService(a.userRepo, a.userDetailRepo)
	// 用户注销 / 恢复 / 留存期清理（doc104 §4）。留存期从 system_configs 读，
	// 缺失或非法回落 180 天；读配置失败也回落，不让清理任务因配置中心异常而停摆。
	a.userDeletionService = service.NewDeletionService(a.userRepo, func(ctx context.Context, key string, fallback int) int {
		raw, ok, err := a.configValueReader(ctx, key)
		if err != nil || !ok {
			return fallback
		}
		return service.ParseRetentionDays(raw, fallback)
	})
	a.userPurgeScheduler = service.NewPurgeScheduler(a.userDeletionService, a.logger)
	a.userGroupService = service.NewUserGroupService(a.userGroupRepo)
	// 默认用户组兜底：后台建号（admin user）未指定分组时归入 is_default 组。
	a.userService.SetDefaultGroupProvider(a.userGroupService)
	// 代登录审计：补写 login_logs + user_sessions，让代登录在安全页可追溯、可强制下线。
	// 会话 expired_at 与令牌有效期对齐，否则在线用户里会挂着到期却仍 active 的会话。
	a.userService.SetLoginRecorder(newImpersonationRecorder(a.database, a.jwtIssuer.ExpireIn()))
	a.userService.SetLogger(a.logger)
	a.roleService = service.NewRoleService(a.roleRepo, a.permCache)
	// 跨域拦截：后台「新建用户 / 分配角色」只能挂客户角色（scope=user）。
	// 员工角色由 admin_roles 承载，绑到 users 上用户在两端都拿不到正确语义，
	// 而员工侧也完全不认这条绑定 —— 必须挡在服务层，不能只靠前端下拉过滤。
	a.userService.SetRoleScopeValidator(a.roleService)
	a.permissionService = service.NewPermissionService(a.permissionRepo)
	a.menuService = menuservice.NewMenuService(a.menuRepo)
	a.securityService = securityservice.NewSecurityService(a.securityRepo)
	// 强制下线要真正生效：撤销会话后立即失效 sessionguard 的校验缓存，
	// 否则被踢的令牌还能再用最多 positiveTTL（5 分钟）。
	a.securityService.SetSessionInvalidator(a.sessionGuard)
	a.sessionExpireScheduler = securityservice.NewSessionExpireScheduler(a.securityService, a.logger)
	// 风控（doc06 §4.3/§4.4）：规则引擎只写风险事件、绝不阻断登录；黑名单守卫会
	// 阻断，因此是独立组件、判定逐条可解释。两者都在装配层构造，经中性接口注入
	// 用户端与管理端登录链路 —— uc/auth 与 admin/manager 都不 import 安全模块。
	a.riskRepo = securityrepo.NewRiskRepository(a.database)
	a.riskEngine = securityservice.NewRiskEngine(a.riskRepo, a.configValueReader)
	a.blacklistGuard = securityservice.NewBlacklistGuard(a.riskRepo)
	a.userLevelService = levelservice.NewUserLevelService(a.levelRepo)
	// 代理等级（doc108）：折扣的唯一来源。用户组只做分类，不再绑定折扣策略。
	a.agentLevelRepo = agentlevelrepo.NewAgentLevelRepository(a.database)
	a.agentLevelService = agentlevelservice.NewAgentLevelService(a.agentLevelRepo)
	a.agentLevelHandler = agentlevelhandler.NewAgentLevelHandler(a.agentLevelService)
	// 商品分组 × 折扣组（doc108 §8I）：录入组织层，应用时展开进逐格矩阵。
	a.schemeRepo = agentlevelrepo.NewSchemeRepository(a.database)
	a.productGroupService = agentlevelservice.NewProductGroupService(a.schemeRepo)
	a.discountSchemeService = agentlevelservice.NewDiscountSchemeService(a.schemeRepo, a.agentLevelService)
	a.schemeHandler = agentlevelhandler.NewSchemeHandler(a.productGroupService, a.discountSchemeService)
	// 用户详情页/建号设代理等级：account 侧只声明端口，校验在 agentlevel 服务里。
	a.userService.SetAgentLevelValidator(newAgentLevelValidatorAdapter(a.agentLevelService))
	// 消费升级服务（P3-03）：订单支付成功后累加累计消费并重算等级（只升不降）。
	a.levelUpgradeService = levelservice.NewLevelUpgradeService(a.levelRepo)
	// 用户详情页改等级：account 侧只声明端口，等级校验与变更留痕都在 level 服务里。
	a.userService.SetLevelAssigner(newLevelAssignerAdapter(a.userLevelService))
	// 建号默认角色：后台建出来的客户号也要挂「普通用户」，与注册链路同口径。
	a.userService.SetDefaultUserRoleProvider(a.userRepo)
	a.userDetailHandler = handler.NewUserDetailHandler(a.userDetailService)
	a.userGroupHandler = handler.NewUserGroupHandler(a.userGroupService)
	a.roleHandler = handler.NewRoleHandler(a.roleService)
	a.permissionHandler = handler.NewPermissionHandler(a.permissionService)
	a.menuHandler = menuhandler.NewMenuHandler(a.menuService)
	a.securityHandler = securityhandler.NewSecurityHandler(a.securityService)
	a.userLevelHandler = levelhandler.NewUserLevelHandler(a.userLevelService)
	a.userDeletionHandler = handler.NewUserDeletionHandler(a.userDeletionService)
}

// levelAssignerAdapter 把 level 服务适配成 account 侧的 LevelAssigner。
type levelAssignerAdapter struct {
	levels levelservice.UserLevelService
}

// newLevelAssignerAdapter 创建等级调整适配器。
func newLevelAssignerAdapter(levels levelservice.UserLevelService) service.LevelAssigner {
	return &levelAssignerAdapter{levels: levels}
}

func (a *levelAssignerAdapter) AssignLevel(ctx context.Context, userID, levelID uint64) error {
	err := a.levels.AssignLevel(ctx, userID, levelID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return service.ErrLevelNotFound
	case errors.Is(err, levelservice.ErrLevelDisabled):
		return service.ErrLevelDisabled
	default:
		return err
	}
}

func (a *levelAssignerAdapter) DefaultLevelID(ctx context.Context) (uint64, error) {
	return a.levels.DefaultLevelID(ctx)
}

func (a *levelAssignerAdapter) CheckAssignable(ctx context.Context, levelID uint64) error {
	err := a.levels.CheckAssignable(ctx, levelID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return service.ErrLevelNotFound
	case errors.Is(err, levelservice.ErrLevelDisabled):
		return service.ErrLevelDisabled
	case errors.Is(err, levelservice.ErrInvalidAssignTarget):
		return service.ErrLevelNotFound
	default:
		return err
	}
}

// agentLevelValidatorAdapter 把 agentlevel 服务适配成 account 侧的 AgentLevelValidator。
type agentLevelValidatorAdapter struct {
	levels agentlevelservice.AgentLevelService
}

// newAgentLevelValidatorAdapter 创建代理等级校验适配器。
func newAgentLevelValidatorAdapter(levels agentlevelservice.AgentLevelService) service.AgentLevelValidator {
	return &agentLevelValidatorAdapter{levels: levels}
}

func (a *agentLevelValidatorAdapter) CheckAssignable(ctx context.Context, levelID uint64) error {
	err := a.levels.CheckAssignable(ctx, levelID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return service.ErrAgentLevelNotFound
	case errors.Is(err, agentlevelservice.ErrLevelDisabled):
		return service.ErrAgentLevelDisabled
	case errors.Is(err, agentlevelservice.ErrInvalidAssignTarget):
		return service.ErrAgentLevelNotFound
	default:
		return err
	}
}
