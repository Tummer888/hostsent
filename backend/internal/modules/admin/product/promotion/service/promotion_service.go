package service

import (
	"context"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/promotion/dto"
	"hostsent/backend/internal/modules/admin/product/promotion/model"
	"hostsent/backend/internal/modules/admin/product/promotion/repository"
)

// CouponService 定义优惠券业务能力。
type CouponService interface {
	List(ctx context.Context, query dto.CouponQuery) (*dto.CouponListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.CouponInfo, error)
	Create(ctx context.Context, req dto.CouponRequest) (*dto.CouponInfo, error)
	Update(ctx context.Context, id uint64, req dto.CouponRequest) (*dto.CouponInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type couponService struct {
	couponRepo repository.CouponRepository
	grantRepo  repository.CouponGrantRepository
}

func NewCouponService(couponRepo repository.CouponRepository, grantRepo repository.CouponGrantRepository) CouponService {
	return &couponService{couponRepo: couponRepo, grantRepo: grantRepo}
}

func (s *couponService) List(ctx context.Context, query dto.CouponQuery) (*dto.CouponListResponse, error) {
	items, total, err := s.couponRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.CouponInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildCouponInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.CouponListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *couponService) FindByID(ctx context.Context, id uint64) (*dto.CouponInfo, error) {
	item, err := s.couponRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildCouponInfo(*item)
	return &info, nil
}

func (s *couponService) Create(ctx context.Context, req dto.CouponRequest) (*dto.CouponInfo, error) {
	code := strings.TrimSpace(req.CouponCode)
	if code == "" {
		code = couponCode(req.CouponType)
	}
	item := &model.Coupon{
		CouponCode:   code,
		Name:         strings.TrimSpace(req.Name),
		CouponType:   req.CouponType,
		Amount:       req.Amount,
		MinAmount:    req.MinAmount,
		Discount:     req.Discount,
		TotalStock:   req.TotalStock,
		PerUserLimit: req.PerUserLimit,
		Scope:        req.Scope,
		ValidFrom:    parseTime(req.ValidFrom),
		ValidTo:      parseTime(req.ValidTo),
		Status:       req.Status,
	}
	if item.PerUserLimit < 1 {
		item.PerUserLimit = 1
	}
	if item.Status == 0 {
		item.Status = model.CouponStatusActive
	}
	if err := s.couponRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *couponService) Update(ctx context.Context, id uint64, req dto.CouponRequest) (*dto.CouponInfo, error) {
	item, err := s.couponRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.CouponType = req.CouponType
	item.Amount = req.Amount
	item.MinAmount = req.MinAmount
	item.Discount = req.Discount
	item.TotalStock = req.TotalStock
	item.PerUserLimit = req.PerUserLimit
	item.Scope = req.Scope
	item.ValidFrom = parseTime(req.ValidFrom)
	item.ValidTo = parseTime(req.ValidTo)
	item.Status = req.Status
	if err := s.couponRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *couponService) Delete(ctx context.Context, id uint64) error {
	return s.couponRepo.Delete(ctx, id)
}

// CouponGrantService 定义优惠券发放业务能力。
type CouponGrantService interface {
	List(ctx context.Context, query dto.CouponGrantQuery) (*dto.CouponGrantListResponse, error)
	Create(ctx context.Context, req dto.CouponGrantCreateRequest) (int, error)
}

type couponGrantService struct {
	grantRepo  repository.CouponGrantRepository
	couponRepo repository.CouponRepository
}

func NewCouponGrantService(grantRepo repository.CouponGrantRepository, couponRepo repository.CouponRepository) CouponGrantService {
	return &couponGrantService{grantRepo: grantRepo, couponRepo: couponRepo}
}

func (s *couponGrantService) List(ctx context.Context, query dto.CouponGrantQuery) (*dto.CouponGrantListResponse, error) {
	items, total, err := s.grantRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.CouponGrantInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildCouponGrantInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.CouponGrantListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *couponGrantService) Create(ctx context.Context, req dto.CouponGrantCreateRequest) (int, error) {
	coupon, err := s.couponRepo.FindByID(ctx, req.CouponID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, userID := range req.UserIDs {
		if userID == 0 {
			continue
		}
		grant := &model.CouponGrant{
			CouponID:  coupon.ID,
			UserID:    userID,
			Status:    model.CouponGrantIssued,
			ValidFrom: coupon.ValidFrom,
			ValidTo:   coupon.ValidTo,
		}
		if err := s.grantRepo.Create(ctx, grant); err != nil {
			return count, err
		}
		count++
	}
	if count > 0 && coupon.TotalStock > 0 {
		_ = s.couponRepo.AddClaimed(ctx, coupon.ID, count)
	}
	return count, nil
}

// PromotionService 定义促销活动业务能力。
type PromotionService interface {
	List(ctx context.Context, query dto.PromotionQuery) (*dto.PromotionListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.PromotionInfo, error)
	Create(ctx context.Context, req dto.PromotionRequest) (*dto.PromotionInfo, error)
	Update(ctx context.Context, id uint64, req dto.PromotionRequest) (*dto.PromotionInfo, error)
	Delete(ctx context.Context, id uint64) error
}

type promotionService struct {
	repo repository.PromotionRepository
}

func NewPromotionService(repo repository.PromotionRepository) PromotionService {
	return &promotionService{repo: repo}
}

func (s *promotionService) List(ctx context.Context, query dto.PromotionQuery) (*dto.PromotionListResponse, error) {
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.PromotionInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildPromotionInfo(item))
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.PromotionListResponse{Items: resp, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *promotionService) FindByID(ctx context.Context, id uint64) (*dto.PromotionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildPromotionInfo(*item)
	return &info, nil
}

func (s *promotionService) Create(ctx context.Context, req dto.PromotionRequest) (*dto.PromotionInfo, error) {
	item := &model.Promotion{
		Name:          strings.TrimSpace(req.Name),
		PromotionType: req.PromotionType,
		Rule:          req.Rule,
		StartTime:     parseTime(req.StartTime),
		EndTime:       parseTime(req.EndTime),
		SortOrder:     req.SortOrder,
		Status:        req.Status,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *promotionService) Update(ctx context.Context, id uint64, req dto.PromotionRequest) (*dto.PromotionInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = strings.TrimSpace(req.Name)
	item.PromotionType = req.PromotionType
	item.Rule = req.Rule
	item.StartTime = parseTime(req.StartTime)
	item.EndTime = parseTime(req.EndTime)
	item.SortOrder = req.SortOrder
	item.Status = req.Status
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
}

func (s *promotionService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// ===== 辅助函数 =====

func parseTime(val string) *time.Time {
	if strings.TrimSpace(val) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil
	}
	return &t
}

func couponCode(couponType string) string {
	return strings.ToUpper(couponType) + "-" + time.Now().Format("20060102150405")
}

func timeStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func buildCouponInfo(item model.Coupon) dto.CouponInfo {
	return dto.CouponInfo{
		ID:           item.ID,
		CouponCode:   item.CouponCode,
		Name:         item.Name,
		CouponType:   item.CouponType,
		Amount:       item.Amount,
		MinAmount:    item.MinAmount,
		Discount:     item.Discount,
		TotalStock:   item.TotalStock,
		ClaimedCount: item.ClaimedCount,
		PerUserLimit: item.PerUserLimit,
		Scope:        item.Scope,
		ValidFrom:    timeStr(item.ValidFrom),
		ValidTo:      timeStr(item.ValidTo),
		Status:       item.Status,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
	}
}

func buildCouponGrantInfo(item model.CouponGrant) dto.CouponGrantInfo {
	return dto.CouponGrantInfo{
		ID:        item.ID,
		CouponID:  item.CouponID,
		UserID:    item.UserID,
		UserName:  item.UserName,
		Status:    item.Status,
		ValidFrom: timeStr(item.ValidFrom),
		ValidTo:   timeStr(item.ValidTo),
		UsedAt:    timeStr(item.UsedAt),
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}
}

func buildPromotionInfo(item model.Promotion) dto.PromotionInfo {
	return dto.PromotionInfo{
		ID:            item.ID,
		Name:          item.Name,
		PromotionType: item.PromotionType,
		Rule:          item.Rule,
		StartTime:     timeStr(item.StartTime),
		EndTime:       timeStr(item.EndTime),
		SortOrder:     item.SortOrder,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     item.UpdatedAt.Format(time.RFC3339),
	}
}
