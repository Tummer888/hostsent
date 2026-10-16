package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	productmodel "hostsent/backend/internal/modules/admin/resource/product/model"
	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	"hostsent/backend/internal/modules/admin/resource/provider/model"
)

// ProviderRepository 上游提供商仓储接口
type ProviderRepository interface {
	List(ctx context.Context, query dto.ProviderListQuery) ([]model.ResourceProvider, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.ResourceProvider, error)
	Create(ctx context.Context, item *model.ResourceProvider) error
	Update(ctx context.Context, item *model.ResourceProvider) error
	Delete(ctx context.Context, id uint64) error
	// CountAssociations 统计提供商关联的商品、资源池与**实例**数量（删除前检查用）。
	//
	// instances 一并统计：渠道删掉后，绑着它的实例再也无法同步/操作
	//（渠道查询直接报"不存在"），而渠道没有恢复入口（只有删除）——
	// 实例会永久卡在列表里（用户实测反馈）。实例数与商品数同理必须挡住。
	//
	// placed 是其中**已开通**（provider_instance_id 非空）的台数，用于给出
	// 可操作的提示：已开通的要"先销毁实例"，未开通的是待开通占位行，
	// 该去处置来源订单（继续开通或取消），两者的处置入口不同。
	CountAssociations(ctx context.Context, id uint64) (products int64, pools int64, instances int64, placed int64, err error)
	// ListAssociationProducts 列出该渠道下的上游商品（删除确认弹窗用：要能看见删的是什么）。
	ListAssociationProducts(ctx context.Context, id uint64) ([]AssociationItem, error)
	// ListAssociationPools 列出该渠道下的资源池（同上）。
	ListAssociationPools(ctx context.Context, id uint64) ([]AssociationItem, error)
	// ListAssociationInstances 列出该渠道下**未销毁**的实例（删除确认弹窗用）。
	//
	// 这些实例是"删渠道前必须先处置"的主体：渠道删除后它们无法同步/操作，
	// 而渠道没有恢复入口（用户实测：删渠道导致实例永久卡死）。
	ListAssociationInstances(ctx context.Context, id uint64) ([]AssociationItem, error)
	// DeleteCascade 连带删除该渠道的商品与资源池（事务内），供"确认后一并清理"。
	DeleteCascade(ctx context.Context, id uint64) (products int64, pools int64, err error)
	// CountSalesRefsByResourceProduct 统计某上游商品被多少销售商品引用
	// （products.source_product_id 指向它）。>0 表示下游有真实售出商品依赖，
	// 连带删除会让那些商品的履约断链，必须拦住。
	CountSalesRefsByResourceProduct(ctx context.Context, resourceProductID uint64) (int64, error)
	// UpdateStats 回写提供商资源统计
	UpdateStats(ctx context.Context, providerID uint64, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int) error
	// MarkSynced 更新提供商最近同步时间
	MarkSynced(ctx context.Context, providerID uint64) error
	// RecordSyncSuccess 记录一次同步成功：刷新 last_sync_at/last_success_at，清零失败计数与错误
	RecordSyncSuccess(ctx context.Context, providerID uint64) error
	// RecordSyncFailure 记录一次同步失败：累加连续失败次数，达到 maxFailures 自动熔断暂停
	RecordSyncFailure(ctx context.Context, providerID uint64, errMsg string, maxFailures int) error
	// PauseSync 永久性错误（适配器未注册/凭证缺失等）立即熔断并写明原因
	PauseSync(ctx context.Context, providerID uint64, reason string) error
	// ResumeSync 手动恢复：解除熔断并清零失败计数
	ResumeSync(ctx context.Context, providerID uint64) error
	// SetSyncEnabled 只改同步总开关（渠道 × 类型调度的上层闸门）。
	SetSyncEnabled(ctx context.Context, providerID uint64, enabled bool) error
	// ListAll 返回全部渠道（含禁用），供启动期凭证归一/注册表同步等批处理使用。
	ListAll(ctx context.Context) ([]model.ResourceProvider, error)
	// UpdateCredentials 仅更新 credentials 列，避免全量 Save 覆盖其他并发修改。
	UpdateCredentials(ctx context.Context, providerID uint64, credentialsJSON string) error
}

