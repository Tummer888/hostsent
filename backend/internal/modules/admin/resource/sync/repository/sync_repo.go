package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/resource/common/providerkind"
	"hostsent/backend/internal/modules/admin/resource/sync/dto"
	"hostsent/backend/internal/modules/admin/resource/sync/model"
	pkgmodel "hostsent/backend/internal/pkg/model"
)

// lifecycleStageCancelled 与 admin/instance 服务层的同名常量取同一字面量：
// 两边都从 pkgmodel 的语义派生，避免"两处各写一个字符串"漂移。
const lifecycleStageCancelled = "cancelled"

// SyncRepository 同步任务/日志/实例仓储接口
type SyncRepository interface {
	ListTasks(ctx context.Context, query dto.SyncTaskListQuery) ([]model.SyncTask, int64, error)
	FindTaskByID(ctx context.Context, id uint64) (*model.SyncTask, error)
	CreateTask(ctx context.Context, item *model.SyncTask) error
	UpdateTask(ctx context.Context, item *model.SyncTask) error
	CreateLog(ctx context.Context, item *model.SyncLog) error
	CountRunning(ctx context.Context, providerID uint64, taskType string) (int64, error)
	FindDueProviders(ctx context.Context, now time.Time) ([]uint64, error)
	ListLogs(ctx context.Context, query dto.SyncLogListQuery) ([]model.SyncLog, int64, error)
	ListInstances(ctx context.Context, query dto.InstanceListQuery) ([]model.Instance, int64, error)
	FindInstanceByID(ctx context.Context, id uint64) (*model.Instance, error)
	UpsertInstances(ctx context.Context, items []model.Instance) error
	// CreatePendingInstance 下单即建「待开通」实例占位（幂等，重复调用不产生第二行）。
	CreatePendingInstance(ctx context.Context, row *model.Instance) error
	// AdoptPendingInstance 开通成功：占位行就地转正为真实上游实例，返回是否命中占位行。
	AdoptPendingInstance(ctx context.Context, orderID uint64, row *model.Instance) (bool, error)
	// CancelPendingInstanceByOrder 把订单的占位实例标为「已取消」（未支付关单/用户取消时），
	// 返回影响行数。只动未转正的行（provider_instance_id 为空）：已开通实例不受影响。
	//
	// 语义（用户口径，第十九轮修订）：未付款订单超时/被取消 → 这台机器**从未开通**，
	// 状态落 cancelled，而不是把行删掉。
	//
	// 为什么不再物理删除：删掉之后运营在实例列表里看不到任何痕迹 —— 客户流失
	//（下了单没付钱）恰恰是最该被看到的数据。留 cancelled 行让"这单没成交"可查，
	// 也让订单与实例两侧能对上账（订单 closed/cancelled ↔ 实例 cancelled）。
	CancelPendingInstanceByOrder(ctx context.Context, orderID uint64) (int64, error)
	CountInstancesByProductUser(ctx context.Context, userID, productID uint64) (int64, error)
	// ExistingInstanceLinks 同步前回查既有归属（保留自营链路的 user_id/source_mode）。
	ExistingInstanceLinks(ctx context.Context, instanceIDs []string) (map[string]InstanceLinkage, error)
	// PlatformOwnerUserID 平台自持实例的归属账号（无单可挂时使用；0 表示无可用账号）。
	PlatformOwnerUserID(ctx context.Context) (uint64, error)
	// PurgeSyncDataBefore 分批清理 before 之前的同步任务与日志，返回删除总行数
	PurgeSyncDataBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
	// FailStaleTasks 将超时仍处于 pending/running 的任务标记为 failed，避免僵死任务永久阻塞调度
	FailStaleTasks(ctx context.Context, olderThan time.Time) (int64, error)
}

type syncRepository struct {
	db *gorm.DB
}

// NewSyncRepository 创建同步仓储实现
func NewSyncRepository(db *gorm.DB) SyncRepository {
	return &syncRepository{db: db}
}

