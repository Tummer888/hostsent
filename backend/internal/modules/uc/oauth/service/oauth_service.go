package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/oauth/dto"
	"hostsent/backend/internal/modules/uc/oauth/model"
	"hostsent/backend/internal/modules/uc/oauth/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/credentials"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
)

// 哨兵错误（handler 逐条映射 HTTP 语义）。
var (
	// ErrProviderUnknown 未登记的渠道类型。
	ErrProviderUnknown = errors.New("未知的第三方登录渠道")
	// ErrProviderNotConfigured 渠道存在但未启用或凭证不可用。
	ErrProviderNotConfigured = errors.New("该第三方登录渠道未启用")
	// ErrBindingNotFound 当前用户没有该渠道的绑定。
	ErrBindingNotFound = errors.New("未绑定该第三方账号")
	// ErrNoInviteCode 保留给未来扩展；当前不返回。
	ErrNoInviteCode = errors.New("邀请码无效")
)

// UserRepo 第三方登录所需的用户读写端口。
//
// 抽成端口而不是直接依赖 uc/auth 的仓储：oauth 服务需要「按 openid 找用户」之外
// 还要写 users 快照与创建自动注册用户，这些都不属于 uc/auth 自助场景的接口语义。
// 实现由装配层用 uc/auth 的模型/数据库适配（见 internal/server/assembly_oauth.go）。
type UserRepo interface {
	FindByID(ctx context.Context, id uint64) (*UserBrief, error)
	FindByUsername(ctx context.Context, username string) (*UserBrief, error)
	FindByEmail(ctx context.Context, email string) (*UserBrief, error)
	// CreateOAuthUser 创建自动注册用户，返回新用户 ID。
	CreateOAuthUser(ctx context.Context, in CreateUserInput) (uint64, error)
	// CountLoginMethods 统计该用户除指定 provider 外还剩多少种登录方式
	// （有密码 / 有其他绑定 / 有已验证手机）。解绑守卫用它。
	CountLoginMethods(ctx context.Context, userID uint64, excludeProvider string) (int, error)
}

// UserBrief 用户最小信息。
type UserBrief struct {
	ID           uint64
	Username     string
	Email        string
	RealName     string
	Avatar       string
	Status       string
	DeletedAt    *time.Time
	IsSubAccount bool
	OwnerUserID  *uint64
	// PasswordHash 仅用于判断「是否设置过可用密码」（自动注册用户写的是随机哈希）。
	PasswordHash    string
	Phone           string
	PhoneVerifiedAt *time.Time
	UserGroupID     *uint64
	HasInviter      bool
}

// CreateUserInput 自动注册用户入参。
type CreateUserInput struct {
	Username string
	Email    string
	// PasswordHash 随机 bcrypt 哈希：该账号永远无法用密码登录，
	// 只能通过第三方登录进入（用户后续可自助设置密码）。
	PasswordHash string
	UserGroupID  *uint64
}

// InviteBinder 邀请码解析与绑定（装配层注入 admin/referral 能力）。
type InviteBinder interface {
	ResolveInviter(ctx context.Context, code string) (uint64, error)
	BindInviter(ctx context.Context, inviteeID, inviterID uint64) error
}

// DefaultGroupResolver 默认用户组解析（自动注册时归组，复用 R5 的解析器）。
type DefaultGroupResolver interface {
	DefaultGroupID(ctx context.Context) (uint64, error)
}

// ConfigReader 读系统配置原文（键不存在返回 ok=false）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// Service 第三方登录服务。
type Service interface {
	// —— 管理端 ——
	ProviderTypes() []dto.ProviderTypeInfo
	ListProviders(ctx context.Context) ([]dto.ProviderInfo, error)
	UpdateProvider(ctx context.Context, provider string, req dto.ProviderUpdateRequest) (*dto.ProviderInfo, error)
	TestProvider(ctx context.Context, provider string) error
	ListBindings(ctx context.Context, query dto.BindingListQuery) (*dto.BindingListResponse, error)

	// —— 用户端 ——
	PublicProviders(ctx context.Context) ([]dto.PublicProviderInfo, error)
	AuthorizeURL(ctx context.Context, provider, mode string, userID uint64, inviteCode string) (string, error)
	Callback(ctx context.Context, provider, code, state, ip, userAgent string) (*CallbackResult, error)
	Exchange(ctx context.Context, ticket string) (*dto.ExchangeResponse, error)
	MyBindings(ctx context.Context, userID uint64) ([]dto.MyBindingInfo, error)
	Unbind(ctx context.Context, userID uint64, provider string) error

	// SetInviteBinder 注入邀请绑定能力（可选）。
	SetInviteBinder(binder InviteBinder)
	// SetDefaultGroupResolver 注入默认用户组解析能力（可选）。
	SetDefaultGroupResolver(resolver DefaultGroupResolver)
}

// CallbackResult 回调处理结果（handler 据此决定重定向地址）。
type CallbackResult struct {
	// RedirectURL 前端回跳地址（带 ticket 或错误码）。
	RedirectURL string
	// Ticket 一次性结果票据；为空表示直接带回错误。
	Ticket   string
	Provider string
}

