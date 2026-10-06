package server

// 用户等级调整的装配适配（用户详情页「编辑资料」改等级）。
//
// 依赖方向：account 模块声明 LevelAssigner 端口（AssignLevel / DefaultLevelID），
// 本文件用 level 模块的服务把它实现出来。account 不 import level，level 也不 import account。
//
// 为什么要适配而不是直接把 levelService 传进去：两侧的错误口径不同。
// level 服务返回 gorm.ErrRecordNotFound（等级不存在）与自己的哨兵错误，
// 而 account 的 handler 把 gorm.ErrRecordNotFound 一律翻成「用户不存在」——
// 直接把 level 的错误透上去，运营改等级时会看到「用户不存在」，指错了对象。

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/service"
	levelservice "hostsent/backend/internal/modules/admin/user/account/level/service"
)

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
