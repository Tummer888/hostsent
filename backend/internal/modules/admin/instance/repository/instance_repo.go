// Package repository 提供实例运维管理台的数据访问实现。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
)

// ErrNotFound 实例不存在。
var ErrNotFound = errors.New("实例不存在")

// InstanceRow 实例 + 归属用户 + 操作人 + 服务商 的联表行。
//
// 内嵌 syncmodel.Instance 复用既有实例主模型（不新建第三份映射）；
// 查询一律显式 .Table("instances AS instances")，避免内嵌结构体提升的 TableName 干扰。
type InstanceRow struct {
	syncmodel.Instance
	Username     string `gorm:"column:username"`
	UserEmail    string `gorm:"column:user_email"`
	UserPhone    string `gorm:"column:user_phone"`
	ActorName    string `gorm:"column:actor_name"`
	ProviderName string `gorm:"column:provider_name"`
	ProviderType string `gorm:"column:provider_type"`
}

// InstanceMark 上游/平台侧实例号（优先 provider_instance_id，回落 instance_id），
// 生命周期动作（暂停/恢复/销毁）一律以该值调用上游（T5.5）。
func (r *InstanceRow) InstanceMark() string {
	if r == nil {
		return ""
	}
	if r.ProviderInstanceID != "" {
		return r.ProviderInstanceID
	}
	return r.InstanceID
}

