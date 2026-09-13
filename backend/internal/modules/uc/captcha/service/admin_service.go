package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	"hostsent/backend/internal/modules/uc/captcha/model"
	"hostsent/backend/internal/modules/uc/captcha/repository"
	captchapkg "hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/integration"
)

// AdminService 管理端验证码配置服务（服务商 / 场景策略 / 统计）。
type AdminService struct {
	repo       repository.Repository
	switches   *Switches
	encryptKey string
	store      captchapkg.Store
	logger     *zap.Logger
}

// NewAdminService 创建管理端服务。
func NewAdminService(repo repository.Repository, switches *Switches, encryptKey string, store captchapkg.Store, logger *zap.Logger) *AdminService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AdminService{repo: repo, switches: switches, encryptKey: encryptKey, store: store, logger: logger}
}

// ProviderTypes 返回可选服务商类型（描述符驱动前端动态表单）。
func (s *AdminService) ProviderTypes() []dto.ProviderTypeInfo {
	all := captchapkg.AllDescriptors()
	out := make([]dto.ProviderTypeInfo, 0, len(all))
	for _, d := range all {
		out = append(out, dto.ProviderTypeInfo{
			Type:             d.Type,
			Name:             d.Name,
			Mode:             d.Mode,
			Icon:             d.Icon,
			DocURL:           d.DocURL,
			AdapterVersion:   d.AdapterVersion,
			Implemented:      d.Implemented,
			Builtin:          d.Builtin,
			CredentialSchema: d.CredentialSchema,
		})
	}
	return out
}

// ListProviders 服务商列表（凭证只回脱敏值，绝不返回明文）。
func (s *AdminService) ListProviders(ctx context.Context) ([]dto.ProviderInfo, error) {
	rows, err := s.repo.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProviderInfo, 0, len(rows))
	for i := range rows {
		out = append(out, s.toProviderInfo(&rows[i]))
	}
	return out, nil
}

// toProviderInfo 转换并脱敏凭证。
func (s *AdminService) toProviderInfo(row *model.CaptchaProvider) dto.ProviderInfo {
	info := dto.ProviderInfo{
		ID:             row.ID,
		ProviderType:   row.ProviderType,
		Name:           row.Name,
		Mode:           row.Mode,
		Endpoint:       row.Endpoint,
		Priority:       row.Priority,
		HealthStatus:   row.HealthStatus,
		LastError:      row.LastError,
		Status:         row.Status,
		IsDefault:      row.IsDefault,
		Remark:         row.Remark,
		Credentials:    map[string]string{},
		CredentialKeys: []string{},
		Supported:      captchapkg.IsRegistered(row.ProviderType),
	}
	if row.LastCheckAt != nil {
		info.LastCheckAt = row.LastCheckAt.Format(time.RFC3339)
	}
	if d, ok := captchapkg.Descriptor(row.ProviderType); ok {
		info.Implemented = d.Implemented
		info.Builtin = d.Builtin
	}
	// 凭证回显：secret 字段一律脱敏（credentials.Mask），非密文字段原样。
	values, err := credentials.Decode(row.Credentials)
	if err == nil && len(values) > 0 {
		schema := []integration.Field{}
		if d, ok := captchapkg.Descriptor(row.ProviderType); ok {
			schema = d.CredentialSchema
		}
		masked := values.Mask(schema)
		for k, v := range masked {
			info.Credentials[k] = v
			info.CredentialKeys = append(info.CredentialKeys, k)
		}
	}
	return info
}

// CreateProvider 新建服务商（凭证字段级加密落库）。
func (s *AdminService) CreateProvider(ctx context.Context, req dto.ProviderUpsertRequest) (*dto.ProviderInfo, error) {
	if _, ok := captchapkg.Descriptor(req.ProviderType); !ok {
		return nil, fmt.Errorf("未知的验证码服务商类型：%s", req.ProviderType)
	}
	d, _ := captchapkg.Descriptor(req.ProviderType)
	if err := captchapkg.ValidateCredentials(d, req.Credentials); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptCredentials(d, req.Credentials)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = d.Name
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}
	row := &model.CaptchaProvider{
		ProviderType: req.ProviderType,
		Name:         name,
		Mode:         d.Mode,
		Descriptor:   mustJSON(d),
		Credentials:  encrypted,
		Endpoint:     req.Endpoint,
		Scenes:       mustJSON(req.Scenes),
		Priority:     req.Priority,
		Status:       status,
		IsDefault:    req.IsDefault != nil && *req.IsDefault,
		Remark:       req.Remark,
	}
	if err := s.repo.CreateProvider(ctx, row); err != nil {
		return nil, err
	}
	if row.IsDefault {
		if err := s.repo.ClearDefaultProviders(ctx, row.ID); err != nil {
			s.logger.Warn("clear default captcha providers failed", zap.Error(err))
		}
	}
	info := s.toProviderInfo(row)
	return &info, nil
}

