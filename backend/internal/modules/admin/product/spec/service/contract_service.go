package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	"hostsent/backend/internal/modules/admin/product/spec/model"
	"hostsent/backend/internal/modules/admin/product/spec/repository"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/specatom"
)

// ErrSpecValidationFailed 规格超出原子字典约束（T2.6）。
var ErrSpecValidationFailed = errors.New("规格不符合原子字典约束")

// SpecContractService 规格契约业务能力（原子字典 / 外部规格快照 / 双向绑定）。
type SpecContractService interface {
	ListAtoms(ctx context.Context) ([]dto.SpecAtomInfo, error)
	ValidateSpec(ctx context.Context, req dto.SpecValidateRequest) *dto.SpecValidateResponse

	ListExternalSpecs(ctx context.Context, providerType, status string) ([]dto.ExternalSpecInfo, error)
	UpsertExternalSpec(ctx context.Context, req dto.ExternalSpecUpsertRequest) (*dto.ExternalSpecInfo, error)
	// RegisterUpstreamSpec 代理链路导入上游商品时登记规格快照（T4.3，由 catalog 服务调用）。
	RegisterUpstreamSpec(ctx context.Context, snap UpstreamSpecSnapshot) error

	ListBindings(ctx context.Context, externalSpecID, productSpecID uint64, status string) ([]dto.SpecBindingInfo, error)
	UpsertBinding(ctx context.Context, req dto.SpecBindingUpsertRequest) (*dto.SpecBindingInfo, error)
	ConfirmBinding(ctx context.Context, id uint64, req dto.SpecBindingConfirmRequest, operatorID uint64) (*dto.SpecBindingInfo, error)
}

// UpstreamSpecSnapshot 上游规格快照登记入参（T4.3）。
// 定义在 service 包：catalog 侧只以该结构描述快照，避免反向依赖 spec/dto 的 RawMessage。
type UpstreamSpecSnapshot struct {
	ProviderID   uint64
	ProviderType string
	ExternalID   string
	ExternalName string
	ExternalKind string
	Raw          string
	Normalized   string
}

type specContractService struct {
	repo repository.SpecContractRepository
}

// NewSpecContractService 创建规格契约业务服务。
func NewSpecContractService(repo repository.SpecContractRepository) SpecContractService {
	return &specContractService{repo: repo}
}

func (s *specContractService) ListAtoms(ctx context.Context) ([]dto.SpecAtomInfo, error) {
	items, err := s.repo.ListAtoms(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.SpecAtomInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildAtomInfo(item))
	}
	return resp, nil
}

// ValidateSpec 校验标准规格是否满足字典约束（T2.6）。
// 字典为空时不阻断（预埋期）；有字典时返回全部 Issue 供前端提示。
func (s *specContractService) ValidateSpec(_ context.Context, req dto.SpecValidateRequest) *dto.SpecValidateResponse {
	spec := pkgmodel.StandardProductSpec{
		CPU:       req.CPU,
		Memory:    req.Memory,
		Disk:      req.Disk,
		DiskType:  req.DiskType,
		Bandwidth: req.Bandwidth,
		OS:        req.OS,
		Region:    req.Region,
		Zone:      req.Zone,
		Extra:     req.Extra,
	}
	issues := specatom.Validate(spec)
	return &dto.SpecValidateResponse{
		Valid:  len(issues) == 0,
		Issues: issues,
		Atoms:  specatom.Global().Len(),
	}
}

func (s *specContractService) ListExternalSpecs(ctx context.Context, providerType, status string) ([]dto.ExternalSpecInfo, error) {
	items, err := s.repo.ListExternalSpecs(ctx, providerType, status)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ExternalSpecInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildExternalSpecInfo(item))
	}
	return resp, nil
}

// UpsertExternalSpec 登记/刷新外部规格快照；fingerprint 由 normalized 计算，
// 变化即意味着上游改了规格，用于把关联绑定置 stale（本期只落指纹，P4 再接状态机触发）。
func (s *specContractService) UpsertExternalSpec(ctx context.Context, req dto.ExternalSpecUpsertRequest) (*dto.ExternalSpecInfo, error) {
	kind := strings.TrimSpace(req.ExternalKind)
	if kind == "" {
		kind = model.ExternalKindFlavor
	}
	now := time.Now()
	registered, err := s.register(ctx, upsertSnapshot{
		ProviderID:   req.ProviderID,
		ProviderType: req.ProviderType,
		ExternalID:   req.ExternalID,
		ExternalName: req.ExternalName,
		ExternalKind: kind,
		Raw:          string(req.Raw),
		Normalized:   string(req.Normalized),
		SyncedAt:     now,
	})
	if err != nil {
		return nil, err
	}
	info := buildExternalSpecInfo(*registered)
	return &info, nil
}