type service struct {
	repo       repository.Repository
	users      UserRepo
	signer     *oauthpkg.StateSigner
	jwtIssuer  *appauth.JWTIssuer
	cfg        ConfigReader
	cache      *cache.Client
	encryptKey string
	logger     *zap.Logger
	// callbackBase 后端回调基地址（如 https://api.example.com/api/v1/uc/oauth）。
	callbackBase string
	// frontendCallback 前端回跳地址（登录页所在域名下的回调页）。
	frontendCallback string
	// db 直连用于写 login_logs / user_sessions（这两个表不属于本模块的领域模型，
	// 走裸 INSERT 而不是把它们拖进 oauth 仓储的接口语义里）。
	db *gorm.DB

	inviteBinder InviteBinder
	defaultGroup DefaultGroupResolver
}

// Deps 服务依赖。
type Deps struct {
	Repo       repository.Repository
	Users      UserRepo
	Signer     *oauthpkg.StateSigner
	JWTIssuer  *appauth.JWTIssuer
	Config     ConfigReader
	Cache      *cache.Client
	EncryptKey string
	Logger     *zap.Logger
	// CallbackBase 后端回调基地址，用于拼装 redirect_uri 与回调地址展示。
	CallbackBase string
	// FrontendCallback 前端回跳地址。
	FrontendCallback string
	// DB 直连（写登录日志与会话用）。
	DB *gorm.DB
}

// New 创建第三方登录服务。
func New(d Deps) Service {
	logger := d.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &service{
		repo:             d.Repo,
		users:            d.Users,
		signer:           d.Signer,
		jwtIssuer:        d.JWTIssuer,
		cfg:              d.Config,
		cache:            d.Cache,
		encryptKey:       d.EncryptKey,
		logger:           logger,
		db:               d.DB,
		callbackBase:     strings.TrimSuffix(d.CallbackBase, "/"),
		frontendCallback: d.FrontendCallback,
	}
}

func (s *service) SetInviteBinder(binder InviteBinder)            { s.inviteBinder = binder }
func (s *service) SetDefaultGroupResolver(r DefaultGroupResolver) { s.defaultGroup = r }

// configBool 读布尔配置，缺失/非法回落 fallback（口径与 doc91 开关一致）。
func (s *service) configBool(ctx context.Context, key string, fallback bool) bool {
	if s.cfg == nil {
		return fallback
	}
	raw, ok, err := s.cfg(ctx, key)
	if err != nil || !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}

// autoRegisterEnabled 是否允许第三方登录自动注册。
//
// 默认 true：第三方登录的主要价值就是降低注册门槛；关闭后未绑定用户会收到
// 「请先注册并绑定」的提示（need_bind=true），这是运营可选的更严格策略。
func (s *service) autoRegisterEnabled(ctx context.Context) bool {
	return s.configBool(ctx, "oauth.auto_register", true)
}

// callbackURI 拼装某渠道的回调地址。
//
// 优先读配置 oauth.callback_base（运营在后台填写对外可访问的后端地址），
// 未配置时回落装配期注入的 CallbackBase（本地联调可写死 127.0.0.1）。
func (s *service) callbackBaseFor(ctx context.Context) string {
	if s.cfg != nil {
		if raw, ok, err := s.cfg(ctx, "oauth.callback_base"); err == nil && ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}
	return s.callbackBase
}

func (s *service) callbackURI(ctx context.Context, provider string) string {
	return strings.TrimSuffix(s.callbackBaseFor(ctx), "/") + "/" + provider + "/callback"
}

