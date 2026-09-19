// Package service 提供实名认证模块的业务逻辑实现。
//
// 本次（doc104 §5）在既有「三个列表接口」之上补齐：
//   - 整单审核（通过 / 驳回 / 撤销），每次流转写 verification_review_logs；
//   - 用户端自助提交（证件号落密文、要求 realname_submit 二次验证）；
//   - verification_configs 的读写（此前表存在但没有任何代码路径）；
//   - realname_providers 的 CRUD 与连通测试（形态对齐 captcha_providers）。
//
// 一条硬规则贯穿全文件：**users.real_name_verified_at 是实名状态的唯一信任信号**。
// users.real_name 只是展示名（资料表单会直写它），任何地方都不得再用
// 「real_name 非空」判断是否已实名（doc104 §5.3，F13）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/model"
	"hostsent/backend/internal/modules/admin/user/verification/repository"
	"hostsent/backend/internal/pkg/credentials"
	"hostsent/backend/internal/pkg/integration"
	realnamepkg "hostsent/backend/internal/pkg/realname"
)

// 哨兵错误。
var (
	// ErrConfigNotAllowed 用户端自助提交被配置关闭。
	ErrConfigNotAllowed = errors.New("当前未开放自助实名认证")
	// ErrTypeNotAllowed 认证类型不在允许列表内。
	ErrTypeNotAllowed = errors.New("不支持该认证类型")
	// ErrAlreadyVerified 已实名且配置禁止重复提交。
	ErrAlreadyVerified = errors.New("账号已完成实名认证，如需变更请先联系客服撤销")
	// ErrPendingExists 已有待审核申请。
	ErrPendingExists = errors.New("已有待审核的实名申请，请耐心等待")
	// ErrCooldown 驳回后的冷却期未满。
	ErrCooldown = errors.New("距离上次驳回未满冷却期，请稍后再试")
	// ErrStatusConflict 状态流转非法（对已审核的申请再审）。
	ErrStatusConflict = errors.New("当前状态不允许该操作")
	// ErrApplicationNotFound 申请不存在，或不属于当前用户（不区分，避免探测）。
	ErrApplicationNotFound = errors.New("实名申请不存在")
	// ErrIDNumberInvalid 证件号格式不合法。
	ErrIDNumberInvalid = errors.New("证件号格式不合法")
	// ErrNotInitializer 该 provider 不支持跳转式核验。
	ErrNotInitializer = errors.New("该核验服务商不支持跳转认证")
	// ErrProviderNotConfigured 核验服务商未配置。
	ErrProviderNotConfigured = errors.New("实名核验服务商未配置")
	// ErrOperatorRequired 审核人缺失。
	ErrOperatorRequired = errors.New("审核人不能为空")
	// ErrRejectReasonRequired 驳回未填理由。
	//
	// 单独立一个哨兵而不是 errors.New 就地返回：驳回理由为空是**参数问题**（400），
	// 不是服务端故障。原先走 default 分支落进 500，前端只能靠 message 文本判断
	// 「这次该不该让运营补理由」，而 5xx 在监控里还会被当成真实故障告警。
	ErrRejectReasonRequired = errors.New("驳回理由不能为空")
)

// 场景常量：提交实名前要求的二次验证场景（策略已在 captcha 种子里铺好）。
const sceneRealnameSubmit = "realname_submit"

// VerifyPort 二次验证端口（提交实名前的场景校验）。
//
// 抽成端口而不是直接依赖 uc/captcha：admin/user/verification 不该反向依赖
// 用户中心的验证码服务，装配层用 captcha 的策略服务适配。
type VerifyPort interface {
	// Required 该场景是否要求二次验证（不要求时 Verify 可跳过）。
	Required(ctx context.Context, scene string, userID uint64) (image, otp bool)
	// VerifyImage 校验图形码。
	VerifyImage(ctx context.Context, scene, captchaKey, captchaCode string) error
	// ConsumeTicket 校验并消费关键操作票据（一次性）。
	ConsumeTicket(ctx context.Context, scene string, userID uint64, ticket string) bool
}

// ConfigReader 读配置原文（缺失返回 ok=false，不报错）。
type ConfigReader func(ctx context.Context, key string) (string, bool, error)

// VerificationService 实名认证服务。
type VerificationService interface {
	// —— 列表与详情 ——
	ListPending(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
	ListApproved(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
	ListRejected(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error)
	Detail(ctx context.Context, id uint64) (*dto.VerificationDetail, error)
	CountPending(ctx context.Context) (int64, error)

	// —— 整单审核 ——
	Approve(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error)
	Reject(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error)
	Revoke(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error)
	ProviderCheck(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProviderCheckResult, error)

	// —— 配置 ——
	ListConfigs(ctx context.Context, query dto.VerificationConfigListQuery) ([]dto.VerificationConfigInfo, error)
	UpsertConfig(ctx context.Context, req dto.ConfigUpsertRequest, operatorID uint64) (*dto.VerificationConfigInfo, error)
	DeleteConfig(ctx context.Context, id uint64) error

	// —— 服务商 ——
	ProviderTypes() []dto.ProviderTypeInfo
	ListProviders(ctx context.Context) ([]dto.ProviderInfo, error)
	UpsertProvider(ctx context.Context, req dto.ProviderUpsertRequest) (*dto.ProviderInfo, error)
	DeleteProvider(ctx context.Context, id uint64) error
	TestProvider(ctx context.Context, id uint64) error

	// —— 用户端 ——
	UserStatus(ctx context.Context, userID uint64) (*dto.UserStatusResponse, error)
	Submit(ctx context.Context, userID uint64, username string, req dto.SubmitRequest) (*dto.VerificationInfo, error)
	ListMyApplications(ctx context.Context, userID uint64, page, pageSize int) (*dto.ListResponse[dto.VerificationInfo], error)
	MyApplication(ctx context.Context, userID, id uint64) (*dto.VerificationDetail, error)
	Authorize(ctx context.Context, userID, id uint64) (*dto.AuthorizeResponse, error)
	// HandleProviderCallback 三方回调：按 certify_id 查结果并落库（免登录入口）。
	HandleProviderCallback(ctx context.Context, applicationID uint64, certifyID string) error
	// IsRealnameVerified 实名判定的唯一口径（doc104 §5.3）。
	IsRealnameVerified(ctx context.Context, userID uint64) (bool, error)

	// —— 材料上传与下载（doc104 §5.7，实现在 document_service.go）——
	UploadDocument(ctx context.Context, userID, applicationID uint64, docType, fileName string, r io.Reader) (*dto.DocumentInfo, error)
	OpenDocument(ctx context.Context, docID uint64, manageView bool, userID uint64) (*dto.DocumentInfo, io.ReadCloser, error)
}

type verificationService struct {
	repo       repository.VerificationRepository
	cfg        ConfigReader
	verify     VerifyPort
	encryptKey string
	logger     *zap.Logger
	// documents 材料存储；未装配时为 nil，上传接口明确报错而不是 panic。
	documents DocumentStore
}

// Deps 服务依赖。
type Deps struct {
	Repo       repository.VerificationRepository
	Config     ConfigReader
	Verify     VerifyPort
	EncryptKey string
	Logger     *zap.Logger
	// Documents 材料存储（可空）。
	Documents DocumentStore
}

// NewVerificationService 创建实名认证业务服务。
func NewVerificationService(d Deps) VerificationService {
	logger := d.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &verificationService{
		repo:       d.Repo,
		cfg:        d.Config,
		verify:     d.Verify,
		encryptKey: d.EncryptKey,
		logger:     logger,
		documents:  d.Documents,
	}
}

// ---------- 列表与详情 ----------

func (s *verificationService) ListPending(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, model.StatusPending, query)
}

func (s *verificationService) ListApproved(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, model.StatusApproved, query)
}

