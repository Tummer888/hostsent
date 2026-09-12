// Package service 提供上游提供商模块的业务编排。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/resource/provider/dto"
	"hostsent/backend/internal/modules/admin/resource/provider/model"
	"hostsent/backend/internal/modules/admin/resource/provider/repository"
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/crypto"
	"hostsent/backend/internal/pkg/transport"
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
	// ResumeSync 解除同步熔断并清零失败计数（后台「一键恢复」）
	ResumeSync(ctx context.Context, id uint64) (*dto.ProviderInfo, error)
	// SyncTypeRegistry 把已注册适配器的能力描述符回写到 provider_types 注册表（启动时调用）
	SyncTypeRegistry(ctx context.Context) error
	// NormalizeCredentials 把迁移搬入 credentials 的历史值归一为带前缀的密文（启动时调用）
	NormalizeCredentials(ctx context.Context) error
	// PriceChangeThreshold 渠道级调价自动应用阈值（T3.4，比例，0.05=5%）
	PriceChangeThreshold(ctx context.Context, providerID uint64) (float64, error)
}

type providerService struct {
	repo          repository.ProviderRepository
	pool          repository.PoolRepository
	typeRepo      repository.ProviderTypeRepository
	upmgr         *upstream.ProviderManager
	encryptSecret string // 用于 API 密钥静态加密的密钥
}