// redirectToFrontend 拼装前端回跳地址（错误也走这里，前端按 error 参数提示）。
//
// 地址优先读配置 oauth.frontend_callback：这是三方登录回跳的落点，
// 部署域名各不相同，写死在代码里等于每次上线都要改代码。
func (s *service) redirectToFrontend(ctx context.Context, ticket, provider, errMsg string) string {
	base := s.frontendCallback
	if s.cfg != nil {
		if raw, ok, err := s.cfg(ctx, "oauth.frontend_callback"); err == nil && ok && strings.TrimSpace(raw) != "" {
			base = strings.TrimSpace(raw)
		}
	}
	if base == "" {
		base = "/oauth/callback"
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	if ticket != "" {
		return base + sep + "ticket=" + ticket + "&provider=" + provider
	}
	return base + sep + "error=" + errMsg + "&provider=" + provider
}

// ---------- 管理端 ----------

// ProviderTypes 渠道类型元数据。
func (s *service) ProviderTypes() []dto.ProviderTypeInfo {
	all := oauthpkg.AllDescriptors()
	out := make([]dto.ProviderTypeInfo, 0, len(all))
	for _, d := range all {
		out = append(out, dto.ProviderTypeInfo{
			Type:             d.Type,
			Name:             d.Name,
			Mode:             d.Mode,
			Icon:             d.Icon,
			DocURL:           d.DocURL,
			AdapterVersion:   d.AdapterVersion,
			DefaultScopes:    d.DefaultScopes,
			Implemented:      d.Implemented,
			CredentialSchema: d.CredentialSchema,
		})
	}
	return out
}

// ListProviders 渠道配置列表。
//
// 库里没有行时按描述符补一条「内存态」条目（ID=0）：运营首次进入配置页
// 应该看到微信/QQ/支付宝三张待配置的卡片，而不是空白页要求他先「新建渠道」。
func (s *service) ListProviders(ctx context.Context) ([]dto.ProviderInfo, error) {
	rows, err := s.repo.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	byProvider := make(map[string]*model.OAuthProvider, len(rows))
	for i := range rows {
		byProvider[rows[i].Provider] = &rows[i]
	}
	// 回调地址按当前配置拼装一次，整列表复用（避免逐条读配置）。
	base := strings.TrimSuffix(s.callbackBaseFor(ctx), "/")
	out := make([]dto.ProviderInfo, 0, len(oauthpkg.AllDescriptors()))
	for _, d := range oauthpkg.AllDescriptors() {
		if row, ok := byProvider[d.Type]; ok {
			out = append(out, s.toProviderInfo(row, d, base))
			delete(byProvider, d.Type)
			continue
		}
		out = append(out, s.virtualProviderInfo(d, base))
	}
	// 库里存在但代码已无描述符的渠道（历史遗留）也要展示，否则运营无法停用它。
	for _, row := range byProvider {
		out = append(out, s.toProviderInfo(row, oauthpkg.CapabilityDescriptor{Type: row.Provider, Name: row.Name}, base))
	}
	return out, nil
}

// virtualProviderInfo 未落库渠道的展示态（enabled=false，凭证为空）。
func (s *service) virtualProviderInfo(d oauthpkg.CapabilityDescriptor, callbackBase string) dto.ProviderInfo {
	return dto.ProviderInfo{
		Provider:       d.Type,
		Name:           d.Name,
		Enabled:        false,
		Mode:           d.Mode,
		Icon:           d.Icon,
		Scopes:         d.DefaultScopes,
		Supported:      d.Implemented,
		Implemented:    d.Implemented,
		Credentials:    map[string]string{},
		CredentialKeys: []string{},
		CallbackURL:    callbackBase + "/" + d.Type + "/callback",
	}
}

// toProviderInfo 转换并脱敏凭证。
func (s *service) toProviderInfo(row *model.OAuthProvider, d oauthpkg.CapabilityDescriptor, callbackBase string) dto.ProviderInfo {
	info := dto.ProviderInfo{
		ID:             row.ID,
		Provider:       row.Provider,
		Name:           row.Name,
		Enabled:        row.Enabled,
		Mode:           row.Mode,
		Scopes:         row.Scopes,
		Icon:           row.Icon,
		SortOrder:      row.SortOrder,
		HealthStatus:   row.HealthStatus,
		LastError:      row.LastError,
		Remark:         row.Remark,
		Supported:      oauthpkg.IsRegistered(row.Provider),
		Credentials:    map[string]string{},
		CredentialKeys: []string{},
		CallbackURL:    callbackBase + "/" + row.Provider + "/callback",
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	info.Implemented = info.Supported
	if row.LastCheckAt != nil {
		info.LastCheckAt = row.LastCheckAt.Format(time.RFC3339)
	}
	// 凭证回显：secret 字段一律脱敏（credentials.Mask），非密文字段原样。
	values, err := credentials.Decode(row.Credentials)
	if err == nil && len(values) > 0 {
		for k, v := range values.Mask(d.CredentialSchema) {
			info.Credentials[k] = v
			info.CredentialKeys = append(info.CredentialKeys, k)
		}
	}
	return info
}

// UpdateProvider 更新渠道配置（不存在则按描述符创建）。
//
// 用 PUT /providers/:provider 而不是 POST + id：渠道类型是固定的三家，
// 「先建行再配置」会让运营多一步无意义操作，且容易建出重复行。
func (s *service) UpdateProvider(ctx context.Context, provider string, req dto.ProviderUpdateRequest) (*dto.ProviderInfo, error) {
	d, ok := oauthpkg.Descriptor(provider)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderUnknown, provider)
	}
	row, err := s.repo.FindProvider(ctx, provider)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = &model.OAuthProvider{
			Provider: provider,
			Name:     d.Name,
			Mode:     d.Mode,
			Icon:     d.Icon,
			Scopes:   d.DefaultScopes,
		}
	} else if err != nil {
		return nil, err
	}

	// 凭证合并：回传脱敏值表示「未修改」，保留原密文（对齐支付/验证码渠道语义）。
	existing, _ := credentials.Decode(row.Credentials)
	merged := map[string]string{}
	for k, v := range req.Credentials {
		if credentials.IsMaskedEcho(credentials.MaskSecret(existing[k]), v) && existing[k] != "" {
			merged[k] = existing[k]
			continue
		}
		merged[k] = v
	}
	// 启用时必须凭证齐全：允许「先存部分凭证但停用」，不允许「启用了却调不通」。
	enabled := row.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if enabled {
		if err := oauthpkg.ValidateCredentials(d, plainOf(merged)); err != nil {
			return nil, err
		}
	}
	encrypted, err := s.encryptCredentials(d, merged)
	if err != nil {
		return nil, err
	}
	row.Enabled = enabled
	row.Credentials = encrypted
	row.Descriptor = mustJSON(d)
	if strings.TrimSpace(req.Scopes) != "" {
		row.Scopes = req.Scopes
	}
	if req.SortOrder != nil {
		row.SortOrder = *req.SortOrder
	}
	row.Remark = req.Remark

	if row.ID == 0 {
		if err := s.repo.CreateProvider(ctx, row); err != nil {
			return nil, err
		}
	} else if err := s.repo.UpdateProvider(ctx, row); err != nil {
		return nil, err
	}
	info := s.toProviderInfo(row, d, strings.TrimSuffix(s.callbackBaseFor(ctx), "/"))
	return &info, nil
}

