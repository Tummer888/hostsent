package server

// 验证码体系装配（doc91 §3/§4/§5）：
//
//   - 公开接口：auth-config / 图形码 / OTP 下发（登录前）
//   - 管理端：服务商（类型/实例/测试）+ 场景策略 + 统计
//   - 用户端：安全设置 + 关键操作校验
//
// 依赖方向：captcha 模块不 import 任何业务模块；OTP 下发经 service.Sender
// 端口外接。doc91 阶段注入 directOTPSender（直读 system_configs 的 SMTP），
// doc90 落地渠道表后包一层 adapter.ChannelSender（渠道表优先、历史 SMTP 兜底），
// 接口与调用方代码不变。

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/uc/captcha/adapter"
	captchahandler "hostsent/backend/internal/modules/uc/captcha/handler"
	captcharepo "hostsent/backend/internal/modules/uc/captcha/repository"
	captchaservice "hostsent/backend/internal/modules/uc/captcha/service"
	appauth "hostsent/backend/internal/pkg/auth"
	"hostsent/backend/internal/pkg/cache"
	"hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/config"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/middleware"
	"hostsent/backend/internal/pkg/response"
	"hostsent/backend/internal/pkg/security"
)

// captchaBundle 验证码体系处理器集合。
type captchaBundle struct {
	publicHandler *captchahandler.PublicHandler
	adminHandler  *captchahandler.AdminHandler
	userHandler   *captchahandler.UserHandler
	service       *captchaservice.Service
	policy        *captchaservice.PolicyService
	loginGuard    *captchaservice.LoginGuard
	// port 中性安全端口：供 admin/manager 与 uc/auth 消费，避免 admin → uc 反向依赖。
	port security.Port
	// limiter 全局 API 限流（配置 api_rate_limit，默认 0=不限）。
	limiter *middleware.RateLimiter
	// userLimiter 用户端限流（同一实现，独立 scope 便于分别观测）。
	userLimiter *middleware.RateLimiter
}

// buildCaptchaBundle 装配验证码体系。
//
// configReader 读 system_configs 原文（键不存在返回 ok=false，不报错）。
// otpResolver 为消息中心渠道路由（doc90）；nil 时退回 doc91 的直读 SMTP 实现。
func buildCaptchaBundle(
	cfg *config.Config,
	db *gorm.DB,
	cacheClient *cache.Client,
	jwtIssuer *appauth.JWTIssuer,
	configReader captchaservice.ConfigValueReader,
	otpResolver adapter.ChannelResolver,
	logger *zap.Logger,
) *captchaBundle {
	repo := captcharepo.NewRepository(db)
	switches := captchaservice.NewSwitches(configSourceAdapter{read: configReader})
	policy := captchaservice.NewPolicyService(repo, switches)
	store := &captchaStore{cache: cacheClient}
	// doc91 §5.1 登记的切换点：OTP 下发端口由「直读 system_configs」升级为
	// 「渠道表优先、历史 SMTP 兜底」；Service 侧接口与调用代码零改动。
	var otpSender captchaservice.Sender = captchaservice.NewDirectOTPSender(db, configReader)
	if otpResolver != nil {
		otpSender = adapter.NewChannelSender(otpResolver, otpSender, configReader)
	}
	svc := captchaservice.NewService(captchaservice.Deps{
		Repo:      repo,
		Switches:  switches,
		Policy:    policy,
		Cache:     store,
		Sender:    otpSender,
		Resolver:  adapter.NewTargetResolver(db),
		JWTIssuer: jwtIssuer,
		// 频控 hash 的 pepper：与凭证加密同源密钥（部署轮换时一并轮换）。
		EncryptKey: cfg.App.EncryptKey,
		Logger:     logger,
	})
	adminSvc := captchaservice.NewAdminService(repo, switches, cfg.App.EncryptKey, store, logger)
	userSvc := captchaservice.NewUserService(svc, repo, policy, switches)
	guard := captchaservice.NewLoginGuard(switches, store, adapter.NewLoginLogRecorder(db))

	// api_rate_limit 挂 admin / uc 两组（0=不限，缓存不可用时降级放行并告警）。
	limitReader := func(ctx context.Context) int { return switches.APIRateLimit(ctx) }
	warn := func(path string) {
		logger.Warn("api rate limit degraded: cache unavailable", zap.String("path", path))
	}
	limiter := middleware.NewRateLimiter(middleware.RateLimitOptions{
		Counter: store, Limit: limitReader, Scope: "admin", Warn: warn,
	})
	userLimiter := middleware.NewRateLimiter(middleware.RateLimitOptions{
		Counter: store, Limit: limitReader, Scope: "uc", Warn: warn,
	})

	return &captchaBundle{
		publicHandler: captchahandler.NewPublicHandler(svc),
		adminHandler:  captchahandler.NewAdminHandler(adminSvc),
		userHandler:   captchahandler.NewUserHandler(userSvc),
		service:       svc,
		policy:        policy,
		loginGuard:    guard,
		port:          captchaservice.NewSecurityPort(svc, guard, jwtIssuer),
		limiter:       limiter,
		userLimiter:   userLimiter,
	}
}

