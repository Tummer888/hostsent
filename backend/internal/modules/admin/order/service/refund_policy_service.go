package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/order/repository"
)

// RefundPolicyService 退款策略业务能力（管理端 CRUD + 匹配判定）。
//
// 同时实现 RefundPolicyMatcher：装配层构造后既交给 handler，也注入订单服务。
type RefundPolicyService interface {
	List(ctx context.Context) (*dto.RefundPolicyListResponse, error)
	Create(ctx context.Context, req dto.RefundPolicyRequest) (*dto.RefundPolicyInfo, error)
	Update(ctx context.Context, id uint64, req dto.RefundPolicyRequest) (*dto.RefundPolicyInfo, error)
	Delete(ctx context.Context, id uint64) error
	// Resolve 按商品解析生效策略（RefundPolicyMatcher）。
	// productID=0（虚拟/无商品订单）或未命中任何策略时返回 nil，调用方按「不可自助退款」处理。
	Resolve(ctx context.Context, productID uint64) (*RefundPolicyDecision, error)
	// SetCategoryNameReader 注入分类名批量读取（装配层调用，列表展示用）。
	SetCategoryNameReader(fn func(ctx context.Context, ids []uint64) (map[uint64]string, error))
}

// RefundPolicyMatcher 策略匹配的最小端口（订单服务只依赖它能做的这一件事）。
type RefundPolicyMatcher interface {
	Resolve(ctx context.Context, productID uint64) (*RefundPolicyDecision, error)
}

// RefundPolicyDecision 生效策略的判定结果（策略命中后的裁剪视图）。
type RefundPolicyDecision struct {
	PolicyID    uint64
	SourceMode  string
	Refundable  bool
	WindowHours int
	AutoApprove bool
}

// CategoryChainReader 分类祖先链读取（装配层注入，避免订单模块依赖商品分类模块）。
//
// 返回从「商品所属分类」到根的完整链路（含自身），用于策略由深到浅匹配：
// 分类越具体优先级越高（如「云主机/香港」覆盖「云主机」的全局口径）。
type CategoryChainReader interface {
	// CategoryChain 返回分类 ID 及其全部祖先（顺序不限，调用方按 ID 集合匹配）。
	CategoryChain(ctx context.Context, categoryID uint64) ([]uint64, error)
}

// ProductPolicyInfoReader 商品策略关联信息读取（来源 + 分类）。
type ProductPolicyInfoReader interface {
	// ProductPolicyInfo 返回商品的 source_mode 与 category_id。
	ProductPolicyInfo(ctx context.Context, productID uint64) (sourceMode string, categoryID uint64, err error)
}

var (
	// ErrRefundPolicyNotFound 退款策略不存在。
	ErrRefundPolicyNotFound = errors.New("退款策略不存在")
	// ErrRefundPolicyDuplicate 同一「来源 + 分类」组合已存在策略。
	ErrRefundPolicyDuplicate = errors.New("该来源与分类组合已存在策略，请直接编辑")
)

type refundPolicyService struct {
	repo       repository.RefundPolicyRepository
	products   ProductPolicyInfoReader
	categories CategoryChainReader
	// categoryNames 可选：回填分类名（未注入时列表不展示分类名）。
	categoryNames func(ctx context.Context, ids []uint64) (map[uint64]string, error)
}

// NewRefundPolicyService 创建退款策略业务服务。
func NewRefundPolicyService(
	repo repository.RefundPolicyRepository,
	products ProductPolicyInfoReader,
	categories CategoryChainReader,
) RefundPolicyService {
	return &refundPolicyService{repo: repo, products: products, categories: categories}
}

// SetCategoryNameReader 注入分类名批量读取（装配层调用）。
func (s *refundPolicyService) SetCategoryNameReader(fn func(ctx context.Context, ids []uint64) (map[uint64]string, error)) {
	s.categoryNames = fn
}

// 策略来源取值（与商品域 catalog model 的 self/upstream 同值）。
//
// 在本域重复声明字面量而非 import 商品域 model：CONTRIBUTING §3 要求订单模块
// 不依赖商品模块的类型，跨域取值由装配层按字符串对齐（同 source_mode 列的既有口径）。
const (
	PolicySourceAny      string = ""         // 不限来源（兜底行）
	PolicySourceSelf     string = "self"     // 自营链路
	PolicySourceUpstream string = "upstream" // 上游转售链路
)

// normalizePolicySource 归一来源取值：只允许 self / upstream / 空（不限）。
func normalizePolicySource(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "":
		return PolicySourceAny, nil
	case PolicySourceSelf:
		return PolicySourceSelf, nil
	case PolicySourceUpstream:
		return PolicySourceUpstream, nil
	default:
		return "", ErrRefundModeInvalid
	}
}

func (s *refundPolicyService) List(ctx context.Context) (*dto.RefundPolicyListResponse, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RefundPolicyInfo, 0, len(items))
	// 批量回填分类名（未注入读取器时跳过，列表仍可用）。
	names := map[uint64]string{}
	if s.categoryNames != nil {
		ids := make([]uint64, 0, len(items))
		for _, it := range items {
			if it.CategoryID > 0 {
				ids = append(ids, it.CategoryID)
			}
		}
		if len(ids) > 0 {
			if got, err := s.categoryNames(ctx, ids); err == nil {
				names = got
			}
		}
	}
	for _, it := range items {
		info := buildRefundPolicyInfo(it)
		info.CategoryName = names[it.CategoryID]
		out = append(out, info)
	}
	return &dto.RefundPolicyListResponse{Items: out}, nil
}

