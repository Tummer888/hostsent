package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/manager/dto"
	"hostsent/backend/internal/modules/admin/manager/model"
	"hostsent/backend/internal/modules/admin/manager/repository"
)

// ErrDepartmentInUse 部门下仍有在职员工或被工单分类引用，禁止删除（doc86 §2.1）。
var ErrDepartmentInUse = errors.New("部门下仍有在职员工或被工单分类引用，请先转移后再删除")

// ErrDepartmentHasChildren 部门下仍有子部门，禁止删除。
var ErrDepartmentHasChildren = errors.New("部门下仍有子部门，请先处理子部门")

// DepartmentService 组织部门业务能力（S1 员工体系）。
type DepartmentService interface {
	List(ctx context.Context, query dto.DepartmentListQuery) (*dto.DepartmentListResponse, error)
	Create(ctx context.Context, req dto.DepartmentSaveRequest) (*dto.DepartmentInfo, error)
	Update(ctx context.Context, id uint64, req dto.DepartmentSaveRequest) (*dto.DepartmentInfo, error)
	Delete(ctx context.Context, id uint64) error
	// FindByID 单部门详情（含在职员工数/分类引用数）。
	FindByID(ctx context.Context, id uint64) (*dto.DepartmentInfo, error)
}

type departmentService struct {
	repo    repository.DepartmentRepository
	adminRe repository.AdminRepository
}

// NewDepartmentService 创建部门服务。adminRe 用于回填主管姓名与员工数（可为 nil，缺失时字段留空）。
func NewDepartmentService(repo repository.DepartmentRepository, adminRe repository.AdminRepository) DepartmentService {
	return &departmentService{repo: repo, adminRe: adminRe}
}

func (s *departmentService) List(ctx context.Context, query dto.DepartmentListQuery) (*dto.DepartmentListResponse, error) {
	depts, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	infos := s.toInfos(ctx, depts)
	if query.Flat == 1 {
		return &dto.DepartmentListResponse{Items: infos, Tree: false}, nil
	}
	return &dto.DepartmentListResponse{Items: buildTree(infos), Tree: true}, nil
}

func (s *departmentService) FindByID(ctx context.Context, id uint64) (*dto.DepartmentInfo, error) {
	dept, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	infos := s.toInfos(ctx, []model.Department{*dept})
	return &infos[0], nil
}

func (s *departmentService) Create(ctx context.Context, req dto.DepartmentSaveRequest) (*dto.DepartmentInfo, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		return nil, errors.New("部门名称与编码不能为空")
	}
	if existing, err := s.repo.FindByCode(ctx, req.Code); err == nil && existing != nil {
		return nil, errors.New("部门编码已存在")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := s.validateParent(ctx, req.ParentID, 0); err != nil {
		return nil, err
	}
	kind := normalizeKind(req.Kind)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	dept := &model.Department{
		Name:          req.Name,
		Code:          req.Code,
		Kind:          kind,
		ParentID:      req.ParentID,
		LeaderAdminID: req.LeaderAdminID,
		Remark:        req.Remark,
		SortOrder:     req.SortOrder,
		Status:        status,
	}
	if err := s.repo.Create(ctx, dept); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, dept.ID)
}

func (s *departmentService) Update(ctx context.Context, id uint64, req dto.DepartmentSaveRequest) (*dto.DepartmentInfo, error) {
	dept, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" || req.Code == "" {
		return nil, errors.New("部门名称与编码不能为空")
	}
	// 编码变更需查重（同码不同部门）。
	if req.Code != dept.Code {
		if other, oerr := s.repo.FindByCode(ctx, req.Code); oerr == nil && other != nil && other.ID != id {
			return nil, errors.New("部门编码已存在")
		} else if oerr != nil && !errors.Is(oerr, gorm.ErrRecordNotFound) {
			return nil, oerr
		}
	}
	if err := s.validateParent(ctx, req.ParentID, id); err != nil {
		return nil, err
	}
	dept.Name = req.Name
	dept.Code = req.Code
	dept.Kind = normalizeKind(req.Kind)
	dept.ParentID = req.ParentID
	dept.LeaderAdminID = req.LeaderAdminID
	dept.Remark = req.Remark
	dept.SortOrder = req.SortOrder
	if status := strings.TrimSpace(req.Status); status != "" {
		dept.Status = status
	}
	if err := s.repo.Update(ctx, dept); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *departmentService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	// 前置校验（doc86 §2.1）：有子部门 / 在职员工 / 工单分类引用一律拒绝。
	children, err := s.repo.List(ctx, dto.DepartmentListQuery{})
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.ParentID == id {
			return ErrDepartmentHasChildren
		}
	}
	adminCount, err := s.repo.CountActiveAdmins(ctx, id)
	if err != nil {
		return err
	}
	categoryCount, err := s.repo.CountCategoryRefs(ctx, id)
	if err != nil {
		return err
	}
	if adminCount > 0 || categoryCount > 0 {
		return ErrDepartmentInUse
	}
	return s.repo.Delete(ctx, id)
}