// InstanceRepository 实例运维仓储接口。
type InstanceRepository interface {
	List(ctx context.Context, query *dto.ListQuery) ([]InstanceRow, int64, error)
	FindByID(ctx context.Context, id uint64) (*InstanceRow, error)
	// FindByOrderID 按来源订单查实例（退款联动用；查不到返回 ErrNotFound，非云主机订单属正常）。
	FindByOrderID(ctx context.Context, orderID uint64) (*InstanceRow, error)
	// FindByOrderIDs 按来源订单批量查实例（订单列表「关联实例」列回填用）。
	// 同一订单可能有多行（换机/重开历史），取每单最新一行（id 最大）。
	FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*InstanceRow, error)
	Stats(ctx context.Context, withinDays int) (*dto.StatsResponse, error)
	// TypeCounts 按产品类型统计实例数（产品线导航计数；未分类行归 other）。
	TypeCounts(ctx context.Context) (map[string]int64, error)
	UpdateRemark(ctx context.Context, id uint64, remark string) error
	UpdateStatus(ctx context.Context, id uint64, status string) error
	ApplySync(ctx context.Context, id uint64, status, publicIP, privateIP, rawData string, syncedAt time.Time) error
	UpdateSpecs(ctx context.Context, id uint64, cpu, memory, disk int, diskType string) error
	// UpdateBandwidth 带宽直改后回写本地快照（Mbps）。
	UpdateBandwidth(ctx context.Context, id uint64, bandwidth int) error
	// UpdateDisk 挂载数据盘后回写本地磁盘容量（GB）。
	UpdateDisk(ctx context.Context, id uint64, disk int) error
	// UpdateLifecycleStage 落库生命周期阶段（T5.4/T5.5：手动暂停/恢复与推进器共用）。
	UpdateLifecycleStage(ctx context.Context, id uint64, stage string) error
	// MarkTransition 记录"某操作正在执行"（迁移 112）：上游指令是异步的，
	// 落库后刷新页面仍能看到「重启中」，并可据此拦截重复指令。
	MarkTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) error
	// ClaimTransition **原子抢占**过渡态：仅当当前没有生效中的过渡态时写入并返回 true。
	//
	// 为什么必须"抢占"而不是"先查后写"：先查后写之间存在窗口 —— 用户连点两下，
	// 两个并发请求会双双通过检查，上游收到两条同样指令（实测复现）。
	// 这里把判定与写入压进同一条 UPDATE 的 WHERE 里，由数据库保证只有一个赢。
	ClaimTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) (bool, error)
	// ClearTransitionIfOwner 仅当过渡态仍等于 state 时清除（下发失败的回滚用）。
	// 避免慢请求失败时误清掉后来者已抢占的过渡态。
	ClearTransitionIfOwner(ctx context.Context, id uint64, state string) error
	// ClearTransition 清除过渡态（回源确认已生效 / 操作失败 / 超时后人工确认）。
	ClearTransition(ctx context.Context, id uint64) error
	// UpdateCredentialPassword 覆盖登录密码密文（重置密码/重装后平台返回新密码时）。
	// 与开通写入的区别：这里允许覆盖（新密码就是权威值），但仍只收密文。
	UpdateCredentialPassword(ctx context.Context, id uint64, encrypted string) error
	// AdjustBilling 运维手工调整账期/付款周期（不产生账单，只改口径）。
	// expireAt 为 nil 且 clearExpire=true 时清空到期时间（一次性/买断商品）。
	AdjustBilling(ctx context.Context, id uint64, expireAt *time.Time, clearExpire bool, billingMode string) error
	// SetRenewalPrice 绑定/清除实例续费单价（instances.renewal_price）。
	// price 为 nil 或负值时清除绑定（回到按商品实时算价）。
	//
	// 与 AdjustBilling 分列写入而非合并：绑定价是"下次续费按这个价"的承诺，
	// 常在只改价不改周期时单独设置，混在一个 Updates 里会互相干扰。
	SetRenewalPrice(ctx context.Context, id uint64, price *float64) error
	// ListBoundRenewalByProduct 列出某售出商品下**已绑定**续费价的实例。
	//
	// 为什么只列已绑定的：绑定价是"钉死价"，不随商品调价变化；未绑定的实例续费时
	// 本就实时读商品周期价，改商品价即生效，无需也不该给它们写绑定值
	//（写了反而把"跟随商品价"变成"钉死在本次价格"）。
	ListBoundRenewalByProduct(ctx context.Context, productID uint64) ([]BoundRenewalInstance, error)
	// SetRenewalPriceByProduct 按商品批量改续费绑定；price 为 nil 表示清除绑定。
	// onlyBound=true 时只动已绑定的实例（默认语义），false 时连未绑定的一并写入。
	// 返回受影响行数。
	SetRenewalPriceByProduct(ctx context.Context, productID uint64, price *float64, onlyBound bool) (int64, error)
	// RelinkUpstream 把实例重新关联到另一个上游实例号（上游换机场景）。
	// migrateInstanceID 为 true 时同时改写本地展示号 instance_id。
	// 只改号与回源字段，不动账期/续费价/归属（那些是本地决策，与上游号无关）。
	RelinkUpstream(ctx context.Context, id uint64, newUpstreamID string, migrateInstanceID bool, inst *UpstreamSnapshot) error
	// CountByProviderInstanceID 统计除 excludeID 外还有哪些实例占用了该上游号。
	// 用于"一号一台"守卫：若已被别的实例占用，重新关联会把两台实例指向同一台机器。
	CountByProviderInstanceID(ctx context.Context, providerInstanceID string, excludeID uint64) (int64, error)
	// RebindProduct 更换实例绑定的本地商品与上游镜像（上游换产品场景）。
	//   - sellProductID > 0 时改写 sell_product_id；
	//   - upstreamProductID 非 nil 时改写 upstream_product_id（指向 0 = 清空）。
	RebindProduct(ctx context.Context, id uint64, sellProductID uint64, upstreamProductID *uint64) error
	// FreeInstanceIDForDestroyed 为重新关联腾出 instance_id：把**已销毁**行的 instance_id
	// 改成带前缀的归档形式，返回被腾出的行数。
	//
	// 为什么必须这么做：instances.instance_id 有唯一索引，而销毁行保留原号（历史可溯）。
	// 上游回收号段后把老号发给新机器是常见做法，此时目标号恰好等于某条销毁行用过的号，
	// 直接改写就会撞唯一约束（实测 23505）。销毁行不再承载任何上游动作，
	// 它的 instance_id 仅用于历史展示，加个标注前缀即可让位。
	FreeInstanceIDForDestroyed(ctx context.Context, instanceID string, keepID uint64) (int64, error)
}

// UpstreamSnapshot 回源拉到的上游实例关键字段（重新关联时回写本地快照）。
type UpstreamSnapshot struct {
	Status    string
	PublicIP  string
	PrivateIP string
	Name      string
	OS        string
	Region    string
	RawData   string
}

type instanceRepository struct {
	db *gorm.DB
}

// NewInstanceRepository 创建实例运维仓储。
func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &instanceRepository{db: db}
}

