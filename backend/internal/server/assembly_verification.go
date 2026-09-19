package server

// 实名认证装配（doc104 §5）。
//
// 依赖方向：admin/user/verification 不 import uc/captcha，二次验证经
// service.VerifyPort 端口外接；本文件把验证码策略服务适配成该端口。
// 用户端 handler 与后台 handler 共用同一个 service 实例——实名审核是同一张
// 申请表的两侧视图，拆成两个 service 会让状态机出现两份实现。

import (
	"context"
	"errors"
	"io"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
	verificationhandler "hostsent/backend/internal/modules/admin/user/verification/handler"
	verificationrepo "hostsent/backend/internal/modules/admin/user/verification/repository"
	verificationservice "hostsent/backend/internal/modules/admin/user/verification/service"
	captchaservice "hostsent/backend/internal/modules/uc/captcha/service"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/storage"
)

// verificationBundle 实名认证处理器集合（后台 + 用户端）。
type verificationBundle struct {
	adminHandler *verificationhandler.VerificationHandler
	userHandler  *verificationhandler.VerificationUserHandler
}

// buildVerificationBundle 装配实名认证。
//
// captcha 为 nil（验证码体系未装配）时二次验证整体跳过——requireSecondFactor
// 在 verify == nil 时直接放行，因此验证码模块不可用不会阻断实名提交。
//
// store 为 nil（存储初始化失败）时实名材料上传不可用，其余实名能力不受影响：
// 上传接口会返回明确错误，而不是让整个实名链路一起挂掉。
func buildVerificationBundle(
	cfg *config.Config,
	db *gorm.DB,
	captcha *captchaBundle,
	store *storage.LocalStore,
	logger *zap.Logger,
) *verificationBundle {
	repo := verificationrepo.NewVerificationRepository(db)
	svc := verificationservice.NewVerificationService(verificationservice.Deps{
		Repo: repo,
		// 实名配置读的是 verification_configs，不是 system_configs —— 配置页写的也是
		// 这张表。传 configReader（system_configs）会让配置页变成一个「保存成功但
		// 永远不生效」的假开关：运营把 provider 改成 alipay，服务端仍按 manual 走，
		// 表现是「提交后点跳转认证报不支持跳转」。
		Config:     newVerificationConfigReader(repo),
		Verify:     newVerificationVerifyPort(captcha),
		EncryptKey: cfg.App.EncryptKey,
		Logger:     logger,
		Documents:  newVerificationDocumentStore(store),
	})
	return &verificationBundle{
		adminHandler: verificationhandler.NewVerificationHandler(svc),
		userHandler:  verificationhandler.NewVerificationUserHandler(svc),
	}
}

// newVerificationConfigReader 把 verification_configs 表适配成实名的配置读取端口。
//
// 为什么不复用 system_configs 的 configValueReader：实名策略存在自己的
// verification_configs 表里（配置页 /admin/verifications/configs 读写的就是它）。
// 拿 system_configs 去读这些键，结果恒为「不存在」→ 全部走代码回落值，
// 运营在配置页怎么改都不生效，而这在界面上完全看不出来。
func newVerificationConfigReader(repo verificationrepo.VerificationRepository) verificationservice.ConfigReader {
	return func(ctx context.Context, key string) (string, bool, error) {
		row, err := repo.FindConfigByKey(ctx, key)
		if err != nil {
			if errors.Is(err, verificationrepo.ErrConfigNotFound) {
				return "", false, nil
			}
			return "", false, err
		}
		if row == nil || !strings.EqualFold(row.Status, "active") {
			return "", false, nil
		}
		return row.ConfigValue, true, nil
	}
}

// verificationDocumentStore 把 storage.LocalStore 适配成实名的材料存储端口。
//
// 适配而非直接用 *storage.LocalStore 的原因：service 层不该 import 文件系统实现，
// 否则换对象存储要改业务代码（与 VerifyPort 同款边界处理）。
type verificationDocumentStore struct{ store *storage.LocalStore }

func newVerificationDocumentStore(store *storage.LocalStore) verificationservice.DocumentStore {
	if store == nil {
		return nil
	}
	return &verificationDocumentStore{store: store}
}

func (d *verificationDocumentStore) Save(dir, filename string, r io.Reader, maxSize int64) (string, int64, error) {
	return d.store.Save(dir, filename, r, maxSize)
}

// Open 把 *os.File 收窄成 io.ReadCloser：service 只需要读，不暴露 Seek/Stat。
func (d *verificationDocumentStore) Open(relPath string) (io.ReadCloser, error) {
	return d.store.Open(relPath)
}

func (d *verificationDocumentStore) Remove(relPath string) error { return d.store.Remove(relPath) }

// verificationVerifyPort 把验证码策略/校验服务适配成实名的二次验证端口。
//
// 两个来源合起来才够用：策略判定在 PolicyService，图形码校验与票据消费在
// Service。任一为 nil 时对应能力退化为「不要求」而不是「一律拒绝」。
type verificationVerifyPort struct {
	policy  *captchaservice.PolicyService
	service *captchaservice.Service
}

func newVerificationVerifyPort(bundle *captchaBundle) verificationservice.VerifyPort {
	if bundle == nil || (bundle.policy == nil && bundle.service == nil) {
		return nil
	}
	return &verificationVerifyPort{policy: bundle.policy, service: bundle.service}
}

func (p *verificationVerifyPort) Required(ctx context.Context, scene string, userID uint64) (bool, bool) {
	if p == nil || p.policy == nil {
		return false, false
	}
	image, otp, _ := p.policy.Required(ctx, scene, captchaservice.Subject{ID: userID})
	return image, otp
}

func (p *verificationVerifyPort) VerifyImage(ctx context.Context, scene, captchaKey, captchaCode string) error {
	if p == nil || p.service == nil {
		return nil
	}
	return p.service.VerifyImage(ctx, scene, captchaKey, captchaCode)
}

func (p *verificationVerifyPort) ConsumeTicket(ctx context.Context, scene string, userID uint64, ticket string) bool {
	if p == nil || p.service == nil {
		// 没有验证码模块时按「不阻断」处理，与 uc/auth 的关键操作口径一致。
		return true
	}
	return p.service.ConsumeVerifyTicket(ctx, scene, captchaservice.Subject{ID: userID}, ticket)
}