func (s *verificationService) ListRejected(ctx context.Context, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	return s.listByStatus(ctx, model.StatusRejected, query)
}

func (s *verificationService) CountPending(ctx context.Context) (int64, error) {
	return s.repo.CountPending(ctx)
}

func (s *verificationService) listByStatus(ctx context.Context, status string, query dto.VerificationListQuery) (*dto.ListResponse[dto.VerificationInfo], error) {
	page, pageSize := normalizeMeta(query.Page, query.PageSize)
	items, total, err := s.repo.ListByStatus(ctx, status, query)
	if err != nil {
		return nil, err
	}
	respItems := make([]dto.VerificationInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toVerificationInfo(item))
	}
	return &dto.ListResponse[dto.VerificationInfo]{
		Items: respItems,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}, nil
}

// Detail 管理端申请详情（含附件、企业信息、审核轨迹）。
func (s *verificationService) Detail(ctx context.Context, id uint64) (*dto.VerificationDetail, error) {
	app, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, err
	}
	return s.buildDetail(ctx, app), nil
}

// MyApplication 用户端查本人申请。
func (s *verificationService) MyApplication(ctx context.Context, userID, id uint64) (*dto.VerificationDetail, error) {
	app, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrApplicationNotFound
	}
	if app.UserID != userID {
		// 越权与不存在返回同一个错误：否则可以靠错误差异探测他人申请 ID。
		return nil, ErrApplicationNotFound
	}
	return s.buildDetail(ctx, app), nil
}

func (s *verificationService) buildDetail(ctx context.Context, app *model.VerificationApplication) *dto.VerificationDetail {
	detail := &dto.VerificationDetail{
		VerificationInfo: toVerificationInfo(*app),
		Documents:        []dto.DocumentInfo{},
		Logs:             []dto.ReviewLogInfo{},
		CertifyModes:     []integration.FieldOption{},
	}
	if docs, err := s.repo.ListDocuments(ctx, app.ID); err == nil {
		for _, d := range docs {
			detail.Documents = append(detail.Documents, dto.DocumentInfo{
				ID: d.ID, DocumentType: d.DocumentType, FileURL: d.FileURL, Sort: d.Sort,
			})
		}
	}
	if logs, err := s.repo.ListReviewLogs(ctx, app.ID); err == nil {
		for _, l := range logs {
			detail.Logs = append(detail.Logs, dto.ReviewLogInfo{
				ID: l.ID, FromStatus: l.FromStatus, ToStatus: l.ToStatus, Action: l.Action,
				OperatorID: l.OperatorID, OperatorName: l.OperatorName, Note: l.Note,
				RejectReasonCode: l.RejectReasonCode, RejectReason: l.RejectReason, CreatedAt: l.CreatedAt,
			})
		}
	}
	if ent, err := s.repo.FindEnterprise(ctx, app.ID); err == nil && ent != nil {
		detail.Enterprise = &dto.EnterpriseInfo{
			CompanyName:      ent.CompanyName,
			CreditCodeMasked: ent.CreditCodeMasked,
			LegalPersonName:  ent.LegalPersonName,
			ContactName:      ent.ContactName,
			BusinessLicense:  ent.BusinessLicense,
		}
	}
	// 该 provider 支持的认证方式（前端渲染「去认证」按钮的形态）。
	if d, ok := realnamepkg.Descriptor(app.Provider); ok {
		detail.CertifyModes = d.CertifyModes
	}
	return detail
}

// ---------- 整单审核 ----------

// Approve 整单通过。
//
// 三件事必须同时成立，缺一都会留下「通过了但查不到为什么」的黑洞：
//  1. 状态从 pending 迁移到 approved（非 pending 一律 409）；
//  2. 写 users.real_name_verified_at（唯一的实名信任信号）；
//  3. 写一条 verification_review_logs（action=approve）。
func (s *verificationService) Approve(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error) {
	return s.transition(ctx, id, []string{model.StatusPending}, model.StatusApproved, model.ActionApprove, req, operatorID, operatorName)
}

// Reject 整单驳回。驳回理由必填（前端也校验，服务层再兜一层）。
func (s *verificationService) Reject(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error) {
	if strings.TrimSpace(req.RejectReason) == "" {
		return nil, ErrRejectReasonRequired
	}
	return s.transition(ctx, id, []string{model.StatusPending}, model.StatusRejected, model.ActionReject, req, operatorID, operatorName)
}

// Revoke 撤销已通过的认证（doc104 §5.3）。
//
// 只允许对 approved 的申请执行；撤销后清空 users 的实名信任信号，
// 并把申请置回 rejected（不是 pending —— 撤销是审核方的判定，不该重回待审队列）。
//
// 清信任信号的动作放在 transition 内、UpdateReview 成功之后：早先的写法是
// 「先清信号再迁移状态」，而 transition 只接受 pending，导致撤销一个已通过的申请
// 会先把 real_name_verified_at 抹掉、再返回 409 —— 用户被判未实名，申请却仍是
// 「已通过」，两边对不上且不可自愈。
func (s *verificationService) Revoke(ctx context.Context, id uint64, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error) {
	return s.transition(ctx, id, []string{model.StatusApproved}, model.StatusRejected, model.ActionRevoke, req, operatorID, operatorName)
}

