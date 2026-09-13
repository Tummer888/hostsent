package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
)

// broadcastDefaultMaxTargets 单次群发命中人数上限默认值（可由 system_configs 覆盖）。
const broadcastDefaultMaxTargets int64 = 5000

// BroadcastService 消息群发。
type BroadcastService interface {
	Targets(ctx context.Context, q notifydto.BroadcastTargetQuery) (*notifydto.BroadcastTargetResponse, error)
	Preview(ctx context.Context, req notifydto.BroadcastPreviewRequest) (*notifydto.BroadcastPreviewResponse, error)
	Send(ctx context.Context, req notifydto.BroadcastRequest) (*notifydto.BroadcastResponse, error)
}

type broadcastService struct {
	broadcastRepo notifyrepo.BroadcastRepository
	deliveryRepo  notifyrepo.DeliveryRepository
	notifyRepo    notifyrepo.NotificationRepository
	smsTplRepo    notifyrepo.SmsTemplateRepository
	configReader  func(ctx context.Context, key string) (string, bool, error)
	logger        *zap.Logger
}

// NewBroadcastService 创建群发服务。
func NewBroadcastService(
	broadcastRepo notifyrepo.BroadcastRepository,
	deliveryRepo notifyrepo.DeliveryRepository,
	notifyRepo notifyrepo.NotificationRepository,
	smsTplRepo notifyrepo.SmsTemplateRepository,
	configReader func(ctx context.Context, key string) (string, bool, error),
	logger *zap.Logger,
) BroadcastService {
	return &broadcastService{
		broadcastRepo: broadcastRepo, deliveryRepo: deliveryRepo,
		notifyRepo: notifyRepo, smsTplRepo: smsTplRepo,
		configReader: configReader, logger: logger,
	}
}

func (s *broadcastService) maxTargets(ctx context.Context) int64 {
	if s.configReader != nil {
		if v, ok, err := s.configReader(ctx, "notify_broadcast_max_targets"); err == nil && ok {
			var n int64
			if _, serr := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); serr == nil && n > 0 {
				return n
			}
		}
	}
	return broadcastDefaultMaxTargets
}

func (s *broadcastService) Targets(ctx context.Context, q notifydto.BroadcastTargetQuery) (*notifydto.BroadcastTargetResponse, error) {
	switch q.Mode {
	case "groups":
		groups, err := s.broadcastRepo.ListGroups(ctx)
		if err != nil {
			return nil, err
		}
		return &notifydto.BroadcastTargetResponse{Groups: groups}, nil
	case "users":
		users, total, err := s.broadcastRepo.SearchUsers(ctx, q.Keyword, q.Page, q.PageSize)
		if err != nil {
			return nil, err
		}
		return &notifydto.BroadcastTargetResponse{Total: total, Items: toBroadcastUserItems(users)}, nil
	case "filter":
		target := notifydto.BroadcastTarget{
			Mode: "filter",
			Filter: &notifydto.BroadcastFilter{
				RegisteredAfter: q.RegisteredAfter, RegisteredBefore: q.RegisteredBefore,
				Tier: q.Tier, HasInstance: q.HasInstance, MinBalance: q.MinBalance,
				MaxBalance: q.MaxBalance, Status: q.Status,
			},
		}
		total, err := s.broadcastRepo.CountTargets(ctx, target)
		if err != nil {
			return nil, err
		}
		return &notifydto.BroadcastTargetResponse{Total: total}, nil
	default:
		return nil, fmt.Errorf("%w: mode 只支持 groups / users / filter", ErrInvalidParams)
	}
}

