package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	"hostsent/backend/internal/modules/uc/captcha/model"
	"hostsent/backend/internal/modules/uc/captcha/repository"
	appauth "hostsent/backend/internal/pkg/auth"
	captchapkg "hostsent/backend/internal/pkg/captcha"
	"hostsent/backend/internal/pkg/credentials"
)

// 错误码（doc91 §3.4/§4.1/§4.5）：与前端约定一致。
const (
	CodeCaptchaInvalid   = 20010 // 验证码错误或已过期（不区分两者）
	CodeSendTooFrequent  = 20011 // 发送过于频繁
	CodeSendDailyLimit   = 20012 // 超出每日/每小时上限
	CodeImageRateLimited = 20013 // 图形码下发频率超限（HTTP 429）
	CodeSceneDisabled    = 20001 // 场景策略不可用/参数错误
	CodeNeedVerification = 20017 // 关键操作需要二次验证
	CodeTargetUnbound    = 20015 // 账号未绑定手机/邮箱
)

// ErrNeedVerification 关键操作缺少/票据无效。
var ErrNeedVerification = errors.New("该操作需要二次验证")

// ErrSceneDisabled 场景策略不可用。
var ErrSceneDisabled = errors.New("该场景验证策略不可用")

// ErrTargetUnbound 目标未绑定。
var ErrTargetUnbound = errors.New("账号未绑定对应的手机号或邮箱")

// ErrRateLimited 频控命中。
type ErrRateLimited struct {
	RetryAfter int
	Daily      bool
}

func (e *ErrRateLimited) Error() string {
	if e.Daily {
		return "今日验证码发送次数已达上限，请稍后再试"
	}
	return fmt.Sprintf("发送过于频繁，请 %d 秒后重试", e.RetryAfter)
}

// TargetInfo 主体的 OTP 目标信息。
type TargetInfo struct {
	// UserID 命中用户的 ID（按账号解析时填充；按 ID 解析时即该 ID）。
	UserID        uint64
	Phone         string
	PhoneVerified bool
	Email         string
	EmailVerified bool
	Username      string
	Exists        bool
}

// TargetResolver 解析主体的 OTP 目标（装配层用数据库查询实现）。
type TargetResolver interface {
	// ResolveUser 按用户 ID 或账号（用户名/邮箱/手机号）解析目标；找不到返回 Exists=false。
	ResolveUser(ctx context.Context, userID uint64, account string) (*TargetInfo, error)
	// ResolveAdmin 按管理员 ID 或用户名解析目标。
	ResolveAdmin(ctx context.Context, adminID uint64, username string) (*TargetInfo, error)
	// SetVerifiedAt 写回绑定验证时间（注册/绑定成功后调用）。
	SetVerifiedAt(ctx context.Context, userID uint64, channel string, at time.Time) error
}

// Sender 验证码下发端口（doc91 §5.1 的唯一切换点）。
//
// doc91 阶段由 directOTPSender 实现（直接读 system_configs 的 SMTP 配置）；
// doc90 落地渠道表后由 adapter.ChannelSender 包装（渠道表优先、历史 SMTP 兜底），
// **接口不变、调用方代码不改**。
type Sender interface {
	// SendMail 下发邮件（HTML 或纯文本由实现决定）。
	SendMail(ctx context.Context, to, subject, body string) error
	// SendSMS 下发短信；未配置短信渠道时返回 notifier 侧明确错误。
	SendSMS(ctx context.Context, target, content string) error
}

// Service 验证码与二次验证核心服务。
type Service struct {
	repo      repository.Repository
	switches  *Switches
	policy    *PolicyService
	cache     CachePort
	sender    Sender
	resolver  TargetResolver
	jwtIssuer *appauth.JWTIssuer
	pepper    string
	logger    *zap.Logger
}

// CachePort 本模块需要的缓存能力子集（由 cache.Client 满足）。
type CachePort interface {
	Enabled() bool
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, bool)
	GetDel(ctx context.Context, key string) (string, bool)
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Del(ctx context.Context, keys ...string) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	WarnDegraded(scene, path string)
}