func (s *refundPolicyService) Create(ctx context.Context, req dto.RefundPolicyRequest) (*dto.RefundPolicyInfo, error) {
	source, err := normalizePolicySource(req.SourceMode)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoDuplicate(ctx, 0, source, req.CategoryID); err != nil {
		return nil, err
	}
	item := &model.RefundPolicy{
		SourceMode:  source,
		CategoryID:  req.CategoryID,
		Refundable:  req.Refundable,
		WindowHours: normalizeWindowHours(req.WindowHours),
		AutoApprove: req.AutoApprove,
		Status:      normalizePolicyStatus(req.Status),
		SortOrder:   req.SortOrder,
		Remark:      strings.TrimSpace(req.Remark),
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	info := buildRefundPolicyInfo(*item)
	return &info, nil
}

func (s *refundPolicyService) Update(ctx context.Context, id uint64, req dto.RefundPolicyRequest) (*dto.RefundPolicyInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefundPolicyNotFound
		}
		return nil, err
	}
	source, err := normalizePolicySource(req.SourceMode)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoDuplicate(ctx, id, source, req.CategoryID); err != nil {
		return nil, err
	}
	item.SourceMode = source
	item.CategoryID = req.CategoryID
	item.Refundable = req.Refundable
	item.WindowHours = normalizeWindowHours(req.WindowHours)
	item.AutoApprove = req.AutoApprove
	item.Status = normalizePolicyStatus(req.Status)
	item.SortOrder = req.SortOrder
	item.Remark = strings.TrimSpace(req.Remark)
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	info := buildRefundPolicyInfo(*item)
	return &info, nil
}

func (s *refundPolicyService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRefundPolicyNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// ensureNoDuplicate 同一「来源 + 分类」只允许一条策略：两条同时命中时口径无法解释，
// 与其在匹配时用排序决定胜负，不如在写入时就拒绝（运营改现有行即可）。
func (s *refundPolicyService) ensureNoDuplicate(ctx context.Context, excludeID uint64, source string, categoryID uint64) error {
	items, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.ID == excludeID {
			continue
		}
		if it.SourceMode == source && it.CategoryID == categoryID {
			return ErrRefundPolicyDuplicate
		}
	}
	return nil
}

func normalizeWindowHours(hours int) int {
	if hours < 0 {
		return 0
	}
	return hours
}

func normalizePolicyStatus(status string) string {
	switch strings.TrimSpace(status) {
	case model.RefundPolicyStatusDisabled:
		return model.RefundPolicyStatusDisabled
	default:
		return model.RefundPolicyStatusActive
	}
}

func buildRefundPolicyInfo(item model.RefundPolicy) dto.RefundPolicyInfo {
	return dto.RefundPolicyInfo{
		ID:          item.ID,
		SourceMode:  item.SourceMode,
		CategoryID:  item.CategoryID,
		Refundable:  item.Refundable,
		WindowHours: item.WindowHours,
		AutoApprove: item.AutoApprove,
		Status:      item.Status,
		SortOrder:   item.SortOrder,
		Remark:      item.Remark,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
}

// Resolve 按商品解析生效退款策略（RefundPolicyMatcher 实现）。
//
// 匹配算法（从具体到宽泛，首个命中即生效）：
//  1. 取商品的 source_mode 与 category_id；
//  2. 展开分类祖先链（含自身），逐级尝试「同来源 + 该分类」；
//     链上按「离商品最近的分类优先」——越具体的分类优先级越高；
//  3. 回落「同来源 + 全部分类」（category_id=0）；
//  4. 仍无命中 → nil（调用方按不可自助退款处理）。
//
// 只考虑 status=active 的策略行。
func (s *refundPolicyService) Resolve(ctx context.Context, productID uint64) (*RefundPolicyDecision, error) {
	if productID == 0 || s.products == nil {
		return nil, nil
	}
	sourceMode, categoryID, err := s.products.ProductPolicyInfo(ctx, productID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]model.RefundPolicy, 0, len(items))
	for _, it := range items {
		if it.Status == model.RefundPolicyStatusActive {
			active = append(active, it)
		}
	}
	if len(active) == 0 {
		return nil, nil
	}

	// 分类链：从商品分类逐级向上（具体 → 宽泛）。
	var chain []uint64
	if categoryID > 0 && s.categories != nil {
		if got, cerr := s.categories.CategoryChain(ctx, categoryID); cerr == nil {
			chain = got
		}
	}
	// 1) 同来源 + 链上分类（链序即优先级）。
	for _, cid := range chain {
		if hit := matchPolicy(active, sourceMode, cid, true); hit != nil {
			return decisionFromPolicy(*hit), nil
		}
	}
	// 2) 同来源 + 全部分类（category_id=0）。
	if hit := matchPolicy(active, sourceMode, 0, false); hit != nil {
		return decisionFromPolicy(*hit), nil
	}
	return nil, nil
}

// matchPolicy 在策略集合里找「来源 + 分类」精确命中。
// exactCategory=true 时要求 category_id 等于给定值；false 时取 category_id=0 的兜底行。
func matchPolicy(items []model.RefundPolicy, sourceMode string, categoryID uint64, exactCategory bool) *model.RefundPolicy {
	for i := range items {
		it := items[i]
		if it.SourceMode != sourceMode {
			continue
		}
		if exactCategory {
			if it.CategoryID == categoryID && it.CategoryID != 0 {
				return &items[i]
			}
			continue
		}
		if it.CategoryID == 0 {
			return &items[i]
		}
	}
	return nil
}

func decisionFromPolicy(p model.RefundPolicy) *RefundPolicyDecision {
	return &RefundPolicyDecision{
		PolicyID:    p.ID,
		SourceMode:  p.SourceMode,
		Refundable:  p.Refundable,
		WindowHours: p.WindowHours,
		AutoApprove: p.AutoApprove,
	}
}
