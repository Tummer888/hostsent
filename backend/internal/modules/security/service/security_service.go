package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/security/dto"
	"hostsent/backend/internal/modules/security/model"
	"hostsent/backend/internal/modules/security/repository"
)

type SecurityService interface {
	ListLoginLogs(ctx context.Context, query dto.LoginLogListQuery) (*dto.ListResponse[dto.LoginLogInfo], error)
	GetLoginLog(ctx context.Context, id uint64) (*dto.LoginLogInfo, error)
	ListAuditLogs(ctx context.Context, query dto.AuditLogListQuery) (*dto.ListResponse[dto.AuditLogInfo], error)
	GetAuditLog(ctx context.Context, id uint64) (*dto.AuditLogInfo, error)
	ListRiskEvents(ctx context.Context, query dto.RiskEventListQuery) (*dto.ListResponse[dto.RiskEventInfo], error)
	GetRiskEvent(ctx context.Context, id uint64) (*dto.RiskEventInfo, error)
	IgnoreRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.RiskEventInfo, error)
	HandleRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.RiskEventInfo, error)
	CreateBlacklistFromRisk(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.BlacklistInfo, error)
	RevokeSessionsFromRisk(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.ListResponse[dto.SessionInfo], error)
	ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) (*dto.ListResponse[dto.BlacklistInfo], error)
	CreateBlacklist(ctx context.Context, req dto.BlacklistCreateRequest) (*dto.BlacklistInfo, error)
	GetBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error)
	UpdateBlacklist(ctx context.Context, id uint64, req dto.BlacklistUpdateRequest) (*dto.BlacklistInfo, error)
	UpdateBlacklistStatus(ctx context.Context, id uint64, req dto.BlacklistStatusRequest) (*dto.BlacklistInfo, error)
	ReleaseBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error)
	ListBlacklistHits(ctx context.Context, id uint64) (*dto.ListResponse[dto.LoginLogInfo], error)
	ListSessions(ctx context.Context, query dto.SessionListQuery) (*dto.ListResponse[dto.SessionInfo], error)
	GetSession(ctx context.Context, id uint64) (*dto.SessionInfo, error)
	RevokeSession(ctx context.Context, id uint64, req dto.SessionRevokeRequest) (*dto.SessionInfo, error)
	BatchRevokeSessions(ctx context.Context, req dto.SessionBatchRevokeRequest) (*dto.ListResponse[dto.SessionInfo], error)
	RevokeUserAllSessions(ctx context.Context, req dto.SessionRevokeUserAllRequest) (*dto.ListResponse[dto.SessionInfo], error)
}

type securityService struct {
	repo repository.SecurityRepository
}

func NewSecurityService(repo repository.SecurityRepository) SecurityService {
	return &securityService{repo: repo}
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
	for _, item := range items {
		result = append(result, toRiskEventInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) GetRiskEvent(ctx context.Context, id uint64) (*dto.RiskEventInfo, error) {
	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	result := toRiskEventInfo(*item)
	return &result, nil
}

func (s *securityService) IgnoreRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.RiskEventInfo, error) {
	return s.updateRiskEventStatus(ctx, id, "ignored", req.Note)
}

func (s *securityService) HandleRiskEvent(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.RiskEventInfo, error) {
	return s.updateRiskEventStatus(ctx, id, "handled", req.Note)
}

func (s *securityService) CreateBlacklistFromRisk(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.BlacklistInfo, error) {
	event, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	now := time.Now()
	item := &model.Blacklist{
		Type:        firstNonEmpty(detectBlacklistType(event), "ip"),
		TargetValue: firstNonEmpty(event.IP, event.Username, event.DeviceFingerprint),
		Status:      "active",
		Source:      "system",
		Reason:      firstNonEmpty(req.Note, event.Summary, "由风险事件自动拉黑"),
		EffectiveAt: now,
		CreatedBy:   1,
		UpdatedBy:   1,
	}
	if err := s.repo.CreateBlacklist(ctx, item); err != nil {
		return nil, err
	}
	result := toBlacklistInfo(*item)
	return &result, nil
}

func (s *securityService) RevokeSessionsFromRisk(ctx context.Context, id uint64, req dto.RiskEventHandleRequest) (*dto.ListResponse[dto.SessionInfo], error) {
	event, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	sessions, total, err := s.repo.ListSessions(ctx, dto.SessionListQuery{UserID: event.UserID, Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	result := make([]dto.SessionInfo, 0, len(sessions))
	for i := range sessions {
		sessions[i].Status = "revoked"
		sessions[i].RevokedReason = firstNonEmpty(req.Note, fmt.Sprintf("风险事件 #%d 处置", id))
		revokedBy := uint64(1)
		now := time.Now()
		sessions[i].RevokedBy = &revokedBy
		sessions[i].RevokedAt = &now
		if err := s.repo.UpdateSession(ctx, &sessions[i]); err != nil {
			return nil, err
		}
		result = append(result, toSessionInfo(sessions[i]))
	}
	return newListResponse(result, 1, len(result), total), nil
}

func (s *securityService) ListBlacklists(ctx context.Context, query dto.BlacklistListQuery) (*dto.ListResponse[dto.BlacklistInfo], error) {
	items, total, err := s.repo.ListBlacklists(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]dto.BlacklistInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toBlacklistInfo(item))
	}
	return newListResponse(result, query.Page, query.PageSize, total), nil
}

func (s *securityService) CreateBlacklist(ctx context.Context, req dto.BlacklistCreateRequest) (*dto.BlacklistInfo, error) {
	now := time.Now()
	expiredAt, err := parseOptionalTime(req.ExpiredAt)
	if err != nil {
		return nil, err
	}
	item := &model.Blacklist{
		Type:        req.Type,
		TargetValue: req.TargetValue,
		Status:      firstNonEmpty(req.Status, "active"),
		Source:      firstNonEmpty(req.Source, "manual"),
		Reason:      req.Reason,
		EffectiveAt: now,
		ExpiredAt:   expiredAt,
		CreatedBy:   1,
		UpdatedBy:   1,
	}
	if err := s.repo.CreateBlacklist(ctx, item); err != nil {
		return nil, err
	}
	result := toBlacklistInfo(*item)
	return &result, nil
}

func (s *securityService) GetBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error) {
	item, err := s.repo.GetBlacklist(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "黑名单不存在")
	}
	result := toBlacklistInfo(*item)
	return &result, nil
}

