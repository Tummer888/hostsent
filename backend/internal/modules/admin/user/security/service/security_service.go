package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/security/dto"
	"hostsent/backend/internal/modules/admin/user/security/model"
	"hostsent/backend/internal/modules/admin/user/security/repository"
	"hostsent/backend/internal/pkg/security"
)

// SecurityService 安全与风控管理服务。
//
// 所有写操作都要求调用方显式传入 operatorID（管理端操作人 ID），而不是在服务内部
// 取一个常量。此前这里硬编码 1，导致封禁、踢会话、处置风险事件全部记在 admin 名下，
// 审计链路形同虚设（doc104 F5）。
type SecurityService interface {
	ListLoginLogs(ctx context.Context, query dto.LoginLogListQuery) (*dto.ListResponse[dto.LoginLogInfo], error)
	GetLoginLog(ctx context.Context, id uint64) (*dto.LoginLogInfo, error)
	ExportLoginLogs(ctx context.Context, query dto.LoginLogListQuery) ([]dto.LoginLogInfo, error)
	ListAuditLogs(ctx context.Context, query dto.AuditLogListQuery) (*dto.ListResponse[dto.AuditLogInfo], error)
	GetAuditLog(ctx context.Context, id uint64) (*dto.AuditLogInfo, error)
	ListRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (*dto.ListResponse[dto.RiskEventInfo], error)
	GetRiskEvent(ctx context.Context, id uint64) (*dto.RiskEventInfo, error)
	// ListRiskEventActions 某事件的处置时间线（详情抽屉）。
	ListRiskEventActions(ctx context.Context, id uint64) (*dto.ListResponse[dto.RiskEventActionInfo], error)
	// CountRiskEvents 风险事件汇总（待处理 / 已处置 / 已忽略 / 总数）。
	CountRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (map[string]int64, error)
	IgnoreRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest, operatorID uint64) (*dto.RiskEventInfo, error)
	HandleRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest, operatorID uint64) (*dto.RiskEventInfo, error)
	// UpdateRiskEventLevel 手动升级 / 下调风险等级（doc06 §4.3 操作建议）。
	//
	// 规则引擎只能按固定阈值定级；真实场景里「连续 5 次失败」在促销日可能纯属正常，
	// 而运营一眼看出某条事件其实很严重 —— 必须允许人工改级，且留处置痕迹。
	UpdateRiskEventLevel(ctx context.Context, id uint64, req dto.RiskEventLevelRequest, operatorID uint64) (*dto.RiskEventInfo, error)
	CreateBlacklistFromRisk(ctx context.Context, id uint64, req dto.RiskEventBlacklistRequest, operatorID uint64) (*dto.BlacklistInfo, error)
	RevokeSessionsFromRisk(ctx context.Context, id uint64, req dto.RiskEventRevokeRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error)
	ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) (*dto.ListResponse[dto.BlacklistInfo], error)
	CreateBlacklist(ctx context.Context, req dto.BlacklistCreateRequest, operatorID uint64) (*dto.BlacklistInfo, error)
	GetBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error)
	UpdateBlacklist(ctx context.Context, id uint64, req dto.BlacklistUpdateRequest, operatorID uint64) (*dto.BlacklistInfo, error)
	UpdateBlacklistStatus(ctx context.Context, id uint64, req dto.BlacklistStatusRequest, operatorID uint64) (*dto.BlacklistInfo, error)
	ReleaseBlacklist(ctx context.Context, id uint64, operatorID uint64) (*dto.BlacklistInfo, error)
	ListBlacklistHits(ctx context.Context, id uint64, query dto.BlacklistHitListQuery) (*dto.ListResponse[dto.LoginLogInfo], error)
	ListSessions(ctx context.Context, query dto.SessionListQuery) (*dto.ListResponse[dto.SessionInfo], error)
	GetSession(ctx context.Context, id uint64) (*dto.SessionInfo, error)
	RevokeSession(ctx context.Context, id uint64, req dto.SessionRevokeRequest, operatorID uint64) (*dto.SessionInfo, error)
	BatchRevokeSessions(ctx context.Context, req dto.SessionBatchRevokeRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error)
	RevokeUserAllSessions(ctx context.Context, req dto.SessionRevokeUserAllRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error)
	// ExpireStaleSessions 把已过期的 active 会话收成 expired（定时任务调用）。
	// 返回本轮处理的会话数。
	ExpireStaleSessions(ctx context.Context, limit int) (int, error)
	// SetSessionInvalidator 注入会话缓存失效能力（可选，装配层调用）。
	SetSessionInvalidator(invalidator SessionInvalidator)
}

// SessionInvalidator 让会话有效性缓存立即失效（由 pkg/sessionguard.Guard 实现）。
//
// 「强制下线」要真正生效，光把 user_sessions.status 改成 revoked 不够：
// 请求链路上还有一层会话校验缓存（pkg/sessionguard），不失效它的话，
// 被踢的令牌最多还能再用 5 分钟（positiveTTL）。
// 用中性接口而不是直接依赖 sessionguard 包，保持 admin 模块不反向依赖 pkg 具体实现。
type SessionInvalidator interface {
	Invalidate(ctx context.Context, sessionIDs ...string)
}

type securityService struct {
	repo        repository.SecurityRepository
	invalidator SessionInvalidator // 可选：撤销会话后立即失效其校验缓存
	logger      *zap.Logger
}

