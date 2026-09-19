package server

// 第三方登录装配（doc104 §6）。
//
// 依赖方向：uc/oauth 只依赖自己声明的端口（UserRepo / InviteBinder /
// DefaultGroupResolver / ConfigReader），本文件负责用 uc/auth 的模型与数据库
// 把这些端口实现出来。oauth 包不 import uc/auth，uc/auth 也不 import oauth。

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"strings"

	authmodel "hostsent/backend/internal/modules/uc/auth/model"
	oauthhandler "hostsent/backend/internal/modules/uc/oauth/handler"
	oauthrepo "hostsent/backend/internal/modules/uc/oauth/repository"
	oauthservice "hostsent/backend/internal/modules/uc/oauth/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/config"
	oauthpkg "hostsent/backend/internal/pkg/oauth"
)

// oauthBundle 第三方登录处理器集合。
type oauthBundle struct {
	adminHandler *oauthhandler.AdminHandler
	userHandler  *oauthhandler.UserHandler
}

// buildOAuthBundle 装配第三方登录。
//
// callbackBase 为后端回调基地址（未在系统配置里填 oauth.callback_base 时的兜底）。
// frontendCallback 为前端回跳地址（未填 oauth.frontend_callback 时的兜底）。
func buildOAuthBundle(
	cfg *config.Config,
	db *gorm.DB,
	cacheClient *cache.Client,
	jwtIssuer *appauth.JWTIssuer,
	configReader oauthservice.ConfigReader,
	inviteBinder oauthservice.InviteBinder,
	defaultGroup oauthservice.DefaultGroupResolver,
	callbackBase, frontendCallback string,
	logger *zap.Logger,
) *oauthBundle {
	repo := oauthrepo.NewRepository(db)
	svc := oauthservice.New(oauthservice.Deps{
		Repo:      repo,
		Users:     newOAuthUserRepo(db),
		Signer:    oauthpkg.NewStateSigner(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer),
		JWTIssuer: jwtIssuer,
		Config:    configReader,
		Cache:     cacheClient,
		// 凭证加密与支付/验证码/通知渠道同源密钥：轮换时一并轮换。
		EncryptKey:       cfg.App.EncryptKey,
		Logger:           logger,
		CallbackBase:     callbackBase,
		FrontendCallback: frontendCallback,
		DB:               db,
	})
	svc.SetInviteBinder(inviteBinder)
	svc.SetDefaultGroupResolver(defaultGroup)
	return &oauthBundle{
		adminHandler: oauthhandler.NewAdminHandler(svc),
		userHandler:  oauthhandler.NewUserHandler(svc),
	}
}

// ---------- UserRepo 适配器 ----------

// oauthUserRepo 用 uc/auth 的 users 模型实现 oauth 的用户端口。
//
// 为什么不复用 uc/auth 的 repository.UserRepository：那份接口面向「用户自助」
// 场景（登录、改资料、改密码），没有「按 openid 建号」「统计剩余登录方式」这类
// 语义；硬塞进去会让 uc/auth 的仓储接口长出与它无关的方法。
type oauthUserRepo struct{ db *gorm.DB }

func newOAuthUserRepo(db *gorm.DB) oauthservice.UserRepo { return &oauthUserRepo{db: db} }

func (r *oauthUserRepo) FindByID(ctx context.Context, id uint64) (*oauthservice.UserBrief, error) {
	var user authmodel.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return toUserBrief(&user), nil
}

func (r *oauthUserRepo) FindByUsername(ctx context.Context, username string) (*oauthservice.UserBrief, error) {
	var user authmodel.User
	// 软删除的账号不参与「用户名是否被占用」的判断：部分唯一索引只约束
	// deleted_at IS NULL 的行，这里必须同口径，否则会误判冲突。
	if err := r.db.WithContext(ctx).Where("username = ? AND deleted_at IS NULL", username).First(&user).Error; err != nil {
		return nil, err
	}
	return toUserBrief(&user), nil
}

func (r *oauthUserRepo) FindByEmail(ctx context.Context, email string) (*oauthservice.UserBrief, error) {
	var user authmodel.User
	if err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error; err != nil {
		return nil, err
	}
	return toUserBrief(&user), nil
}

// CreateOAuthUser 创建第三方登录自动注册用户。
//
// 列写全了 status/tier/user_group_id，因为 users 的 status 与 tier 都是
// NOT NULL DEFAULT，靠默认值虽然也能建出来，但显式写入让「第三方注册用户
// 的初始状态」在代码里可见，不依赖迁移里的默认值是否被改过。
func (r *oauthUserRepo) CreateOAuthUser(ctx context.Context, in oauthservice.CreateUserInput) (uint64, error) {
	user := &authmodel.User{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: in.PasswordHash,
		Status:       "active",
		Tier:         "free",
		UserGroupID:  in.UserGroupID,
	}
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return 0, err
	}
	return user.ID, nil
}

