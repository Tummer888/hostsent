// Package service 提供折扣策略（price policy）子域的业务编排。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/discount/dto"
	"hostsent/backend/internal/modules/admin/product/discount/model"
	"hostsent/backend/internal/modules/admin/product/discount/repository"
	"hostsent/backend/internal/pkg/pricing"
)

// PolicyService 折扣策略业务能力。
type PolicyService interface {
	List(ctx context.Context, query dto.PolicyQuery) (*dto.PolicyListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.PolicyInfo, error)
	Create(ctx context.Context, req dto.PolicyRequest) (*dto.PolicyInfo, error)
	Update(ctx context.Context, id uint64, req dto.PolicyRequest) (*dto.PolicyInfo, error)
	Delete(ctx context.Context, id uint64) error
	// RuleForProduct 解析策略对指定商品/分类生效的折扣规则；不生效返回 nil（P5-03）。
	RuleForProduct(ctx context.Context, policyID, productID, categoryID uint64) (*pricing.Rule, error)
	// RuleForUserGroup 解析用户所属用户组的折扣规则，供算价管线 GroupRule 使用。
	RuleForUserGroup(ctx context.Context, userID, productID, categoryID uint64) (*pricing.Rule, error)
}

type policyService struct {
	repo repository.PricePolicyRepository
	now  func() time.Time
}

// NewPolicyService 创建折扣策略服务。
func NewPolicyService(repo repository.PricePolicyRepository) PolicyService {
	return &policyService{repo: repo, now: time.Now}
}

func (s *policyService) List(ctx context.Context, query dto.PolicyQuery) (*dto.PolicyListResponse, error) {
	policies, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items := make([]dto.PolicyInfo, 0, len(policies))
	for _, policy := range policies {
		items = append(items, s.toInfo(ctx, policy))
	}
	return &dto.PolicyListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *policyService) FindByID(ctx context.Context, id uint64) (*dto.PolicyInfo, error) {
	policy, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, *policy)
	return &info, nil
}

func (s *policyService) Create(ctx context.Context, req dto.PolicyRequest) (*dto.PolicyInfo, error) {
	policy := &model.PricePolicy{
		Name:          strings.TrimSpace(req.Name),
		Code:          strings.TrimSpace(req.Code),
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		Scope:         normalizeScope(req.Scope),
		Priority:      req.Priority,
		Status:        normalizeStatus(req.Status),
		Remark:        req.Remark,
	}
	applyEffectiveWindow(policy, req)
	if err := s.repo.Create(ctx, policy); err != nil {
		return nil, err
	}
	if err := s.saveItems(ctx, policy.ID, req.Items); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, policy.ID)
}

func (s *policyService) Update(ctx context.Context, id uint64, req dto.PolicyRequest) (*dto.PolicyInfo, error) {
	policy, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	policy.Name = strings.TrimSpace(req.Name)
	policy.Code = strings.TrimSpace(req.Code)
	policy.DiscountType = req.DiscountType
	policy.DiscountValue = req.DiscountValue
	policy.Scope = normalizeScope(req.Scope)
	policy.Priority = req.Priority
	policy.Status = normalizeStatus(req.Status)
	policy.Remark = req.Remark
	applyEffectiveWindow(policy, req)
	if err := s.repo.Update(ctx, policy); err != nil {
		return nil, err
	}
	if err := s.saveItems(ctx, policy.ID, req.Items); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, policy.ID)
}