// transition 审核状态迁移的统一实现。
//
// fromStatuses 是本次动作允许的起始状态集合：通过/驳回只接受 pending（幂等与并发保护——
// 对已审核的再审会覆盖前一次的审核人与时间，让「谁放行的」永久丢失），撤销只接受 approved。
func (s *verificationService) transition(ctx context.Context, id uint64, fromStatuses []string, toStatus, action string, req dto.ReviewRequest, operatorID uint64, operatorName string) (*dto.VerificationInfo, error) {
	app, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApplicationNotFound
		}
		return nil, err
	}
	if !containsStatus(fromStatuses, app.Status) {
		return nil, fmt.Errorf("%w: 当前状态为%s", ErrStatusConflict, statusLabel(app.Status))
	}
	if operatorID == 0 {
		return nil, ErrOperatorRequired
	}

	fromStatus := app.Status
	now := time.Now()
	app.Status = toStatus
	app.ReviewedAt = &now
	app.ReviewedBy = &operatorID
	app.ReviewerName = operatorName
	app.ReviewNote = strings.TrimSpace(req.Note)
	app.RejectReasonCode = strings.TrimSpace(req.RejectReasonCode)
	if toStatus == model.StatusRejected {
		app.RejectReason = strings.TrimSpace(req.RejectReason)
	} else {
		app.RejectReason = ""
		app.RejectReasonCode = ""
	}

	reviewLog := &model.VerificationReviewLog{
		ApplicationID:    app.ID,
		FromStatus:       fromStatus,
		ToStatus:         toStatus,
		Action:           action,
		OperatorID:       operatorID,
		OperatorName:     operatorName,
		Note:             app.ReviewNote,
		RejectReasonCode: app.RejectReasonCode,
		RejectReason:     app.RejectReason,
	}
	if err := s.repo.UpdateReview(ctx, app, reviewLog); err != nil {
		return nil, err
	}

	// 通过 → 写实名信任信号；驳回/撤销 → 清空（重复清幂等）。
	if toStatus == model.StatusApproved {
		source := app.Provider
		if source == "" {
			source = "manual"
		}
		if err := s.repo.MarkUserVerified(ctx, app.UserID, now, source); err != nil {
			// 信任信号写失败必须让整个操作失败：否则申请显示「已通过」而用户仍被判未实名。
			return nil, fmt.Errorf("写入实名状态失败：%w", err)
		}
	} else if action == model.ActionReject || action == model.ActionRevoke {
		if err := s.repo.ClearUserVerified(ctx, app.UserID); err != nil {
			return nil, err
		}
	}

	info := toVerificationInfo(*app)
	return &info, nil
}

// ProviderCheck 手动触发一次三方核验（doc104 §5.7）。
//
// 只对实现了 DirectVerify 的 provider 有效；支付宝属跳转式核验，
// 手动触发会明确返回「不支持」，由运营改走人工审核。
func (s *verificationService) ProviderCheck(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.ProviderCheckResult, error) {
	app, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrApplicationNotFound
	}
	providerType := s.effectiveProviderType(ctx)
	row, err := s.repo.FindDefaultProvider(ctx, providerType)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, providerType)
	}
	cfg, err := s.providerConfig(row)
	if err != nil {
		return nil, err
	}
	prov, err := realnamepkg.New(providerType, cfg)
	if err != nil {
		return nil, err
	}
	idNumber, err := s.decryptSecret(app.IDNumberEncrypted)
	if err != nil {
		return nil, err
	}
	mobile, _ := s.decryptSecret(app.MobileEncrypted)
	res, err := prov.DirectVerify(ctx, cfg, realnamepkg.VerifyRequest{
		ApplicationID: app.ID,
		UserID:        app.UserID,
		RealName:      app.RealName,
		IDNumber:      idNumber,
		Mobile:        mobile,
		CertifyMode:   certifyModeOf(row),
	})
	if err != nil {
		// 适配器未实现：这是业务状态而非错误，返回 ok=false 让运营看到原因。
		if errors.Is(err, realnamepkg.ErrAdapterNotImplemented) {
			return &dto.ProviderCheckResult{
				OK:       false,
				Provider: providerType,
				Message:  "该服务商不支持无跳转核验，请走人工审核或让用户完成跳转认证",
			}, nil
		}
		return nil, err
	}

	now := time.Now()
	app.Provider = providerType
	app.ProviderTxnNo = res.TxnNo
	app.ProviderCheckedAt = &now
	switch {
	case res.Passed:
		app.ProviderResult = model.ProviderResultPass
	case res.BizCode == "PENDING":
		app.ProviderResult = ""
	default:
		app.ProviderResult = model.ProviderResultFail
	}
	app.ProviderMessage = truncate(res.Message, 255)
	if err := s.repo.UpdateProviderResult(ctx, app); err != nil {
		return nil, err
	}
	// 核验动作本身要留痕（action=provider_pass / provider_fail），
	// 否则「这条申请为什么被自动放行」在审核轨迹里看不到。
	action := model.ActionProviderFail
	if res.Passed {
		action = model.ActionProviderPass
	}
	_ = s.repo.WriteReviewLog(ctx, &model.VerificationReviewLog{
		ApplicationID: app.ID,
		FromStatus:    app.Status,
		ToStatus:      app.Status,
		Action:        action,
		OperatorID:    operatorID,
		OperatorName:  operatorName,
		Note:          app.ProviderMessage,
	})

	// 自动放行 / 自动驳回（默认关闭，需运营显式开启）。
	autoApproved, autoRejected := false, false
	if app.Status == model.StatusPending {
		if res.Passed && s.configBool(ctx, model.ConfigKeyAutoApproveOnPass, false) {
			if _, err := s.Approve(ctx, app.ID, dto.ReviewRequest{Note: "三方核验通过自动放行"}, operatorID, operatorName); err == nil {
				autoApproved = true
			} else {
				s.logger.Warn("auto approve after provider check failed", zap.Error(err), zap.Uint64("application_id", app.ID))
			}
		}
		if app.ProviderResult == model.ProviderResultFail && s.configBool(ctx, model.ConfigKeyAutoRejectOnFail, false) {
			if _, err := s.Reject(ctx, app.ID, dto.ReviewRequest{
				RejectReason:     "三方核验未通过：" + app.ProviderMessage,
				RejectReasonCode: "provider_failed",
			}, operatorID, operatorName); err == nil {
				autoRejected = true
			} else {
				s.logger.Warn("auto reject after provider check failed", zap.Error(err), zap.Uint64("application_id", app.ID))
			}
		}
	}

	return &dto.ProviderCheckResult{
		OK:           res.Passed,
		Provider:     providerType,
		Passed:       res.Passed,
		BizCode:      res.BizCode,
		Message:      app.ProviderMessage,
		TxnNo:        res.TxnNo,
		AutoApproved: autoApproved,
		AutoRejected: autoRejected,
	}, nil
}

