// Package service 提供用户等级模块的业务编排。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"hostsent/backend/internal/modules/admin/user/level/dto"
	"hostsent/backend/internal/modules/admin/user/level/model"
	"hostsent/backend/internal/modules/admin/user/level/repository"
)

// UserLevelService 定义用户等级的业务能力。
type UserLevelService interface {
	List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.Info, error)
	Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error)
	Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error)
	Delete(ctx context.Context, id uint64) error
	// DefaultLevelID 返回新账号的起始等级（启用中、权重最低的一级）。
	// 无可用等级时返回 0，调用方按「不设等级」处理（不阻断建号）。
	DefaultLevelID(ctx context.Context) (uint64, error)
	// AssignLevel 管理员手工调整某用户的等级，并写一条变更日志。
	//
	// 与消费升级（LevelUpgradeService.Recalculate）是两条不同通道：
	// 那条只升不降、由累计消费驱动；这条允许升降、由人工决定并留痕。
	// 两条都写 user_level_change_logs，靠 reason 区分
	// （consume_upgrade / manual_adjust）。
	AssignLevel(ctx context.Context, userID, levelID uint64) error
	// CheckAssignable 校验等级存在且启用中（建号/改等级前的先验）。
	CheckAssignable(ctx context.Context, levelID uint64) error
}

type userLevelService struct {
	repo repository.UserLevelRepository
}

// NewUserLevelService 创建用户等级业务服务。
func NewUserLevelService(repo repository.UserLevelRepository) UserLevelService {
	return &userLevelService{repo: repo}
}