// TestProvider 连通性测试。
//
// 未注册适配器时把 health 标 pending 并返回 ErrAdapterNotImplemented ——
// 与验证码渠道同款：这是「待接入」的业务状态，不是服务器错误。
func (s *service) TestProvider(ctx context.Context, provider string) error {
	row, err := s.repo.FindProvider(ctx, provider)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: 请先填写并保存 %s 的凭证", ErrProviderNotConfigured, provider)
		}
		return err
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		return err
	}
	prov, err := oauthpkg.New(provider, oauthpkg.ProviderConfig{
		ProviderID:  row.ID,
		Type:        provider,
		Credentials: creds,
		Scopes:      row.Scopes,
	})
	if err != nil {
		s.markHealth(ctx, row, model.HealthPending, oauthpkg.ErrAdapterNotImplemented.Error())
		return oauthpkg.ErrAdapterNotImplemented
	}
	if err := prov.Test(ctx, oauthpkg.ProviderConfig{
		ProviderID: row.ID, Type: provider, Credentials: creds, Scopes: row.Scopes,
	}); err != nil {
		s.markHealth(ctx, row, model.HealthDown, err.Error())
		return err
	}
	s.markHealth(ctx, row, model.HealthHealthy, "")
	return nil
}

func (s *service) markHealth(ctx context.Context, row *model.OAuthProvider, status, lastErr string) {
	now := time.Now()
	row.HealthStatus = status
	row.LastError = lastErr
	row.LastCheckAt = &now
	if err := s.repo.UpdateProvider(ctx, row); err != nil {
		s.logger.Warn("update oauth provider health failed", zap.Error(err), zap.String("provider", row.Provider))
	}
}

// ListBindings 管理端绑定排查列表。
func (s *service) ListBindings(ctx context.Context, query dto.BindingListQuery) (*dto.BindingListResponse, error) {
	items, total, err := s.repo.ListBindings(ctx, query)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []dto.BindingInfo{}
	}
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.BindingListResponse{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// ---------- 用户端 ----------

// PublicProviders 已启用的渠道元数据（**不含任何凭证**）。
//
// 只返回「已启用且适配器已注册」的渠道：配了一半的渠道出现在登录页上，
// 用户点进去只会拿到「渠道未启用」——不如不展示。
func (s *service) PublicProviders(ctx context.Context) ([]dto.PublicProviderInfo, error) {
	rows, err := s.repo.ListEnabledProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PublicProviderInfo, 0, len(rows))
	for i := range rows {
		if !oauthpkg.IsRegistered(rows[i].Provider) {
			continue
		}
		out = append(out, dto.PublicProviderInfo{
			Provider: rows[i].Provider,
			Name:     rows[i].Name,
			Icon:     rows[i].Icon,
			Mode:     rows[i].Mode,
		})
	}
	return out, nil
}

// AuthorizeURL 生成授权地址。
//
// mode=bind 时 userID 必须非零：绑定必须落到发起时的登录用户，否则攻击者
// 可以用自己的 state 把别人的账号绑到自己的第三方账号上（doc104 §6.4）。
func (s *service) AuthorizeURL(ctx context.Context, provider, mode string, userID uint64, inviteCode string) (string, error) {
	row, prov, err := s.resolveProvider(ctx, provider)
	if err != nil {
		return "", err
	}
	if mode == oauthpkg.ModeBind && userID == 0 {
		return "", errors.New("绑定第三方账号需要先登录")
	}
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}
	state, err := s.signer.IssueState(provider, mode, userID, inviteCode, nonce)
	if err != nil {
		return "", err
	}
	// Redis 可用时记一份 nonce 供一次性消费；不可用时降级为「仅短 TTL」。
	// 这里显式记录降级事实，而不是静默 —— 与 pkg/captcha「降级即放行」不同，
	// OAuth 的 CSRF 防护不能放行，只能把重放窗口压到 TTL 内。
	if s.cache != nil {
		if err := s.cache.Set(ctx, oauthNonceKey(nonce), "1", oauthpkg.StateTTL); err != nil {
			s.logger.Warn("oauth state nonce not cached (replay window = state TTL)",
				zap.String("provider", provider))
		}
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		return "", err
	}
	return prov.AuthorizeURL(oauthpkg.ProviderConfig{
		ProviderID: row.ID, Type: provider, Credentials: creds, Scopes: row.Scopes,
	}, state, s.callbackURI(ctx, provider)), nil
}

