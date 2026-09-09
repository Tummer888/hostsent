// Package service 提供上游提供商模块的业务编排。
package service

import (
	"context"
	"fmt"
	"time"

	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	"hostsent/backend/internal/modules/admin/resource/provider/model"
	"hostsent/backend/internal/modules/admin/resource/provider/repository"
	"hostsent/backend/internal/pkg/crypto"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderService 上游提供商业务能力
type ProviderService interface {
	List(ctx context.Context, query dto.ProviderListQuery) (*dto.ProviderListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.ProviderInfo, error)
	Create(ctx context.Context, req dto.ProviderCreateRequest) (*dto.ProviderInfo, error)
	Update(ctx context.Context, id uint64, req dto.ProviderUpdateRequest) (*dto.ProviderInfo, error)
	Delete(ctx context.Context, id uint64) error
	ListTypes(ctx context.Context) []dto.ProviderTypeItem
	TestConnection(ctx context.Context, id uint64) (*dto.TestConnectionResult, error)
	ListPools(ctx context.Context, query dto.PoolListQuery) (*dto.PoolListResponse, error)
	FindPool(ctx context.Context, id uint64) (*dto.PoolInfo, error)
	// BuildProviderConfig 根据提供商 ID 构建适配器配置（密钥解密注入），供同步引擎复用
	BuildProviderConfig(ctx context.Context, id uint64) (*upstream.ProviderConfig, error)
}

type providerService struct {
	repo          repository.ProviderRepository
	pool          repository.PoolRepository
	upmgr         *upstream.ProviderManager
	encryptSecret string // 用于 API 密钥静态加密的密钥
}

// NewProviderService 创建上游提供商业务服务
func NewProviderService(repo repository.ProviderRepository, pool repository.PoolRepository, upmgr *upstream.ProviderManager, encryptSecret string) ProviderService {
	return &providerService{repo: repo, pool: pool, upmgr: upmgr, encryptSecret: encryptSecret}
}

func (s *providerService) List(ctx context.Context, query dto.ProviderListQuery) (*dto.ProviderListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.ProviderInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, s.buildProviderInfo(item))
	}
	return &dto.ProviderListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *providerService) FindByID(ctx context.Context, id uint64) (*dto.ProviderInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := s.buildProviderInfo(*item)
	return &info, nil
}

func (s *providerService) Create(ctx context.Context, req dto.ProviderCreateRequest) (*dto.ProviderInfo, error) {
	status := req.Status
	if status == 0 {
		status = 1
	}
	syncInterval := req.SyncInterval
	if syncInterval <= 0 {
		syncInterval = 3600
	}
	upstreamType := req.UpstreamType
	if req.ProviderType == "mofangfinance" && upstreamType == "" {
		upstreamType = "zjmf_api" // 智简魔方（默认接口类型）
	}
	item := &model.ResourceProvider{
		Name:             req.Name,
		ProviderType:     req.ProviderType,
		APIEndpoint:      req.APIEndpoint,
		APIKey:           s.encryptValue(req.APIKey),
		APISecret:        s.encryptValue(req.APISecret),
		Region:           req.Region,
		ContactWay:       req.ContactWay,
		Des:              req.Des,
		UpstreamType:     upstreamType,
		ZjmfFinanceAPIID: req.ZjmfFinanceAPIID,
		Port:             req.Port,
		Secure:           req.Secure,
		Disabled:         req.Disabled,
		UserPrefix:       req.UserPrefix,
		AccountType:      req.AccountType,
		Status:           status,
		SyncEnabled:      req.SyncEnabled,
		SyncInterval:     syncInterval,
	}
	// 说明：上游对接仅保存连接信息（地址/账号/密码），连通性由 TestConnection 按真实协议
	// 校验（魔方财务走 /zjmf_api_login 登录换 JWT，魔方云走 /v1/login 或 /token 换 access-token）。
	// 此前在此处调用上游 createApi 注册凭证的做法是错误的——那是对方系统的管理端接口，外部无法调用。
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *providerService) Update(ctx context.Context, id uint64, req dto.ProviderUpdateRequest) (*dto.ProviderInfo, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = req.Name
	item.APIEndpoint = req.APIEndpoint
	if req.APIKey != "" && !s.isMaskedEcho(item.APIKey, req.APIKey) {
		item.APIKey = s.encryptValue(req.APIKey)
	}
	if req.APISecret != "" && !s.isMaskedEcho(item.APISecret, req.APISecret) {
		item.APISecret = s.encryptValue(req.APISecret)
	}
	item.Region = req.Region
	item.ContactWay = req.ContactWay
	item.Des = req.Des
	item.UpstreamType = req.UpstreamType
	if req.ZjmfFinanceAPIID > 0 {
		item.ZjmfFinanceAPIID = req.ZjmfFinanceAPIID
	}
	item.Port = req.Port
	item.Secure = req.Secure
	item.Disabled = req.Disabled
	item.UserPrefix = req.UserPrefix
	item.AccountType = req.AccountType
	item.Status = req.Status
	item.SyncEnabled = req.SyncEnabled
	if req.SyncInterval > 0 {
		item.SyncInterval = req.SyncInterval
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, item.ID)
}

