// Package service 提供通知与消息中心模块的业务编排。
package service

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	"hostsent/backend/internal/pkg/notifier"
)

// Sentinel errors
var (
	ErrTemplateNotFound     = gorm.ErrRecordNotFound
	ErrNotificationNotFound = &appError{"notification not found", 20002}
	ErrAnnouncementNotFound = &appError{"announcement not found", 20002}
	ErrTemplateExists       = &appError{"event template already exists", 20004}
	ErrInvalidParams        = &appError{"invalid parameters", 20001}
)

type appError struct {
	msg  string
	code int
}

func (e *appError) Error() string { return e.msg }

// NotificationService 通知服务接口。
type NotificationService interface {
	// Publish 发布通知：模板渲染 → 偏好过滤 → 站内信落库 → 外发通道入队。
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
	// Resend 邮件失败重发（转为对投递记录重投，见 DeliveryService.RetryByNotification）。
	Resend(ctx context.Context, id uint64) error
	// SendMailTest 发送测试邮件。
	SendMailTest(ctx context.Context, to string) error
	// SetDeliveryDeps 注入投递队列依赖（doc90 N4）。
	//
	// 单独 setter 而非构造参数：装配顺序上投递仓储在通知服务之后构建，
	// 且既有单测仍按旧签名构造本服务。未注入时外发通道静默跳过（站内信不受影响）。
	SetDeliveryDeps(
		deliveries notifyrepo.DeliveryRepository,
		smsTplRepo notifyrepo.SmsTemplateRepository,
		userRepo notifyrepo.RecipientResolver,
	)
}