func NewSecurityService(repo repository.SecurityRepository) SecurityService {
	return &securityService{repo: repo, logger: zap.NewNop()}
}

// SetSessionInvalidator 注入会话缓存失效能力（装配层调用）。
func (s *securityService) SetSessionInvalidator(invalidator SessionInvalidator) {
	s.invalidator = invalidator
}

// invalidateSessions 让一批已撤销会话的校验缓存立即失效。
//
// 失败不返回错误：撤销本身已经落库，缓存删不掉最坏只是延迟到 TTL 自然过期，
// 不该让「强制下线」这个动作整体失败。
func (s *securityService) invalidateSessions(ctx context.Context, sessionIDs []string) {
	if s.invalidator == nil || len(sessionIDs) == 0 {
		return
	}
	s.invalidator.Invalidate(ctx, sessionIDs...)
}

// sessionIDsOf 从会话行里取出 session_id 列表（撤销后调用失效用）。
func sessionIDsOf(sessions []model.Session) []string {
	ids := make([]string, 0, len(sessions))
	for i := range sessions {
		if sessions[i].SessionID != "" {
			ids = append(ids, sessions[i].SessionID)
		}
	}
	return ids
}

func (s *securityService) ListLoginLogs(ctx context.Context, query dto.LoginLogListQuery) (*dto.ListResponse[dto.LoginLogInfo], error) {
	items, total, err := s.repo.ListLoginLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.LoginLogInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toLoginLogInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) GetLoginLog(ctx context.Context, id uint64) (*dto.LoginLogInfo, error) {
	item, err := s.repo.GetLoginLog(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "登录日志不存在")
	}
	result := toLoginLogInfo(*item)
	return &result, nil
}

// ExportLoginLogs 导出登录日志（与审计日志导出口径一致：按筛选条件取前 N 条）。
//
// 导出走与列表相同的仓储查询，因此「界面上筛出来的」与「导出的」必然一致——
// 旧实现返回一个常量字符串，导出的其实是一句作业名，用户拿到手里是个空文件。
func (s *securityService) ExportLoginLogs(ctx context.Context, query dto.LoginLogListQuery) ([]dto.LoginLogInfo, error) {
	// 上限 1000 条：与审计日志导出一致，避免一次拉爆内存。
	if query.PageSize <= 0 || query.PageSize > 1000 {
		query.PageSize = 1000
	}
	query.Page = 1
	resp, err := s.ListLoginLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (s *securityService) ListAuditLogs(ctx context.Context, query dto.AuditLogListQuery) (*dto.ListResponse[dto.AuditLogInfo], error) {
	items, total, err := s.repo.ListAuditLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.AuditLogInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toAuditLogInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) GetAuditLog(ctx context.Context, id uint64) (*dto.AuditLogInfo, error) {
	item, err := s.repo.GetAuditLog(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "审计日志不存在")
	}
	result := toAuditLogInfo(*item)
	return &result, nil
}

func (s *securityService) ListRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (*dto.ListResponse[dto.RiskEventInfo], error) {
	items, total, err := s.repo.ListRiskEvents(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.RiskEventInfo, 0, len(items))
	handledByIDs := make([]uint64, 0, len(items))
	eventIDs := make([]uint64, 0, len(items))
	for i := range items {
		result = append(result, toRiskEventInfo(items[i]))
		eventIDs = append(eventIDs, items[i].ID)
		if items[i].HandledBy != nil {
			handledByIDs = append(handledByIDs, *items[i].HandledBy)
		}
	}
	// 批量补处置人账号名：一页 10 条逐个查会变成 10 次查询（N+1），
	// 而这是运营每翻一页都会走的路径。
	if names, err := s.repo.AdminNames(ctx, handledByIDs); err == nil {
		for i := range result {
			result[i].HandledByName = names[result[i].HandledBy]
		}
	}
	// 批量补处置动作：列表上必须能看出「这条已经拉黑 / 已经踢过会话」。
	// 与 status 分开显示 —— status 说「待办关掉没有」，动作说「做了哪些管控」，
	// 两者可以同时成立（还要盯，但已经封了）。
	s.fillActionSummary(ctx, result, eventIDs)
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) GetRiskEvent(ctx context.Context, id uint64) (*dto.RiskEventInfo, error) {
	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	result := toRiskEventInfo(*item)
	if item.HandledBy != nil {
		if names, err := s.repo.AdminNames(ctx, []uint64{*item.HandledBy}); err == nil {
			result.HandledByName = names[*item.HandledBy]
		}
	}
	if actions, err := s.repo.ListRiskEventActionsByEvents(ctx, []uint64{id}); err == nil {
		summary := summarizeActions(actions[id])
		result.Actions = summary.codes
		result.ActionSummary = summary.text
	}
	return &result, nil
}

// ListRiskEventActions 事件处置时间线。
func (s *securityService) ListRiskEventActions(ctx context.Context, id uint64) (*dto.ListResponse[dto.RiskEventActionInfo], error) {
	if _, err := s.repo.GetRiskEvent(ctx, id); err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	items, err := s.repo.ListRiskEventActions(ctx, id)
	if err != nil {
		return nil, err
	}
	operatorIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		operatorIDs = append(operatorIDs, item.OperatorID)
	}
	names, _ := s.repo.AdminNames(ctx, operatorIDs)
	result := make([]dto.RiskEventActionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, dto.RiskEventActionInfo{
			ID:           item.ID,
			Action:       item.Action,
			OperatorID:   item.OperatorID,
			OperatorName: names[item.OperatorID],
			Note:         item.Note,
			Detail:       item.Detail,
			CreatedAt:    item.CreatedAt,
		})
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