// validateParent 校验父部门存在且不构成自引用/环（自我归属时直接拒绝）。
func (s *departmentService) validateParent(ctx context.Context, parentID, selfID uint64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == selfID {
		return errors.New("上级部门不能是自己")
	}
	parent, err := s.repo.FindByID(ctx, parentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("上级部门不存在")
		}
		return err
	}
	// 仅允许两级：父部门自身不能再有上级，避免出现深树。
	if parent.ParentID != 0 {
		return errors.New("最多支持两级部门")
	}
	return nil
}

// toInfos 批量补齐只读字段（主管姓名/员工数/分类数），避免树组装时逐条查库。
func (s *departmentService) toInfos(ctx context.Context, depts []model.Department) []dto.DepartmentInfo {
	leaderIDs := make([]uint64, 0, len(depts))
	for _, d := range depts {
		if d.LeaderAdminID > 0 {
			leaderIDs = append(leaderIDs, d.LeaderAdminID)
		}
	}
	leaderNames := map[uint64]string{}
	if s.adminRe != nil && len(leaderIDs) > 0 {
		for _, lid := range leaderIDs {
			admin, err := s.adminRe.FindByID(ctx, lid)
			if err != nil || admin == nil {
				continue
			}
			name := admin.RealName
			if name == "" {
				name = admin.Username
			}
			leaderNames[lid] = name
		}
	}
	infos := make([]dto.DepartmentInfo, 0, len(depts))
	for _, d := range depts {
		info := dto.DepartmentInfo{
			ID:            d.ID,
			Name:          d.Name,
			Code:          d.Code,
			Kind:          d.Kind,
			ParentID:      d.ParentID,
			LeaderAdminID: d.LeaderAdminID,
			LeaderName:    leaderNames[d.LeaderAdminID],
			Remark:        d.Remark,
			SortOrder:     d.SortOrder,
			Status:        d.Status,
			CreatedAt:     d.CreatedAt,
		}
		if count, err := s.repo.CountActiveAdmins(ctx, d.ID); err == nil {
			info.AdminCount = count
		}
		if count, err := s.repo.CountCategoryRefs(ctx, d.ID); err == nil {
			info.CategoryCount = count
		}
		infos = append(infos, info)
	}
	return infos
}

// buildTree 将平铺列表组装为两级树：根节点为 ParentID=0 的部门，其余挂到对应父节点。
// 父节点缺失（脏数据）的子部门提升为根，保证不丢数据。
func buildTree(infos []dto.DepartmentInfo) []dto.DepartmentInfo {
	roots := make([]dto.DepartmentInfo, 0, len(infos))
	index := make(map[uint64]int, len(infos))
	for i, info := range infos {
		index[info.ID] = i
	}
	// 先按树的父子关系归位（用索引写回，避免结构体值拷贝丢失 Children）。
	children := make(map[uint64][]dto.DepartmentInfo)
	for _, info := range infos {
		if info.ParentID == 0 {
			roots = append(roots, info)
			continue
		}
		if _, ok := index[info.ParentID]; !ok {
			roots = append(roots, info)
			continue
		}
		children[info.ParentID] = append(children[info.ParentID], info)
	}
	for i := range roots {
		if kids, ok := children[roots[i].ID]; ok {
			roots[i].Children = kids
		}
	}
	return roots
}

// normalizeKind 归一化部门类型，未知值回落 general（不阻断保存）。
func normalizeKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if model.IsValidDepartmentKind(kind) {
		return kind
	}
	return model.DepartmentKindGeneral
}