// RegisterUpstreamSpec 代理链路导入上游商品时登记规格快照（T4.3）。
// 入参为 JSON 文本，指纹缺失由 normalized 现算；作为 catalog 服务的内部契约。
func (s *specContractService) RegisterUpstreamSpec(ctx context.Context, snap UpstreamSpecSnapshot) error {
	kind := strings.TrimSpace(snap.ExternalKind)
	if kind == "" {
		kind = model.ExternalKindFlavor
	}
	now := time.Now()
	_, err := s.register(ctx, upsertSnapshot{
		ProviderID:   snap.ProviderID,
		ProviderType: snap.ProviderType,
		ExternalID:   snap.ExternalID,
		ExternalName: snap.ExternalName,
		ExternalKind: kind,
		Raw:          snap.Raw,
		Normalized:   snap.Normalized,
		SyncedAt:     now,
	})
	return err
}

// upsertSnapshot 外部规格登记的内部入参（UpsertExternalSpec 与 RegisterUpstreamSpec 共用）。
type upsertSnapshot struct {
	ProviderID   uint64
	ProviderType string
	ExternalID   string
	ExternalName string
	ExternalKind string
	Raw          string
	Normalized   string
	SyncedAt     time.Time
}

// register 统一的落库路径：归一化输入 → 写 external_specs → 回读真实行。
func (s *specContractService) register(ctx context.Context, in upsertSnapshot) (*model.ExternalSpec, error) {
	item := &model.ExternalSpec{
		ProviderID:   in.ProviderID,
		ProviderType: strings.TrimSpace(in.ProviderType),
		ExternalID:   strings.TrimSpace(in.ExternalID),
		ExternalName: in.ExternalName,
		ExternalKind: in.ExternalKind,
		Raw:          jsonOrEmpty(in.Raw),
		Normalized:   jsonOrEmpty(in.Normalized),
		Fingerprint:  fingerprint(in.Normalized),
		Status:       model.ExternalStatusActive,
		SyncedAt:     &in.SyncedAt,
	}
	if err := s.repo.UpsertExternalSpec(ctx, item); err != nil {
		return nil, err
	}
	// 重查以拿到真实 ID 与指纹（FirstOrCreate 后 item 已被填充，但保险起见回读）。
	if stored, err := s.findExternalSpec(ctx, item.ProviderType, item.ExternalID); err == nil {
		return stored, nil
	}
	return item, nil
}

func (s *specContractService) findExternalSpec(ctx context.Context, providerType, externalID string) (*model.ExternalSpec, error) {
	items, err := s.repo.ListExternalSpecs(ctx, providerType, "")
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ExternalID == externalID {
			return &items[i], nil
		}
	}
	return nil, errors.New("external spec not found")
}

func (s *specContractService) ListBindings(ctx context.Context, externalSpecID, productSpecID uint64, status string) ([]dto.SpecBindingInfo, error) {
	items, err := s.repo.ListBindings(ctx, externalSpecID, productSpecID, status)
	if err != nil {
		return nil, err
	}
	// 自营绑定的展示：回填 product_specs.spec_code，便于前端直接显示 SKU 编码。
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.ProductSpecID != nil && *item.ProductSpecID > 0 {
			ids = append(ids, *item.ProductSpecID)
		}
	}
	codes := map[uint64]string{}
	if len(ids) > 0 {
		if got, cerr := s.repo.ProductSpecCodes(ctx, ids); cerr == nil {
			codes = got
		}
	}
	resp := make([]dto.SpecBindingInfo, 0, len(items))
	for _, item := range items {
		info := buildBindingInfo(item)
		if item.ProductSpecID != nil {
			info.ProductSpecCode = codes[*item.ProductSpecID]
		}
		resp = append(resp, info)
	}
	return resp, nil
}

func (s *specContractService) UpsertBinding(ctx context.Context, req dto.SpecBindingUpsertRequest) (*dto.SpecBindingInfo, error) {
	if req.ExternalSpecID == 0 && req.ProductSpecID == 0 {
		return nil, errors.New("绑定对象不能为空：需指定外部规格或商品 SKU")
	}
	item := &model.SpecBinding{
		SpecTemplateID: req.SpecTemplateID,
		Direction:      normalizeDirection(req.Direction),
		PlatformParams: rawOrEmpty(req.PlatformParams),
		MatchType:      normalizeMatchType(req.MatchType),
		Status:         normalizeBindingStatus(req.Status, req.SpecTemplateID),
		Confidence:     clampConfidence(req.Confidence),
		Remark:         req.Remark,
		Priority:       req.Priority,
	}
	if req.ExternalSpecID > 0 {
		v := req.ExternalSpecID
		item.ExternalSpecID = &v
	}
	if req.ProductSpecID > 0 {
		v := req.ProductSpecID
		item.ProductSpecID = &v
	}
	if err := s.repo.UpsertBinding(ctx, item); err != nil {
		return nil, err
	}
	// 回读真实主键后返回（FirstOrCreate 命中已有行时 item.ID 已填充）。
	if stored, err := s.repo.FindBindingByID(ctx, item.ID); err == nil {
		item = stored
	}
	info := buildBindingInfo(*item)
	return &info, nil
}