// CallbackResult 回调处理（doc104 §6.5）。
func (s *service) Callback(ctx context.Context, provider, code, state, ip, userAgent string) (*CallbackResult, error) {
	claims, err := s.signer.ParseState(state)
	if err != nil {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "state_invalid")}, nil
	}
	if claims.Provider != provider {
		// state 里的渠道与回调路径不一致：可能是把 A 渠道的 state 用到 B 渠道，
		// 一律按无效处理（不做「以 state 为准」的宽容，那会打开跨渠道混淆的口子）。
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "state_mismatch")}, nil
	}
	s.consumeNonce(ctx, claims.Nonce)

	row, prov, err := s.resolveProvider(ctx, provider)
	if err != nil {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "provider_unavailable")}, nil
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "credential_error")}, nil
	}
	cfg := oauthpkg.ProviderConfig{ProviderID: row.ID, Type: provider, Credentials: creds, Scopes: row.Scopes}

	token, err := prov.Exchange(ctx, cfg, code, s.callbackURI(ctx, provider))
	if err != nil {
		s.logger.Warn("oauth exchange failed", zap.String("provider", provider), zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "exchange_failed")}, nil
	}
	ext, err := prov.UserInfo(ctx, cfg, token)
	if err != nil {
		s.logger.Warn("oauth userinfo failed", zap.String("provider", provider), zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "userinfo_failed")}, nil
	}
	if ext.OpenID == "" {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "openid_missing")}, nil
	}

	switch claims.Mode {
	case oauthpkg.ModeBind:
		return s.handleBind(ctx, provider, ext, claims.UserID)
	default:
		return s.handleLogin(ctx, provider, ext, claims.InviteCode, ip, userAgent)
	}
}

// handleBind 已登录用户绑定新渠道。
func (s *service) handleBind(ctx context.Context, provider string, ext *oauthpkg.ExternalUser, userID uint64) (*CallbackResult, error) {
	if userID == 0 {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "not_logged_in")}, nil
	}
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "user_not_found")}, nil
	}
	// 该 openid 是否已被别人绑走。
	if existing, err := s.repo.FindBinding(ctx, provider, ext.OpenID); err == nil {
		if existing.UserID != userID {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "openid_taken")}, nil
		}
		// 已绑给自己：幂等成功，刷新资料。
		existing.Nickname = ext.Nickname
		existing.Avatar = ext.Avatar
		existing.UnionID = ext.UnionID
		existing.Status = model.StatusActive
		if err := s.repo.UpdateProviderBinding(ctx, existing); err != nil {
			s.logger.Warn("refresh oauth binding failed", zap.Error(err))
		}
		return s.issueBindTicket(ctx, provider, userID)
	}
	now := time.Now()
	binding := &model.UserOAuthBinding{
		UserID:   userID,
		Provider: provider,
		OpenID:   ext.OpenID,
		UnionID:  ext.UnionID,
		Nickname: ext.Nickname,
		Avatar:   ext.Avatar,
		Status:   model.StatusActive,
		BoundAt:  now,
	}
	if err := s.repo.CreateBinding(ctx, binding); err != nil {
		if errors.Is(err, repository.ErrOpenIDTaken) {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "openid_taken")}, nil
		}
		s.logger.Warn("create oauth binding failed", zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "bind_failed")}, nil
	}
	s.syncPrimarySnapshot(ctx, userID)
	return s.issueBindTicket(ctx, provider, userID)
}

func (s *service) issueBindTicket(ctx context.Context, provider string, userID uint64) (*CallbackResult, error) {
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	ticket, err := s.signer.IssueTicket(userID, provider, false, nonce)
	if err != nil {
		return nil, err
	}
	s.issueTicketNonce(ctx, nonce)
	return &CallbackResult{
		RedirectURL: s.redirectToFrontend(ctx, ticket, provider, ""),
		Ticket:      ticket,
		Provider:    provider,
	}, nil
}

