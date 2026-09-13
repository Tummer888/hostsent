package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	notifyrepo "hostsent/backend/internal/modules/admin/notification/repository"
	sysconfigrepo "hostsent/backend/internal/modules/admin/system/repository"
	"hostsent/backend/internal/pkg/notifier"
)

// smsTemplateNamePattern 模板编码：小写字母开头，允许下划线数字。
var smsTemplateNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// cnMobilePattern 国内手机号。
var cnMobilePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// SmsTemplateService 短信模板管理与预览。
type SmsTemplateService interface {
	List(ctx context.Context, q notifydto.SmsTemplateListQuery) (*notifydto.SmsTemplateListResponse, error)
	Get(ctx context.Context, id uint64) (*notifydto.SmsTemplateInfo, error)
	Create(ctx context.Context, req notifydto.SmsTemplateSaveRequest) (*notifydto.SmsTemplateInfo, error)
	Update(ctx context.Context, id uint64, req notifydto.SmsTemplateSaveRequest) (*notifydto.SmsTemplateInfo, error)
	Delete(ctx context.Context, id uint64) error
	// Preview 用注册变量样例值（或调用方传入值）渲染预览。
	Preview(ctx context.Context, req notifydto.SmsTemplatePreviewRequest) (*notifydto.SmsTemplatePreviewResponse, error)
}

type smsTemplateService struct {
	repo    notifyrepo.SmsTemplateRepository
	varRepo notifyrepo.TemplateVarRepository
}

// NewSmsTemplateService 创建短信模板服务。
func NewSmsTemplateService(repo notifyrepo.SmsTemplateRepository, varRepo notifyrepo.TemplateVarRepository) SmsTemplateService {
	return &smsTemplateService{repo: repo, varRepo: varRepo}
}

// validateAndExtract 解析正文变量并与注册表白名单比对；未注册变量直接拒存并回传变量名。
//
// 这比现状「缺变量就原样把 {var} 发出去」安全得多：运营在保存阶段就能发现拼写错误。
func (s *smsTemplateService) validateAndExtract(ctx context.Context, content string) ([]string, error) {
	found := ExtractVarNames(content)
	if len(found) == 0 {
		return []string{}, nil
	}
	registered, err := s.varRepo.ListActiveKeys(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(registered))
	for _, k := range registered {
		allowed[k] = true
	}
	for _, key := range found {
		if !allowed[key] {
			return nil, fmt.Errorf("%w: {%s}，请先在变量注册表补登记", ErrUnregisteredVar, key)
		}
	}
	return found, nil
}

func (s *smsTemplateService) List(ctx context.Context, q notifydto.SmsTemplateListQuery) (*notifydto.SmsTemplateListResponse, error) {
	items, total, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]notifydto.SmsTemplateInfo, 0, len(items))
	for i := range items {
		out = append(out, toSmsTemplateInfo(&items[i]))
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 10
	}
	return &notifydto.SmsTemplateListResponse{
		Items: out,
		Meta:  notifydto.ListMeta{Page: page, PageSize: size, Total: total},
	}, nil
}

func (s *smsTemplateService) Get(ctx context.Context, id uint64) (*notifydto.SmsTemplateInfo, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSmsTemplateNotFound
		}
		return nil, err
	}
	info := toSmsTemplateInfo(t)
	return &info, nil
}