type providerRepository struct {
	db *gorm.DB
}

// NewProviderRepository 创建上游提供商仓储实现
func NewProviderRepository(db *gorm.DB) ProviderRepository {
	return &providerRepository{db: db}
}

func (r *providerRepository) List(ctx context.Context, query dto.ProviderListQuery) ([]model.ResourceProvider, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.db.WithContext(ctx).Model(&model.ResourceProvider{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("name ILIKE ? OR provider_type ILIKE ? OR api_endpoint ILIKE ?", like, like, like)
	}
	if pt := strings.TrimSpace(query.ProviderType); pt != "" {
		base = base.Where("provider_type = ?", pt)
	}
	if k := strings.TrimSpace(query.Kind); k != "" {
		base = base.Where("kind = ?", k)
	}
	if query.Status != 0 {
		base = base.Where("status = ?", query.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ResourceProvider
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *providerRepository) FindByID(ctx context.Context, id uint64) (*model.ResourceProvider, error) {
	var item model.ResourceProvider
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *providerRepository) Create(ctx context.Context, item *model.ResourceProvider) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *providerRepository) Update(ctx context.Context, item *model.ResourceProvider) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *providerRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ResourceProvider{}, id).Error
}

func (r *providerRepository) CountAssociations(ctx context.Context, id uint64) (int64, int64, int64, int64, error) {
	var products, pools, instances, placed int64
	if err := r.db.WithContext(ctx).Model(&productmodel.ResourceProduct{}).
		Where("provider_id = ?", id).Count(&products).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	if err := r.db.WithContext(ctx).Model(&model.ResourcePool{}).
		Where("provider_id = ?", id).Count(&pools).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	// 只数**未销毁**的实例：已销毁的实例留着历史记录，不该永远挡住渠道清理。
	if err := r.db.WithContext(ctx).Table("instances").
		Where("provider_id = ? AND status <> ?", id, "deleted").
		Count(&instances).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	// 其中已开通的（有渠道侧实例号）：待开通占位行处置方式不同，故分开数。
	if err := r.db.WithContext(ctx).Table("instances").
		Where("provider_id = ? AND status <> ? AND COALESCE(provider_instance_id, '') <> ''", id, "deleted").
		Count(&placed).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	return products, pools, instances, placed, nil
}

// AssociationItem 关联数据的一行（删除确认弹窗展示用）。
//
// 直接复用 dto.ProviderAssociationItem 而不是再定义一份同形结构：
// 两份同形类型会在服务层拼装时被迫做无意义的逐字段搬运。
type AssociationItem = dto.ProviderAssociationItem

// ListAssociationProducts 列出该渠道下的上游商品。
func (r *providerRepository) ListAssociationProducts(ctx context.Context, id uint64) ([]AssociationItem, error) {
	var rows []productmodel.ResourceProduct
	if err := r.db.WithContext(ctx).
		Where("provider_id = ?", id).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AssociationItem, 0, len(rows))
	for _, p := range rows {
		// 副标题优先给「规格」——运营核对时关心的是这台机器什么配置，
		// 只给规格文本又常常为空（上游没回 specs），因此退到 CPU/内存/磁盘，
		// 这两个来源至少有一个有值，弹窗里不会出现一行只有名字的空白项。
		sub := strings.TrimSpace(p.Specs)
		if sub == "" {
			if p.CPU > 0 || p.Memory > 0 || p.Disk > 0 {
				sub = fmt.Sprintf("%d核 / %dG 内存 / %dG 磁盘", p.CPU, p.Memory, p.Disk)
			} else {
				sub = p.Region
			}
		}
		if len(sub) > 60 {
			sub = sub[:60] + "…"
		}
		out = append(out, AssociationItem{
			ID: p.ID, Name: p.Name, Kind: "product", Subtitle: sub,
		})
	}
	return out, nil
}

