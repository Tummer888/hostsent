// Package service 提供用户等级模块的业务编排。
package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/level/model"
	"hostsent/backend/internal/modules/admin/user/level/repository"
)

// LevelUpgradeService 按累计消费维护用户等级（P3-03）。
//
// 规则：只升不降——目标等级 weight 必须严格大于当前等级 weight 才更新；
// 等级变更时写一条 user_level_change_logs（权益本期只记录不发放）。
type LevelUpgradeService interface {
	// Recalculate 按当前累计消费重算等级，返回是否变更及变更前后的等级 code。
	Recalculate(ctx context.Context, userID uint64) (changed bool, from, to string, err error)
	// ApplyConsume 累加消费额后重算等级；供订单支付成功后调用（调用方可异步执行）。
	ApplyConsume(ctx context.Context, userID uint64, amount float64) error
}

type levelUpgradeService struct {
	repo repository.UserLevelRepository
}

// NewLevelUpgradeService 创建消费升级服务。
func NewLevelUpgradeService(repo repository.UserLevelRepository) LevelUpgradeService {
	return &levelUpgradeService{repo: repo}
}

func (s *levelUpgradeService) Recalculate(ctx context.Context, userID uint64) (bool, string, string, error) {
	currentLevelID, totalConsume, err := s.repo.GetUserTier(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, "", "", nil
		}
		return false, "", "", err
	}

	target, err := s.repo.FindBestByThreshold(ctx, totalConsume)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 没有达到任何等级门槛（例如等级表未配置 standard），保持现状。
			return false, "", "", nil
		}
		return false, "", "", err
	}

	var current *model.UserLevel
	if currentLevelID != nil && *currentLevelID > 0 {
		if item, ferr := s.repo.FindByID(ctx, *currentLevelID); ferr == nil {
			current = item
		} else if !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return false, "", "", ferr
		}
	}

	fromCode := ""
	if current != nil {
		fromCode = current.Code
		if target.Weight <= current.Weight {
			return false, fromCode, target.Code, nil
		}
	}

	if err := s.repo.SetUserTier(ctx, userID, target.ID); err != nil {
		return false, fromCode, target.Code, err
	}

	log := &model.UserLevelChangeLog{
		UserID:             userID,
		FromLevelCode:      fromCode,
		ToLevelID:          target.ID,
		ToLevelCode:        target.Code,
		TotalConsumeAmount: totalConsume,
		BenefitsSnapshot:   target.Benefits,
		Reason:             "consume_upgrade",
	}
	if current != nil {
		log.FromLevelID = &current.ID
	}
	// 日志失败不影响等级更新结果，仅返回错误交调用方记录。
	if err := s.repo.CreateChangeLog(ctx, log); err != nil {
		return true, fromCode, target.Code, err
	}
	return true, fromCode, target.Code, nil
}

func (s *levelUpgradeService) ApplyConsume(ctx context.Context, userID uint64, amount float64) error {
	if userID == 0 || amount <= 0 {
		return nil
	}
	if _, err := s.repo.AddUserConsume(ctx, userID, amount); err != nil {
		return err
	}
	_, _, _, err := s.Recalculate(ctx, userID)
	return err
}
