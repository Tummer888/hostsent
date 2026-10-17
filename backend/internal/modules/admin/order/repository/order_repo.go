// Package repository 提供订单域的数据访问实现。
package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
)

// OrderRepository 定义订单数据访问能力。
type OrderRepository interface {
	List(ctx context.Context, query dto.OrderListQuery) ([]model.Order, int64, error)
	FindByID(ctx context.Context, id uint64) (*model.Order, error)
	// FindByNo 按订单号读取（支付成功事件回查业务单用）。
	FindByNo(ctx context.Context, orderNo string) (*model.Order, error)
	FindByIDs(ctx context.Context, ids []uint64) ([]model.Order, error)
	Create(ctx context.Context, item *model.Order) error
	Update(ctx context.Context, item *model.Order) error
	// SetUpstreamInvoiceID 记录上游账单 id（开通幂等：重试复用同一张账单）。
	SetUpstreamInvoiceID(ctx context.Context, id, invoiceID uint64) error
	// SetDownstreamToken 记录上游对接令牌（首次履约生成后固话，重试复用同一令牌）。
	SetDownstreamToken(ctx context.Context, id uint64, token string) error
	// SetCredentialPassword 记录开通密码密文（首次履约生成后固化，重试复用同一密码；
	// 开通成功由履约装配层原样搬进 instances.credential_password）。
	SetCredentialPassword(ctx context.Context, id uint64, encrypted string) error
	// SetCredentialPasswordIfEmpty 仅在 credential_password 为空时写入（原子占位）：
	// 并发重试下只有一个 worker 的密码能被采纳，避免"已下发密码 X、本地记着 Y"的错位。
	SetCredentialPasswordIfEmpty(ctx context.Context, id uint64, encrypted string) error
	// ListProvisioningStale 扫描「开通中」且已超过 since 的订单（上游主机异步开通的兜底）。
	// 上游创建主机后可能长时间停在 Pending，本系统若无后续推进，订单会永远卡在
	// provisioning（用户实测：开通成功但订单/实例一直是"开通中"）。
	ListProvisioningStale(ctx context.Context, before time.Time, limit int) ([]model.Order, error)
	// ListExpiredPending 扫描超期未支付的订单（created_at < before，按下单先后）。
	// 供待支付订单过期关单调度器使用：以「下单时间 + 配置的有效期」判定，
	// 不用 orders.expire_time —— 那列是实例计费到期时间，语义不同。
	ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]model.Order, error)
	// TransitionStatus 条件状态迁移（CAS）：仅当当前状态为 from 时置为 to，返回受影响行数。
	// 支付成功回调与取消/超时关单会并发命中同一笔待支付订单，用 CAS 保证只有一方生效：
	// 关单不会覆盖并发的支付成功，支付成功也不会复活已关闭的订单。
	TransitionStatus(ctx context.Context, id uint64, from, to string) (int64, error)
	// MarkVerified 条件核验（CAS）：仅当 verified_at 为空时写入核验人与时间，返回是否生效。
	// 重复核验/并发核验只有一方算数，另一方得 0 行 —— 与状态迁移同一套「谁先谁生效」语义。
	MarkVerified(ctx context.Context, id, operatorID uint64, operatorName, remark string, at time.Time) (int64, error)
	// ConfirmPayment 条件确认收款（CAS）：仅当订单仍为待支付时才置为已支付并落地
	// 支付方式/实收金额/到账时间/操作人，返回受影响行数。
	// 与关单/取消并发时只有一方生效，避免人工入账把已作废的订单复活。
	ConfirmPayment(ctx context.Context, id uint64, payMethod string, paidAmount float64, payTime time.Time, operatorID uint64, remark string) (int64, error)
	// SoftDelete 条件软删除（CAS）：仅当当前状态在 allowStatuses 内才置 deleted_at。
	// 订单是财务凭证，删除只做「从列表隐藏」的软删除，行与金额始终留在库里可审计。
	SoftDelete(ctx context.Context, id uint64, allowStatuses []string, at time.Time) (int64, error)
	// StatsCountToday 今日订单量
	StatsCountToday(ctx context.Context) (int64, error)
	// StatsSalesToday 今日销售额（已支付口径）
	StatsSalesToday(ctx context.Context) (float64, error)
	// StatsTotal 累计（订单总量 + 销售总额）
	StatsTotal(ctx context.Context) (total int64, sales float64, err error)
	// CountByStatus 订单状态分布
	CountByStatus(ctx context.Context) ([]model.OrderStatusCount, error)
	// StatsTrend 近 N 日订单趋势（销售额 + 订单量）
	StatsTrend(ctx context.Context, days int) ([]model.OrderTrendPoint, error)
}