// UpdateProvider 更新服务商。
//
// 凭证语义与支付渠道一致：前端回传脱敏值表示「未修改」，此时保留原密文。
func (s *AdminService) UpdateProvider(ctx context.Context, id uint64, req dto.ProviderUpsertRequest) (*dto.ProviderInfo, error) {
	row, err := s.repo.FindProviderByID(ctx, id)
	if err != nil {
		return nil, errors.New("验证码服务商不存在")
	}
	if row.ProviderType == "native" && req.Status != nil && *req.Status != 1 {
		return nil, errors.New("内置图形验证码不可停用（它是第三方不可用时的兜底）")
	}
	d, _ := captchapkg.Descriptor(row.ProviderType)
	existing, _ := credentials.Decode(row.Credentials)

	merged := map[string]string{}
	for k, v := range req.Credentials {
		// 脱敏回显值原样保留原密文（用户未修改该字段）。
		if credentials.IsMaskedEcho(credentials.MaskSecret(existing[k]), v) && existing[k] != "" {
			merged[k] = existing[k]
			continue
		}
		merged[k] = v
	}
	if err := captchapkg.ValidateCredentials(d, merged); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptCredentials(d, merged)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) != "" {
		row.Name = req.Name
	}
	if req.Endpoint != "" {
		row.Endpoint = req.Endpoint
	}
	if req.Scenes != nil {
		row.Scenes = mustJSON(req.Scenes)
	}
	row.Priority = req.Priority
	row.Credentials = encrypted
	row.Remark = req.Remark
	if req.Status != nil {
		row.Status = *req.Status
	}
	if req.IsDefault != nil {
		row.IsDefault = *req.IsDefault
	}
	if err := s.repo.UpdateProvider(ctx, row); err != nil {
		return nil, err
	}
	if row.IsDefault {
		if err := s.repo.ClearDefaultProviders(ctx, row.ID); err != nil {
			s.logger.Warn("clear default captcha providers failed", zap.Error(err))
		}
	}
	info := s.toProviderInfo(row)
	return &info, nil
}

// DeleteProvider 软删除服务商（native 不可删除）。
func (s *AdminService) DeleteProvider(ctx context.Context, id uint64) error {
	row, err := s.repo.FindProviderByID(ctx, id)
	if err != nil {
		return errors.New("验证码服务商不存在")
	}
	if row.ProviderType == "native" {
		return errors.New("内置图形验证码不可删除（它是第三方不可用时的兜底）")
	}
	return s.repo.DeleteProvider(ctx, id)
}

// TestProvider 测试服务商；占位 provider 返回「待接入」的明确业务错误。
func (s *AdminService) TestProvider(ctx context.Context, id uint64) error {
	row, err := s.repo.FindProviderByID(ctx, id)
	if err != nil {
		return errors.New("验证码服务商不存在")
	}
	d, _ := captchapkg.Descriptor(row.ProviderType)
	creds, err := s.decryptCredentials(row)
	if err != nil {
		return err
	}
	store := s.store
	prov, err := captchapkg.New(row.ProviderType, captchapkg.ProviderConfig{
		ProviderID:  row.ID,
		Type:        row.ProviderType,
		Endpoint:    row.Endpoint,
		Credentials: creds,
		Store:       store,
		Level:       captchapkg.LevelNormal,
	})
	if err != nil {
		// 未注册适配器（占位类型）：标记 pending，返回「待接入」而非 500。
		row.HealthStatus = model.HealthPending
		row.LastError = captchapkg.ErrAdapterNotImplemented.Error()
		now := time.Now()
		row.LastCheckAt = &now
		_ = s.repo.UpdateProvider(ctx, row)
		_ = d
		return captchapkg.ErrAdapterNotImplemented
	}
	if err := prov.Test(ctx, captchapkg.ProviderConfig{
		ProviderID: row.ID, Type: row.ProviderType, Endpoint: row.Endpoint, Credentials: creds,
	}); err != nil {
		row.HealthStatus = model.HealthDown
		row.LastError = err.Error()
		now := time.Now()
		row.LastCheckAt = &now
		_ = s.repo.UpdateProvider(ctx, row)
		return err
	}
	row.HealthStatus = model.HealthHealthy
	row.LastError = ""
	now := time.Now()
	row.LastCheckAt = &now
	return s.repo.UpdateProvider(ctx, row)
}

// ListPolicies 场景策略列表。
func (s *AdminService) ListPolicies(ctx context.Context) ([]dto.PolicyInfo, error) {
	rows, err := s.repo.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PolicyInfo, 0, len(rows))
	for i := range rows {
		out = append(out, toPolicyInfo(&rows[i]))
	}
	return out, nil
}