// ConfirmBinding 人工确认绑定：状态机 → confirmed，并留痕确认人/时间。
func (s *specContractService) ConfirmBinding(ctx context.Context, id uint64, req dto.SpecBindingConfirmRequest, operatorID uint64) (*dto.SpecBindingInfo, error) {
	target, err := s.repo.FindBindingByID(ctx, id)
	if err != nil {
		return nil, errors.New("规格绑定不存在")
	}
	now := time.Now()
	if req.SpecTemplateID > 0 {
		target.SpecTemplateID = req.SpecTemplateID
	}
	if len(req.PlatformParams) > 0 {
		target.PlatformParams = string(req.PlatformParams)
	}
	if req.Remark != "" {
		target.Remark = req.Remark
	}
	target.Status = model.BindingStatusConfirmed
	target.MatchType = model.BindingMatchManual
	target.Confidence = 100
	target.ConfirmedBy = operatorID
	target.ConfirmedAt = &now
	if err := s.repo.UpsertBinding(ctx, target); err != nil {
		return nil, err
	}
	info := buildBindingInfo(*target)
	return &info, nil
}

// ===== 辅助 =====

func buildAtomInfo(item model.SpecAtom) dto.SpecAtomInfo {
	return dto.SpecAtomInfo{
		ID:             item.ID,
		Key:            item.Key,
		Name:           item.Name,
		Unit:           item.Unit,
		ValueType:      item.ValueType,
		EnumValues:     rawOrEmptyPtr(item.EnumValues),
		MinValue:       item.MinValue,
		MaxValue:       item.MaxValue,
		StepValue:      item.StepValue,
		Required:       item.Required,
		Configurable:   item.Configurable,
		AppliesTo:      item.AppliesTo,
		PlatformFields: rawOrEmptyPtr(item.PlatformFields),
		Description:    item.Description,
		SortOrder:      item.SortOrder,
		Status:         item.Status,
	}
}

func buildExternalSpecInfo(item model.ExternalSpec) dto.ExternalSpecInfo {
	info := dto.ExternalSpecInfo{
		ID:           item.ID,
		ProviderID:   item.ProviderID,
		ProviderType: item.ProviderType,
		ExternalID:   item.ExternalID,
		ExternalName: item.ExternalName,
		ExternalKind: item.ExternalKind,
		Raw:          rawOrEmptyPtr(item.Raw),
		Normalized:   rawOrEmptyPtr(item.Normalized),
		Fingerprint:  item.Fingerprint,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
	if item.SyncedAt != nil {
		v := item.SyncedAt.Format(time.RFC3339)
		info.SyncedAt = &v
	}
	return info
}

func buildBindingInfo(item model.SpecBinding) dto.SpecBindingInfo {
	info := dto.SpecBindingInfo{
		ID:             item.ID,
		SpecTemplateID: item.SpecTemplateID,
		Direction:      item.Direction,
		PlatformParams: rawOrEmptyPtr(item.PlatformParams),
		MatchType:      item.MatchType,
		Status:         item.Status,
		Confidence:     item.Confidence,
		ConfirmedBy:    item.ConfirmedBy,
		Remark:         item.Remark,
		Priority:       item.Priority,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
	if item.ExternalSpecID != nil {
		info.ExternalSpecID = *item.ExternalSpecID
	}
	if item.ProductSpecID != nil {
		info.ProductSpecID = *item.ProductSpecID
	}
	if item.ConfirmedAt != nil {
		v := item.ConfirmedAt.Format(time.RFC3339)
		info.ConfirmedAt = &v
	}
	return info
}

// rawOrEmpty 返回合法 JSON 文本；空输入输出 "{}"。
func rawOrEmpty(raw json.RawMessage) string {
	return jsonOrEmpty(string(raw))
}

// jsonOrEmpty 返回合法 JSON 文本（字符串入参）；空输入输出 "{}"。
func jsonOrEmpty(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return "{}"
	}
	return s
}

func rawOrEmptyPtr(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(s)
}

// fingerprint 对归一结果取 SHA-256，作为"上游改了规格"的变更检测依据。
func fingerprint(normalized string) string {
	s := strings.TrimSpace(normalized)
	if s == "" || s == "{}" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func normalizeDirection(v string) string {
	if v == model.BindingDirectionInbound {
		return model.BindingDirectionInbound
	}
	return model.BindingDirectionOutbound
}

func normalizeMatchType(v string) string {
	switch v {
	case model.BindingMatchExact, model.BindingMatchRange:
		return v
	default:
		return model.BindingMatchManual
	}
}

func normalizeBindingStatus(v string, templateID uint64) string {
	switch v {
	case model.BindingStatusUnmapped, model.BindingStatusAutoMapped,
		model.BindingStatusConfirmed, model.BindingStatusStale:
		return v
	}
	if templateID > 0 {
		return model.BindingStatusAutoMapped
	}
	return model.BindingStatusUnmapped
}

func clampConfidence(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
