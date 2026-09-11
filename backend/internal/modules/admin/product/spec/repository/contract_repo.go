package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/product/spec/model"
	"hostsent/backend/internal/pkg/specatom"
)

// SpecContractRepository 规格契约（原子字典 / 外部规格 / 绑定）数据访问。
type SpecContractRepository interface {
	// ListAtoms 读取全部启用的规格原子。
	ListAtoms(ctx context.Context) ([]model.SpecAtom, error)
	CreateAtom(ctx context.Context, item *model.SpecAtom) error
	UpdateAtom(ctx context.Context, item *model.SpecAtom) error
	FindAtomByKey(ctx context.Context, key string) (*model.SpecAtom, error)

	// ListExternalSpecs 分页查询外部规格快照。
	ListExternalSpecs(ctx context.Context, providerType string, status string) ([]model.ExternalSpec, error)
	// UpsertExternalSpec 按 (provider_type, external_id) 幂等写入外部规格。
	UpsertExternalSpec(ctx context.Context, item *model.ExternalSpec) error

	// ListBindings 查询绑定（可按外部规格 / 商品 SKU / 状态过滤，T4.2 起支持自营锚点）。
	ListBindings(ctx context.Context, externalSpecID, productSpecID uint64, status string) ([]model.SpecBinding, error)
	// ListBindingsByProductSpecIDs 批量查询若干 SKU 的绑定（自营链路列表展示，T4.2）。
	ListBindingsByProductSpecIDs(ctx context.Context, productSpecIDs []uint64) ([]model.SpecBinding, error)
	// ProductSpecCodes 批量取 SKU 编码：product_specs.id → spec_code（绑定列表回填展示用）。
	ProductSpecCodes(ctx context.Context, productSpecIDs []uint64) (map[uint64]string, error)
	// BindingStatusByProductSpecs 批量取 SKU 的出站绑定状态：product_spec_id → status。
	// 同一 SKU 有多条时取 priority 最高者。返回 map 便于 catalog 列表回填绑定状态。
	BindingStatusByProductSpecs(ctx context.Context, productSpecIDs []uint64) (map[uint64]string, error)
	// FindConfirmedBindingByProductSpec 取某 SKU 已确认的出站绑定；不存在返回 (nil,nil)。
	FindConfirmedBindingByProductSpec(ctx context.Context, productSpecID uint64) (*model.SpecBinding, error)
	// ConfirmedPlatformParamsByProductSpec 取某 SKU 已确认绑定的 platform_params JSON；
	// 无绑定返回空串。实现 catalog 侧 SpecBindingReader（仅返回字符串，避免跨包依赖模型）。
	ConfirmedPlatformParamsByProductSpec(ctx context.Context, productSpecID uint64) (string, error)
	// HasConfirmedBindingForExternal 判断某外部规格是否存在已确认的入站/出站绑定
	// （代理商品上架门禁用，T4.3/T4.6）：provider_type + external_id 定位 external_specs。
	HasConfirmedBindingForExternal(ctx context.Context, providerType, externalID string) (bool, error)
	// FindBindingByID 按主键取单条绑定（确认状态机用，避免全表扫描）。
	FindBindingByID(ctx context.Context, id uint64) (*model.SpecBinding, error)
	// UpsertBinding 按 (external_spec_id | product_spec_id, direction) 幂等写入绑定。
	UpsertBinding(ctx context.Context, item *model.SpecBinding) error
}

type specContractRepository struct {
	db *gorm.DB
}

// NewSpecContractRepository 创建规格契约仓储。
func NewSpecContractRepository(db *gorm.DB) SpecContractRepository {
	return &specContractRepository{db: db}
}