// verifyTicket 关键操作二次验证中间件（doc91 C5）。
//
// 语义：策略要求 OTP 时，请求头必须带一次性 verify_ticket；校验通过即消费
// （同一票据不能重复使用）。策略不要求时直接放行（绝不因为验证码模块而阻断业务）。
func (b *captchaBundle) verifyTicket(scene string, isAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if b == nil || b.service == nil {
			c.Next()
			return
		}
		subject := captchaservice.Subject{IsAdmin: isAdmin}
		if isAdmin {
			if claims, ok := middleware.GetAdminClaims(c); ok {
				subject.ID = claims.AdminID
			}
		} else if claims, ok := middleware.GetUserClaims(c); ok {
			subject.ID = claims.UserID
		}
		if subject.ID == 0 {
			c.Next()
			return
		}
		_, otpRequired, _ := b.policy.Required(c.Request.Context(), scene, subject)
		if !otpRequired {
			c.Next()
			return
		}
		ticket := strings.TrimSpace(c.GetHeader("X-Verify-Ticket"))
		if ticket == "" {
			// 兼容 body 传参（部分前端表单无法设置自定义头）。
			ticket = strings.TrimSpace(c.Query("verify_ticket"))
		}
		if !b.service.ConsumeVerifyTicket(c.Request.Context(), scene, subject, ticket) {
			response.ErrorWithStatus(c, http.StatusForbidden,
				apperrors.New(captchaservice.CodeNeedVerification, captchaservice.ErrNeedVerification.Error()))
			c.Abort()
			return
		}
		c.Next()
	}
}

// captchaStore 适配 cache.Client 到验证码模块需要的存储端口。
type captchaStore struct {
	cache *cache.Client
}

// Enabled 报告缓存是否可用（false 时服务层统一降级放行，D11）。
func (s *captchaStore) Enabled() bool { return s.cache != nil && s.cache.Enabled() }

func (s *captchaStore) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	if s.cache == nil {
		return errors.New("captcha: cache unavailable")
	}
	return s.cache.Set(ctx, key, val, ttl)
}

func (s *captchaStore) SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error) {
	if s.cache == nil {
		return false, errors.New("captcha: cache unavailable")
	}
	return s.cache.SetNX(ctx, key, val, ttl)
}

func (s *captchaStore) Get(ctx context.Context, key string) (string, bool) {
	if s.cache == nil {
		return "", false
	}
	return s.cache.Get(ctx, key)
}

func (s *captchaStore) GetDel(ctx context.Context, key string) (string, bool) {
	if s.cache == nil {
		return "", false
	}
	return s.cache.GetDel(ctx, key)
}

func (s *captchaStore) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if s.cache == nil {
		return 0, errors.New("captcha: cache unavailable")
	}
	return s.cache.Incr(ctx, key, ttl)
}

func (s *captchaStore) Del(ctx context.Context, keys ...string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Del(ctx, keys...)
}

func (s *captchaStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	if s.cache == nil {
		return 0, errors.New("captcha: cache unavailable")
	}
	return s.cache.TTL(ctx, key)
}

func (s *captchaStore) WarnDegraded(scene, path string) {
	if s.cache == nil {
		return
	}
	s.cache.WarnDegraded(scene, path)
}

// captchaStore 同时满足 captcha.Store（图形码答案存取）。
var _ captcha.Store = (*captchaStore)(nil)

// configSourceAdapter 把「读一个键」的函数适配为 Switches 需要的配置源。
//
// 逐键读取：读失败的键留空（该键按默认值走），其余键仍然生效；绝不整体返回
// 空表，否则会表现为「配置了也不生效」。
type configSourceAdapter struct {
	read captchaservice.ConfigValueReader
}

// All 载入开关表用到的全部键。
func (a configSourceAdapter) All(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(captchaservice.SwitchKeys()))
	if a.read == nil {
		return out, nil
	}
	for _, key := range captchaservice.SwitchKeys() {
		if v, ok, err := a.read(ctx, key); err == nil && ok {
			out[key] = v
		}
	}
	return out, nil
}