func (s *securityService) UpdateBlacklist(ctx context.Context, id uint64, req dto.BlacklistUpdateRequest) (*dto.BlacklistInfo, error) {
	item, err := s.repo.GetBlacklist(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "黑名单不存在")
	}
	if req.Status != "" {
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
		item.ExpiredAt = expiredAt
	}
	item.UpdatedBy = 1
	item.UpdatedAt = time.Now()
	if err := s.repo.UpdateBlacklist(ctx, item); err != nil {
		return nil, err
	}
	result := toBlacklistInfo(*item)
	return &result, nil
}

func (s *securityService) UpdateBlacklistStatus(ctx context.Context, id uint64, req dto.BlacklistStatusRequest) (*dto.BlacklistInfo, error) {
	return s.UpdateBlacklist(ctx, id, dto.BlacklistUpdateRequest{Status: req.Status})
}

func (s *securityService) ReleaseBlacklist(ctx context.Context, id uint64) (*dto.BlacklistInfo, error) {
	return s.UpdateBlacklist(ctx, id, dto.BlacklistUpdateRequest{Status: "inactive", Reason: "人工解除"})
}

func (s *securityService) ListBlacklistHits(ctx context.Context, id uint64) (*dto.ListResponse[dto.LoginLogInfo], error) {
	items, total, err := s.repo.ListBlacklistHits(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]dto.LoginLogInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toLoginLogInfo(item))
	}
	return newListResponse(result, 1, 10, total), nil
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

func (s *securityService) RevokeSession(ctx context.Context, id uint64, req dto.SessionRevokeRequest) (*dto.SessionInfo, error) {
	item, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "会话不存在")
	}
	now := time.Now()
	revokedBy := uint64(1)
	item.Status = "revoked"
	item.RevokedReason = firstNonEmpty(req.Reason, "管理员强制下线")
	item.RevokedBy = &revokedBy
	item.RevokedAt = &now
	item.UpdatedAt = now
	if err := s.repo.UpdateSession(ctx, item); err != nil {
		return nil, err
	}
	result := toSessionInfo(*item)
	return &result, nil
}

func (s *securityService) BatchRevokeSessions(ctx context.Context, req dto.SessionBatchRevokeRequest) (*dto.ListResponse[dto.SessionInfo], error) {
	items, err := s.repo.BatchRevokeSessions(ctx, req.IDs, firstNonEmpty(req.Reason, "批量失效"), 1)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionInfo(item))
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

func (s *securityService) RevokeUserAllSessions(ctx context.Context, req dto.SessionRevokeUserAllRequest) (*dto.ListResponse[dto.SessionInfo], error) {
	items, err := s.repo.RevokeUserAllSessions(ctx, req.UserID, firstNonEmpty(req.Reason, "仅保留当前会话"), 1)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionInfo(item))
	}
	return newListResponse(result, 1, len(result), int64(len(result))), nil
}

func (s *securityService) updateRiskEventStatus(ctx context.Context, id uint64, status string, note string) (*dto.RiskEventInfo, error) {
	item, err := s.repo.GetRiskEvent(ctx, id)
	if err != nil {
		return nil, notFoundMessage(err, "风险事件不存在")
	}
	now := time.Now()
	handledBy := uint64(1)
	item.Status = status
	item.HandleNote = note
	item.HandledBy = &handledBy
	item.HandledAt = &now
	item.UpdatedAt = now
	if err := s.repo.UpdateRiskEvent(ctx, item); err != nil {
		return nil, err
	}
	result := toRiskEventInfo(*item)
	return &result, nil
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
		Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total},
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

func detectBlacklistType(event *model.RiskEvent) string {
	if strings.TrimSpace(event.IP) != "" {
		return "ip"
	}
	if strings.TrimSpace(event.DeviceFingerprint) != "" {
		return "device"
	}
	return "user"
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
		IPRegion:          item.IPRegion,
		UserAgent:         item.UserAgent,
		DeviceFingerprint: item.DeviceFingerprint,
		Platform:          item.Platform,
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
	result := dto.RiskEventInfo{
		ID:                item.ID,
		RiskType:          item.RiskType,
		RiskLevel:         item.RiskLevel,
		UserID:            item.UserID,
		Username:          item.Username,
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

func toBlacklistInfo(item model.Blacklist) dto.BlacklistInfo {
	return dto.BlacklistInfo{
		ID:          item.ID,
		Type:        item.Type,
		TargetValue: item.TargetValue,
		Status:      item.Status,
		Source:      item.Source,
		Reason:      item.Reason,
		EffectiveAt: item.EffectiveAt,
		ExpiredAt:   item.ExpiredAt,
		HitCount:    item.HitCount,
		CreatedBy:   item.CreatedBy,
		UpdatedBy:   item.UpdatedBy,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
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
		IPRegion:          item.IPRegion,
		UserAgent:         item.UserAgent,
		DeviceFingerprint: item.DeviceFingerprint,
		LoginAt:           item.LoginAt,
		LastActiveAt:      item.LastActiveAt,
		Status:            item.Status,
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