func (r *syncRepository) ListTasks(ctx context.Context, query dto.SyncTaskListQuery) ([]model.SyncTask, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.SyncTask{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	// 链路过滤下推到 SQL：分页口径必须与筛选口径一致（见 providerkind 包注释）。
	base = providerkind.Apply(base, providerkind.Normalize(query.ProviderKind))
	if tt := strings.TrimSpace(query.TaskType); tt != "" {
		base = base.Where("task_type = ?", tt)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.SyncTask
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) FindTaskByID(ctx context.Context, id uint64) (*model.SyncTask, error) {
	var item model.SyncTask
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *syncRepository) CreateTask(ctx context.Context, item *model.SyncTask) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncRepository) UpdateTask(ctx context.Context, item *model.SyncTask) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *syncRepository) CreateLog(ctx context.Context, item *model.SyncLog) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *syncRepository) CountRunning(ctx context.Context, providerID uint64, taskType string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.SyncTask{}).
		Where("provider_id = ? AND task_type = ? AND status IN ?", providerID, taskType, []string{"pending", "running"}).
		Count(&n).Error
	return n, err
}

func (r *syncRepository) FindDueProviders(ctx context.Context, now time.Time) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).
		Table("resource_providers AS p").
		Where("p.sync_enabled = ?", true).
		Where("p.status = ?", 1).
		Where("p.sync_paused = ?", false).
		Where("p.deleted_at IS NULL").
		Where("p.last_sync_at IS NULL OR p.last_sync_at + make_interval(secs => p.sync_interval) <= ?", now).
		Where("NOT EXISTS (SELECT 1 FROM sync_tasks t WHERE t.provider_id = p.id AND t.status IN ('pending','running'))").
		Pluck("p.id", &ids).Error
	return ids, err
}

func (r *syncRepository) ListLogs(ctx context.Context, query dto.SyncLogListQuery) ([]model.SyncLog, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.SyncLog{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	base = providerkind.Apply(base, providerkind.Normalize(query.ProviderKind))
	if query.TaskID > 0 {
		base = base.Where("task_id = ?", query.TaskID)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.SyncLog
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) ListInstances(ctx context.Context, query dto.InstanceListQuery) ([]model.Instance, int64, error) {
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

	base := r.db.WithContext(ctx).Model(&model.Instance{})
	if query.ProviderID > 0 {
		base = base.Where("provider_id = ?", query.ProviderID)
	}
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if st := strings.TrimSpace(query.Status); st != "" {
		base = base.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("name ILIKE ? OR instance_id ILIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Instance
	if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *syncRepository) FindInstanceByID(ctx context.Context, id uint64) (*model.Instance, error) {
	var item model.Instance
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// UpsertInstances 按 instance_id 幂等写入实例（同步发现与订单开通共用）。
//
// provider_instance_id 单独处理：只在本次值非空时才随 upsert 覆盖。
// 该列是"实例是否已在上游开通"的判据（占位行为空），一旦被空串覆盖，
// 已开通的实例会被当成待开通——详情页能力位全关、重新开通按钮出现、
// 回源刷新直接跳过（实测：实例 1003 的 2049 在一个只读路径后被清空，
// 此后订单 832 的推进日志一直是 "refresh skipped status: ”"）。
// 同步/开通两条链路里"不知道上游实例号"的写入方不应有权抹掉已知的实例号。
func (r *syncRepository) UpsertInstances(ctx context.Context, items []model.Instance) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "instance_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":     gorm.Expr("excluded.status"),
			"name":       gorm.Expr("excluded.name"),
			"public_ip":  gorm.Expr("excluded.public_ip"),
			"private_ip": gorm.Expr("excluded.private_ip"),
			"cpu":        gorm.Expr("excluded.cpu"),
			"memory":     gorm.Expr("excluded.memory"),
			"disk":       gorm.Expr("excluded.disk"),
			"raw_data":   gorm.Expr("excluded.raw_data"),
			"expire_at":  gorm.Expr("excluded.expire_at"),
			"updated_at": gorm.Expr("excluded.updated_at"),
			// 规格明细随回源刷新（此前只更新 cpu/memory/disk，带宽/系统/区域
			// 在线路上永远保持首次落库值：实测同一实例 bandwidth=30 而上游商品是 15、
			// os 显示 CentOS 而上游是 Debian 这类错值）。
			// 用 CASE 保护：回源未给值（空/0）时不覆盖本地已有值 —— 上游详情拉取
			// 失败时适配器只返回最小实例，无条件覆盖会把好数据抹成空。
			"disk_type": gorm.Expr("CASE WHEN excluded.disk_type <> '' THEN excluded.disk_type ELSE instances.disk_type END"),
			"bandwidth": gorm.Expr("CASE WHEN excluded.bandwidth > 0 THEN excluded.bandwidth ELSE instances.bandwidth END"),
			"os":        gorm.Expr("CASE WHEN excluded.os <> '' THEN excluded.os ELSE instances.os END"),
			"region":    gorm.Expr("CASE WHEN excluded.region <> '' THEN excluded.region ELSE instances.region END"),
			"zone":      gorm.Expr("CASE WHEN excluded.zone <> '' THEN excluded.zone ELSE instances.zone END"),
			// 双链路语义列随同步刷新（T1.2）；sell_product_id 属自营链路，不被上游同步覆盖。
			"source_mode":         gorm.Expr("excluded.source_mode"),
			"upstream_product_id": gorm.Expr("excluded.upstream_product_id"),
			"provider_instance_id": gorm.Expr(
				"CASE WHEN excluded.provider_instance_id <> '' THEN excluded.provider_instance_id ELSE instances.provider_instance_id END",
			),
			// 登录密码密文（迁移 100）：同步来源的行永远为空、开通落库的行有值。
			// 与 raw_data 相反，这里只进不退——空值不得覆盖已有密文，否则每次同步都会把密码抹掉。
			"credential_password": gorm.Expr(
				"CASE WHEN excluded.credential_password <> '' THEN excluded.credential_password ELSE instances.credential_password END",
			),
		}),
	}).Create(&items).Error
}

