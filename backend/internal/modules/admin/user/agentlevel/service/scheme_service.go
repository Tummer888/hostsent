// Package service：商品分组与折扣组（doc108 §8I）。
//
// 商品分组 = 分类/商品的命名集合；折扣组 = 绑定一个商品分组 + 每个代理等级一个
// 折扣率。「应用」把 (折扣组 × 商品分组) 展开成逐格矩阵写入 agent_level_discounts
// —— 复用 ApplyRateCells 的成本线/单调性/目标校验，算价链路零改动。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hostsent/backend/internal/modules/admin/user/agentlevel/dto"
	"hostsent/backend/internal/modules/admin/user/agentlevel/model"
	"hostsent/backend/internal/modules/admin/user/agentlevel/repository"
)

// 折扣组域的业务错误（handler 映射 400/409）。
var (
	// ErrSchemeNoGroup 折扣组未绑定商品分组，无法应用。
	ErrSchemeNoGroup = errors.New("请先为该折扣组绑定商品分组")
	// ErrSchemeGroupEmpty 绑定的商品分组为空。
	ErrSchemeGroupEmpty = errors.New("绑定的商品分组为空，请先添加分类或商品")
	// ErrSchemeNoRates 折扣组内没有大于 0 的费率。
	ErrSchemeNoRates = errors.New("折扣组内没有大于 0 的折扣率，无需应用")
	// ErrInvalidRate 折扣率非法。
	ErrInvalidRate = errors.New("折扣率必须是 0 到 1 之间的数值")
)

// normalizeSchemeStatus 归一状态值。
func normalizeSchemeStatus(status string) string {
	if strings.TrimSpace(status) == "" {
		return "active"
	}
	return status
}

// —— 商品分组 ——

type productGroupService struct {
	repo repository.SchemeRepository
}

// ProductGroupService 商品分组业务能力。
type ProductGroupService interface {
	List(ctx context.Context) (*dto.ProductGroupListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProductGroupInfo, error)
	Create(ctx context.Context, req dto.ProductGroupRequest) (*dto.ProductGroupInfo, error)
	Update(ctx context.Context, id uint64, req dto.ProductGroupRequest) (*dto.ProductGroupInfo, error)
	Delete(ctx context.Context, id uint64) error
}

// NewProductGroupService 创建商品分组服务。
func NewProductGroupService(repo repository.SchemeRepository) ProductGroupService {
	return &productGroupService{repo: repo}
}

func (s *productGroupService) List(ctx context.Context) (*dto.ProductGroupListResponse, error) {
	groups, err := s.repo.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.ProductGroupInfo, 0, len(groups))
	for _, group := range groups {
		info, err := s.groupInfo(ctx, group)
		if err != nil {
			return nil, err
		}
		items = append(items, *info)
	}
	return &dto.ProductGroupListResponse{Items: items}, nil
}

func (s *productGroupService) FindByID(ctx context.Context, id uint64) (*dto.ProductGroupInfo, error) {
	group, err := s.repo.FindGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.groupInfo(ctx, *group)
}

func (s *productGroupService) groupInfo(ctx context.Context, group model.ProductGroup) (*dto.ProductGroupInfo, error) {
	info := &dto.ProductGroupInfo{
		ID:          group.ID,
		Name:        group.Name,
		Code:        group.Code,
		Description: group.Description,
		Status:      group.Status,
		ItemCount:   group.ItemCount,
		CreatedAt:   group.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   group.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	items, err := s.repo.GroupItems(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	categoryIDs := make([]uint64, 0, len(items))
	productIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.TargetType == model.GroupTargetCategory {
			categoryIDs = append(categoryIDs, item.TargetID)
		} else {
			productIDs = append(productIDs, item.TargetID)
		}
	}
	categoryNames, _ := s.repo.CategoryNames(ctx, categoryIDs)
	productNames, _ := s.repo.ProductNames(ctx, productIDs)
	info.Items = make([]dto.ProductGroupTargetInfo, 0, len(items))
	for _, item := range items {
		name := ""
		switch item.TargetType {
		case model.GroupTargetCategory:
			name = categoryNames[item.TargetID]
			if name == "" {
				name = fmt.Sprintf("分类 #%d（已删除）", item.TargetID)
			}
		default:
			name = productNames[item.TargetID]
			if name == "" {
				name = fmt.Sprintf("商品 #%d（已删除）", item.TargetID)
			}
		}
		info.Items = append(info.Items, dto.ProductGroupTargetInfo{
			TargetType: item.TargetType,
			TargetID:   item.TargetID,
			TargetName: name,
		})
	}
	return info, nil
}

func buildGroupItems(req []dto.ProductGroupItemRequest) ([]model.ProductGroupItem, error) {
	items := make([]model.ProductGroupItem, 0, len(req))
	seen := make(map[string]struct{}, len(req))
	for _, raw := range req {
		targetType := strings.TrimSpace(raw.TargetType)
		if targetType != model.GroupTargetCategory && targetType != model.GroupTargetProduct {
			return nil, fmt.Errorf("%w：成员类型必须是 category 或 product", ErrInvalidAssignTarget)
		}
		if raw.TargetID == 0 {
			return nil, fmt.Errorf("%w：请选择具体的分类或商品", ErrInvalidAssignTarget)
		}
		key := targetType + ":" + fmt.Sprint(raw.TargetID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, model.ProductGroupItem{TargetType: targetType, TargetID: raw.TargetID})
	}
	return items, nil
}

func (s *productGroupService) Create(ctx context.Context, req dto.ProductGroupRequest) (*dto.ProductGroupInfo, error) {
	items, err := buildGroupItems(req.Items)
	if err != nil {
		return nil, err
	}
	group := &model.ProductGroup{
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Description: req.Description,
		Status:      normalizeSchemeStatus(req.Status),
	}
	if err := s.repo.CreateGroup(ctx, group, items); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, group.ID)
}

