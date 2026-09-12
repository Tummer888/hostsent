package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hostsent/backend/internal/modules/admin/finance/bill/dto"
	"hostsent/backend/internal/modules/admin/finance/bill/model"
)

// BillRepository 账单数据访问。
type BillRepository interface {
	Upsert(ctx context.Context, b *model.Bill) error
	FindByID(ctx context.Context, id uint64) (*model.Bill, error)
	FindByUserPeriod(ctx context.Context, userID uint64, period string) (*model.Bill, error)
	List(ctx context.Context, q dto.BillListQuery) ([]model.Bill, int64, error)
	Close(ctx context.Context, id uint64) error
	// MarkPaid 记录账单收款方式并置为已结清（doc34 F-11：账单须描述所用支付方式）。
	MarkPaid(ctx context.Context, id uint64, paidAmountFen int64, paidMethod string, paidChannelID uint64) error
	// MarkInvoice 回写账单发票状态（doc36 §3.3）。
	MarkInvoice(ctx context.Context, id uint64, status, invoiceNo string) error
	// —— 发票申请单 ——
	CreateInvoice(ctx context.Context, req *model.InvoiceRequest) error
	FindInvoiceByID(ctx context.Context, id uint64) (*model.InvoiceRow, error)
	FindPendingInvoiceByBill(ctx context.Context, billID uint64) (*model.InvoiceRequest, error)
	UpdateInvoice(ctx context.Context, id uint64, fields map[string]any) error
	ListInvoices(ctx context.Context, q dto.InvoiceListQuery) ([]model.InvoiceRow, int64, error)
}

type billRepository struct {
	db *gorm.DB
}

// NewBillRepository 创建账单仓储。
func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepository{db: db}
}

// Upsert 依据 (user_id, period) 唯一约束幂等写入账单。
//
// 重算只更新金额与分类，不覆盖 status/paid_*：账单一旦结清或关账，
// 重新生成（口径调整、补算）不得把它打回未结，否则会丢失收款与发票依据。
func (r *billRepository) Upsert(ctx context.Context, b *model.Bill) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "period"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"total_amount", "refund_amount", "detail",
			"bill_type", "consume_amount", "renewal_amount",
			"channel_refund_amount", "refund_fee_amount", "updated_at",
		}),
	}).Create(b).Error
}

func (r *billRepository) FindByID(ctx context.Context, id uint64) (*model.Bill, error) {
	var b model.Bill
	if err := r.db.WithContext(ctx).First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *billRepository) FindByUserPeriod(ctx context.Context, userID uint64, period string) (*model.Bill, error) {
	var b model.Bill
	if err := r.db.WithContext(ctx).Where("user_id = ? AND period = ?", userID, period).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *billRepository) List(ctx context.Context, q dto.BillListQuery) ([]model.Bill, int64, error) {
	var total int64
	if err := r.listQuery(ctx, q, true).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Bill
	err := r.listQuery(ctx, q, false).
		Order("period desc, id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// listQuery 构造账单查询条件；count 模式不 JOIN users（无需用户名，省一次关联）。
func (r *billRepository) listQuery(ctx context.Context, q dto.BillListQuery, isCount bool) *gorm.DB {
	base := r.db.WithContext(ctx).Model(&model.Bill{})
	if q.UserID > 0 {
		base = base.Where("user_id = ?", q.UserID)
	}
	if kw := normalizeKeyword(q.UserKeyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("user_id IN (SELECT id FROM users WHERE username ILIKE ? OR email ILIKE ?)", like, like)
	}
	if q.BillNo != "" {
		base = base.Where("bill_no = ?", q.BillNo)
	}
	if q.Keyword != "" {
		base = base.Where("bill_no ILIKE ?", "%"+q.Keyword+"%")
	}
	if q.Period != "" {
		base = base.Where("period = ?", q.Period)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if q.BillType != "" {
		base = base.Where("bill_type = ?", q.BillType)
	}
	if q.InvoiceStatus != "" {
		base = base.Where("invoice_status = ?", q.InvoiceStatus)
	}
	_ = isCount
	return base
}

func (r *billRepository) Close(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Bill{}).Where("id = ?", id).Update("status", model.BillStatusClosed).Error
}

// MarkPaid 记录账单实际收款方式并结清。
func (r *billRepository) MarkPaid(ctx context.Context, id uint64, paidAmountFen int64, paidMethod string, paidChannelID uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Bill{}).Where("id = ?", id).Updates(map[string]any{
		"status":          model.BillStatusPaid,
		"paid_amount_fen": paidAmountFen,
		"paid_method":     paidMethod,
		"paid_channel_id": paidChannelID,
		"paid_at":         &now,
	}).Error
}

// MarkInvoice 回写账单发票状态；已开票时记录发票号与开票时间。
func (r *billRepository) MarkInvoice(ctx context.Context, id uint64, status, invoiceNo string) error {
	fields := map[string]any{"invoice_status": status, "invoice_no": invoiceNo}
	if status == model.InvoiceStatusIssued {
		now := time.Now()
		fields["invoiced_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&model.Bill{}).Where("id = ?", id).Updates(fields).Error
}

// —— 发票申请单 ——

func (r *billRepository) CreateInvoice(ctx context.Context, req *model.InvoiceRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *billRepository) FindInvoiceByID(ctx context.Context, id uint64) (*model.InvoiceRow, error) {
	var row model.InvoiceRow
	err := r.db.WithContext(ctx).Table("invoice_requests ir").
		Select("ir.*, COALESCE(u.username, '') AS username").
		Joins("LEFT JOIN users u ON u.id = ir.user_id").
		Where("ir.id = ?", id).Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *billRepository) FindPendingInvoiceByBill(ctx context.Context, billID uint64) (*model.InvoiceRequest, error) {
	var req model.InvoiceRequest
	err := r.db.WithContext(ctx).
		Where("bill_id = ? AND status = ?", billID, model.InvoiceReqPending).
		First(&req).Error
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *billRepository) UpdateInvoice(ctx context.Context, id uint64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.InvoiceRequest{}).Where("id = ?", id).Updates(fields).Error
}

func (r *billRepository) ListInvoices(ctx context.Context, q dto.InvoiceListQuery) ([]model.InvoiceRow, int64, error) {
	var total int64
	if err := r.invoiceQuery(ctx, q).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.InvoiceRow
	err := r.invoiceQuery(ctx, q).
		Select("ir.*, COALESCE(u.username, '') AS username").
		Joins("LEFT JOIN users u ON u.id = ir.user_id").
		Order("ir.id desc").
		Offset((normalizePage(q.Page) - 1) * normalizePageSize(q.PageSize)).
		Limit(normalizePageSize(q.PageSize)).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *billRepository) invoiceQuery(ctx context.Context, q dto.InvoiceListQuery) *gorm.DB {
	base := r.db.WithContext(ctx).Table("invoice_requests ir")
	if q.UserID > 0 {
		base = base.Where("ir.user_id = ?", q.UserID)
	}
	if q.Status != "" {
		base = base.Where("ir.status = ?", q.Status)
	}
	if q.BillNo != "" {
		base = base.Where("ir.bill_no = ?", q.BillNo)
	}
	return base
}

// —— 工具 ——

func normalizeKeyword(raw string) string {
	return strings.TrimSpace(raw)
}
