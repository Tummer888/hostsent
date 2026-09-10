// Package pricing 实现统一算价管线（P5-03）。
//
// 职责边界：
//   - 基础价来自 product_pricing（读不到回落 products.price），见 P5-02；
//   - 折扣来源按 D3 定稿顺序叠加：代理价 → 用户组策略 → 促销/优惠券 → 管理员手动改价；
//   - 用户等级**不参与**算价（D3），等级只决定消费升级与子账号上限。
//
// 叠加模式由全局配置 pricing.stack_mode 决定：
//   - best（默认）：只取优惠幅度最大的一条规则；
//   - stack：按顺序叠加全部规则。
package pricing

import (
	"context"
	"math"
)

// 折扣来源标识。
const (
	SourceAgent     = "agent"     // 代理价
	SourceGroup     = "group"     // 用户组策略
	SourcePromotion = "promotion" // 促销/优惠券
	SourceManual    = "manual"    // 管理员手动改价
)

// 折扣类型。
const (
	TypeRate   = "rate"   // 折扣率：0.85 表示 85 折
	TypeAmount = "amount" // 直减：固定减免金额
	TypeManual = "manual" // 手动指定实付
)

// 叠加模式。
const (
	StackModeBest  = "best"
	StackModeStack = "stack"
)

// Rule 单条命中规则。
type Rule struct {
	Source   string  `json:"source"`
	Code     string  `json:"code"`
	Type     string  `json:"type"`
	Value    float64 `json:"value"`
	PolicyID uint64  `json:"policy_id,omitempty"`
}

// Quote 算价结果：原价、优惠、实付与命中来源。
type Quote struct {
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	PolicyID       *uint64 `json:"price_policy_id"`
	Source         string  `json:"discount_source"`
	Snapshot       []Rule  `json:"price_snapshot"`
}

// ResolveInput 算价入参。
type ResolveInput struct {
	UserID    uint64
	ProductID uint64
	SpecCode  string
	Quantity  int
	Period    int
	// ManualAmount 非 nil 时为管理员手动改价，直接作为实付金额（最高优先级）。
	ManualAmount *float64
}

// Deps 各折扣来源解析器，由装配层注入；为 nil 表示该来源不参与本次算价。
type Deps struct {
	// BasePrice 返回商品基础单价与分类 ID（product_pricing 优先，回落 products.price）。
	BasePrice func(ctx context.Context, productID uint64) (unitPrice float64, categoryID uint64, err error)
	// AgentRule 代理价（P6 接入 agent_levels.price_policy_id，当前可为 nil）。
	AgentRule func(ctx context.Context, userID, productID, categoryID uint64) (*Rule, error)
	// GroupRule 用户组策略（user_groups.price_policy_id）。
	GroupRule func(ctx context.Context, userID, productID, categoryID uint64) (*Rule, error)
	// PromotionRule 促销/优惠券（当前未接入返回 nil）。
	PromotionRule func(ctx context.Context, in ResolveInput, amount float64) (*Rule, error)
}

// Service 统一算价服务。
type Service struct {
	deps      Deps
	stackMode string
}

// NewService 创建算价服务。stackMode 为空时按 best 处理。
func NewService(deps Deps, stackMode string) *Service {
	if stackMode != StackModeStack {
		stackMode = StackModeBest
	}
	return &Service{deps: deps, stackMode: stackMode}
}