// ---------- 配置 ----------

func (s *verificationService) ListConfigs(ctx context.Context, query dto.VerificationConfigListQuery) ([]dto.VerificationConfigInfo, error) {
	rows, err := s.repo.ListConfigs(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VerificationConfigInfo, 0, len(rows))
	for i := range rows {
		out = append(out, toConfigInfo(&rows[i]))
	}
	return out, nil
}

// UpsertConfig 按 config_key 新增或更新配置项。
//
// 用 key 而不是 id 作为入口：配置是「键值」语义，运营心智里没有 ID；
// 同时避免前端先查列表拿 ID 再更新的两步操作。
func (s *verificationService) UpsertConfig(ctx context.Context, req dto.ConfigUpsertRequest, operatorID uint64) (*dto.VerificationConfigInfo, error) {
	key := strings.TrimSpace(req.ConfigKey)
	if key == "" {
		return nil, errors.New("配置键不能为空")
	}
	row, err := s.repo.FindConfigByKey(ctx, key)
	if err != nil {
		if !errors.Is(err, repository.ErrConfigNotFound) {
			return nil, err
		}
		row = &model.VerificationConfig{
			ConfigKey:   key,
			ConfigGroup: model.ConfigGroupVerification,
			ValueType:   "string",
			Status:      "active",
		}
	}
	if req.ConfigValue != nil {
		row.ConfigValue = *req.ConfigValue
	}
	if v := strings.TrimSpace(req.ValueType); v != "" {
		row.ValueType = v
	}
	if req.Description != nil {
		row.Description = *req.Description
	}
	if v := strings.TrimSpace(req.Status); v != "" {
		row.Status = v
	}
	row.UpdatedBy = operatorID
	if err := s.repo.SaveConfig(ctx, row); err != nil {
		return nil, err
	}
	info := toConfigInfo(row)
	return &info, nil
}

func (s *verificationService) DeleteConfig(ctx context.Context, id uint64) error {
	return s.repo.DeleteConfig(ctx, id)
}

// ---------- 服务商 ----------

func (s *verificationService) ProviderTypes() []dto.ProviderTypeInfo {
	all := realnamepkg.AllDescriptors()
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
			CertifyModes:     d.CertifyModes,
		})
	}
	return out
}

func (s *verificationService) ListProviders(ctx context.Context) ([]dto.ProviderInfo, error) {
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

func (s *verificationService) toProviderInfo(row *model.RealnameProvider) dto.ProviderInfo {
	d, hasDescriptor := realnamepkg.Descriptor(row.ProviderType)
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
		Supported:      realnamepkg.IsRegistered(row.ProviderType),
		Credentials:    map[string]string{},
		CredentialKeys: []string{},
		CertifyModes:   []integration.FieldOption{},
	}
	if row.LastCheckAt != nil {
		info.LastCheckAt = row.LastCheckAt.Format(time.RFC3339)
	}
	if hasDescriptor {
		info.Implemented = d.Implemented
		info.Builtin = d.Builtin
		info.CertifyModes = d.CertifyModes
	}
	// 凭证回显：secret 字段一律脱敏（credentials.Mask），非密文字段原样。
	values, err := credentials.Decode(row.Credentials)
	if err == nil && len(values) > 0 {
		schema := []integration.Field{}
		if hasDescriptor {
			schema = d.CredentialSchema
		}
		for k, v := range values.Mask(schema) {
			info.Credentials[k] = v
			info.CredentialKeys = append(info.CredentialKeys, k)
		}
	}
	return info
}

// UpsertProvider 新建或更新服务商配置。
//
// ID=0 时新建（类型必须已登记描述符）；ID>0 时更新。凭证语义与验证码/支付渠道一致：
// 回传脱敏值表示未修改，保留原密文。
func (s *verificationService) UpsertProvider(ctx context.Context, req dto.ProviderUpsertRequest) (*dto.ProviderInfo, error) {
	var row *model.RealnameProvider
	var err error
	if req.ID > 0 {
		row, err = s.repo.FindProviderByID(ctx, req.ID)
		if err != nil {
			return nil, err
		}
	} else {
		providerType := strings.TrimSpace(req.ProviderType)
		if providerType == "" {
			return nil, errors.New("服务商类型不能为空")
		}
		d, ok := realnamepkg.Descriptor(providerType)
		if !ok {
			return nil, fmt.Errorf("未知的实名核验服务商类型：%s", providerType)
		}
		row = &model.RealnameProvider{
			ProviderType: providerType,
			Name:         d.Name,
			Mode:         d.Mode,
			Status:       1,
		}
	}
	d, hasDescriptor := realnamepkg.Descriptor(row.ProviderType)

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
	// 启用（status=1）时凭证必须齐全，避免「启用了但一调用就报配置缺失」。
	status := row.Status
	if req.Status != nil {
		status = *req.Status
	}
	if status == 1 && hasDescriptor {
		if err := realnamepkg.ValidateCredentials(d, merged); err != nil {
			return nil, err
		}
	}
	encrypted, err := s.encryptCredentials(d, merged)
	if err != nil {
		return nil, err
	}

	if name := strings.TrimSpace(req.Name); name != "" {
		row.Name = name
	}
	row.Credentials = encrypted
	row.Descriptor = mustJSON(d)
	if req.Endpoint != "" {
		row.Endpoint = req.Endpoint
	}
	row.Priority = req.Priority
	row.Status = status
	if req.IsDefault != nil {
		row.IsDefault = *req.IsDefault
	}
	row.Remark = req.Remark

	if row.ID == 0 {
		if err := s.repo.CreateProvider(ctx, row); err != nil {
			return nil, err
		}
	} else if err := s.repo.UpdateProvider(ctx, row); err != nil {
		return nil, err
	}
	if row.IsDefault {
		if err := s.repo.ClearDefaultProviders(ctx, row.ID); err != nil {
			s.logger.Warn("clear default realname providers failed", zap.Error(err))
		}
	}
	info := s.toProviderInfo(row)
	return &info, nil
}