// handleLogin 登录 / 自动注册。
func (s *service) handleLogin(ctx context.Context, provider string, ext *oauthpkg.ExternalUser, inviteCode, ip, userAgent string) (*CallbackResult, error) {
	binding, err := s.repo.FindBinding(ctx, provider, ext.OpenID)
	if err == nil {
		// 已绑定：校验绑定与用户状态。
		if binding.Status != model.StatusActive {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "binding_disabled")}, nil
		}
		user, err := s.users.FindByID(ctx, binding.UserID)
		if err != nil {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "user_not_found")}, nil
		}
		// 已注销用户不得通过第三方登录进入（软删除的账号必须彻底不可登录）。
		if user.DeletedAt != nil || user.Status != "active" {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "account_unavailable")}, nil
		}
		now := time.Now()
		_ = s.repo.UpdateBindingLogin(ctx, binding.ID, now)
		_ = s.repo.UpdateBindingProfile(ctx, binding.ID, ext.Nickname, ext.Avatar, ext.UnionID)
		_ = s.recordLogin(ctx, user.ID, user.Username, provider, ip, userAgent)
		_ = s.openSession(ctx, user.ID, user.Username, provider, ip, userAgent)
		return s.issueLoginTicket(ctx, provider, user.ID, false)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warn("find oauth binding failed", zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "lookup_failed")}, nil
	}

	// 未绑定：自动注册关闭时回 need_bind，让用户先用账号密码登录再绑定。
	if !s.autoRegisterEnabled(ctx) {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "need_bind")}, nil
	}

	userID, err := s.autoRegister(ctx, provider, ext, inviteCode)
	if err != nil {
		s.logger.Warn("oauth auto register failed", zap.String("provider", provider), zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "register_failed")}, nil
	}
	now := time.Now()
	binding = &model.UserOAuthBinding{
		UserID:   userID,
		Provider: provider,
		OpenID:   ext.OpenID,
		UnionID:  ext.UnionID,
		Nickname: ext.Nickname,
		Avatar:   ext.Avatar,
		Status:   model.StatusActive,
		BoundAt:  now,
	}
	if err := s.repo.CreateBinding(ctx, binding); err != nil {
		// 注册成功但绑定被抢（并发）：返回失败，用户可重试登录（此时已能查到绑定）。
		if errors.Is(err, repository.ErrOpenIDTaken) {
			return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "openid_taken")}, nil
		}
		s.logger.Warn("create oauth binding after register failed", zap.Error(err))
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "bind_failed")}, nil
	}
	s.syncPrimarySnapshot(ctx, userID)
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return &CallbackResult{RedirectURL: s.redirectToFrontend(ctx, "", provider, "user_not_found")}, nil
	}
	_ = s.recordLogin(ctx, user.ID, user.Username, provider, ip, userAgent)
	_ = s.openSession(ctx, user.ID, user.Username, provider, ip, userAgent)
	return s.issueLoginTicket(ctx, provider, userID, false)
}

// autoRegister 自动注册第三方账号用户。
//
// 用户名 `{provider}_{openid前8位}`（冲突追加随机后缀），邮箱
// `{provider}_{openid}@oauth.local` 保证非空且唯一（users.email 是 NOT NULL + 唯一索引）。
// 密码写随机 32 字节的 bcrypt —— 该账号永远无法用密码登录，只能走第三方，
// 用户可在「安全设置」里自助设置密码后再用密码登录。
func (s *service) autoRegister(ctx context.Context, provider string, ext *oauthpkg.ExternalUser, inviteCode string) (uint64, error) {
	username := s.uniqueUsername(ctx, provider, ext.OpenID)
	email := provider + "_" + sanitizeEmailPart(ext.OpenID) + "@oauth.local"
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		email = provider + "_" + sanitizeEmailPart(ext.OpenID) + "_" + randomSuffix() + "@oauth.local"
	}
	hash, err := randomPasswordHash()
	if err != nil {
		return 0, err
	}
	var groupID *uint64
	if s.defaultGroup != nil {
		if id, err := s.defaultGroup.DefaultGroupID(ctx); err == nil && id > 0 {
			groupID = &id
		}
	}
	userID, err := s.users.CreateOAuthUser(ctx, CreateUserInput{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		UserGroupID:  groupID,
	})
	if err != nil {
		return 0, err
	}
	// 邀请关系：OAuth 注册也要能计入返现，否则「邀请链接 + 微信登录」这条路径的
	// 邀请人永远拿不到返现（doc104 §6.5）。
	if s.inviteBinder != nil && strings.TrimSpace(inviteCode) != "" {
		if inviterID, err := s.inviteBinder.ResolveInviter(ctx, inviteCode); err == nil && inviterID > 0 && inviterID != userID {
			if err := s.inviteBinder.BindInviter(ctx, userID, inviterID); err != nil {
				s.logger.Warn("bind inviter for oauth user failed", zap.Error(err), zap.Uint64("user_id", userID))
			}
		}
	}
	return userID, nil
}

// uniqueUsername 生成不冲突的用户名。
func (s *service) uniqueUsername(ctx context.Context, provider, openid string) string {
	base := provider + "_" + sanitizeEmailPart(openid)
	if len(base) > 48 {
		base = base[:48]
	}
	if _, err := s.users.FindByUsername(ctx, base); err != nil {
		return base
	}
	for i := 0; i < 5; i++ {
		candidate := base + "_" + randomSuffix()
		if _, err := s.users.FindByUsername(ctx, candidate); err != nil {
			return candidate
		}
	}
	// 极端情况下仍冲突：交给数据库唯一索引报错，由上层转成 register_failed。
	return base + "_" + randomSuffix()
}

func (s *service) issueLoginTicket(ctx context.Context, provider string, userID uint64, needBind bool) (*CallbackResult, error) {
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	ticket, err := s.signer.IssueTicket(userID, provider, needBind, nonce)
	if err != nil {
		return nil, err
	}
	s.issueTicketNonce(ctx, nonce)
	return &CallbackResult{
		RedirectURL: s.redirectToFrontend(ctx, ticket, provider, ""),
		Ticket:      ticket,
		Provider:    provider,
	}, nil
}