type notifyListResponse struct {
	Total    int64                        `json:"total"`
	List     []notifydto.NotificationInfo `json:"list"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

type notificationService struct {
	db       *gorm.DB
	repo     notifyrepo.NotificationRepository
	tplRepo  notifyrepo.TemplateRepository
	prefRepo notifyrepo.PreferenceRepository
	annRepo  notifyrepo.AnnouncementRepository
	// mailChannel 兼容门面（doc91 阶段的直发实现）；doc90 起外发统一走投递队列。
	mailChannel MailChannel
	// deliveries 投递队列；为 nil 时外发通道退化为不可用（站内信不受影响）。
	deliveries notifyrepo.DeliveryRepository
	// smsTplRepo 短信模板（读正文与上游模板号）。
	smsTplRepo notifyrepo.SmsTemplateRepository
	// userRepo 收件地址解析（邮箱/手机号）。
	userRepo notifyrepo.RecipientResolver
	logger   *zap.Logger
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

// SetDeliveryDeps 注入投递队列依赖（doc90 N4）。
//
// 单独 setter 而非构造参数：装配顺序上投递仓储/短信模板仓储在通知服务之后构建，
// 且既有单测（service_test.go）仍按旧签名构造本服务。
func (s *notificationService) SetDeliveryDeps(
	deliveries notifyrepo.DeliveryRepository,
	smsTplRepo notifyrepo.SmsTemplateRepository,
	userRepo notifyrepo.RecipientResolver,
) {
	s.deliveries = deliveries
	s.smsTplRepo = smsTplRepo
	s.userRepo = userRepo
}

// Publish 发布通知（doc90 §5.1 新流程）：
//
//	渲染模板 → 站内信落库（受偏好控制）→ 邮件/短信入投递队列
//
// notifications 表从此只装站内信；外发通道一律进 notification_deliveries。
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

	title := RenderTemplate(tpl.TitleTpl, in.Vars)
	content := RenderTemplate(tpl.ContentTpl, in.Vars)

	target := in.Target
	if target == "" {
		target = notifymodel.TargetUser
	}

	// 站内信通道：落库即送达。
	// bug ④ 修复：站内信同样受偏好控制（此前只判 tpl.InboxOn，用户关不掉）。
	// 例外：验证类事件（OTP）豁免，见 notifier.IsMandatoryEvent。
	if tpl.InboxOn && s.prefAllowed(ctx, in.UserID, in.Event, notifymodel.ChannelInbox) {
		n := &notifymodel.Notification{
			UserID:        in.UserID,
			TargetType:    target,
			Event:         in.Event,
			Title:         title,
			Content:       content,
			Channel:       notifymodel.ChannelInbox,
			SendStatus:    notifymodel.SendStatusSent,
			ContentFormat: notifymodel.FormatText,
			SourceModule:  in.SourceModule,
			SourceID:      in.SourceID,
		}
		if err := s.repo.CreateInbox(ctx, n); err != nil {
			if isDuplicateKey(err) {
				// bug ③：幂等唯一索引冲突说明重复发布，视为成功（不再真的插两条）。
				s.logger.Info("notification: inbox duplicate, skip",
					zap.String("event", in.Event), zap.String("source_id", in.SourceID))
			} else {
				s.logger.Error("notification: inbox create failed", zap.String("event", in.Event), zap.Error(err))
				return err
			}
		}
	}

	// 外发通道：邮件 / 短信。只入队，由 worker 投递（bug ⑤：不再往 notifications 插 mail 行）。
	s.enqueueExternal(ctx, tpl, in, target, title, content)
	return nil
}

// enqueueExternal 按模板开关与用户偏好把外发通道写入投递队列。
func (s *notificationService) enqueueExternal(
	ctx context.Context,
	tpl *notifymodel.NotificationTemplate,
	in notifydto.PublishInput,
	target, title, content string,
) {
	if s.deliveries == nil {
		return
	}
	wantMail := tpl.MailOn && s.prefAllowed(ctx, in.UserID, in.Event, notifymodel.ChannelMail)
	wantSMS := tpl.SmsOn && s.prefAllowed(ctx, in.UserID, in.Event, notifymodel.ChannelSMS)
	if !wantMail && !wantSMS {
		return
	}
	recipient := notifyrepo.Recipient{}
	if s.userRepo != nil {
		if r, err := s.userRepo.ResolveRecipient(ctx, in.UserID); err == nil {
			recipient = r
		}
	}
	if wantMail {
		if recipient.Email == "" {
			s.logger.Info("notification: skip mail, user has no email",
				zap.Uint64("user_id", in.UserID), zap.String("event", in.Event))
		} else {
			row := &notifymodel.NotificationDelivery{
				Event:         in.Event,
				Channel:       notifymodel.ChannelMail,
				TargetType:    target,
				TargetID:      in.UserID,
				TargetName:    recipient.Name,
				Recipient:     recipient.Email,
				Title:         title,
				Content:       content,
				ContentFormat: firstNonEmptyString(tpl.MailFormat, notifymodel.FormatText),
				Vars:          encodeVars(in.Vars),
				SendStatus:    notifymodel.DeliveryStatusPending,
				SourceModule:  in.SourceModule,
				SourceID:      in.SourceID,
			}
			if err := s.deliveries.Enqueue(ctx, row); err != nil {
				s.logger.Warn("notification: enqueue mail failed", zap.String("event", in.Event), zap.Error(err))
			}
		}
	}
	if wantSMS {
		if recipient.Phone == "" {
			s.logger.Info("notification: skip sms, user has no phone",
				zap.Uint64("user_id", in.UserID), zap.String("event", in.Event))
			return
		}
		// 短信正文来自 sms_templates（模板变量已按注册表校验过）。
		smsContent := content
		upstreamCode := ""
		if tpl.SmsTemplateID > 0 && s.smsTplRepo != nil {
			if st, err := s.smsTplRepo.FindByID(ctx, tpl.SmsTemplateID); err == nil {
				smsContent = RenderTemplate(st.Content, in.Vars)
				upstreamCode = st.UpstreamCode
			} else {
				s.logger.Warn("notification: sms template missing, fallback to mail content",
					zap.Uint64("sms_template_id", tpl.SmsTemplateID), zap.Error(err))
			}
		}
		row := &notifymodel.NotificationDelivery{
			Event:         in.Event,
			Channel:       notifymodel.ChannelSMS,
			TargetType:    target,
			TargetID:      in.UserID,
			TargetName:    recipient.Name,
			Recipient:     recipient.Phone,
			Title:         title,
			Content:       smsContent,
			ContentFormat: notifymodel.FormatText,
			Vars:          encodeVars(mergeVars(in.Vars, upstreamCode)),
			SendStatus:    notifymodel.DeliveryStatusPending,
			SourceModule:  in.SourceModule,
			SourceID:      in.SourceID,
		}
		if err := s.deliveries.Enqueue(ctx, row); err != nil {
			s.logger.Warn("notification: enqueue sms failed", zap.String("event", in.Event), zap.Error(err))
		}
	}
}

// prefAllowed 检查用户偏好是否允许该通道。
//
// 验证类事件（OTP/绑定/重置密码）必须豁免：用户不能通过关掉通知偏好来关掉登录验证码。
// 白名单与 doc91 共用 notifier.IsMandatoryEvent，避免两处各写一份。
func (s *notificationService) prefAllowed(ctx context.Context, userID uint64, event, channel string) bool {
	if notifier.IsMandatoryEvent(event) {
		return true
	}
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
	case notifymodel.ChannelSMS:
		return pref.SmsOn
	}
	return false
}

// mergeVars 把上游模板号并入变量表（worker 发送短信时需要）。
func mergeVars(vars map[string]string, upstreamCode string) map[string]string {
	out := map[string]string{}
	for k, v := range vars {
		out[k] = v
	}
	if upstreamCode != "" {
		out["_upstream_code"] = upstreamCode
	}
	return out
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

// Resend 重发（bug ② 修复）。
//
// 旧实现把 FindByID 拿到的（ID≠0）对象交给 SendAsync，SendAsync 内部又
// CreateInbox → 主键冲突 → 直接 return，邮件永远发不出去。新语义是
// 「找到该通知关联的投递记录并重投」，天然没有主键冲突。
func (s *notificationService) Resend(ctx context.Context, id uint64) error {
	n, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrNotificationNotFound
	}
	if s.deliveries == nil {
		return ErrNoDeliveryForNotification
	}
	channel := ""
	if n.Channel != notifymodel.ChannelInbox {
		channel = n.Channel
	}
	d, ferr := s.deliveries.FindBySource(ctx, n.SourceModule, n.SourceID, channel, n.UserID)
	if ferr != nil {
		if ferr == gorm.ErrRecordNotFound && n.DeliveryID > 0 {
			d, ferr = s.deliveries.FindByID(ctx, n.DeliveryID)
		}
		if ferr != nil {
			if ferr == gorm.ErrRecordNotFound {
				return ErrNoDeliveryForNotification
			}
			return ferr
		}
	}
	if !retryable(d.SendStatus) {
		return ErrDeliveryNotRetryable
	}
	return s.deliveries.ResetForRetry(ctx, d.ID)
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
		ID:            n.ID,
		UserID:        n.UserID,
		TargetType:    n.TargetType,
		Event:         n.Event,
		Title:         n.Title,
		Content:       n.Content,
		Channel:       n.Channel,
		SendStatus:    n.SendStatus,
		FailReason:    n.FailReason,
		SourceModule:  n.SourceModule,
		SourceID:      n.SourceID,
		ReadAt:        readAt,
		CreatedAt:     n.CreatedAt.Format("2006-01-02 15:04:05"),
		ContentFormat: n.ContentFormat,
		DeliveryID:    n.DeliveryID,
	}
}

func toNotificationInfoList(items []notifymodel.Notification) []notifydto.NotificationInfo {
	result := make([]notifydto.NotificationInfo, 0, len(items))
	for i := range items {
		result = append(result, *toNotificationInfo(&items[i]))
	}
	return result
}