// DeleteProvider 软删除服务商。
//
// 内置 manual 不可删除：它是「没有三方核验」时的合法兜底路径，
// 删掉会让配置项指向一个不存在的 provider，实名提交直接失败。
func (s *verificationService) DeleteProvider(ctx context.Context, id uint64) error {
	row, err := s.repo.FindProviderByID(ctx, id)
	if err != nil {
		return err
	}
	if d, ok := realnamepkg.Descriptor(row.ProviderType); ok && d.Builtin {
		return errors.New("内置的人工审核不可删除（它是未接入三方核验时的兜底）")
	}
	return s.repo.DeleteProvider(ctx, id)
}

// TestProvider 连通性测试；未注册适配器的类型标记 pending 并返回「待接入」。
func (s *verificationService) TestProvider(ctx context.Context, id uint64) error {
	row, err := s.repo.FindProviderByID(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := s.providerConfig(row)
	if err != nil {
		s.markHealth(ctx, row, "down", err.Error())
		return err
	}
	prov, err := realnamepkg.New(row.ProviderType, cfg)
	if err != nil {
		s.markHealth(ctx, row, "pending", realnamepkg.ErrAdapterNotImplemented.Error())
		return realnamepkg.ErrAdapterNotImplemented
	}
	if err := prov.Test(ctx, cfg); err != nil {
		s.markHealth(ctx, row, "down", err.Error())
		return err
	}
	s.markHealth(ctx, row, "healthy", "")
	return nil
}

func (s *verificationService) markHealth(ctx context.Context, row *model.RealnameProvider, status, lastErr string) {
	now := time.Now()
	row.HealthStatus = status
	row.LastError = lastErr
	row.LastCheckAt = &now
	if err := s.repo.UpdateProvider(ctx, row); err != nil {
		s.logger.Warn("update realname provider health failed", zap.Error(err), zap.Uint64("provider_id", row.ID))
	}
}

// ---------- 用户端 ----------

// UserStatus 当前用户的实名状态（用户端状态卡）。
func (s *verificationService) UserStatus(ctx context.Context, userID uint64) (*dto.UserStatusResponse, error) {
	resp := &dto.UserStatusResponse{
		Enabled:      s.configBool(ctx, model.ConfigKeyEnabled, true),
		AllowedTypes: s.allowedTypes(ctx),
	}
	latest, err := s.repo.LatestByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Status = "none"
			return resp, nil
		}
		return nil, err
	}
	resp.Status = latest.Status
	resp.ApplicationID = latest.ID
	resp.VerificationType = latest.VerificationType
	resp.RealName = latest.RealName
	resp.IDNumberMasked = latest.IDNumberMasked
	resp.MobileMasked = latest.MobileMasked
	resp.SubmittedAt = &latest.SubmittedAt
	resp.ReviewedAt = latest.ReviewedAt
	resp.RejectReason = latest.RejectReason
	resp.ReviewNote = latest.ReviewNote
	resp.ProviderResult = latest.ProviderResult
	resp.ReviewRound = latest.ReviewRound
	resp.CooldownHours = s.configInt(ctx, model.ConfigKeyResubmitCooldown, model.DefaultResubmitCooldownHours)
	// 冷却期剩余：驳回时间 + 冷却小时数 - 现在（已通过或时间已过则为 0）。
	if latest.Status == model.StatusRejected && latest.ReviewedAt != nil && resp.CooldownHours > 0 {
		until := latest.ReviewedAt.Add(time.Duration(resp.CooldownHours) * time.Hour)
		if remain := time.Until(until); remain > 0 {
			resp.CooldownRemainSeconds = int(remain.Seconds())
		}
	}
	return resp, nil
}

