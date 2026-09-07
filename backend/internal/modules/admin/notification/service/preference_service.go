package service

import (
	"context"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// PreferenceService 用户通知偏好服务。
type PreferenceService interface {
	ListByUser(ctx context.Context, userID uint64) ([]notifydto.PreferenceItem, error)
	BatchUpsert(ctx context.Context, userID uint64, items []notifydto.PreferenceItem) error
}

type preferenceService struct {
	repo    notifyrepo.PreferenceRepository
	tplRepo notifyrepo.TemplateRepository
}

func NewPreferenceService(repo notifyrepo.PreferenceRepository, tplRepo notifyrepo.TemplateRepository) PreferenceService {
	return &preferenceService{repo: repo, tplRepo: tplRepo}
}

func (s *preferenceService) ListByUser(ctx context.Context, userID uint64) ([]notifydto.PreferenceItem, error) {
	prefs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 对所有活跃模板事件生成偏好项（无偏好记录时返回模板默认值）
	templates, err := s.tplRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	prefMap := make(map[string]*notifymodel.NotificationPreference)
	for i := range prefs {
		prefMap[prefs[i].Event] = &prefs[i]
	}
	result := make([]notifydto.PreferenceItem, 0, len(templates))
	for _, tpl := range templates {
		item := notifydto.PreferenceItem{
			Event:   tpl.Event,
			InboxOn: tpl.InboxOn,
			MailOn:  tpl.MailOn,
		}
		if pref, ok := prefMap[tpl.Event]; ok {
			item.InboxOn = pref.InboxOn
			item.MailOn = pref.MailOn
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *preferenceService) BatchUpsert(ctx context.Context, userID uint64, items []notifydto.PreferenceItem) error {
	prefs := make([]notifymodel.NotificationPreference, 0, len(items))
	for _, item := range items {
		prefs = append(prefs, notifymodel.NotificationPreference{
			UserID:  userID,
			Event:   item.Event,
			InboxOn: item.InboxOn,
			MailOn:  item.MailOn,
		})
	}
	return s.repo.BatchUpsert(ctx, prefs)
}