// instanceSelect 列表/详情统一投影列。
const instanceSelect = "instances.*, " +
	"u.username AS username, u.email AS user_email, u.phone AS user_phone, " +
	"a.username AS actor_name, " +
	"p.name AS provider_name, p.provider_type AS provider_type"

// baseQuery 构造带归属用户/操作人/服务商联表的过滤查询（列表与计数共用）。
func (r *instanceRepository) baseQuery(ctx context.Context, query *dto.ListQuery) *gorm.DB {
	db := r.db.WithContext(ctx).
		Table("instances AS instances").
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id")

	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("instances.instance_id ILIKE ? OR instances.name ILIKE ?", like, like)
	}
	if kw := strings.TrimSpace(query.UserKeyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("u.username ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		db = db.Where("instances.user_id = ?", query.UserID)
	}
	if query.ProviderID > 0 {
		db = db.Where("instances.provider_id = ?", query.ProviderID)
	}
	if s := strings.TrimSpace(query.Status); s != "" {
		db = db.Where("instances.status = ?", s)
	}
	if sm := strings.TrimSpace(query.SourceMode); sm != "" {
		db = db.Where("instances.source_mode = ?", sm)
	}
	// 产品类型筛选（产品线导航）：空值 = 全部，不做过滤。
	//
	// 存量行可能是空串（迁移 108 回填前，或回填时商品也缺类型），这些行按 other 处理 ——
	// 否则选「其它」看不到它们、选各具体类别也看不到，等于从界面上消失。
	// 所以 other 这一档要把「未分类」的行一并兜住。
	if it := strings.TrimSpace(query.InstanceType); it != "" {
		if it == pkgmodel.ProductKindOther {
			db = db.Where("COALESCE(NULLIF(TRIM(instances.instance_type), ''), ?) = ?",
				pkgmodel.ProductKindOther, pkgmodel.ProductKindOther)
		} else {
			db = db.Where("instances.instance_type = ?", it)
		}
	}

	days := query.ExpireWithinDays
	if days <= 0 {
		days = 7
	}
	now := time.Now()
	switch query.ExpireState {
	case dto.ExpireStateExpiring:
		db = db.Where("instances.expire_at IS NOT NULL AND instances.expire_at >= ? AND instances.expire_at <= ?", now, now.AddDate(0, 0, days))
	case dto.ExpireStateExpired:
		db = db.Where("instances.expire_at IS NOT NULL AND instances.expire_at < ?", now)
	case dto.ExpireStateNone:
		db = db.Where("instances.expire_at IS NULL")
	}
	return db
}