// ListAssociationPools 列出该渠道下的资源池。
func (r *providerRepository) ListAssociationPools(ctx context.Context, id uint64) ([]AssociationItem, error) {
	var rows []model.ResourcePool
	if err := r.db.WithContext(ctx).
		Where("provider_id = ?", id).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AssociationItem, 0, len(rows))
	for _, p := range rows {
		// 副标题给中文类型名 + 地域：类型码（node/region）直接丢到界面会与
		// 前端的中文标注打架，弹窗里要能一眼读出"这是台宿主机，在哪个区域"。
		sub := poolTypeLabel(p.PoolType)
		if p.Region != "" {
			if sub != "" {
				sub += " · "
			}
			sub += p.Region
		}
		out = append(out, AssociationItem{
			ID: p.ID, Name: p.Name, Kind: "pool", Subtitle: sub,
		})
	}
	return out, nil
}

// poolTypeLabel 池类型码 → 中文名（与前端 poolTypeLabel 同口径）。
// 后端也留一份：列表副标题由后端拼装，回给前端的就是成品文案，避免两边各拼一半。
func poolTypeLabel(poolType string) string {
	switch strings.ToLower(strings.TrimSpace(poolType)) {
	case "node", "host":
		return "宿主机"
	case "region":
		return "区域"
	case "zone":
		return "可用区"
	case "storage":
		return "存储"
	case "compute":
		return "计算池"
	case "memory":
		return "内存池"
	case "cluster":
		return "集群"
	default:
		return poolType
	}
}

// ListAssociationInstances 列出该渠道下未销毁的实例。
//
// 副标题给"状态 + 实例号"：运营要能定位是哪几台（去实例页处置）。
// 只列未销毁的：已销毁的实例留着历史记录，不该永远挡渠道清理
// （与 CountAssociations 的计数口径一致）。
func (r *providerRepository) ListAssociationInstances(ctx context.Context, id uint64) ([]AssociationItem, error) {
	type row struct {
		ID         uint64
		InstanceID string
		Name       string
		Status     string
		SourceMode string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("instances").
		Select("id, instance_id, COALESCE(name, '') AS name, COALESCE(status, '') AS status, COALESCE(source_mode, '') AS source_mode").
		Where("provider_id = ? AND status <> ?", id, "deleted").
		Order("id asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AssociationItem, 0, len(rows))
	for _, it := range rows {
		label := it.Name
		if label == "" {
			label = it.InstanceID
		}
		sub := it.InstanceID
		if it.SourceMode == "self" {
			sub += " · 自营"
		} else if it.SourceMode == "upstream" {
			sub += " · 转售"
		}
		if it.Status != "" {
			sub += " · " + it.Status
		}
		out = append(out, AssociationItem{ID: it.ID, Name: label, Kind: "instance", Subtitle: sub})
	}
	return out, nil
}

// DeleteCascade 连带删除该渠道的商品与资源池，再删渠道本身（单事务）。
//
// 顺序：先删子表（商品/资源池）再删主表，避免中途失败留下"渠道已删、子行还在"
// 的孤儿数据。任一步失败整体回滚。
func (r *providerRepository) DeleteCascade(ctx context.Context, id uint64) (int64, int64, error) {
	var products, pools int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("provider_id = ?", id).Delete(&productmodel.ResourceProduct{})
		if res.Error != nil {
			return res.Error
		}
		products = res.RowsAffected
		res = tx.Where("provider_id = ?", id).Delete(&model.ResourcePool{})
		if res.Error != nil {
			return res.Error
		}
		pools = res.RowsAffected
		return tx.Delete(&model.ResourceProvider{}, id).Error
	})
	if err != nil {
		return 0, 0, err
	}
	return products, pools, nil
}