func (s *productGroupService) Update(ctx context.Context, id uint64, req dto.ProductGroupRequest) (*dto.ProductGroupInfo, error) {
	group, err := s.repo.FindGroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var items []model.ProductGroupItem
	replace := false
	if req.Items != nil {
		items, err = buildGroupItems(req.Items)
		if err != nil {
			return nil, err
		}
		replace = true
	}
	group.Name = strings.TrimSpace(req.Name)
	group.Code = strings.TrimSpace(req.Code)
	group.Description = req.Description
	group.Status = normalizeSchemeStatus(req.Status)
	if err := s.repo.UpdateGroup(ctx, group, items, replace); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *productGroupService) Delete(ctx context.Context, id uint64) error {
	bound, err := s.repo.CountSchemesOnGroup(ctx, id, 0)
	if err != nil {
		return err
	}
	if bound > 0 {
		return repository.ErrGroupInUse
	}
	return s.repo.DeleteGroup(ctx, id)
}

// —— 折扣组 ——

type discountSchemeService struct {
	repo   repository.SchemeRepository
	levels AgentLevelService
}

// DiscountSchemeService 折扣组业务能力。
type DiscountSchemeService interface {
	List(ctx context.Context) (*dto.SchemeListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.SchemeInfo, error)
	Create(ctx context.Context, req dto.SchemeRequest) (*dto.SchemeInfo, error)
	Update(ctx context.Context, id uint64, req dto.SchemeRequest) (*dto.SchemeInfo, error)
	Delete(ctx context.Context, id uint64) error
	// Apply 把折扣组展开写入其绑定的商品分组（等级 × 分组内全部目标）。
	Apply(ctx context.Context, id uint64) (*dto.MatrixResponse, error)
}

// NewDiscountSchemeService 创建折扣组服务；levels 提供应用时的统一校验入口。
func NewDiscountSchemeService(repo repository.SchemeRepository, levels AgentLevelService) DiscountSchemeService {
	return &discountSchemeService{repo: repo, levels: levels}
}

func (s *discountSchemeService) List(ctx context.Context) (*dto.SchemeListResponse, error) {
	schemes, err := s.repo.ListSchemes(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.SchemeInfo, 0, len(schemes))
	for _, scheme := range schemes {
		info, err := s.schemeInfo(ctx, scheme)
		if err != nil {
			return nil, err
		}
		items = append(items, *info)
	}
	return &dto.SchemeListResponse{Items: items}, nil
}

func (s *discountSchemeService) FindByID(ctx context.Context, id uint64) (*dto.SchemeInfo, error) {
	scheme, err := s.repo.FindSchemeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.schemeInfo(ctx, *scheme)
}

