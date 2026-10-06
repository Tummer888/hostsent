// Package service 提供限时活动折扣的业务编排（doc108 §8J）。
//
// 定位（与代理折扣的分工，写在这里避免后来人混淆）：
//   - 代理折扣：面向**代理**，长期有效的拿货价，存 agent_level_discounts 逐格矩阵；
//   - 活动折扣：面向**普通用户**，按生效窗口自动开始/结束的营销活动，存 flash_discounts。
//
// 两者**互不叠加**：代理用户不参与活动折扣（RuleForUser 直接返回 nil），
// 普通用户只看活动。这样不会出现「大促时代理比普通用户还贵」这种说不清的价格。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/flashdiscount/dto"
	"hostsent/backend/internal/modules/admin/product/flashdiscount/model"
	"hostsent/backend/internal/modules/admin/product/flashdiscount/repository"
	"hostsent/backend/internal/pkg/pricing"
)

// 业务错误。
var (
	// ErrInvalidValue 折扣值非法（rate 必须 (0,1]，amount 必须 > 0）。
	ErrInvalidValue = errors.New("折扣值非法：折扣率需在 0 到 1 之间，直减金额需大于 0")
	// ErrInvalidWindow 生效窗口非法（结束早于开始）。
	ErrInvalidWindow = errors.New("结束时间不能早于开始时间")
	// ErrInvalidItem 定向条目非法。
	ErrInvalidItem = errors.New("定向条目必须选择具体的分类或商品")
	// ErrBelowCost 折扣低于目标成本率，会亏本。
	ErrBelowCost = errors.New("折扣低于目标成本率，会亏本销售")
	// ErrNoTarget 既没有定向条目、作用范围也不是全站 —— 活动不会命中任何商品。
	ErrNoTarget = errors.New("请选择作用范围或添加定向分类/商品，否则活动不会命中任何商品")
)

// AgentChecker 判断用户是否代理（由装配层注入代理等级服务实现）。
//
// 定义成端口而不是直接依赖 agentlevel 模块：本模块只需要回答「这个用户是不是代理」
// 这一个问题，不该把代理折扣的矩阵、成本校验等整套逻辑拖进来。
type AgentChecker interface {
	IsAgent(ctx context.Context, userID uint64) (bool, error)
}

// Service 活动折扣业务能力。
type Service interface {
	List(ctx context.Context, query dto.Query) (*dto.ListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.Info, error)
	Create(ctx context.Context, req dto.Request) (*dto.Info, error)
	Update(ctx context.Context, id uint64, req dto.Request) (*dto.Info, error)
	Delete(ctx context.Context, id uint64) error

	// RuleForUser 算价命中：解析该用户在该商品上命中的活动折扣规则。
	// 返回 nil 表示不参与（非活动期、未命中、或**用户是代理**）。
	//
	// 只收 productID 而不是「productID + categoryID」：算价管线给 PromotionRule 的口子
	// 没有传分类，而活动的分类命中需要分类 ID —— 由本方法自己按商品解析（仅在确实
	// 存在生效中活动时才查一次），避免为此改动 pricing.Deps 的公共签名。
	RuleForUser(ctx context.Context, userID, productID uint64) (*pricing.Rule, error)
}

type flashService struct {
	repo   repository.Repository
	agents AgentChecker
	now    func() time.Time
}

// NewService 创建活动折扣服务；agents 为 nil 时按「所有用户都不是代理」处理
// （单测可直接构造，装配层务必注入真实实现）。
func NewService(repo repository.Repository, agents AgentChecker) Service {
	return &flashService{repo: repo, agents: agents, now: time.Now}
}

func (s *flashService) List(ctx context.Context, query dto.Query) (*dto.ListResponse, error) {
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	out := make([]dto.Info, 0, len(items))
	for _, item := range items {
		info, err := s.toInfo(ctx, item)
		if err != nil {
			return nil, err
		}
		out = append(out, *info)
	}
	return &dto.ListResponse{
		Items: out,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *flashService) FindByID(ctx context.Context, id uint64) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toInfo(ctx, *item)
}

