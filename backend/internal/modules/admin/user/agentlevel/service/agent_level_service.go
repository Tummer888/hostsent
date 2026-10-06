// Package service 提供代理等级模块的业务编排。
//
// 本模块承载「代理拿货折扣」这一唯一职责（doc108）：用户组只做分类、不打折，
// 折扣一律来自用户等级 → 折扣矩阵 → 算价管线。
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/user/agentlevel/dto"
	"hostsent/backend/internal/modules/admin/user/agentlevel/model"
	"hostsent/backend/internal/modules/admin/user/agentlevel/repository"
	"hostsent/backend/internal/pkg/pricing"
)

// 业务错误（handler 据此映射状态码）。
var (
	// ErrInvalidAssignTarget 用户或等级 ID 非法。
	ErrInvalidAssignTarget = errors.New("用户与代理等级 ID 不能为空")
	// ErrLevelDisabled 目标等级已停用，不允许指派。
	ErrLevelDisabled = errors.New("目标代理等级已停用，不能指派")
	// ErrRateBelowCost 折扣率低于成本率（会亏本卖）。
	ErrRateBelowCost = errors.New("折扣率低于成本率，会亏本销售")
	// ErrRateOverOne 折扣率大于 1，比标价还贵。
	ErrRateOverOne = errors.New("折扣率不能大于 1（100%）")
	// ErrNotMonotonic 同一目标下等级越高折扣反而越差。
	ErrNotMonotonic = errors.New("同一目标下，等级越高折扣必须越优（折扣率越小）")
)

// AgentLevelService 代理等级业务能力。
type AgentLevelService interface {
	List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.Info, error)
	Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error)
	Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error)
	Delete(ctx context.Context, id uint64) error

	// Matrix 折扣矩阵视图：列为代理等级（权重降序），行为目标（分类 + 全站兜底）。
	Matrix(ctx context.Context) (*dto.MatrixResponse, error)
	// PreviewLadder 按「锚点 + 步长」展开各等级折扣率，并给出毛利与约束提示。
	PreviewLadder(ctx context.Context, req dto.LadderPreviewRequest) (*dto.LadderPreviewResponse, error)
	// ApplyLadder 把展开结果写入指定目标下的所有等级（逐格 upsert 后统一校验）。
	ApplyLadder(ctx context.Context, req dto.ApplyLadderRequest) (*dto.MatrixResponse, error)
	// ApplyRateCells 把一批折扣格写入逐格矩阵（折扣组应用与阶梯填充共用的唯一入口：
	// 先整批校验成本/目标/单调性，再统一 upsert，保证不会"写一半被拒"）。
	ApplyRateCells(ctx context.Context, cells []model.AgentLevelDiscount) (*dto.MatrixResponse, error)
	// UpdateCell 更新矩阵单格（折扣率 0 = 清除该格），供矩阵上直接微调一格。
	UpdateCell(ctx context.Context, req dto.CellUpdateRequest) (*dto.MatrixResponse, error)

	// RuleForUser 解析用户的代理折扣规则，供算价管线 AgentRule 使用（P5-03）。
	RuleForUser(ctx context.Context, userID, productID, categoryID uint64) (*pricing.Rule, error)
	// IsAgent 该用户是否代理（agent_level_id 非空即代理，不看等级是否停用）。
	//
	// 与 RuleForUser 的区别是「身份」与「价格」：活动折扣要判断的是「这用户算不算代理」
	// —— 等级停用的代理仍然是代理（只是暂时没有折扣），不该因为等级停用就被活动折扣收编。
	IsAgent(ctx context.Context, userID uint64) (bool, error)
	// CheckAssignable 校验代理等级存在且启用（给用户分配前的先验）。
	CheckAssignable(ctx context.Context, levelID uint64) error

	// —— 代理分组成员（归属管理）——
	// ListMembers 列出该代理分组下的账号；query.Unassigned=true 时跨分组列未归属账号。
	ListMembers(ctx context.Context, levelID uint64, query dto.MemberListQuery) (*dto.MemberListResponse, error)
	// AssignMembers 批量纳入本分组（可含从其它分组转入）。
	AssignMembers(ctx context.Context, levelID uint64, userIDs []uint64) (*dto.MemberAssignResponse, error)
	// RemoveMembers 批量移出本分组（取消代理身份）。
	RemoveMembers(ctx context.Context, levelID uint64, userIDs []uint64) (*dto.MemberAssignResponse, error)
}

type agentLevelService struct {
	repo repository.AgentLevelRepository
}

// NewAgentLevelService 创建代理等级业务服务。
func NewAgentLevelService(repo repository.AgentLevelRepository) AgentLevelService {
	return &agentLevelService{repo: repo}
}

func (s *agentLevelService) List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	memberCounts, err := s.repo.MemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.Info, 0, len(items))
	for _, item := range items {
		info, err := s.toInfo(ctx, item, true)
		if err != nil {
			return nil, err
		}
		info.MemberCount = memberCounts[item.ID]
		respItems = append(respItems, *info)
	}
	return &dto.ListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *agentLevelService) FindByID(ctx context.Context, id uint64) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toInfo(ctx, *item, true)
}