func (s *discountSchemeService) schemeInfo(ctx context.Context, scheme model.DiscountScheme) (*dto.SchemeInfo, error) {
	info := &dto.SchemeInfo{
		ID:          scheme.ID,
		Name:        scheme.Name,
		Code:        scheme.Code,
		Description: scheme.Description,
		Status:      scheme.Status,
		ItemCount:   scheme.ItemCount,
		CreatedAt:   scheme.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   scheme.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if scheme.ProductGroupID != nil {
		info.ProductGroupID = *scheme.ProductGroupID
		if group, err := s.repo.FindGroupByID(ctx, *scheme.ProductGroupID); err == nil {
			info.ProductGroupName = group.Name
		}
	}
	items, err := s.repo.SchemeItems(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	levels, err := s.repo.LevelsBasic(ctx)
	if err != nil {
		return nil, err
	}
	basics := make(map[uint64]repository.LevelBasic, len(levels))
	for _, level := range levels {
		basics[level.ID] = level
	}
	info.Items = make([]dto.SchemeItemInfo, 0, len(items))
	for _, item := range items {
		name := ""
		weight := 0
		if basic, ok := basics[item.AgentLevelID]; ok {
			name = basic.Name
			weight = basic.Weight
		}
		info.Items = append(info.Items, dto.SchemeItemInfo{
			AgentLevelID:     item.AgentLevelID,
			AgentLevelName:   name,
			AgentLevelWeight: weight,
			DiscountRate:     item.DiscountRate,
		})
	}
	return info, nil
}

func buildSchemeItems(req []dto.SchemeItemRequest) ([]model.DiscountSchemeItem, error) {
	items := make([]model.DiscountSchemeItem, 0, len(req))
	seen := make(map[uint64]struct{}, len(req))
	for _, raw := range req {
		if raw.AgentLevelID == 0 {
			continue
		}
		if _, ok := seen[raw.AgentLevelID]; ok {
			continue
		}
		seen[raw.AgentLevelID] = struct{}{}
		if raw.DiscountRate < 0 || raw.DiscountRate > 1 {
			return nil, ErrInvalidRate
		}
		items = append(items, model.DiscountSchemeItem{AgentLevelID: raw.AgentLevelID, DiscountRate: raw.DiscountRate})
	}
	return items, nil
}

// checkGroupBindable 一个商品分组至多被一个折扣组绑定（截图中每行绑定一个分组的语义）。
func (s *discountSchemeService) checkGroupBindable(ctx context.Context, groupID uint64, excludeSchemeID uint64) error {
	if groupID == 0 {
		return nil
	}
	if _, err := s.repo.FindGroupByID(ctx, groupID); err != nil {
		return fmt.Errorf("%w：商品分组不存在", ErrInvalidAssignTarget)
	}
	bound, err := s.repo.CountSchemesOnGroup(ctx, groupID, excludeSchemeID)
	if err != nil {
		return err
	}
	if bound > 0 {
		return repository.ErrGroupBound
	}
	return nil
}

func (s *discountSchemeService) Create(ctx context.Context, req dto.SchemeRequest) (*dto.SchemeInfo, error) {
	if err := s.checkGroupBindable(ctx, req.ProductGroupID, 0); err != nil {
		return nil, err
	}
	items, err := buildSchemeItems(req.Items)
	if err != nil {
		return nil, err
	}
	scheme := &model.DiscountScheme{
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Description: req.Description,
		Status:      normalizeSchemeStatus(req.Status),
	}
	if req.ProductGroupID > 0 {
		scheme.ProductGroupID = &req.ProductGroupID
	}
	if err := s.repo.CreateScheme(ctx, scheme, items); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, scheme.ID)
}

func (s *discountSchemeService) Update(ctx context.Context, id uint64, req dto.SchemeRequest) (*dto.SchemeInfo, error) {
	scheme, err := s.repo.FindSchemeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkGroupBindable(ctx, req.ProductGroupID, id); err != nil {
		return nil, err
	}
	var items []model.DiscountSchemeItem
	replace := false
	if req.Items != nil {
		items, err = buildSchemeItems(req.Items)
		if err != nil {
			return nil, err
		}
		replace = true
	}
	scheme.Name = strings.TrimSpace(req.Name)
	scheme.Code = strings.TrimSpace(req.Code)
	scheme.Description = req.Description
	scheme.Status = normalizeSchemeStatus(req.Status)
	if req.ProductGroupID > 0 {
		scheme.ProductGroupID = &req.ProductGroupID
	} else {
		scheme.ProductGroupID = nil
	}
	if err := s.repo.UpdateScheme(ctx, scheme, items, replace); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *discountSchemeService) Delete(ctx context.Context, id uint64) error {
	return s.repo.DeleteScheme(ctx, id)
}

// Apply 把折扣组展开写入绑定的商品分组：折扣组（等级→费率）× 商品分组（分类/商品集合）
// = 逐格矩阵，复用 ApplyRateCells 的成本线/单调性/目标校验。
func (s *discountSchemeService) Apply(ctx context.Context, id uint64) (*dto.MatrixResponse, error) {
	scheme, err := s.repo.FindSchemeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if scheme.ProductGroupID == nil || *scheme.ProductGroupID == 0 {
		return nil, ErrSchemeNoGroup
	}
	groupItems, err := s.repo.GroupItems(ctx, *scheme.ProductGroupID)
	if err != nil {
		return nil, err
	}
	if len(groupItems) == 0 {
		return nil, ErrSchemeGroupEmpty
	}
	schemeItems, err := s.repo.SchemeItems(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	cells := make([]model.AgentLevelDiscount, 0, len(schemeItems)*len(groupItems))
	for _, item := range schemeItems {
		if item.DiscountRate <= 0 {
			continue // 0 = 该等级在此方案不打折，应用时跳过
		}
		for _, target := range groupItems {
			cells = append(cells, model.AgentLevelDiscount{
				AgentLevelID: item.AgentLevelID,
				TargetType:   target.TargetType,
				TargetID:     target.TargetID,
				DiscountRate: item.DiscountRate,
			})
		}
	}
	if len(cells) == 0 {
		return nil, ErrSchemeNoRates
	}
	return s.levels.ApplyRateCells(ctx, cells)
}
