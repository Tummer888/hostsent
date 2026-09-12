package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/payment/dto"
	"hostsent/backend/internal/modules/admin/payment/model"
	"hostsent/backend/internal/modules/admin/payment/repository"
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/payment"
)

// ChannelService 支付渠道管理能力。
type ChannelService interface {
	// ListTypes 渠道类型列表（驱动动态凭证表单与能力矩阵）。
	ListTypes(ctx context.Context) ([]dto.ChannelTypeItem, error)
	List(ctx context.Context, q dto.ChannelListQuery) (*dto.ChannelListResponse, error)
	Get(ctx context.Context, id uint64) (*dto.ChannelInfo, error)
	Create(ctx context.Context, req dto.ChannelCreateRequest) (*dto.ChannelInfo, error)
	Update(ctx context.Context, id uint64, req dto.ChannelUpdateRequest) (*dto.ChannelInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status int) error
	Test(ctx context.Context, id uint64) (*dto.ChannelTestResponse, error)
	// SyncTypeRegistry 把已注册适配器的描述符回写 payment_types（启动时调用，幂等）。
	SyncTypeRegistry(ctx context.Context) error
}

// ChannelResolver 渠道路由：按场景 + 用户偏好 + 优先级挑选渠道实例。
// 由 PaymentOrderService 依赖（最小接口，避免反向依赖）。
type ChannelResolver interface {
	Resolve(ctx context.Context, userID uint64, scene string, amountFen int64, channelCode string) (*model.PaymentChannel, error)
	// BuildConfig 组装解密后的渠道运行配置。
	BuildConfig(ctx context.Context, ch *model.PaymentChannel) (*payment.ChannelConfig, error)
}

type channelService struct {
	repo       repository.ChannelRepository
	typeRepo   repository.TypeRepository
	prefRepo   repository.PreferenceRepository
	encryptKey string
}

// NewChannelService 创建渠道服务。
func NewChannelService(repo repository.ChannelRepository, typeRepo repository.TypeRepository, prefRepo repository.PreferenceRepository, encryptKey string) ChannelService {
	return &channelService{repo: repo, typeRepo: typeRepo, prefRepo: prefRepo, encryptKey: encryptKey}
}

// NewChannelResolver 创建渠道路由解析器（与 ChannelService 同一实现，供支付单服务使用）。
func NewChannelResolver(repo repository.ChannelRepository, typeRepo repository.TypeRepository, prefRepo repository.PreferenceRepository, encryptKey string) ChannelResolver {
	return &channelService{repo: repo, typeRepo: typeRepo, prefRepo: prefRepo, encryptKey: encryptKey}
}

func (s *channelService) ListTypes(ctx context.Context) ([]dto.ChannelTypeItem, error) {
	// 以注册表为准，叠加落库的展示信息（图标/文档/名称）。
	items := make([]dto.ChannelTypeItem, 0)
	stored := map[string]model.PaymentType{}
	if s.typeRepo != nil {
		if rows, err := s.typeRepo.List(ctx); err == nil {
			for _, r := range rows {
				stored[r.Type] = r
			}
		}
	}
	seen := map[string]bool{}
	for _, t := range payment.RegisteredDescriptorTypes() {
		d, _ := payment.Descriptor(t)
		d.Implemented = true
		name := TypeName(t)
		item := dto.ChannelTypeItem{Type: t, Name: name, Mode: d.Mode, Implemented: true, AdapterVersion: "1.0.0", Capabilities: d}
		if row, ok := stored[t]; ok {
			if row.Name != "" {
				item.Name = row.Name
			}
			item.Icon = row.Icon
			item.DocURL = row.DocURL
			if row.AdapterVersion != "" {
				item.AdapterVersion = row.AdapterVersion
			}
		}
		items = append(items, item)
		seen[t] = true
	}
	// 落库但暂无适配器的"假渠道"也要展示（便于预配置）。
	for t, row := range stored {
		if seen[t] {
			continue
		}
		d := s.descriptorFor(t)
		d.Implemented = false
		items = append(items, dto.ChannelTypeItem{
			Type: t, Name: row.Name, Mode: d.Mode, Implemented: false,
			AdapterVersion: row.AdapterVersion, Icon: row.Icon, DocURL: row.DocURL, Capabilities: d,
		})
	}
	return items, nil
}

func (s *channelService) List(ctx context.Context, q dto.ChannelListQuery) (*dto.ChannelListResponse, error) {
	items, total, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.ChannelInfo, 0, len(items))
	for _, item := range items {
		resp = append(resp, s.buildChannelInfo(item))
	}
	return &dto.ChannelListResponse{
		Items: resp,
		Meta:  dto.ListMeta{Page: normalizePage(q.Page), PageSize: normalizePageSize(q.PageSize), Total: total},
	}, nil
}