func (s *broadcastService) Preview(ctx context.Context, req notifydto.BroadcastPreviewRequest) (*notifydto.BroadcastPreviewResponse, error) {
	maxTargets := s.maxTargets(ctx)
	total, err := s.broadcastRepo.CountTargets(ctx, req.Target)
	if err != nil {
		return nil, err
	}
	resp := &notifydto.BroadcastPreviewResponse{
		Total:      total,
		Sample:     []notifydto.BroadcastPreviewSample{},
		MaxTargets: maxTargets,
	}
	if total == 0 {
		resp.Message = "该条件未命中任何用户，请放宽筛选"
		return resp, nil
	}
	sample, err := s.broadcastRepo.SampleTargets(ctx, req.Target, 10)
	if err != nil {
		return nil, err
	}
	for _, u := range sample {
		resp.Sample = append(resp.Sample, notifydto.BroadcastPreviewSample{
			ID:          u.ID,
			Username:    u.Username,
			EmailMasked: MaskRecipient(notifymodel.ChannelMail, u.Email),
			PhoneMasked: MaskRecipient(notifymodel.ChannelSMS, u.Phone),
		})
	}

	// 渲染预览必须用真实模板 + 样例变量（或首个目标用户），让运营看得见最终文案。
	vars := map[string]string{
		"site_name":   s.siteName(ctx),
		"username":    "",
		"order_no":    "",
		"amount":      "",
		"balance":     "",
		"threshold":   "",
		"days_left":   "",
		"expire_date": "",
		"hostname":    "",
	}
	for k, v := range req.Vars {
		vars[k] = v
	}
	if len(sample) > 0 {
		vars["username"] = sample[0].Username
	}
	resp.Rendered = notifydto.RenderedPreview{
		Title:   RenderTemplate(req.Title, vars),
		Content: RenderTemplate(req.Content, vars),
	}

	// 费用与条数估算：只对真的会被外发的通道计数。
	for _, ch := range req.Channels {
		switch ch {
		case notifymodel.ChannelMail:
			resp.EstimatedMailCount = total
		case notifymodel.ChannelSMS:
			content := resp.Rendered.Content
			if code := strings.TrimSpace(req.SmsTemplateCode); code != "" && s.smsTplRepo != nil {
				if tpl, terr := s.smsTplRepo.FindByCode(ctx, code); terr == nil {
					content = RenderTemplate(tpl.Content, vars)
				}
			}
			_, seg := SmsSegments(content)
			if seg < 1 {
				seg = 1
			}
			// 单价按 system_configs.sms_unit_price_fen（分/条），默认 5 分。
			resp.EstimatedSMSCostFen = int64(seg) * total * s.smsUnitPriceFen(ctx)
		}
	}
	if total > maxTargets {
		resp.Exceeded = true
		resp.Message = fmt.Sprintf("命中 %d 人，超过单次群发上限 %d 人，请缩小范围后重试", total, maxTargets)
	}
	return resp, nil
}