// fillActionSummary 批量给列表补处置动作码与摘要。
//
// 失败不报错：动作摘要是辅助信息，查不到最坏是这一列显示「—」，
// 不该让整页列表打不开。
func (s *securityService) fillActionSummary(ctx context.Context, rows []dto.RiskEventInfo, eventIDs []uint64) {
	if len(rows) == 0 || len(eventIDs) == 0 {
		return
	}
	grouped, err := s.repo.ListRiskEventActionsByEvents(ctx, eventIDs)
	if err != nil {
		return
	}
	byID := make(map[uint64]int, len(eventIDs))
	for i := range rows {
		byID[rows[i].ID] = i
	}
	for eventID, actions := range grouped {
		idx, ok := byID[eventID]
		if !ok {
			continue
		}
		summary := summarizeActions(actions)
		rows[idx].Actions = summary.codes
		rows[idx].ActionSummary = summary.text
	}
}

// actionSummary 处置动作的展示摘要。
type actionSummary struct {
	codes []string
	text  string
}

// actionLabel 动作码 → 中文名（与前端 RISK_ACTION_LABEL 同一套口径）。
var actionLabel = map[string]string{
	model.RiskActionLevel:          "调整等级",
	model.RiskActionHandle:         "处置",
	model.RiskActionIgnore:         "忽略",
	model.RiskActionBlacklist:      "拉黑",
	model.RiskActionRevokeSessions: "失效会话",
}

// summarizeActions 把流水折叠成「有哪些动作 + 一串中文摘要」。
//
// 去重且按固定顺序（管控类在前）：一条事件可能被处置多次，列表上要的是
// 「做过哪些类别的动作」，而不是重复五遍「拉黑」。
func summarizeActions(actions []model.RiskEventAction) actionSummary {
	if len(actions) == 0 {
		return actionSummary{}
	}
	order := []string{
		model.RiskActionBlacklist,
		model.RiskActionRevokeSessions,
		model.RiskActionHandle,
		model.RiskActionIgnore,
		model.RiskActionLevel,
	}
	present := make(map[string]bool, len(actions))
	for _, a := range actions {
		present[a.Action] = true
	}
	out := actionSummary{}
	labels := make([]string, 0, len(order))
	for _, code := range order {
		if !present[code] {
			continue
		}
		out.codes = append(out.codes, code)
		if label, ok := actionLabel[code]; ok {
			labels = append(labels, label)
		}
	}
	out.text = strings.Join(labels, " · ")
	return out
}

// CountRiskEvents 风险事件汇总（页面顶部卡片）。
func (s *securityService) CountRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (map[string]int64, error) {
	return s.repo.CountRiskEvents(ctx, query)
}

func (s *securityService) IgnoreRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest, operatorID uint64) (*dto.RiskEventInfo, error) {
	return s.closeRiskEvent(ctx, id, model.RiskStatusIgnored, req, operatorID)
}
func (s *securityService) HandleRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest, operatorID uint64) (*dto.RiskEventInfo, error) {
	return s.closeRiskEvent(ctx, id, model.RiskStatusHandled, req, operatorID)
}

// UpdateRiskEventLevel 手动调整风险等级（doc06 §4.3「手动升级风险等级」）。
//
// 默认只改等级、不动状态：升级等级是「这条要重点看」的标记，不等于已经处置。
// 若顺手把状态改成 handled，运营刚提级的那条就会从「待处理」里消失，反而丢了待办。
// 但「降级结案」也是真实诉求（一眼看出是误报），所以给了显式的 CloseEvent 开关，
// 由运营自己决定，而不是偷偷替他关掉。
func (s *securityService) UpdateRiskEventLevel(ctx context.Context, id uint64, req dto.RiskEventLevelRequest, operatorID uint64) (*dto.RiskEventInfo, error) {
	level := strings.ToLower(strings.TrimSpace(req.RiskLevel))
	if !isValidRiskLevel(level) {
		return nil, fmt.Errorf("风险等级非法：%s", req.RiskLevel)
	}
	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	previous := item.RiskLevel
	now := time.Now()
	operator := operatorID
	item.RiskLevel = level
	if strings.TrimSpace(req.Note) != "" {
		item.HandleNote = req.Note
	}
	item.HandledBy = &operator
	item.HandledAt = &now
	if req.CloseEvent {
		closeAs, err := normalizeCloseStatus(req.CloseAs)
		if err != nil {
			return nil, err
		}
		item.Status = closeAs
	}
	item.UpdatedAt = now
	if err := s.repo.UpdateRiskEvent(ctx, item); err != nil {
		return nil, err
	}
	detail, _ := json.Marshal(map[string]any{
		"from": previous, "to": level, "closed": req.CloseEvent,
	})
	s.recordAction(ctx, id, model.RiskActionLevel, operatorID, req.Note, string(detail))
	return s.riskEventResult(ctx, *item, operatorID)
}