func (s *agentLevelService) Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.AgentLevel{
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Weight:      req.Weight,
		Status:      status,
		Description: req.Description,
	}
	discounts := s.buildDiscounts(0, req.Discounts)
	if err := s.validateDiscounts(ctx, nil, discounts); err != nil {
		return nil, err
	}
	// 单调性必须在**写入前**用「预期状态」判断：新增等级若权重低却折扣更优，
	// 会立刻破坏阶梯（低级代理比高级代理便宜 → 套利口子）。
	// 新建时等级还没有 ID（0），pendingCell.LevelID=0 只用于标记"不与库中任何行去重"。
	if err := s.validateMonotonic(ctx, pendingCells(0, item.Weight, discounts), nil); err != nil {
		return nil, err
	}
	// 等级与矩阵同事务写入：分开写的话第二个语句失败会留下"空等级"。
	if err := s.repo.CreateWithDiscounts(ctx, item, discounts); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *agentLevelService) Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.Code = strings.TrimSpace(req.Code)
	item.Weight = req.Weight
	item.Status = req.Status
	item.Description = req.Description

	// Discounts=nil 表示不改矩阵；非 nil（含空数组）整体覆盖。
	var next []model.AgentLevelDiscount
	if req.Discounts != nil {
		next = s.buildDiscounts(id, req.Discounts)
		if err := s.validateDiscounts(ctx, item, next); err != nil {
			return nil, err
		}
	} else {
		// 不改矩阵时，现有折扣仍参与单调性判断（改权重/停用同样会破坏阶梯）。
		existing, err := s.repo.Discounts(ctx, id)
		if err != nil {
			return nil, err
		}
		next = existing
	}
	// 校验用「预期状态」在写入前完成：权重、状态、矩阵三者任一变都会影响单调性。
	// 放在写入后会留下"接口报 409、数据却已落库"的脏状态（运营会以为没保存，
	// 但算价已经在用错误折扣），所以校验必须先于任何写操作。
	var cells []pendingCell
	var strip map[uint64]bool
	if item.Status == "active" {
		cells = pendingCells(id, item.Weight, next)
	} else {
		// 停用后该等级不参与算价，其折扣行必须从比较集合里剔除，
		// 否则"停用"这个动作本身会被判成破坏单调性。
		strip = map[uint64]bool{id: true}
	}
	if err := s.validateMonotonic(ctx, cells, strip); err != nil {
		return nil, err
	}
	// 等级与矩阵同事务写入（replace = 是否整体覆盖矩阵）。
	if err := s.repo.UpdateWithDiscounts(ctx, item, next, req.Discounts != nil); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *agentLevelService) Delete(ctx context.Context, id uint64) error {
	count, err := s.repo.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrLevelInUse
	}
	return s.repo.Delete(ctx, id)
}