// CreatePendingInstance 下单即建「待开通」实例占位（instance_id 为本地占位号）。
//
// 占位行让"已下单但还没开通成功"的机器在实例列表里可见可操作（查看/重新开通/重试同步），
// 而不是开通失败后什么都看不到。判定"是否已在上游开通"的唯一依据是
// ProviderInstanceID 是否为空 —— 占位行为空，转正时写入真实实例号。
// 幂等：ON CONFLICT(instance_id) DO NOTHING，重复下单/重试不会建出第二行占位。
func (r *syncRepository) CreatePendingInstance(ctx context.Context, row *model.Instance) error {
	if row == nil || row.InstanceID == "" {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}},
		DoNothing: true,
	}).Create(row).Error
}

// AdoptPendingInstance 开通成功：把该订单的占位行就地改写为真实上游实例
// （实例号/状态/IP/规格/账期一次补齐），返回是否命中占位行。
//
// 为什么不是"插一行新的"：占位行是实例列表/详情的稳定锚点（前端可能正开着详情页），
// 另插一行会留下两条记录、用户会看到一台"待开通"的幽灵机。
// 命中条件 provider_instance_id = ” 保证只改占位、不改已转正行；
// 若目标实例号已被别的行占用（早前失败重试留下的行）则不更新，交调用方走常规 upsert。
//
// `status <> 'deleted'` 排除**已被运营清除的占位行**：运营在列表里删掉一台开不出来的
// 占位机，是在表达"这条记录我不再要了"。若此时履约迟到成功仍把它就地转正，就会得到
// 一台 status=running 但 lifecycle_stage 还是 destroyed 的怪行 —— 而终态行按
// "终态不复活"规则不再接受回源刷新，这台真机器此后永远刷不出真实状态。
// 排除后该订单走常规 upsert 落一条**新行**，被删的旧行保持已删除。
func (r *syncRepository) AdoptPendingInstance(ctx context.Context, orderID uint64, row *model.Instance) (bool, error) {
	if orderID == 0 || row == nil || row.InstanceID == "" {
		return false, nil
	}
	res := r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("order_id = ? AND provider_instance_id = ''", orderID).
		// COALESCE 而非裸 `status <> 'deleted'`：status 为空的历史行在 SQL 三值逻辑下
		// 会让整个条件求值为 NULL（= 不成立），那些行就再也无法被转正，
		// 履约成功后会另插一行、留下"待开通"幽灵机 —— 正是本函数要避免的情形。
		Where("COALESCE(status, '') <> ?", "deleted").
		Where("NOT EXISTS (SELECT 1 FROM instances x WHERE x.instance_id = ? AND x.id <> instances.id)", row.InstanceID).
		Updates(adoptPendingUpdates(row))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// adoptPendingUpdates 占位行转正的更新列。
//
// 独立成函数是为了让「哪些列不该被空值覆盖」显式可见：credential_password 只在
// 开通侧确实带了密文时才写——占位行本没有密码，用空串覆盖会把上一次开通已存的
// 密码抹掉（例如补跑一次不带密码的接管流程）。
func adoptPendingUpdates(row *model.Instance) map[string]any {
	updates := map[string]any{
		"instance_id":          row.InstanceID,
		"provider_instance_id": row.ProviderInstanceID,
		"status":               row.Status,
		"name":                 row.Name,
		"public_ip":            row.PublicIP,
		"private_ip":           row.PrivateIP,
		"cpu":                  row.CPU,
		"memory":               row.Memory,
		"disk":                 row.Disk,
		"disk_type":            row.DiskType,
		"bandwidth":            row.Bandwidth,
		"os":                   row.OS,
		"region":               row.Region,
		"zone":                 row.Zone,
		"raw_data":             row.RawData,
		"expire_at":            row.ExpireAt,
		"last_synced_at":       time.Now(),
		"updated_at":           time.Now(),
	}
	if row.CredentialPassword != "" {
		updates["credential_password"] = row.CredentialPassword
	}
	return updates
}

// CancelPendingInstanceByOrder 把某订单的占位实例标为「已取消」（未支付关单/取消时调用）。
//
// 只动尚未转正的行（provider_instance_id 为空）：已开通的实例不因订单状态变化被抹掉。
// 同时清掉过渡态：一台从未开通的机器不可能"正在执行"任何动作。
func (r *syncRepository) CancelPendingInstanceByOrder(ctx context.Context, orderID uint64) (int64, error) {
	if orderID == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("order_id = ? AND provider_instance_id = ''", orderID).
		// 已是终态的行不覆盖：已取消的重复关单不该改写时间戳，
		// 已转正/已销毁的行更不该被这次写回退（双保险，条件与 WHERE 同源）。
		Where("COALESCE(status, '') NOT IN ?", []string{"deleted", "cancelled"}).
		Updates(map[string]any{
			"status":                string(pkgmodel.InstanceStatusCancelled),
			"lifecycle_stage":       lifecycleStageCancelled,
			"transition_state":      nil,
			"transition_started_at": nil,
			"transition_until":      nil,
			"updated_at":            time.Now(),
		})
	return res.RowsAffected, res.Error
}

// CountInstancesByProductUser 统计某用户**已在上游开通**某商品的实例数，用于开通幂等判断。
//
// 匹配列是 sell_product_id（= products.id），与订单 orders.product_id 同一命名空间。
// 注意：instances 的旧兼容列 product_id 已于迁移 035 退役，禁止再引用，否则查询直接报错，
// 幂等护栏静默失效（订购重试会重复向上游下单）。
//
// 必须排除「待开通」占位行（provider_instance_id 为空）：占位行在下单时即创建、
// 同样带 user_id 与 sell_product_id，把它算作"已开通"会让履约侧的
// "已记录实例则直接激活"短路在机器还没开出来时就把订单置为服务中。
func (r *syncRepository) CountInstancesByProductUser(ctx context.Context, userID, productID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Instance{}).
		Where("user_id = ? AND sell_product_id = ?", userID, productID).
		Where("provider_instance_id <> ''").
		Count(&count).Error
	return count, err
}