func (s *providerService) Delete(ctx context.Context, id uint64) error {
	products, pools, err := s.repo.CountAssociations(ctx, id)
	if err != nil {
		return err
	}
	if products > 0 || pools > 0 {
		return fmt.Errorf("提供商存在 %d 个关联商品、%d 个资源池，请先删除关联数据", products, pools)
	}
	return s.repo.Delete(ctx, id)
}

// providerTypeNames 提供商类型显示名映射；未收录时兜底使用类型标识。
var providerTypeNames = map[string]string{
	"mofangyun":     "魔方云",
	"mofangfinance": "魔方财务",
	"aliyun":        "阿里云",
	"tencent":       "腾讯云",
	"aws":           "AWS",
	"azure":         "Azure",
	"gcp":           "GCP",
}

func (s *providerService) ListTypes(_ context.Context) []dto.ProviderTypeItem {
	items := make([]dto.ProviderTypeItem, 0)
	for _, t := range s.upmgr.ListProviders() {
		name, ok := providerTypeNames[t]
		if !ok {
			name = t
		}
		items = append(items, dto.ProviderTypeItem{Type: t, Name: name})
	}
	return items
}

func (s *providerService) TestConnection(ctx context.Context, id uint64) (*dto.TestConnectionResult, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 从存储配置构建独立适配器实例（密钥解密注入）
	provider, err := s.upmgr.Build(item.ProviderType, s.buildProviderConfig(item))
	if err != nil {
		provider, err = s.upmgr.GetProvider(item.ProviderType)
		if err != nil {
			return &dto.TestConnectionResult{Success: false, Message: err.Error()}, nil
		}
	}
	if err := provider.HealthCheck(ctx); err != nil {
		return &dto.TestConnectionResult{Success: false, Message: err.Error()}, nil
	}
	return &dto.TestConnectionResult{Success: true, Message: "ok"}, nil
}

func (s *providerService) ListPools(ctx context.Context, query dto.PoolListQuery) (*dto.PoolListResponse, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	items, total, err := s.pool.List(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.PoolInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildPoolInfo(item))
	}
	return &dto.PoolListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *providerService) FindPool(ctx context.Context, id uint64) (*dto.PoolInfo, error) {
	item, err := s.pool.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildPoolInfo(*item)
	return &info, nil
}