// Deps 服务依赖。
type Deps struct {
	Repo     repository.Repository
	Switches *Switches
	Policy   *PolicyService
	Cache    CachePort
	Sender   Sender
	Resolver TargetResolver
	// JWTIssuer 签发登录二次验证待验证令牌（doc91 §4.6）。
	JWTIssuer *appauth.JWTIssuer
	// EncryptKey 凭证解密密钥（app.encrypt_key）。
	EncryptKey string
	Logger     *zap.Logger
}

// NewService 创建服务。
func NewService(deps Deps) *Service {
	if deps.Logger == nil {
		deps.Logger = zap.NewNop()
	}
	return &Service{
		repo:      deps.Repo,
		switches:  deps.Switches,
		policy:    deps.Policy,
		cache:     deps.Cache,
		sender:    deps.Sender,
		resolver:  deps.Resolver,
		jwtIssuer: deps.JWTIssuer,
		pepper:    deps.EncryptKey,
		logger:    deps.Logger,
	}
}

// Policy 暴露策略解算器（登录链路需要）。
func (s *Service) Policy() *PolicyService { return s.policy }

// Switches 暴露开关读取器（登录链路需要）。
func (s *Service) Switches() *Switches { return s.switches }

// ---------------------------------------------------------------------------
// 图形验证码
// ---------------------------------------------------------------------------

// ImageChallenge 生成图形码挑战。
//
// 频控：单 IP 每分钟上限（降级为放行但记录，doc91 §3.3）。
func (s *Service) ImageChallenge(ctx context.Context, scene, level, ip string) (*dto.ImageChallengeResponse, error) {
	if err := s.checkImageRate(ctx, ip); err != nil {
		return nil, err
	}
	pc, prov, err := s.resolveProvider(ctx, scene, level)
	if err != nil {
		return nil, err
	}
	challenge, err := prov.Challenge(ctx, pc, scene)
	if err != nil {
		// 第三方失败：回落 native（兜底必须存在，doc91 §7.4）。
		s.logger.Warn("captcha provider challenge failed, falling back to native",
			zap.String("scene", scene), zap.String("provider", pc.Type), zap.Error(err))
		_ = s.markProviderDown(ctx, pc.ProviderID, err)
		pc.Type = providerNativeType
		native, nerr := captchapkg.New(providerNativeType, pc)
		if nerr != nil {
			return nil, nerr
		}
		challenge, err = native.Challenge(ctx, pc, scene)
		if err != nil {
			return nil, err
		}
	}
	return &dto.ImageChallengeResponse{
		CaptchaKey:  challenge.CaptchaKey,
		ImageBase64: captchapkg.EncodeImageDataURI(challenge.ImagePNG),
		ExpireIn:    int(pc.ImageTTL / time.Second),
		Provider:    challenge.Provider,
		Params:      challenge.Params,
	}, nil
}

// VerifyImage 校验图形码；成功即销毁（一次性），失败累计。
//
// 必须走与 ImageChallenge **同一个** provider：第三方图形码的答案不在本地
// （本地只存 challenge key），用 native 校验第三方挑战必然永远失败。
//
// Redis 降级（Enabled()==false）时按 D11 放行：宁可放行也不阻断登录，
// 但必须打一次 Warn 供事后取证（同一 scene 每分钟至多一条）。
func (s *Service) VerifyImage(ctx context.Context, scene, key, code string) error {
	if s.cache == nil || !s.cache.Enabled() {
		s.cache.WarnDegraded(scene, "captcha.VerifyImage")
		return nil
	}
	if strings.TrimSpace(key) == "" || strings.TrimSpace(code) == "" {
		return captchapkg.ErrInvalid
	}
	pc, prov, err := s.resolveProvider(ctx, scene, "")
	if err != nil {
		return err
	}
	payload := map[string]string{
		"captcha_key":  key,
		"captcha_code": code,
		// 第三方 SDK 回传的票据字段名（易盾/极验均用 validate）。
		"validate": code,
		"user":     key,
	}
	if verr := prov.Verify(ctx, pc, scene, payload); verr != nil {
		// 第三方校验通道故障（非「答案错误」）时回落 native 再试一次：
		// 否则服务商一抖动，所有人都登不进来。ErrInvalid 表示答案确实不对，
		// 此时不能回落（回落也只会再错一次，且会掩盖真实失败）。
		if !errors.Is(verr, captchapkg.ErrInvalid) && pc.Type != providerNativeType {
			s.logger.Warn("captcha provider verify failed, falling back to native",
				zap.String("scene", scene), zap.String("provider", pc.Type), zap.Error(verr))
			_ = s.markProviderDown(ctx, pc.ProviderID, verr)
			pc.Type = providerNativeType
			native, nerr := captchapkg.New(providerNativeType, pc)
			if nerr != nil {
				return verr
			}
			return native.Verify(ctx, pc, scene, payload)
		}
		return verr
	}
	return nil
}