func (s *channelService) Get(ctx context.Context, id uint64) (*dto.ChannelInfo, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	info := s.buildChannelInfo(*ch)
	return &info, nil
}

func (s *channelService) Create(ctx context.Context, req dto.ChannelCreateRequest) (*dto.ChannelInfo, error) {
	if _, err := s.repo.FindByCode(ctx, req.ChannelCode); err == nil {
		return nil, ErrChannelCodeExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	desc := s.descriptorFor(req.Type)
	if !payment.IsRegistered(req.Type) {
		return nil, ErrChannelTypeUnknown
	}
	creds, err := credentials.EncryptFields(credentials.Map(req.Credentials), desc.CredentialSchema, s.encryptKey)
	if err != nil {
		return nil, err
	}
	encoded, err := creds.Encode()
	if err != nil {
		return nil, err
	}
	environment := req.Environment
	if environment == "" {
		environment = "prod"
	}
	ch := &model.PaymentChannel{
		ChannelCode:  req.ChannelCode,
		Name:         req.Name,
		Type:         req.Type,
		Credentials:  encoded,
		Endpoint:     req.Endpoint,
		NotifyURL:    req.NotifyURL,
		ReturnURL:    req.ReturnURL,
		Scenes:       encodeScenes(req.Scenes),
		Priority:     req.Priority,
		Weight:       req.Weight,
		FeeRate:      req.FeeRate,
		SettleMode:   req.SettleMode,
		MinAmountFen: req.MinAmountFen,
		MaxAmountFen: req.MaxAmountFen,
		Environment:  environment,
		IsDefault:    req.IsDefault,
		Remark:       req.Remark,
		Status:       model.StatusEnabled,
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	if req.IsDefault {
		_ = s.repo.ClearDefault(ctx, ch.ID)
	}
	return s.Get(ctx, ch.ID)
}

func (s *channelService) Update(ctx context.Context, id uint64, req dto.ChannelUpdateRequest) (*dto.ChannelInfo, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	desc := s.descriptorFor(ch.Type)
	if req.Name != "" {
		ch.Name = req.Name
	}
	if len(req.Credentials) > 0 {
		// 脱敏回显值跳过（未修改），其余重新加密。
		stored, _ := credentials.Decode(ch.Credentials)
		merged := credentials.Map{}
		for k, v := range stored {
			merged[k] = v
		}
		masked := stored.Mask(desc.CredentialSchema)
		for k, v := range req.Credentials {
			if credentials.IsMaskedEcho(masked[k], v) {
				continue
			}
			merged[k] = v
		}
		enc, eerr := credentials.EncryptFields(merged, desc.CredentialSchema, s.encryptKey)
		if eerr != nil {
			return nil, eerr
		}
		encoded, eerr := enc.Encode()
		if eerr != nil {
			return nil, eerr
		}
		ch.Credentials = encoded
	}
	ch.Endpoint = req.Endpoint
	ch.NotifyURL = req.NotifyURL
	ch.ReturnURL = req.ReturnURL
	if req.Scenes != nil {
		ch.Scenes = encodeScenes(req.Scenes)
	}
	ch.Priority = req.Priority
	ch.Weight = req.Weight
	ch.FeeRate = req.FeeRate
	ch.SettleMode = req.SettleMode
	ch.MinAmountFen = req.MinAmountFen
	ch.MaxAmountFen = req.MaxAmountFen
	if req.Environment != "" {
		ch.Environment = req.Environment
	}
	ch.IsDefault = req.IsDefault
	ch.Remark = req.Remark
	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	if req.IsDefault {
		_ = s.repo.ClearDefault(ctx, ch.ID)
	}
	return s.Get(ctx, id)
}

func (s *channelService) UpdateStatus(ctx context.Context, id uint64, status int) error {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return err
	}
	ch.Status = status
	ch.HealthStatus = ""
	ch.LastError = ""
	return s.repo.Update(ctx, ch)
}

func (s *channelService) Test(ctx context.Context, id uint64) (*dto.ChannelTestResponse, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	cfg, err := s.BuildConfig(ctx, ch)
	if err != nil {
		return &dto.ChannelTestResponse{OK: false, Message: err.Error()}, nil
	}
	gw, err := payment.Build(ch.Type, cfg)
	if err != nil {
		return &dto.ChannelTestResponse{OK: false, Message: err.Error()}, nil
	}
	if err := gw.HealthCheck(ctx); err != nil {
		ch.HealthStatus = "down"
		ch.LastError = err.Error()
		_ = s.repo.Update(ctx, ch)
		return &dto.ChannelTestResponse{OK: false, Message: err.Error()}, nil
	}
	ch.HealthStatus = "healthy"
	ch.LastError = ""
	_ = s.repo.Update(ctx, ch)
	return &dto.ChannelTestResponse{OK: true, Message: "渠道连通正常"}, nil
}

// BuildConfig 组装解密后的渠道运行配置。
func (s *channelService) BuildConfig(ctx context.Context, ch *model.PaymentChannel) (*payment.ChannelConfig, error) {
	creds, err := credentials.Decode(ch.Credentials)
	if err != nil {
		return nil, err
	}
	plain, err := creds.DecryptFields(s.encryptKey)
	if err != nil {
		return nil, err
	}
	return &payment.ChannelConfig{
		ID:          ch.ID,
		Code:        ch.ChannelCode,
		Name:        ch.Name,
		Type:        ch.Type,
		Endpoint:    ch.Endpoint,
		NotifyURL:   ch.NotifyURL,
		ReturnURL:   ch.ReturnURL,
		Credentials: payment.CredentialMap(plain),
	}, nil
}

// Resolve 按场景 + 用户偏好 + 优先级挑选渠道实例。
//   - 指定 channelCode 时直接校验可用性；
//   - 否则按用户在该场景的偏好顺序优先，再按 priority/weight/id 回退；
//   - 金额须落在渠道限额内。
func (s *channelService) Resolve(ctx context.Context, userID uint64, scene string, amountFen int64, channelCode string) (*model.PaymentChannel, error) {
	if channelCode != "" {
		ch, err := s.repo.FindByCode(ctx, channelCode)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrChannelNotFound
			}
			return nil, err
		}
		if ch.Status != model.StatusEnabled {
			return nil, payment.ErrChannelDisabled
		}
		if err := checkSceneAndAmount(ch, scene, amountFen); err != nil {
			return nil, err
		}
		return ch, nil
	}

	enabled, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	byCode := map[string]model.PaymentChannel{}
	for _, c := range enabled {
		byCode[c.ChannelCode] = c
	}
	// 用户偏好优先
	if s.prefRepo != nil && userID > 0 {
		if prefs, perr := s.prefRepo.ListPreferences(ctx, userID); perr == nil {
			for _, p := range prefs {
				if p.Scene != "" && p.Scene != scene {
					continue
				}
				if c, ok := byCode[p.ChannelCode]; ok {
					if checkSceneAndAmount(&c, scene, amountFen) == nil {
						return &c, nil
					}
				}
			}
		}
	}
	// 平台顺序回退（ListEnabled 已按 priority/weight 排序）
	for i := range enabled {
		c := enabled[i]
		if checkSceneAndAmount(&c, scene, amountFen) == nil {
			return &c, nil
		}
	}
	return nil, ErrNoChannelAvailable
}