func (s *providerService) buildProviderInfo(item model.ResourceProvider) dto.ProviderInfo {
	info := dto.ProviderInfo{
		ID:               item.ID,
		Name:             item.Name,
		ProviderType:     item.ProviderType,
		APIEndpoint:      item.APIEndpoint,
		APIKey:           s.maskSecret(item.APIKey),
		APISecret:        s.maskSecret(item.APISecret),
		Region:           item.Region,
		ContactWay:       item.ContactWay,
		Des:              item.Des,
		UpstreamType:     item.UpstreamType,
		ZjmfFinanceAPIID: item.ZjmfFinanceAPIID,
		Port:             item.Port,
		Secure:           item.Secure,
		Disabled:         item.Disabled,
		UserPrefix:       item.UserPrefix,
		AccountType:      item.AccountType,
		Status:           item.Status,
		SyncEnabled:      item.SyncEnabled,
		SyncInterval:     item.SyncInterval,
		TotalCPU:         item.TotalCPU,
		TotalMemory:      item.TotalMemory,
		TotalDisk:        item.TotalDisk,
		UsedCPU:          item.UsedCPU,
		UsedMemory:       item.UsedMemory,
		UsedDisk:         item.UsedDisk,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
	if item.LastSyncAt != nil {
		val := item.LastSyncAt.Format(time.RFC3339)
		info.LastSyncAt = &val
	}
	return info
}

// encryptValue 加密存储在库中的密钥；空值或失败时原样返回（空值保持为空）。
func (s *providerService) encryptValue(v string) string {
	if v == "" {
		return ""
	}
	enc, err := crypto.Encrypt(v, s.encryptSecret)
	if err != nil {
		return v
	}
	return enc
}

// isMaskedEcho 判断更新请求中的密钥是否为脱敏回显值（即用户未修改，保持原值）。
func (s *providerService) isMaskedEcho(storedEncrypted, incoming string) bool {
	plain := s.decryptValue(storedEncrypted)
	return plain != "" && maskSecret(plain) == incoming
}

// maskSecret 解密后脱敏，用于对外输出不回显明文。
func (s *providerService) maskSecret(storedEncrypted string) string {
	plain := s.decryptValue(storedEncrypted)
	return maskSecret(plain)
}

// decryptValue 解密存储值；解密失败（密钥变更或明文历史数据）时回退为原值。
func (s *providerService) decryptValue(stored string) string {
	if stored == "" {
		return ""
	}
	plain, err := crypto.Decrypt(stored, s.encryptSecret)
	if err != nil {
		return stored
	}
	return plain
}

// BuildProviderConfig 根据提供商 ID 构建适配器配置（密钥解密注入），供同步引擎复用。
func (s *providerService) BuildProviderConfig(ctx context.Context, id uint64) (*upstream.ProviderConfig, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.Status != 1 {
		return nil, fmt.Errorf("provider %d 已禁用", id)
	}
	return s.buildProviderConfig(item), nil
}

// buildProviderConfig 由存储记录构建适配器配置（密钥解密注入）。
func (s *providerService) buildProviderConfig(item *model.ResourceProvider) *upstream.ProviderConfig {
	cfg := &upstream.ProviderConfig{
		ID:           uint(item.ID),
		Name:         item.Name,
		Type:         item.ProviderType,
		APIEndpoint:  item.APIEndpoint,
		Region:       item.Region,
		Timeout:      15,
		UpstreamType: item.UpstreamType,
		Port:         item.Port,
		Secure:       item.Secure,
		UserPrefix:   item.UserPrefix,
		AccountType:  item.AccountType,
	}
	if item.APIKey != "" {
		cfg.APIKey = s.decryptValue(item.APIKey)
	}
	if item.APISecret != "" {
		cfg.APISecret = s.decryptValue(item.APISecret)
	}
	return cfg
}

// maskSecret 通用脱敏：仅保留首尾各 2 位，中间替换为 ****。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 4 {
		return "****"
	}
	return string(r[:2]) + "****" + string(r[len(r)-2:])
}

func buildPoolInfo(item model.ResourcePool) dto.PoolInfo {
	info := dto.PoolInfo{
		ID:          item.ID,
		ProviderID:  item.ProviderID,
		UpstreamID:  item.UpstreamID,
		Name:        item.Name,
		PoolType:    item.PoolType,
		TotalCPU:    item.TotalCPU,
		TotalMemory: item.TotalMemory,
		TotalDisk:   item.TotalDisk,
		UsedCPU:     item.UsedCPU,
		UsedMemory:  item.UsedMemory,
		UsedDisk:    item.UsedDisk,
		Status:      item.Status,
	}
	if item.LastSyncAt != nil {
		val := item.LastSyncAt.Format(time.RFC3339)
		info.LastSyncAt = &val
	}
	return info
}
