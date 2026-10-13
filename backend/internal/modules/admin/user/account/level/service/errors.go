package service

import (
	"errors"

	"hostsent/backend/internal/modules/admin/user/account/level/repository"
)

var (
	ErrInvalidAssignTarget = errors.New("用户与等级 ID 不能为空")
	// ErrLevelDisabled 目标等级已停用，不允许指派。
	ErrLevelDisabled = errors.New("目标等级已停用，不能指派")
	// ErrLevelInUse 是 repository 层同义哨兵的 service 再出口：
	// handler 只允许依赖本包判断语义（CONTRIBUTING §3）。
	ErrLevelInUse = repository.ErrLevelInUse
)