// CountSalesRefsByResourceProduct 统计引用该上游商品的销售商品数。
//
// 用原生表名而不是 import 销售商品模型：资源域反向依赖商品域会形成循环
// （商品域已依赖资源域的商品模型），这里只读一个计数值，不值得为此拆包。
func (r *providerRepository) CountSalesRefsByResourceProduct(ctx context.Context, resourceProductID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("products").
		Where("source_product_id = ?", resourceProductID).Count(&n).Error
	return n, err
}

func (r *providerRepository) UpdateStats(ctx context.Context, providerID uint64, totalCPU, totalMemory, totalDisk, usedCPU, usedMemory, usedDisk int) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"total_cpu":    totalCPU,
			"total_memory": totalMemory,
			"total_disk":   totalDisk,
			"used_cpu":     usedCPU,
			"used_memory":  usedMemory,
			"used_disk":    usedDisk,
		}).Error
}

func (r *providerRepository) MarkSynced(ctx context.Context, providerID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Update("last_sync_at", &now).Error
}

// RecordSyncSuccess 成功用例：同步间隔以 last_sync_at 为准，失败也要 touch（见 RecordSyncFailure）。
func (r *providerRepository) RecordSyncSuccess(ctx context.Context, providerID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_sync_at":         &now,
			"last_success_at":      &now,
			"consecutive_failures": 0,
			"last_sync_error":      "",
		}).Error
}

// RecordSyncFailure 失败用例：失败同样刷新 last_sync_at，使 sync_interval 对失败渠道也生效，
// 避免坏渠道每 60 秒重试；连续失败达到 maxFailures 时置 sync_paused（由调度器跳过）。
func (r *providerRepository) RecordSyncFailure(ctx context.Context, providerID uint64, errMsg string, maxFailures int) error {
	now := time.Now()
	if maxFailures <= 0 {
		maxFailures = 5
	}
	// 单条 UPDATE 内自增并用旧值判定，避免读改写竞态。
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"last_sync_at":         &now,
			"last_sync_error":      errMsg,
			"consecutive_failures": gorm.Expr("consecutive_failures + 1"),
			"sync_paused":          gorm.Expr("(consecutive_failures + 1) >= ?", maxFailures),
		}).Error
}

func (r *providerRepository) PauseSync(ctx context.Context, providerID uint64, reason string) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"sync_paused":     true,
			"last_sync_error": reason,
		}).Error
}

func (r *providerRepository) ResumeSync(ctx context.Context, providerID uint64) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{
			"sync_paused":          false,
			"consecutive_failures": 0,
			"last_sync_error":      "",
		}).Error
}

// SetSyncEnabled 只改同步总开关一个字段。
//
// 为什么不用通用 Update：那是"整行替换"语义（Name/APIEndpoint/Status 等无条件赋值），
// 传半个 body 就会把没带的字段清空。同步总开关需要能单独切换（渠道同步与调度面板里
// 一键开关），必须有一条只带一个字段也安全的窄路径，故单独开一条。
func (r *providerRepository) SetSyncEnabled(ctx context.Context, providerID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Updates(map[string]interface{}{"sync_enabled": enabled}).Error
}

func (r *providerRepository) ListAll(ctx context.Context) ([]model.ResourceProvider, error) {
	var items []model.ResourceProvider
	if err := r.db.WithContext(ctx).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *providerRepository) UpdateCredentials(ctx context.Context, providerID uint64, credentialsJSON string) error {
	return r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Where("id = ?", providerID).
		Update("credentials", credentialsJSON).Error
}

