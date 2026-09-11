package repository

import (
	"context"

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

	// ListBindings 查询绑定（可按外部规格或状态过滤）。
	ListBindings(ctx context.Context, externalSpecID uint64, status string) ([]model.SpecBinding, error)
	// UpsertBinding 按 (external_spec_id, direction) 幂等写入绑定。
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

func (r *specContractRepository) ListBindings(ctx context.Context, externalSpecID uint64, status string) ([]model.SpecBinding, error) {
	base := r.db.WithContext(ctx).Model(&model.SpecBinding{})
	if externalSpecID > 0 {
		base = base.Where("external_spec_id = ?", externalSpecID)
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

func (r *specContractRepository) UpsertBinding(ctx context.Context, item *model.SpecBinding) error {
	return r.db.WithContext(ctx).Where("external_spec_id = ? AND direction = ?", item.ExternalSpecID, item.Direction).
		Assign(map[string]any{
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
		}
		if row.EnumValues != "" {
			atom.EnumValues = []byte(row.EnumValues)
		}
		atoms = append(atoms, atom)
	}
	specatom.Global().Replace(atoms)
	return nil
}
