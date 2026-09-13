package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
	"hostsent/backend/internal/modules/admin/ticket/repository"
)

// CategoryService 定义工单分类管理业务能力。
type CategoryService interface {
	// List 分类列表（管理端含禁用，用户端仅启用）
	List(ctx context.Context, includeDisabled bool) ([]dto.CategoryInfo, error)
	// ListForUser 用户端分类列表（S2）：仅启用分类，附当前用户实名状态与前置条件。
	ListForUser(ctx context.Context, accountID, actorID uint64) (*dto.UserCategoryListResponse, error)
	Create(ctx context.Context, req dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Update(ctx context.Context, id uint64, req dto.CategorySaveRequest) (*dto.CategoryInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
	ticketRepo   repository.TicketRepository
	precondition repository.PreconditionChecker
}

// NewCategoryService 创建工单分类业务服务。precondition 可为 nil（用户端不回显实名状态）。
func NewCategoryService(
	categoryRepo repository.CategoryRepository,
	ticketRepo repository.TicketRepository,
	precondition repository.PreconditionChecker,
) CategoryService {
	return &categoryService{categoryRepo: categoryRepo, ticketRepo: ticketRepo, precondition: precondition}
}

func (s *categoryService) List(ctx context.Context, includeDisabled bool) ([]dto.CategoryInfo, error) {
	items, err := s.categoryRepo.List(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	names := s.departmentNames(ctx, items)
	infos := make([]dto.CategoryInfo, 0, len(items))
	for _, item := range items {
		infos = append(infos, buildCategoryInfo(item, names[item.DepartmentID]))
	}
	return infos, nil
}

// ListForUser 用户端分类列表：仅启用分类，附提交前置条件与实名状态，
// 供前端动态渲染表单（require_binding 时显示关联产品/实例选择器）。
func (s *categoryService) ListForUser(ctx context.Context, accountID, actorID uint64) (*dto.UserCategoryListResponse, error) {
	items, err := s.categoryRepo.List(ctx, false)
	if err != nil {
		return nil, err
	}
	realnameOK := false
	if s.precondition != nil {
		// 实名按归属账号判定：子账号不具备独立实名主体，跟随主账号；
		// 归属账号查不到时再按真实操作人兜底。
		if ok, err := s.precondition.IsRealnameVerified(ctx, accountID); err == nil && ok {
			realnameOK = true
		} else if actorID != accountID {
			if ok, err := s.precondition.IsRealnameVerified(ctx, actorID); err == nil {
				realnameOK = ok
			}
		}
	}
	infos := make([]dto.UserCategoryInfo, 0, len(items))
	for _, item := range items {
		infos = append(infos, dto.UserCategoryInfo{
			ID:              item.ID,
			Name:            item.Name,
			Code:            item.Code,
			Description:     item.Description,
			RequireRealname: item.RequireRealname,
			RequireBinding:  item.RequireBinding,
			NeedReview:      item.NeedReview,
		})
	}
	return &dto.UserCategoryListResponse{Items: infos, RealnameOK: realnameOK}, nil
}

// Create 创建分类：编码唯一性校验。
func (s *categoryService) Create(ctx context.Context, req dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	code := strings.TrimSpace(req.Code)
	if existing, err := s.categoryRepo.FindByCode(ctx, code); err == nil && existing != nil {
		return nil, ErrCategoryCodeExists
	}
	status := req.Status
	if status == "" {
		status = model.CategoryStatusActive
	}
	item := &model.TicketCategory{
		Name:             req.Name,
		Code:             code,
		Description:      req.Description,
		SortOrder:        req.SortOrder,
		Status:           status,
		DefaultRoleCode:  req.DefaultRoleCode,
		DefaultGroupID:   optionalID(req.DefaultGroupID),
		SLAHours:         req.SLAHours,
		DepartmentID:     req.DepartmentID,
		RequireRealname:  req.RequireRealname,
		RequireBinding:   req.RequireBinding,
		NeedReview:       req.NeedReview,
		VisibleRoleCodes: model.EncodeRoleCodes(req.VisibleRoleCodes),
	}
	if err := s.categoryRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	info := buildCategoryInfo(*item, s.departmentName(ctx, item.DepartmentID))
	return &info, nil
}

// Update 更新分类：编码唯一性校验（排除自身）。
func (s *categoryService) Update(ctx context.Context, id uint64, req dto.CategorySaveRequest) (*dto.CategoryInfo, error) {
	item, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapCategoryErr(err)
	}
	code := strings.TrimSpace(req.Code)
	if existing, err := s.categoryRepo.FindByCode(ctx, code); err == nil && existing != nil && existing.ID != id {
		return nil, ErrCategoryCodeExists
	}
	item.Name = req.Name
	item.Code = code
	item.Description = req.Description
	item.SortOrder = req.SortOrder
	item.DefaultRoleCode = req.DefaultRoleCode
	item.DefaultGroupID = optionalID(req.DefaultGroupID)
	item.SLAHours = req.SLAHours
	item.DepartmentID = req.DepartmentID
	item.RequireRealname = req.RequireRealname
	item.RequireBinding = req.RequireBinding
	item.NeedReview = req.NeedReview
	item.VisibleRoleCodes = model.EncodeRoleCodes(req.VisibleRoleCodes)
	if req.Status != "" {
		item.Status = req.Status
	}
	if err := s.categoryRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	info := buildCategoryInfo(*item, s.departmentName(ctx, item.DepartmentID))
	return &info, nil
}

// Delete 删除分类：使用中的分类禁止删除。
func (s *categoryService) Delete(ctx context.Context, id uint64) error {
	category, err := s.categoryRepo.FindByID(ctx, id)
	if err != nil {
		return mapCategoryErr(err)
	}
	// 引用校验：分类下存在工单则禁止删除
	stats, err := s.ticketRepo.CountByCategory(ctx)
	if err != nil {
		return err
	}
	for _, stat := range stats {
		if stat.Category == category.Code && stat.Count > 0 {
			return ErrCategoryInUse
		}
	}
	return s.categoryRepo.Delete(ctx, id)
}

// departmentNames 批量取部门 id → 名称（列表避免 N+1）。
func (s *categoryService) departmentNames(ctx context.Context, items []model.TicketCategory) map[uint64]string {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.DepartmentID > 0 {
			ids = append(ids, item.DepartmentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := s.ticketRepo.DepartmentNameMap(ctx, ids)
	if err != nil {
		return nil
	}
	return names
}

func (s *categoryService) departmentName(ctx context.Context, id uint64) string {
	if id == 0 {
		return ""
	}
	names, err := s.ticketRepo.DepartmentNameMap(ctx, []uint64{id})
	if err != nil {
		return ""
	}
	return names[id]
}

func buildCategoryInfo(item model.TicketCategory, departmentName string) dto.CategoryInfo {
	var groupID uint64
	if item.DefaultGroupID != nil {
		groupID = *item.DefaultGroupID
	}
	return dto.CategoryInfo{
		ID:               item.ID,
		Name:             item.Name,
		Code:             item.Code,
		Description:      item.Description,
		SortOrder:        item.SortOrder,
		Status:           item.Status,
		DefaultRoleCode:  item.DefaultRoleCode,
		DefaultGroupID:   groupID,
		DepartmentID:     item.DepartmentID,
		DepartmentName:   departmentName,
		RequireRealname:  item.RequireRealname,
		RequireBinding:   item.RequireBinding,
		NeedReview:       item.NeedReview,
		VisibleRoleCodes: item.RoleCodeList(),
		SLAHours:         item.SLAHours,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
}

// optionalID 把 0 值转为 nil，避免写入无意义的默认客服组。
func optionalID(id uint64) *uint64 {
	if id == 0 {
		return nil
	}
	return &id
}

func mapCategoryErr(err error) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return ErrCategoryNotFound
	}
	return err
}