func (r *specContractRepository) ListAtoms(ctx context.Context) ([]model.SpecAtom, error) {
	var items []model.SpecAtom
	if err := r.db.WithContext(ctx).Where("status = ?", 1).
		Order("sort_order asc, key asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *specContractRepository) CreateAtom(ctx context.Context, item *model.SpecAtom) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *specContractRepository) UpdateAtom(ctx context.Context, item *model.SpecAtom) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *specContractRepository) FindAtomByKey(ctx context.Context, key string) (*model.SpecAtom, error) {
	var item model.SpecAtom
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *specContractRepository) ListExternalSpecs(ctx context.Context, providerType, status string) ([]model.ExternalSpec, error) {
	base := r.db.WithContext(ctx).Model(&model.ExternalSpec{})
	if providerType != "" {
		base = base.Where("provider_type = ?", providerType)
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var items []model.ExternalSpec
	if err := base.Order("id desc").Limit(500).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *specContractRepository) UpsertExternalSpec(ctx context.Context, item *model.ExternalSpec) error {
	return r.db.WithContext(ctx).Where("provider_type = ? AND external_id = ?", item.ProviderType, item.ExternalID).
		Assign(map[string]any{
			"provider_id":   item.ProviderID,
			"external_name": item.ExternalName,
			"external_kind": item.ExternalKind,
			"raw":           item.Raw,
			"normalized":    item.Normalized,
			"fingerprint":   item.Fingerprint,
			"status":        item.Status,
			"synced_at":     item.SyncedAt,
		}).
		FirstOrCreate(item).Error
}

func (r *specContractRepository) ListBindings(ctx context.Context, externalSpecID, productSpecID uint64, status string) ([]model.SpecBinding, error) {
	base := r.db.WithContext(ctx).Model(&model.SpecBinding{})
	if externalSpecID > 0 {
		base = base.Where("external_spec_id = ?", externalSpecID)
	}
	if productSpecID > 0 {
		base = base.Where("product_spec_id = ?", productSpecID)
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var items []model.SpecBinding
	if err := base.Order("id desc").Limit(500).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ProductSpecCodes 批量取 SKU 编码：product_specs.id → spec_code（绑定列表回填展示用）。
// 用原生表查询，避免 spec 仓储依赖 catalog 模型。
func (r *specContractRepository) ProductSpecCodes(ctx context.Context, productSpecIDs []uint64) (map[uint64]string, error) {
	if len(productSpecIDs) == 0 {
		return nil, nil
	}
	var rows []struct {
		ID       uint64 `gorm:"column:id"`
		SpecCode string `gorm:"column:spec_code"`
	}
	if err := r.db.WithContext(ctx).Table("product_specs").
		Select("id, spec_code").Where("id IN ?", productSpecIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]string, len(rows))
	for _, row := range rows {
		out[row.ID] = row.SpecCode
	}
	return out, nil
}

func (r *specContractRepository) ListBindingsByProductSpecIDs(ctx context.Context, productSpecIDs []uint64) ([]model.SpecBinding, error) {
	if len(productSpecIDs) == 0 {
		return nil, nil
	}
	var items []model.SpecBinding
	if err := r.db.WithContext(ctx).
		Where("product_spec_id IN ?", productSpecIDs).
		Order("id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// BindingStatusByProductSpecs 批量取 SKU 出站绑定状态；多条时按 priority 高 → id 小优先。
func (r *specContractRepository) BindingStatusByProductSpecs(ctx context.Context, productSpecIDs []uint64) (map[uint64]string, error) {
	rows, err := r.ListBindingsByProductSpecIDs(ctx, productSpecIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]string, len(rows))
	best := make(map[uint64]int, len(rows))
	for _, row := range rows {
		if row.ProductSpecID == nil || row.Direction != model.BindingDirectionOutbound {
			continue
		}
		id := *row.ProductSpecID
		if prev, ok := best[id]; ok && prev >= row.Priority {
			continue
		}
		best[id] = row.Priority
		out[id] = row.Status
	}
	return out, nil
}

func (r *specContractRepository) FindConfirmedBindingByProductSpec(ctx context.Context, productSpecID uint64) (*model.SpecBinding, error) {
	var item model.SpecBinding
	err := r.db.WithContext(ctx).
		Where("product_spec_id = ? AND direction = ? AND status = ?",
			productSpecID, model.BindingDirectionOutbound, model.BindingStatusConfirmed).
		Order("priority desc, id asc").First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *specContractRepository) FindBindingByID(ctx context.Context, id uint64) (*model.SpecBinding, error) {
	var item model.SpecBinding
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *specContractRepository) ConfirmedPlatformParamsByProductSpec(ctx context.Context, productSpecID uint64) (string, error) {
	binding, err := r.FindConfirmedBindingByProductSpec(ctx, productSpecID)
	if err != nil || binding == nil {
		return "", err
	}
	return binding.PlatformParams, nil
}

// HasConfirmedBindingForExternal 判断某外部规格是否存在已确认绑定（代理商品上架门禁，T4.3）。
func (r *specContractRepository) HasConfirmedBindingForExternal(ctx context.Context, providerType, externalID string) (bool, error) {
	if strings.TrimSpace(providerType) == "" || strings.TrimSpace(externalID) == "" {
		return false, nil
	}
	var spec model.ExternalSpec
	if err := r.db.WithContext(ctx).
		Where("provider_type = ? AND external_id = ?", providerType, externalID).
		First(&spec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.SpecBinding{}).
		Where("external_spec_id = ? AND status = ?", spec.ID, model.BindingStatusConfirmed).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpsertBinding 幂等写入绑定：优先按 SKU（自营），否则按外部规格（代理）定位唯一行。
func (r *specContractRepository) UpsertBinding(ctx context.Context, item *model.SpecBinding) error {
	query := r.db.WithContext(ctx).Model(&model.SpecBinding{})
	if item.ProductSpecID != nil && *item.ProductSpecID > 0 {
		query = query.Where("product_spec_id = ? AND direction = ?", *item.ProductSpecID, item.Direction)
	} else {
		query = query.Where("external_spec_id = ? AND direction = ?", item.ExternalSpecID, item.Direction)
	}
	return query.
		Assign(map[string]any{
			"external_spec_id": item.ExternalSpecID,
			"product_spec_id":  item.ProductSpecID,
			"spec_template_id": item.SpecTemplateID,
			"platform_params":  item.PlatformParams,
			"match_type":       item.MatchType,
			"status":           item.Status,
			"confidence":       item.Confidence,
			"confirmed_by":     item.ConfirmedBy,
			"confirmed_at":     item.ConfirmedAt,
			"remark":           item.Remark,
			"priority":         item.Priority,
		}).
		FirstOrCreate(item).Error
}

// LoadAtomDictionary 把 spec_atoms 载入 specatom 全局字典（启动时调用，T2.6）。
func LoadAtomDictionary(ctx context.Context, repo SpecContractRepository) error {
	rows, err := repo.ListAtoms(ctx)
	if err != nil {
		return err
	}
	atoms := make([]specatom.Atom, 0, len(rows))
	for _, row := range rows {
		atom := specatom.Atom{
			Key:         row.Key,
			Name:        row.Name,
			Unit:        row.Unit,
			ValueType:   row.ValueType,
			MinValue:    row.MinValue,
			MaxValue:    row.MaxValue,
			AppliesTo:   row.AppliesTo,
			Status:      row.Status,
			Description: row.Description,
			Required:    row.Required,
		}
		if row.EnumValues != "" {
			atom.EnumValues = []byte(row.EnumValues)
		}
		// 平台字段字典（T4.1）：SKU 开通时据此把原子取值翻译为平台写参数。
		if row.PlatformFields != "" {
			fields := map[string]specatom.AtomPlatformField{}
			if err := json.Unmarshal([]byte(row.PlatformFields), &fields); err == nil {
				atom.PlatformFields = fields
			}
		}
		atoms = append(atoms, atom)
	}
	specatom.Global().Replace(atoms)
	return nil
}
