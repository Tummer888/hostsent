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
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/notifier"
)

// ChannelService 管理端渠道能力：CRUD + 测试 + 类型注册表同步。
type ChannelService interface {
	ListTypes(ctx context.Context, category string) ([]notifydto.ChannelTypeItem, error)
	List(ctx context.Context, q notifydto.ChannelListQuery) (*notifydto.ChannelListResponse, error)
	Get(ctx context.Context, id uint64) (*notifydto.ChannelInfo, error)
	Create(ctx context.Context, req notifydto.ChannelCreateRequest) (*notifydto.ChannelInfo, error)
	Update(ctx context.Context, id uint64, req notifydto.ChannelUpdateRequest) (*notifydto.ChannelInfo, error)
	UpdateStatus(ctx context.Context, id uint64, status int) error
	Test(ctx context.Context, id uint64, target string) (*notifydto.ChannelTestResponse, error)
	// SyncTypeRegistry 启动时把注册表描述符回写 notification_channel_types（幂等）。
	SyncTypeRegistry(ctx context.Context) error
}

// ChannelResolver 发送侧：按 category + scene 选渠道并返回解密后的配置。
// 与 ChannelService 是同一 struct 的两个接口，避免发送路径依赖管理端类型。
type ChannelResolver interface {
	Resolve(ctx context.Context, category, scene string) (*notifier.ChannelConfig, error)
	// ResolveByID 指定渠道实例（测试发送用）。
	ResolveByID(ctx context.Context, id uint64) (*notifier.ChannelConfig, error)
}

type channelService struct {
	repo       notifyrepo.ChannelRepository
	typeRepo   notifyrepo.ChannelTypeRepository
	encryptKey string
}

// NewChannelService 创建渠道服务。
func NewChannelService(repo notifyrepo.ChannelRepository, typeRepo notifyrepo.ChannelTypeRepository, encryptKey string) ChannelService {
	return &channelService{repo: repo, typeRepo: typeRepo, encryptKey: encryptKey}
}

// NewChannelResolver 创建渠道解析器（与 ChannelService 同一实现，供发送侧使用）。
func NewChannelResolver(repo notifyrepo.ChannelRepository, typeRepo notifyrepo.ChannelTypeRepository, encryptKey string) ChannelResolver {
	return &channelService{repo: repo, typeRepo: typeRepo, encryptKey: encryptKey}
}

// ListTypes 渠道类型列表（注册表为主，落库展示信息叠加）。
func (s *channelService) ListTypes(ctx context.Context, category string) ([]notifydto.ChannelTypeItem, error) {
	stored := map[string]notifymodel.NotificationChannelType{}
	if s.typeRepo != nil {
		if rows, err := s.typeRepo.List(ctx); err == nil {
			for _, r := range rows {
				stored[r.Type] = r
			}
		}
	}
	seen := map[string]bool{}
	items := make([]notifydto.ChannelTypeItem, 0)
	for _, d := range notifier.DescriptorsByCategory(category) {
		item := notifydto.ChannelTypeItem{
			Type: d.Type, Name: d.Name, Category: d.Category, Mode: d.Mode,
			Icon: d.Icon, DocURL: d.DocURL, AdapterVersion: d.AdapterVersion,
			Implemented: d.Implemented, Capabilities: d,
		}
		if row, ok := stored[d.Type]; ok {
			if row.Name != "" {
				item.Name = row.Name
			}
			if row.Icon != "" {
				item.Icon = row.Icon
			}
			if row.DocURL != "" {
				item.DocURL = row.DocURL
			}
			if row.AdapterVersion != "" {
				item.AdapterVersion = row.AdapterVersion
			}
		}
		items = append(items, item)
		seen[d.Type] = true
	}
	// 落库但无描述符的"历史类型"也要能看到（便于清理）。
	for t, row := range stored {
		if seen[t] {
			continue
		}
		if category != "" && row.Category != category {
			continue
		}
		items = append(items, notifydto.ChannelTypeItem{
			Type: row.Type, Name: row.Name, Category: row.Category, Mode: row.Mode,
			Icon: row.Icon, DocURL: row.DocURL, AdapterVersion: row.AdapterVersion,
			Implemented: false, Capabilities: s.storedDescriptor(row),
		})
	}
	return items, nil
}

func (s *channelService) List(ctx context.Context, q notifydto.ChannelListQuery) (*notifydto.ChannelListResponse, error) {
	items, total, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp := make([]notifydto.ChannelInfo, 0, len(items))
	for i := range items {
		resp = append(resp, s.buildChannelInfo(items[i]))
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 10
	}
	return &notifydto.ChannelListResponse{
		Items: resp,
		Meta:  notifydto.ListMeta{Page: page, PageSize: size, Total: total},
	}, nil
}

func (s *channelService) Get(ctx context.Context, id uint64) (*notifydto.ChannelInfo, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	info := s.buildChannelInfo(*ch)
	return &info, nil
}