// Matrix 折扣矩阵：列为启用中的等级（权重降序），行为「全站兜底 + 各分类」。
//
// 行只列分类 + 全站，不列全部商品：商品例外走独立列表（Get 详情里的 discounts），
// 否则商品一多矩阵会变成几千列无法浏览。
func (s *agentLevelService) Matrix(ctx context.Context) (*dto.MatrixResponse, error) {
	levels, err := s.repo.ListAllActive(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.repo.AllDiscounts(ctx)
	if err != nil {
		return nil, err
	}
	columns := make([]dto.MatrixColumn, 0, len(levels))
	for _, level := range levels {
		columns = append(columns, dto.MatrixColumn{
			AgentLevelID: level.ID,
			Name:         level.Name,
			Code:         level.Code,
			Weight:       level.Weight,
			Status:       level.Status,
		})
	}
	// 矩阵行 = 全部启用中的分类 ∪ 只被折扣引用的分类（已停用但仍有历史折扣的也要能看见）。
	// 之前只列「配过折扣的分类」，导致一个新分类在矩阵上无处下手 ——
	// 运营必须先去阶梯填充里配一次才能看到行，这不符合直觉。
	referenced := map[uint64]struct{}{}
	for _, item := range all {
		if item.TargetType == model.TargetCategory && item.TargetID > 0 {
			referenced[item.TargetID] = struct{}{}
		}
	}
	active, err := s.repo.ActiveCategories(ctx)
	if err != nil {
		return nil, err
	}
	categoryIDs := make([]uint64, 0, len(referenced)+len(active))
	for _, ref := range active {
		categoryIDs = append(categoryIDs, ref.ID)
		delete(referenced, ref.ID)
	}
	for id := range referenced {
		categoryIDs = append(categoryIDs, id) // 已停用但被引用
	}
	names, err := s.repo.CategoryNames(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}
	costs, err := s.repo.CategoryCostRates(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}

	// 单元格索引：(targetType, targetID, levelID) → rate
	cellIndex := make(map[string]float64, len(all))
	configured := make(map[string]bool, len(all))
	for _, item := range all {
		key := cellKey(item.TargetType, item.TargetID, item.AgentLevelID)
		cellIndex[key] = item.DiscountRate
		configured[key] = true
	}

	rows := make([]dto.MatrixRow, 0, len(categoryIDs)+1)
	// 全站兜底行恒在首位（未配任何分类时运营也得有个地方配基础折扣）。
	rows = append(rows, dto.MatrixRow{
		TargetType: model.TargetAll,
		TargetID:   0,
		TargetName: "全站兜底",
		Cells:      s.buildCells(columns, cellIndex, configured, model.TargetAll, 0),
	})
	for _, id := range categoryIDs {
		rows = append(rows, dto.MatrixRow{
			TargetType: model.TargetCategory,
			TargetID:   id,
			TargetName: names[id],
			CostRate:   costs[id],
			Cells:      s.buildCells(columns, cellIndex, configured, model.TargetCategory, id),
		})
	}

	// 商品例外行：有任一商品级折扣的商品才出现（商品可能上千，不能全列）。
	// 商品级阶梯应用后，这一段就是"每个等级都排好了"的直接证据。
	productIDSet := map[uint64]struct{}{}
	for _, item := range all {
		if item.TargetType == model.TargetProduct && item.TargetID > 0 {
			productIDSet[item.TargetID] = struct{}{}
		}
	}
	productIDs := make([]uint64, 0, len(productIDSet))
	for id := range productIDSet {
		productIDs = append(productIDs, id)
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
	var productRows []dto.MatrixRow
	if len(productIDs) > 0 {
		productNames, err := s.repo.ProductNames(ctx, productIDs)
		if err != nil {
			return nil, err
		}
		productCosts, err := s.repo.ProductCostRates(ctx, productIDs)
		if err != nil {
			return nil, err
		}
		productRows = make([]dto.MatrixRow, 0, len(productIDs))
		for _, id := range productIDs {
			name := productNames[id]
			if name == "" {
				name = fmt.Sprintf("商品 #%d（已删除）", id)
			}
			productRows = append(productRows, dto.MatrixRow{
				TargetType: model.TargetProduct,
				TargetID:   id,
				TargetName: name,
				CostRate:   productCosts[id],
				Cells:      s.buildCells(columns, cellIndex, configured, model.TargetProduct, id),
			})
		}
	}
	return &dto.MatrixResponse{Columns: columns, Rows: rows, ProductRows: productRows}, nil
}

func (s *agentLevelService) buildCells(columns []dto.MatrixColumn, index map[string]float64, configured map[string]bool, targetType string, targetID uint64) []dto.MatrixCell {
	cells := make([]dto.MatrixCell, 0, len(columns))
	for _, col := range columns {
		key := cellKey(targetType, targetID, col.AgentLevelID)
		cells = append(cells, dto.MatrixCell{
			AgentLevelID: col.AgentLevelID,
			DiscountRate: index[key],
			Configured:   configured[key],
		})
	}
	return cells
}

// PreviewLadder 展开阶梯：最优等级 = anchor，每降一级 +step。
//
// 目标可以是全站兜底 / 某分类 / **单个商品**（doc108 §8E）：商品阶梯让运营
// 只填一次锚点+步长，该商品下所有启用等级的折扣一起排好，不必逐等级手填。
func (s *agentLevelService) PreviewLadder(ctx context.Context, req dto.LadderPreviewRequest) (*dto.LadderPreviewResponse, error) {
	levels, err := s.repo.ListAllActive(ctx)
	if err != nil {
		return nil, err
	}
	anchor, err := normalizeRate(req.AnchorRate, true)
	if err != nil {
		return nil, err
	}
	step := req.Step
	if step < 0 {
		return nil, errors.New("步长不能为负数")
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = model.TargetAll
	}
	if targetType == model.TargetAll {
		req.TargetID = 0
	}
	// 成本基准与落库走同一解析（分类 cost_rate / 商品 cost_price÷price），
	// 请求里显式带的 CostRate 仅作兜底 —— 否则会出现"预览说没事、保存被拦"。
	cost := req.CostRate
	if cost <= 0 {
		if c, err := s.costRateOfTarget(ctx, targetType, req.TargetID); err == nil {
			cost = c
		}
	}
	cells := make([]dto.LadderPreviewCell, 0, len(levels))
	warnings := make([]string, 0, 2)
	for i, level := range levels {
		rate := anchor + float64(i)*step
		if rate > 1 {
			rate = 1
			warnings = append(warnings, fmt.Sprintf("按步长外推后 %s 已到 100%% 封顶；建议缩小步长", level.Name))
		}
		margin := 0.0
		feasible := true
		if cost > 0 {
			margin = rate - cost
			if rate < cost {
				feasible = false
				warnings = append(warnings, fmt.Sprintf("%s 的折扣率 %.4f 低于成本率 %.4f，会亏本", level.Name, rate, cost))
			}
		}
		cells = append(cells, dto.LadderPreviewCell{
			AgentLevelID: level.ID,
			Name:         level.Name,
			Weight:       level.Weight,
			DiscountRate: round4(rate),
			GrossMargin:  round4(margin),
			Feasible:     feasible,
		})
	}
	return &dto.LadderPreviewResponse{Cells: cells, Warnings: dedupeStrings(warnings)}, nil
}

// ApplyLadder 把阶梯展开结果写入指定目标的所有启用等级。
func (s *agentLevelService) ApplyLadder(ctx context.Context, req dto.ApplyLadderRequest) (*dto.MatrixResponse, error) {
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = model.TargetAll
	}
	if targetType == model.TargetAll {
		req.TargetID = 0
	}
	levels, err := s.repo.ListAllActive(ctx)
	if err != nil {
		return nil, err
	}
	if len(levels) == 0 {
		return nil, errors.New("没有启用中的代理等级，请先创建代理等级")
	}
	preview, err := s.PreviewLadder(ctx, dto.LadderPreviewRequest{
		AnchorRate: req.AnchorRate,
		Step:       req.Step,
		TargetType: targetType,
		TargetID:   req.TargetID,
	})
	if err != nil {
		return nil, err
	}
	next := make([]model.AgentLevelDiscount, 0, len(preview.Cells))
	for _, cell := range preview.Cells {
		next = append(next, model.AgentLevelDiscount{
			AgentLevelID: cell.AgentLevelID,
			TargetType:   targetType,
			TargetID:     req.TargetID,
			DiscountRate: cell.DiscountRate,
		})
	}
	return s.ApplyRateCells(ctx, next)
}

// ApplyRateCells 把一批折扣格写入逐格矩阵。
//
// 折扣组应用（§8I）与阶梯填充共用这条唯一入口：先整批校验（成本线 / 目标存在 /
// 预期态单调性），全部通过后统一 upsert —— 任何一格违规都整批拒绝，不会写一半。
// 只处理启用中的等级；引用停用/不存在等级的格子整批拒绝，避免静默丢配置。
func (s *agentLevelService) ApplyRateCells(ctx context.Context, cells []model.AgentLevelDiscount) (*dto.MatrixResponse, error) {
	if len(cells) == 0 {
		return s.Matrix(ctx)
	}
	levels, err := s.repo.ListAllActive(ctx)
	if err != nil {
		return nil, err
	}
	weightOf := make(map[uint64]int, len(levels))
	for _, level := range levels {
		weightOf[level.ID] = level.Weight
	}
	pending := make([]pendingCell, 0, len(cells))
	for i := range cells {
		if _, ok := weightOf[cells[i].AgentLevelID]; !ok {
			return nil, fmt.Errorf("%w（等级 #%d 未启用或不存在）", ErrInvalidAssignTarget, cells[i].AgentLevelID)
		}
		cost, err := s.costRateOfTarget(ctx, cells[i].TargetType, cells[i].TargetID)
		if err != nil {
			return nil, err
		}
		if err := s.checkRateAgainstCost(cells[i].DiscountRate, cost); err != nil {
			return nil, err
		}
		pending = append(pending, pendingCell{
			LevelID:    cells[i].AgentLevelID,
			Weight:     weightOf[cells[i].AgentLevelID],
			TargetType: cells[i].TargetType,
			TargetID:   cells[i].TargetID,
			Rate:       cells[i].DiscountRate,
		})
	}
	if err := s.repo.ValidateTargetRefs(ctx, cells); err != nil {
		return nil, err
	}
	if err := s.validateMonotonic(ctx, pending, nil); err != nil {
		return nil, err
	}
	for i := range cells {
		if err := s.repo.UpsertDiscount(ctx, &cells[i]); err != nil {
			return nil, err
		}
	}
	return s.Matrix(ctx)
}

// UpdateCell 更新矩阵单格：rate=0 视为清除该格（不打折）。
//
// 与 Update（整档覆盖）互补：运营在矩阵上看到某格不合适，直接点开改这一格即可，
// 不必把该等级的全部折扣重新提交一遍（那种做法很容易顺手把别的格子改错）。
func (s *agentLevelService) UpdateCell(ctx context.Context, req dto.CellUpdateRequest) (*dto.MatrixResponse, error) {
	level, err := s.repo.FindByID(ctx, req.AgentLevelID)
	if err != nil {
		return nil, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = model.TargetAll
	}
	targetID := req.TargetID
	if targetType == model.TargetAll {
		targetID = 0
	}
	if targetType != model.TargetAll && targetID == 0 {
		return nil, fmt.Errorf("%w：请选择分类或商品", ErrInvalidAssignTarget)
	}

	if req.DiscountRate <= 0 {
		// 0 = 未配置：删除该格，回到「不打折」。
		if err := s.repo.DeleteDiscount(ctx, level.ID, targetType, targetID); err != nil {
			return nil, err
		}
		return s.Matrix(ctx)
	}

	rate, err := normalizeRate(req.DiscountRate, false)
	if err != nil {
		return nil, err
	}
	if rate <= 0 {
		return nil, errors.New("折扣率必须是 0 到 1 之间的数值")
	}
	cell := model.AgentLevelDiscount{
		AgentLevelID: level.ID,
		TargetType:   targetType,
		TargetID:     targetID,
		DiscountRate: rate,
	}
	cost, err := s.costRateOfTarget(ctx, targetType, targetID)
	if err != nil {
		return nil, err
	}
	if err := s.checkRateAgainstCost(rate, cost); err != nil {
		return nil, err
	}
	if err := s.repo.ValidateTargetRefs(ctx, []model.AgentLevelDiscount{cell}); err != nil {
		return nil, err
	}
	// 预期态单调性校验（写入前）：把这一格叠加到当前快照上判断。
	existing, err := s.repo.Discounts(ctx, level.ID)
	if err != nil {
		return nil, err
	}
	next := make([]model.AgentLevelDiscount, 0, len(existing)+1)
	replaced := false
	for _, item := range existing {
		if item.TargetType == targetType && item.TargetID == targetID {
			next = append(next, cell)
			replaced = true
			continue
		}
		next = append(next, item)
	}
	if !replaced {
		next = append(next, cell)
	}
	if err := s.validateMonotonic(ctx, pendingCells(level.ID, level.Weight, next), nil); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertDiscount(ctx, &cell); err != nil {
		return nil, err
	}
	return s.Matrix(ctx)
}

// RuleForUser 解析用户的代理折扣：代理等级 → 折扣矩阵 → 命中的那一条。
//
// 命中优先级 product > category > all。返回 nil 表示不打折（非代理、等级禁用、
// 或未配置），调用方按"无折扣"继续，不阻断下单。
func (s *agentLevelService) RuleForUser(ctx context.Context, userID, productID, categoryID uint64) (*pricing.Rule, error) {
	if userID == 0 {
		return nil, nil
	}
	levelID, err := s.repo.LevelIDOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if levelID == 0 {
		return nil, nil // 非代理
	}
	level, err := s.repo.FindByID(ctx, levelID)
	if err != nil {
		// 等级行被删（无外键时的悬空引用）：按"非代理"处理，不阻断算价。
		return nil, nil
	}
	if level.Status != "active" {
		return nil, nil
	}
	items, err := s.repo.Discounts(ctx, levelID)
	if err != nil {
		return nil, err
	}
	rate, matched := matchDiscount(items, productID, categoryID)
	if !matched {
		return nil, nil
	}
	normalized, err := normalizeRate(rate, false)
	if err != nil || normalized <= 0 {
		return nil, nil
	}
	return &pricing.Rule{
		Source: pricing.SourceAgent,
		Code:   level.Code,
		Type:   pricing.TypeRate,
		Value:  normalized,
	}, nil
}

// IsAgent 该用户是否代理。判据就是 users.agent_level_id 非空 ——
// 等级即使被停用，这仍是一个「登记在册的代理」，活动折扣不该把他当成普通用户。
func (s *agentLevelService) IsAgent(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	levelID, err := s.repo.LevelIDOfUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return levelID > 0, nil
}

// CheckAssignable 校验代理等级存在且启用（给用户分配代理身份前的先验）。
func (s *agentLevelService) CheckAssignable(ctx context.Context, levelID uint64) error {
	if levelID == 0 {
		return ErrInvalidAssignTarget
	}
	target, err := s.repo.FindByID(ctx, levelID)
	if err != nil {
		return err
	}
	if target.Status != "active" {
		return ErrLevelDisabled
	}
	return nil
}

// —— 代理分组成员（归属管理）——
//
// 归属即 users.agent_level_id：面板上的「代理分组」就是在管这条归属。
// 纳入分组不校验等级是否启用 —— 停用只是「不再参与算价」，把一个代理先归到
// 某个分组（哪怕该组暂时停用）是合理的运营动作，不该被接口挡住。

// ListMembers 列出该代理分组的成员；query.Unassigned=true 时列出未归属账号。
func (s *agentLevelService) ListMembers(ctx context.Context, levelID uint64, query dto.MemberListQuery) (*dto.MemberListResponse, error) {
	if !query.Unassigned {
		// 未归属列表与分组无关（一个分组都没建时也该能看候选池）；只有查具体成员时才校验分组。
		if _, err := s.repo.FindByID(ctx, levelID); err != nil {
			return nil, err
		}
	}
	rows, total, err := s.repo.ListMembers(ctx, levelID, query)
	if err != nil {
		return nil, err
	}
	unassigned, err := s.repo.CountUnassigned(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.AgentMemberInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.AgentMemberInfo{
			ID:             row.ID,
			Username:       row.Username,
			RealName:       row.RealName,
			Email:          row.Email,
			Phone:          row.Phone,
			Status:         row.Status,
			AgentLevelID:   row.AgentLevelID,
			AgentLevelName: row.AgentLevelName,
			CreatedAt:      formatTime(row.CreatedAt),
		})
	}
	return &dto.MemberListResponse{
		Items: items,
		Meta: dto.ListMeta{
			Page:     normalizeMetaPage(query.Page),
			PageSize: normalizeMetaSize(query.PageSize),
			Total:    total,
		},
		UnassignedTotal: unassigned,
	}, nil
}

