// Package service 提供通知与消息中心模块的业务编排。
package service

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// Sentinel errors
var (
	ErrTemplateNotFound = gorm.ErrRecordNotFound
	ErrNotificationNotFound = &appError{"notification not found", 20002}
	ErrAnnouncementNotFound = &appError{"announcement not found", 20002}
	ErrTemplateExists = &appError{"event template already exists", 20004}
	ErrInvalidParams = &appError{"invalid parameters", 20001}
)

type appError struct {
	msg  string
	code int
}

func (e *appError) Error() string { return e.msg }

// NotificationService 通知服务接口。
type NotificationService interface {
	// Publish 发布通知：模板渲染 → 偏好过滤 → 站内信落库 → 邮件异步。
	Publish(ctx context.Context, in notifydto.PublishInput) error
	// UnreadCount 未读数（定向 + 广播聚合）。
	UnreadCount(ctx context.Context, userID uint64, targetType string) (int64, error)
	// ListRecords 管理端通知记录列表。
	ListRecords(ctx context.Context, q notifydto.NotificationListQuery) (*notifyListResponse, error)
	// ListByUser 用户端通知列表（含定向 + 广播）。
	ListByUser(ctx context.Context, userID uint64, page, pageSize int) (*notifyListResponse, error)
	// GetDetail 用户端通知详情（自动置已读）。
	GetDetail(ctx context.Context, id, userID uint64) (*notifydto.NotificationInfo, error)
	// ReadAll 全部已读。
	ReadAll(ctx context.Context, userID uint64) error
	// Resend 邮件失败重发。
	Resend(ctx context.Context, id uint64) error
	// SendMailTest 发送测试邮件。
	SendMailTest(ctx context.Context, to string) error
}

type notifyListResponse struct {
	Total    int64                        `json:"total"`
	List     []notifydto.NotificationInfo `json:"list"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"page_size"`
}

type notificationService struct {
	db          *gorm.DB
	repo        notifyrepo.NotificationRepository
	tplRepo     notifyrepo.TemplateRepository
	prefRepo    notifyrepo.PreferenceRepository
	annRepo     notifyrepo.AnnouncementRepository
	mailChannel MailChannel
	logger      *zap.Logger
}

func NewNotificationService(
	db *gorm.DB,
	repo notifyrepo.NotificationRepository,
	tplRepo notifyrepo.TemplateRepository,
	prefRepo notifyrepo.PreferenceRepository,
	annRepo notifyrepo.AnnouncementRepository,
	mailChannel MailChannel,
	logger *zap.Logger,
) NotificationService {
	return &notificationService{
		db:          db,
		repo:        repo,
		tplRepo:     tplRepo,
		prefRepo:    prefRepo,
		annRepo:     annRepo,
		mailChannel: mailChannel,
		logger:      logger,
	}
}

// Publish 发布通知：模板渲染 → 偏好过滤 → 站内信落库 → 邮件异步。
func (s *notificationService) Publish(ctx context.Context, in notifydto.PublishInput) error {
	tpl, err := s.tplRepo.FindByEvent(ctx, in.Event)
	if err != nil {
		s.logger.Error("notification: template not found", zap.String("event", in.Event), zap.Error(err))
		return err
	}
	if tpl.Status != notifymodel.TemplateStatusActive {
		s.logger.Info("notification: template disabled, skip", zap.String("event", in.Event))
		return nil
	}

	title := renderTemplate(tpl.TitleTpl, in.Vars)
	content := renderTemplate(tpl.ContentTpl, in.Vars)

	target := in.Target
	if target == "" {
		target = notifymodel.TargetUser
	}

	// 站内信通道（默认开启）：落库即送达
	if tpl.InboxOn {
		n := &notifymodel.Notification{
			UserID:       in.UserID,
			TargetType:   target,
			Event:        in.Event,
			Title:        title,
			Content:      content,
			Channel:      notifymodel.ChannelInbox,
			SendStatus:   notifymodel.SendStatusSent,
			SourceModule: in.SourceModule,
			SourceID:     in.SourceID,
		}
		if err := s.repo.CreateInbox(ctx, n); err != nil {
			// 幂等索引冲突说明重复发布，视为成功
			s.logger.Info("notification: inbox duplicate, skip", zap.String("event", in.Event), zap.String("source_id", in.SourceID))
			return nil
		}
	}

	// 邮件通道：偏好允许且模板开启时异步发送
	if tpl.MailOn && s.prefAllowed(ctx, in.UserID, in.Event, notifymodel.ChannelMail) {
		s.mailChannel.SendAsync(notifymodel.Notification{
			UserID:       in.UserID,
			TargetType:   target,
			Event:        in.Event,
			Title:        title,
			Content:      content,
			Channel:      notifymodel.ChannelMail,
			SendStatus:   notifymodel.SendStatusPending,
			SourceModule: in.SourceModule,
			SourceID:     in.SourceID,
		})
	}
	return nil
}