type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建订单仓储实现。
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) List(ctx context.Context, query dto.OrderListQuery) ([]model.Order, int64, error) {
	page := normalizePage(query.Page)
	pageSize := normalizePageSize(query.PageSize)

	// 软删除的订单对列表不可见（订单模型是普通 *time.Time 列，GORM 不会自动加作用域，
	// 所有读取路径必须显式过滤）。
	base := r.db.WithContext(ctx).Model(&model.Order{}).Where("deleted_at IS NULL")
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("order_no ILIKE ? OR product_name ILIKE ?", like, like)
	}
	if userKeyword := strings.TrimSpace(query.UserKeyword); userKeyword != "" {
		like := "%" + userKeyword + "%"
		base = base.Joins("JOIN users ON users.id = orders.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ?", like, like, like)
	}
	if query.UserID > 0 {
		base = base.Where("user_id = ?", query.UserID)
	}
	if query.ProductID > 0 {
		base = base.Where("product_id = ?", query.ProductID)
	}
	if query.Status != "" {
		base = base.Where("status = ?", query.Status)
	}
	if query.PayMethod != "" {
		base = base.Where("pay_method = ?", query.PayMethod)
	}
	// 支付单号 / 渠道流水号检索（doc36 §3.1）：支付单与业务单解耦存储，
	// 按 biz_no 命中 payment_orders 再回捞订单，避免在 orders 上冗余渠道字段。
	// 用裸 SQL 子查询而非 import 支付模型，保持订单域不反向依赖支付中心。
	if paymentNo := strings.TrimSpace(query.PaymentNo); paymentNo != "" {
		base = base.Where("order_no IN (?)",
			r.db.Table("payment_orders").Select("biz_no").
				Where("biz_type = ? AND payment_no ILIKE ?", "order", "%"+paymentNo+"%"))
	}
	if channelTx := strings.TrimSpace(query.ChannelTx); channelTx != "" {
		base = base.Where("order_no IN (?)",
			r.db.Table("payment_orders").Select("biz_no").
				Where("biz_type = ? AND channel_tx ILIKE ?", "order", "%"+channelTx+"%"))
	}
	// 订单类型（doc36 §3.4）：续费订单以 renewal_id != 0 判定，无独立 order_type 列。
	switch strings.TrimSpace(query.OrderType) {
	case "renewal":
		base = base.Where("renewal_id <> 0")
	case "consume":
		base = base.Where("renewal_id = 0")
	}
	// 核验状态（运营人工核验）：pending=服务中且未核验；verified=已核验。
	// 以状态列为准推导，不读配置开关 —— 列表筛选是"查数据"，不是"看提醒"。
	switch strings.TrimSpace(query.VerifyState) {
	case dto.VerifyStatePending:
		base = base.Where("status = ? AND verified_at IS NULL", model.OrderStatusActive)
	case dto.VerifyStateVerified:
		base = base.Where("verified_at IS NOT NULL")
	}
	if start := normalizeTime(query.StartTime); start != nil {
		base = base.Where("created_at >= ?", *start)
	}
	if end := normalizeTime(query.EndTime); end != nil {
		base = base.Where("created_at <= ?", *end)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Order
	if err := base.Order("created_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	// 反查本页订单的支付单号 / 渠道流水号（doc36 §3.1）；失败不阻断列表。
	_ = r.fillPaymentInfo(ctx, items)
	return items, total, nil
}

// fillPaymentInfo 从支付中心补齐订单的支付单号与渠道流水号（doc36 §3.1）。
// 同一业务单可能有多张支付单（换渠道重试 / 关闭后重开），取最近一张。
func (r *orderRepository) fillPaymentInfo(ctx context.Context, items []model.Order) error {
	if len(items) == 0 {
		return nil
	}
	orderNos := make([]string, 0, len(items))
	for _, it := range items {
		orderNos = append(orderNos, it.OrderNo)
	}
	var rows []struct {
		BizNo     string `gorm:"column:biz_no"`
		PaymentNo string `gorm:"column:payment_no"`
		ChannelTx string `gorm:"column:channel_tx"`
	}
	if err := r.db.WithContext(ctx).Table("payment_orders").
		Select("biz_no, payment_no, channel_tx").
		Where("biz_type = ? AND biz_no IN ?", "order", orderNos).
		Order("id desc").
		Find(&rows).Error; err != nil {
		return err
	}
	type payInfo struct{ paymentNo, channelTx string }
	latest := make(map[string]payInfo, len(rows))
	for _, row := range rows {
		if _, ok := latest[row.BizNo]; ok {
			continue // id desc：首个即最新
		}
		latest[row.BizNo] = payInfo{paymentNo: row.PaymentNo, channelTx: row.ChannelTx}
	}
	for i := range items {
		if info, ok := latest[items[i].OrderNo]; ok {
			items[i].PaymentNo = info.paymentNo
			items[i].ChannelTx = info.channelTx
		}
	}
	return nil
}

func (r *orderRepository) FindByID(ctx context.Context, id uint64) (*model.Order, error) {
	var item model.Order
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByNo 按订单号读取。
func (r *orderRepository) FindByNo(ctx context.Context, orderNo string) (*model.Order, error) {
	var item model.Order
	if err := r.db.WithContext(ctx).Where("order_no = ? AND deleted_at IS NULL", orderNo).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *orderRepository) FindByIDs(ctx context.Context, ids []uint64) ([]model.Order, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.Order
	if err := r.db.WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *orderRepository) Create(ctx context.Context, item *model.Order) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// SetUpstreamInvoiceID 只更新上游账单 id 一列：开通链路里调用，避免整行写回带来的并发覆盖。
func (r *orderRepository) SetUpstreamInvoiceID(ctx context.Context, id, invoiceID uint64) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ?", id).
		Update("upstream_invoice_id", invoiceID).Error
}

// SetDownstreamToken 只更新上游对接令牌一列（首次履约生成，之后固定不变）。
func (r *orderRepository) SetDownstreamToken(ctx context.Context, id uint64, token string) error {
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ?", id).
		Update("downstream_token", token).Error
}

// SetCredentialPassword 只更新开通密码密文一列（与令牌同理：首发生成后固化，
// 重试必须复用，避免上游机器上实际密码与本地留存不一致）。
func (r *orderRepository) SetCredentialPassword(ctx context.Context, id uint64, encrypted string) error {
	if strings.TrimSpace(encrypted) == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ?", id).
		Update("credential_password", encrypted).Error
}

// SetCredentialPasswordIfEmpty 原子占位：仅当该列当前为空才写入。
//
// 用条件更新而不是"先查后写"：开通任务可能被两个 worker 同时领取（锁过期重领），
// 先查后写会让后到的覆盖先到的，而上游已经收到了先到的那个密码——本地留存与实际密码就此错位。
func (r *orderRepository) SetCredentialPasswordIfEmpty(ctx context.Context, id uint64, encrypted string) error {
	if strings.TrimSpace(encrypted) == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND (credential_password IS NULL OR credential_password = '')", id).
		Update("credential_password", encrypted).Error
}

func (r *orderRepository) Update(ctx context.Context, item *model.Order) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// ListExpiredPending 超期未支付订单（按下单时间升序，便于先进先出关单）。
//
// 只挑「用户走过收银台」的订单：必须有支付单记录（payment_orders.biz_type='order'）。
// 续费单（renewal_id<>0）与后台代下单是无支付单的 pending，语义是「等余额/等人工结算」，
// 超时关掉会把用户已充值的续费意图悄悄作废，因此排除在外（与 doc88 §6.2 口径一致）。
// ListProvisioningStale 扫描开通中且久未推进的订单（renewal_id=0 的首次开通订单）。
// 只取有下游绑定信息的（upstream_invoice_id>0，即确实在上游下了单），
// 避免把纯本地推进的订单卷进来。
func (r *orderRepository) ListProvisioningStale(ctx context.Context, before time.Time, limit int) ([]model.Order, error) {
	if limit <= 0 {
		limit = 200
	}
	var items []model.Order
	if err := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ? AND renewal_id = 0 AND upstream_invoice_id > 0 AND deleted_at IS NULL",
			model.OrderStatusProvisioning, before).
		Order("updated_at asc, id asc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *orderRepository) ListExpiredPending(ctx context.Context, before time.Time, limit int) ([]model.Order, error) {
	if limit <= 0 {
		limit = 200
	}
	var items []model.Order
	if err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ? AND renewal_id = 0 AND channel = '' AND deleted_at IS NULL", model.OrderStatusPending, before).
		Where("order_no IN (?)",
			r.db.Table("payment_orders").Select("biz_no").Where("biz_type = ?", "order")).
		Order("created_at asc, id asc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// TransitionStatus 条件状态迁移（CAS）：仅当当前状态为 from 时置为 to。
func (r *orderRepository) TransitionStatus(ctx context.Context, id uint64, from, to string) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, from).
		Updates(map[string]interface{}{"status": to, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// MarkVerified 条件核验（CAS）：只有未核验的订单能被核验，重复调用返回 0 行。
func (r *orderRepository) MarkVerified(ctx context.Context, id, operatorID uint64, operatorName, remark string, at time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND verified_at IS NULL AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"verified_at":      at,
			"verified_by":      operatorID,
			"verified_by_name": operatorName,
			"verify_remark":    remark,
			"updated_at":       time.Now(),
		})
	return res.RowsAffected, res.Error
}

// ConfirmPayment 条件确认收款（CAS）：仅待支付订单可被确认为已支付。
//
// 用一条带 status='pending' 守卫的 UPDATE 而不是「先读再 Save」：
// 关单调度器与用户取消都会改同一行的状态，读改写会让"已被超时关掉的订单"
// 被人工入账复活（时间差内两边都读到 pending）。受影响行数 0 即说明状态已变，
// 由 service 层翻译成「订单状态不允许该操作」。
func (r *orderRepository) ConfirmPayment(ctx context.Context, id uint64, payMethod string, paidAmount float64, payTime time.Time, operatorID uint64, remark string) (int64, error) {
	updates := map[string]interface{}{
		"status":      model.OrderStatusPaid,
		"pay_method":  payMethod,
		"paid_amount": paidAmount,
		"pay_time":    payTime,
		"operator_id": operatorID,
		"updated_at":  time.Now(),
	}
	if remark != "" {
		updates["remark"] = remark
	}
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, model.OrderStatusPending).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// SoftDelete 条件软删除（CAS）：仅当状态在 allowStatuses 内才置 deleted_at。
// 不用 GORM 的 Delete（那会按 DeletedAt 语义走，但这里需要状态白名单守卫 + 返回行数）。
func (r *orderRepository) SoftDelete(ctx context.Context, id uint64, allowStatuses []string, at time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND deleted_at IS NULL AND status IN ?", id, allowStatuses).
		Updates(map[string]interface{}{"deleted_at": at, "updated_at": time.Now()})
	return res.RowsAffected, res.Error
}

// paidStatuses 统计销售额时视为已支付的订单状态集合。
var paidStatuses = []string{
	model.OrderStatusPaid,
	model.OrderStatusProvisioning,
	model.OrderStatusActive,
	model.OrderStatusRefunding,
	model.OrderStatusRefunded,
	model.OrderStatusCompleted,
}

func (r *orderRepository) StatsCountToday(ctx context.Context) (int64, error) {
	var count int64
	start := dayStart(time.Now())
	if err := r.db.WithContext(ctx).Model(&model.Order{}).Where("created_at >= ? AND deleted_at IS NULL", start).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *orderRepository) StatsSalesToday(ctx context.Context) (float64, error) {
	var sum float64
	start := dayStart(time.Now())
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("created_at >= ? AND status IN ? AND deleted_at IS NULL", start, paidStatuses).
		Select("COALESCE(SUM(paid_amount), 0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *orderRepository) StatsTotal(ctx context.Context) (int64, float64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var sales float64
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status IN ? AND deleted_at IS NULL", paidStatuses).
		Select("COALESCE(SUM(paid_amount), 0)").Scan(&sales).Error; err != nil {
		return 0, 0, err
	}
	return total, sales, nil
}

func (r *orderRepository) CountByStatus(ctx context.Context) ([]model.OrderStatusCount, error) {
	var counts []model.OrderStatusCount
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("deleted_at IS NULL").
		Select("status, count(*) as count").
		Group("status").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	return counts, nil
}

func (r *orderRepository) StatsTrend(ctx context.Context, days int) ([]model.OrderTrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	start := dayStart(time.Now()).AddDate(0, 0, -(days - 1))
	salesIn := strings.Join(paidStatuses, "','")
	var points []model.OrderTrendPoint
	if err := r.db.WithContext(ctx).Model(&model.Order{}).
		Select("to_char(created_at, 'YYYY-MM-DD') as date, SUM(CASE WHEN status IN ('"+salesIn+"') THEN paid_amount ELSE 0 END) as sales, COUNT(*) as orders").
		Where("created_at >= ? AND deleted_at IS NULL", start).
		Group("date").
		Order("date asc").
		Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
