package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// DeliveryService 投递队列管理能力（发送日志页）。
type DeliveryService interface {
	List(ctx context.Context, q notifydto.DeliveryListQuery) (*notifydto.DeliveryListResponse, error)
	Get(ctx context.Context, id uint64) (*notifydto.DeliveryInfo, error)
	// Retry 单条重投：只允许 failed / dead / skipped（sent 不允许，避免重复发送）。
	Retry(ctx context.Context, id uint64) error
	// BatchRetry 批量重投，逐条校验状态；返回成功与跳过明细。
	BatchRetry(ctx context.Context, ids []uint64) (*notifydto.DeliveryRetryResponse, error)
	// RetryByNotification 通知记录页「重发」的转发实现：
	// 读通知 → 找对应投递记录 → 重投；无关联投递记录时提示无外发通道。
	RetryByNotification(ctx context.Context, notificationID uint64) error
}

type deliveryService struct {
	repo    notifyrepo.DeliveryRepository
	notRepo notifyrepo.NotificationRepository
}

// NewDeliveryService 创建投递管理服务。
func NewDeliveryService(repo notifyrepo.DeliveryRepository, notRepo notifyrepo.NotificationRepository) DeliveryService {
	return &deliveryService{repo: repo, notRepo: notRepo}
}

func (s *deliveryService) List(ctx context.Context, q notifydto.DeliveryListQuery) (*notifydto.DeliveryListResponse, error) {
	items, total, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]notifydto.DeliveryInfo, 0, len(items))
	for i := range items {
		out = append(out, toDeliveryInfo(&items[i]))
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 20
	}
	return &notifydto.DeliveryListResponse{
		Items: out,
		Meta:  notifydto.ListMeta{Page: page, PageSize: size, Total: total},
	}, nil
}

func (s *deliveryService) Get(ctx context.Context, id uint64) (*notifydto.DeliveryInfo, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}
	info := toDeliveryInfo(d)
	return &info, nil
}

func (s *deliveryService) Retry(ctx context.Context, id uint64) error {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrDeliveryNotFound
		}
		return err
	}
	if !retryable(d.SendStatus) {
		return ErrDeliveryNotRetryable
	}
	return s.repo.ResetForRetry(ctx, id)
}

func (s *deliveryService) BatchRetry(ctx context.Context, ids []uint64) (*notifydto.DeliveryRetryResponse, error) {
	resp := &notifydto.DeliveryRetryResponse{Skipped: []string{}}
	for _, id := range ids {
		d, err := s.repo.FindByID(ctx, id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				resp.Skipped = append(resp.Skipped, idLabel(id)+": 记录不存在")
				continue
			}
			return nil, err
		}
		if !retryable(d.SendStatus) {
			resp.Skipped = append(resp.Skipped, idLabel(id)+": 当前状态 "+d.SendStatus+" 不允许重投")
			continue
		}
		if err := s.repo.ResetForRetry(ctx, id); err != nil {
			return nil, err
		}
		resp.Retried++
	}
	return resp, nil
}

// RetryByNotification 兼容旧调用：/notifications/:id/resend 内部转发到这里。
func (s *deliveryService) RetryByNotification(ctx context.Context, notificationID uint64) error {
	n, err := s.notRepo.FindByID(ctx, notificationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrNotificationNotFound
		}
		return err
	}
	// 纯站内信没有外发投递记录，无从重投（前端会隐藏按钮，这里兜底报错）。
	if n.Channel == notifymodel.ChannelInbox && n.DeliveryID == 0 {
		return ErrNoDeliveryForNotification
	}
	channel := n.Channel
	if channel == notifymodel.ChannelInbox {
		channel = ""
	}
	d, err := s.repo.FindBySource(ctx, n.SourceModule, n.SourceID, channel, n.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound && n.DeliveryID > 0 {
			d, err = s.repo.FindByID(ctx, n.DeliveryID)
		}
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrNoDeliveryForNotification
			}
			return err
		}
	}
	if !retryable(d.SendStatus) {
		return ErrDeliveryNotRetryable
	}
	return s.repo.ResetForRetry(ctx, d.ID)
}

func retryable(status string) bool {
	switch status {
	case notifymodel.DeliveryStatusFailed, notifymodel.DeliveryStatusDead, notifymodel.DeliveryStatusSkipped:
		return true
	}
	return false
}

func idLabel(id uint64) string {
	return "#" + itoa(id)
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// toDeliveryInfo 转换并打码收件地址（前端不再二次打码）。
func toDeliveryInfo(d *notifymodel.NotificationDelivery) notifydto.DeliveryInfo {
	var vars map[string]string
	if strings.TrimSpace(d.Vars) != "" {
		_ = json.Unmarshal([]byte(d.Vars), &vars)
	}
	return notifydto.DeliveryInfo{
		ID:            d.ID,
		BatchID:       d.BatchID,
		Event:         d.Event,
		Channel:       d.Channel,
		TargetType:    d.TargetType,
		TargetID:      d.TargetID,
		TargetName:    d.TargetName,
		Recipient:     MaskRecipient(d.Channel, d.Recipient),
		ChannelID:     d.ChannelID,
		Title:         d.Title,
		Content:       d.Content,
		ContentFormat: d.ContentFormat,
		Vars:          vars,
		SendStatus:    d.SendStatus,
		Attempts:      d.Attempts,
		MaxAttempts:   d.MaxAttempts,
		NextRetryAt:   formatTimePtr(d.NextRetryAt),
		ProviderMsgID: d.ProviderMsgID,
		ProviderCode:  d.ProviderCode,
		CostFen:       d.CostFen,
		FailReason:    d.FailReason,
		SourceModule:  d.SourceModule,
		SourceID:      d.SourceID,
		SentAt:        formatTimePtr(d.SentAt),
		CreatedAt:     d.CreatedAt.Format(time.RFC3339),
	}
}

// MaskRecipient 收件地址打码：邮箱保留首字符与域名，手机保留前 3 后 4。
// 空地址与站内信原样返回（无敏感信息）。
func MaskRecipient(channel, recipient string) string {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return ""
	}
	switch channel {
	case notifymodel.ChannelMail:
		at := strings.LastIndex(recipient, "@")
		if at <= 0 {
			return maskMiddle(recipient, 1, 0)
		}
		local, domain := recipient[:at], recipient[at:]
		if len(local) <= 1 {
			return "*" + domain
		}
		return local[:1] + strings.Repeat("*", len(local)-1) + domain
	case notifymodel.ChannelSMS:
		if len(recipient) >= 7 {
			return recipient[:3] + strings.Repeat("*", len(recipient)-7) + recipient[len(recipient)-4:]
		}
		return maskMiddle(recipient, 1, 0)
	}
	return recipient
}

func maskMiddle(s string, head, tail int) string {
	r := []rune(s)
	if len(r) <= head+tail {
		return strings.Repeat("*", len(r))
	}
	return string(r[:head]) + strings.Repeat("*", len(r)-head-tail) + string(r[len(r)-tail:])
}
