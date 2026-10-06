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
	"sort"
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
	// ErrTargetConflict 本折扣组要写的目标已被别的折扣组占用。
	//
	// 为什么必须拦：同一个 (代理分组, 目标) 单元格只有一个值，两个折扣组都写它时后应用的
	// 静默覆盖前面的，矩阵上只看得到一个数字、查不出是谁写的，运营会以为两套折扣都在生效。
	ErrTargetConflict = errors.New("该折扣组绑定的商品与其它折扣组重叠，无法应用")
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

// dedupeUint64 去重并保序（0 视为无效丢弃）。
// 目标唯一键复用同包 targetKey（agent_level_service.go，格式 type:id）。
func dedupeUint64(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
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
	groupIDs, err := s.repo.SchemeGroupIDs(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	info.ProductGroupIDs = groupIDs
	info.ProductGroups = make([]dto.SchemeGroupInfo, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := s.repo.FindGroupByID(ctx, groupID)
		if err != nil {
			// 分组被删（无外键的悬空绑定）：仍然列出，标出来让运营能解绑。
			info.ProductGroups = append(info.ProductGroups, dto.SchemeGroupInfo{
				ID:   groupID,
				Name: fmt.Sprintf("分组 #%d（已删除）", groupID),
			})
			continue
		}
		info.ProductGroups = append(info.ProductGroups, dto.SchemeGroupInfo{
			ID:        group.ID,
			Name:      group.Name,
			Code:      group.Code,
			ItemCount: group.ItemCount,
		})
	}
	// 展开后的去重目标数：这就是「应用」时会写入的格子数基数（× 有费率的代理分组数）。
	targets, err := s.repo.SchemeTargets(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	info.TargetCount = int64(len(targets))
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

// checkGroupsBindable 校验一批商品分组可以绑到本折扣组：
//   - 分组必须存在；
//   - 一个分组至多被一个折扣组绑定（沿用既有约束，避免两个折扣组抢同一批商品）。
//
// 返回去重后的分组 ID（保持调用方顺序）。
func (s *discountSchemeService) checkGroupsBindable(ctx context.Context, groupIDs []uint64, excludeSchemeID uint64) ([]uint64, error) {
	ids := dedupeUint64(groupIDs)
	for _, groupID := range ids {
		if _, err := s.repo.FindGroupByID(ctx, groupID); err != nil {
			return nil, fmt.Errorf("%w：商品分组 #%d 不存在", ErrInvalidAssignTarget, groupID)
		}
		bound, err := s.repo.CountSchemesOnGroup(ctx, groupID, excludeSchemeID)
		if err != nil {
			return nil, err
		}
		if bound > 0 {
			return nil, fmt.Errorf("%w（分组 #%d 已属于其它折扣组，请先从那一组解绑）", repository.ErrGroupBound, groupID)
		}
	}
	return ids, nil
}

func (s *discountSchemeService) Create(ctx context.Context, req dto.SchemeRequest) (*dto.SchemeInfo, error) {
	groupIDs, err := s.checkGroupsBindable(ctx, req.ProductGroupIDs, 0)
	if err != nil {
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
	if err := s.repo.CreateScheme(ctx, scheme, items, groupIDs); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, scheme.ID)
}

func (s *discountSchemeService) Update(ctx context.Context, id uint64, req dto.SchemeRequest) (*dto.SchemeInfo, error) {
	scheme, err := s.repo.FindSchemeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 绑定分组为 nil 表示不改；非 nil（含空数组）整体覆盖。
	// 覆盖时若已有绑定关系被移除，那些商品上的旧折扣**留着不动**（改的是归属，
	// 不是矩阵），要清掉请去生效矩阵或重新应用。
	var groupIDs []uint64
	replaceGroups := req.ProductGroupIDs != nil
	if replaceGroups {
		groupIDs, err = s.checkGroupsBindable(ctx, req.ProductGroupIDs, id)
		if err != nil {
			return nil, err
		}
	}
	var items []model.DiscountSchemeItem
	replaceItems := false
	if req.Items != nil {
		items, err = buildSchemeItems(req.Items)
		if err != nil {
			return nil, err
		}
		replaceItems = true
	}
	scheme.Name = strings.TrimSpace(req.Name)
	scheme.Code = strings.TrimSpace(req.Code)
	scheme.Description = req.Description
	scheme.Status = normalizeSchemeStatus(req.Status)
	if err := s.repo.UpdateScheme(ctx, scheme, items, replaceItems, groupIDs, replaceGroups); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *discountSchemeService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindSchemeByID(ctx, id); err != nil {
		return err
	}
	// 删除只清掉「配置」，不动已写入的矩阵 —— 矩阵是算价的唯一真相，
	// 删一条配置不该让在售商品的价格凭空变化（要改价请去生效矩阵或重新应用）。
	return s.repo.DeleteScheme(ctx, id)
}

// Apply 把折扣组展开写入它绑定的**全部分组**：
// 折扣组（代理分组→费率）× 绑定的商品分组（并集去重后的目标集合）= 逐格矩阵。
//
// 去重与冲突校验都在这里收口：
//   - 组内去重：同一单品被两个绑定分组各包含一次（一个按分类、一个按单品）时只写一格，
//     否则会撞 agent_level_discounts 的唯一索引让整次应用失败；
//   - 跨组冲突：若本组要写的目标同时被**别的折扣组**绑定，拒绝应用 ——
//     同一格被两个折扣组先后写入，后应用的静默覆盖前面，矩阵上看不出是谁写的。
func (s *discountSchemeService) Apply(ctx context.Context, id uint64) (*dto.MatrixResponse, error) {
	scheme, err := s.repo.FindSchemeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.repo.SchemeGroupIDs(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return nil, ErrSchemeNoGroup
	}
	targets, err := s.repo.SchemeTargets(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, ErrSchemeGroupEmpty
	}
	conflicts, err := s.crossSchemeConflicts(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		return nil, fmt.Errorf("%w：%s", ErrTargetConflict, strings.Join(conflicts, "；"))
	}
	schemeItems, err := s.repo.SchemeItems(ctx, scheme.ID)
	if err != nil {
		return nil, err
	}
	cells := make([]model.AgentLevelDiscount, 0, len(schemeItems)*len(targets))
	for _, item := range schemeItems {
		if item.DiscountRate <= 0 {
			continue // 0 = 该代理分组在此方案不打折，应用时跳过
		}
		for _, target := range targets {
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

// crossSchemeConflicts 报出「本组要写的目标同时属于别的折扣组」的具体冲突。
//
// 用目标展开后的交集判断，而不是「分组是否相同」：两个分组名字与成员都不同，
// 但一个按分类、一个按单品，仍可能落在同一格上（分组 A 含分类「对象存储」、
// 分组 B 含单品「对象存储 100GB」）。这种重叠在配置阶段看不出来，只有展开才暴露，
// 所以这道校验放在「应用」这个真正要写矩阵的时刻。
func (s *discountSchemeService) crossSchemeConflicts(ctx context.Context, schemeID uint64) ([]string, error) {
	sets, err := s.repo.SchemeTargetSets(ctx)
	if err != nil {
		return nil, err
	}
	mine := make(map[string]struct{}, len(sets[schemeID]))
	// 收集冲突目标，稍后统一解析名字。
	collisions := make(map[string]uint64) // targetKey → 抢占它的折扣组 ID
	for _, row := range sets[schemeID] {
		mine[targetKey(row.TargetType, row.TargetID)] = struct{}{}
	}
	if len(mine) == 0 {
		return nil, nil
	}
	categoryIDs := make([]uint64, 0)
	productIDs := make([]uint64, 0)
	for otherID, rows := range sets {
		if otherID == schemeID {
			continue
		}
		for _, row := range rows {
			key := targetKey(row.TargetType, row.TargetID)
			if _, ok := mine[key]; !ok {
				continue
			}
			if _, ok := collisions[key]; ok {
				continue
			}
			collisions[key] = otherID
			switch row.TargetType {
			case model.GroupTargetCategory:
				categoryIDs = append(categoryIDs, row.TargetID)
			case model.GroupTargetProduct:
				productIDs = append(productIDs, row.TargetID)
			}
		}
	}
	if len(collisions) == 0 {
		return nil, nil
	}
	names, err := s.repo.SchemeNames(ctx)
	if err != nil {
		return nil, err
	}
	categoryNames, err := s.repo.CategoryNames(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}
	productNames, err := s.repo.ProductNames(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(collisions))
	for key, otherID := range collisions {
		parts := strings.SplitN(key, ":", 2)
		var id uint64
		if _, err := fmt.Sscan(parts[1], &id); err != nil {
			continue
		}
		out = append(out, fmt.Sprintf("「%s」已被折扣组「%s」占用",
			targetLabel(parts[0], id, categoryNames, productNames), names[otherID]))
	}
	sort.Strings(out)
	return out, nil
}

// targetLabel 生成目标描述（有名字表时用名字，否则退回类型 + ID）。
func targetLabel(targetType string, targetID uint64, categoryNames, productNames map[uint64]string) string {
	switch targetType {
	case model.GroupTargetCategory:
		if name := categoryNames[targetID]; name != "" {
			return "分类·" + name
		}
		return fmt.Sprintf("分类 #%d", targetID)
	case model.GroupTargetProduct:
		if name := productNames[targetID]; name != "" {
			return "商品·" + name
		}
		return fmt.Sprintf("商品 #%d", targetID)
	default:
		return fmt.Sprintf("%s #%d", targetType, targetID)
	}
}