// CountLoginMethods 统计该用户「除 excludeProvider 之外」还剩多少种登录方式。
//
// 三类各算一种：可用密码（有 password_hash 且非空）、其他第三方绑定、
// 已验证手机号。自动注册用户写的是随机哈希，无法判断「是否可用」，
// 因此把「有 password_hash」一律视为一种方式 —— 用户可以走「忘记密码」重设，
// 保守估计比激进估计安全（激进估计会把账号锁死成谁都进不去）。
func (r *oauthUserRepo) CountLoginMethods(ctx context.Context, userID uint64, excludeProvider string) (int, error) {
	var user authmodel.User
	if err := r.db.WithContext(ctx).Select("id, password_hash, phone, phone_verified_at").
		First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	if strings.TrimSpace(user.PasswordHash) != "" {
		count++
	}
	if strings.TrimSpace(user.Phone) != "" && user.PhoneVerifiedAt != nil {
		count++
	}
	query := r.db.WithContext(ctx).Table("user_oauth_bindings").
		Where("user_id = ? AND status = ?", userID, "active")
	if provider := strings.TrimSpace(excludeProvider); provider != "" {
		query = query.Where("provider <> ?", provider)
	}
	var bindings int64
	if err := query.Count(&bindings).Error; err != nil {
		return 0, err
	}
	// 绑定算一种方式（多条绑定不重复计数）：判断的是「还有没有别的入口」。
	if bindings > 0 {
		count++
	}
	return count, nil
}

// toUserBrief 把 users 行投影成 oauth 侧的最小视图。
func toUserBrief(user *authmodel.User) *oauthservice.UserBrief {
	brief := &oauthservice.UserBrief{
		ID:              user.ID,
		Username:        user.Username,
		Email:           user.Email,
		RealName:        user.RealName,
		Avatar:          user.Avatar,
		Status:          user.Status,
		DeletedAt:       user.DeletedAt,
		IsSubAccount:    user.IsSubAccount,
		OwnerUserID:     user.OwnerUserID,
		PasswordHash:    user.PasswordHash,
		Phone:           user.Phone,
		PhoneVerifiedAt: user.PhoneVerifiedAt,
		UserGroupID:     user.UserGroupID,
		HasInviter:      user.InviterUserID != nil,
	}
	return brief
}

// oauthRouteGroup 注册第三方登录路由（管理端与用户端）。
//
// 单独成函数而不是塞进 router.go 的一大段：oauth 有「免登录回调」这一特殊
// 鉴权形态，集中在一处便于审查「哪几条真的不需要登录」。
func registerOAuthRoutes(admin *gin.RouterGroup, uc *gin.RouterGroup, bundle *oauthBundle, app *App) {
	if bundle == nil {
		return
	}
	oauthAdmin := admin.Group("/oauth")
	{
		oauthAdmin.GET("/provider-types", app.perm("oauth:config"), bundle.adminHandler.ProviderTypes)
		oauthAdmin.GET("/providers", app.perm("oauth:config"), bundle.adminHandler.ListProviders)
		oauthAdmin.PUT("/providers/:provider", app.perm("oauth:config"), bundle.adminHandler.UpdateProvider)
		oauthAdmin.POST("/providers/:provider/test", app.perm("oauth:config"), bundle.adminHandler.TestProvider)
		oauthAdmin.GET("/bindings", app.perm("oauth:binding:list"), bundle.adminHandler.ListBindings)
	}

	oauthUC := uc.Group("/oauth")
	{
		// 公开：登录页需要知道「哪几家可用」，不含任何凭证。
		oauthUC.GET("/providers", bundle.userHandler.Providers)
		oauthUC.GET("/:provider/authorize", bundle.userHandler.Authorize)
		// 免登录回调：用户在浏览器里被三方重定向过来，此时还没有平台令牌。
		// 信任来源是签名 state（校验 state 与渠道是否匹配、是否过期）。
		oauthUC.GET("/:provider/callback", bundle.userHandler.Callback)
		// ticket → 正式令牌。ticket 是回调后签发的 60 秒一次性票据。
		oauthUC.POST("/exchange", bundle.userHandler.Exchange)
		// 以下三条需要登录：绑定/解绑必须落到明确的账号。
		oauthUC.GET("/bindings", app.userAuth(), bundle.userHandler.MyBindings)
		oauthUC.GET("/:provider/bind-authorize", app.userAuth(), bundle.userHandler.BindAuthorize)
		oauthUC.DELETE("/bindings/:provider", app.userAuth(), bundle.userHandler.Unbind)
	}
}
