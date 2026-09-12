package model

import "time"

// 发票申请状态
const (
	InvoiceStatusNone     string = "none"     // 未开票（账单字段值）
	InvoiceStatusApplied  string = "applied"  // 已申请（账单字段值）
	InvoiceStatusIssued   string = "issued"   // 已开票（账单字段值）
	InvoiceStatusRejected string = "rejected" // 已驳回（账单字段值）
)

// 发票申请单状态（invoice_requests.status）
const (
	InvoiceReqPending  string = "pending"  // 待开票
	InvoiceReqIssued   string = "issued"   // 已开票
	InvoiceReqRejected string = "rejected" // 已驳回
)

// 发票申请渠道（预埋：税控系统对接）
const (
	InvoiceChannelManual string = "manual"  // 人工开票
	InvoiceChannelTaxAPI string = "tax_api" // 税控接口自动开票（预埋）
)

// 发票类型
const (
	InvoiceTypeNormal  string = "normal"  // 增值税普通发票
	InvoiceTypeSpecial string = "special" // 增值税专用发票
)

// InvoiceRequest 发票申请单。
//
// 本轮只承载「用户申请 → 管理端人工开票/驳回」闭环；
// channel/external_no/file_url 为税务系统与邮件下发预埋字段（doc36 §3.3 / P-04）。
type InvoiceRequest struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	RequestNo    string     `gorm:"column:request_no;size:64;uniqueIndex:uk_invoice_requests_no;not null"`
	BillID       uint64     `gorm:"column:bill_id;not null;default:0;index"`
	BillNo       string     `gorm:"column:bill_no;size:64;not null;default:''"`
	UserID       uint64     `gorm:"column:user_id;not null;index"`
	InvoiceType  string     `gorm:"column:invoice_type;size:20;not null;default:normal"`
	Title        string     `gorm:"size:200;not null;default:''"`
	TaxNo        string     `gorm:"column:tax_no;size:64;not null;default:''"`
	Amount       float64    `gorm:"type:decimal(15,2);not null;default:0"`
	Email        string     `gorm:"size:128;not null;default:''"`
	Status       string     `gorm:"size:20;not null;default:pending;index"`
	Channel      string     `gorm:"size:20;not null;default:manual"`
	ExternalNo   string     `gorm:"column:external_no;size:128;not null;default:''"`
	FileURL      string     `gorm:"column:file_url;size:512;not null;default:''"`
	RejectReason string     `gorm:"column:reject_reason;size:255;not null;default:''"`
	OperatorID   uint64     `gorm:"column:operator_id;not null;default:0"`
	IssuedAt     *time.Time `gorm:"column:issued_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (InvoiceRequest) TableName() string { return "invoice_requests" }

// InvoiceRow 发票申请展示行：附带用户名。
type InvoiceRow struct {
	InvoiceRequest
	Username string `gorm:"column:username"`
}