func (s *smsTemplateService) Create(ctx context.Context, req notifydto.SmsTemplateSaveRequest) (*notifydto.SmsTemplateInfo, error) {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = genTemplateCode(req.Name)
	}
	if !smsTemplateNamePattern.MatchString(code) {
		return nil, fmt.Errorf("%w: 模板编码只能是小写字母、数字与下划线，且以字母开头", ErrInvalidParams)
	}
	if _, err := s.repo.FindByCode(ctx, code); err == nil {
		return nil, ErrSmsTemplateCodeExists
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	varNames, err := s.validateAndExtract(ctx, req.Content)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(varNames)
	t := &notifymodel.SmsTemplate{
		Code:         code,
		Name:         req.Name,
		Scene:        firstNonEmptyString(req.Scene, "notify"),
		Content:      req.Content,
		UpstreamCode: req.UpstreamCode,
		VarNames:     string(raw),
		Status:       firstNonEmptyString(req.Status, notifymodel.SmsTemplateStatusActive),
		Remark:       req.Remark,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	info := toSmsTemplateInfo(t)
	return &info, nil
}

func (s *smsTemplateService) Update(ctx context.Context, id uint64, req notifydto.SmsTemplateSaveRequest) (*notifydto.SmsTemplateInfo, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSmsTemplateNotFound
		}
		return nil, err
	}
	varNames, err := s.validateAndExtract(ctx, req.Content)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(varNames)
	t.Name = req.Name
	t.Content = req.Content
	t.UpstreamCode = req.UpstreamCode
	t.VarNames = string(raw)
	if req.Scene != "" {
		t.Scene = req.Scene
	}
	if req.Status != "" {
		t.Status = req.Status
	}
	t.Remark = req.Remark
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	info := toSmsTemplateInfo(t)
	return &info, nil
}

// Delete 物理删除模板（未被任何通知模板引用的前提下）。
func (s *smsTemplateService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrSmsTemplateNotFound
		}
		return err
	}
	// 被通知模板引用时不允许删：删了会导致模板渲染取不到正文。
	if referenced, err := s.repo.ReferencedByNotificationTemplate(ctx, id); err != nil {
		return err
	} else if referenced {
		return ErrSmsTemplateInUse
	}
	return s.repo.Delete(ctx, id)
}

func (s *smsTemplateService) Preview(ctx context.Context, req notifydto.SmsTemplatePreviewRequest) (*notifydto.SmsTemplatePreviewResponse, error) {
	vars := map[string]string{}
	// 先用注册表 sample 铺底，再用调用方传入值覆盖。
	if items, err := s.varRepo.List(ctx, ""); err == nil {
		for _, v := range items {
			vars[v.VarKey] = v.Sample
		}
	}
	for k, v := range req.Vars {
		vars[k] = v
	}
	rendered := RenderTemplate(req.Content, vars)
	chars, segments := SmsSegments(rendered)
	return &notifydto.SmsTemplatePreviewResponse{
		Rendered:  rendered,
		CharCount: chars,
		Segments:  segments,
	}, nil
}

// TemplateVarService 模板变量注册表管理（D7）。
type TemplateVarService interface {
	List(ctx context.Context, category string) ([]notifydto.TemplateVarInfo, error)
	Create(ctx context.Context, req notifydto.TemplateVarSaveRequest) (*notifydto.TemplateVarInfo, error)
	Update(ctx context.Context, id uint64, req notifydto.TemplateVarSaveRequest) (*notifydto.TemplateVarInfo, error)
	// Disable 逻辑停用（不物理删），保留历史模板的变量可读性。
	Disable(ctx context.Context, id uint64) error
}

type templateVarService struct {
	repo notifyrepo.TemplateVarRepository
}

// NewTemplateVarService 创建变量注册表服务。
func NewTemplateVarService(repo notifyrepo.TemplateVarRepository) TemplateVarService {
	return &templateVarService{repo: repo}
}

func (s *templateVarService) List(ctx context.Context, category string) ([]notifydto.TemplateVarInfo, error) {
	items, err := s.repo.List(ctx, category)
	if err != nil {
		return nil, err
	}
	out := make([]notifydto.TemplateVarInfo, 0, len(items))
	for i := range items {
		out = append(out, toTemplateVarInfo(&items[i]))
	}
	return out, nil
}

