package service

import (
	"errors"

	"hostsent/backend/internal/modules/admin/user/account/repository"
)

var (
	ErrUserNotFound  = errors.New("用户不存在")
	ErrUsernameTaken = errors.New("用户名已被占用")
	ErrEmailTaken    = errors.New("邮箱已被占用")
	// ErrLevelAssignUnavailable 装配层未注入等级调整能力，改等级请求无法处理。
	ErrLevelAssignUnavailable = errors.New("用户等级调整能力未配置")
	// ErrLevelNotFound 目标等级不存在。
	ErrLevelNotFound = errors.New("用户等级不存在")
	// ErrLevelDisabled 目标等级已停用。
	ErrLevelDisabled = errors.New("目标等级已停用，不能指派")
	// ErrAgentLevelUnavailable 装配层未注入代理等级校验能力，改代理等级请求无法处理。
	ErrAgentLevelUnavailable = errors.New("代理等级能力未配置")
	// ErrAgentLevelNotFound 目标代理等级不存在。
	ErrAgentLevelNotFound = errors.New("代理等级不存在")
	// ErrAgentLevelDisabled 目标代理等级已停用。
	ErrAgentLevelDisabled  = errors.New("目标代理等级已停用，不能指派")
	ErrRoleScopeMismatch   = errors.New("角色不属于该账号域，不能跨域分配")
	ErrEmptyRoleList       = errors.New("角色列表不能为空")
	ErrInvalidAdjustAmount = errors.New("调账金额不能为 0")
	ErrUserDeleted         = errors.New("用户已注销")
	// ErrUserNotDeleted 目标用户未注销（恢复一个没注销的用户）。
	ErrUserNotDeleted = errors.New("用户未注销，无需恢复")
	// ErrDeletionBlocked 存在硬阻断项（在管实例），force 也绕不过。
	ErrDeletionBlocked = errors.New("用户仍存在未释放的实例，无法注销")
	// ErrDeletionNeedsForce 存在警告项（余额/未结账单/工单/订单），需 force=true。
	ErrDeletionNeedsForce = errors.New("用户存在未结清事项，需确认后强制注销")
	// ErrInvalidOperator 注销操作人缺失（无法追溯「谁注销的」）。
	ErrInvalidOperator = errors.New("注销操作人不能为空")
	// ErrRestoreConflict 恢复时该账号名/邮箱已被新注册用户占用（doc104 §4.2）。
	//
	// 注销期间 username/email 是「释放」的（部分唯一索引只约束未注销行），
	// 别人可能已经用同样的账号名注册了。此时**必须明确失败**，不做静默改名——
	// 静默改名会让「这条审计记录对应的是谁」变得不可追溯。
	ErrRestoreConflict = errors.New("该用户名或邮箱已被占用，请先修改后再恢复")
	// ErrUserHasInstances 是 repository 层同义哨兵的 service 再出口：
	// handler 只允许依赖本包判断语义（CONTRIBUTING §3）。
	ErrUserHasInstances = repository.ErrUserHasInstances
)