// checkSceneAndAmount 校验渠道场景与金额限额。
func checkSceneAndAmount(ch *model.PaymentChannel, scene string, amountFen int64) error {
	if scene != "" {
		scenes := decodeScenes(ch.Scenes)
		if len(scenes) > 0 {
			matched := false
			for _, sc := range scenes {
				if sc == scene {
					matched = true
					break
				}
			}
			if !matched {
				return payment.ErrUnsupportedScene
			}
		}
	}
	if ch.MinAmountFen > 0 && amountFen < ch.MinAmountFen {
		return payment.ErrAmountOutOfRange
	}
	if ch.MaxAmountFen > 0 && amountFen > ch.MaxAmountFen {
		return payment.ErrAmountOutOfRange
	}
	return nil
}

// SyncTypeRegistry 把已注册适配器的描述符回写 payment_types（幂等）。
func (s *channelService) SyncTypeRegistry(ctx context.Context) error {
	if s.typeRepo == nil {
		return nil
	}
	for _, t := range payment.RegisteredDescriptorTypes() {
		d, ok := payment.Descriptor(t)
		if !ok {
			continue
		}
		name := TypeName(t)
		if row, err := s.typeRepo.FindByType(ctx, t); err == nil && strings.TrimSpace(row.Name) != "" {
			name = row.Name
		}
		raw, err := jsonMarshal(d)
		if err != nil {
			return err
		}
		item := &model.PaymentType{
			Type:           t,
			Name:           name,
			Mode:           d.Mode,
			DescriptorJSON: raw,
			AdapterVersion: "1.0.0",
			Status:         1,
		}
		if err := s.typeRepo.Upsert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
