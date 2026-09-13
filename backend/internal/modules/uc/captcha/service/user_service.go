package service

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	"hostsent/backend/internal/modules/uc/captcha/model"
	captchapkg "hostsent/backend/internal/pkg/captcha"
)

// UserService 用户端安全设置服务（doc91 §4.4/§4.5）。
//
// 语义只有「加严」：用户设置永远不会让平台基线变松。请求里带关闭意图时
// 一律忽略，并回显真实生效值 + 说明（前端展示「已按平台安全要求使用 X 验证」）。
type UserService struct {
	svc      *Service
	repo     repositoryRepository
	policy   *PolicyService
	switches *Switches
	logger   *zap.Logger
}

// repositoryRepository 本文件用到的仓储能力子集。
type repositoryRepository interface {
	FindSettings(ctx context.Context, userID uint64) (*model.UserSecuritySettings, error)
	SaveSettings(ctx context.Context, s *model.UserSecuritySettings) error
	ListPolicies(ctx context.Context) ([]model.CaptchaPolicy, error)
	FindPolicyByScene(ctx context.Context, scene string) (*model.CaptchaPolicy, error)
}

// NewUserService 创建用户端服务。
func NewUserService(svc *Service, repo repositoryRepository, policy *PolicyService, switches *Switches) *UserService {
	if svc != nil && svc.logger != nil {
		return &UserService{svc: svc, repo: repo, policy: policy, switches: switches, logger: svc.logger}
	}
	return &UserService{svc: svc, repo: repo, policy: policy, switches: switches, logger: zap.NewNop()}
}

// GetSettings 读取用户安全设置（只回 effective 值）。
func (u *UserService) GetSettings(ctx context.Context, userID uint64) (*dto.SecuritySettingsResponse, error) {
	return u.buildResponse(ctx, userID, "")
}

// UpdateSettings 更新用户安全设置（只表达加严）。
func (u *UserService) UpdateSettings(ctx context.Context, userID uint64, req dto.SecuritySettingsUpdateRequest) (*dto.SecuritySettingsResponse, error) {
	st, err := u.repo.FindSettings(ctx, userID)
	if err != nil || st == nil {
		st = &model.UserSecuritySettings{UserID: userID}
	}
	notice := ""
	if req.MFAEnabled != nil {
		// 用户可自行开启强二次验证；关闭时若平台基线强制，则忽略并提示。
		if *req.MFAEnabled {
			st.MFAEnabled = true
		} else if u.platformForcesMFA(ctx) {
			notice = "平台已强制开启二次验证，无法关闭"
		} else {
			st.MFAEnabled = false
		}
	}
	if ch := normalizeChannel(req.MFAChannel); ch != "" {
		if model.ChannelLevel(ch) == 0 {
			notice = "不支持的验证通道：" + req.MFAChannel
		} else {
			st.MFAChannel = ch
		}
	}
	if req.SceneOverrides != nil {
		st.SceneOverrides = encodeOverrides(req.SceneOverrides)
	}
	if req.TrustWindowMinutes != nil && *req.TrustWindowMinutes >= 0 {
		st.TrustWindowMinutes = *req.TrustWindowMinutes
	}
	if err := u.repo.SaveSettings(ctx, st); err != nil {
		return nil, err
	}
	return u.buildResponse(ctx, userID, notice)
}

// SendVerification 已登录用户下发验证码（目标取账号绑定，前端传空即可）。
func (u *UserService) SendVerification(ctx context.Context, userID uint64, req dto.SendCodeRequest, ip, userAgent string) (*dto.SendCodeResponse, error) {
	// 目标缺省时由服务端按账号绑定补全，避免前端传错别人的手机号。
	if strings.TrimSpace(req.Target) == "" && u.svc.resolver != nil {
		info, err := u.svc.resolver.ResolveUser(ctx, userID, "")
		if err == nil && info != nil && info.Exists {
			if normalizeChannel(req.Channel) == model.ChannelSMS {
				req.Target = info.Phone
			} else {
				req.Target = info.Email
			}
		}
	}
	return u.svc.SendCode(ctx, req, Subject{ID: userID}, userID, ip, userAgent)
}

// VerifyCode 校验 OTP 并换取关键操作票据。
func (u *UserService) VerifyCode(ctx context.Context, userID uint64, req dto.VerifyCodeRequest) (*dto.VerifyCodeResponse, error) {
	return u.svc.VerifyCode(ctx, strings.TrimSpace(req.Scene), req.Code, Subject{ID: userID}, "")
}

// VerificationRequirement 查询某场景的验证要求（无票据时前端据此弹框）。
func (u *UserService) VerificationRequirement(ctx context.Context, userID uint64, scene string) (*dto.VerificationRequirementResponse, error) {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		scene = model.ScenePasswordChange
	}
	subject := Subject{ID: userID}
	eff, err := u.policy.EffectivePolicy(ctx, scene, subject)
	if err != nil {
		return nil, err
	}
	out := &dto.VerificationRequirementResponse{
		Scene:           scene,
		ImageRequired:   eff.ImageRequired,
		Channel:         eff.OTPChannel,
		VerifyTicketTTL: 300,
	}
	// 票据已存在则无需再次验证（前端据此直接执行）。
	out.TicketExists = u.svc.TicketExists(ctx, scene, subject)
	out.NeedVerification = eff.OTPRequired && !out.TicketExists

	base, _ := u.repo.FindPolicyByScene(ctx, scene)
	out.Channels = u.channelOptions(base, eff.OTPChannel)

	if u.svc.resolver != nil {
		if info, rerr := u.svc.resolver.ResolveUser(ctx, userID, ""); rerr == nil && info != nil {
			out.TargetMasked = maskChannelTarget(eff.OTPChannel, info.Phone, info.Email)
		}
	}
	return out, nil
}

