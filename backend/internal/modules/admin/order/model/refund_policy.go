package model

import "time"

// 退款策略（按「商品来源 + 分类」配置可退时限与自动退款）。
//
// 为什么独立成表而不是塞进 system_configs：
//   - 策略是多行结构（一个来源下多个分类各一套口径），JSON 配置无法做行级编辑、
//     无法用外键约束分类、也无法让运营在列表里直观看到「哪条生效」；
//   - 分类需要与 product_categories 建立可追溯的引用，配置表做不到。
//
// 匹配顺序（详见 service.MatchRefundPolicy）：
//  1. 商品来源 + 商品所属分类（含祖先链）逐级向上找，取**最深命中**；
//  2. 回落「同来源 + 全部分类」（category_id=0）；
//  3. 仍无命中 → 不可自助申请（保守默认：运营显式配置才放开退款）。
type RefundPolicy struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`
	// SourceMode 适用链路：self / upstream；空串表示不限来源（兜底行）。
	SourceMode string `gorm:"column:source_mode;size:16;not null;default:'';index"`
	// CategoryID 适用分类：0 表示「该来源下全部商品」；非 0 时含其子孙分类。
	CategoryID uint64 `gorm:"column:category_id;not null;default:0;index"`
	// Refundable 是否允许用户自助申请退款。false = 不允许（买了不退）。
	Refundable bool `gorm:"column:refundable;not null;default:false"`
	// WindowHours 可退时限（小时，从订单开通成功时间起算）；0 = 不限时。
	WindowHours int `gorm:"column:window_hours;not null;default:0"`
	// AutoApprove 符合条件时免人工审核自动退款（退回余额 + 全额 + 在时限内）。
	// 原路退回（channel）与部分退款永远走人工审核，不受本开关影响。
	AutoApprove bool      `gorm:"column:auto_approve;not null;default:false"`
	Status      string    `gorm:"size:16;not null;default:active;index"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0"`
	Remark      string    `gorm:"size:255"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (RefundPolicy) TableName() string {
	return "refund_policies"
}

// 策略状态
const (
	RefundPolicyStatusActive   string = "active"
	RefundPolicyStatusDisabled string = "disabled"
)

// RefundAttachment 退款审核凭据附件（人工退款时上传的转账凭证等）。
//
// 与工单附件同构（本地存储 + 鉴权下载），但独立成表：退款附件属资金审计材料，
// 保留期与可见范围都应与工单分开管理。
type RefundAttachment struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	RefundID   uint64    `gorm:"column:refund_id;index;not null"`
	FileName   string    `gorm:"column:file_name;size:255;not null"`
	FileURL    string    `gorm:"column:file_url;size:512;not null"` // 存储相对路径
	FileSize   int64     `gorm:"column:file_size"`
	FileType   string    `gorm:"column:file_type;size:64"`
	UploaderID uint64    `gorm:"column:uploader_id;not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (RefundAttachment) TableName() string {
	return "refund_attachments"
}
