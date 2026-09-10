package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
	"hostsent/backend/internal/modules/admin/ticket/repository"
)

// Notifier 工单域需要的通知能力（窄接口，避免与 notification 模块循环依赖，P2-04）。
// target 取 "user" / "admin"，recipientID 为接收方 ID。
type Notifier interface {
	Publish(ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string) error
}

// noopNotifier 未注入通知服务时的空实现。
type noopNotifier struct{}

func (noopNotifier) Publish(context.Context, string, string, uint64, map[string]string, string) error {
	return nil
}

// TicketService 定义工单域业务能力（管理端与用户端共用）。
type TicketService interface {
	// —— 管理端 ——
	List(ctx context.Context, q dto.TicketListQuery) (*dto.TicketListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.TicketDetail, error)
	Reply(ctx context.Context, id uint64, senderID uint64, senderName string, content string) (*dto.TicketDetail, error)
	Assign(ctx context.Context, id uint64, assignedTo uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	// Claim 认领未分配工单（P2-03）。
	Claim(ctx context.Context, id uint64, adminID uint64, adminName string) (*dto.TicketDetail, error)
	// Transfer 转派工单给其他员工（P2-03）。
	Transfer(ctx context.Context, id uint64, fromID uint64, fromName string, toID uint64, note string) (*dto.TicketDetail, error)
	UpdateStatus(ctx context.Context, id uint64, target string, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	Close(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	Stats(ctx context.Context) (*dto.TicketStatsResponse, error)
	// —— 用户端 ——
	UserList(ctx context.Context, userID uint64, page, pageSize int) (*dto.TicketListResponse, error)
	UserFindByID(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error)
	Create(ctx context.Context, userID uint64, username string, req dto.TicketCreateRequest) (*dto.TicketDetail, error)
	UserReply(ctx context.Context, accountID, actorID uint64, username string, id uint64, content string) (*dto.TicketDetail, error)
	Cancel(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error)
	// SetNotifier 延迟注入通知能力（通知服务在装配期晚于工单服务创建）。
	SetNotifier(n Notifier)
}

type ticketService struct {
	ticketRepo   repository.TicketRepository
	replyRepo    repository.ReplyRepository
	categoryRepo repository.CategoryRepository
	notifier     Notifier
}

// NewTicketService 创建工单业务服务。notifier 可为 nil（退化为不发通知）。
func NewTicketService(
	ticketRepo repository.TicketRepository,
	replyRepo repository.ReplyRepository,
	categoryRepo repository.CategoryRepository,
	notifier Notifier,
) TicketService {
	if notifier == nil {
		notifier = noopNotifier{}
	}
	return &ticketService{ticketRepo: ticketRepo, replyRepo: replyRepo, categoryRepo: categoryRepo, notifier: notifier}
}

// SetNotifier 延迟注入通知能力。
func (s *ticketService) SetNotifier(n Notifier) {
	if n == nil {
		s.notifier = noopNotifier{}
		return
	}
	s.notifier = n
}

// —— 管理端 ——

func (s *ticketService) List(ctx context.Context, q dto.TicketListQuery) (*dto.TicketListResponse, error) {
	items, total, err := s.ticketRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	resp, err := s.buildListResponse(ctx, items, total, normalizeP(q.Page), normalizePS(q.PageSize))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *ticketService) FindByID(ctx context.Context, id uint64) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	return s.buildDetail(ctx, *item)
}

// Reply 管理员回复工单：写入回复并推进状态（open→in_progress），
// 首次回复记录 first_reply_at，同时落操作日志并通知用户。
func (s *ticketService) Reply(ctx context.Context, id uint64, senderID uint64, senderName string, content string) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if IsFinalStatus(item.Status) {
		return nil, ErrStatusConflict
	}
	// 管理员回复：open → in_progress
	if item.Status == model.TicketStatusOpen {
		if err := EnsureStatus(item.Status, model.TicketStatusInProgress); err != nil {
			return nil, err
		}
		item.Status = model.TicketStatusInProgress
	}
	// 首次响应时间
	if item.FirstReplyAt == nil {
		now := time.Now()
		item.FirstReplyAt = &now
	}
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	if err := s.createReply(ctx, item.ID, model.SenderTypeAdmin, senderID, senderName, content); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, senderID, senderName, model.LogActionReply, "", "", "")
	// 通知用户：有新回复
	s.notify(ctx, notifEventTicketReplied, "user", item.UserID,
		map[string]string{"ticket_no": item.TicketNo}, fmt.Sprintf("ticket:%d:reply:%d", item.ID, time.Now().UnixNano()))
	return s.buildDetail(ctx, *item)
}