// Submit 用户自助提交实名申请。
//
// 前置校验顺序（doc104 §5.4）：
//  1. 配置开关 verification.enabled
//  2. 认证类型在 allowed_types 内
//  3. 已有 pending → 拒绝（避免刷审核队列）
//  4. 已实名且 real_name_locked → 拒绝（改名须走撤销）
//  5. 上次驳回未过冷却期 → 拒绝
//  6. realname_submit 场景的二次验证（图形码 + 关键操作票据）
//  7. 证件号格式校验 → 落密文（provider 核验必须用原文）
func (s *verificationService) Submit(ctx context.Context, userID uint64, username string, req dto.SubmitRequest) (*dto.VerificationInfo, error) {
	if !s.configBool(ctx, model.ConfigKeyEnabled, true) {
		return nil, ErrConfigNotAllowed
	}
	vType := strings.TrimSpace(req.VerificationType)
	if vType == "" {
		vType = model.TypePersonal
	}
	if !s.typeAllowed(ctx, vType) {
		return nil, fmt.Errorf("%w: %s", ErrTypeNotAllowed, vType)
	}
	realName := strings.TrimSpace(req.RealName)
	if realName == "" {
		return nil, errors.New("真实姓名不能为空")
	}
	idType, idNumber, err := normalizeID(vType, req.IDType, req.IDNumber)
	if err != nil {
		return nil, err
	}
	mobile := strings.TrimSpace(req.Mobile)

	latest, err := s.repo.LatestByUser(ctx, userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if latest != nil {
		switch latest.Status {
		case model.StatusPending:
			return nil, ErrPendingExists
		case model.StatusApproved:
			if s.configBool(ctx, model.ConfigKeyRealNameLocked, true) {
				return nil, ErrAlreadyVerified
			}
		case model.StatusRejected:
			cooldown := s.configInt(ctx, model.ConfigKeyResubmitCooldown, model.DefaultResubmitCooldownHours)
			if cooldown > 0 && latest.ReviewedAt != nil {
				if until := latest.ReviewedAt.Add(time.Duration(cooldown) * time.Hour); time.Now().Before(until) {
					return nil, ErrCooldown
				}
			}
		}
	}

	// 二次验证：图形码 + 关键操作票据（策略未启用时一律放行，不阻断提交）。
	if err := s.requireSecondFactor(ctx, userID, req); err != nil {
		return nil, err
	}

	idCipher, err := s.encryptSecret(idNumber)
	if err != nil {
		return nil, err
	}
	mobileCipher := ""
	if mobile != "" {
		if mobileCipher, err = s.encryptSecret(mobile); err != nil {
			return nil, err
		}
	}
	round, err := s.repo.NextReviewRound(ctx, userID)
	if err != nil {
		return nil, err
	}
	providerType := s.effectiveProviderType(ctx)

	app := &model.VerificationApplication{
		UserID:            userID,
		Username:          username,
		VerificationType:  vType,
		Status:            model.StatusPending,
		RealName:          realName,
		SubjectName:       subjectName(vType, req.SubjectName, realName),
		IDType:            idType,
		IDNumberMasked:    maskIDNumber(idNumber),
		MobileMasked:      maskMobile(mobile),
		CountryCode:       strings.TrimSpace(req.CountryCode),
		SubmittedAt:       time.Now(),
		ReviewRound:       round,
		IDNumberEncrypted: idCipher,
		MobileEncrypted:   mobileCipher,
		Provider:          providerType,
		Version:           1,
	}
	if err := s.repo.Create(ctx, app); err != nil {
		return nil, err
	}
	// 企业认证的扩展信息（公司名 / 统一社会信用代码 / 法人）。
	if vType == model.TypeEnterprise {
		ent := &model.VerificationEnterprise{
			ApplicationID:    app.ID,
			CompanyName:      subjectName(vType, req.CompanyName, realName),
			CreditCodeMasked: maskCreditCode(strings.TrimSpace(req.CreditCode)),
			LegalPersonName:  strings.TrimSpace(req.LegalPersonName),
			ContactName:      strings.TrimSpace(req.ContactName),
		}
		if err := s.repo.SaveEnterprise(ctx, ent); err != nil {
			// 企业信息写失败不能让申请变成「半张单」：直接报错，由用户重试。
			return nil, fmt.Errorf("保存企业信息失败：%w", err)
		}
	}
	_ = s.repo.WriteReviewLog(ctx, &model.VerificationReviewLog{
		ApplicationID: app.ID,
		FromStatus:    "",
		ToStatus:      model.StatusPending,
		Action:        model.ActionSubmit,
		// 自助提交没有管理员操作人：operator_id=0，operator_name 记用户名。
		OperatorID:   0,
		OperatorName: username,
		Note:         fmt.Sprintf("第 %d 次提交", round),
	})

	info := toVerificationInfo(*app)
	return &info, nil
}

// requireSecondFactor 提交前的二次验证。
//
// 与 uc/auth 的关键操作口径一致：策略未启用 → 放行（绝不因为验证码模块阻断业务）；
// 策略要求图形码 → 校验图形码；要求 OTP → 校验一次性票据。
func (s *verificationService) requireSecondFactor(ctx context.Context, userID uint64, req dto.SubmitRequest) error {
	if s.verify == nil {
		return nil
	}
	imageRequired, otpRequired := s.verify.Required(ctx, sceneRealnameSubmit, userID)
	if imageRequired {
		if err := s.verify.VerifyImage(ctx, sceneRealnameSubmit, req.CaptchaKey, req.CaptchaCode); err != nil {
			return err
		}
	}
	if otpRequired && !s.verify.ConsumeTicket(ctx, sceneRealnameSubmit, userID, req.VerifyTicket) {
		return errors.New("关键操作验证已失效，请重新验证后再提交")
	}
	return nil
}

// ListMyApplications 本人申请历史。
func (s *verificationService) ListMyApplications(ctx context.Context, userID uint64, page, pageSize int) (*dto.ListResponse[dto.VerificationInfo], error) {
	p, ps := normalizeMeta(page, pageSize)
	items, total, err := s.repo.ListByUser(ctx, userID, p, ps)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VerificationInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toVerificationInfo(item))
	}
	return &dto.ListResponse[dto.VerificationInfo]{
		Items: out,
		Meta:  dto.ListMeta{Page: p, PageSize: ps, Total: total},
	}, nil
}

// Authorize 跳转式核验的认证入口（支付宝）。
//
// 只有 provider 实现了 realname.Initializer 才可用；否则明确报「不支持」，
// 让前端提示用户改走人工审核，而不是给一个点不动的按钮。
func (s *verificationService) Authorize(ctx context.Context, userID, id uint64) (*dto.AuthorizeResponse, error) {
	app, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrApplicationNotFound
	}
	if app.UserID != userID {
		return nil, ErrApplicationNotFound
	}
	if app.Status != model.StatusPending {
		return nil, fmt.Errorf("%w: 当前状态为%s", ErrStatusConflict, statusLabel(app.Status))
	}
	providerType := s.effectiveProviderType(ctx)
	row, err := s.repo.FindDefaultProvider(ctx, providerType)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, providerType)
	}
	cfg, err := s.providerConfig(row)
	if err != nil {
		return nil, err
	}
	prov, err := realnamepkg.New(providerType, cfg)
	if err != nil {
		return nil, err
	}
	init, ok := prov.(realnamepkg.Initializer)
	if !ok {
		return nil, ErrNotInitializer
	}
	idNumber, err := s.decryptSecret(app.IDNumberEncrypted)
	if err != nil {
		return nil, err
	}
	mobile, _ := s.decryptSecret(app.MobileEncrypted)
	challenge, err := init.Initialize(ctx, cfg, realnamepkg.VerifyRequest{
		ApplicationID: app.ID,
		UserID:        app.UserID,
		RealName:      app.RealName,
		IDNumber:      idNumber,
		Mobile:        mobile,
		CertifyMode:   certifyModeOf(row),
	})
	if err != nil {
		return nil, err
	}
	// 流水号落库：用户完成认证后靠它查结果。
	now := time.Now()
	app.Provider = providerType
	app.ProviderTxnNo = challenge.TxnNo
	app.ProviderCheckedAt = &now
	if err := s.repo.UpdateProviderResult(ctx, app); err != nil {
		return nil, err
	}
	return &dto.AuthorizeResponse{
		AuthURL:  challenge.AuthURL,
		TxnNo:    challenge.TxnNo,
		ExpireAt: challenge.ExpireAt,
		Provider: providerType,
	}, nil
}

