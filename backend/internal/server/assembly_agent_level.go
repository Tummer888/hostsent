package server

// 代理等级校验的装配适配（用户详情页/建号时授予代理身份）。
//
// 依赖方向：account 模块声明 AgentLevelValidator 端口（CheckAssignable），
// 本文件用 agentlevel 模块的服务把它实现出来。account 不 import agentlevel。
//
// 错误口径转换的理由同 assembly_level.go：agentlevel 服务返回
// gorm.ErrRecordNotFound（等级不存在）与自己的哨兵错误，而 account 的 handler
// 把 gorm.ErrRecordNotFound 一律翻成「用户不存在」——直接把 agentlevel 的错误
// 透上去，运营设代理等级时会看到「用户不存在」，指错了对象。

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/account/service"
	agentlevelservice "hostsent/backend/internal/modules/admin/user/agentlevel/service"
)

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