// Exchange ticket → 正式令牌。
//
// ticket 是一次性的：解析成功后立刻在 Redis 中消费 nonce，防止同一张票据被
// 并发换多次。Redis 不可用时降级为 TTL 防护（60 秒窗口）——此时不阻断，
// 因为票据本身仍是签名 JWT，攻击者无法伪造，只是重放窗口从 0 放宽到 60 秒。
func (s *service) Exchange(ctx context.Context, ticket string) (*dto.ExchangeResponse, error) {
	claims, err := s.signer.ParseTicket(ticket)
	if err != nil {
		return nil, err
	}
	if err := s.consumeTicketNonce(ctx, claims.ID); err != nil {
		return nil, err
	}
	if claims.NeedBind {
		return &dto.ExchangeResponse{NeedBind: true, Provider: claims.Provider}, nil
	}
	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	if user.DeletedAt != nil || user.Status != "active" {
		return nil, errors.New("账号状态不可用，请联系客服")
	}
	ownerID := uint64(0)
	if user.OwnerUserID != nil {
		ownerID = *user.OwnerUserID
	}
	token, err := s.jwtIssuer.GenerateUserFull(user.Username, user.ID, "free", ownerID, user.IsSubAccount)
	if err != nil {
		return nil, err
	}
	name := user.RealName
	if name == "" {
		name = user.Username
	}
	return &dto.ExchangeResponse{
		Token:    token,
		Provider: claims.Provider,
		User: &dto.ExchangeUser{
			ID:       user.ID,
			Username: user.Username,
			Name:     name,
			Email:    user.Email,
			Avatar:   user.Avatar,
			Status:   user.Status,
		},
	}, nil
}

// MyBindings 当前用户的绑定列表。
func (s *service) MyBindings(ctx context.Context, userID uint64) ([]dto.MyBindingInfo, error) {
	rows, err := s.repo.ListBindingsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 解绑守卫的预判：解绑任一渠道后是否还有别的登录方式。
	// 这里算的是「解绑该渠道后剩下的方式数」，逐条独立判断。
	remainingBase, err := s.users.CountLoginMethods(ctx, userID, "")
	if err != nil {
		remainingBase = 0
	}
	out := make([]dto.MyBindingInfo, 0, len(rows))
	for i := range rows {
		name, icon := providerLabel(rows[i].Provider)
		canUnbind := remainingBase > 1
		out = append(out, dto.MyBindingInfo{
			Provider:    rows[i].Provider,
			Name:        name,
			Icon:        icon,
			Nickname:    rows[i].Nickname,
			Avatar:      rows[i].Avatar,
			Status:      rows[i].Status,
			BoundAt:     rows[i].BoundAt,
			LastLoginAt: rows[i].LastLoginAt,
			CanUnbind:   canUnbind,
		})
	}
	return out, nil
}

// Unbind 解绑。
//
// 守卫：解绑后用户必须**仍至少保留一种登录方式**（有密码 / 有其他绑定 /
// 有已验证手机）。否则账号会变成「谁都进不去」的孤岛，只能靠管理员介入。
func (s *service) Unbind(ctx context.Context, userID uint64, provider string) error {
	rows, err := s.repo.ListBindingsByUser(ctx, userID)
	if err != nil {
		return err
	}
	found := false
	for _, row := range rows {
		if row.Provider == provider {
			found = true
			break
		}
	}
	if !found {
		return ErrBindingNotFound
	}
	remaining, err := s.users.CountLoginMethods(ctx, userID, provider)
	if err != nil {
		return err
	}
	if remaining == 0 {
		return oauthpkg.ErrLastLoginMethod
	}
	if err := s.repo.DeleteBinding(ctx, userID, provider); err != nil {
		return err
	}
	s.syncPrimarySnapshot(ctx, userID)
	return nil
}

// syncPrimarySnapshot 把主绑定快照写回 users。
//
// 主绑定 = 剩余绑定中 bound_at 最早的一条；无绑定则清空两列。
// 管理端列表与详情页读的就是这两列，不维护会让「已绑定微信」的图标凭空消失。
func (s *service) syncPrimarySnapshot(ctx context.Context, userID uint64) {
	rows, err := s.repo.ListBindingsByUser(ctx, userID)
	if err != nil {
		s.logger.Warn("list bindings for snapshot failed", zap.Error(err), zap.Uint64("user_id", userID))
		return
	}
	primaryProvider, primaryOpenID := "", ""
	for _, row := range rows {
		if row.Status != model.StatusActive {
			continue
		}
		primaryProvider, primaryOpenID = row.Provider, row.OpenID
		break
	}
	if err := s.repo.SyncPrimarySnapshot(ctx, userID, primaryProvider, primaryOpenID); err != nil {
		s.logger.Warn("sync primary oauth snapshot failed", zap.Error(err), zap.Uint64("user_id", userID))
	}
}

// resolveProvider 取出已启用的渠道与已构造的适配器。
func (s *service) resolveProvider(ctx context.Context, provider string) (*model.OAuthProvider, oauthpkg.Provider, error) {
	if _, ok := oauthpkg.Descriptor(provider); !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrProviderUnknown, provider)
	}
	row, err := s.repo.FindProvider(ctx, provider)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, provider)
		}
		return nil, nil, err
	}
	if !row.Enabled {
		return nil, nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, provider)
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		return nil, nil, err
	}
	prov, err := oauthpkg.New(provider, oauthpkg.ProviderConfig{
		ProviderID: row.ID, Type: provider, Credentials: creds, Scopes: row.Scopes,
	})
	if err != nil {
		return nil, nil, err
	}
	return row, prov, nil
}