func (s *templateVarService) Create(ctx context.Context, req notifydto.TemplateVarSaveRequest) (*notifydto.TemplateVarInfo, error) {
	key := strings.TrimSpace(req.VarKey)
	if !smsTemplateNamePattern.MatchString(key) {
		return nil, fmt.Errorf("%w: 变量名只能是小写字母、数字与下划线，且以字母开头", ErrInvalidParams)
	}
	if _, err := s.repo.FindByKey(ctx, key); err == nil {
		return nil, ErrTemplateVarKeyExists
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	v := &notifymodel.SmsTemplateVar{
		VarKey:      key,
		Label:       req.Label,
		Category:    firstNonEmptyString(req.Category, "common"),
		ValueType:   firstNonEmptyString(req.ValueType, "string"),
		Sample:      req.Sample,
		Description: req.Description,
		Scenes:      encodeJSONArray(req.Scenes),
		SortOrder:   req.SortOrder,
		Status:      firstNonEmptyString(req.Status, notifymodel.TemplateVarStatusActive),
	}
	if err := s.repo.Create(ctx, v); err != nil {
		return nil, err
	}
	info := toTemplateVarInfo(v)
	return &info, nil
}

func (s *templateVarService) Update(ctx context.Context, id uint64, req notifydto.TemplateVarSaveRequest) (*notifydto.TemplateVarInfo, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrTemplateVarNotFound
		}
		return nil, err
	}
	v.Label = req.Label
	if req.Category != "" {
		v.Category = req.Category
	}
	if req.ValueType != "" {
		v.ValueType = req.ValueType
	}
	v.Sample = req.Sample
	v.Description = req.Description
	if req.Scenes != nil {
		v.Scenes = encodeJSONArray(req.Scenes)
	}
	v.SortOrder = req.SortOrder
	if req.Status != "" {
		v.Status = req.Status
	}
	if err := s.repo.Update(ctx, v); err != nil {
		return nil, err
	}
	info := toTemplateVarInfo(v)
	return &info, nil
}

// Disable 逻辑停用：只改状态，不删行。已用该变量的历史模板仍能正常渲染。
func (s *templateVarService) Disable(ctx context.Context, id uint64) error {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrTemplateVarNotFound
		}
		return err
	}
	v.Status = notifymodel.TemplateVarStatusDisabled
	return s.repo.Update(ctx, v)
}

// ---- 测试发送 ----