// HandleProviderCallback 三方回调：查询核验结果并落库。
//
// 回调入口免登录，因此以 certify_id（在服务端生成并落库）作为唯一关联依据，
// 不接受任何来自回调请求的 user_id / 姓名 / 证件号 —— 那些都可被伪造。
func (s *verificationService) HandleProviderCallback(ctx context.Context, applicationID uint64, certifyID string) error {
	app, err := s.repo.FindByID(ctx, applicationID)
	if err != nil {
		return ErrApplicationNotFound
	}
	// 已审核的申请不再改结果：回调可能是用户补完认证后的迟到通知。
	if app.Status != model.StatusPending {
		return nil
	}
	// certify_id 必须与落库的流水号一致，防止用一个合法申请 ID + 任意 certify_id
	// 去「代查」别人的核验结果。
	if strings.TrimSpace(app.ProviderTxnNo) == "" || app.ProviderTxnNo != certifyID {
		return errors.New("核验流水号不匹配")
	}
	providerType := s.effectiveProviderType(ctx)
	row, err := s.repo.FindDefaultProvider(ctx, providerType)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrProviderNotConfigured, providerType)
	}
	cfg, err := s.providerConfig(row)
	if err != nil {
		return err
	}
	prov, err := realnamepkg.New(providerType, cfg)
	if err != nil {
		return err
	}
	init, ok := prov.(realnamepkg.Initializer)
	if !ok {
		return ErrNotInitializer
	}
	res, err := init.Query(ctx, cfg, certifyID)
	if err != nil {
		return err
	}
	now := time.Now()
	app.ProviderCheckedAt = &now
	app.ProviderMessage = truncate(res.Message, 255)
	switch {
	case res.Passed:
		app.ProviderResult = model.ProviderResultPass
	case res.BizCode == "PENDING":
		app.ProviderResult = ""
	default:
		app.ProviderResult = model.ProviderResultFail
	}
	if err := s.repo.UpdateProviderResult(ctx, app); err != nil {
		return err
	}
	action := model.ActionProviderFail
	if res.Passed {
		action = model.ActionProviderPass
	}
	_ = s.repo.WriteReviewLog(ctx, &model.VerificationReviewLog{
		ApplicationID: app.ID,
		FromStatus:    app.Status,
		ToStatus:      app.Status,
		Action:        action,
		OperatorName:  providerType,
		Note:          app.ProviderMessage,
	})
	// 自动放行 / 自动驳回（默认关闭）。回调没有管理员身份，操作人记 system。
	if res.Passed && s.configBool(ctx, model.ConfigKeyAutoApproveOnPass, false) {
		if _, err := s.Approve(ctx, app.ID, dto.ReviewRequest{Note: "三方核验通过自动放行"}, systemOperatorID, "system"); err != nil {
			s.logger.Warn("auto approve on provider callback failed", zap.Error(err), zap.Uint64("application_id", app.ID))
		}
	}
	if app.ProviderResult == model.ProviderResultFail && s.configBool(ctx, model.ConfigKeyAutoRejectOnFail, false) {
		if _, err := s.Reject(ctx, app.ID, dto.ReviewRequest{
			RejectReason:     "三方核验未通过：" + app.ProviderMessage,
			RejectReasonCode: "provider_failed",
		}, systemOperatorID, "system"); err != nil {
			s.logger.Warn("auto reject on provider callback failed", zap.Error(err), zap.Uint64("application_id", app.ID))
		}
	}
	return nil
}

// systemOperatorID 系统自动动作的 operator_id。
//
// transition 拒绝 operator_id=0（那代表「操作人未知」），而系统自动放行是
// 有明确主体的动作，因此用一个保留 ID + operator_name="system" 表达。
// 值取 1 与既有审计表的系统账号口径一致。
const systemOperatorID = 1

// IsRealnameVerified 实名判定的唯一口径（doc104 §5.3）。
//
// 只认 users.real_name_verified_at：users.real_name 是展示名（资料表单可直写），
// 用它判断是否已实名就是那个必须修掉的「实名后门」。
func (s *verificationService) IsRealnameVerified(ctx context.Context, userID uint64) (bool, error) {
	return s.repo.IsUserVerified(ctx, userID)
}

// ---------- 配置读取工具 ----------