// normalizeCloseStatus 关单目标状态校验（默认 handled）。
func normalizeCloseStatus(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return model.RiskStatusHandled, nil
	case model.RiskStatusHandled:
		return model.RiskStatusHandled, nil
	case model.RiskStatusIgnored:
		return model.RiskStatusIgnored, nil
	case model.RiskStatusPending:
		// 显式允许「只标记不关单」的写法，语义等于 CloseEvent=false。
		return model.RiskStatusPending, nil
	default:
		return "", fmt.Errorf("处置状态非法：%s", raw)
	}
}

func isValidRiskLevel(level string) bool {
	for _, v := range model.RiskLevels() {
		if v == level {
			return true
		}
	}
	return false
}

// closeRiskEvent 关闭一条待办（处置 / 忽略），可选同时执行管控动作。
//
// 为什么要能在关单的同时做管控：真实排查里两个动作往往是连在一起想的 ——
// 「确认是攻击，处置掉，顺手把这个 IP 封了」。拆成三个按钮会让运营关完单
// 再去列表里重新找这条事件，实战里就会漏做管控。做成可选开关既保留
// 「只看不动」的轻量路径，也照顾「一次点完」的高频路径。
//
// 顺序刻意是**先管控、后关单**：管控失败（如黑名单重复、踢会话查库报错）
// 直接返回错误且不关单 —— 反过来先关单再管控失败，运营会看到「已处置」
// 但其实什么都没封上，比报错危险得多。
func (s *securityService) closeRiskEvent(ctx context.Context, id uint64, status string, req dto.RiskEventHandleRequest, operatorID uint64) (*dto.RiskEventInfo, error) {
	if _, err := s.repo.GetRiskEvent(ctx, id); err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	var outcomes []string
	if req.Blacklist {
		blk, err := s.CreateBlacklistFromRisk(ctx, id, dto.RiskEventBlacklistRequest{
			Type: req.BlacklistType,
			Note: req.Note,
		}, operatorID)
		if err != nil {
			return nil, fmt.Errorf("拉黑失败，未关闭事件：%w", err)
		}
		outcomes = append(outcomes, fmt.Sprintf("已拉黑 %s %s", blacklistTypeLabel(blk.Type), blk.TargetValue))
	}
	if req.RevokeSessions {
		revoked, err := s.RevokeSessionsFromRisk(ctx, id, dto.RiskEventRevokeRequest{Note: req.Note}, operatorID)
		if err != nil {
			return nil, fmt.Errorf("失效会话失败，未关闭事件：%w", err)
		}
		outcomes = append(outcomes, fmt.Sprintf("已失效 %d 个会话", revoked.Meta.Total))
	}

	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	now := time.Now()
	operator := operatorID
	item.Status = status
	if strings.TrimSpace(req.Note) != "" {
		item.HandleNote = req.Note
	}
	item.HandledBy = &operator
	item.HandledAt = &now
	item.UpdatedAt = now
	if err := s.repo.UpdateRiskEvent(ctx, item); err != nil {
		return nil, err
	}

	action := model.RiskActionHandle
	if status == model.RiskStatusIgnored {
		action = model.RiskActionIgnore
	}
	detail, _ := json.Marshal(map[string]any{"status": status, "outcomes": outcomes})
	s.recordAction(ctx, id, action, operatorID, req.Note, string(detail))
	return s.riskEventResult(ctx, *item, operatorID)
}

// CreateBlacklistFromRisk 从风险事件拉黑来源。
//
// 维度选择：运营可以显式指定，留空则按事件里**最有区分度的可用字段**推断。
// 旧实现只看 IP，于是只有设备指纹、没有 IP 的事件会被写进一条空的 IP 黑名单
// （TargetValue 落到 username 上但 Type 还是 ip），既拦不住人又污染列表。
func (s *securityService) CreateBlacklistFromRisk(ctx context.Context, id uint64, req dto.RiskEventBlacklistRequest, operatorID uint64) (*dto.BlacklistInfo, error) {
	event, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	kind, target, err := resolveBlacklistTarget(event, req.Type)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &model.Blacklist{
		Type:        kind,
		TargetValue: target,
		Status:      model.BlacklistStatusActive,
		Source:      model.BlacklistSourceRiskEvent,
		Reason:      firstNonEmpty(req.Note, event.Summary, fmt.Sprintf("由风险事件 #%d 拉黑", id)),
		EffectiveAt: now,
		CreatedBy:   operatorID,
		UpdatedBy:   operatorID,
	}
	if err := s.repo.CreateBlacklist(ctx, item); err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("该%s已在黑名单中（%s），无需重复添加", blacklistTypeLabel(kind), target)
		}
		return nil, err
	}
	detail, _ := json.Marshal(map[string]any{"type": kind, "target_value": target})
	s.recordAction(ctx, id, model.RiskActionBlacklist, operatorID, req.Note, string(detail))
	if req.CloseEvent {
		if err := s.closeEventOnly(ctx, id, model.RiskStatusHandled, req.Note, operatorID); err != nil {
			return nil, err
		}
	}
	result := toBlacklistInfo(*item)
	if names, err := s.repo.AdminNames(ctx, []uint64{operatorID}); err == nil {
		result.CreatedByName = names[operatorID]
		result.UpdatedByName = names[operatorID]
	}
	return &result, nil
}

