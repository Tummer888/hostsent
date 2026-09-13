package service

import (
	"context"
	"strings"
	"time"

	"hostsent/backend/internal/modules/uc/captcha/dto"
	"hostsent/backend/internal/modules/uc/captcha/model"
	"hostsent/backend/internal/modules/uc/captcha/repository"
	cachepkg "hostsent/backend/internal/pkg/cache"
	captchapkg "hostsent/backend/internal/pkg/captcha"
)

// Subject 策略解算的主体（用户或管理员）。
type Subject struct {
	// IsAdmin true 时设置读 admins 表扩展列，否则读 user_security_settings。
	IsAdmin bool
	ID      uint64
}

// PolicyService 策略解算（D4 核心）。平台基线是地板，用户设置只能加严或换更强通道。
type PolicyService struct {
	repo     repository.Repository
	switches *Switches
}

// NewPolicyService 创建策略服务。
func NewPolicyService(repo repository.Repository, switches *Switches) *PolicyService {
	return &PolicyService{repo: repo, switches: switches}
}

// EffectivePolicy 计算某主体在某场景的最终策略。
func (s *PolicyService) EffectivePolicy(ctx context.Context, scene string, subject Subject) (*dto.EffectiveScene, error) {
	base, err := s.repo.FindPolicyByScene(ctx, scene)
	if err != nil || base == nil {
		// 无策略行视为「不需要验证」：登录链路绝不因为查不到策略而失败。
		return &dto.EffectiveScene{Scene: scene, Source: "platform"}, nil
	}
	eff := &dto.EffectiveScene{
		Scene:           base.Scene,
		Name:            base.Name,
		ImageRequired:   base.ImageRequired,
		OTPRequired:     base.OTPRequired,
		OTPChannel:      base.OTPChannel,
		MinChannelLevel: base.MinChannelLevel,
		UserCanTighten:  base.UserCanTighten,
		Source:          "platform",
	}

	// 全局强制二次验证（§9.1 mfa_required）：无策略行的场景也提升。
	if s.switches.MFARequired(ctx) {
		if !eff.OTPRequired {
			eff.OTPRequired = true
			eff.Source = "platform"
			if eff.OTPChannel == "" {
				eff.OTPChannel = model.ChannelEmail
			}
		}
	}
	// 总闸关闭：策略一律不生效（默认安全上线）。
	if !s.switches.CaptchaEnabled(ctx) {
		eff.PlatformForced = base.ImageRequired || base.OTPRequired
		return eff, nil
	}
	eff.PlatformForced = base.ImageRequired || base.OTPRequired

	if subject.ID == 0 || !base.UserCanTighten {
		s.applyFloor(base, eff)
		return eff, nil
	}

	// 用户侧加严（失败不影响主流程：读不到设置就按平台基线走）。
	mfaEnabled, mfaChannel, overrides := s.userSettings(ctx, subject)
	if mfaEnabled && !eff.OTPRequired {
		eff.OTPRequired = true
		eff.Source = "user_tightened"
	}
	if mfaChannel != "" && mfaEnabled {
		eff.OTPChannel = mfaChannel
	}
	if ov, ok := overrides[scene]; ok {
		if ov.OTPRequired != nil && *ov.OTPRequired && !eff.OTPRequired {
			eff.OTPRequired = true
			eff.Source = "user_tightened"
		}
		if ov.OTPChannel != "" {
			eff.OTPChannel = ov.OTPChannel
		}
		if ov.ImageRequired {
			eff.ImageRequired = true
		}
	}
	s.applyFloor(base, eff)
	return eff, nil
}

// applyFloor 地板校验：低于基线的一律提回基线，并给出提示；平台强制项不可被关闭。
func (s *PolicyService) applyFloor(base *model.CaptchaPolicy, eff *dto.EffectiveScene) {
	if base.MinChannelLevel > 0 && model.ChannelLevel(eff.OTPChannel) < base.MinChannelLevel {
		eff.OTPChannel = base.OTPChannel
		eff.Notice = "已按平台安全要求使用 " + model.ChannelLabel(eff.OTPChannel) + " 验证"
	}
	// 兜底：平台基线为 true 时用户设置只能让它保持 true（没有代码路径能改回 false）。
	if base.OTPRequired {
		eff.OTPRequired = true
	}
	if base.ImageRequired {
		eff.ImageRequired = true
	}
}

// userSettings 读取主体加严设置（读不到返回零值，不报错）。
func (s *PolicyService) userSettings(ctx context.Context, subject Subject) (bool, string, map[string]model.SceneOverride) {
	if subject.IsAdmin {
		m, err := s.repo.FindAdminMFA(ctx, subject.ID)
		if err != nil || m == nil {
			return false, "", nil
		}
		return m.MFAEnabled, m.MFAChannel, parseOverrides(m.SceneOverride)
	}
	st, err := s.repo.FindSettings(ctx, subject.ID)
	if err != nil || st == nil {
		return false, "", nil
	}
	return st.MFAEnabled, st.MFAChannel, parseOverrides(st.SceneOverrides)
}

// Required 判定是否需要进行某项验证（总闸 + 场景策略）。
func (s *PolicyService) Required(ctx context.Context, scene string, subject Subject) (image, otp bool, channel string) {
	if !s.switches.CaptchaEnabled(ctx) && !s.switches.MFARequired(ctx) {
		return false, false, ""
	}
	eff, err := s.EffectivePolicy(ctx, scene, subject)
	if err != nil || eff == nil {
		return false, false, ""
	}
	return eff.ImageRequired, eff.OTPRequired, eff.OTPChannel
}

// maskChannelTarget 依据通道选择要展示的目标（手机/邮箱）。
func maskChannelTarget(channel, phone, email string) string {
	if channel == model.ChannelSMS {
		return maskTarget(phone)
	}
	return maskTarget(email)
}

// cacheClient 类型断言辅助：保证 Service 依赖的是接口而非具体实现。
func cacheEnabled(c *cachepkg.Client) bool { return c != nil && c.Enabled() }

// normalizeChannel 归一通道名。
func normalizeChannel(ch string) string {
	return strings.ToLower(strings.TrimSpace(ch))
}

// challengeTTL 图形码 TTL（供 handler 回显 expires_in）。
func challengeTTL(sw *Switches, ctx context.Context) time.Duration {
	return sw.ImageTTL(ctx)
}

var _ = captchapkg.ErrInvalid
