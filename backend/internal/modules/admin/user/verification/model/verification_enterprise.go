// Package model 提供实名认证模块的数据模型。
package model


import "time"

// VerificationEnterprise 表示企业实名认证扩展信息。
type VerificationEnterprise struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement"`
	ApplicationID     uint64    `gorm:"column:application_id;not null;uniqueIndex"`
	CompanyName       string    `gorm:"column:company_name;size:255;not null"`
	CreditCodeMasked  string    `gorm:"column:credit_code_masked;size:64;not null"`
	LegalPersonName   string    `gorm:"column:legal_person_name;size:128"`
	LegalPersonIDMask string    `gorm:"column:legal_person_id_masked;size:64"`
	ContactName       string    `gorm:"column:contact_name;size:128"`
	ContactMobileMask string    `gorm:"column:contact_mobile_masked;size:32"`
	BusinessLicense   string    `gorm:"column:business_license_image;size:500"`
	ProofImages       string    `gorm:"column:company_proof_images;type:text"`
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

// TableName 返回企业实名认证扩展表名。
func (VerificationEnterprise) TableName() string {
	return "verification_enterprises"
}