// blacklistTypeLabel 黑名单维度的中文名（错误提示与流水摘要共用）。
func blacklistTypeLabel(kind string) string {
	switch kind {
	case security.BlacklistTypeIP:
		return "IP"
	case security.BlacklistTypeDevice:
		return "设备"
	case security.BlacklistTypePhone:
		return "手机号"
	case security.BlacklistTypeEmail:
		return "邮箱"
	default:
		return "账号"
	}
}

// resolveBlacklistTarget 决定拉黑维度与命中值。
//
// 默认优先级：IP > 设备指纹 > 账号。理由是**区分度与稳定性** ——
// IP 能挡住同一来源的一批攻击（撞库的典型形态）；同一 NAT 下 IP 拉黑会连坐，
// 这时运营应当显式改选设备或账号，而不是靠系统猜。
//
// 显式指定维度时若该事件没有对应字段，直接报错而不是写入空值：
// 一条「命中值为空」的黑名单永远不会命中，却会出现在列表里让运营以为封上了。
func resolveBlacklistTarget(event *model.RiskEvent, requested string) (string, string, error) {
	kind := strings.ToLower(strings.TrimSpace(requested))
	if kind == "" {
		switch {
		case strings.TrimSpace(event.IP) != "":
			kind = security.BlacklistTypeIP
		case strings.TrimSpace(event.DeviceFingerprint) != "":
			kind = security.BlacklistTypeDevice
		default:
			kind = security.BlacklistTypeUser
		}
	}
	if !model.IsBlacklistType(kind) {
		return "", "", fmt.Errorf("黑名单类型非法：%s", requested)
	}
	var value string
	switch kind {
	case security.BlacklistTypeIP:
		value = strings.TrimSpace(event.IP)
	case security.BlacklistTypeDevice:
		value = strings.TrimSpace(event.DeviceFingerprint)
	case security.BlacklistTypePhone:
		value = strings.TrimSpace(event.Username)
	case security.BlacklistTypeEmail:
		value = strings.TrimSpace(event.Username)
	default:
		value = strings.TrimSpace(event.Username)
	}
	if value == "" {
		return "", "", fmt.Errorf("该事件没有%s信息，无法按此维度拉黑，请改选其它维度", blacklistTypeLabel(kind))
	}
	return kind, value, nil
}

func (s *securityService) RevokeSessionsFromRisk(ctx context.Context, id uint64, req dto.RiskEventRevokeRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error) {
	event, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	subject := firstNonEmpty(event.SubjectType, model.SubjectTypeUser)
	// 员工会话目前不落 user_sessions（管理端 JWT 不带 sid，见 middleware.AdminAuth），
	// 所以对员工事件做「失效会话」必然一个都踢不掉。此前这会返回一个空列表 +
	// 「操作成功」，运营点完以为已经把盗号员工踢下线了，实际什么都没发生 ——
	// 静默的假成功比报错危险得多，这里显式拒绝。
	if subject == model.SubjectTypeAdmin {
		return nil, fmt.Errorf("该事件属于员工后台账号，当前不支持对员工强制下线，请改用拉黑或直接禁用该员工")
	}
	if event.UserID == 0 {
		return nil, fmt.Errorf("该事件的账号不存在（无有效用户 ID），没有可失效的会话")
	}
	// 必须按事件自己的主体域 + 用户 ID 精确匹配：user_id 承载 users.id 与 admins.id
	// 两个 ID 空间且会撞号，不带域过滤会去踢 ID 相同的另一域会话（此前实测踩到）。
	sessions, err := s.repo.RevokeActiveSessionsOfSubject(ctx, subject, event.UserID,
		firstNonEmpty(req.Note, fmt.Sprintf("风险事件 #%d 处置", id)), operatorID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SessionInfo, 0, len(sessions))
	for i := range sessions {
		result = append(result, toSessionInfo(sessions[i]))
	}
	s.invalidateSessions(ctx, sessionIDsOf(sessions))
	detail, _ := json.Marshal(map[string]any{"revoked": len(result), "subject_type": subject})
	s.recordAction(ctx, id, model.RiskActionRevokeSessions, operatorID, req.Note, string(detail))
	if req.CloseEvent {
		if err := s.closeEventOnly(ctx, id, model.RiskStatusHandled, req.Note, operatorID); err != nil {
			return nil, err
		}
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

// closeEventOnly 只改风险事件的状态（不重复写管控流水）。
//
// 与 closeRiskEvent 的分工：后者处理「关单 + 顺带管控」，管控失败要整体回滚；
// 本方法用于「管控已完成、顺带关单」的场景（拉黑/踢会话的 CloseEvent 开关）。
func (s *securityService) closeEventOnly(ctx context.Context, id uint64, status, note string, operatorID uint64) error {
	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return notFoundMessage(err, "风险事件不存在")
	}
	now := time.Now()
	operator := operatorID
	item.Status = status
	if strings.TrimSpace(note) != "" {
		item.HandleNote = note
	}
	item.HandledBy = &operator
	item.HandledAt = &now
	item.UpdatedAt = now
	if err := s.repo.UpdateRiskEvent(ctx, item); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"status": status, "via": "case_action"})
	s.recordAction(ctx, id, model.RiskActionHandle, operatorID, note, string(detail))
	return nil
}