// Assign 分配工单给指定管理员（快照处理人名称），落日志并通知被指派人。
func (s *ticketService) Assign(ctx context.Context, id uint64, assignedTo uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	name, err := s.ticketRepo.AdminNameByID(ctx, assignedTo)
	if err != nil || name == "" {
		return nil, ErrAdminNotFound
	}
	from := item.AssignedName
	item.AssignedTo = assignedTo
	item.AssignedName = name
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, operatorID, operatorName, model.LogActionAssign, from, name, "")
	s.notifyAssign(ctx, *item)
	return s.buildDetail(ctx, *item)
}

// Claim 认领未分配工单（仅 assigned_to=0 可认领）。
func (s *ticketService) Claim(ctx context.Context, id uint64, adminID uint64, adminName string) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if item.AssignedTo != 0 {
		return nil, ErrTicketAssigned
	}
	if name, err := s.ticketRepo.AdminNameByID(ctx, adminID); err == nil && name != "" {
		adminName = name
	}
	item.AssignedTo = adminID
	item.AssignedName = adminName
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, adminID, adminName, model.LogActionClaim, "", adminName, "")
	s.notifyAssign(ctx, *item)
	return s.buildDetail(ctx, *item)
}

// Transfer 转派工单：记录 from→to 并通知新处理人。
func (s *ticketService) Transfer(ctx context.Context, id uint64, fromID uint64, fromName string, toID uint64, note string) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if item.AssignedTo == toID {
		return nil, ErrSameAssignee
	}
	name, err := s.ticketRepo.AdminNameByID(ctx, toID)
	if err != nil || name == "" {
		return nil, ErrAdminNotFound
	}
	from := item.AssignedName
	if from == "" {
		from = fromName
	}
	item.AssignedTo = toID
	item.AssignedName = name
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, fromID, fromName, model.LogActionTransfer, from, name, note)
	s.notifyAssign(ctx, *item)
	return s.buildDetail(ctx, *item)
}

// UpdateStatus 更新工单状态（状态机校验），落日志并通知用户。
func (s *ticketService) UpdateStatus(ctx context.Context, id uint64, target string, operatorID uint64, operatorName string) (*dto.TicketDetail, error) {
	if !IsValidStatus(target) {
		return nil, ErrStatusConflict
	}
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if item.Status == target {
		return s.buildDetail(ctx, *item) // 幂等
	}
	if err := EnsureStatus(item.Status, target); err != nil {
		return nil, err
	}
	from := item.Status
	// 写入目标状态并应用伴生时间戳（resolved_at / closed_at）
	item.Status = target
	if err := applyStatusSideEffect(item, target); err != nil {
		return nil, err
	}
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	action := model.LogActionStatus
	if target == model.TicketStatusClosed {
		action = model.LogActionClose
	}
	s.logAction(ctx, item.ID, operatorID, operatorName, action, from, target, "")
	s.notify(ctx, notifEventTicketStatus, "user", item.UserID,
		map[string]string{"ticket_no": item.TicketNo, "status": target},
		fmt.Sprintf("ticket:%d:status:%d", item.ID, time.Now().UnixNano()))
	return s.buildDetail(ctx, *item)
}

// Close 关闭工单（管理员强制关闭，resolved → closed 或其他非终态 → closed）。
func (s *ticketService) Close(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error) {
	return s.UpdateStatus(ctx, id, model.TicketStatusClosed, operatorID, operatorName)
}

// Stats 工单统计概览。
func (s *ticketService) Stats(ctx context.Context) (*dto.TicketStatsResponse, error) {
	total, err := s.ticketRepo.StatsTotal(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.ticketRepo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}
	avgReply, err := s.ticketRepo.AvgFirstReplySeconds(ctx)
	if err != nil {
		return nil, err
	}
	trendPoints, err := s.ticketRepo.StatsTrend(ctx, 7)
	if err != nil {
		return nil, err
	}
	categoryStats, err := s.ticketRepo.CountByCategory(ctx)
	if err != nil {
		return nil, err
	}

	byStatus := make(map[string]int64, len(counts))
	distribution := make([]dto.TicketStatusStat, 0, len(counts))
	for _, c := range counts {
		byStatus[c.Status] = c.Count
		distribution = append(distribution, dto.TicketStatusStat{Status: c.Status, Count: c.Count})
	}

	return &dto.TicketStatsResponse{
		Total:                total,
		Open:                 byStatus[model.TicketStatusOpen],
		InProgress:           byStatus[model.TicketStatusInProgress],
		WaitingUser:          byStatus[model.TicketStatusWaitingUser],
		Resolved:             byStatus[model.TicketStatusResolved],
		Closed:               byStatus[model.TicketStatusClosed] + byStatus[model.TicketStatusCancelled],
		AvgFirstReplySeconds: avgReply,
		StatusDistribution:   distribution,
		Trend:                fillTrend(trendPoints, 7),
		CategoryDistribution: buildCategoryDistribution(categoryStats),
	}, nil
}