// UpdatePolicy 更新场景策略。
func (s *AdminService) UpdatePolicy(ctx context.Context, scene string, req dto.PolicyUpdateRequest) (*dto.PolicyInfo, error) {
	row, err := s.repo.FindPolicyByScene(ctx, scene)
	if err != nil {
		return nil, errors.New("场景策略不存在")
	}
	if req.ImageRequired != nil {
		row.ImageRequired = *req.ImageRequired
	}
	if req.ImageProviderID != nil {
		row.ImageProviderID = *req.ImageProviderID
	}
	if req.ImageLevel != nil {
		row.ImageLevel = captchapkg.NormalizeLevel(*req.ImageLevel)
	}
	if req.OTPRequired != nil {
		row.OTPRequired = *req.OTPRequired
	}
	if req.OTPChannel != nil {
		row.OTPChannel = normalizeChannel(*req.OTPChannel)
	}
	if req.MinChannelLevel != nil {
		row.MinChannelLevel = *req.MinChannelLevel
	}
	if req.UserCanTighten != nil {
		row.UserCanTighten = *req.UserCanTighten
	}
	if req.UserCanChooseChannel != nil {
		row.UserCanChooseChannel = *req.UserCanChooseChannel
	}
	if req.MaxAttempts != nil {
		row.MaxAttempts = *req.MaxAttempts
	}
	if req.TTLSeconds != nil {
		row.TTLSeconds = *req.TTLSeconds
	}
	if req.SendIntervalSeconds != nil {
		row.SendIntervalSeconds = *req.SendIntervalSeconds
	}
	if req.DailyLimitPerTarget != nil {
		row.DailyLimitPerTarget = *req.DailyLimitPerTarget
	}
	if req.Status != nil {
		row.Status = *req.Status
	}
	if req.Remark != nil {
		row.Remark = *req.Remark
	}
	if err := s.repo.SavePolicy(ctx, row); err != nil {
		return nil, err
	}
	info := toPolicyInfo(row)
	return &info, nil
}

// Stats 统计（按场景聚合）。
func (s *AdminService) Stats(ctx context.Context, from, to time.Time) (*dto.StatsResponse, error) {
	rows, err := s.repo.StatsByScene(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := dto.StatsResponse{
		From:  from.Format(time.RFC3339),
		To:    to.Format(time.RFC3339),
		Items: make([]dto.SceneStatInfo, 0, len(rows)),
	}
	for _, r := range rows {
		item := dto.SceneStatInfo{
			Scene:    r.Scene,
			Total:    r.Total,
			Used:     r.Used,
			Failed:   r.Failed,
			Expired:  r.Expired,
			CostFen:  r.CostFen,
			CostYuan: float64(r.CostFen) / 100,
		}
		if r.Total > 0 {
			item.PassRate = float64(r.Used) / float64(r.Total)
		}
		out.Items = append(out.Items, item)
	}
	return &out, nil
}

// encryptCredentials 按描述符加密 secret 字段（幂等：已加密原样保留）。
func (s *AdminService) encryptCredentials(d captchapkg.CapabilityDescriptor, values map[string]string) (string, error) {
	m := credentials.Map{}
	for k, v := range values {
		m[k] = v
	}
	encrypted, err := credentials.EncryptFields(m, d.CredentialSchema, s.encryptKey)
	if err != nil {
		return "", err
	}
	return encrypted.Encode()
}

// decryptCredentials 解密凭证（解密失败必须报错，绝不回退明文）。
func (s *AdminService) decryptCredentials(row *model.CaptchaProvider) (map[string]string, error) {
	m, err := credentials.Decode(row.Credentials)
	if err != nil {
		return nil, err
	}
	plain, err := m.DecryptFields(s.encryptKey)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range plain {
		out[k] = v
	}
	return out, nil
}

func toPolicyInfo(row *model.CaptchaPolicy) dto.PolicyInfo {
	return dto.PolicyInfo{
		Scene:                row.Scene,
		Name:                 row.Name,
		ImageRequired:        row.ImageRequired,
		ImageProviderID:      row.ImageProviderID,
		ImageLevel:           row.ImageLevel,
		OTPRequired:          row.OTPRequired,
		OTPChannel:           row.OTPChannel,
		MinChannelLevel:      row.MinChannelLevel,
		UserCanTighten:       row.UserCanTighten,
		UserCanChooseChannel: row.UserCanChooseChannel,
		MaxAttempts:          row.MaxAttempts,
		TTLSeconds:           row.TTLSeconds,
		SendIntervalSeconds:  row.SendIntervalSeconds,
		DailyLimitPerTarget:  row.DailyLimitPerTarget,
		Status:               row.Status,
		Remark:               row.Remark,
		// 图形码或 OTP 开启 = 平台强制：开启后用户无法关闭（前端显示徽标）。
		PlatformForced: row.ImageRequired || row.OTPRequired,
	}
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