func (s *channelService) Create(ctx context.Context, req notifydto.ChannelCreateRequest) (*notifydto.ChannelInfo, error) {
	if _, err := s.repo.FindByCode(ctx, req.ChannelCode); err == nil {
		return nil, ErrChannelCodeExists
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	desc, ok := notifier.Descriptor(req.Type)
	if !ok {
		return nil, ErrChannelTypeUnknown
	}
	creds := credentials.Map(req.Credentials)
	if err := notifier.ValidateCredentials(desc, creds); err != nil {
		return nil, err
	}
	enc, err := credentials.EncryptFields(creds, desc.CredentialSchema, s.encryptKey)
	if err != nil {
		return nil, err
	}
	encoded, err := enc.Encode()
	if err != nil {
		return nil, err
	}
	ch := &notifymodel.NotificationChannel{
		ChannelCode:  req.ChannelCode,
		Name:         req.Name,
		Category:     desc.Category,
		Type:         req.Type,
		Credentials:  encoded,
		Endpoint:     req.Endpoint,
		SignName:     req.SignName,
		Sender:       req.Sender,
		TemplateCode: req.TemplateCode,
		Scenes:       encodeScenes(req.Scenes),
		Priority:     req.Priority,
		Weight:       req.Weight,
		DailyLimit:   req.DailyLimit,
		IsDefault:    req.IsDefault,
		Remark:       req.Remark,
		Status:       notifymodel.ChannelStatusEnabled,
	}
	// 同一 category 至多一个默认，这条规则由部分唯一索引
	// uk_notify_channels_default 兜底。但索引是在 INSERT 的那一刻生效的：
	// 必须先把旧默认摘掉再插入新行，否则 INSERT 先撞 23505，后面的 ClearDefault
	// 根本没机会执行 —— 「设第二个默认为默认」永远失败而不是自动切换。
	if req.IsDefault {
		if err := s.repo.ClearDefault(ctx, ch.Category, 0); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	return s.Get(ctx, ch.ID)
}

func (s *channelService) Update(ctx context.Context, id uint64, req notifydto.ChannelUpdateRequest) (*notifydto.ChannelInfo, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	desc, _ := notifier.Descriptor(ch.Type)
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
	ch.SignName = req.SignName
	ch.Sender = req.Sender
	ch.TemplateCode = req.TemplateCode
	if req.Scenes != nil {
		ch.Scenes = encodeScenes(req.Scenes)
	}
	ch.Priority = req.Priority
	ch.Weight = req.Weight
	ch.DailyLimit = req.DailyLimit
	ch.IsDefault = req.IsDefault
	ch.Remark = req.Remark
	// 同 Create：部分唯一索引在 UPDATE 时同样先于后续语句生效，必须先把同
	// category 的旧默认摘掉，否则「把另一个渠道设为默认」直接 23505。
	if req.IsDefault {
		if err := s.repo.ClearDefault(ctx, ch.Category, ch.ID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *channelService) UpdateStatus(ctx context.Context, id uint64, status int) error {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrChannelNotFound
		}
		return err
	}
	ch.Status = status
	if status == notifymodel.ChannelStatusDisabled {
		// 停用即清空健康标记与默认位，避免停用渠道仍被路由命中。
		ch.HealthStatus = ""
		ch.LastError = ""
		if ch.IsDefault {
			ch.IsDefault = false
		}
	}
	return s.repo.Update(ctx, ch)
}

// Test 渠道连通性测试：占位 provider 返回 pending 而非失败。
func (s *channelService) Test(ctx context.Context, id uint64, target string) (*notifydto.ChannelTestResponse, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	cfg, err := s.BuildConfig(ch)
	if err != nil {
		return &notifydto.ChannelTestResponse{OK: false, Message: err.Error()}, nil
	}
	sender, err := notifier.New(ch.Type, *cfg)
	if err != nil {
		// 占位 provider：不是错误，是「配置已保存，适配器待接入」。
		if err == notifier.ErrAdapterNotImplemented {
			_ = s.repo.UpdateHealth(ctx, ch.ID, notifymodel.HealthPending, "该服务商适配器待接入，配置已就绪")
			return &notifydto.ChannelTestResponse{
				OK: false, Pending: true,
				Message: "该服务商适配器待接入，已完成配置占位",
			}, nil
		}
		return &notifydto.ChannelTestResponse{OK: false, Message: err.Error()}, nil
	}
	if strings.TrimSpace(target) == "" {
		return &notifydto.ChannelTestResponse{OK: false, Message: "请填写测试接收目标（邮箱或手机号）"}, nil
	}
	if terr := sender.Test(ctx, *cfg, target); terr != nil {
		if terr == notifier.ErrAdapterNotImplemented {
			_ = s.repo.UpdateHealth(ctx, ch.ID, notifymodel.HealthPending, "该服务商适配器待接入，配置已就绪")
			return &notifydto.ChannelTestResponse{OK: false, Pending: true, Message: "该服务商适配器待接入，已完成配置占位"}, nil
		}
		_ = s.repo.UpdateHealth(ctx, ch.ID, notifymodel.HealthDown, terr.Error())
		return &notifydto.ChannelTestResponse{OK: false, Message: terr.Error()}, nil
	}
	// last_check_at 必须真被更新（支付中心写了但从不更新，这里修正范式）。
	_ = s.repo.UpdateHealth(ctx, ch.ID, notifymodel.HealthHealthy, "")
	return &notifydto.ChannelTestResponse{OK: true, Message: "渠道连通正常"}, nil
}

// Resolve 按 category + scene 选渠道并返回解密配置。
func (s *channelService) Resolve(ctx context.Context, category, scene string) (*notifier.ChannelConfig, error) {
	ch, err := s.repo.PickByCategoryAndScene(ctx, category, scene)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrChannelNotConfigured
		}
		return nil, err
	}
	return s.BuildConfig(ch)
}

