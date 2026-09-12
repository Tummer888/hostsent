package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/modules/admin/payment/repository"
)

// PreferenceService 用户支付方式偏好与收款账户管理。
type PreferenceService interface {
	// MethodOptions 返回某场景下可用支付方式（按用户偏好排序，标注默认）。
	MethodOptions(ctx context.Context, userID uint64, scene string, amountFen int64) (*dto.MethodOptionsResponse, error)
	Preferences(ctx context.Context, userID uint64) ([]dto.PreferenceItem, error)
	SavePreferences(ctx context.Context, userID uint64, req dto.PreferenceSaveRequest) ([]dto.PreferenceItem, error)

	ListAccounts(ctx context.Context, userID uint64) ([]dto.PayoutAccountInfo, error)
	CreateAccount(ctx context.Context, userID uint64, req dto.PayoutAccountCreateRequest) (*dto.PayoutAccountInfo, error)
	SetDefaultAccount(ctx context.Context, userID, id uint64) error
	// AccountForPayout 取提现打款目标账户（解密后）。
	AccountForPayout(ctx context.Context, userID, id uint64) (*model.UserPayoutAccount, error)
	PreferenceChannels(ctx context.Context, userID uint64, scene string) []string
}

type preferenceService struct {
	prefRepo   repository.PreferenceRepository
	channelSvc ChannelService
	encryptKey string
}

// NewPreferenceService 创建偏好服务。
func NewPreferenceService(prefRepo repository.PreferenceRepository, channelSvc ChannelService, encryptKey string) PreferenceService {
	return &preferenceService{prefRepo: prefRepo, channelSvc: channelSvc, encryptKey: encryptKey}
}

// MethodOptions 返回场景内可用渠道：按用户偏好顺序置顶，其余按平台优先级。
func (s *preferenceService) MethodOptions(ctx context.Context, userID uint64, scene string, amountFen int64) (*dto.MethodOptionsResponse, error) {
	if scene == "" {
		scene = "native"
	}
	list, err := s.channelSvc.List(ctx, dto.ChannelListQuery{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	available := map[string]dto.ChannelInfo{}
	for _, ch := range list.Items {
		if ch.Status != model.StatusEnabled {
			continue
		}
		if len(ch.Scenes) > 0 && !contains(ch.Scenes, scene) {
			continue
		}
		if amountFen > 0 {
			if ch.MinAmountFen > 0 && amountFen < ch.MinAmountFen {
				continue
			}
			if ch.MaxAmountFen > 0 && amountFen > ch.MaxAmountFen {
				continue
			}
		}
		available[ch.ChannelCode] = ch
	}

	ordered := make([]dto.ChannelInfo, 0, len(available))
	defaultCode := ""
	// 用户偏好优先
	if prefItems, perr := s.prefRepo.ListPreferences(ctx, userID); perr == nil {
		for _, p := range prefItems {
			if p.Scene != "" && p.Scene != scene {
				continue
			}
			if ch, ok := available[p.ChannelCode]; ok {
				ordered = append(ordered, ch)
				delete(available, p.ChannelCode)
				if p.IsDefault && defaultCode == "" {
					defaultCode = p.ChannelCode
				}
			}
		}
	}
	// 剩余按平台顺序
	for _, ch := range list.Items {
		if _, ok := available[ch.ChannelCode]; !ok {
			continue
		}
		ordered = append(ordered, ch)
		delete(available, ch.ChannelCode)
		if ch.IsDefault && defaultCode == "" {
			defaultCode = ch.ChannelCode
		}
	}
	if defaultCode == "" && len(ordered) > 0 {
		defaultCode = ordered[0].ChannelCode
	}
	return &dto.MethodOptionsResponse{Scene: scene, Channels: ordered, Default: defaultCode}, nil
}

func (s *preferenceService) Preferences(ctx context.Context, userID uint64) ([]dto.PreferenceItem, error) {
	items, err := s.prefRepo.ListPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PreferenceItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.PreferenceItem{
			Scene: item.Scene, ChannelCode: item.ChannelCode,
			Priority: item.Priority, IsDefault: item.IsDefault,
		})
	}
	return out, nil
}

