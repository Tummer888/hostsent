package repository

import (
	"context"

	"gorm.io/gorm"
)

// Recipient 收件人信息（邮箱/手机号/显示名）。
type Recipient struct {
	UserID uint64
	Name   string
	Email  string
	Phone  string
}

// RecipientResolver 解析用户收件地址（通知发布时用）。
//
// 定义在仓储侧而非用户模块：通知域只需要「给我这个用户的邮箱与手机号」，
// 不希望为此依赖 uc/auth 的 repository 类型。
type RecipientResolver interface {
	ResolveRecipient(ctx context.Context, userID uint64) (Recipient, error)
}

type recipientResolver struct {
	db *gorm.DB
}

// NewRecipientResolver 创建收件人解析器。
func NewRecipientResolver(db *gorm.DB) RecipientResolver {
	return &recipientResolver{db: db}
}

func (r *recipientResolver) ResolveRecipient(ctx context.Context, userID uint64) (Recipient, error) {
	out := Recipient{UserID: userID}
	if userID == 0 {
		return out, gorm.ErrRecordNotFound
	}
	var row struct {
		Username string
		Email    string
		Phone    string
		RealName string
	}
	err := r.db.WithContext(ctx).Table("users").
		Select("username", "email", "phone", "real_name").
		Where("id = ?", userID).
		Take(&row).Error
	if err != nil {
		return out, err
	}
	out.Email = row.Email
	out.Phone = row.Phone
	out.Name = row.Username
	if row.RealName != "" {
		out.Name = row.RealName
	}
	return out, nil
}
