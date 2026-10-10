package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	catalogmodel "hostsent/backend/internal/modules/admin/product/catalog/model"
	productmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
)

// InstanceWithUser 实例行 + 关联展示字段。
type InstanceWithUser struct {
	syncmodel.Instance
	Username   string `gorm:"column:username" json:"username"`
	AutoRenew  bool   `gorm:"column:auto_renew" json:"auto_renew"`
	AutoPeriod int    `gorm:"column:auto_period" json:"auto_period"`
}

// StageWindow 生命周期阶段对应的 expire_at 时间窗口（半开区间：(ExpireAfter, ExpireBefore]）。
type StageWindow struct {
	ExpireAfter  *time.Time // expire_at > ExpireAfter
	ExpireBefore *time.Time // expire_at <= ExpireBefore
}

// InstanceReader instances 表只读访问（资源同步模块独占业务写入，本模块仅续费成功后延长 expire_at）。
type InstanceReader interface {
	ListByStage(ctx context.Context, window *StageWindow, keyword string, page, pageSize int) ([]InstanceWithUser, int64, error)
	GetByID(ctx context.Context, id uint64) (*syncmodel.Instance, error)
	ListByUser(ctx context.Context, userID uint64) ([]InstanceWithUser, error)
	ListDueForAutoRenew(ctx context.Context, expireBefore time.Time, instanceIDs []uint64) ([]syncmodel.Instance, error)
	ExtendExpireAt(ctx context.Context, id uint64, expireAt time.Time) error
	ResolveProduct(ctx context.Context, inst *syncmodel.Instance) (name string, unitPrice float64, err error)
	// ListStageCandidates 生命周期推进器候选：expire_at 落在窗口内且 lifecycle_stage 不等于目标阶段（T5.4）。
	// lifecycle_stage 为 NULL 的存量行也会被选出（由推进器"只落阶段、不触发上游动作"做种子化）。
	// 退避闸门：enforce_next_at 非空且未到期的实例不返回（失败退避，见 doc61 §8.4）。
	ListStageCandidates(ctx context.Context, window *StageWindow, stage string, limit int) ([]syncmodel.Instance, error)
	// ListActiveResetCandidates 续费后需回退到 active 的实例：expire_at > now 且 stage 非空且不等于 active。
	ListActiveResetCandidates(ctx context.Context, now time.Time, limit int) ([]syncmodel.Instance, error)
	// ListStageCandidatesWithUser / ListActiveResetCandidatesWithUser 为到期处置预演报告用的同口径查询，
	// 附带用户名，且**不受退避闸门限制**（预演要看见全部将被处理的实例）。
	ListStageCandidatesWithUser(ctx context.Context, window *StageWindow, stage string, limit int) ([]InstanceWithUser, error)
	ListActiveResetCandidatesWithUser(ctx context.Context, now time.Time, limit int) ([]InstanceWithUser, error)
	// UpdateLifecycleStage 落库生命周期阶段（T5.4 推进器的唯一写入点）。
	UpdateLifecycleStage(ctx context.Context, id uint64, stage string) error
	// MarkEnforceFailure 记录一次强制执行失败：自增退避计数、写失败原因、设置下次可重试时间。
	MarkEnforceFailure(ctx context.Context, id uint64, errMsg string, nextAt time.Time, attempts int) error
	// ClearEnforce 清零退避状态（动作成功或人为重置后调用）。
	ClearEnforce(ctx context.Context, id uint64) error
}

type instanceReader struct {
	db *gorm.DB
}

// NewInstanceReader 创建实例只读仓库。
func NewInstanceReader(db *gorm.DB) InstanceReader {
	return &instanceReader{db: db}
}

// instanceListQuery 基础查询：LEFT JOIN users（用户名）与 instance_auto_renewals（自动续费开关）。
func (r *instanceReader) instanceListQuery() *gorm.DB {
	return r.db.Table("instances AS i").
		Select("i.*, COALESCE(u.username, '') AS username, COALESCE(ar.enabled, FALSE) AS auto_renew, COALESCE(ar.period_count, 0) AS auto_period").
		Joins("LEFT JOIN users u ON u.id = i.user_id").
		Joins("LEFT JOIN instance_auto_renewals ar ON ar.instance_id = i.id")
}