// recordAction 追加一条处置流水。
//
// 失败只记日志不返回错误：流水是留痕能力，写不进去最坏是时间线少一条，
// 不该让已经生效的管控动作（黑名单已写入、会话已踢）整体回退。
func (s *securityService) recordAction(ctx context.Context, eventID uint64, action string, operatorID uint64, note, detail string) {
	entry := &model.RiskEventAction{
		EventID:    eventID,
		Action:     action,
		OperatorID: operatorID,
		Note:       truncateRunes(note, 255),
		Detail:     detail,
	}
	// operator_id 非空约束：未取到操作人（理论上不会，接口都有鉴权）时落到系统值 0。
	if err := s.repo.CreateRiskEventAction(ctx, entry); err != nil && s.logger != nil {
		s.logger.Warn("写入风险事件处置流水失败", zap.Uint64("event_id", eventID), zap.String("action", action), zap.Error(err))
	}
}

// riskEventResult 组装单条事件的返回体（含处置人姓名与动作摘要）。
func (s *securityService) riskEventResult(ctx context.Context, item model.RiskEvent, operatorID uint64) (*dto.RiskEventInfo, error) {
	result := toRiskEventInfo(item)
	if names, err := s.repo.AdminNames(ctx, []uint64{operatorID}); err == nil {
		result.HandledByName = names[operatorID]
	}
	if actions, err := s.repo.ListRiskEventActionsByEvents(ctx, []uint64{item.ID}); err == nil {
		summary := summarizeActions(actions[item.ID])
		result.Actions = summary.codes
		result.ActionSummary = summary.text
	}
	return &result, nil
}

func (s *securityService) ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) (*dto.ListResponse[dto.BlacklistInfo], error) {
	items, total, err := s.repo.ListBlacklists(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.BlacklistInfo, 0, len(items))
	operatorIDs := make([]uint64, 0, len(items)*2)
	for i := range items {
		result = append(result, toBlacklistInfo(items[i]))
		operatorIDs = append(operatorIDs, items[i].CreatedBy, items[i].UpdatedBy)
	}
	if names, err := s.repo.AdminNames(ctx, operatorIDs); err == nil {
		for i := range result {
			result[i].CreatedByName = names[result[i].CreatedBy]
			result[i].UpdatedByName = names[result[i].UpdatedBy]
		}
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) CreateBlacklist(ctx context.Context, req dto.BlacklistCreateRequest, operatorID uint64) (*dto.BlacklistInfo, error) {
	kind := strings.ToLower(strings.TrimSpace(req.Type))
	if !model.IsBlacklistType(kind) {
		return nil, fmt.Errorf("黑名单类型非法：%s", req.Type)
	}
	target := strings.TrimSpace(req.TargetValue)
	if target == "" {
		return nil, fmt.Errorf("命中值不能为空")
	}
	now := time.Now()
	expiredAt, err := parseOptionalTime(req.ExpiredAt)
	if err != nil {
		return nil, err
	}
	// 失效时间早于生效时间没有任何意义，且会让这条黑名单「一创建就过期」——
	// 运营却以为已经封上了。直接拒绝比静默接受安全。
	if expiredAt != nil && !expiredAt.After(now) {
		return nil, fmt.Errorf("失效时间必须晚于当前时间")
	}
	item := &model.Blacklist{
		Type:        kind,
		TargetValue: target,
		Status:      firstNonEmpty(req.Status, model.BlacklistStatusActive),
		Source:      firstNonEmpty(req.Source, model.BlacklistSourceManual),
		Reason:      req.Reason,
		EffectiveAt: now,
		ExpiredAt:   expiredAt,
		CreatedBy:   operatorID,
		UpdatedBy:   operatorID,
	}
	if err := s.repo.CreateBlacklist(ctx, item); err != nil {
		// (type, target_value) 上有唯一索引：重复添加同一目标此前会抛原始 SQL 错误
		// （页面上是一串英文），现在翻译成可操作的中文提示。
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("该类型的命中值已存在（%s：%s），请直接编辑已有记录", kind, target)
		}
		return nil, err
	}
	result := toBlacklistInfo(*item)
	if names, err := s.repo.AdminNames(ctx, []uint64{operatorID}); err == nil {
		result.CreatedByName = names[operatorID]
		result.UpdatedByName = names[operatorID]
	}
	return &result, nil
}

func (s *securityService) GetBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error) {
	item, err := s.repo.GetBlacklist(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "黑名单不存在")
	}
	result := toBlacklistInfo(*item)
	if names, err := s.repo.AdminNames(ctx, []uint64{item.CreatedBy, item.UpdatedBy}); err == nil {
		result.CreatedByName = names[item.CreatedBy]
		result.UpdatedByName = names[item.UpdatedBy]
	}
	return &result, nil
}