// checkImageRate IP 级图形码下发频控。
func (s *Service) checkImageRate(ctx context.Context, ip string) error {
	if s.cache == nil || !s.cache.Enabled() {
		s.cache.WarnDegraded("image_rate", "captcha.checkImageRate")
		return nil
	}
	limit := s.switches.ImageIPMinuteLimit(ctx)
	if limit <= 0 || ip == "" {
		return nil
	}
	n, err := s.cache.Incr(ctx, "cap:send:ip:img:"+ip, time.Minute)
	if err != nil {
		return nil
	}
	if n > int64(limit) {
		return &ErrRateLimited{RetryAfter: 60}
	}
	return nil
}

// ---------------------------------------------------------------------------
// OTP 下发
// ---------------------------------------------------------------------------

// SendCode 下发 OTP（执行顺序不可调换，doc91 §4.1）。
func (s *Service) SendCode(ctx context.Context, req dto.SendCodeRequest, subject Subject, operatorID uint64, ip, userAgent string) (*dto.SendCodeResponse, error) {
	scene := strings.TrimSpace(req.Scene)
	channel := normalizeChannel(req.Channel)
	if scene == "" || (channel != model.ChannelEmail && channel != model.ChannelSMS) {
		return nil, ErrSceneDisabled
	}

	// ① 场景策略：不存在/停用直接拒绝。
	base, err := s.repo.FindPolicyByScene(ctx, scene)
	if err != nil || base == nil || base.Status != model.StatusActive {
		return nil, ErrSceneDisabled
	}
	// ② 先验图形码（该场景 image_required 时）：防脚本批量刷短信。
	// 调用方已验过则跳过（SkipCaptcha）：图形码一次性，再验必然失败。
	eff, err := s.policy.EffectivePolicy(ctx, scene, subject)
	if err != nil {
		return nil, err
	}
	if eff.ImageRequired && !req.SkipCaptcha {
		if err := s.VerifyImage(ctx, scene, req.CaptchaKey, req.CaptchaCode); err != nil {
			return nil, err
		}
	}
	// ③ 目标归一化与校验。
	target, err := s.normalizeTarget(channel, req.Target)
	if err != nil {
		return nil, err
	}
	targetHash := hashTarget(s.pepper, target)
	// ④ 三重频控。
	if err := s.checkSendFrequency(ctx, scene, targetHash, ip, base); err != nil {
		return nil, err
	}
	// ⑤ 生成 6 位数字验证码（crypto/rand，首位不为 0）。
	code := randomNumericCode(6)
	salt := randomSalt()
	ttl := s.codeTTL(ctx, base)
	// ⑥ 落 verification_codes（hash + salt；绝不落明文）。
	row := &model.VerificationCode{
		Scene:       scene,
		Channel:     channel,
		Target:      target,
		TargetHash:  targetHash,
		CodeHash:    hashCode(code, salt),
		CodeSalt:    salt,
		Status:      model.CodeStatusPending,
		MaxAttempts: s.attempts(ctx, base),
		RequestIP:   ip,
		UserAgent:   truncate(userAgent, 255),
		OperatorID:  operatorID,
		ExpireAt:    time.Now().Add(ttl),
	}
	if err := s.repo.CreateCode(ctx, row); err != nil {
		return nil, err
	}
	// 写 Redis：key 与校验次数计数（Redis 不可用时校验走 DB 降级）。
	if s.cache != nil && s.cache.Enabled() {
		_ = s.cache.Set(ctx, otpKey(scene, targetHash), code, ttl)
		_ = s.cache.Del(ctx, otpTryKey(scene, targetHash))
	}
	// ⑦ 通过 notifier 下发（IsMandatoryEvent 恒真，不走偏好过滤）。
	body := renderOTPBody(base, code, int(ttl/time.Minute))
	if err := s.deliver(ctx, channel, target, body); err != nil {
		_ = s.repo.MarkCodeUsed(ctx, row.ID)
		return nil, err
	}
	// ⑧ 日志脱敏：绝不把 code 写进日志（doc91 §4.1 第 8 条）。
	s.logger.Info("verification code sent",
		zap.String("scene", scene), zap.String("channel", channel),
		zap.String("target", maskTarget(target)), zap.Uint64("operator_id", operatorID))
	// ⑨ 返回打码值。
	return &dto.SendCodeResponse{
		Sent:         true,
		Channel:      channel,
		TargetMasked: maskTarget(target),
		ExpireIn:     int(ttl / time.Second),
		Cooldown:     int(s.sendInterval(ctx, base) / time.Second),
	}, nil
}