// TestRateCounter 测试发送频控计数器（Redis；不可用时 Enabled() 返回 false 即降级放行）。
type TestRateCounter interface {
	Enabled() bool
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// testSendLimitPerMinute 每管理员每分钟测试发送上限。
const testSendLimitPerMinute = 10

// TestSendService 后台「短信测试 / 邮箱测试」统一入口。
type TestSendService interface {
	Send(ctx context.Context, adminID uint64, req notifydto.TestSendRequest) (*notifydto.TestSendResponse, error)
}

type testSendService struct {
	resolver  ChannelResolver
	chRepo    notifyrepo.ChannelRepository
	smsRepo   notifyrepo.SmsTemplateRepository
	varRepo   notifyrepo.TemplateVarRepository
	delivRepo notifyrepo.DeliveryRepository
	cfgRepo   sysconfigrepo.ConfigRepository
	counter   TestRateCounter
	logger    *zap.Logger
}

// NewTestSendService 创建测试发送服务。
func NewTestSendService(
	resolver ChannelResolver,
	chRepo notifyrepo.ChannelRepository,
	smsRepo notifyrepo.SmsTemplateRepository,
	varRepo notifyrepo.TemplateVarRepository,
	delivRepo notifyrepo.DeliveryRepository,
	cfgRepo sysconfigrepo.ConfigRepository,
	counter TestRateCounter,
	logger *zap.Logger,
) TestSendService {
	return &testSendService{
		resolver: resolver, chRepo: chRepo, smsRepo: smsRepo, varRepo: varRepo,
		delivRepo: delivRepo, cfgRepo: cfgRepo, counter: counter, logger: logger,
	}
}

func (s *testSendService) Send(ctx context.Context, adminID uint64, req notifydto.TestSendRequest) (*notifydto.TestSendResponse, error) {
	// 1. 目标格式校验。
	target := strings.TrimSpace(req.Target)
	switch req.Category {
	case notifier.CategoryMail:
		if _, err := mail.ParseAddress(target); err != nil {
			return nil, fmt.Errorf("%w: 邮箱格式不正确", ErrInvalidParams)
		}
	case notifier.CategorySMS:
		if !cnMobilePattern.MatchString(target) {
			return nil, fmt.Errorf("%w: 手机号格式不正确", ErrInvalidParams)
		}
	default:
		return nil, fmt.Errorf("%w: 通道类别只支持 mail / sms", ErrInvalidParams)
	}

	// 2. 频控：防止被当短信轰炸机用（Redis 不可用时降级放行）。
	if s.counter != nil && s.counter.Enabled() {
		key := fmt.Sprintf("notify:test:%d", adminID)
		n, err := s.counter.Incr(ctx, key, time.Minute)
		if err == nil && n > testSendLimitPerMinute {
			return nil, fmt.Errorf("%w: 测试发送过于频繁，请稍后再试", ErrInvalidParams)
		}
	}

	// 3. 解析渠道：指定 ID 优先，否则取该类别默认渠道。
	var cfg *notifier.ChannelConfig
	var err error
	if req.ChannelID > 0 {
		cfg, err = s.resolver.ResolveByID(ctx, req.ChannelID)
	} else {
		cfg, err = s.resolver.Resolve(ctx, req.Category, notifymodel.ChannelSceneTest)
	}
	if err != nil {
		return nil, err
	}

	// 4. 组装消息：短信走模板或纯文本；邮件支持 HTML。
	msg := notifier.Message{
		Category:  req.Category,
		Recipient: target,
		SignName:  cfg.SignName,
		Format:    notifier.FormatText,
	}
	if req.Category == notifier.CategoryMail {
		msg.Subject = "HostSent 测试邮件"
	}
	if code := strings.TrimSpace(req.TemplateCode); code != "" && req.Category == notifier.CategorySMS {
		tpl, terr := s.smsRepo.FindByCode(ctx, code)
		if terr != nil {
			if terr == gorm.ErrRecordNotFound {
				return nil, ErrSmsTemplateNotFound
			}
			return nil, terr
		}
		vars := map[string]string{}
		if items, verr := s.varRepo.List(ctx, ""); verr == nil {
			for _, v := range items {
				vars[v.VarKey] = v.Sample
			}
		}
		// 未传变量时用注册表样例值，让运营看到真实长度与占位效果。
		for k, v := range req.Vars {
			vars[k] = v
		}
		msg.Body = RenderTemplate(tpl.Content, vars)
		msg.TemplateID = firstNonEmptyString(tpl.UpstreamCode, cfg.TemplateCode)
		msg.Vars = vars
	} else {
		msg.Body = firstNonEmptyString(strings.TrimSpace(req.Content), "这是一条来自 HostSent 系统的测试消息，收到即表示通道配置正常。")
	}

	// 5. 同步发送（测试要立即拿结果），但不入队；仍写一条 source_module='test'
	//    的投递记录，使测试也进发送日志可查。
	sender, err := notifier.New(cfg.Type, *cfg)
	if err != nil {
		if err == notifier.ErrAdapterNotImplemented {
			return &notifydto.TestSendResponse{
				OK: false, Pending: true, ChannelID: cfg.ChannelID,
				Message: "该服务商适配器待接入，已完成配置占位",
			}, nil
		}
		return nil, err
	}
	res, sendErr := sender.Send(ctx, *cfg, msg)
	row := &notifymodel.NotificationDelivery{
		Event:         "test_send",
		Channel:       req.Category,
		TargetType:    notifymodel.TargetAdmin,
		TargetID:      adminID,
		Recipient:     target,
		ChannelID:     cfg.ChannelID,
		Title:         msg.Subject,
		Content:       msg.Body,
		ContentFormat: notifier.FormatText,
		SendStatus:    notifymodel.DeliveryStatusPending,
		SourceModule:  notifymodel.SourceModuleTest,
		SourceID:      genSourceID(adminID),
	}
	if row.Title == "" {
		row.Title = "测试发送"
	}
	if sendErr != nil {
		row.SendStatus = notifymodel.DeliveryStatusFailed
		row.FailReason = truncateRunes(sendErr.Error(), 500)
		_ = s.delivRepo.Enqueue(ctx, row)
		_ = s.chRepo.UpdateHealth(ctx, cfg.ChannelID, notifymodel.HealthDown, sendErr.Error())
		return &notifydto.TestSendResponse{
			OK: false, ChannelID: cfg.ChannelID, ChannelName: channelDisplayName(cfg),
			Message: sendErr.Error(),
		}, nil
	}
	row.SendStatus = notifymodel.DeliveryStatusSent
	row.ProviderMsgID = res.ProviderMsgID
	row.ProviderCode = res.ProviderCode
	row.CostFen = res.CostFen
	now := time.Now()
	row.SentAt = &now
	_ = s.delivRepo.Enqueue(ctx, row)
	_ = s.chRepo.UpdateHealth(ctx, cfg.ChannelID, notifymodel.HealthHealthy, "")
	return &notifydto.TestSendResponse{
		OK:            true,
		ChannelID:     cfg.ChannelID,
		ChannelName:   channelDisplayName(cfg),
		ProviderCode:  res.ProviderCode,
		ProviderMsgID: res.ProviderMsgID,
		CostFen:       res.CostFen,
		Message:       "发送成功",
	}, nil
}

// ---- 转换与工具 ----

func toSmsTemplateInfo(t *notifymodel.SmsTemplate) notifydto.SmsTemplateInfo {
	var names []string
	if strings.TrimSpace(t.VarNames) != "" {
		_ = json.Unmarshal([]byte(t.VarNames), &names)
	}
	chars, segments := SmsSegments(t.Content)
	return notifydto.SmsTemplateInfo{
		ID:           t.ID,
		Code:         t.Code,
		Name:         t.Name,
		Scene:        t.Scene,
		Content:      t.Content,
		UpstreamCode: t.UpstreamCode,
		VarNames:     names,
		Status:       t.Status,
		Remark:       t.Remark,
		CharCount:    chars,
		Segments:     segments,
		CreatedAt:    t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    t.UpdatedAt.Format(time.RFC3339),
	}
}

func toTemplateVarInfo(v *notifymodel.SmsTemplateVar) notifydto.TemplateVarInfo {
	var scenes []string
	if strings.TrimSpace(v.Scenes) != "" {
		_ = json.Unmarshal([]byte(v.Scenes), &scenes)
	}
	return notifydto.TemplateVarInfo{
		ID:          v.ID,
		VarKey:      v.VarKey,
		Label:       v.Label,
		Category:    v.Category,
		ValueType:   v.ValueType,
		Sample:      v.Sample,
		Description: v.Description,
		Scenes:      scenes,
		SortOrder:   v.SortOrder,
		Status:      v.Status,
	}
}

func encodeJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func genTemplateCode(name string) string {
	code := strings.ToLower(strings.TrimSpace(name))
	code = strings.NewReplacer(" ", "_", "-", "_").Replace(code)
	code = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(code, "")
	if code == "" || !smsTemplateNamePattern.MatchString(code) {
		code = "tpl_" + time.Now().Format("20060102150405")
	}
	return code
}

func genSourceID(adminID uint64) string {
	return fmt.Sprintf("%d-%d", adminID, time.Now().UnixNano())
}

func channelDisplayName(cfg *notifier.ChannelConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.ChannelCode != "" {
		return cfg.ChannelCode
	}
	return cfg.Type
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