func (s *preferenceService) SavePreferences(ctx context.Context, userID uint64, req dto.PreferenceSaveRequest) ([]dto.PreferenceItem, error) {
	scene := req.Scene
	items := make([]model.UserPaymentPreference, 0, len(req.Priorities))
	seen := map[string]bool{}
	for i, p := range req.Priorities {
		code := strings.TrimSpace(p.ChannelCode)
		if code == "" || seen[code] {
			continue
		}
		if _, err := s.channelSvc.List(ctx, dto.ChannelListQuery{Keyword: code, Page: 1, PageSize: 100}); err != nil {
			return nil, err
		}
		seen[code] = true
		items = append(items, model.UserPaymentPreference{
			UserID:      userID,
			Scene:       scene,
			ChannelCode: code,
			Priority:    len(req.Priorities) - i,
			IsDefault:   req.Default != "" && code == req.Default,
		})
	}
	if req.Default != "" && !seen[req.Default] {
		items = append(items, model.UserPaymentPreference{
			UserID: userID, Scene: scene, ChannelCode: req.Default, Priority: 0, IsDefault: true,
		})
	}
	if err := s.prefRepo.ReplacePreferences(ctx, userID, items); err != nil {
		return nil, err
	}
	return s.Preferences(ctx, userID)
}

// PreferenceChannels 取用户在某场景的偏好渠道顺序（供渠道路由使用）。
func (s *preferenceService) PreferenceChannels(ctx context.Context, userID uint64, scene string) []string {
	items, err := s.prefRepo.ListPreferences(ctx, userID)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Scene != "" && item.Scene != scene {
			continue
		}
		out = append(out, item.ChannelCode)
	}
	return out
}

func (s *preferenceService) ListAccounts(ctx context.Context, userID uint64) ([]dto.PayoutAccountInfo, error) {
	items, err := s.prefRepo.ListAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PayoutAccountInfo, 0, len(items))
	for _, item := range items {
		out = append(out, buildAccountInfo(item, s.encryptKey))
	}
	return out, nil
}

func (s *preferenceService) CreateAccount(ctx context.Context, userID uint64, req dto.PayoutAccountCreateRequest) (*dto.PayoutAccountInfo, error) {
	channel := strings.TrimSpace(req.Channel)
	if channel != "bank" && channel != "alipay" {
		return nil, errors.New("不支持的收款渠道（仅支持 bank/alipay）")
	}
	enc, err := encryptSecret(req.AccountNo, s.encryptKey)
	if err != nil {
		return nil, err
	}
	a := &model.UserPayoutAccount{
		UserID:      userID,
		Channel:     channel,
		AccountNo:   enc,
		AccountName: req.AccountName,
		BankName:    req.BankName,
		Branch:      req.Branch,
		IsDefault:   req.IsDefault,
	}
	if err := s.prefRepo.CreateAccount(ctx, a); err != nil {
		return nil, err
	}
	if req.IsDefault {
		_ = s.prefRepo.ClearDefaultAccount(ctx, userID, a.ID)
	}
	info := buildAccountInfo(*a, s.encryptKey)
	return &info, nil
}

func (s *preferenceService) SetDefaultAccount(ctx context.Context, userID, id uint64) error {
	a, err := s.prefRepo.FindAccount(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAccountNotFound
		}
		return err
	}
	if a.UserID != userID {
		return ErrAccountNotFound
	}
	a.IsDefault = true
	if err := s.prefRepo.UpdateAccount(ctx, a); err != nil {
		return err
	}
	return s.prefRepo.ClearDefaultAccount(ctx, userID, id)
}

// AccountForPayout 取提现目标账户（解密账号，供打款使用）。
func (s *preferenceService) AccountForPayout(ctx context.Context, userID, id uint64) (*model.UserPayoutAccount, error) {
	a, err := s.prefRepo.FindAccount(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	if a.UserID != userID {
		return nil, ErrAccountNotFound
	}
	plain, derr := decryptSecret(a.AccountNo, s.encryptKey)
	if derr != nil {
		return nil, derr
	}
	a.AccountNo = plain
	return a, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