func (r *instanceReader) ListByStage(ctx context.Context, window *StageWindow, keyword string, page, pageSize int) ([]InstanceWithUser, int64, error) {
	tx := r.instanceListQuery().WithContext(ctx)
	tx = tx.Where("i.expire_at IS NOT NULL")
	if window != nil {
		if window.ExpireAfter != nil {
			tx = tx.Where("i.expire_at > ?", *window.ExpireAfter)
		}
		if window.ExpireBefore != nil {
			tx = tx.Where("i.expire_at <= ?", *window.ExpireBefore)
		}
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		tx = tx.Where("i.instance_id LIKE ? OR i.name LIKE ? OR u.username LIKE ?", kw, kw, kw)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 10
	}
	var items []InstanceWithUser
	if err := tx.Order("i.expire_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *instanceReader) GetByID(ctx context.Context, id uint64) (*syncmodel.Instance, error) {
	var instance syncmodel.Instance
	if err := r.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

func (r *instanceReader) ListByUser(ctx context.Context, userID uint64) ([]InstanceWithUser, error) {
	var items []InstanceWithUser
	err := r.instanceListQuery().WithContext(ctx).
		Where("i.user_id = ? AND i.expire_at IS NOT NULL", userID).
		Order("i.expire_at ASC").
		Scan(&items).Error
	return items, err
}

// ListDueForAutoRenew 查询开启自动续费且已到期的实例（expire_at <= expireBefore）。
func (r *instanceReader) ListDueForAutoRenew(ctx context.Context, expireBefore time.Time, instanceIDs []uint64) ([]syncmodel.Instance, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	var items []syncmodel.Instance
	err := r.db.WithContext(ctx).
		Where("id IN ? AND expire_at IS NOT NULL AND expire_at <= ?", instanceIDs, expireBefore).
		Find(&items).Error
	return items, err
}

// ExtendExpireAt 续费成功后延长实例到期时间（本模块对 instances 的唯一写入点）。
func (r *instanceReader) ExtendExpireAt(ctx context.Context, id uint64, expireAt time.Time) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("expire_at", expireAt).Error
}

// UpdateLifecycleStage 落库生命周期阶段（T5.4 推进器的唯一写入点）。
func (r *instanceReader) UpdateLifecycleStage(ctx context.Context, id uint64, stage string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("lifecycle_stage", stage).Error
}

// MarkEnforceFailure 记录一次强制执行失败（doc61 §8.4 退避）：
// 覆盖式写入失败原因与下次重试时间，并落库累计次数（由调用方按指数退避算好传入）。
func (r *instanceReader) MarkEnforceFailure(ctx context.Context, id uint64, errMsg string, nextAt time.Time, attempts int) error {
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"enforce_attempts":   attempts,
			"enforce_next_at":    nextAt,
			"last_enforce_error": errMsg,
		}).Error
}

// ClearEnforce 清零退避状态（动作成功或运维手动重置后调用）。
func (r *instanceReader) ClearEnforce(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"enforce_attempts":   0,
			"enforce_next_at":    nil,
			"last_enforce_error": "",
		}).Error
}