// Resolve 计算商品价格明细，不落库、不扣款（P5-04 下单与 P5-05 预结算共用）。
func (s *Service) Resolve(ctx context.Context, in ResolveInput) (*Quote, error) {
	qty := in.Quantity
	if qty <= 0 {
		qty = 1
	}

	var unitPrice float64
	var categoryID uint64
	if s.deps.BasePrice != nil {
		price, category, err := s.deps.BasePrice(ctx, in.ProductID)
		if err != nil {
			return nil, err
		}
		unitPrice, categoryID = price, category
	}
	if unitPrice < 0 {
		unitPrice = 0
	}
	original := round2(unitPrice * float64(qty))

	// 管理员手动改价优先级最高，直接覆盖折扣管线。
	if in.ManualAmount != nil {
		final := round2(math.Max(0, *in.ManualAmount))
		return &Quote{
			OriginalAmount: original,
			DiscountAmount: round2(math.Max(0, original-final)),
			FinalAmount:    final,
			Source:         SourceManual,
			Snapshot:       []Rule{{Source: SourceManual, Code: "manual", Type: TypeManual, Value: final}},
		}, nil
	}

	rules := make([]Rule, 0, 3)
	for _, resolve := range []func() (*Rule, error){
		func() (*Rule, error) {
			if s.deps.AgentRule == nil {
				return nil, nil
			}
			return s.deps.AgentRule(ctx, in.UserID, in.ProductID, categoryID)
		},
		func() (*Rule, error) {
			if s.deps.GroupRule == nil {
				return nil, nil
			}
			return s.deps.GroupRule(ctx, in.UserID, in.ProductID, categoryID)
		},
		func() (*Rule, error) {
			if s.deps.PromotionRule == nil {
				return nil, nil
			}
			return s.deps.PromotionRule(ctx, in, original)
		},
	} {
		rule, err := resolve()
		if err != nil {
			return nil, err
		}
		if rule == nil {
			continue
		}
		rules = append(rules, *rule)
	}

	if len(rules) == 0 {
		return &Quote{
			OriginalAmount: original,
			DiscountAmount: 0,
			FinalAmount:    original,
			Snapshot:       []Rule{},
		}, nil
	}

	if s.stackMode == StackModeStack {
		amount := original
		for _, rule := range rules {
			amount = applyRule(amount, rule)
		}
		final := round2(math.Max(0, amount))
		last := rules[len(rules)-1]
		return &Quote{
			OriginalAmount: original,
			DiscountAmount: round2(math.Max(0, original-final)),
			FinalAmount:    final,
			PolicyID:       policyIDOf(last),
			Source:         last.Source,
			Snapshot:       rules,
		}, nil
	}

	// best：取优惠幅度最大的一条。
	best := rules[0]
	bestDiscount := discountOf(original, best)
	for _, rule := range rules[1:] {
		if discount := discountOf(original, rule); discount > bestDiscount {
			best, bestDiscount = rule, discount
		}
	}
	final := round2(math.Max(0, original-bestDiscount))
	return &Quote{
		OriginalAmount: original,
		DiscountAmount: round2(math.Max(0, original-final)),
		FinalAmount:    final,
		PolicyID:       policyIDOf(best),
		Source:         best.Source,
		Snapshot:       rules,
	}, nil
}

// applyRule 在给定金额上应用一条规则（叠加模式逐条串行）。
func applyRule(amount float64, rule Rule) float64 {
	switch rule.Type {
	case TypeRate:
		rate := clampRate(rule.Value)
		return amount * rate
	case TypeAmount:
		return amount - math.Min(rule.Value, amount)
	default:
		return amount
	}
}

// discountOf 计算一条规则在给定金额上的优惠额（best 模式用于比较）。
func discountOf(amount float64, rule Rule) float64 {
	switch rule.Type {
	case TypeRate:
		rate := clampRate(rule.Value)
		return amount * (1 - rate)
	case TypeAmount:
		return math.Min(rule.Value, amount)
	default:
		return 0
	}
}

// clampRate 把折扣率限制在 [0,1]，非法值按不打折处理。
func clampRate(value float64) float64 {
	if value <= 0 || value > 1 {
		return 1
	}
	return value
}

func policyIDOf(rule Rule) *uint64 {
	if rule.PolicyID == 0 {
		return nil
	}
	id := rule.PolicyID
	return &id
}

// round2 金额保留两位小数，避免浮点误差落库。
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
