package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
)

// SmsTemplateRepository 短信模板数据访问。
type SmsTemplateRepository interface {
	Create(ctx context.Context, t *notifymodel.SmsTemplate) error
	Update(ctx context.Context, t *notifymodel.SmsTemplate) error
	FindByID(ctx context.Context, id uint64) (*notifymodel.SmsTemplate, error)
	FindByCode(ctx context.Context, code string) (*notifymodel.SmsTemplate, error)
	List(ctx context.Context, q notifydto.SmsTemplateListQuery) ([]notifymodel.SmsTemplate, int64, error)
	Delete(ctx context.Context, id uint64) error
	// ReferencedByNotificationTemplate 是否被通知模板引用（引用中不允许删除）。
	ReferencedByNotificationTemplate(ctx context.Context, id uint64) (bool, error)
}

type smsTemplateRepository struct {
	db *gorm.DB
}

// NewSmsTemplateRepository 创建短信模板仓储。
func NewSmsTemplateRepository(db *gorm.DB) SmsTemplateRepository {
	return &smsTemplateRepository{db: db}
}

func (r *smsTemplateRepository) Create(ctx context.Context, t *notifymodel.SmsTemplate) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *smsTemplateRepository) Update(ctx context.Context, t *notifymodel.SmsTemplate) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *smsTemplateRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.SmsTemplate, error) {
	var t notifymodel.SmsTemplate
	if err := r.db.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *smsTemplateRepository) FindByCode(ctx context.Context, code string) (*notifymodel.SmsTemplate, error) {
	var t notifymodel.SmsTemplate
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *smsTemplateRepository) List(ctx context.Context, q notifydto.SmsTemplateListQuery) ([]notifymodel.SmsTemplate, int64, error) {
	base := r.db.WithContext(ctx).Model(&notifymodel.SmsTemplate{})
	if q.Scene != "" {
		base = base.Where("scene = ?", q.Scene)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	var items []notifymodel.SmsTemplate
	if err := base.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *smsTemplateRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&notifymodel.SmsTemplate{}, id).Error
}

func (r *smsTemplateRepository) ReferencedByNotificationTemplate(ctx context.Context, id uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&notifymodel.NotificationTemplate{}).
		Where("sms_template_id = ?", id).
		Count(&count).Error
	return count > 0, err
}

// TemplateVarRepository 模板变量注册表数据访问。
type TemplateVarRepository interface {
	Create(ctx context.Context, v *notifymodel.SmsTemplateVar) error
	Update(ctx context.Context, v *notifymodel.SmsTemplateVar) error
	FindByID(ctx context.Context, id uint64) (*notifymodel.SmsTemplateVar, error)
	FindByKey(ctx context.Context, key string) (*notifymodel.SmsTemplateVar, error)
	List(ctx context.Context, category string) ([]notifymodel.SmsTemplateVar, error)
	// ListActiveKeys 返回启用变量名（模板保存时校验白名单）。
	ListActiveKeys(ctx context.Context) ([]string, error)
}

type templateVarRepository struct {
	db *gorm.DB
}

// NewTemplateVarRepository 创建模板变量仓储。
func NewTemplateVarRepository(db *gorm.DB) TemplateVarRepository {
	return &templateVarRepository{db: db}
}

func (r *templateVarRepository) Create(ctx context.Context, v *notifymodel.SmsTemplateVar) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *templateVarRepository) Update(ctx context.Context, v *notifymodel.SmsTemplateVar) error {
	return r.db.WithContext(ctx).Save(v).Error
}

func (r *templateVarRepository) FindByID(ctx context.Context, id uint64) (*notifymodel.SmsTemplateVar, error) {
	var v notifymodel.SmsTemplateVar
	if err := r.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *templateVarRepository) FindByKey(ctx context.Context, key string) (*notifymodel.SmsTemplateVar, error) {
	var v notifymodel.SmsTemplateVar
	if err := r.db.WithContext(ctx).Where("var_key = ?", key).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *templateVarRepository) List(ctx context.Context, category string) ([]notifymodel.SmsTemplateVar, error) {
	q := r.db.WithContext(ctx).Model(&notifymodel.SmsTemplateVar{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var items []notifymodel.SmsTemplateVar
	if err := q.Order("sort_order asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *templateVarRepository) ListActiveKeys(ctx context.Context) ([]string, error) {
	var keys []string
	if err := r.db.WithContext(ctx).Model(&notifymodel.SmsTemplateVar{}).
		Where("status = ?", notifymodel.TemplateVarStatusActive).
		Pluck("var_key", &keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}