// PoolRepository 资源池仓储接口
type PoolRepository interface {
	List(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.ResourcePool, error)
	UpsertPools(ctx context.Context, providerID uint64, items []model.ResourcePool) error
	// UpdatePoolProbes 批量回写池探针结论（probe_status/at/message）。
	// 单独成方法：UpsertPools 刻意不碰 probe_*，避免周期同步清掉实时通道的结论。
	UpdatePoolProbes(ctx context.Context, providerID uint64, probes []PoolProbe) error
	// ListAll 按当前筛选返回全量池（不分页），供容量汇总与地域枚举使用。
	ListAll(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, error)
	// ProviderMeta 返回 provider_id → 渠道元信息（名称 + 运维平台地址），供列表展示。
	ProviderMeta(ctx context.Context) (map[uint64]ProviderMeta, error)
	// Delete 删除单个资源池。
	//
	// 注意语义：池数据由平台同步 upsert（ON CONFLICT provider_id+upstream_id）。
	// 对**仍在线**的渠道，删掉的池会在下次同步时重新出现——删除的适用场景是
	// 演示/历史脏数据清理。前端据此提示，避免运营以为删了就能永久屏蔽某节点。
	Delete(ctx context.Context, id uint64) error
	// DeleteByProvider 删除某渠道的全部资源池（渠道连带删除用，事务外单独调用）。
	DeleteByProvider(ctx context.Context, providerID uint64) (int64, error)
}

// ProviderMeta 渠道元信息（池列表附带展示用，避免前端再查一次渠道列表）。
type ProviderMeta struct {
	Name          string
	OpsConsoleURL string
}

// NewPoolRepository 创建资源池仓储实现
func NewPoolRepository(db *gorm.DB) PoolRepository {
	return &poolRepository{db: db}
}

type poolRepository struct {
	db *gorm.DB
}

func (r *poolRepository) List(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, int64, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	base := r.applyPoolFilters(r.db.WithContext(ctx).Model(&model.ResourcePool{}), query)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ResourcePool
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListAll 返回当前筛选下的全量池（上限 5000，防止看板统计被极端数据拖垮）。
func (r *poolRepository) ListAll(ctx context.Context, query dto.PoolListQuery) ([]model.ResourcePool, error) {
	base := r.applyPoolFilters(r.db.WithContext(ctx).Model(&model.ResourcePool{}), query)
	var items []model.ResourcePool
	if err := base.Order("id desc").Limit(5000).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ProviderMeta provider_id → 渠道元信息（名称 + 运维平台地址）。
func (r *poolRepository) ProviderMeta(ctx context.Context) (map[uint64]ProviderMeta, error) {
	var rows []struct {
		ID            uint64
		Name          string
		OpsConsoleURL string
	}
	if err := r.db.WithContext(ctx).Model(&model.ResourceProvider{}).
		Select("id, name, ops_console_url").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]ProviderMeta, len(rows))
	for _, row := range rows {
		out[row.ID] = ProviderMeta{Name: row.Name, OpsConsoleURL: row.OpsConsoleURL}
	}
	return out, nil
}

// Delete 删除单个资源池。
func (r *poolRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ResourcePool{}, id).Error
}

// DeleteByProvider 删除某渠道的全部资源池。
func (r *poolRepository) DeleteByProvider(ctx context.Context, providerID uint64) (int64, error) {
	res := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Delete(&model.ResourcePool{})
	return res.RowsAffected, res.Error
}