func (s *securityService) UpdateBlacklist(ctx context.Context, id uint64, req dto.BlacklistUpdateRequest, operatorID uint64) (*dto.BlacklistInfo, error) {
	item, err := s.repo.GetBlacklist(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "黑名单不存在")
	}
	if req.Status != "" {
		if req.Status != model.BlacklistStatusActive && req.Status != model.BlacklistStatusInactive {
			return nil, fmt.Errorf("黑名单状态非法：%s", req.Status)
		}
		item.Status = req.Status
	}
	if req.Reason != "" {
		item.Reason = req.Reason
	}
	if req.ExpiredAt != "" {
		expiredAt, err := parseOptionalTime(req.ExpiredAt)
		if err != nil {
			return nil, err
		}
		// 更新允许把失效时间改到过去：这等价于「立刻失效」，是运营提前解除
		// 限时黑名单最直接的做法（比先停用再删除更贴近意图）。
		// 新增时则拒绝已过去的时间 —— 那是配置填错了，静默接受会让运营
		// 以为封上了，而实际从未生效过。
		item.ExpiredAt = expiredAt
	}
	item.UpdatedBy = operatorID
	item.UpdatedAt = time.Now()
	if err := s.repo.UpdateBlacklist(ctx, item); err != nil {
		return nil, err
	}
	result := toBlacklistInfo(*item)
	if names, err := s.repo.AdminNames(ctx, []uint64{item.CreatedBy, item.UpdatedBy}); err == nil {
		result.CreatedByName = names[item.CreatedBy]
		result.UpdatedByName = names[item.UpdatedBy]
	}
	return &result, nil
}

func (s *securityService) UpdateBlacklistStatus(ctx context.Context, id uint64, req dto.BlacklistStatusRequest, operatorID uint64) (*dto.BlacklistInfo, error) {
	return s.UpdateBlacklist(ctx, id, dto.BlacklistUpdateRequest{Status: req.Status}, operatorID)
}

func (s *securityService) ReleaseBlacklist(ctx context.Context, id uint64, operatorID uint64) (*dto.BlacklistInfo, error) {
	return s.UpdateBlacklist(ctx, id, dto.BlacklistUpdateRequest{Status: "inactive", Reason: "人工解除"}, operatorID)
}

// ListBlacklistHits 黑名单命中记录。
//
// 命中 = 登录日志里与该黑名单 target 对得上的行。关联键必须按黑名单类型走：
// 拿黑名单行的 ID 去比 login_logs.user_id（旧实现）永远匹配不到任何东西，
// 页面上因此恒为空。
func (s *securityService) ListBlacklistHits(ctx context.Context, id uint64, query dto.BlacklistHitListQuery) (*dto.ListResponse[dto.LoginLogInfo], error) {
	items, total, err := s.repo.ListBlacklistHits(ctx, id, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.LoginLogInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toLoginLogInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) ListSessions(ctx context.Context, query dto.SessionListQuery) (*dto.ListResponse[dto.SessionInfo], error) {
	items, total, err := s.repo.ListSessions(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) GetSession(ctx context.Context, id uint64) (*dto.SessionInfo, error) {
	item, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "会话不存在")
	}
	result := toSessionInfo(*item)
	return &result, nil
}

func (s *securityService) RevokeSession(ctx context.Context, id uint64, req dto.SessionRevokeRequest, operatorID uint64) (*dto.SessionInfo, error) {
	item, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "会话不存在")
	}
	// 已失效/已过期的会话直接原样返回，不覆盖首次的操作人留痕。
	// 重复点击（页面停留后重试、双开标签页）不该把「谁在什么时候踢的」
	// 改成后一次操作，那会让审计链路指向错误的人。
	if item.Status != "active" {
		result := toSessionInfo(*item)
		return &result, nil
	}
	now := time.Now()
	revokedBy := operatorID
	item.Status = "revoked"
	item.RevokedReason = firstNonEmpty(req.Reason, "管理员强制下线")
	item.RevokedBy = &revokedBy
	item.RevokedAt = &now
	item.UpdatedAt = now
	if err := s.repo.UpdateSession(ctx, item); err != nil {
		return nil, err
	}
	// 撤销后立即失效校验缓存，否则被踢的令牌还能再用最多 positiveTTL（5 分钟）。
	s.invalidateSessions(ctx, []string{item.SessionID})
	result := toSessionInfo(*item)
	return &result, nil
}

func (s *securityService) BatchRevokeSessions(ctx context.Context, req dto.SessionBatchRevokeRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error) {
	items, err := s.repo.BatchRevokeSessions(ctx, req.IDs, firstNonEmpty(req.Reason, "批量失效"), operatorID)
	if err != nil {
		return nil, err
	}
	s.invalidateSessions(ctx, sessionIDsOf(items))
	result := make([]dto.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionInfo(item))
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

func (s *securityService) RevokeUserAllSessions(ctx context.Context, req dto.SessionRevokeUserAllRequest, operatorID uint64) (*dto.ListResponse[dto.SessionInfo], error) {
	items, err := s.repo.RevokeUserAllSessions(ctx, req.UserID, firstNonEmpty(req.Reason, "仅保留当前会话"), operatorID)
	if err != nil {
		return nil, err
	}
	s.invalidateSessions(ctx, sessionIDsOf(items))
	result := make([]dto.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionInfo(item))
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

// ExpireStaleSessions 把已过期的 active 会话收成 expired（定时任务调用）。
//
// 顺带失效缓存：这些会话本来就已过期，sessionguard 查库也会判无效，
// 但缓存里可能还留着 5 分钟前写入的 "1"，不删就还有一段「过期却放行」的窗口。
func (s *securityService) ExpireStaleSessions(ctx context.Context, limit int) (int, error) {
	sessionIDs, err := s.repo.ExpireStaleSessions(ctx, limit)
	if err != nil {
		return 0, err
	}
	s.invalidateSessions(ctx, sessionIDs)
	return len(sessionIDs), nil
}

func newListResponse[T any](items []T, page, pageSize int, total int64) *dto.ListResponse[T] {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return &dto.ListResponse[T]{
		Items: items,
		Meta:  dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
	}
}

func notFoundMessage(err error, message string) error {
	if errorsIsRecordNotFound(err) {
		return fmt.Errorf("%s", message)
	}
	return err
}

func errorsIsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound || strings.Contains(err.Error(), gorm.ErrRecordNotFound.Error())
}

// isDuplicateKey 判断是否为唯一约束冲突（pgx 23505）。
//
// 用错误文本而不是驱动类型断言：仓储层已经把驱动错误包过一层，
// 引入 pgconn 只为判一个错误码会让本模块多一个直连驱动的依赖。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "23505") ||
		strings.Contains(msg, "unique constraint")
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid expired_at format: %s", value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toLoginLogInfo(item model.LoginLog) dto.LoginLogInfo {
	return dto.LoginLogInfo{
		ID:                item.ID,
		UserID:            item.UserID,
		Username:          item.Username,
		LoginType:         item.LoginType,
		Result:            item.Result,
		FailureReason:     item.FailureReason,
		IP:                item.IP,
		UserAgent:         item.UserAgent,
		DeviceFingerprint: item.DeviceFingerprint,
		Platform:          item.Platform,
		SubjectType:       item.SubjectType,
		RiskFlag:          item.RiskFlag,
		CreatedAt:         item.CreatedAt,
	}
}