// ResolveByID 指定渠道实例。
func (s *channelService) ResolveByID(ctx context.Context, id uint64) (*notifier.ChannelConfig, error) {
	ch, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return s.BuildConfig(ch)
}

// BuildConfig 组装解密后的渠道运行配置。
func (s *channelService) BuildConfig(ch *notifymodel.NotificationChannel) (*notifier.ChannelConfig, error) {
	creds, err := credentials.Decode(ch.Credentials)
	if err != nil {
		return nil, err
	}
	plain, err := creds.DecryptFields(s.encryptKey)
	if err != nil {
		return nil, err
	}
	return &notifier.ChannelConfig{
		ChannelID:    ch.ID,
		ChannelCode:  ch.ChannelCode,
		Type:         ch.Type,
		Category:     ch.Category,
		Endpoint:     ch.Endpoint,
		SignName:     ch.SignName,
		Sender:       ch.Sender,
		TemplateCode: ch.TemplateCode,
		Credentials:  plain,
	}, nil
}

// buildChannelInfo 组装渠道信息（凭证按描述符脱敏，绝不回显明文）。
func (s *channelService) buildChannelInfo(c notifymodel.NotificationChannel) notifydto.ChannelInfo {
	desc, ok := notifier.Descriptor(c.Type)
	if !ok {
		desc = s.storedDescriptorByType(c.Type)
	}
	masked := map[string]string{}
	if creds, err := credentials.Decode(c.Credentials); err == nil {
		masked = creds.Mask(desc.CredentialSchema)
	}
	implemented := notifier.IsRegistered(c.Type)
	return notifydto.ChannelInfo{
		ID:           c.ID,
		ChannelCode:  c.ChannelCode,
		Name:         c.Name,
		Category:     c.Category,
		Type:         c.Type,
		TypeName:     desc.Name,
		Credentials:  masked,
		Endpoint:     c.Endpoint,
		SignName:     c.SignName,
		Sender:       c.Sender,
		TemplateCode: c.TemplateCode,
		Scenes:       decodeScenes(c.Scenes),
		Priority:     c.Priority,
		Weight:       c.Weight,
		DailyLimit:   c.DailyLimit,
		HealthStatus: c.HealthStatus,
		LastError:    c.LastError,
		LastCheckAt:  formatTimePtr(c.LastCheckAt),
		Status:       c.Status,
		IsDefault:    c.IsDefault,
		Remark:       c.Remark,
		Implemented:  implemented,
		Capabilities: desc,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    c.UpdatedAt.Format(time.RFC3339),
	}
}

// storedDescriptor 读取落库描述符（仅类型注册表用）。
func (s *channelService) storedDescriptor(row notifymodel.NotificationChannelType) notifier.CapabilityDescriptor {
	d := s.storedDescriptorByType(row.Type)
	if d.Name == "" {
		d.Name = row.Name
	}
	return d
}

func (s *channelService) storedDescriptorByType(providerType string) notifier.CapabilityDescriptor {
	if s.typeRepo == nil {
		return notifier.CapabilityDescriptor{Type: providerType}
	}
	row, err := s.typeRepo.FindByType(context.Background(), providerType)
	if err != nil || strings.TrimSpace(row.DescriptorJSON) == "" {
		return notifier.CapabilityDescriptor{Type: providerType}
	}
	var d notifier.CapabilityDescriptor
	if jerr := json.Unmarshal([]byte(row.DescriptorJSON), &d); jerr != nil {
		return notifier.CapabilityDescriptor{Type: providerType}
	}
	d.Implemented = notifier.IsRegistered(providerType)
	return d
}

// SyncTypeRegistry 把注册表描述符回写 notification_channel_types（幂等 upsert）。
func (s *channelService) SyncTypeRegistry(ctx context.Context) error {
	if s.typeRepo == nil {
		return nil
	}
	for _, d := range notifier.AllDescriptors() {
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		item := &notifymodel.NotificationChannelType{
			Type:           d.Type,
			Name:           d.Name,
			Category:       d.Category,
			Mode:           d.Mode,
			DescriptorJSON: string(raw),
			Icon:           d.Icon,
			DocURL:         d.DocURL,
			AdapterVersion: d.AdapterVersion,
			Status:         1,
		}
		if d.Implemented {
			item.AdapterVersion = d.AdapterVersion
		}
		if err := s.typeRepo.Upsert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// ---- helpers ----

func encodeScenes(scenes []string) string {
	if len(scenes) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(scenes)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func decodeScenes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