func (s *userLevelService) List(ctx context.Context, query dto.ListQuery) (*dto.ListResponse, error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.Info, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toInfo(item))
	}
	return &dto.ListResponse{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

func (s *userLevelService) FindByID(ctx context.Context, id uint64) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Create(ctx context.Context, req dto.CreateRequest) (*dto.Info, error) {
	status := req.Status
	if status == "" {
		status = "active"
	}
	item := &model.UserLevel{
		Name:             req.Name,
		Code:             req.Code,
		Weight:           req.Weight,
		Status:           status,
		FeatureFlags:     req.FeatureFlags,
		UpgradeCondition: req.UpgradeCondition,
		UpgradeThreshold: req.UpgradeThreshold,
		MaxSubAccounts:   req.MaxSubAccounts,
		Benefits:         normalizeBenefits(req.Benefits),
		Description:      req.Description,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Update(ctx context.Context, id uint64, req dto.UpdateRequest) (*dto.Info, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.Code = req.Code
	item.Weight = req.Weight
	item.Status = req.Status
	item.FeatureFlags = req.FeatureFlags
	item.UpgradeCondition = req.UpgradeCondition
	item.UpgradeThreshold = req.UpgradeThreshold
	item.MaxSubAccounts = req.MaxSubAccounts
	item.Benefits = normalizeBenefits(req.Benefits)
	item.Description = req.Description
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	resp := toInfo(*item)
	return &resp, nil
}

func (s *userLevelService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// normalizeBenefits 把 benefits 收敛成合法 JSONB 文本。
//
// benefits 列是 jsonb，写入非 JSON 文本会直接抛
// `invalid input syntax for type json (SQLSTATE 22P02)`，而 handler 把它当
// 500 回给前端 —— 运营在「新建等级」里留空权益，看到的是「服务器内部错误」
// （空串不是合法 JSON，这一点与 SQL 的 NULL 直觉相反）。
// 空值/空白 → {"benefits":[]}（无权益）；已是 JSON 对象/数组 → 原样；
// 其余（纯文本、逗号/换行分隔）→ 包成 {"benefits":[...]}，与 seed 的存法一致。
func normalizeBenefits(raw string) string {
	const emptyBenefits = `{"benefits":[]}`
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return emptyBenefits
	}
	var probe any
	if err := json.Unmarshal([]byte(trimmed), &probe); err == nil {
		switch probe.(type) {
		case map[string]any, []any:
			return trimmed
		}
	}
	items := make([]string, 0, 4)
	for _, line := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == ',' || r == '，'
	}) {
		if item := strings.TrimSpace(line); item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return emptyBenefits
	}
	encoded, err := json.Marshal(map[string]any{"benefits": items})
	if err != nil {
		return emptyBenefits
	}
	return string(encoded)
}

// DefaultLevelID 返回起始等级：启用中权重最低的一级（当前即白银会员）。
//
// 用「权重最低」而不是写死 code：运营完全可以改名或换掉起始等级的编码，
// 写死 code 会让改名当天注册链路整条断掉。
func (s *userLevelService) DefaultLevelID(ctx context.Context) (uint64, error) {
	items, _, err := s.repo.List(ctx, dto.ListQuery{Page: 1, PageSize: 100, Status: "active"})
	if err != nil {
		return 0, err
	}
	best := uint64(0)
	bestWeight := 0
	for _, item := range items {
		if best == 0 || item.Weight < bestWeight {
			best = item.ID
			bestWeight = item.Weight
		}
	}
	return best, nil
}

// AssignLevel 管理员手工调整用户等级（允许升降），并写变更日志。
// 与消费升级的差别只在「谁做决定」：那条由累计消费驱动且只升不降，
// 这条由人工决定。两者都留痕，运营在变更历史里能看出这次是消费升级还是人工调整。
// 目标等级必须存在且启用中：把用户挂到禁用等级上，等级权益页会显示一个
// 运营已经下线的等级，而升级链路又不会把他从那里挪走（只升不降）。
func (s *userLevelService) AssignLevel(ctx context.Context, userID, levelID uint64) error {
	if userID == 0 || levelID == 0 {
		return ErrInvalidAssignTarget
	}
	if err := s.CheckAssignable(ctx, levelID); err != nil {
		return err
	}
	target, err := s.repo.FindByID(ctx, levelID)
	if err != nil {
		return err
	}
	currentID, totalConsume, err := s.repo.GetUserTier(ctx, userID)
	if err != nil {
		return err
	}
	// 等级没变就直接返回：写一条「从 A 调到 A」的日志只会污染变更历史。
	if currentID != nil && *currentID == target.ID {
		return nil
	}
	if err := s.repo.SetUserTier(ctx, userID, target.ID); err != nil {
		return err
	}
	log := &model.UserLevelChangeLog{
		UserID:             userID,
		ToLevelID:          target.ID,
		ToLevelCode:        target.Code,
		TotalConsumeAmount: totalConsume,
		BenefitsSnapshot:   target.Benefits,
		Reason:             model.ReasonManualAdjust,
	}
	if currentID != nil && *currentID > 0 {
		if current, ferr := s.repo.FindByID(ctx, *currentID); ferr == nil {
			log.FromLevelID = &current.ID
			log.FromLevelCode = current.Code
		}
	}
	// 日志失败上抛：等级已经写进去了，但调用方必须知道「这次调整没有留痕」。
	return s.repo.CreateChangeLog(ctx, log)
}

// CheckAssignable 校验等级存在且启用中（建号/改等级前的先验）。
//
// 单独暴露出来是为了让 account 侧能在**落库之前**校验建号时选的等级：
// 等 users 行写进去再校验，被拒的等级会留下一条等级非法的用户行。
func (s *userLevelService) CheckAssignable(ctx context.Context, levelID uint64) error {
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

// 等级人工调整的哨兵错误，handler 据此映射 400。
var (
	// ErrInvalidAssignTarget 用户或等级 ID 非法。
	ErrInvalidAssignTarget = errors.New("用户与等级 ID 不能为空")
	// ErrLevelDisabled 目标等级已停用，不允许指派。
	ErrLevelDisabled = errors.New("目标等级已停用，不能指派")
)

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

func toInfo(item model.UserLevel) dto.Info {
	return dto.Info{
		ID:               item.ID,
		Name:             item.Name,
		Code:             item.Code,
		Weight:           item.Weight,
		Status:           item.Status,
		FeatureFlags:     item.FeatureFlags,
		UpgradeCondition: item.UpgradeCondition,
		UpgradeThreshold: item.UpgradeThreshold,
		MaxSubAccounts:   item.MaxSubAccounts,
		Benefits:         item.Benefits,
		Description:      item.Description,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}