// InstanceLinkage 某上游实例号在本地的既有归属与链路语义。
type InstanceLinkage struct {
	UserID     uint64
	SourceMode string
	// UpstreamProductID 既有上游资源商品绑定；自营实例为 0，同步不得覆盖。
	UpstreamProductID uint64
	// LifecycleStage 既有生命周期阶段。destroyed 是本系统认定的终态（已销毁/终止），
	// 上游若仍返回该实例（软删未清、接口缓存），同步不得把它复活成在跑机器。
	LifecycleStage string
}

// ExistingInstanceLinks 批量查 instance_id → 既有归属与链路语义。
//
// 用途：同步只应"发现"上游实例，不应把我方经订单开通的自营实例改写成上游链路、
// 也不应改掉它的归属用户——那会让实例从用户控制台消失、续费/生命周期错挂。
// 因此 upsert 前回查既有行，命中则把 user_id / source_mode / upstream_product_id
// 恢复为原值（这三列都在 UpsertInstances 的更新列或NOT NULL 约束上）。
func (r *syncRepository) ExistingInstanceLinks(ctx context.Context, instanceIDs []string) (map[string]InstanceLinkage, error) {
	out := make(map[string]InstanceLinkage, len(instanceIDs))
	if len(instanceIDs) == 0 {
		return out, nil
	}
	type row struct {
		InstanceID        string
		UserID            uint64
		SourceMode        string
		UpstreamProductID uint64
		LifecycleStage    string
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&model.Instance{}).
		Select("instance_id, user_id, source_mode, upstream_product_id, lifecycle_stage").
		Where("instance_id IN ?", instanceIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		out[item.InstanceID] = InstanceLinkage{
			UserID:            item.UserID,
			SourceMode:        item.SourceMode,
			UpstreamProductID: item.UpstreamProductID,
			LifecycleStage:    item.LifecycleStage,
		}
	}
	return out, nil
}