// —— 用户端 ——

// accountUserIDs 返回账号及其子账号 ID 集合（P4-05）。
// 子账号登录时 userID 已是主账号 ID（由 handler 取 EffectiveUserID）。
func (s *ticketService) accountUserIDs(ctx context.Context, userID uint64) []uint64 {
	ids, err := s.ticketRepo.AccountUserIDs(ctx, userID)
	if err != nil || len(ids) == 0 {
		return []uint64{userID}
	}
	return ids
}

// belongsToAccount 判断工单是否属于该账号家族（主账号自身或任一子账号提交）。
func (s *ticketService) belongsToAccount(ctx context.Context, accountID, ticketUserID uint64) bool {
	for _, id := range s.accountUserIDs(ctx, accountID) {
		if id == ticketUserID {
			return true
		}
	}
	return false
}

func (s *ticketService) UserList(ctx context.Context, userID uint64, page, pageSize int) (*dto.TicketListResponse, error) {
	query := dto.TicketListQuery{
		UserID:   userID,
		UserIDs:  s.accountUserIDs(ctx, userID),
		Page:     page,
		PageSize: pageSize,
	}
	items, total, err := s.ticketRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.buildListResponse(ctx, items, total, normalizeP(page), normalizePS(pageSize))
}

// UserFindByID 用户查看工单详情：仅允许查看归属账号（主账号或成员）的工单。
func (s *ticketService) UserFindByID(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if !s.belongsToAccount(ctx, userID, item.UserID) {
		return nil, ErrTicketNotFound // 权限隔离：他人工单视为不存在
	}
	return s.buildDetail(ctx, *item)
}

// Create 用户提交工单：校验分类、按分类策略自动派单、落操作日志。
func (s *ticketService) Create(ctx context.Context, userID uint64, username string, req dto.TicketCreateRequest) (*dto.TicketDetail, error) {
	priority := req.Priority
	if priority == "" {
		priority = model.PriorityMedium
	}
	if !isValidPriority(priority) {
		return nil, ErrInvalidPriority
	}
	category, err := s.categoryRepo.FindByCode(ctx, req.Category)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}
	if category.Status != model.CategoryStatusActive {
		return nil, ErrCategoryNotFound
	}

	item := &model.Ticket{
		TicketNo:    genTicketNo(),
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Category:    category.Code,
		CategoryID:  category.ID,
		Priority:    priority,
		Status:      model.TicketStatusOpen,
		OrderID:     req.OrderID,
		InstanceID:  req.InstanceID,
	}
	// 自动派单（P2-03）：按分类的客服组 / 角色挑在岗且负载最低的员工；无匹配进未分配池。
	var groupID uint64
	if category.DefaultGroupID != nil {
		groupID = *category.DefaultGroupID
	}
	if assigneeID, assigneeName, err := s.ticketRepo.PickAssignee(ctx, category.DefaultRoleCode, groupID); err == nil && assigneeID > 0 {
		item.AssignedTo = assigneeID
		item.AssignedName = assigneeName
	}
	if err := s.ticketRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, userID, username, model.LogActionCreate, "", item.TicketNo, "")
	if item.AssignedTo > 0 {
		s.notifyAssign(ctx, *item)
	}
	return s.buildDetail(ctx, *item)
}

// UserReply 用户追加回复：写入回复并推进状态（waiting_user/resolved → in_progress）。
// accountID 为归属账号（用于权限校验），actorID/username 为真实操作人（子账号回复可追溯到人，P4-05）。
func (s *ticketService) UserReply(ctx context.Context, accountID, actorID uint64, username string, id uint64, content string) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if !s.belongsToAccount(ctx, accountID, item.UserID) {
		return nil, ErrTicketNotFound
	}
	if IsFinalStatus(item.Status) {
		return nil, ErrStatusConflict
	}
	// 用户回复：waiting_user / resolved → in_progress
	if item.Status == model.TicketStatusWaitingUser || item.Status == model.TicketStatusResolved {
		if err := EnsureStatus(item.Status, model.TicketStatusInProgress); err != nil {
			return nil, err
		}
		item.Status = model.TicketStatusInProgress
	}
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	if err := s.createReply(ctx, item.ID, model.SenderTypeUser, actorID, username, content); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, actorID, username, model.LogActionReply, "", "", "")
	return s.buildDetail(ctx, *item)
}

