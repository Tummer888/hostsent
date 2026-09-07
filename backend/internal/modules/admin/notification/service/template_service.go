package service

import (
	"context"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// TemplateService 通知模板服务。
type TemplateService interface {
	List(ctx context.Context) ([]notifydto.TemplateInfo, error)
	Update(ctx context.Context, id uint64, req *notifydto.TemplateUpdateRequest) (*notifydto.TemplateInfo, error)
}

type templateService struct {
	repo notifyrepo.TemplateRepository
}

func NewTemplateService(repo notifyrepo.TemplateRepository) TemplateService {
	return &templateService{repo: repo}
}

func (s *templateService) List(ctx context.Context) ([]notifydto.TemplateInfo, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]notifydto.TemplateInfo, 0, len(items))
	for _, t := range items {
		result = append(result, *toTemplateInfo(&t))
	}
	return result, nil
}

func (s *templateService) Update(ctx context.Context, id uint64, req *notifydto.TemplateUpdateRequest) (*notifydto.TemplateInfo, error) {
	// 先查询所有模板找到对应 ID 的
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	var found *notifymodel.NotificationTemplate
	for i := range items {
		if items[i].ID == id {
			found = &items[i]
			break
		}
	}
	if found == nil {
		return nil, ErrTemplateNotFound
	}
	if req.TitleTpl != "" {
		found.TitleTpl = req.TitleTpl
	}
	if req.ContentTpl != "" {
		found.ContentTpl = req.ContentTpl
	}
	if req.InboxOn != nil {
		found.InboxOn = *req.InboxOn
	}
	if req.MailOn != nil {
		found.MailOn = *req.MailOn
	}
	if req.Status != "" {
		found.Status = req.Status
	}
	if err := s.repo.Update(ctx, found); err != nil {
		return nil, err
	}
	return toTemplateInfo(found), nil
}

func toTemplateInfo(t *notifymodel.NotificationTemplate) *notifydto.TemplateInfo {
	return &notifydto.TemplateInfo{
		ID:         t.ID,
		Event:      t.Event,
		TitleTpl:   t.TitleTpl,
		ContentTpl: t.ContentTpl,
		InboxOn:    t.InboxOn,
		MailOn:     t.MailOn,
		Status:     t.Status,
		UpdatedAt:  t.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