func (s *policyService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// RuleForProduct 命中判定：条目按 product 优先、category 次之；无条目时仅 scope=all 生效。
func (s *policyService) RuleForProduct(ctx context.Context, policyID, productID, categoryID uint64) (*pricing.Rule, error) {
	return s.ruleForPolicy(ctx, policyID, productID, categoryID, pricing.SourceGroup)
}

// ruleForPolicy 按策略 ID 解析对指定商品/分类生效的折扣规则；不生效返回 nil。
// source 决定规则来源标识（用户组折扣 / 代理价），同一策略对不同来源语义不同（P6-01）。
func (s *policyService) ruleForPolicy(ctx context.Context, policyID, productID, categoryID uint64, source string) (*pricing.Rule, error) {
	if policyID == 0 {
		return nil, nil
	}
	policy, err := s.repo.FindByID(ctx, policyID)
	if err != nil {
		return nil, err
	}
	if policy.Status != "active" {
		return nil, nil
	}
	now := s.now()
	if policy.EffectiveFrom != nil && now.Before(*policy.EffectiveFrom) {
		return nil, nil
	}
	if policy.EffectiveTo != nil && now.After(*policy.EffectiveTo) {
		return nil, nil
	}

	items, err := s.repo.Items(ctx, policy.ID)
	if err != nil {
		return nil, err
	}
	var matched *model.PricePolicyItem
	for i := range items {
		if items[i].TargetType == model.TargetProduct && items[i].TargetID == productID {
			matched = &items[i]
			break
		}
	}
	if matched == nil && categoryID > 0 {
		for i := range items {
			if items[i].TargetType == model.TargetCategory && items[i].TargetID == categoryID {
				matched = &items[i]
				break
			}
		}
	}

	rule := &pricing.Rule{Source: source, Code: policy.Code, PolicyID: policy.ID}
	switch {
	case matched != nil:
		rule.Type = matched.DiscountType
		rule.Value = matched.DiscountValue
	case len(items) == 0 && policy.Scope == model.ScopeAll:
		rule.Type = policy.DiscountType
		rule.Value = policy.DiscountValue
	default:
		return nil, nil
	}
	if !validRule(*rule) {
		return nil, nil
	}
	return rule, nil
}

// RuleForUserGroup 组合「用户 → 用户组 → 策略 → 规则」链路。
func (s *policyService) RuleForUserGroup(ctx context.Context, userID, productID, categoryID uint64) (*pricing.Rule, error) {
	policyID, err := s.repo.GroupPolicyIDForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if policyID == 0 {
		return nil, nil
	}
	return s.RuleForProduct(ctx, policyID, productID, categoryID)
}

func (s *policyService) saveItems(ctx context.Context, policyID uint64, reqItems []dto.PolicyItemRequest) error {
	items := make([]model.PricePolicyItem, 0, len(reqItems))
	for _, item := range reqItems {
		if item.TargetID == 0 {
			continue
		}
		if item.TargetType != model.TargetCategory && item.TargetType != model.TargetProduct {
			continue
		}
		if item.DiscountType != model.DiscountTypeRate && item.DiscountType != model.DiscountTypeAmount {
			continue
		}
		items = append(items, model.PricePolicyItem{
			PolicyID:      policyID,
			TargetType:    item.TargetType,
			TargetID:      item.TargetID,
			DiscountType:  item.DiscountType,
			DiscountValue: item.DiscountValue,
		})
	}
	return s.repo.ReplaceItems(ctx, policyID, items)
}

func (s *policyService) toInfo(ctx context.Context, policy model.PricePolicy) dto.PolicyInfo {
	info := dto.PolicyInfo{
		ID:            policy.ID,
		Name:          policy.Name,
		Code:          policy.Code,
		DiscountType:  policy.DiscountType,
		DiscountValue: policy.DiscountValue,
		Scope:         policy.Scope,
		Priority:      policy.Priority,
		EffectiveFrom: formatTime(policy.EffectiveFrom),
		EffectiveTo:   formatTime(policy.EffectiveTo),
		Status:        policy.Status,
		Remark:        policy.Remark,
		Items:         []dto.PolicyItemInfo{},
		CreatedAt:     policy.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     policy.UpdatedAt.Format(time.RFC3339),
	}
	items, err := s.repo.Items(ctx, policy.ID)
	if err == nil {
		for _, item := range items {
			info.Items = append(info.Items, dto.PolicyItemInfo{
				ID:            item.ID,
				TargetType:    item.TargetType,
				TargetID:      item.TargetID,
				DiscountType:  item.DiscountType,
				DiscountValue: item.DiscountValue,
			})
		}
	}
	return info
}

func normalizeScope(scope string) string {
	switch scope {
	case model.ScopeCategory, model.ScopeProduct:
		return scope
	default:
		return model.ScopeAll
	}
}

func normalizeStatus(status string) string {
	if status == "disabled" {
		return "disabled"
	}
	return "active"
}

// applyEffectiveWindow 解析可选的生效时间，空值表示不限。
func applyEffectiveWindow(policy *model.PricePolicy, req dto.PolicyRequest) {
	policy.EffectiveFrom = parseTime(req.EffectiveFrom)
	policy.EffectiveTo = parseTime(req.EffectiveTo)
}

func parseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

// validRule 折扣率必须落在 (0,1]，直减必须为正。
func validRule(rule pricing.Rule) bool {
	if rule.Type == pricing.TypeRate {
		return rule.Value > 0 && rule.Value <= 1
	}
	if rule.Type == pricing.TypeAmount {
		return rule.Value > 0
	}
	return false
}

// ErrPolicyNotFound 折扣策略不存在。
var ErrPolicyNotFound = errors.New("折扣策略不存在")