// ListStageCandidates 列出目标阶段的候选实例（T5.4）：
// expire_at 落在窗口内，且 lifecycle_stage 尚未等于目标阶段（含 NULL 存量行）。
// 按 expire_at 升序（越早到期越先处理），limit 控制单轮处理量避免长事务。
// 退避闸门：enforce_next_at 非空且大于当前时间（连续失败后退避中）的实例跳过，
// 避免对同一失败实例每 10 分钟无脑重试（doc61 §8.4）。
func (r *instanceReader) ListStageCandidates(ctx context.Context, window *StageWindow, stage string, limit int) ([]syncmodel.Instance, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	tx := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("expire_at IS NOT NULL").
		Where("(lifecycle_stage IS NULL OR lifecycle_stage <> ?)", stage).
		Where("(enforce_next_at IS NULL OR enforce_next_at <= now())")
	if window != nil {
		if window.ExpireAfter != nil {
			tx = tx.Where("expire_at > ?", *window.ExpireAfter)
		}
		if window.ExpireBefore != nil {
			tx = tx.Where("expire_at <= ?", *window.ExpireBefore)
		}
	}
	var items []syncmodel.Instance
	if err := tx.Order("expire_at ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListActiveResetCandidates 列出续费后需回退 active 的实例：
// expire_at > now 但 lifecycle_stage 非空且不等于 active（曾被暂停/标记销毁后已续费）。
func (r *instanceReader) ListActiveResetCandidates(ctx context.Context, now time.Time, limit int) ([]syncmodel.Instance, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var items []syncmodel.Instance
	err := r.db.WithContext(ctx).
		Where("expire_at IS NOT NULL AND expire_at > ?", now).
		Where("lifecycle_stage IS NOT NULL AND lifecycle_stage <> ?", "active").
		Order("expire_at ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// ListStageCandidatesWithUser 到期处置预演用：与 ListStageCandidates 同口径但带用户名、
// 且不受退避闸门限制（预演需看到全部候选，否则运维会误以为没有待处理实例）。
func (r *instanceReader) ListStageCandidatesWithUser(ctx context.Context, window *StageWindow, stage string, limit int) ([]InstanceWithUser, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	tx := r.instanceListQuery().WithContext(ctx).
		Where("i.expire_at IS NOT NULL").
		Where("(i.lifecycle_stage IS NULL OR i.lifecycle_stage <> ?)", stage)
	if window != nil {
		if window.ExpireAfter != nil {
			tx = tx.Where("i.expire_at > ?", *window.ExpireAfter)
		}
		if window.ExpireBefore != nil {
			tx = tx.Where("i.expire_at <= ?", *window.ExpireBefore)
		}
	}
	var items []InstanceWithUser
	if err := tx.Order("i.expire_at ASC").Limit(limit).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListActiveResetCandidatesWithUser 预演用：与 ListActiveResetCandidates 同口径但带用户名。
func (r *instanceReader) ListActiveResetCandidatesWithUser(ctx context.Context, now time.Time, limit int) ([]InstanceWithUser, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var items []InstanceWithUser
	err := r.instanceListQuery().WithContext(ctx).
		Where("i.expire_at IS NOT NULL AND i.expire_at > ?", now).
		Where("i.lifecycle_stage IS NOT NULL AND i.lifecycle_stage <> ?", "active").
		Order("i.expire_at ASC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

// ResolveProduct 解析实例对应的产品名称与单周期单价。
// 双链路判据（P1/T1.2）：自营取售出商品 sell_product_id（products.id）；
// 上游取 upstream_product_id（resource_products.id）；两者皆空时回落到含义含糊的
// 旧 product_id 解析（保留只读一版，待存量为空后移除）。
func (r *instanceReader) ResolveProduct(ctx context.Context, inst *syncmodel.Instance) (string, float64, error) {
	if inst == nil {
		return "", 0, nil
	}
	if inst.SellProductID > 0 {
		var saleProd catalogmodel.Product
		err := r.db.WithContext(ctx).First(&saleProd, inst.SellProductID).Error
		if err == nil {
			return saleProd.Name, saleProd.Price, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, err
		}
	}
	if inst.UpstreamProductID > 0 {
		var resourceProd productmodel.ResourceProduct
		err := r.db.WithContext(ctx).First(&resourceProd, inst.UpstreamProductID).Error
		if err == nil {
			return resourceProd.Name, resourceProd.SalePrice, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, err
		}
	}
	// P8：product_id 兼容列已删除，旧数据兜底（resolveLegacyProduct）一并退役；
	// 双链路列均解析不到时返回空，由调用方自行处理。
	return "", 0, nil
}

// resolveLegacyProduct 旧 product_id 语义的兜底解析：先按 source_product_id 找售出商品，
// 再按主键找上游资源商品。仅为存量数据兼容，新数据不应走到这里。
func (r *instanceReader) resolveLegacyProduct(ctx context.Context, productID uint64) (string, float64, error) {
	if productID == 0 {
		return "", 0, nil
	}
	var saleProd catalogmodel.Product
	err := r.db.WithContext(ctx).
		Where("source_product_id = ?", productID).
		Order("id DESC").First(&saleProd).Error
	if err == nil {
		return saleProd.Name, saleProd.Price, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", 0, err
	}
	var resourceProd productmodel.ResourceProduct
	err = r.db.WithContext(ctx).First(&resourceProd, productID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, nil
		}
		return "", 0, err
	}
	return resourceProd.Name, resourceProd.SalePrice, nil
}