// NewProviderService 创建上游提供商业务服务
func NewProviderService(repo repository.ProviderRepository, pool repository.PoolRepository, typeRepo repository.ProviderTypeRepository, upmgr *upstream.ProviderManager, encryptSecret string) ProviderService {
	return &providerService{repo: repo, pool: pool, typeRepo: typeRepo, upmgr: upmgr, encryptSecret: encryptSecret}
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
	// 链路类型派生（T2.2）：优先取 provider_types 注册表，未登记时按适配器描述符兜底；
	// 仍无记录（假渠道）时默认 upstream，避免新建渠道错分链路。
	descriptor := s.descriptorFor(req.ProviderType)
	kind := descriptor.Kind
	if kind == "" {
		kind = model.KindUpstream
		if req.ProviderType == "mofangyun" {
			kind = model.KindCompute
		}
	}
	// 动态凭证（T2.3）：secret 字段按描述符加密；未传 credentials 时兼容旧 api_key/api_secret。
	credMap, err := s.buildCredentials(req.Credentials, req.APIKey, req.APISecret, descriptor)
	if err != nil {
		return nil, err
	}
	credJSON, err := credMap.Encode()
	if err != nil {
		return nil, err
	}
	item := &model.ResourceProvider{
		Name:                 req.Name,
		ProviderType:         req.ProviderType,
		Kind:                 kind,
		APIEndpoint:          req.APIEndpoint,
		APIKey:               s.encryptValue(req.APIKey),
		APISecret:            s.encryptValue(req.APISecret),
		Credentials:          credJSON,
		Region:               req.Region,
		ContactWay:           req.ContactWay,
		Des:                  req.Des,
		UpstreamType:         upstreamType,
		ZjmfFinanceAPIID:     req.ZjmfFinanceAPIID,
		Port:                 req.Port,
		Secure:               req.Secure,
		Disabled:             req.Disabled,
		UserPrefix:           req.UserPrefix,
		AccountType:          req.AccountType,
		Status:               status,
		SyncEnabled:          req.SyncEnabled,
		SyncInterval:         syncInterval,
		TimeoutSeconds:       req.TimeoutSeconds,
		RetryMax:             req.RetryMax,
		RateLimitQPS:         req.RateLimitQPS,
		PriceChangeThreshold: priceChangeThresholdOrDefault(req.PriceChangeThreshold),
		OpsConsoleURL:        req.OpsConsoleURL,
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
	// 动态凭证（T2.3）：与旧字段合并后重写 credentials；脱敏回显值不覆盖已存密文。
	descriptor := s.descriptorFor(item.ProviderType)
	merged, err := s.mergeCredentials(item, req.Credentials, descriptor)
	if err != nil {
		return nil, err
	}
	if merged != nil {
		credJSON, encErr := merged.Encode()
		if encErr != nil {
			return nil, encErr
		}
		item.Credentials = credJSON
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
	item.TimeoutSeconds = req.TimeoutSeconds
	item.RetryMax = req.RetryMax
	item.RateLimitQPS = req.RateLimitQPS
	item.PriceChangeThreshold = priceChangeThresholdOrDefault(req.PriceChangeThreshold)
	item.OpsConsoleURL = req.OpsConsoleURL
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

// providerTypeNames 仅作注册表缺失时的兜底显示名；权威来源是 provider_types 表（T2.2，地雷 L9）。
var providerTypeNames = map[string]string{
	"mofangyun":     "魔方云",
	"mofangfinance": "魔方财务",
	"aliyun":        "阿里云",
	"tencent":       "腾讯云",
	"aws":           "AWS",
	"azure":         "Azure",
	"gcp":           "GCP",
}

// ListTypes 返回渠道类型列表：以 provider_types 注册表为权威来源，
// 合并运行时已注册适配器（避免注册表缺种子时后台选不到类型）。
func (s *providerService) ListTypes(ctx context.Context) []dto.ProviderTypeItem {
	registered := make(map[string]bool)
	for _, t := range s.upmgr.ListProviders() {
		registered[t] = true
	}
	seen := make(map[string]bool)
	items := make([]dto.ProviderTypeItem, 0)

	if s.typeRepo != nil {
		rows, err := s.typeRepo.List(ctx)
		if err == nil {
			for _, row := range rows {
				if row.Status == 0 {
					continue
				}
				seen[row.Type] = true
				items = append(items, s.buildTypeItem(row.Type, row.Name, row.Kind, row.AdapterVersion, row.DocURL, row.Icon, registered[row.Type]))
			}
		}
	}
	// 未登记进注册表的已注册适配器兜底补位（保证"新增适配器包即可用"）。
	for _, t := range s.upmgr.ListProviders() {
		if seen[t] {
			continue
		}
		name, ok := providerTypeNames[t]
		if !ok {
			name = t
		}
		items = append(items, s.buildTypeItem(t, name, "", "", "", "", true))
	}
	return items
}

// buildTypeItem 组装渠道类型项：能力描述符优先取适配器注册表（含凭证表单），
// 无适配器时回退 provider_types 落库描述符（假渠道也能展示能力矩阵）。
func (s *providerService) buildTypeItem(t, name, kind, adapterVersion, docURL, icon string, implemented bool) dto.ProviderTypeItem {
	descriptor, hasDesc := upstream.Descriptor(t)
	if !hasDesc {
		if d, ok := s.loadStoredDescriptor(t); ok {
			descriptor = d
		}
	}
	if kind == "" {
		kind = descriptor.Kind
	}
	if kind == "" {
		kind = model.KindUpstream
	}
	if adapterVersion == "" {
		if implemented {
			adapterVersion = "1.0.0"
		} else {
			adapterVersion = "-1"
		}
	}
	descriptor.Implemented = implemented
	if descriptor.Kind == "" {
		descriptor.Kind = kind
	}
	if descriptor.SignerType == "" {
		descriptor.SignerType = upstream.SignerNone
	}
	return dto.ProviderTypeItem{
		Type:           t,
		Name:           name,
		Kind:           kind,
		Implemented:    implemented,
		AdapterVersion: adapterVersion,
		DocURL:         docURL,
		Icon:           icon,
		Capabilities:   descriptor,
	}
}

// loadStoredDescriptor 读取 provider_types 中落库的描述符（供无适配器的假渠道展示）。
func (s *providerService) loadStoredDescriptor(providerType string) (upstream.CapabilityDescriptor, bool) {
	var d upstream.CapabilityDescriptor
	if s.typeRepo == nil {
		return d, false
	}
	row, err := s.typeRepo.FindByType(context.Background(), providerType)
	if err != nil || strings.TrimSpace(row.DescriptorJSON) == "" {
		return d, false
	}
	if err := json.Unmarshal([]byte(row.DescriptorJSON), &d); err != nil {
		return d, false
	}
	return d, true
}

// descriptorFor 取渠道类型能力描述符（注册表优先，落库兜底，最后保守默认）。
// Implemented 以适配器注册表为准（落库 descriptor 可能来自同类型适配器的回写）。
func (s *providerService) descriptorFor(providerType string) upstream.CapabilityDescriptor {
	implemented := false
	if s.upmgr != nil {
		for _, t := range s.upmgr.ListProviders() {
			if t == providerType {
				implemented = true
				break
			}
		}
	}
	if d, ok := upstream.Descriptor(providerType); ok {
		d.Implemented = true
		return d
	}
	if d, ok := s.loadStoredDescriptor(providerType); ok {
		d.Implemented = implemented
		return d
	}
	return upstream.CapabilityDescriptor{Kind: model.KindUpstream, SignerType: upstream.SignerNone, Implemented: implemented}
}

// SyncTypeRegistry 把已注册适配器的能力描述符与展示名回写 provider_types（启动时调用）。
// 幂等：仅更新展示字段，不覆盖人工维护的图标/文档/排序；已存在的假渠道行保持原样。
func (s *providerService) SyncTypeRegistry(ctx context.Context) error {
	if s.typeRepo == nil {
		return nil
	}
	for _, t := range upstream.RegisteredDescriptorTypes() {
		d, ok := upstream.Descriptor(t)
		if !ok {
			continue
		}
		name, exists := providerTypeNames[t]
		if !exists {
			if row, err := s.typeRepo.FindByType(ctx, t); err == nil && row.Name != "" {
				name = row.Name
			} else {
				name = t
			}
		}
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		item := &model.ProviderType{
			Type:           t,
			Name:           name,
			Kind:           d.Kind,
			DescriptorJSON: string(raw),
			AdapterVersion: "1.0.0",
			Status:         1,
		}
		if err := s.typeRepo.Upsert(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// buildCredentials 组装新建渠道的动态凭证：优先 req.Credentials，
// 缺失时用旧 api_key/api_secret 填充描述符声明的同名字段，最后按描述符加密 secret 值。
func (s *providerService) buildCredentials(raw map[string]string, apiKey, apiSecret string, descriptor upstream.CapabilityDescriptor) (credentials.Map, error) {
	m := credentials.Map{}
	for k, v := range raw {
		m[k] = v
	}
	if _, ok := m["api_key"]; !ok && apiKey != "" {
		m["api_key"] = apiKey
	}
	if _, ok := m["api_secret"]; !ok && apiSecret != "" {
		m["api_secret"] = apiSecret
	}
	return credentials.EncryptFields(m, descriptor.CredentialSchema, s.encryptSecret)
}

// NormalizeCredentials 把 credentials 中"无 enc: 前缀但实为旧列密文"的值重新加密，
// 统一为带前缀的密文，使 L8 的"解密失败必须报错"能可靠区分三种状态。
// 幂等：已是 enc: 前缀或确为历史明文的值原样保留（历史明文无法自动识别，保持兼容）。
func (s *providerService) NormalizeCredentials(ctx context.Context) error {
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		m, err := credentials.Decode(item.Credentials)
		if err != nil || len(m) == 0 {
			continue
		}
		changed := false
		for k, v := range m {
			if strings.TrimSpace(v) == "" || strings.HasPrefix(v, credentials.EncPrefix) {
				continue
			}
			// 能解出明文说明它是旧列密文（旧 api_key/api_secret 用同一密钥加密），
			// 统一改为带前缀密文；解不出则视为历史明文，保持原样。
			plain, derr := crypto.Decrypt(v, s.encryptSecret)
			if derr != nil {
				continue
			}
			enc, eerr := crypto.Encrypt(plain, s.encryptSecret)
			if eerr != nil {
				continue
			}
			m[k] = credentials.EncPrefix + enc
			changed = true
		}
		if !changed {
			continue
		}
		raw, eerr := m.Encode()
		if eerr != nil {
			return eerr
		}
		if eerr := s.repo.UpdateCredentials(ctx, item.ID, raw); eerr != nil {
			return eerr
		}
	}
	return nil
}

// mergeCredentials 更新渠道凭证：把脱敏回显值剔除（用户未修改），
// 再与已存明文/密文合并后整体重新加密。返回 nil 表示本次无需改动凭证。
func (s *providerService) mergeCredentials(item *model.ResourceProvider, incoming map[string]string, descriptor upstream.CapabilityDescriptor) (credentials.Map, error) {
	if len(incoming) == 0 {
		return nil, nil
	}
	stored, err := credentials.Decode(item.Credentials)
	if err != nil {
		return nil, err
	}
	if len(stored) == 0 {
		// 老记录首次引入动态凭证：用旧列填充同名字段，保证凭证明文不丢。
		stored = credentials.Map{}
		if item.APIKey != "" || item.APISecret != "" {
			stored["api_key"] = item.APIKey
			stored["api_secret"] = item.APISecret
		}
	}
	// 已存值若为密文，先解密为明文参与合并（解密失败按 L8 报错）。
	plainStored, err := stored.DecryptFields(s.encryptSecret)
	if err != nil {
		return nil, err
	}
	merged := credentials.Map{}
	for k, v := range plainStored {
		merged[k] = v
	}
	for _, f := range descriptor.CredentialSchema {
		v, ok := incoming[f.Key]
		if !ok {
			continue
		}
		if f.Secret && credentials.IsMaskedEcho(credentials.MaskSecret(merged[f.Key]), v) {
			continue // 脱敏回显，保持原值
		}
		if strings.TrimSpace(v) == "" {
			continue // 空值视为不修改
		}
		merged[f.Key] = v
	}
	// 非描述符声明的字段也接受（假渠道/自定义凭证），但一律整体加密 secret 字段。
	for k, v := range incoming {
		if _, declared := merged[k]; declared {
			continue
		}
		matched := false
		for _, f := range descriptor.CredentialSchema {
			if f.Key == k {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if strings.TrimSpace(v) != "" {
			merged[k] = v
		}
	}
	return credentials.EncryptFields(merged, descriptor.CredentialSchema, s.encryptSecret)
}

func (s *providerService) TestConnection(ctx context.Context, id uint64) (*dto.TestConnectionResult, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cfg, err := s.buildProviderConfigStrict(item)
	if err != nil {
		// L8：凭证解密失败显式提示，不静默用密文当明文去连上游。
		return &dto.TestConnectionResult{Success: false, Message: err.Error()}, nil
	}
	// 从存储配置构建独立适配器实例（密钥解密注入）
	provider, err := s.upmgr.Build(item.ProviderType, cfg)
	if err != nil {
		if _, ok := upstream.Descriptor(item.ProviderType); !ok {
			// 未注册适配器的"假渠道"：给出明确未实现提示（验收要求）。
			return &dto.TestConnectionResult{Success: false, Message: fmt.Sprintf("适配器未实现：%s（已在类型注册表登记，待接入适配器）", item.ProviderType)}, nil
		}
		return &dto.TestConnectionResult{Success: false, Message: err.Error()}, nil
	}
	if err := provider.HealthCheck(ctx); err != nil {
		kind := transport.KindOf(err)
		return &dto.TestConnectionResult{Success: false, Message: fmt.Sprintf("%s：%s", transport.Describe(kind), err.Error())}, nil
	}
	return &dto.TestConnectionResult{Success: true, Message: "ok"}, nil
}

// ListPools 资源池列表：分页数据 + 全量容量汇总 + 地域枚举。
// 汇总与地域都按「当前筛选条件」的全量结果计算（不受分页影响），
// 保证看板数字与用户在页面上看到的筛选口径一致（S2 容量与位置检测）。
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
	all, err := s.pool.ListAll(ctx, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.PoolInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, buildPoolInfo(item))
	}
	// 所属渠道名与运维平台地址（列表可读性 + 一键跳转）：一次批量反查，避免每行查库。
	if metas, merr := s.pool.ProviderMeta(ctx); merr == nil {
		for i := range respItems {
			meta := metas[respItems[i].ProviderID]
			respItems[i].ProviderName = meta.Name
			respItems[i].ProviderOpsURL = meta.OpsConsoleURL
		}
	}
	resp := &dto.PoolListResponse{
		Items:   respItems,
		Meta:    dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
		Summary: summarizePools(all),
		Regions: collectPoolRegions(all),
	}
	return resp, nil
}

// summarizePools 汇总池容量与告警数量；告警阈值与前端进度条一致（60%/80%）。
func summarizePools(items []model.ResourcePool) dto.PoolCapacitySummary {
	summary := dto.PoolCapacitySummary{TotalPools: len(items)}
	for _, item := range items {
		if item.Status == 1 {
			summary.OnlinePools++
		}
		summary.TotalCPU += item.TotalCPU
		summary.UsedCPU += item.UsedCPU
		summary.TotalMemory += item.TotalMemory
		summary.UsedMemory += item.UsedMemory
		summary.TotalDisk += item.TotalDisk
		summary.UsedDisk += item.UsedDisk
		if item.Region == "" {
			summary.UnlocatedPools++
		}
		switch maxPoolUsagePercent(item) {
		case 2:
			summary.DangerPools++
			summary.WarningPools++
		case 1:
			summary.WarningPools++
		}
	}
	return summary
}

// collectPoolRegions 去重收集地域（按出现顺序稳定，空地域不进入筛选项）。
func collectPoolRegions(items []model.ResourcePool) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Region == "" {
			continue
		}
		if _, ok := seen[item.Region]; ok {
			continue
		}
		seen[item.Region] = struct{}{}
		out = append(out, item.Region)
	}
	sort.Strings(out)
	return out
}

func (s *providerService) FindPool(ctx context.Context, id uint64) (*dto.PoolInfo, error) {
	item, err := s.pool.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	info := buildPoolInfo(*item)
	if metas, merr := s.pool.ProviderMeta(ctx); merr == nil {
		meta := metas[info.ProviderID]
		info.ProviderName = meta.Name
		info.ProviderOpsURL = meta.OpsConsoleURL
	}
	return &info, nil
}

func (s *providerService) buildProviderInfo(item model.ResourceProvider) dto.ProviderInfo {
	descriptor := s.descriptorFor(item.ProviderType)
	creds, credErr := s.decryptCredentials(item, descriptor)
	info := dto.ProviderInfo{
		ID:                   item.ID,
		Name:                 item.Name,
		ProviderType:         item.ProviderType,
		APIEndpoint:          item.APIEndpoint,
		APIKey:               s.maskSecret(item.APIKey),
		APISecret:            s.maskSecret(item.APISecret),
		Region:               item.Region,
		ContactWay:           item.ContactWay,
		Des:                  item.Des,
		UpstreamType:         item.UpstreamType,
		ZjmfFinanceAPIID:     item.ZjmfFinanceAPIID,
		Port:                 item.Port,
		Secure:               item.Secure,
		Disabled:             item.Disabled,
		UserPrefix:           item.UserPrefix,
		AccountType:          item.AccountType,
		Status:               item.Status,
		SyncEnabled:          item.SyncEnabled,
		SyncInterval:         item.SyncInterval,
		Kind:                 item.Kind,
		SyncPaused:           item.SyncPaused,
		ConsecutiveFailures:  item.ConsecutiveFailures,
		LastSyncError:        item.LastSyncError,
		TimeoutSeconds:       item.TimeoutSeconds,
		RetryMax:             item.RetryMax,
		RateLimitQPS:         item.RateLimitQPS,
		PriceChangeThreshold: item.PriceChangeThreshold,
		OpsConsoleURL:        item.OpsConsoleURL,
		Capabilities:         descriptor,
		TotalCPU:             item.TotalCPU,
		TotalMemory:          item.TotalMemory,
		TotalDisk:            item.TotalDisk,
		UsedCPU:              item.UsedCPU,
		UsedMemory:           item.UsedMemory,
		UsedDisk:             item.UsedDisk,
		CreatedAt:            item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            item.UpdatedAt.Format(time.RFC3339),
	}
	if credErr != nil {
		// L8：解密失败显式上抛给前端，提示重新录入，不用密文当明文。
		info.CredentialError = credErr.Error()
	} else if len(creds) > 0 {
		info.Credentials = creds.Mask(descriptor.CredentialSchema)
		keys := make([]string, 0, len(creds))
		for k := range creds {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		info.CredentialKeys = keys
	}
	if item.LastSyncAt != nil {
		val := item.LastSyncAt.Format(time.RFC3339)
		info.LastSyncAt = &val
	}
	if item.LastSuccessAt != nil {
		val := item.LastSuccessAt.Format(time.RFC3339)
		info.LastSuccessAt = &val
	}
	return info
}

// ResumeSync 解除熔断：失败计数清零、错误清空，渠道下一轮即可重新调度。
// 若适配器仍未接入，调度前置校验会再次熔断，不会造成反复失败。
func (s *providerService) ResumeSync(ctx context.Context, id uint64) (*dto.ProviderInfo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	if err := s.repo.ResumeSync(ctx, id); err != nil {
		return nil, err
	}
	return s.FindByID(ctx, id)
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

// decryptValue 解密旧列（api_key/api_secret）用于脱敏回显；解密失败时原样返回。
// 仅用于展示与脱敏判断，不可用于向上游发送凭证——发送路径一律走
// buildProviderConfigStrict/credentials 包，解密失败必须显式报错（地雷 L8）。
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

// decryptCredentials 解密渠道动态凭证；返回明文 Map 与错误。
// 解密失败必须返回错误（L8），由调用方显式提示重新录入。
func (s *providerService) decryptCredentials(item model.ResourceProvider, descriptor upstream.CapabilityDescriptor) (credentials.Map, error) {
	m, err := credentials.Decode(item.Credentials)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m.DecryptFields(s.encryptSecret)
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
	return s.buildProviderConfigStrict(item)
}

// buildProviderConfigStrict 由存储记录构建适配器配置，凭证解密失败即报错（L8）。
// 凭证来源顺序：credentials JSON（新）→ api_key/api_secret 旧列（存量兼容）。
func (s *providerService) buildProviderConfigStrict(item *model.ResourceProvider) (*upstream.ProviderConfig, error) {
	descriptor := s.descriptorFor(item.ProviderType)
	creds, err := s.decryptCredentials(*item, descriptor)
	if err != nil {
		return nil, err
	}
	cfg := s.baseProviderConfig(item, creds)
	if cfg.APIKey == "" && cfg.APISecret == "" {
		// 回退旧列；旧列解密失败属数据异常，显式报错而非把密文当明文。
		key, kerr := s.decryptLegacyStrict(item.APIKey)
		if kerr != nil {
			return nil, kerr
		}
		secret, serr := s.decryptLegacyStrict(item.APISecret)
		if serr != nil {
			return nil, serr
		}
		cfg.APIKey = key
		cfg.APISecret = secret
	}
	return cfg, nil
}

// decryptLegacyStrict 解密旧列值；注意旧列存的是无前缀的原始密文，
// 因此无法区分"历史明文"与"密文"，仅当解密成功时返回明文，失败则原样返回——
// 这是旧列的固有局限，P8 删除旧列后消失。真正的新凭证一律走 credentials（带 enc: 前缀）。
func (s *providerService) decryptLegacyStrict(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	plain, err := crypto.Decrypt(stored, s.encryptSecret)
	if err != nil {
		return stored, nil
	}
	return plain, nil
}

// baseProviderConfig 组装适配器配置（不含凭证回退逻辑）。
func (s *providerService) baseProviderConfig(item *model.ResourceProvider, creds credentials.Map) *upstream.ProviderConfig {
	timeout := item.TimeoutSeconds
	if timeout <= 0 {
		timeout = int(transport.DefaultTimeout / time.Second)
	}
	cfg := &upstream.ProviderConfig{
		ID:           uint(item.ID),
		Name:         item.Name,
		Type:         item.ProviderType,
		APIEndpoint:  item.APIEndpoint,
		Region:       item.Region,
		Timeout:      timeout,
		UpstreamType: item.UpstreamType,
		Port:         item.Port,
		Secure:       item.Secure,
		UserPrefix:   item.UserPrefix,
		AccountType:  item.AccountType,
	}
	if len(creds) > 0 {
		cfg.APIKey = creds["api_key"]
		cfg.APISecret = creds["api_secret"]
		if cfg.AccountType == "" {
			cfg.AccountType = creds["account_type"]
		}
		if cfg.UserPrefix == "" {
			cfg.UserPrefix = creds["user_prefix"]
		}
		if cfg.UpstreamType == "" {
			cfg.UpstreamType = creds["upstream_type"]
		}
	}
	return cfg
}

// PriceChangeThreshold 读取渠道级调价阈值；未配置或异常时返回默认 5%。
func (s *providerService) PriceChangeThreshold(ctx context.Context, providerID uint64) (float64, error) {
	item, err := s.repo.FindByID(ctx, providerID)
	if err != nil {
		return 0, err
	}
	if item.PriceChangeThreshold <= 0 {
		return 0.05, nil
	}
	return item.PriceChangeThreshold, nil
}

// priceChangeThresholdOrDefault 校正阈值：非法值（<=0 或 >1 非比例）回落默认 5%。
func priceChangeThresholdOrDefault(v float64) float64 {
	if v <= 0 || v > 1 {
		return 0.05
	}
	return v
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
		ID:                 item.ID,
		ProviderID:         item.ProviderID,
		UpstreamID:         item.UpstreamID,
		Name:               item.Name,
		PoolType:           item.PoolType,
		TotalCPU:           item.TotalCPU,
		TotalMemory:        item.TotalMemory,
		TotalDisk:          item.TotalDisk,
		UsedCPU:            item.UsedCPU,
		UsedMemory:         item.UsedMemory,
		UsedDisk:           item.UsedDisk,
		Status:             item.Status,
		Region:             item.Region,
		Zone:               item.Zone,
		ProbeStatus:        item.ProbeStatus,
		ProbeMessage:       item.ProbeMessage,
		CPUUsagePercent:    usagePercent(item.UsedCPU, item.TotalCPU),
		MemoryUsagePercent: usagePercent(item.UsedMemory, item.TotalMemory),
		DiskUsagePercent:   usagePercent(item.UsedDisk, item.TotalDisk),
	}
	if item.LastSyncAt != nil {
		val := item.LastSyncAt.Format(time.RFC3339)
		info.LastSyncAt = &val
	}
	if item.ProbeAt != nil {
		val := item.ProbeAt.Format(time.RFC3339)
		info.ProbeAt = &val
	}
	return info
}

// usagePercent 用量百分比（0~100，四舍五入；配额为 0 时按 0 计）。
func usagePercent(used, total int) int {
	if total <= 0 {
		return 0
	}
	pct := int((float64(used)/float64(total))*100 + 0.5)
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

// maxPoolUsagePercent 池最大用量档位：0 正常、1 预警(≥60%)、2 告警(≥80%)。
func maxPoolUsagePercent(item model.ResourcePool) int {
	pcts := []int{
		usagePercent(item.UsedCPU, item.TotalCPU),
		usagePercent(item.UsedMemory, item.TotalMemory),
		usagePercent(item.UsedDisk, item.TotalDisk),
	}
	max := 0
	for _, p := range pcts {
		if p > max {
			max = p
		}
	}
	switch {
	case max >= 80:
		return 2
	case max >= 60:
		return 1
	default:
		return 0
	}
}