// PlatformOwnerUserID 返回"平台自持实例"的归属账号：上游新增、我方无订单可挂的实例
// （典型是面板上人工开的机器）需要有合法 user_id——instances.user_id 是全库唯一指向
// users 的外键，同步写 0 会直接撞 23503，整个实例同步任务失败（实测 provider 19 连续
// 数十次 23503）。
//
// 选号规则（不新建账号，避免与既有 69 行用户数据口径冲突）：
//  1. 优先取 id 最小的正常账号（status=active、非子账号、未注销）——平台自持资源挂首个账号；
//  2. 一个都没有时返回 0，由调用方把该实例登记为差异并跳过，绝不写入非法 user_id。
func (r *syncRepository) PlatformOwnerUserID(ctx context.Context) (uint64, error) {
	var id uint64
	err := r.db.WithContext(ctx).Table("users").
		Select("id").
		Where("status = ? AND deleted_at IS NULL AND is_sub_account = ?", "active", false).
		Order("id asc").
		Limit(1).
		Pluck("id", &id).Error
	return id, err
}

// PurgeSyncDataBefore 分批删除 before 之前的同步日志与任务，返回删除总行数。
// 先删日志（引用任务），再删终态任务；pending/running 任务保留，避免清掉在跑的任务。
// 每批 batchSize 行（默认 5000），避免单条长事务锁表。
func (r *syncRepository) PurgeSyncDataBefore(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 5000
	}
	var total int64
	for {
		res := r.db.WithContext(ctx).Exec(
			`DELETE FROM sync_logs WHERE id IN (SELECT id FROM sync_logs WHERE created_at < ? ORDER BY id LIMIT ?)`,
			before, batchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batchSize) {
			break
		}
	}
	for {
		res := r.db.WithContext(ctx).Exec(
			`DELETE FROM sync_tasks WHERE id IN (SELECT id FROM sync_tasks WHERE created_at < ? AND status NOT IN ('pending','running') ORDER BY id LIMIT ?)`,
			before, batchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batchSize) {
			break
		}
	}
	return total, nil
}

// FailStaleTasks 回收僵死任务：进程重启/执行中断会让任务永远停在 pending/running，
// 而 FindDueProviders 的 NOT EXISTS 防重入判定会因此永久跳过该渠道。
func (r *syncRepository) FailStaleTasks(ctx context.Context, olderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE sync_tasks
		    SET status = 'failed',
		        error_message = COALESCE(NULLIF(error_message, ''), '任务超时被回收（进程重启或执行中断）'),
		        completed_at = now()
		  WHERE status IN ('pending', 'running')
		    AND COALESCE(started_at, created_at) < ?`, olderThan)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