func (s *verificationService) configBool(ctx context.Context, key string, fallback bool) bool {
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

func (s *verificationService) configInt(ctx context.Context, key string, fallback int) int {
	if s.cfg == nil {
		return fallback
	}
	raw, ok, err := s.cfg(ctx, key)
	if err != nil || !ok {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

// allowedTypes 允许的认证类型；配置缺失或非法时回落「两种都允许」。
func (s *verificationService) allowedTypes(ctx context.Context) []string {
	defaults := []string{model.TypePersonal, model.TypeEnterprise}
	if s.cfg == nil {
		return defaults
	}
	raw, ok, err := s.cfg(ctx, model.ConfigKeyAllowedTypes)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return defaults
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out) == 0 {
		return defaults
	}
	return out
}

func (s *verificationService) typeAllowed(ctx context.Context, vType string) bool {
	for _, t := range s.allowedTypes(ctx) {
		if t == vType {
			return true
		}
	}
	return false
}

// effectiveProviderType 当前生效的核验服务商类型。
//
// 配置里指定的类型若未注册适配器，回落 manual（人工审核）而不是报错：
// 「配了一个还没接入的 provider」不应该让用户无法提交实名。
func (s *verificationService) effectiveProviderType(ctx context.Context) string {
	configured := ""
	if s.cfg != nil {
		if raw, ok, err := s.cfg(ctx, model.ConfigKeyProvider); err == nil && ok {
			configured = strings.TrimSpace(raw)
		}
	}
	if configured == "" {
		return "manual"
	}
	if !realnamepkg.IsRegistered(configured) {
		s.logger.Warn("configured realname provider not registered, fallback to manual",
			zap.String("provider", configured))
		return "manual"
	}
	return configured
}

// providerConfig 构造 provider 调用配置（含解密后的凭证）。
func (s *verificationService) providerConfig(row *model.RealnameProvider) (realnamepkg.ProviderConfig, error) {
	m, err := credentials.Decode(row.Credentials)
	if err != nil {
		return realnamepkg.ProviderConfig{}, err
	}
	plain, err := m.DecryptFields(s.encryptKey)
	if err != nil {
		return realnamepkg.ProviderConfig{}, err
	}
	creds := map[string]string{}
	for k, v := range plain {
		creds[k] = v
	}
	return realnamepkg.ProviderConfig{
		ProviderID:  row.ID,
		Type:        row.ProviderType,
		Endpoint:    row.Endpoint,
		Credentials: creds,
	}, nil
}

// certifyModeOf 取配置里的认证方式（FACE / CERT_PHOTO）。
func certifyModeOf(row *model.RealnameProvider) string {
	if row == nil {
		return ""
	}
	m, err := credentials.Decode(row.Credentials)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(m["certify_mode"])
}

// encryptCredentials 按描述符加密 secret 字段（幂等：已加密原样保留）。
func (s *verificationService) encryptCredentials(d realnamepkg.CapabilityDescriptor, values map[string]string) (string, error) {
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

// encryptSecret 单值加密（证件号 / 手机号）。
//
// 复用 pkg/credentials 的单字段形态：密文格式与渠道凭证完全一致（enc:v1: 前缀），
// 运维只需记住一套格式，密钥轮换也只有一处。
func (s *verificationService) encryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	m := credentials.Map{"value": value}
	encrypted, err := credentials.EncryptFields(m, []integration.Field{{Key: "value", Secret: true}}, s.encryptKey)
	if err != nil {
		return "", err
	}
	return encrypted["value"], nil
}

// decryptSecret 单值解密。解密失败必须报错：绝不能让密文被当作明文传给 provider。
func (s *verificationService) decryptSecret(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	m := credentials.Map{"value": cipherText}
	plain, err := m.DecryptFields(s.encryptKey)
	if err != nil {
		return "", err
	}
	return plain["value"], nil
}

// ---------- 校验与格式化 ----------

// normalizeID 校验并规整证件号。
//
// 个人认证要求 18 位身份证（末位可为 X）；企业认证要求 18 位统一社会信用代码。
// 只做格式校验不做校验位计算：真正的有效性由三方核验/人工审核判定，
// 在这里算校验位反而会在遇到历史证件（15 位身份证）时误拒。
func normalizeID(vType, idType, idNumber string) (string, string, error) {
	raw := strings.ToUpper(strings.TrimSpace(idNumber))
	if raw == "" {
		return "", "", errors.New("证件号不能为空")
	}
	if vType == model.TypeEnterprise {
		if idType == "" {
			idType = "CREDIT_CODE"
		}
		if len(raw) != 18 {
			return "", "", fmt.Errorf("%w: 统一社会信用代码应为 18 位", ErrIDNumberInvalid)
		}
		return idType, raw, nil
	}
	if idType == "" {
		idType = "IDENTITY_CARD"
	}
	if len(raw) != 18 {
		return "", "", fmt.Errorf("%w: 身份证号应为 18 位", ErrIDNumberInvalid)
	}
	for i, r := range raw {
		if i == 17 && r == 'X' {
			continue
		}
		if r < '0' || r > '9' {
			return "", "", fmt.Errorf("%w: 身份证号只能包含数字（末位可为 X）", ErrIDNumberInvalid)
		}
	}
	return idType, raw, nil
}

// maskIDNumber 证件号脱敏：保留前 6 后 4（与既有 id_number_masked 列口径一致）。
func maskIDNumber(id string) string {
	if len(id) <= 10 {
		return strings.Repeat("*", len(id))
	}
	return id[:6] + strings.Repeat("*", len(id)-10) + id[len(id)-4:]
}

// maskMobile 手机号脱敏：保留前 3 后 4。
func maskMobile(mobile string) string {
	if mobile == "" {
		return ""
	}
	if len(mobile) <= 7 {
		return strings.Repeat("*", len(mobile))
	}
	return mobile[:3] + strings.Repeat("*", len(mobile)-7) + mobile[len(mobile)-4:]
}

// maskCreditCode 统一社会信用代码脱敏：保留前 4 后 4。
func maskCreditCode(code string) string {
	if len(code) <= 8 {
		return code
	}
	return code[:4] + strings.Repeat("*", len(code)-8) + code[len(code)-4:]
}

// subjectName 认证主体名：显式传入优先，否则回落真实姓名。
func subjectName(vType, subject, realName string) string {
	if s := strings.TrimSpace(subject); s != "" {
		return s
	}
	return realName
}

func normalizeMeta(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// containsStatus 判断当前状态是否在允许的起始状态集合内。
func containsStatus(allowed []string, status string) bool {
	for _, s := range allowed {
		if s == status {
			return true
		}
	}
	return false
}

func statusLabel(status string) string {
	switch status {
	case model.StatusPending:
		return "待审核"
	case model.StatusApproved:
		return "已通过"
	case model.StatusRejected:
		return "已驳回"
	default:
		return status
	}
}

// truncate 按字节截断并对齐 UTF-8 边界（provider_message 是 varchar(255)）。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func toVerificationInfo(item model.VerificationApplication) dto.VerificationInfo {
	return dto.VerificationInfo{
		ID:                item.ID,
		UserID:            item.UserID,
		Username:          item.Username,
		VerificationType:  item.VerificationType,
		Status:            item.Status,
		RealName:          item.RealName,
		SubjectName:       item.SubjectName,
		IDType:            item.IDType,
		IDNumberMasked:    item.IDNumberMasked,
		MobileMasked:      item.MobileMasked,
		RiskFlags:         item.RiskFlags,
		SubmittedAt:       item.SubmittedAt,
		ReviewedAt:        item.ReviewedAt,
		ReviewedBy:        item.ReviewedBy,
		ReviewerName:      item.ReviewerName,
		RejectReasonCode:  item.RejectReasonCode,
		RejectReason:      item.RejectReason,
		ReviewNote:        item.ReviewNote,
		Provider:          item.Provider,
		ProviderResult:    item.ProviderResult,
		ProviderMessage:   item.ProviderMessage,
		ProviderCheckedAt: item.ProviderCheckedAt,
		ReviewRound:       item.ReviewRound,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toConfigInfo(row *model.VerificationConfig) dto.VerificationConfigInfo {
	return dto.VerificationConfigInfo{
		ID:          row.ID,
		ConfigKey:   row.ConfigKey,
		ConfigGroup: row.ConfigGroup,
		ConfigValue: row.ConfigValue,
		ValueType:   row.ValueType,
		Status:      row.Status,
		Description: row.Description,
		UpdatedBy:   row.UpdatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