// deliver 按通道下发。
func (s *Service) deliver(ctx context.Context, channel, target, body string) error {
	if s.sender == nil {
		return errors.New("验证码发送通道未配置")
	}
	if channel == model.ChannelSMS {
		return s.sender.SendSMS(ctx, target, body)
	}
	return s.sender.SendMail(ctx, target, "HostSent 验证码", body)
}

// normalizeTarget 按通道归一化并校验目标。
func (s *Service) normalizeTarget(channel, raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", errors.New("接收目标不能为空")
	}
	if channel == model.ChannelEmail {
		if !validEmail(target) {
			return "", errors.New("邮箱格式不正确")
		}
		return strings.ToLower(target), nil
	}
	normalized := normalizePhone(target)
	if !validCNMobile(normalized) {
		return "", errors.New("手机号格式不正确")
	}
	return normalized, nil
}

// checkSendFrequency 三重频控：发送间隔锁 / 单目标日限 / 单 IP 小时限。
func (s *Service) checkSendFrequency(ctx context.Context, scene, targetHash, ip string, base *model.CaptchaPolicy) error {
	interval := s.sendInterval(ctx, base)
	dailyLimit := s.dailyLimit(ctx, base)

	if s.cache != nil && s.cache.Enabled() {
		// 间隔锁：SETNX 成功才允许发；已存在则回剩余秒数。
		ok, err := s.cache.SetNX(ctx, sendGapKey(scene, targetHash), "1", interval)
		if err == nil && !ok {
			retry := int(interval / time.Second)
			if ttl, terr := s.cache.TTL(ctx, sendGapKey(scene, targetHash)); terr == nil && ttl > 0 {
				retry = int(ttl / time.Second)
			}
			return &ErrRateLimited{RetryAfter: retry}
		}
		dayKey := sendDayKey(scene, targetHash)
		ttl := untilMidnight()
		n, err := s.cache.Incr(ctx, dayKey, ttl)
		if err == nil && dailyLimit > 0 && n > int64(dailyLimit) {
			return &ErrRateLimited{Daily: true}
		}
		if ip != "" {
			ipLimit := s.switches.SendIPHourlyLimit(ctx)
			ipN, ierr := s.cache.Incr(ctx, "cap:send:ip:otp:"+ip, time.Hour)
			if ierr == nil && ipLimit > 0 && ipN > int64(ipLimit) {
				return &ErrRateLimited{Daily: true}
			}
		}
		return nil
	}
	// 降级：查 verification_codes 计数（doc91 §2.4 频控行）。
	s.cache.WarnDegraded(scene, "captcha.checkSendFrequency")
	if interval > 0 {
		if last, err := s.repo.FindLatestPending(ctx, scene, targetHash); err == nil && last != nil {
			if since := time.Since(last.CreatedAt); since < interval {
				return &ErrRateLimited{RetryAfter: int((interval - since) / time.Second)}
			}
		}
	}
	if dailyLimit > 0 {
		n, err := s.repo.CountSentSince(ctx, scene, targetHash, startOfDay())
		if err == nil && n >= int64(dailyLimit) {
			return &ErrRateLimited{Daily: true}
		}
	}
	if ip != "" {
		ipLimit := s.switches.SendIPHourlyLimit(ctx)
		if ipLimit > 0 {
			n, err := s.repo.CountSentSince(ctx, scene, hashTarget(s.pepper, ip), time.Now().Add(-time.Hour))
			if err == nil && n >= int64(ipLimit) {
				return &ErrRateLimited{Daily: true}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// OTP 校验
// ---------------------------------------------------------------------------

// VerifyCode 校验 OTP（用户已登录，用于关键操作），成功发放 verify ticket。
func (s *Service) VerifyCode(ctx context.Context, scene, code string, subject Subject, targetOverride string) (*dto.VerifyCodeResponse, error) {
	target, err := s.resolveTargetFor(ctx, scene, subject, targetOverride)
	if err != nil {
		return nil, err
	}
	if err := s.consumeCode(ctx, scene, target, code); err != nil {
		return nil, err
	}
	ticket := randomTicket()
	ttl := 300 * time.Second
	if s.cache != nil {
		if err := s.cache.Set(ctx, ticketKey(subject, scene), ticket, ttl); err != nil {
			return nil, err
		}
	}
	return &dto.VerifyCodeResponse{VerifyTicket: ticket, ExpireIn: int(ttl / time.Second)}, nil
}

// VerifyCodeForTarget 校验指定目标的 OTP（登录短信/邮箱、找回密码等登录前场景）。
func (s *Service) VerifyCodeForTarget(ctx context.Context, scene, target, code string) error {
	return s.consumeCode(ctx, scene, target, code)
}

// consumeCode 执行校验（Redis 优先，取不到降级查库）。
func (s *Service) consumeCode(ctx context.Context, scene, target, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return captchapkg.ErrInvalid
	}
	targetHash := hashTarget(s.pepper, target)
	if s.cache != nil && s.cache.Enabled() {
		stored, ok := s.cache.GetDel(ctx, otpKey(scene, targetHash))
		if ok {
			if !constantTimeEqual(stored, code) {
				n, _ := s.cache.Incr(ctx, otpTryKey(scene, targetHash), time.Minute*10)
				if n >= int64(s.switches.MaxAttempts(ctx)) {
					_ = s.cache.Del(ctx, otpKey(scene, targetHash))
					return captchapkg.ErrTooManyAttempts
				}
				return captchapkg.ErrInvalid
			}
			s.markCodeUsedByHash(ctx, scene, targetHash)
			return nil
		}
		// Redis 未命中：可能是重启丢 key 或写库失败，继续走 DB 降级。
	}
	s.cache.WarnDegraded(scene, "captcha.consumeCode")
	row, err := s.repo.FindLatestPending(ctx, scene, targetHash)
	if err != nil || row == nil {
		return captchapkg.ErrInvalid
	}
	if row.Attempts >= row.MaxAttempts {
		_ = s.repo.MarkCodeUsed(ctx, row.ID)
		return captchapkg.ErrTooManyAttempts
	}
	if hashCode(code, row.CodeSalt) != row.CodeHash {
		_ = s.repo.IncrCodeAttempts(ctx, row.ID)
		if row.Attempts+1 >= row.MaxAttempts {
			_ = s.repo.MarkCodeUsed(ctx, row.ID)
			return captchapkg.ErrTooManyAttempts
		}
		return captchapkg.ErrInvalid
	}
	return s.repo.MarkCodeUsed(ctx, row.ID)
}

// markCodeUsedByHash 把该 (scene, target_hash) 的最新 pending 行标记为已用。
func (s *Service) markCodeUsedByHash(ctx context.Context, scene, targetHash string) {
	row, err := s.repo.FindLatestPending(ctx, scene, targetHash)
	if err == nil && row != nil {
		_ = s.repo.MarkCodeUsed(ctx, row.ID)
	}
}

// verifyTicket 校验并消费关键操作票据（一次性）。
func (s *Service) ConsumeTicket(ctx context.Context, scene string, subject Subject) bool {
	if s.cache == nil {
		return false
	}
	if !s.cache.Enabled() {
		// 降级：票据无法校验。关键操作依赖票据的语义会失效，
		// 此时按「不阻断」处理并记录（与图形码降级放行同口径，D11）。
		s.cache.WarnDegraded(scene, "captcha.ConsumeTicket")
		return true
	}
	_, ok := s.cache.GetDel(ctx, ticketKey(subject, scene))
	return ok
}

// CheckTicket 只读检查票据是否存在（中间件用；消费在 Execute 前由中间件完成）。
func (s *Service) TicketExists(ctx context.Context, scene string, subject Subject) bool {
	if s.cache == nil {
		return false
	}
	if !s.cache.Enabled() {
		s.cache.WarnDegraded(scene, "captcha.TicketExists")
		return true
	}
	_, ok := s.cache.Get(ctx, ticketKey(subject, scene))
	return ok
}

// ConsumeVerifyTicket 校验并消费关键操作票据（一次性，须与发放值一致）。
//
// 与 ConsumeTicket 的区别：本方法比对票据内容，避免任意非空字符串都能通过。
// Redis 降级时按 D11 放行（该场景已在别处打过告警），保证降级不阻断业务。
func (s *Service) ConsumeVerifyTicket(ctx context.Context, scene string, subject Subject, ticket string) bool {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return false
	}
	if s.cache == nil {
		return false
	}
	if !s.cache.Enabled() {
		s.cache.WarnDegraded(scene, "captcha.ConsumeVerifyTicket")
		return true
	}
	stored, ok := s.cache.GetDel(ctx, ticketKey(subject, scene))
	return ok && constantTimeEqual(stored, ticket)
}

// resolveTargetFor 解析校验目标：显式 target > 用户设置里的通道目标。
func (s *Service) resolveTargetFor(ctx context.Context, scene string, subject Subject, override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	if s.resolver == nil {
		return "", ErrTargetUnbound
	}
	var info *TargetInfo
	var err error
	if subject.IsAdmin {
		info, err = s.resolver.ResolveAdmin(ctx, subject.ID, "")
	} else {
		info, err = s.resolver.ResolveUser(ctx, subject.ID, "")
	}
	if err != nil || info == nil || !info.Exists {
		return "", ErrTargetUnbound
	}
	eff, _ := s.policy.EffectivePolicy(ctx, scene, subject)
	channel := model.ChannelEmail
	if eff != nil && eff.OTPChannel != "" {
		channel = eff.OTPChannel
	}
	if channel == model.ChannelSMS {
		if info.Phone == "" {
			return "", ErrTargetUnbound
		}
		return info.Phone, nil
	}
	if info.Email == "" {
		return "", ErrTargetUnbound
	}
	return info.Email, nil
}

// resolveProvider 解析图形码服务商：策略指定 → 默认 provider → native 兜底。
func (s *Service) resolveProvider(ctx context.Context, scene, level string) (captchapkg.ProviderConfig, captchapkg.Provider, error) {
	cfg := captchapkg.ProviderConfig{
		Type:        providerNativeType,
		Store:       s.cache,
		Level:       captchapkg.NormalizeLevel(level),
		ImageTTL:    s.switches.ImageTTL(ctx),
		MaxAttempts: maxInt(3, s.switches.MaxAttempts(ctx)),
	}
	var row *model.CaptchaProvider
	if base, err := s.repo.FindPolicyByScene(ctx, scene); err == nil && base != nil {
		cfg.Level = captchapkg.NormalizeLevel(base.ImageLevel)
		if base.ImageProviderID > 0 {
			if p, err := s.repo.FindProviderByID(ctx, base.ImageProviderID); err == nil {
				row = p
			}
		}
	}
	if row == nil {
		if p, err := s.repo.FindDefaultProvider(ctx); err == nil {
			row = p
		}
	}
	if row == nil || row.Status != 1 {
		// 无任何自定义 provider：native（总闸与策略仍生效）。
		if fallback := s.switches.CaptchaProvider(ctx); fallback != "" && fallback != providerNativeType {
			if _, ok := captchapkg.Descriptor(fallback); ok {
				cfg.Type = fallback
			}
		}
		prov, err := captchapkg.New(cfg.Type, cfg)
		if err != nil {
			cfg.Type = providerNativeType
			prov, err = captchapkg.New(cfg.Type, cfg)
			if err != nil {
				return cfg, nil, err
			}
		}
		return cfg, prov, nil
	}
	if !captchapkg.IsRegistered(row.ProviderType) {
		// 占位 provider 或未知类型：回落 native，不 500。
		s.logger.Warn("captcha provider not implemented, falling back to native",
			zap.String("provider_type", row.ProviderType), zap.String("scene", scene))
		prov, err := captchapkg.New(providerNativeType, cfg)
		if err != nil {
			return cfg, nil, err
		}
		return cfg, prov, nil
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		// 解密失败必须显式降级到 native（L8：绝不回退明文）。
		s.logger.Error("captcha provider credential decrypt failed, falling back to native",
			zap.Uint64("provider_id", row.ID), zap.Error(err))
		prov, nerr := captchapkg.New(providerNativeType, cfg)
		if nerr != nil {
			return cfg, nil, nerr
		}
		return cfg, prov, nil
	}
	cfg.ProviderID = row.ID
	cfg.Type = row.ProviderType
	cfg.Endpoint = row.Endpoint
	cfg.Credentials = creds
	prov, err := captchapkg.New(cfg.Type, cfg)
	if err != nil {
		prov, err = captchapkg.New(providerNativeType, cfg)
	}
	return cfg, prov, err
}

// decryptCredentials 解密服务商凭证（secret 字段 enc:v1: 密文）。
func (s *Service) decryptCredentials(row *model.CaptchaProvider) (map[string]string, error) {
	m, err := credentials.Decode(row.Credentials)
	if err != nil {
		return nil, err
	}
	plain, err := m.DecryptFields(s.pepper)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range plain {
		out[k] = v
	}
	return out, nil
}

// markProviderDown 标记服务商异常（回落 native 时留痕，管理页展示告警条）。
func (s *Service) markProviderDown(ctx context.Context, providerID uint64, cause error) error {
	if providerID == 0 {
		return nil
	}
	row, err := s.repo.FindProviderByID(ctx, providerID)
	if err != nil || row == nil {
		return err
	}
	row.HealthStatus = model.HealthDown
	row.LastError = truncate(cause.Error(), 500)
	now := time.Now()
	row.LastCheckAt = &now
	return s.repo.UpdateProvider(ctx, row)
}

// providerNativeType native 类型常量（避免各处硬编码字符串）。
const providerNativeType = "native"

func (s *Service) codeTTL(ctx context.Context, base *model.CaptchaPolicy) time.Duration {
	if base != nil && base.TTLSeconds > 0 {
		return time.Duration(base.TTLSeconds) * time.Second
	}
	return s.switches.VerifyCodeTTL(ctx)
}

func (s *Service) sendInterval(ctx context.Context, base *model.CaptchaPolicy) time.Duration {
	if base != nil && base.SendIntervalSeconds > 0 {
		return time.Duration(base.SendIntervalSeconds) * time.Second
	}
	return s.switches.SendInterval(ctx)
}

func (s *Service) dailyLimit(ctx context.Context, base *model.CaptchaPolicy) int {
	if base != nil && base.DailyLimitPerTarget > 0 {
		return base.DailyLimitPerTarget
	}
	return s.switches.DailyLimit(ctx)
}

func (s *Service) attempts(ctx context.Context, base *model.CaptchaPolicy) int {
	if base != nil && base.MaxAttempts > 0 {
		return base.MaxAttempts
	}
	return s.switches.MaxAttempts(ctx)
}

// renderOTPBody 渲染验证码正文（短信短、邮件带说明）。
func renderOTPBody(base *model.CaptchaPolicy, code string, minutes int) string {
	if minutes <= 0 {
		minutes = 5
	}
	name := "身份验证"
	if base != nil && base.Name != "" {
		name = base.Name
	}
	return fmt.Sprintf("【HostSent】您正在%s，验证码为 %s，%d 分钟内有效。请勿将验证码告知他人。", name, code, minutes)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func untilMidnight() time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	return next.Sub(now)
}

func startOfDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