// AssignMembers 把一批账号纳入本分组（覆盖原有归属，即从别组转入）。
func (s *agentLevelService) AssignMembers(ctx context.Context, levelID uint64, userIDs []uint64) (*dto.MemberAssignResponse, error) {
	if _, err := s.repo.FindByID(ctx, levelID); err != nil {
		return nil, err
	}
	skipped, err := s.filterExisting(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	ids := validIDs(userIDs, skipped)
	changed, err := s.repo.AssignMembers(ctx, levelID, ids)
	if err != nil {
		return nil, err
	}
	return &dto.MemberAssignResponse{Changed: int(changed), Skipped: skipped}, nil
}

// RemoveMembers 把一批账号移出本分组（agent_level_id 置空 = 取消代理身份）。
func (s *agentLevelService) RemoveMembers(ctx context.Context, levelID uint64, userIDs []uint64) (*dto.MemberAssignResponse, error) {
	if _, err := s.repo.FindByID(ctx, levelID); err != nil {
		return nil, err
	}
	skipped, err := s.filterExisting(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	ids := validIDs(userIDs, skipped)
	changed, err := s.repo.ClearMembers(ctx, levelID, ids)
	if err != nil {
		return nil, err
	}
	// 少改的那几条 = 已经不在本分组（被别的分组收走、或本来就是非代理）。
	// 逐条回显原因，避免运营以为"点了移出却没生效"。
	if remaining := int64(len(ids)) - changed; remaining > 0 {
		skipped = append(skipped, dto.MemberAssignSkip{
			UserID: 0,
			Reason: fmt.Sprintf("另有 %d 个账号已不在本分组，未做改动", remaining),
		})
	}
	return &dto.MemberAssignResponse{Changed: int(changed), Skipped: skipped}, nil
}

// filterExisting 挑出不存在/已注销的账号，作为可回显的跳过项。
func (s *agentLevelService) filterExisting(ctx context.Context, userIDs []uint64) ([]dto.MemberAssignSkip, error) {
	unique := dedupeIDs(userIDs)
	existing, err := s.repo.ExistingUserIDs(ctx, unique)
	if err != nil {
		return nil, err
	}
	skipped := make([]dto.MemberAssignSkip, 0)
	for _, id := range unique {
		if _, ok := existing[id]; !ok {
			skipped = append(skipped, dto.MemberAssignSkip{UserID: id, Reason: "账号不存在或已注销"})
		}
	}
	return skipped, nil
}

// dedupeIDs 去重并保序。
func dedupeIDs(ids []uint64) []uint64 {
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

// validIDs 去掉被跳过的账号，返回可提交的 ID 列表。
func validIDs(ids []uint64, skipped []dto.MemberAssignSkip) []uint64 {
	if len(skipped) == 0 {
		return dedupeIDs(ids)
	}
	bad := make(map[uint64]struct{}, len(skipped))
	for _, item := range skipped {
		bad[item.UserID] = struct{}{}
	}
	out := make([]uint64, 0, len(ids))
	for _, id := range dedupeIDs(ids) {
		if _, ok := bad[id]; ok {
			continue
		}
		out = append(out, id)
	}
	return out
}

func normalizeMetaPage(page int) int {
	p, _ := normalizeMeta(page, 0)
	return p
}

func normalizeMetaSize(pageSize int) int {
	_, size := normalizeMeta(0, pageSize)
	return size
}

// —— 内部辅助 ——

// matchDiscount 按 product > category > all 取命中的折扣率。
func matchDiscount(items []model.AgentLevelDiscount, productID, categoryID uint64) (float64, bool) {
	var fallback *model.AgentLevelDiscount
	for i := range items {
		item := &items[i]
		switch item.TargetType {
		case model.TargetProduct:
			if productID > 0 && item.TargetID == productID {
				return item.DiscountRate, true
			}
		case model.TargetCategory:
			if categoryID > 0 && item.TargetID == categoryID {
				return item.DiscountRate, true
			}
		case model.TargetAll:
			fallback = item
		}
	}
	if fallback != nil {
		return fallback.DiscountRate, true
	}
	return 0, false
}

// buildDiscounts 把请求条目收敛成模型行（target_type=all 时 target_id 固定 0）。
// levelID 为 0 表示新建（ID 待落库后回填）。
func (s *agentLevelService) buildDiscounts(levelID uint64, req []dto.DiscountItemRequest) []model.AgentLevelDiscount {
	items := make([]model.AgentLevelDiscount, 0, len(req))
	seen := make(map[string]struct{}, len(req))
	for _, raw := range req {
		targetType := strings.TrimSpace(raw.TargetType)
		if targetType == "" {
			targetType = model.TargetAll
		}
		targetID := raw.TargetID
		if targetType == model.TargetAll {
			targetID = 0
		}
		key := cellKey(targetType, targetID, levelID)
		if _, ok := seen[key]; ok {
			// 同目标重复提交：后者覆盖前者，避免撞唯一索引直接 500。
			for i := range items {
				if items[i].TargetType == targetType && items[i].TargetID == targetID {
					items[i].DiscountRate = round4(raw.DiscountRate)
				}
			}
			continue
		}
		seen[key] = struct{}{}
		items = append(items, model.AgentLevelDiscount{
			AgentLevelID: levelID,
			TargetType:   targetType,
			TargetID:     targetID,
			DiscountRate: round4(raw.DiscountRate),
		})
	}
	return items
}

// validateDiscounts 校验单等级的矩阵：折扣率区间 + 目标存在。
// 单调性需要跨等级比较，由 validateMonotonic 统一做。
func (s *agentLevelService) validateDiscounts(ctx context.Context, level *model.AgentLevel, items []model.AgentLevelDiscount) error {
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		if _, err := normalizeRate(items[i].DiscountRate, true); err != nil {
			return err
		}
		items[i].DiscountRate = round4(items[i].DiscountRate)
		// 折扣率必须 ≥ 目标成本率，否则亏本。
		cost, err := s.costRateOfTarget(ctx, items[i].TargetType, items[i].TargetID)
		if err != nil {
			return err
		}
		if err := s.checkRateAgainstCost(items[i].DiscountRate, cost); err != nil {
			return err
		}
	}
	return s.repo.ValidateTargetRefs(ctx, items)
}

// pendingCell 描述"某等级在某个目标上即将生效的折扣率"，用于写入前的预期态校验。
type pendingCell struct {
	LevelID    uint64
	Weight     int
	TargetType string
	TargetID   uint64
	Rate       float64
}

// pendingCells 把模型行转成预期态单元格；levelID=0 用于新建（库里还没有这一行）。
func pendingCells(levelID uint64, weight int, items []model.AgentLevelDiscount) []pendingCell {
	cells := make([]pendingCell, 0, len(items))
	for _, item := range items {
		cells = append(cells, pendingCell{
			LevelID:    levelID,
			Weight:     weight,
			TargetType: item.TargetType,
			TargetID:   item.TargetID,
			Rate:       item.DiscountRate,
		})
	}
	return cells
}

// validateMonotonic 校验同一目标下「权重越高折扣率越小」，判定的是**写入后的
// 预期状态**而不是当前库里的状态。
//
// 为什么必须拦：等级是可比较的有序阶梯，如果核心代理（权重高）拿到的折扣比
// 普通代理还贵，代理会去开小号冒充低级代理 —— 这是定价体系最典型的套利口子。
//
// 为什么是"预期状态"：校验必须发生在写库之前，否则接口回 409 但脏数据已经落库
// （运营看到保存失败，算价却已经按错误折扣在跑）。快照 = 库中现状 − cells 覆盖掉的
// 格子 − strip 里被停用的等级，再叠加 cells。
func (s *agentLevelService) validateMonotonic(ctx context.Context, cells []pendingCell, strip map[uint64]bool) error {
	levels, err := s.repo.ListAllActive(ctx)
	if err != nil {
		return err
	}
	weightOf := make(map[uint64]int, len(levels))
	for _, level := range levels {
		weightOf[level.ID] = level.Weight
	}

	type entry struct {
		weight int
		rate   float64
	}
	// (target) → []{weight, rate}
	grouped := map[string][]entry{}
	add := func(targetType string, targetID uint64, weight int, rate float64) {
		if rate <= 0 {
			return // 未配置不参与比较
		}
		key := targetKey(targetType, targetID)
		grouped[key] = append(grouped[key], entry{weight: weight, rate: rate})
	}

	// 1) 库中现状：剔除将被 cells 覆盖的格子，以及被停用的等级。
	all, err := s.repo.AllDiscounts(ctx)
	if err != nil {
		return err
	}
	overridden := make(map[string]bool, len(cells))
	for _, cell := range cells {
		overridden[cellKey(cell.TargetType, cell.TargetID, cell.LevelID)] = true
	}
	for _, item := range all {
		if overridden[cellKey(item.TargetType, item.TargetID, item.AgentLevelID)] {
			continue
		}
		if strip[item.AgentLevelID] {
			continue
		}
		weight, ok := weightOf[item.AgentLevelID]
		if !ok {
			continue // 已禁用等级不参与
		}
		add(item.TargetType, item.TargetID, weight, item.DiscountRate)
	}

	// 2) 预期新增/覆盖的格子。
	for _, cell := range cells {
		add(cell.TargetType, cell.TargetID, cell.Weight, cell.Rate)
	}

	for key, entries := range grouped {
		for i := 0; i < len(entries); i++ {
			for j := i + 1; j < len(entries); j++ {
				higher, lower := entries[i], entries[j]
				if higher.weight < lower.weight {
					higher, lower = lower, higher
				}
				if higher.rate > lower.rate {
					return fmt.Errorf("%w（目标 %s：权重 %d 的折扣率 %.4f 比权重 %d 的 %.4f 更差）",
						ErrNotMonotonic, key, higher.weight, higher.rate, lower.weight, lower.rate)
				}
			}
		}
	}
	return nil
}

// costRateOfTarget 取目标的成本率：分类读 product_categories.cost_rate，
// 商品按 cost_price/price 折算，全站返回 0（无基准 → 跳过毛利校验）。
func (s *agentLevelService) costRateOfTarget(ctx context.Context, targetType string, targetID uint64) (float64, error) {
	switch targetType {
	case model.TargetCategory:
		if targetID == 0 {
			return 0, nil
		}
		costs, err := s.repo.CategoryCostRates(ctx, []uint64{targetID})
		if err != nil {
			return 0, err
		}
		return costs[targetID], nil
	case model.TargetProduct:
		if targetID == 0 {
			return 0, nil
		}
		costs, err := s.repo.ProductCostRates(ctx, []uint64{targetID})
		if err != nil {
			return 0, err
		}
		return costs[targetID], nil
	default:
		return 0, nil
	}
}

func (s *agentLevelService) checkRateAgainstCost(rate, cost float64) error {
	if cost <= 0 {
		return nil // 无成本基准，跳过（存量分类都未配成本率）
	}
	if rate <= 0 {
		return nil // 未配置
	}
	if rate < cost {
		return fmt.Errorf("%w（折扣率 %.4f < 成本率 %.4f）", ErrRateBelowCost, rate, cost)
	}
	return nil
}

// toInfo 组装展示结构；withDiscounts 为 true 时附上折扣明细（列表页也需要，
// 因为列表要显示"全站兜底折扣"这一列）。
func (s *agentLevelService) toInfo(ctx context.Context, item model.AgentLevel, withDiscounts bool) (*dto.Info, error) {
	memberCounts, err := s.repo.MemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	info := &dto.Info{
		ID:            item.ID,
		Name:          item.Name,
		Code:          item.Code,
		Weight:        item.Weight,
		Status:        item.Status,
		Description:   item.Description,
		MemberCount:   memberCounts[item.ID],
		DiscountCount: item.DiscountCount,
		CreatedAt:     formatTime(item.CreatedAt),
		UpdatedAt:     formatTime(item.UpdatedAt),
	}
	if !withDiscounts {
		return info, nil
	}
	items, err := s.repo.Discounts(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	categoryIDs := make([]uint64, 0)
	productIDs := make([]uint64, 0)
	for _, d := range items {
		switch d.TargetType {
		case model.TargetCategory:
			categoryIDs = append(categoryIDs, d.TargetID)
		case model.TargetProduct:
			productIDs = append(productIDs, d.TargetID)
		}
	}
	categoryNames, err := s.repo.CategoryNames(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}
	productNames, err := s.repo.ProductNames(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	details := make([]dto.DiscountItemInfo, 0, len(items))
	for _, d := range items {
		name := "全站兜底"
		switch d.TargetType {
		case model.TargetCategory:
			name = categoryNames[d.TargetID]
			if name == "" {
				name = fmt.Sprintf("分类 #%d（已删除）", d.TargetID)
			}
		case model.TargetProduct:
			name = productNames[d.TargetID]
			if name == "" {
				name = fmt.Sprintf("商品 #%d（已删除）", d.TargetID)
			}
		}
		details = append(details, dto.DiscountItemInfo{
			TargetType:   d.TargetType,
			TargetID:     d.TargetID,
			TargetName:   name,
			DiscountRate: d.DiscountRate,
		})
	}
	info.Discounts = details
	// 折扣项数直接取明细条数：FindByID 路径没有跑聚合查询，之前详情里恒为 0。
	info.DiscountCount = int64(len(details))
	return info, nil
}

// normalizeRate 归一折扣率：必须落在 (0, 1]；0 视为"未配置"是允许的，
// 因此 requirePositive=false 时（算价路径）0 直接当作未配置返回。
func normalizeRate(raw float64, rejectZero bool) (float64, error) {
	if math.IsNaN(raw) || math.IsInf(raw, 0) || raw < 0 {
		return 0, errors.New("折扣率必须是 0 到 1 之间的数值")
	}
	if raw > 1 {
		return 0, ErrRateOverOne
	}
	if raw == 0 && rejectZero {
		return 0, nil // 未配置：合法，表示该格不打折
	}
	return round4(raw), nil
}

func cellKey(targetType string, targetID, levelID uint64) string {
	return fmt.Sprintf("%s:%d:%d", targetType, targetID, levelID)
}

func targetKey(targetType string, targetID uint64) string {
	return fmt.Sprintf("%s:%d", targetType, targetID)
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeMeta(page, pageSize int) (int, int) {
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

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