// applyPoolFilters 复用池列表的筛选条件（关键词/地域/状态/告警/类别），
// 保证分页列表与容量汇总统计口径一致。
func (r *poolRepository) applyPoolFilters(base *gorm.DB, query dto.PoolListQuery) *gorm.DB {
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("name ILIKE ? OR upstream_id ILIKE ?", like, like)
	}
	if query.Region != "" {
		base = base.Where("region = ?", query.Region)
	}
	// 状态：仅当显式传值时过滤（0 是有意义的「已停用」，不能用零值判断）。
	if query.Status != nil {
		base = base.Where("status = ?", *query.Status)
	}
	// 类别：容量型与位置型是两类实体（宿主机 vs 区域），展示口径必须能分开。
	// 判定口径与 model.PoolCategoryOf 严格一致（排除法：区域/可用区为位置型，
	// 其余为容量型），否则列表与汇总会互相打架。
	switch strings.ToLower(strings.TrimSpace(query.Category)) {
	case model.PoolCategoryCapacity:
		base = base.Where("LOWER(COALESCE(pool_type, '')) NOT IN ?", locationPoolTypes)
	case model.PoolCategoryLocation:
		base = base.Where("LOWER(COALESCE(pool_type, '')) IN ?", locationPoolTypes)
	}
	// 告警：按 CPU/内存/磁盘任一项用量比例过滤，与前端 usagePercent 口径一致
	// （NULLIF 防 0 配额除零）。
	switch query.Alert {
	case "warn":
		base = base.Where(poolUsageRatioSQL + " >= 0.6")
	case "danger":
		base = base.Where(poolUsageRatioSQL + " >= 0.8")
	}
	return base
}

// locationPoolTypes 位置型池的类型（与 model.PoolCategoryOf 的口径保持同步）。
var locationPoolTypes = []string{"region", "zone"}

// poolUsageRatioSQL 资源池最大用量比例（CPU/内存/磁盘取最大，0 配额按 0 计）。
const poolUsageRatioSQL = `GREATEST(
	COALESCE(used_cpu::numeric / NULLIF(total_cpu, 0), 0),
	COALESCE(used_memory::numeric / NULLIF(total_memory, 0), 0),
	COALESCE(used_disk::numeric / NULLIF(total_disk, 0), 0)
)`

func (r *poolRepository) FindByID(ctx context.Context, id uint64) (*model.ResourcePool, error) {
	var item model.ResourcePool
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *poolRepository) UpsertPools(ctx context.Context, providerID uint64, items []model.ResourcePool) error {
	if len(items) == 0 {
		return nil
	}
	// 注意：probe_* 不在更新列中——探针结论只由 UpdatePoolProbes 单独回写，
	// 这样没有实时通道的池（区域）不会被周期同步把既有探针结论清零。
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_id"}, {Name: "upstream_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "pool_type", "total_cpu", "total_memory", "total_disk", "used_cpu", "used_memory", "used_disk", "status", "region", "zone", "parent_upstream_id", "last_sync_at", "updated_at"}),
	}).Create(&items).Error
}

// PoolProbe 一条池探针结论（批量回写用）。
type PoolProbe struct {
	UpstreamID string
	Status     string
	Message    string
	At         time.Time
}

// UpdatePoolProbes 批量回写池探针结论。用一条 UPDATE ... FROM (VALUES ...) 完成，
// 避免按池逐条更新（面板节点可达数百）。
//
// VALUES 显式加类型转换：裸占位符在 VALUES 里会被 PG 统一推断成 text，
// 与 probe_at 的 timestamptz 冲突（"column probe_at is of type timestamp with
// time zone but expression is of type text"）。
func (r *poolRepository) UpdatePoolProbes(ctx context.Context, providerID uint64, probes []PoolProbe) error {
	if len(probes) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(probes))
	args := make([]interface{}, 0, len(probes)*4+1)
	for _, p := range probes {
		at := p.At
		if at.IsZero() {
			at = time.Now()
		}
		placeholders = append(placeholders, "(?::varchar, ?::varchar, ?::varchar, ?::timestamptz)")
		args = append(args, p.UpstreamID, p.Status, p.Message, at)
	}
	args = append(args, providerID)
	sql := `UPDATE resource_pools AS rp
	SET probe_status = v.status, probe_message = v.message, probe_at = v.at, updated_at = now()
	FROM (VALUES ` + strings.Join(placeholders, ",") + `) AS v(upstream_id, status, message, at)
	WHERE rp.provider_id = ? AND rp.upstream_id = v.upstream_id`
	return r.db.WithContext(ctx).Exec(sql, args...).Error
}