func (s *flashService) toInfo(ctx context.Context, item model.FlashDiscount) (*dto.Info, error) {
	info := &dto.Info{
		ID:            item.ID,
		Name:          item.Name,
		Code:          item.Code,
		Description:   item.Description,
		DiscountType:  item.DiscountType,
		DiscountValue: item.DiscountValue,
		Scope:         item.Scope,
		StartAt:       formatTime(item.StartAt),
		EndAt:         formatTime(item.EndAt),
		Status:        item.Status,
		RuntimeStatus: item.RuntimeStatus(s.now()),
		Remark:        item.Remark,
		ItemCount:     item.ItemCount,
		CreatedAt:     item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	items, err := s.repo.Items(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	categoryIDs := make([]uint64, 0, len(items))
	productIDs := make([]uint64, 0, len(items))
	for _, it := range items {
		if it.TargetType == model.TargetCategory {
			categoryIDs = append(categoryIDs, it.TargetID)
		} else {
			productIDs = append(productIDs, it.TargetID)
		}
	}
	categoryNames, _ := s.repo.CategoryNames(ctx, categoryIDs)
	productNames, _ := s.repo.ProductNames(ctx, productIDs)
	info.Items = make([]dto.ItemInfo, 0, len(items))
	for _, it := range items {
		name := ""
		if it.TargetType == model.TargetCategory {
			name = categoryNames[it.TargetID]
			if name == "" {
				name = fmt.Sprintf("分类 #%d（已删除）", it.TargetID)
			}
		} else {
			name = productNames[it.TargetID]
			if name == "" {
				name = fmt.Sprintf("商品 #%d（已删除）", it.TargetID)
			}
		}
		info.Items = append(info.Items, dto.ItemInfo{
			TargetType: it.TargetType,
			TargetID:   it.TargetID,
			TargetName: name,
		})
	}
	info.ItemCount = int64(len(info.Items))
	return info, nil
}

// buildItems 收敛请求条目（去重、校验类型）。
func buildItems(req []dto.ItemRequest) ([]model.FlashDiscountItem, error) {
	items := make([]model.FlashDiscountItem, 0, len(req))
	seen := make(map[string]struct{}, len(req))
	for _, raw := range req {
		targetType := strings.TrimSpace(raw.TargetType)
		if targetType != model.TargetCategory && targetType != model.TargetProduct {
			return nil, fmt.Errorf("%w：类型必须是 category 或 product", ErrInvalidItem)
		}
		if raw.TargetID == 0 {
			return nil, ErrInvalidItem
		}
		key := targetType + ":" + fmt.Sprint(raw.TargetID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, model.FlashDiscountItem{TargetType: targetType, TargetID: raw.TargetID})
	}
	return items, nil
}

// validateCost 逐条目校验折扣不低于成本率。
//
// 只在**有条目**时能查成本（分类读 cost_rate，商品按 cost_price/price）；
// scope=all 没有具体目标，无成本基准，跳过（与代理折扣「全站行不校验成本」同口径）。
// 直减（amount）不做成本校验：直减后的金额取决于原价，服务端这里拿不到单价。
func (s *flashService) validateCost(ctx context.Context, discountType string, value float64, items []model.FlashDiscountItem) error {
	if discountType != model.DiscountTypeRate {
		return nil
	}
	for _, it := range items {
		var cost float64
		var err error
		if it.TargetType == model.TargetCategory {
			cost, err = s.repo.CategoryCostRate(ctx, it.TargetID)
		} else {
			cost, err = s.repo.ProductCostRate(ctx, it.TargetID)
		}
		if err != nil {
			return err
		}
		if cost > 0 && value < cost {
			return fmt.Errorf("%w（折扣率 %.4f < 成本率 %.4f）", ErrBelowCost, value, cost)
		}
	}
	return nil
}

func normalizeType(v string) string {
	if strings.TrimSpace(v) == "" {
		return model.DiscountTypeRate
	}
	return v
}

func normalizeScope(v string) string {
	if strings.TrimSpace(v) == "" {
		return model.ScopeAll
	}
	return v
}

func normalizeStatus(v string) string {
	if strings.TrimSpace(v) == "" {
		return model.StatusActive
	}
	return v
}

// validate 校验折扣值、时间窗与作用范围。
func validate(discountType string, value float64, startAt, endAt *time.Time, scope string, items []model.FlashDiscountItem) error {
	switch discountType {
	case model.DiscountTypeRate:
		if value <= 0 || value > 1 {
			return ErrInvalidValue
		}
	case model.DiscountTypeAmount:
		if value <= 0 {
			return ErrInvalidValue
		}
	default:
		return ErrInvalidValue
	}
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		return ErrInvalidWindow
	}
	// 没有条目且范围不是 all：活动命中不了任何商品，是配置失误而非「空活动」。
	if len(items) == 0 && scope != model.ScopeAll {
		return ErrNoTarget
	}
	return nil
}

func (s *flashService) Create(ctx context.Context, req dto.Request) (*dto.Info, error) {
	discountType := normalizeType(req.DiscountType)
	scope := normalizeScope(req.Scope)
	items, err := buildItems(req.Items)
	if err != nil {
		return nil, err
	}
	startAt, endAt := parseTime(req.StartAt), parseTime(req.EndAt)
	if err := validate(discountType, req.DiscountValue, startAt, endAt, scope, items); err != nil {
		return nil, err
	}
	if err := s.validateCost(ctx, discountType, req.DiscountValue, items); err != nil {
		return nil, err
	}
	item := &model.FlashDiscount{
		Name:          strings.TrimSpace(req.Name),
		Code:          strings.TrimSpace(req.Code),
		Description:   req.Description,
		DiscountType:  discountType,
		DiscountValue: req.DiscountValue,
		Scope:         scope,
		StartAt:       startAt,
		EndAt:         endAt,
		Status:        normalizeStatus(req.Status),
		Remark:        req.Remark,
	}
	if err := s.repo.Create(ctx, item, items); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *flashService) Update(ctx context.Context, id uint64, req dto.Request) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	discountType := normalizeType(req.DiscountType)
	scope := normalizeScope(req.Scope)
	// Items=nil 表示不改条目：用现有条目参与校验（作用范围与条目的一致性不能只看请求）。
	var items []model.FlashDiscountItem
	replace := false
	if req.Items != nil {
		items, err = buildItems(req.Items)
		if err != nil {
			return nil, err
		}
		replace = true
	} else {
		items, err = s.repo.Items(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	startAt, endAt := parseTime(req.StartAt), parseTime(req.EndAt)
	if err := validate(discountType, req.DiscountValue, startAt, endAt, scope, items); err != nil {
		return nil, err
	}
	if err := s.validateCost(ctx, discountType, req.DiscountValue, items); err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.Code = strings.TrimSpace(req.Code)
	item.Description = req.Description
	item.DiscountType = discountType
	item.DiscountValue = req.DiscountValue
	item.Scope = scope
	item.StartAt = startAt
	item.EndAt = endAt
	item.Status = normalizeStatus(req.Status)
	item.Remark = req.Remark
	if err := s.repo.Update(ctx, item, items, replace); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *flashService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// RuleForUser 算价命中。
//
// 三条拒绝线，顺序刻意如此：
//  1. 用户是代理 → 直接返回 nil（活动只面向普通用户，代理只走代理折扣）；
//  2. 没有生效中的活动 → nil；
//  3. 生效中的活动里没有任何一条命中该商品 → nil。
//
// 多条活动同时命中时取**优惠最大**的一条（折扣率最小 / 直减最大）：
// 活动是运营手配的少量条目，出现重叠属于配置重叠，取最优对用户最友好且口径可解释。
func (s *flashService) RuleForUser(ctx context.Context, userID, productID uint64) (*pricing.Rule, error) {
	if userID == 0 {
		return nil, nil
	}
	if s.agents != nil {
		isAgent, err := s.agents.IsAgent(ctx, userID)
		if err != nil {
			return nil, err
		}
		if isAgent {
			return nil, nil // 代理不参与活动折扣
		}
	}
	actives, err := s.repo.ActiveAt(ctx, s.now())
	if err != nil {
		return nil, err
	}
	if len(actives) == 0 {
		return nil, nil
	}
	categoryID, err := s.repo.CategoryOfProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	var best *pricing.Rule
	var bestScore float64
	var bestHas bool
	for _, active := range actives {
		rule := matchActive(active, productID, categoryID)
		if rule == nil {
			continue
		}
		score := scoreOf(*rule)
		if !bestHas || score > bestScore {
			best, bestScore, bestHas = rule, score, true
		}
	}
	return best, nil
}

// matchActive 在单个活动内判定命中：条目按「商品优先于分类」，无条目时按 scope。
func matchActive(active repository.ActiveDiscount, productID, categoryID uint64) *pricing.Rule {
	d := active.Discount
	if len(active.Items) == 0 {
		if d.Scope != model.ScopeAll {
			return nil
		}
		return ruleOf(d)
	}
	// 商品优先：同一活动里同时列了分类与商品时，商品条目更具体。
	for _, it := range active.Items {
		if it.TargetType == model.TargetProduct && productID > 0 && it.TargetID == productID {
			return ruleOf(d)
		}
	}
	if categoryID > 0 {
		for _, it := range active.Items {
			if it.TargetType == model.TargetCategory && it.TargetID == categoryID {
				return ruleOf(d)
			}
		}
	}
	return nil
}

func ruleOf(d model.FlashDiscount) *pricing.Rule {
	if d.DiscountValue <= 0 {
		return nil
	}
	return &pricing.Rule{
		Source:   pricing.SourcePromotion,
		Code:     d.Code,
		Type:     d.DiscountType,
		Value:    d.DiscountValue,
		PolicyID: d.ID,
	}
}

// scoreOf 折扣力度的可比分数：折扣率取 (1 - 率)，直减取金额本身。
// 只用于「多条活动重叠时挑最优」，与 pricing 的 discountOf 同一意图但不需要金额。
func scoreOf(rule pricing.Rule) float64 {
	if rule.Type == pricing.TypeRate {
		return 1 - rule.Value
	}
	return rule.Value
}

// parseTime 宽松解析：兼容 RFC3339（前端的 ISO 串）与「2006-01-02 15:04:05」。
// 解析失败按「未设置」处理（不限），不因一个格式差异让整次保存失败。
func parseTime(val string) *time.Time {
	v := strings.TrimSpace(val)
	if v == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