// ---------- 内部工具 ----------

// oauthNonceKey nonce 的缓存键（一次性消费用）。
func oauthNonceKey(nonce string) string { return "oauth:state:" + nonce }

// oauthTicketKey 票据 nonce 的缓存键。
//
// 与 state 的键分开：两者生命周期不同（state 5 分钟、票据 60 秒），
// 混在一个命名空间里会让「谁消费了谁」在排查时看不出来。
func oauthTicketKey(nonce string) string { return "oauth:ticket:" + nonce }

// consumeNonce 尝试一次性消费 state 的 nonce。
//
// Redis 可用时删除成功才继续；不可用（键不存在）时降级放行并记警告——
// 注意这与「放行」不同：state 本身仍是签名 JWT，攻击者无法伪造，
// 降级只是把「重放窗口」从 0 放宽到 TTL（5 分钟）。
func (s *service) consumeNonce(ctx context.Context, nonce string) {
	if s.cache == nil || nonce == "" {
		return
	}
	if _, ok := s.cache.GetDel(ctx, oauthNonceKey(nonce)); !ok {
		s.logger.Warn("oauth state nonce not consumed (cache degraded or already used)",
			zap.String("nonce", nonce))
	}
}

// issueTicketNonce 登记票据 nonce 供一次性消费。
//
// 必须在签发票据时登记：Exchange 侧靠 GetDel 判断「这张票是不是已经被用过」，
// 没登记过就永远删不到，一次性就成了纸面约定（重复提交同一张票能反复换到令牌）。
func (s *service) issueTicketNonce(ctx context.Context, nonce string) {
	if s.cache == nil || nonce == "" {
		return
	}
	if err := s.cache.Set(ctx, oauthTicketKey(nonce), "1", oauthpkg.TicketTTL); err != nil {
		s.logger.Warn("oauth ticket nonce not cached (ticket replay window = TTL)",
			zap.String("nonce", nonce))
	}
}

// oauthTicketUsedKey 票据「已兑换」标记的缓存键。
func oauthTicketUsedKey(nonce string) string { return "oauth:ticket:used:" + nonce }

// consumeTicketNonce 消费票据 nonce，已兑换过的票据返回错误。
//
// 为什么不用「GetDel 拿不到就拒绝」：拿不到有两种原因——「已经被兑换过」
// 与「缓存被清空/键过期了但票据还在 60 秒有效期内」。前者必须拒绝，后者拒绝
// 等于把正在登录的用户挡在门外（Redis 重启一次，所有在途登录全失败）。
// 因此额外落一个 used 标记来区分：
//   - 键还在 → 正常，删键 + 落 used 标记，放行；
//   - 键没了但 used 标记也没有 → 缓存侧的意外丢失，记 Warn 后放行（重放窗口 = TTL）；
//   - 键没了而 used 标记在 → 确凿的重复兑换，拒绝。
func (s *service) consumeTicketNonce(ctx context.Context, nonce string) error {
	if s.cache == nil || !s.cache.Enabled() {
		// 整体降级（未启用 Redis）：退回 TTL 防护，不阻断登录。
		return nil
	}
	if nonce == "" {
		return oauthpkg.ErrInvalidState
	}
	if _, ok := s.cache.GetDel(ctx, oauthTicketKey(nonce)); ok {
		if err := s.cache.Set(ctx, oauthTicketUsedKey(nonce), "1", oauthpkg.TicketTTL); err != nil {
			s.logger.Warn("mark oauth ticket used failed", zap.String("nonce", nonce), zap.Error(err))
		}
		return nil
	}
	if first, err := s.cache.SetNX(ctx, oauthTicketUsedKey(nonce), "1", oauthpkg.TicketTTL); err == nil && first {
		s.logger.Warn("oauth ticket nonce missing but not used (cache degraded?)",
			zap.String("nonce", nonce))
		return nil
	}
	s.logger.Warn("oauth ticket already used", zap.String("nonce", nonce))
	return oauthpkg.ErrInvalidState
}

// encryptCredentials 按描述符加密 secret 字段（幂等：已加密原样保留）。
func (s *service) encryptCredentials(d oauthpkg.CapabilityDescriptor, values map[string]string) (string, error) {
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
func (s *service) decryptCredentials(row *model.OAuthProvider) (map[string]string, error) {
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

// plainOf 把可能含密文的 map 原样返回（ValidateCredentials 只判空）。
func plainOf(m map[string]string) map[string]string { return m }

func providerLabel(provider string) (name, icon string) {
	if d, ok := oauthpkg.Descriptor(provider); ok {
		return d.Name, d.Icon
	}
	return provider, ""
}

// sanitizeEmailPart 把 openid 规整成可放进用户名/邮箱的片段。
func sanitizeEmailPart(openid string) string {
	var b strings.Builder
	for _, r := range openid {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "user"
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}
