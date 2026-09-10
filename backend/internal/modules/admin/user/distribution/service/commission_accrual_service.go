// Package service 提供分销模块的业务编排与规则处理。
package service

import (
	"context"
	"errors"
	"math"

	"go.uber.org/zap"
	"gorm.io/gorm"

	distributionmodel "hostsent/backend/internal/modules/admin/user/distribution/model"
	distributionrepo "hostsent/backend/internal/modules/admin/user/distribution/repository"
)

// 佣金来源与类型（与 distribution_commissions 现有取值保持一致）。
const (
	commissionSourceOrder   = "order"    // 新购订单
	commissionSourceRenewal = "renewal"  // 续费订单
	commissionTypeDirect    = "direct"   // 下级直推
	commissionTypeIndirect  = "indirect" // 下级间接（上级代理）
	commissionTypeSelf      = "self"     // 代理自购返佣
)

// AccrualInput 订单完成时的佣金计提入参，仅用基础类型避免与订单模块循环依赖（P6-02）。
type AccrualInput struct {
	OrderID     uint64  // 订单 ID，(order_id, agent_id) 幂等键
	OrderNo     string  // 订单号，落库便于对账
	BuyerUserID uint64  // 下单用户
	BaseAmount  float64 // 计提基数（实付金额）
	IsRenewal   bool    // 续费订单使用续费佣金率
}

// CommissionAccrualService 订单完成后的佣金自动计提（P6-02）。
type CommissionAccrualService interface {
	// AccrueForOrder 按代理等级佣金率生成佣金记录；无代理关系时静默跳过。
	// 幂等：同一 (order_id, agent_id) 重复调用不会重复计提。
	AccrueForOrder(ctx context.Context, in AccrualInput) error
}

type commissionAccrualService struct {
	repo   distributionrepo.AccrualRepository
	logger *zap.Logger
}

// NewCommissionAccrualService 创建佣金计提服务。
func NewCommissionAccrualService(repo distributionrepo.AccrualRepository, logger *zap.Logger) CommissionAccrualService {
	return &commissionAccrualService{repo: repo, logger: logger}
}

// AccrueForOrder 计提规则：
//   - 下单人本身是代理 → 自购返佣（self），用 self_purchase_rebate_rate / renewal_commission_rate；
//   - 下单人是某代理的下级 → 直推佣金（direct），上级代理（parent_agent_id）→ 间接佣金（indirect）；
//   - 无任何代理关系、佣金率为 0 或提成额为 0 → 不产生记录。
//
// 自购与直推互斥：代理自购不再给其上级计提（避免同一订单双重计佣）。
func (s *commissionAccrualService) AccrueForOrder(ctx context.Context, in AccrualInput) error {
	if in.OrderID == 0 || in.BuyerUserID == 0 || in.BaseAmount <= 0 {
		return nil
	}

	if agent, err := s.repo.AgentByUserID(ctx, in.BuyerUserID); err == nil {
		return s.accrueSelf(ctx, in, agent)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	sub, err := s.repo.SubordinateByUserID(ctx, in.BuyerUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if sub.Status != "" && sub.Status != "active" {
		return nil
	}

	// 直推：下级关系上的代理
	if err := s.accrueForAgent(ctx, in, sub.AgentID, commissionTypeDirect, &sub.ID, false); err != nil {
		return err
	}
	// 间接：上级代理（若有）
	if sub.ParentAgentID != nil && *sub.ParentAgentID != 0 && *sub.ParentAgentID != sub.AgentID {
		if err := s.accrueForAgent(ctx, in, *sub.ParentAgentID, commissionTypeIndirect, &sub.ID, true); err != nil {
			return err
		}
	}
	return nil
}

// accrueSelf 代理自购返佣。
func (s *commissionAccrualService) accrueSelf(ctx context.Context, in AccrualInput, agent *distributionmodel.Agent) error {
	if agent.Status != "" && agent.Status != "active" {
		return nil
	}
	level, err := s.repo.AgentLevelByID(ctx, agent.AgentLevelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if level.Status != "" && level.Status != "active" {
		return nil
	}
	rate := level.SelfPurchaseRebateRate
	if in.IsRenewal {
		rate = level.RenewalCommissionRate
	}
	return s.writeCommission(ctx, in, agent.ID, commissionTypeSelf, rate, "代理自购返佣")
}

// accrueForAgent 给指定代理计提直推/间接佣金。
func (s *commissionAccrualService) accrueForAgent(ctx context.Context, in AccrualInput, agentID uint64, commissionType string, subordinateID *uint64, indirect bool) error {
	agent, err := s.repo.AgentByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if agent.Status != "" && agent.Status != "active" {
		return nil
	}
	level, err := s.repo.AgentLevelByID(ctx, agent.AgentLevelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if level.Status != "" && level.Status != "active" {
		return nil
	}
	rate := level.DirectCommissionRate
	if indirect {
		rate = level.IndirectCommissionRate
	}
	if in.IsRenewal {
		rate = level.RenewalCommissionRate
	}
	remark := "下级订单直推佣金"
	if indirect {
		remark = "下级订单间接佣金"
	}
	return s.writeCommissionWithSubordinate(ctx, in, agentID, commissionType, rate, remark, subordinateID)
}

func (s *commissionAccrualService) writeCommission(ctx context.Context, in AccrualInput, agentID uint64, commissionType string, rate float64, remark string) error {
	return s.writeCommissionWithSubordinate(ctx, in, agentID, commissionType, rate, remark, nil)
}

func (s *commissionAccrualService) writeCommissionWithSubordinate(ctx context.Context, in AccrualInput, agentID uint64, commissionType string, rate float64, remark string, subordinateID *uint64) error {
	if rate <= 0 {
		return nil
	}
	amount := math.Round(in.BaseAmount*rate*100) / 100
	if amount <= 0 {
		return nil
	}
	sourceType := commissionSourceOrder
	if in.IsRenewal {
		sourceType = commissionSourceRenewal
	}
	orderID := in.OrderID
	item := &distributionmodel.Commission{
		OrderID:        &orderID,
		AgentID:        agentID,
		SubordinateID:  subordinateID,
		OrderNo:        in.OrderNo,
		SourceType:     sourceType,
		CommissionType: commissionType,
		BaseAmount:     in.BaseAmount,
		Rate:           rate,
		Amount:         amount,
		Status:         "pending",
		Remark:         remark,
	}
	inserted, err := s.repo.CreateCommissionIfAbsent(ctx, item)
	if err != nil {
		return err
	}
	if inserted {
		s.logger.Info("commission accrued",
			zap.Uint64("order_id", in.OrderID),
			zap.String("order_no", in.OrderNo),
			zap.Uint64("agent_id", agentID),
			zap.String("commission_type", commissionType),
			zap.Float64("amount", amount))
	}
	return nil
}