// channelOptions 组装通道选项（按策略 min_channel_level 过滤，可用性看账号绑定）。
func (u *UserService) channelOptions(base *model.CaptchaPolicy, current string) []dto.ChannelOption {
	allowed := channelsForPolicy(base)
	if len(allowed) == 0 {
		return nil
	}
	if base != nil && !base.UserCanChooseChannel {
		// 平台不允许自选通道：只回当前生效通道，避免前端给出无意义选项。
		return []dto.ChannelOption{{
			Channel: current, Label: model.ChannelLabel(current), Level: model.ChannelLevel(current), Available: true,
		}}
	}
	out := make([]dto.ChannelOption, 0, len(allowed))
	for _, ch := range allowed {
		out = append(out, dto.ChannelOption{
			Channel:   ch,
			Label:     model.ChannelLabel(ch),
			Level:     model.ChannelLevel(ch),
			Available: true,
		})
	}
	return out
}

// buildResponse 组装安全设置响应（含逐场景 effective 策略）。
func (u *UserService) buildResponse(ctx context.Context, userID uint64, notice string) (*dto.SecuritySettingsResponse, error) {
	out := &dto.SecuritySettingsResponse{Notice: notice, Scenes: []dto.EffectiveScene{}}
	subject := Subject{ID: userID}

	if st, err := u.repo.FindSettings(ctx, userID); err == nil && st != nil {
		out.MFAEnabled = st.MFAEnabled
		out.MFAChannel = st.MFAChannel
		out.TrustWindowMinutes = st.TrustWindowMinutes
	}
	if u.svc.resolver != nil {
		if info, err := u.svc.resolver.ResolveUser(ctx, userID, ""); err == nil && info != nil {
			out.Phone = info.Phone
			out.PhoneMasked = maskTarget(info.Phone)
			out.PhoneBound = info.Phone != "" && info.PhoneVerified
			out.Email = info.Email
			out.EmailMasked = maskTarget(info.Email)
			out.EmailBound = info.Email != "" && info.EmailVerified
		}
	}

	// 逐场景解算；某项解算失败不整体失败（设置页必须能打开）。
	policies, err := u.repo.ListPolicies(ctx)
	if err != nil {
		return out, nil
	}
	for i := range policies {
		eff, perr := u.policy.EffectivePolicy(ctx, policies[i].Scene, subject)
		if perr != nil || eff == nil {
			continue
		}
		// 用户端只展示与「关键操作」相关的场景，登录/注册类由登录页自行处理。
		if isLoginScene(policies[i].Scene) {
			continue
		}
		eff.PlatformForced = policies[i].ImageRequired || policies[i].OTPRequired
		out.Scenes = append(out.Scenes, *eff)
	}
	return out, nil
}

// platformForcesMFA 判断平台是否强制二次验证（总闸 + 全局开关 + 任一场景基线）。
func (u *UserService) platformForcesMFA(ctx context.Context) bool {
	if u.switches == nil || !u.switches.CaptchaEnabled(ctx) {
		return false
	}
	if u.switches.MFARequired(ctx) {
		return true
	}
	policies, err := u.repo.ListPolicies(ctx)
	if err != nil {
		return false
	}
	for i := range policies {
		if policies[i].OTPRequired {
			return true
		}
	}
	return false
}

// isLoginScene 登录/注册/找回密码类场景（不在用户安全设置页展示）。
func isLoginScene(scene string) bool {
	switch scene {
	case model.SceneAdminLogin, model.SceneUserLogin, model.SceneUserLoginSMS,
		model.SceneUserLoginEmail, model.SceneUserRegister, model.ScenePasswordReset:
		return true
	default:
		return false
	}
}

// ThirdPartyParams 第三方图形码前端 SDK 初始化参数。
type ThirdPartyParams struct {
	Provider  string
	CaptchaID string
}

// ThirdPartyFrontendParams 返回第三方图形码的前端参数（native 时 ok=false）。
//
// captcha_id / site_key 属于「可公开」字段，明文回给前端是设计如此；
// secret 类字段永远不出现在响应里。
func (s *Service) ThirdPartyFrontendParams(ctx context.Context) (ThirdPartyParams, bool) {
	row, err := s.repo.FindDefaultProvider(ctx)
	if err != nil || row == nil || row.Status != 1 || row.ProviderType == providerNativeType {
		return ThirdPartyParams{}, false
	}
	if _, ok := captchapkg.Descriptor(row.ProviderType); !ok {
		return ThirdPartyParams{}, false
	}
	creds, err := s.decryptCredentials(row)
	if err != nil {
		// 解密失败按「无可公开参数」处理（L8：绝不回退明文）。
		s.logger.Error("third-party captcha credential decrypt failed",
			zap.Uint64("provider_id", row.ID), zap.Error(err))
		return ThirdPartyParams{}, false
	}
	id := creds["captcha_id"]
	if id == "" {
		id = creds["site_key"]
	}
	if id == "" {
		return ThirdPartyParams{}, false
	}
	return ThirdPartyParams{Provider: row.ProviderType, CaptchaID: id}, true
}