func (s *broadcastService) Send(ctx context.Context, req notifydto.BroadcastRequest) (*notifydto.BroadcastResponse, error) {
	channels := normalizeChannels(req.Channels)
	if len(channels) == 0 {
		return nil, ErrBroadcastChannelEmpty
	}
	needSMS := containsString(channels, notifymodel.ChannelSMS)
	if needSMS && strings.TrimSpace(req.SmsTemplateCode) == "" {
		return nil, ErrBroadcastSmsTemplateRequired
	}
	maxTargets := s.maxTargets(ctx)
	total, err := s.broadcastRepo.CountTargets(ctx, req.Target)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, ErrBroadcastTargetEmpty
	}
	if total > maxTargets {
		return nil, fmt.Errorf("%w（%d > %d）", ErrBroadcastTooMany, total, maxTargets)
	}

	// 短信模板正文（channels 含 sms 时取；含变量校验过的正文）。
	smsContent := ""
	smsUpstreamCode := ""
	if needSMS && s.smsTplRepo != nil {
		tpl, terr := s.smsTplRepo.FindByCode(ctx, req.SmsTemplateCode)
		if terr != nil {
			return nil, ErrSmsTemplateNotFound
		}
		smsContent = tpl.Content
		smsUpstreamCode = tpl.UpstreamCode
	}

	batchID := newBatchID()
	siteName := s.siteName(ctx)
	var queued int64
	err = s.broadcastRepo.StreamTargets(ctx, req.Target, 1000, func(users []notifyrepo.BroadcastUser) error {
		// 逐人渲染（per_user_vars）时变量取用户上下文，投递记录存该用户实际变量，
		// 重投时无需重查。
		rows := make([]notifymodel.NotificationDelivery, 0, len(users)*len(channels))
		for _, u := range users {
			vars := map[string]string{
				"site_name": siteName,
				"username":  u.Username,
			}
			for k, v := range req.Vars {
				vars[k] = v
			}
			title := RenderTemplate(req.Title, vars)
			content := RenderTemplate(req.Content, vars)

			for _, ch := range channels {
				row := notifymodel.NotificationDelivery{
					BatchID:      batchID,
					Event:        "broadcast",
					Channel:      ch,
					TargetType:   notifymodel.TargetUser,
					TargetID:     u.ID,
					TargetName:   u.Username,
					Title:        title,
					Content:      content,
					SendStatus:   notifymodel.DeliveryStatusPending,
					SourceModule: notifymodel.SourceModuleBroadcast,
					SourceID:     fmt.Sprintf("%s:%d", batchID, u.ID),
					Vars:         encodeVars(vars),
				}
				switch ch {
				case notifymodel.ChannelInbox:
					row.ContentFormat = notifymodel.FormatText
				case notifymodel.ChannelMail:
					if u.Email == "" {
						row.SendStatus = notifymodel.DeliveryStatusSkipped
						row.FailReason = "用户未绑定邮箱"
						row.Recipient = ""
					} else {
						row.Recipient = u.Email
						row.ContentFormat = firstNonEmptyString(req.Format, notifymodel.FormatText)
					}
				case notifymodel.ChannelSMS:
					if u.Phone == "" {
						row.SendStatus = notifymodel.DeliveryStatusSkipped
						row.FailReason = "用户未绑定手机号"
						row.Recipient = ""
					} else {
						row.Recipient = u.Phone
						row.Content = RenderTemplate(smsContent, vars)
						row.ContentFormat = notifymodel.FormatText
						if smsUpstreamCode != "" {
							row.Vars = encodeVars(mergeVars(vars, smsUpstreamCode))
						}
					}
				}
				rows = append(rows, row)
			}
		}
		if err := s.deliveryRepo.BulkInsert(ctx, rows); err != nil {
			return err
		}
		// 站内信群发：与单发不同，不走偏好过滤（运营主动触达），
		// 但用户端「通知偏好」里 broadcast 事件的开关只影响红点，不影响可见性。
		if containsString(channels, notifymodel.ChannelInbox) {
			inboxRows := make([]notifymodel.Notification, 0, len(users))
			for _, u := range users {
				vars := map[string]string{"site_name": siteName, "username": u.Username}
				for k, v := range req.Vars {
					vars[k] = v
				}
				inboxRows = append(inboxRows, notifymodel.Notification{
					UserID:        u.ID,
					TargetType:    notifymodel.TargetUser,
					Event:         "broadcast",
					Title:         RenderTemplate(req.Title, vars),
					Content:       RenderTemplate(req.Content, vars),
					Channel:       notifymodel.ChannelInbox,
					SendStatus:    notifymodel.SendStatusSent,
					ContentFormat: notifymodel.FormatText,
					SourceModule:  notifymodel.SourceModuleBroadcast,
					SourceID:      fmt.Sprintf("%s:%d", batchID, u.ID),
				})
			}
			for i := range inboxRows {
				if err := s.notifyRepo.CreateInbox(ctx, &inboxRows[i]); err != nil {
					// 单条失败不中断整批：记录后继续（幂等索引冲突即视为已存在）。
					if !isDuplicateKey(err) {
						s.logger.Warn("broadcast: create inbox failed",
							zap.Uint64("user_id", inboxRows[i].UserID), zap.Error(err))
					}
				}
			}
		}
		queued += int64(len(rows))
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("broadcast: queued",
		zap.String("batch_id", batchID), zap.Int64("targets", total), zap.Int64("rows", queued))
	return &notifydto.BroadcastResponse{
		BatchID: batchID,
		Total:   total,
		Queued:  queued,
		Message: fmt.Sprintf("已提交 %d 条投递，可在发送日志按批次 %s 查看", queued, batchID),
	}, nil
}

func (s *broadcastService) siteName(ctx context.Context) string {
	if s.configReader != nil {
		if v, ok, err := s.configReader(ctx, "site_name"); err == nil && ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "HostSent"
}

// smsUnitPriceFen 短信单价（分/条），配置键 sms_unit_price_fen，默认 5。
func (s *broadcastService) smsUnitPriceFen(ctx context.Context) int64 {
	if s.configReader != nil {
		if v, ok, err := s.configReader(ctx, "sms_unit_price_fen"); err == nil && ok {
			var n int64
			if _, serr := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); serr == nil && n >= 0 {
				return n
			}
		}
	}
	return 5
}

func toBroadcastUserItems(users []notifyrepo.BroadcastUser) []notifydto.BroadcastUserItem {
	out := make([]notifydto.BroadcastUserItem, 0, len(users))
	for _, u := range users {
		out = append(out, notifydto.BroadcastUserItem{
			ID:          u.ID,
			Username:    u.Username,
			EmailMasked: MaskRecipient(notifymodel.ChannelMail, u.Email),
			PhoneMasked: MaskRecipient(notifymodel.ChannelSMS, u.Phone),
		})
	}
	return out
}

// normalizeChannels 去重并剔除非法通道；只允许 inbox / mail / sms。
func normalizeChannels(channels []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(channels))
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		switch ch {
		case notifymodel.ChannelInbox, notifymodel.ChannelMail, notifymodel.ChannelSMS:
			if !seen[ch] {
				seen[ch] = true
				out = append(out, ch)
			}
		}
	}
	return out
}

func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// newBatchID 生成群发批次号：BC + 时间戳 + 4 位随机 hex。
func newBatchID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return notifyrepo.NewBatchID(time.Now().Format("20060102150405"), "0000")
	}
	return notifyrepo.NewBatchID(time.Now().Format("20060102150405"), hex.EncodeToString(b[:]))
}