// List 跨用户实例分页列表。
func (r *instanceRepository) List(ctx context.Context, query *dto.ListQuery) ([]InstanceRow, int64, error) {
	page, pageSize := normalizePage(query.Page, query.PageSize)

	var total int64
	if err := r.baseQuery(ctx, query).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []InstanceRow
	err := r.baseQuery(ctx, query).
		Select(instanceSelect).
		Order("instances.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// FindByID 按主键查询实例（联表归属用户/服务商）。
func (r *instanceRepository) FindByID(ctx context.Context, id uint64) (*InstanceRow, error) {
	var row InstanceRow
	err := r.db.WithContext(ctx).
		Table("instances AS instances").
		Select(instanceSelect).
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id").
		Where("instances.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindByOrderID 按来源订单查询该订单开通的实例（退款联动用）。
//
// instances.order_id 由开通成功时写入（buildRecordedInstance）；存量行为 0，查不到即返回
// ErrNotFound，调用方据此跳过（退款不是每单都有实例，非云主机订单本就没有）。
func (r *instanceRepository) FindByOrderID(ctx context.Context, orderID uint64) (*InstanceRow, error) {
	if orderID == 0 {
		return nil, ErrNotFound
	}
	var row InstanceRow
	err := r.db.WithContext(ctx).
		Table("instances AS instances").
		Select(instanceSelect).
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id").
		Where("instances.order_id = ?", orderID).
		Order("instances.id DESC").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindByOrderIDs 按来源订单批量查实例（订单列表「关联实例」列回填用）。
//
// 只取 id 最大的那一行（同一订单换机/重开会有多行历史），返回 order_id → 行 的映射；
// 没有实例的订单不出现在返回表里，调用方按缺失处理。
func (r *instanceRepository) FindByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64]*InstanceRow, error) {
	if len(orderIDs) == 0 {
		return map[uint64]*InstanceRow{}, nil
	}
	var rows []InstanceRow
	if err := r.db.WithContext(ctx).
		Table("instances AS instances").
		Select(instanceSelect).
		Joins("LEFT JOIN users AS u ON u.id = instances.user_id").
		Joins("LEFT JOIN users AS a ON a.id = instances.actor_user_id").
		Joins("LEFT JOIN resource_providers AS p ON p.id = instances.provider_id").
		Where("instances.order_id IN ?", orderIDs).
		Order("instances.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]*InstanceRow, len(rows))
	for i := range rows {
		// id ASC 遍历：同订单后出现的即更新的行，直接覆盖，最终留下最新一行。
		out[rows[i].OrderID] = &rows[i]
	}
	return out, nil
}

// Stats 全局概览统计（不受列表筛选影响）。
func (r *instanceRepository) Stats(ctx context.Context, withinDays int) (*dto.StatsResponse, error) {
	if withinDays <= 0 {
		withinDays = 7
	}
	now := time.Now()
	var out dto.StatsResponse
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'running')  AS running,
			COUNT(*) FILTER (WHERE status = 'stopped')  AS stopped,
			COUNT(*) FILTER (WHERE status = 'creating') AS creating,
			COUNT(*) FILTER (WHERE status = 'error')    AS "error",
			COUNT(*) FILTER (WHERE expire_at IS NOT NULL AND expire_at >= ? AND expire_at <= ?) AS expiring,
			COUNT(*) FILTER (WHERE expire_at IS NOT NULL AND expire_at < ?) AS expired
		FROM instances`, now, now.AddDate(0, 0, withinDays), now).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// TypeCounts 按产品类型统计实例数（产品线导航的计数来源）。
//
// 返回 map[类型值]数量。未登记/为空的行归到 other —— 与 baseQuery 的筛选口径一致：
// 导航里 other 标签的数量必须等于点进去看到的行数，否则运营会以为筛选坏了。
//
// 只做一次 GROUP BY 而不是逐类型 COUNT：类型会随产品线扩展变多，
// N 个类型 N 次查询既慢又容易与列表口径漂移。
func (r *instanceRepository) TypeCounts(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Kind  string
		Count int64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(NULLIF(TRIM(instance_type), ''), ?) AS kind, COUNT(*) AS count
		FROM instances
		GROUP BY 1`, pkgmodel.ProductKindOther).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Kind] = row.Count
	}
	return counts, nil
}

// UpdateRemark 更新管理员内部备注。
func (r *instanceRepository) UpdateRemark(ctx context.Context, id uint64, remark string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("remark", remark).Error
}

