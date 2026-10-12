// 系统配置域装配（批次四 4b）。

package server

import (
	"context"
	"errors"
	systemhandler "hostsent/backend/internal/modules/admin/system/config/handler"
	systemrepo "hostsent/backend/internal/modules/admin/system/config/repository"
	systemservice "hostsent/backend/internal/modules/admin/system/config/service"

	"gorm.io/gorm"
)

// buildConfig 构造系统配置服务与逐键读取器，并回填财务各服务的配置端口。
func (a *asm) buildConfig() {
	// 系统配置
	a.configRepo = systemrepo.NewConfigRepository(a.database)
	a.configService = systemservice.NewConfigService(a.configRepo)
	a.configHandler = systemhandler.NewConfigHandler(a.configService)
	// 逐键读取 system_configs 原文（键不存在返回 ok=false，不报错）。
	// 验证码策略（doc91）与消息中心（doc90）共用同一读取器。
	a.configValueReader = func(ctx context.Context, key string) (string, bool, error) {
		item, err := a.configRepo.FindByKey(ctx, key)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", false, nil
			}
			return "", false, err
		}
		if item == nil {
			return "", false, nil
		}
		return item.ConfigValue, true, nil
	}
	// 财务配置接线（finance 分组）：
	//   调账开关 → 账务核心（Adjust 前置校验）；对账容差 → 对账中心；余额预警 → 财务统计。
	// 键缺失/非法时各服务各自回默认值，不会因配置问题锁死入口。
	a.walletService.SetConfigReader(a.configValueReader)
	a.reconService.SetConfigReader(a.configValueReader)
	a.financeStatsService.SetConfigReader(a.configValueReader)
}