// Cancel 用户取消工单：仅 open 状态可取消。
func (s *ticketService) Cancel(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if !s.belongsToAccount(ctx, userID, item.UserID) {
		return nil, ErrTicketNotFound
	}
	if err := EnsureStatus(item.Status, model.TicketStatusCancelled); err != nil {
		return nil, err
	}
	from := item.Status
	item.Status = model.TicketStatusCancelled
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, userID, "", model.LogActionCancel, from, model.TicketStatusCancelled, "")
	return s.buildDetail(ctx, *item)
}

// —— 内部方法 ——

// notify 统一发送通知（失败仅忽略，不影响主流程）。
func (s *ticketService) notify(ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string) {
	if recipientID == 0 {
		return
	}
	_ = s.notifier.Publish(ctx, event, target, recipientID, vars, sourceID)
}

// notifyAssign 通知被指派人（管理员）。
func (s *ticketService) notifyAssign(ctx context.Context, item model.Ticket) {
	s.notify(ctx, notifEventTicketAssigned, "admin", item.AssignedTo,
		map[string]string{"ticket_no": item.TicketNo, "title": item.Title},
		fmt.Sprintf("ticket:%d:assign:%d", item.ID, item.AssignedTo))
}

// logAction 写入工单操作日志（失败仅忽略，不影响主流程）。
func (s *ticketService) logAction(ctx context.Context, ticketID, operatorID uint64, operatorName, action, from, to, note string) {
	_ = s.ticketRepo.CreateLog(ctx, &model.TicketLog{
		TicketID:     ticketID,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		Action:       action,
		FromValue:    from,
		ToValue:      to,
		Note:         note,
	})
}

// createReply 写入回复消息。
func (s *ticketService) createReply(ctx context.Context, ticketID uint64, senderType string, senderID uint64, senderName string, content string) error {
	return s.replyRepo.Create(ctx, &model.TicketReply{
		TicketID:   ticketID,
		SenderType: senderType,
		SenderID:   senderID,
		SenderName: senderName,
		Content:    content,
	})
}

// buildListResponse 构建列表响应（补充用户账号、分类、回复数、SLA）。
func (s *ticketService) buildListResponse(ctx context.Context, items []model.Ticket, total int64, page, pageSize int) (*dto.TicketListResponse, error) {
	userIDs := make([]uint64, 0, len(items))
	ticketIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
		ticketIDs = append(ticketIDs, item.ID)
	}
	userNames, err := s.ticketRepo.UserNamesByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	replyCounts, err := s.ticketRepo.CountByTicketIDs(ctx, ticketIDs)
	if err != nil {
		return nil, err
	}
	categories := s.categoryMap(ctx, items)

	respItems := make([]dto.TicketInfo, 0, len(items))
	for _, item := range items {
		info := buildTicketInfo(item, userNames[item.UserID], categories[item.Category], replyCounts[item.ID])
		respItems = append(respItems, info)
	}
	return &dto.TicketListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