func toAuditLogInfo(item model.AuditLog) dto.AuditLogInfo {
	return dto.AuditLogInfo{
		ID:              item.ID,
		OperatorID:      item.OperatorID,
		OperatorName:    item.OperatorName,
		Module:          item.Module,
		ResourceType:    item.ResourceType,
		ResourceID:      item.ResourceID,
		Action:          item.Action,
		RequestMethod:   item.RequestMethod,
		RequestPath:     item.RequestPath,
		RequestPayload:  item.RequestPayload,
		ResponseCode:    item.ResponseCode,
		ResponseMessage: item.ResponseMessage,
		IP:              item.IP,
		UserAgent:       item.UserAgent,
		TraceID:         item.TraceID,
		CreatedAt:       item.CreatedAt,
	}
}

func toRiskEventInfo(item model.RiskEvent) dto.RiskEventInfo {
	subject := firstNonEmpty(item.SubjectType, model.SubjectTypeUser)
	result := dto.RiskEventInfo{
		ID:                item.ID,
		RiskType:          item.RiskType,
		RiskLevel:         item.RiskLevel,
		UserID:            item.UserID,
		Username:          item.Username,
		SubjectType:       subject,
		IP:                item.IP,
		DeviceFingerprint: item.DeviceFingerprint,
		RuleCode:          item.RuleCode,
		Summary:           item.Summary,
		DetailPayload:     item.DetailPayload,
		OccurCount:        item.OccurCount,
		FirstOccurredAt:   item.FirstOccurredAt,
		LastOccurredAt:    item.LastOccurredAt,
		Status:            item.Status,
		HandleNote:        item.HandleNote,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
	if item.HandledBy != nil {
		result.HandledBy = *item.HandledBy
	}
	result.HandledAt = item.HandledAt
	return result
}

// blacklistRuntimeStatus 推导黑名单运行态。
//
// 与 status 分开的原因：status 是运营开关，运行态还要叠加生效/失效时间。
// 一条限时黑名单到期后 status 仍是 active，但**实际已经不再拦截** ——
// 页面若只显示 status，运营会以为封着，而攻击者早已能登录。
func blacklistRuntimeStatus(item model.Blacklist, now time.Time) string {
	if item.Status != model.BlacklistStatusActive {
		return model.BlacklistStatusInactive
	}
	if item.EffectiveAt.After(now) {
		return "pending"
	}
	if item.ExpiredAt != nil && !item.ExpiredAt.After(now) {
		return "expired"
	}
	return model.BlacklistStatusActive
}

func toBlacklistInfo(item model.Blacklist) dto.BlacklistInfo {
	return dto.BlacklistInfo{
		ID:            item.ID,
		Type:          item.Type,
		TargetValue:   item.TargetValue,
		Status:        item.Status,
		Source:        item.Source,
		Reason:        item.Reason,
		EffectiveAt:   item.EffectiveAt,
		ExpiredAt:     item.ExpiredAt,
		HitCount:      item.HitCount,
		CreatedBy:     item.CreatedBy,
		UpdatedBy:     item.UpdatedBy,
		RuntimeStatus: blacklistRuntimeStatus(item, time.Now()),
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toSessionInfo(item model.Session) dto.SessionInfo {
	result := dto.SessionInfo{
		ID:                item.ID,
		SessionID:         item.SessionID,
		UserID:            item.UserID,
		Username:          item.Username,
		Platform:          item.Platform,
		IP:                item.IP,
		UserAgent:         item.UserAgent,
		DeviceFingerprint: item.DeviceFingerprint,
		LoginAt:           item.LoginAt,
		LastActiveAt:      item.LastActiveAt,
		Status:            item.Status,
		SubjectType:       item.SubjectType,
		RiskFlag:          item.RiskFlag,
		RevokedReason:     item.RevokedReason,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
	if item.ExpiredAt != nil {
		result.ExpiredAt = *item.ExpiredAt
	}
	if item.RevokedBy != nil {
		result.RevokedBy = *item.RevokedBy
	}
	result.RevokedAt = item.RevokedAt
	return result
}