// prefAllowed 检查用户偏好是否允许该通道。
func (s *notificationService) prefAllowed(ctx context.Context, userID uint64, event, channel string) bool {
	pref, err := s.prefRepo.FindByUserAndEvent(ctx, userID, event)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return true // 无偏好记录时走模板默认通道
		}
		return false
	}
	switch channel {
	case notifymodel.ChannelInbox:
		return pref.InboxOn
	case notifymodel.ChannelMail:
		return pref.MailOn
	}
	return false
}

// renderTemplate 模板变量替换：{var_name} → value。变量缺失时保留 {var} 原文。
func renderTemplate(tpl string, vars map[string]string) string {
	if vars == nil {
		return tpl
	}
	result := tpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// UnreadCount 未读数 = 定向通知未读 + 广播未读（排除已读表记录）。
func (s *notificationService) UnreadCount(ctx context.Context, userID uint64, targetType string) (int64, error) {
	return s.repo.CountUnread(ctx, userID, targetType)
}

// ListRecords 管理端通知记录列表。
func (s *notificationService) ListRecords(ctx context.Context, q notifydto.NotificationListQuery) (*notifyListResponse, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	items, total, err := s.repo.List(ctx, notifyrepo.NotificationListParams{
		Event:      q.Event,
		Channel:    q.Channel,
		SendStatus: q.SendStatus,
		TargetType: q.TargetType,
		Keyword:    q.Keyword,
		Offset:     offset,
		Limit:      pageSize,
	})
	if err != nil {
		return nil, err
	}
	return &notifyListResponse{
		Total:    total,
		List:     toNotificationInfoList(items),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListByUser 用户端通知列表（含定向 + 广播）。
func (s *notificationService) ListByUser(ctx context.Context, userID uint64, page, pageSize int) (*notifyListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	// 定向通知
	directItems, directTotal, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	// 广播通知
	broadcastItems, broadcastTotal, err := s.repo.ListBroadcast(ctx, notifymodel.TargetUser, page, pageSize)
	if err != nil {
		return nil, err
	}
	// 合并结果（简单拼接，实际可做更精细的排序分页）
	allItems := append(directItems, broadcastItems...)
	total := directTotal + broadcastTotal
	// 截取当前页
	offset := (page - 1) * pageSize
	if offset >= len(allItems) {
		allItems = nil
	} else {
		end := offset + pageSize
		if end > len(allItems) {
			end = len(allItems)
		}
		allItems = allItems[offset:end]
	}
	return &notifyListResponse{
		Total:    total,
		List:     toNotificationInfoList(allItems),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetDetail 用户端通知详情（自动置已读）。
func (s *notificationService) GetDetail(ctx context.Context, id, userID uint64) (*notifydto.NotificationInfo, error) {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrNotificationNotFound
	}
	// 权限校验：只能看自己的定向通知或广播通知
	if n.UserID != 0 && n.UserID != userID {
		return nil, ErrNotificationNotFound
	}
	// 自动置已读
	_ = s.repo.MarkRead(ctx, id, userID)
	// 重新查询以获取更新后的 read_at
	n, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toNotificationInfo(n), nil
}

// ReadAll 全部已读。
func (s *notificationService) ReadAll(ctx context.Context, userID uint64) error {
	return s.repo.MarkAllRead(ctx, userID)
}

// Resend 邮件失败重发。
func (s *notificationService) Resend(ctx context.Context, id uint64) error {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrNotificationNotFound
	}
	if n.Channel != notifymodel.ChannelMail {
		return ErrInvalidParams
	}
	// 异步重发
	s.mailChannel.SendAsync(*n)
	return nil
}

// SendMailTest 发送测试邮件。
func (s *notificationService) SendMailTest(ctx context.Context, to string) error {
	return s.mailChannel.SendTestMail(to)
}

// toNotificationInfo 将模型转换为 DTO。
func toNotificationInfo(n *notifymodel.Notification) *notifydto.NotificationInfo {
	var readAt *string
	if n.ReadAt != nil {
		t := n.ReadAt.Format("2006-01-02 15:04:05")
		readAt = &t
	}
	return &notifydto.NotificationInfo{
		ID:           n.ID,
		UserID:       n.UserID,
		TargetType:   n.TargetType,
		Event:        n.Event,
		Title:        n.Title,
		Content:      n.Content,
		Channel:      n.Channel,
		SendStatus:   n.SendStatus,
		FailReason:   n.FailReason,
		SourceModule: n.SourceModule,
		SourceID:     n.SourceID,
		ReadAt:       readAt,
		CreatedAt:    n.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toNotificationInfoList(items []notifymodel.Notification) []notifydto.NotificationInfo {
	result := make([]notifydto.NotificationInfo, 0, len(items))
	for i := range items {
		result = append(result, *toNotificationInfo(&items[i]))
	}
	return result
}