// buildDetail 构建工单详情（含对话回复记录与操作日志时间线）。
func (s *ticketService) buildDetail(ctx context.Context, item model.Ticket) (*dto.TicketDetail, error) {
	replies, err := s.replyRepo.ListByTicket(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	logs, err := s.ticketRepo.ListLogs(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	userNames, err := s.ticketRepo.UserNamesByIDs(ctx, []uint64{item.UserID})
	if err != nil {
		return nil, err
	}
	categories := s.categoryMap(ctx, []model.Ticket{item})

	replyInfos := make([]dto.TicketReplyInfo, 0, len(replies))
	for _, r := range replies {
		replyInfos = append(replyInfos, buildReplyInfo(r))
	}
	logInfos := make([]dto.TicketLogInfo, 0, len(logs))
	for _, l := range logs {
		logInfos = append(logInfos, buildLogInfo(l))
	}
	detail := &dto.TicketDetail{
		TicketInfo:   buildTicketInfo(item, userNames[item.UserID], categories[item.Category], int64(len(replies))),
		Description:  item.Description,
		FirstReplyAt: formatTime(item.FirstReplyAt),
		ResolvedAt:   formatTime(item.ResolvedAt),
		ClosedAt:     formatTime(item.ClosedAt),
		Replies:      replyInfos,
		Logs:         logInfos,
	}
	return detail, nil
}

// categoryMap 批量查询分类编码 → 分类实体映射（用于名称与 SLA）。
func (s *ticketService) categoryMap(ctx context.Context, items []model.Ticket) map[string]model.TicketCategory {
	codes := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.Category != "" && !seen[item.Category] {
			seen[item.Category] = true
			codes = append(codes, item.Category)
		}
	}
	result := make(map[string]model.TicketCategory, len(codes))
	if len(codes) == 0 {
		return result
	}
	categories, err := s.categoryRepo.List(ctx, true)
	if err != nil {
		return result
	}
	for _, c := range categories {
		if seen[c.Code] {
			result[c.Code] = c
		}
	}
	return result
}

// —— 构建器与工具 ——

func buildTicketInfo(item model.Ticket, username string, category model.TicketCategory, replyCount int64) dto.TicketInfo {
	return dto.TicketInfo{
		ID:           item.ID,
		TicketNo:     item.TicketNo,
		UserID:       item.UserID,
		Username:     username,
		Title:        item.Title,
		Category:     item.Category,
		CategoryName: category.Name,
		Priority:     item.Priority,
		Status:       item.Status,
		AssignedTo:   item.AssignedTo,
		AssignedName: item.AssignedName,
		OrderID:      item.OrderID,
		InstanceID:   item.InstanceID,
		ReplyCount:   replyCount,
		SLAHours:     category.SLAHours,
		SLABreached:  slaBreached(item, category.SLAHours),
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
	}
}

// slaBreached 计算是否超时未达标（P2-05）：SLA 未启用返回 false；
// 未首次响应且已过时限，或首次响应晚于时限，均视为超时。
func slaBreached(item model.Ticket, slaHours int) bool {
	if slaHours <= 0 {
		return false
	}
	due := item.CreatedAt.Add(time.Duration(slaHours) * time.Hour)
	if item.FirstReplyAt == nil {
		return time.Now().After(due)
	}
	return item.FirstReplyAt.After(due)
}

func buildReplyInfo(r model.TicketReply) dto.TicketReplyInfo {
	return dto.TicketReplyInfo{
		ID:         r.ID,
		TicketID:   r.TicketID,
		SenderType: r.SenderType,
		SenderID:   r.SenderID,
		SenderName: r.SenderName,
		Content:    r.Content,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
	}
}

func buildLogInfo(l model.TicketLog) dto.TicketLogInfo {
	return dto.TicketLogInfo{
		ID:           l.ID,
		TicketID:     l.TicketID,
		OperatorID:   l.OperatorID,
		OperatorName: l.OperatorName,
		Action:       l.Action,
		FromValue:    l.FromValue,
		ToValue:      l.ToValue,
		Note:         l.Note,
		CreatedAt:    l.CreatedAt.Format(time.RFC3339),
	}
}

// applyStatusSideEffect 写入目标状态的伴生时间戳。
func applyStatusSideEffect(item *model.Ticket, target string) error {
	now := time.Now()
	switch target {
	case model.TicketStatusResolved:
		item.ResolvedAt = &now
	case model.TicketStatusClosed:
		item.ClosedAt = &now
	}
	return nil
}

// fillTrend 保证返回连续 N 日趋势点（缺失日期补零）。
func fillTrend(points []model.TicketTrendPoint, days int) []dto.TicketTrendPoint {
	byDate := make(map[string]model.TicketTrendPoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}
	result := make([]dto.TicketTrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		count := int64(0)
		if p, ok := byDate[d]; ok {
			count = p.Count
		}
		result = append(result, dto.TicketTrendPoint{Date: d, Count: count})
	}
	return result
}

// buildCategoryDistribution 构建分类分布（编码 → 显示名称）。
func buildCategoryDistribution(stats []model.TicketCategoryStat) []dto.TicketCategoryStat {
	result := make([]dto.TicketCategoryStat, 0, len(stats))
	for _, s := range stats {
		result = append(result, dto.TicketCategoryStat{Category: s.Category, Count: s.Count})
	}
	return result
}

// genTicketNo 生成工单号：TK + 时间戳 + 随机数。
func genTicketNo() string {
	return fmt.Sprintf("TK%s%03d", time.Now().Format("20060102150405"), rand.Intn(1000))
}

// formatTime 指针时间格式化（nil 返回空串）。
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func isValidPriority(priority string) bool {
	switch priority {
	case model.PriorityLow, model.PriorityMedium, model.PriorityHigh, model.PriorityUrgent:
		return true
	}
	return false
}

func normalizeP(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePS(pageSize int) int {
	if pageSize <= 0 {
		return 10
	}
	return pageSize
}

func mapTicketErr(err error) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return ErrTicketNotFound
	}
	return err
}