// UpdateStatus 更新实例服务状态。
func (r *instanceRepository) UpdateStatus(ctx context.Context, id uint64, status string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ApplySync 回源刷新：回写状态、IP、原始数据与回源时间。
//
// os 一并回写（实测故障）：实例的 os 列在开通落库时取自订单快照（商品默认镜像），
// 与客户实际所选镜像可能不同（选 Debian 开通、上游 host_data.os=Debian-12.0_x64，
// 本地却显示 CentOS）。回源拿到的就是这台机器真实的镜像，必须以它为准修正 ——
// 否则「同步刷新」这个按钮永远修不好已经存错的历史实例。
// 空值不覆盖（与 IP 同一取舍）：上游没给 os 时保持本地现值。
// status 同样接受空串：调用方判定回源状态不可信（如电源操作后上游状态机延迟）
// 时传空，本次只刷新 IP / 原始数据，不动本地状态。
func (r *instanceRepository) ApplySync(ctx context.Context, id uint64, status, publicIP, privateIP, rawData string, syncedAt time.Time) error {
	updates := map[string]any{
		"last_synced_at": syncedAt,
		"updated_at":     syncedAt,
	}
	if status != "" {
		updates["status"] = status
	}
	// 上游未返回的字段不覆盖本地值，避免把已有 IP 抹成空串。
	if publicIP != "" {
		updates["public_ip"] = publicIP
	}
	if privateIP != "" {
		updates["private_ip"] = privateIP
	}
	if rawData != "" {
		updates["raw_data"] = rawData
	}
	// os 从 raw_data 里取（各适配器统一写 raw_data.os）：上游没给就跳过。
	if os := osFromRawData(rawData); os != "" {
		updates["os"] = os
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// osFromRawData 从原始数据 JSON 里读操作系统名（适配器统一落 raw_data.os）。
// 读不到返回空串（调用方据此跳过覆盖）。
func osFromRawData(rawData string) string {
	if strings.TrimSpace(rawData) == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawData), &m); err != nil {
		return ""
	}
	if v, ok := m["os"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// UpdateSpecs 变配后回写规格快照。
func (r *instanceRepository) UpdateSpecs(ctx context.Context, id uint64, cpu, memory, disk int, diskType string) error {
	updates := map[string]any{}
	if cpu > 0 {
		updates["cpu"] = cpu
	}
	if memory > 0 {
		updates["memory"] = memory
	}
	if disk > 0 {
		updates["disk"] = disk
	}
	if diskType != "" {
		updates["disk_type"] = diskType
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateBandwidth 带宽直改后回写本地快照（Mbps）。
func (r *instanceRepository) UpdateBandwidth(ctx context.Context, id uint64, bandwidth int) error {
	if bandwidth <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("bandwidth", bandwidth).Error
}

// UpdateDisk 挂载数据盘后回写本地磁盘容量（GB）。
// 上游是"在原系统盘基础上新挂一块"，所以调用方传入的是新总量。
func (r *instanceRepository) UpdateDisk(ctx context.Context, id uint64, disk int) error {
	if disk <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("disk", disk).Error
}

// UpdateLifecycleStage 落库生命周期阶段（T5.4/T5.5）。同步流程不覆盖该列，
// 仅生命周期推进器与手动暂停/恢复写入，保证阶段与上游动作一致。
func (r *instanceRepository) UpdateLifecycleStage(ctx context.Context, id uint64, stage string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("lifecycle_stage", stage).Error
}

// MarkTransition 记录进行中的操作（迁移 112）。
//
// 覆盖写而不是追加：一台机器同一时刻只应有一个进行中的动作
// （新指令在服务层就被过渡态拦截，走到这里说明是合法的新动作）。
func (r *instanceRepository) MarkTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"transition_state":      state,
			"transition_started_at": startedAt,
			"transition_until":      until,
		}).Error
}

// ClaimTransition 原子抢占过渡态：仅当没有"生效中"的过渡态时写入。
//
// WHERE 里的两个条件对应两类可抢占情形：
//   - transition_state IS NULL：本来空闲；
//   - transition_until < now：上一个动作已超过典型耗时，视为结果未知，
//     不该永久锁死实例（与 ensureNoActiveTransition 的超时放行同一口径）。
//
// 条件与赋值在同一条 UPDATE 里，由数据库的行锁保证互斥 —— 两个并发的
// "开机"请求只有一个能拿到 RowsAffected=1，另一个拿到 0 并被服务层拒绝。
// 这是"用户连点两次都下发成功"的直接修法（先查后写存在检查-执行窗口）。
func (r *instanceRepository) ClaimTransition(ctx context.Context, id uint64, state string, startedAt, until time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ? AND (transition_state IS NULL OR transition_state = '' OR transition_until IS NULL OR transition_until < ?)",
			id, startedAt).
		Updates(map[string]any{
			"transition_state":      state,
			"transition_started_at": startedAt,
			"transition_until":      until,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ClearTransitionIfOwner 仅当过渡态仍等于 state 时清除。
//
// 用于"抢到了但下发失败"的回滚：如果不加 owner 条件，一个慢请求的失败回滚
// 可能清掉后来者已成功抢占的过渡态，让界面在动作执行中错误地恢复成空闲。
func (r *instanceRepository) ClearTransitionIfOwner(ctx context.Context, id uint64, state string) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ? AND transition_state = ?", id, state).
		Updates(map[string]any{
			"transition_state":      nil,
			"transition_started_at": nil,
			"transition_until":      nil,
		}).Error
}

// ClearTransition 清除过渡态。
//
// 只清非空行（WHERE transition_state IS NOT NULL）：回源确认是高频动作，
// 绝大多数实例本来就没有过渡态，带上这个条件可以让无谓的行更新不写库
// （也避免 updated_at 被反复刷新，干扰"最近变更"的判断）。
func (r *instanceRepository) ClearTransition(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ? AND transition_state IS NOT NULL", id).
		Updates(map[string]any{
			"transition_state":      nil,
			"transition_started_at": nil,
			"transition_until":      nil,
		}).Error
}

// UpdateCredentialPassword 覆盖登录密码密文（重置密码后调用）。
//
// 重置成功后旧密码立即失效，留存必须同步更新——否则「查看密码」会一直显示
// 一个已经登不上去的历史密码。参数是密文（调用方加密），本层不接触明文。
func (r *instanceRepository) UpdateCredentialPassword(ctx context.Context, id uint64, encrypted string) error {
	if encrypted == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("credential_password", encrypted).Error
}

// AdjustBilling 运维手工调整账期/付款周期。
//
// 只改 instances 的两列，不写 lifecycle_stage：阶段由推进器按新的 expire_at 重新派生
// （若这里顺手改阶段，会把"已暂停但有未来到期时间"这种矛盾状态写进库）。
func (r *instanceRepository) AdjustBilling(ctx context.Context, id uint64, expireAt *time.Time, clearExpire bool, billingMode string) error {
	updates := map[string]any{}
	if clearExpire {
		updates["expire_at"] = nil
	} else if expireAt != nil {
		updates["expire_at"] = *expireAt
	}
	if billingMode != "" {
		updates["billing_mode"] = billingMode
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// SetRenewalPrice 绑定/清除续费单价（instances.renewal_price）。
//
// 非正值一律写 NULL（未绑定）：0 元续费会被续费守卫拒绝，
// 存一个算不出钱的绑定价只会让续费在别的环节报错，不如明确当作"没绑过"。
func (r *instanceRepository) SetRenewalPrice(ctx context.Context, id uint64, price *float64) error {
	var value any
	if price != nil && *price > 0 {
		value = money2(*price) // 同包（detail_repo.go）的金额取整工具
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Update("renewal_price", value).Error
}

// BoundRenewalInstance 已绑定续费价的实例（批量续费价同步的目标行）。
//
// 带上 BillingMode：绑定价是"每周期单价"，而实例的付款周期各不相同
// （月付/年付…），写回时必须按各自的周期取对应档位价，不能所有实例写同一个数。
type BoundRenewalInstance struct {
	ID           uint64
	BillingMode  string
	RenewalPrice *float64
}

// ListBoundRenewalByProduct 列出某售出商品下已绑定续费价的实例。
//
// 排除已销毁实例（口径同 CountByProviderInstanceID：status=deleted 或 lifecycle_stage=destroyed）：
// 销毁行不再续费，改它的续费价没有意义，还会让"将更新 N 台"的数字虚高。
func (r *instanceRepository) ListBoundRenewalByProduct(ctx context.Context, productID uint64) ([]BoundRenewalInstance, error) {
	var rows []BoundRenewalInstance
	err := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Select("id, billing_mode, renewal_price").
		Where("sell_product_id = ? AND renewal_price IS NOT NULL", productID).
		Where("status <> ?", string(pkgmodel.InstanceStatusDeleted)).
		Where("coalesce(lifecycle_stage, '') <> ?", "destroyed").
		Order("id asc").
		Scan(&rows).Error
	return rows, err
}

// SetRenewalPriceByProduct 按商品批量设置/清除续费绑定。
//
// price 为 nil：清除绑定（这些实例回到"续费时实时读商品周期价"）。
// onlyBound=true：只动已绑定的行 —— 避免把"跟随商品价"的实例意外钉死。
func (r *instanceRepository) SetRenewalPriceByProduct(ctx context.Context, productID uint64, price *float64, onlyBound bool) (int64, error) {
	var value any
	if price != nil && *price > 0 {
		value = money2(*price)
	}
	base := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("sell_product_id = ?", productID).
		Where("status <> ?", string(pkgmodel.InstanceStatusDeleted)).
		Where("coalesce(lifecycle_stage, '') <> ?", "destroyed")
	if onlyBound {
		base = base.Where("renewal_price IS NOT NULL")
	}
	res := base.Update("renewal_price", value)
	return res.RowsAffected, res.Error
}

// CountByProviderInstanceID 统计仍会向上游发指令的实例中，占用该上游号的**其他**实例数。
//
// 只统计"活着的"实例（排除已销毁）：销毁行不再向上游发任何指令，攥着一个上游号不会
// 造成"两台实例抢同一台机器"的问题；把它算作占用反而会挡住正常换绑
// （实测：上游换机后新号恰好等于某台已销毁行用过的号，重新关联被死锁）。
func (r *instanceRepository) CountByProviderInstanceID(ctx context.Context, providerInstanceID string, excludeID uint64) (int64, error) {
	id := strings.TrimSpace(providerInstanceID)
	if id == "" {
		return 0, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("provider_instance_id = ? AND id <> ?", id, excludeID).
		Where("status <> ?", string(pkgmodel.InstanceStatusDeleted)).
		Where("coalesce(lifecycle_stage, '') <> ?", "destroyed").
		Count(&n).Error
	return n, err
}

// RelinkUpstream 把实例重新关联到新的上游实例号，并回写回源拉到的字段。
//
// 只改「上游号 + 回源字段」这几列，**不动**账期/续费价/归属/来源订单：
// 那些是本系统的经营决策（谈定的价、本地账期口径），与"机器换成哪一台"无关。
// 上游号本身必须已探活（由服务层保证），这里不再复核。
func (r *instanceRepository) RelinkUpstream(ctx context.Context, id uint64, newUpstreamID string, migrateInstanceID bool, inst *UpstreamSnapshot) error {
	target := strings.TrimSpace(newUpstreamID)
	if target == "" {
		return errors.New("目标上游实例号不能为空")
	}
	updates := map[string]any{
		"provider_instance_id": target,
		"last_synced_at":       time.Now(),
		"updated_at":           time.Now(),
	}
	if migrateInstanceID {
		updates["instance_id"] = target
	}
	if inst != nil {
		if inst.Status != "" {
			updates["status"] = inst.Status
		}
		// IP / 名称 / 系统 / 区域只在回源确实给到时覆盖：
		// 空值覆盖会把本地已有信息抹成空（与 ApplySync 同一取舍）。
		for col, val := range map[string]string{
			"public_ip":  inst.PublicIP,
			"private_ip": inst.PrivateIP,
			"name":       inst.Name,
			"os":         inst.OS,
			"region":     inst.Region,
			"raw_data":   inst.RawData,
		} {
			if strings.TrimSpace(val) != "" {
				updates[col] = val
			}
		}
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// RebindProduct 更换实例绑定的本地商品与上游镜像。
//
// 不校验商品是否存在（服务层已校验）：本方法只负责写两列，
// 保持"仓储不做业务判断"的分工。
func (r *instanceRepository) RebindProduct(ctx context.Context, id uint64, sellProductID uint64, upstreamProductID *uint64) error {
	updates := map[string]any{"updated_at": time.Now()}
	if sellProductID > 0 {
		updates["sell_product_id"] = sellProductID
	}
	if upstreamProductID != nil {
		updates["upstream_product_id"] = *upstreamProductID
	}
	if len(updates) == 1 {
		// 只有 updated_at：调用方没给任何要改的绑定，视为空操作。
		return nil
	}
	return r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// FreeInstanceIDForDestroyed 把占用目标 instance_id 的**已销毁**行的号腾出来。
//
// 归档形式：`arc-<原号>-<记录ID>`（唯一且仍可读出原号），并同步把
// provider_instance_id 清空 —— 那台机器在上游已经不存在，留着一个指向已删机器的号
// 只会让"一号一台"守卫和回源刷新重复命中死号。
func (r *instanceRepository) FreeInstanceIDForDestroyed(ctx context.Context, instanceID string, keepID uint64) (int64, error) {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&syncmodel.Instance{}).
		Where("instance_id = ? AND id <> ?", id, keepID).
		Where("status = ? OR coalesce(lifecycle_stage, '') = ?", string(pkgmodel.InstanceStatusDeleted), "destroyed").
		Updates(map[string]any{
			"instance_id":          gorm.Expr("'arc-' || instance_id || '-' || id::text"),
			"provider_instance_id": "",
			"updated_at":           time.Now(),
		})
	return res.RowsAffected, res.Error
}

// normalizePage 规整分页参数（默认 20，单页上限 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
