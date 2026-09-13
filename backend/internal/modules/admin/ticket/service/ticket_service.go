package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
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

// ReplyInput 管理端回复入参（S2/S3）。
type ReplyInput struct {
	SenderID      uint64
	SenderName    string
	Content       string
	IsInternal    bool     // 内部备注：仅管理员可见，不推状态、不计首响、不通知用户
	AttachmentIDs []uint64 // 随回复携带的附件（先上传拿 ID）
}

// ReviewInput 复核入参（S3）。
type ReviewInput struct {
	ReviewerID   uint64
	ReviewerName string
	Action       string // approve / reject
	Note         string
}

// TicketService 定义工单域业务能力（管理端与用户端共用）。
type TicketService interface {
	// —— 管理端 ——
	List(ctx context.Context, q dto.TicketListQuery) (*dto.TicketListResponse, error)
	FindByID(ctx context.Context, id uint64) (*dto.TicketDetail, error)
	Reply(ctx context.Context, id uint64, in ReplyInput) (*dto.TicketDetail, error)
	Assign(ctx context.Context, id uint64, assignedTo uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	// Claim 认领未分配工单（P2-03）。
	Claim(ctx context.Context, id uint64, adminID uint64, adminName string) (*dto.TicketDetail, error)
	// Transfer 转派工单给其他员工（P2-03）。
	Transfer(ctx context.Context, id uint64, fromID uint64, fromName string, toID uint64, note string) (*dto.TicketDetail, error)
	UpdateStatus(ctx context.Context, id uint64, target string, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	Close(ctx context.Context, id uint64, operatorID uint64, operatorName string) (*dto.TicketDetail, error)
	Stats(ctx context.Context) (*dto.TicketStatsResponse, error)
	// —— S3 双人复核 ——
	// ListReviews 待复核回复队列（按权限数据范围过滤）。
	ListReviews(ctx context.Context, q dto.TicketListQuery) (*dto.ReviewQueueResponse, error)
	// ReviewReply 复核一条待复核回复，硬校验复核人 ≠ 提交人。
	ReviewReply(ctx context.Context, replyID uint64, in ReviewInput) (*dto.TicketDetail, error)
	// —— 用户端 ——
	UserList(ctx context.Context, userID uint64, page, pageSize int) (*dto.TicketListResponse, error)
	UserFindByID(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error)
	Create(ctx context.Context, userID uint64, username string, req dto.TicketCreateRequest) (*dto.TicketDetail, error)
	UserReply(ctx context.Context, accountID, actorID uint64, username string, id uint64, req dto.TicketReplyRequest) (*dto.TicketDetail, error)
	Cancel(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error)
	// AccountUserIDs 返回账号家族 ID（主账号 + 子账号），用户端附件鉴权用。
	AccountUserIDs(ctx context.Context, accountID uint64) []uint64
	// VisibleCategoriesByDepartment 取部门下的启用分类 code（数据范围注入用，S2）。
	// 返回空切片表示该部门没有可见分类，调用方据此收紧范围。
	VisibleCategoriesByDepartment(ctx context.Context, departmentID uint64) ([]string, error)
	// SetNotifier 延迟注入通知能力（通知服务在装配期晚于工单服务创建）。
	SetNotifier(n Notifier)
	// SetAttachmentService 延迟注入附件能力（附件服务依赖工单仓储，装配期后置）。
	SetAttachmentService(s AttachmentService)
	// SetPreconditionChecker 注入提交前置条件校验能力（S2）。
	SetPreconditionChecker(pc repository.PreconditionChecker)
}

type ticketService struct {
	ticketRepo   repository.TicketRepository
	replyRepo    repository.ReplyRepository
	categoryRepo repository.CategoryRepository
	attachment   AttachmentService
	precondition repository.PreconditionChecker
	notifier     Notifier
}

// NewTicketService 创建工单业务服务。notifier / attachment / precondition 均可为 nil。
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

// SetAttachmentService 延迟注入附件能力（附件服务在装配期后于工单服务构造）。
func (s *ticketService) SetAttachmentService(as AttachmentService) {
	s.attachment = as
}

// SetPreconditionChecker 注入提交前置条件校验能力（S2）。
func (s *ticketService) SetPreconditionChecker(pc repository.PreconditionChecker) {
	s.precondition = pc
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
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
}

// Reply 管理员回复工单（S2/S3）：
//   - 内部备注：不推进状态、不写首响、不发用户通知；
//   - 普通回复命中 need_review 分类：落 pending 等复核，副作用（首响/状态/用户通知）延后到复核通过；
//   - 普通回复未开启复核：沿用旧行为（open→in_progress、记首响、通知用户）。
func (s *ticketService) Reply(ctx context.Context, id uint64, in ReplyInput) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if IsFinalStatus(item.Status) {
		return nil, ErrStatusConflict
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, ErrReplyContentRequired
	}
	if err := s.validateAttachments(ctx, item.ID, in.AttachmentIDs); err != nil {
		return nil, err
	}

	needReview := false
	if !in.IsInternal {
		category, err := s.categoryRepo.FindByCode(ctx, item.Category)
		if err == nil {
			needReview = category.NeedReview
		}
	}

	replyStatus := model.ReviewStatusNone
	if needReview {
		replyStatus = model.ReviewStatusPending
	}

	// 内部备注与待复核回复都不产生用户可见副作用，状态与首响保持原样。
	if !in.IsInternal && !needReview {
		if item.Status == model.TicketStatusOpen {
			if err := EnsureStatus(item.Status, model.TicketStatusInProgress); err != nil {
				return nil, err
			}
			item.Status = model.TicketStatusInProgress
		}
		if item.FirstReplyAt == nil {
			now := time.Now()
			item.FirstReplyAt = &now
		}
	}
	if needReview {
		item.ReviewStatus = model.ReviewStatusPending
		item.ReviewRequestedBy = in.SenderID
	}
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}

	reply, err := s.createReply(ctx, item.ID, model.SenderTypeAdmin, in.SenderID, in.SenderName, in.Content, in.IsInternal, replyStatus, in.AttachmentIDs)
	if err != nil {
		return nil, err
	}

	switch {
	case in.IsInternal:
		s.logAction(ctx, item.ID, in.SenderID, in.SenderName, model.LogActionInternalNote, "", "", "")
	case needReview:
		// 复核过程属内部协作，不通知用户；改通知本部门复核人。
		s.logAction(ctx, item.ID, in.SenderID, in.SenderName, model.LogActionReviewRequest, "", replyStatus, "")
		s.notifyReviewers(ctx, *item, reply.ID, in.SenderName)
	default:
		s.logAction(ctx, item.ID, in.SenderID, in.SenderName, model.LogActionReply, "", "", "")
		s.notify(ctx, notifEventTicketReplied, "user", item.UserID,
			map[string]string{"ticket_no": item.TicketNo}, fmt.Sprintf("ticket:%d:reply:%d", item.ID, reply.ID))
	}
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
}

// ListReviews 待复核回复队列（S3）。
// VisibleDepartments 由 handler 按鉴权上下文注入：非超管仅见本部门（空表示全量）。
func (s *ticketService) ListReviews(ctx context.Context, q dto.TicketListQuery) (*dto.ReviewQueueResponse, error) {
	page, pageSize := normalizeP(q.Page), normalizePS(q.PageSize)
	replies, total, err := s.replyRepo.ListPendingReviews(ctx, q.VisibleDepartments, page, pageSize)
	if err != nil {
		return nil, err
	}
	if len(replies) == 0 {
		return &dto.ReviewQueueResponse{Items: []dto.ReviewQueueItem{}, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
	}

	ticketIDs := make([]uint64, 0, len(replies))
	senderIDs := make([]uint64, 0, len(replies))
	for _, r := range replies {
		ticketIDs = append(ticketIDs, r.TicketID)
		senderIDs = append(senderIDs, r.SenderID)
	}
	tickets, err := s.ticketRepo.FindByIDs(ctx, ticketIDs)
	if err != nil {
		return nil, err
	}
	senderNames, err := s.ticketRepo.AdminNameMap(ctx, senderIDs)
	if err != nil {
		return nil, err
	}
	categoryNames := s.categoryNameMap(ctx, tickets)
	departmentNames := s.departmentNamesByTicket(ctx, tickets)

	now := time.Now()
	items := make([]dto.ReviewQueueItem, 0, len(replies))
	for _, r := range replies {
		ticket := tickets[r.TicketID]
		name := senderNames[r.SenderID]
		if name == "" {
			name = r.SenderName
		}
		waiting := int64(now.Sub(r.CreatedAt).Seconds())
		if waiting < 0 {
			waiting = 0
		}
		items = append(items, dto.ReviewQueueItem{
			ReplyID:        r.ID,
			TicketID:       r.TicketID,
			TicketNo:       ticket.TicketNo,
			Title:          ticket.Title,
			Category:       ticket.Category,
			CategoryName:   categoryNames[ticket.Category],
			DepartmentID:   ticket.DepartmentID,
			DepartmentName: departmentNames[ticket.DepartmentID],
			SenderID:       r.SenderID,
			SenderName:     name,
			Content:        r.Content,
			WaitingSeconds: waiting,
			CreatedAt:      r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &dto.ReviewQueueResponse{Items: items, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

// ReviewReply 复核一条待复核回复（S3）。
//
// 放行条件（缺一不可）：回复处于 pending、动作合法、复核人不是提交人。
// 通过才补副作用（首响 / open→in_progress / 通知用户）；驳回则通知原提交客服改写重发。
// 只按回复 ID 定位：调用方（复核中心队列）手上未必有工单 ID，工单由回复反查。
func (s *ticketService) ReviewReply(ctx context.Context, replyID uint64, in ReviewInput) (*dto.TicketDetail, error) {
	reply, err := s.replyRepo.FindByID(ctx, replyID)
	if err != nil {
		return nil, ErrReplyNotFound
	}
	item, err := s.ticketRepo.FindByID(ctx, reply.TicketID)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if reply.ReviewStatus != model.ReviewStatusPending {
		return nil, ErrNotPendingReview
	}
	// 双人复核底线：提交人不得复核自己的回复。
	if reply.SenderID == in.ReviewerID {
		return nil, ErrReviewSelf
	}

	now := time.Now()
	switch in.Action {
	case model.ReviewActionApprove:
		reply.ReviewStatus = model.ReviewStatusApproved
		reply.ReviewerID = in.ReviewerID
		reply.ReviewedAt = &now
		reply.ReviewNote = strings.TrimSpace(in.Note)
	case model.ReviewActionReject:
		if strings.TrimSpace(in.Note) == "" {
			return nil, ErrReviewNoteRequired
		}
		reply.ReviewStatus = model.ReviewStatusRejected
		reply.ReviewerID = in.ReviewerID
		reply.ReviewedAt = &now
		reply.ReviewNote = strings.TrimSpace(in.Note)
	default:
		return nil, ErrInvalidReviewAction
	}
	if err := s.replyRepo.Update(ctx, reply); err != nil {
		return nil, err
	}

	item.ReviewStatus = reply.ReviewStatus
	item.ReviewerID = in.ReviewerID
	item.ReviewedAt = &now
	item.ReviewNote = reply.ReviewNote
	if in.Action == model.ReviewActionApprove {
		// 通过时才补上被判为「已响应」的副作用。
		if item.Status == model.TicketStatusOpen {
			item.Status = model.TicketStatusInProgress
		}
		if item.FirstReplyAt == nil {
			item.FirstReplyAt = &now
		}
	}
	if err := s.ticketRepo.Update(ctx, item); err != nil {
		return nil, err
	}

	s.logAction(ctx, item.ID, in.ReviewerID, in.ReviewerName, model.LogActionReview,
		model.ReviewStatusPending, reply.ReviewStatus, reply.ReviewNote)

	if in.Action == model.ReviewActionApprove {
		// 通过即视为该回复对用户公开，补一条用户可见的「回复」日志，
		// 否则用户侧时间线会缺一次回复记录（复核动作本身不外泄）。
		s.logAction(ctx, item.ID, reply.SenderID, reply.SenderName, model.LogActionReply, "", "", "")
		s.notify(ctx, notifEventTicketReplied, "user", item.UserID,
			map[string]string{"ticket_no": item.TicketNo}, fmt.Sprintf("ticket:%d:reply:%d:approved", item.ID, reply.ID))
	} else {
		s.notify(ctx, notifEventTicketReviewResult, "admin", reply.SenderID,
			map[string]string{"ticket_no": item.TicketNo, "result": "驳回", "note": "原因：" + reply.ReviewNote},
			fmt.Sprintf("ticket:%d:reply:%d:rejected", item.ID, reply.ID))
	}
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
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
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
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
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
}

// Transfer 转派工单：记录 from→to、通知新处理人，并告知用户已转交专人（S2）。
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
	s.notify(ctx, notifEventTicketTransferred, "user", item.UserID,
		map[string]string{"ticket_no": item.TicketNo, "title": item.Title},
		fmt.Sprintf("ticket:%d:transfer:%d", item.ID, time.Now().UnixNano()))
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
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
		return s.buildDetail(ctx, *item, detailViewer{manage: true}) // 幂等
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
	return s.buildDetail(ctx, *item, detailViewer{manage: true})
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

// AccountUserIDs 暴露账号家族 ID 给用户端附件鉴权复用（避免鉴权口径与查询口径分叉）。
func (s *ticketService) AccountUserIDs(ctx context.Context, accountID uint64) []uint64 {
	return s.accountUserIDs(ctx, accountID)
}

// VisibleCategoriesByDepartment 取部门下的启用分类 code（数据范围注入用）。
func (s *ticketService) VisibleCategoriesByDepartment(ctx context.Context, departmentID uint64) ([]string, error) {
	if departmentID == 0 {
		return nil, nil
	}
	return s.ticketRepo.ListCategoriesByDepartment(ctx, departmentID)
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

// UserFindByID 用户查看工单详情：仅允许查看归属账号（主账号或成员）的工单，
// 且回复/附件/日志按用户可见范围裁剪（S2）。
func (s *ticketService) UserFindByID(ctx context.Context, userID, id uint64) (*dto.TicketDetail, error) {
	item, err := s.ticketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapTicketErr(err)
	}
	if !s.belongsToAccount(ctx, userID, item.UserID) {
		return nil, ErrTicketNotFound // 权限隔离：他人工单视为不存在
	}
	return s.buildDetail(ctx, *item, detailViewer{accountID: userID})
}

// Create 用户提交工单：校验分类前置条件（S2）、按分类部门派单、落操作日志、绑定附件。
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
	userIDs := s.accountUserIDs(ctx, userID)
	if err := s.checkPreconditions(ctx, *category, userIDs, req); err != nil {
		return nil, err
	}

	item := &model.Ticket{
		TicketNo:     genTicketNo(),
		UserID:       userID,
		Title:        req.Title,
		Description:  req.Description,
		Category:     category.Code,
		CategoryID:   category.ID,
		Priority:     priority,
		Status:       model.TicketStatusOpen,
		DepartmentID: category.DepartmentID, // 部门快照：分类事后改部门不影响历史归属
		OrderID:      req.OrderID,
		InstanceID:   req.InstanceID,
	}
	// 自动派单（P2-03/S2）：优先按分类部门挑在岗且负载最低的员工；
	// 未配部门的存量分类回落到按角色挑人；都不匹配则进未分配池。
	if assigneeID, assigneeName, err := s.ticketRepo.PickAssignee(ctx, category.DefaultRoleCode, category.DepartmentID); err == nil && assigneeID > 0 {
		item.AssignedTo = assigneeID
		item.AssignedName = assigneeName
	}
	if err := s.ticketRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	if err := s.bindAttachments(ctx, item.ID, req.AttachmentIDs); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, userID, username, model.LogActionCreate, "", item.TicketNo, "")
	if item.AssignedTo > 0 {
		s.notifyAssign(ctx, *item)
	}
	return s.buildDetail(ctx, *item, detailViewer{accountID: userID})
}

// checkPreconditions 按分类开关校验提交前置条件（S2）。
// 三项都基于服务端数据判定，前端隐藏控件只是体验，不作为安全边界。
func (s *ticketService) checkPreconditions(
	ctx context.Context, category model.TicketCategory, userIDs []uint64, req dto.TicketCreateRequest,
) error {
	if s.precondition == nil {
		return nil
	}
	if category.RequireRealname {
		verified := false
		for _, uid := range userIDs {
			ok, err := s.precondition.IsRealnameVerified(ctx, uid)
			if err != nil {
				return err
			}
			if ok {
				verified = true
				break
			}
		}
		if !verified {
			return ErrRealnameRequired
		}
	}
	if category.RequireBinding {
		if req.OrderID == 0 && req.InstanceID == 0 {
			return ErrBindingRequired
		}
		owned, err := s.precondition.OwnsOrderOrInstance(ctx, userIDs, req.OrderID, req.InstanceID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrBindingNotOwned
		}
	}
	if allowed := category.RoleCodeList(); len(allowed) > 0 {
		roles, err := s.precondition.UserRoleCodes(ctx, userIDs[0])
		if err != nil {
			return err
		}
		if !containsAny(roles, allowed) {
			return ErrCategoryNotAllowed
		}
	}
	return nil
}

// UserReply 用户追加回复（S2：支持附件）。
// accountID 为归属账号（用于权限校验），actorID/username 为真实操作人（子账号回复可追溯到人，P4-05）。
func (s *ticketService) UserReply(ctx context.Context, accountID, actorID uint64, username string, id uint64, req dto.TicketReplyRequest) (*dto.TicketDetail, error) {
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
	if strings.TrimSpace(req.Content) == "" {
		return nil, ErrReplyContentRequired
	}
	if err := s.validateAttachments(ctx, item.ID, req.AttachmentIDs); err != nil {
		return nil, err
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
	// 用户侧不支持内部备注（服务端强制 false），也不走复核。
	if _, err := s.createReply(ctx, item.ID, model.SenderTypeUser, actorID, username, req.Content, false, model.ReviewStatusNone, req.AttachmentIDs); err != nil {
		return nil, err
	}
	s.logAction(ctx, item.ID, actorID, username, model.LogActionReply, "", "", "")
	return s.buildDetail(ctx, *item, detailViewer{accountID: accountID})
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
	return s.buildDetail(ctx, *item, detailViewer{accountID: userID})
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

// notifyReviewers 通知本部门复核人（S3）：support_lead 角色且同部门的在职员工。
func (s *ticketService) notifyReviewers(ctx context.Context, item model.Ticket, replyID uint64, senderName string) {
	reviewers, err := s.ticketRepo.ReviewerIDs(ctx, item.DepartmentID)
	if err != nil {
		return
	}
	for _, id := range reviewers {
		// 提交人不通知自己（也无法自审）；这里顺带省掉一次无效站内信。
		if id == item.ReviewRequestedBy {
			continue
		}
		s.notify(ctx, notifEventTicketReviewPending, "admin", id,
			map[string]string{"ticket_no": item.TicketNo, "title": item.Title, "sender": senderName},
			fmt.Sprintf("ticket:%d:reply:%d:review:%d", item.ID, replyID, id))
	}
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

// createReply 写入回复消息并挂载附件（返回落库后的回复，供日志与通知引用）。
func (s *ticketService) createReply(
	ctx context.Context, ticketID uint64, senderType string, senderID uint64, senderName string,
	content string, isInternal bool, reviewStatus string, attachmentIDs []uint64,
) (*model.TicketReply, error) {
	reply := &model.TicketReply{
		TicketID:     ticketID,
		SenderType:   senderType,
		SenderID:     senderID,
		SenderName:   senderName,
		Content:      content,
		IsInternal:   isInternal,
		ReviewStatus: reviewStatus,
	}
	if err := s.replyRepo.Create(ctx, reply); err != nil {
		return nil, err
	}
	if len(attachmentIDs) > 0 {
		if err := s.replyRepo.BindAttachments(ctx, reply.ID, attachmentIDs); err != nil {
			return nil, err
		}
	}
	return reply, nil
}

// validateAttachments 校验附件可挂到本工单（归属与数量）；未配置附件服务时直接放行。
func (s *ticketService) validateAttachments(ctx context.Context, ticketID uint64, ids []uint64) error {
	if len(ids) == 0 || s.attachment == nil {
		return nil
	}
	return s.attachment.ValidateBindable(ctx, ticketID, ids)
}

// bindAttachments 建单成功后把待挂载附件挂到工单。
func (s *ticketService) bindAttachments(ctx context.Context, ticketID uint64, ids []uint64) error {
	if len(ids) == 0 || s.attachment == nil {
		return nil
	}
	return s.attachment.BindTicket(ctx, ticketID, ids)
}

// buildListResponse 构建列表响应（补充用户账号、分类、回复数、部门、SLA）。
// review_status 保留原值：用户端据它提示「回复正在内部复核」（doc86 §4.2.2），不含复核人信息。
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
	departmentNames := s.departmentNameMap(ctx, items)

	respItems := make([]dto.TicketInfo, 0, len(items))
	for _, item := range items {
		info := buildTicketInfo(item, userNames[item.UserID], categories[item.Category], departmentNames[item.DepartmentID], replyCounts[item.ID])
		respItems = append(respItems, info)
	}
	return &dto.TicketListResponse{Items: respItems, Meta: dto.ListMeta{Page: page, PageSize: pageSize, Total: total}}, nil
}

// detailViewer 详情视角：manage=true 为管理端（可见内部备注/附件、复核信息）；
// accountID>0 为用户端（按回复级可见性裁剪）。
type detailViewer struct {
	manage    bool
	accountID uint64
}

// buildDetail 构建工单详情。
//
// 用户端不只看「哪些回复可见」，还要按可见回复的 ID 集合裁剪附件，避免用户拿到
// 挂在待复核回复上的附件；日志同样只保留 UserVisibleLogActions。
func (s *ticketService) buildDetail(ctx context.Context, item model.Ticket, viewer detailViewer) (*dto.TicketDetail, error) {
	var (
		replies     []model.TicketReply
		attachments []model.TicketAttachment
		logs        []model.TicketLog
		err         error
	)
	if viewer.manage {
		if replies, err = s.replyRepo.ListByTicket(ctx, item.ID); err != nil {
			return nil, err
		}
		if logs, err = s.ticketRepo.ListLogs(ctx, item.ID); err != nil {
			return nil, err
		}
	} else {
		if replies, err = s.replyRepo.ListByTicketVisible(ctx, item.ID); err != nil {
			return nil, err
		}
		if logs, err = s.ticketRepo.ListLogs(ctx, item.ID); err != nil {
			return nil, err
		}
		logs = filterUserVisibleLogs(logs)
		// 用户端不返回「谁在复核」，只保留 pending 作为「处理中」提示。
		if item.ReviewStatus != model.ReviewStatusPending {
			item.ReviewStatus = model.ReviewStatusNone
		}
	}
	if s.attachment != nil {
		if viewer.manage {
			attachments, err = s.attachment.ListByTicket(ctx, item.ID)
		} else {
			attachments, err = s.attachment.ListByTicketVisible(ctx, item.ID)
		}
		if err != nil {
			return nil, err
		}
	}

	attachmentInfos := make([]dto.TicketAttachmentInfo, 0, len(attachments))
	byReply := make(map[uint64][]dto.TicketAttachmentInfo, len(attachments))
	uploaderIDs := make([]uint64, 0, len(attachments))
	for _, a := range attachments {
		uploaderIDs = append(uploaderIDs, a.UploaderID)
	}
	uploaderNames, err := s.ticketRepo.AdminNameMap(ctx, uploaderIDs)
	if err != nil {
		uploaderNames = nil
	}
	for _, a := range attachments {
		info := buildAttachmentInfo(a)
		// 上传者姓名属内部信息（管理员展示用），用户端不下发。
		if viewer.manage {
			info.UploaderName = uploaderNames[a.UploaderID]
		}
		attachmentInfos = append(attachmentInfos, info)
		if a.ReplyID > 0 {
			byReply[a.ReplyID] = append(byReply[a.ReplyID], info)
		}
	}

	replyInfos := make([]dto.TicketReplyInfo, 0, len(replies))
	replyIDs := make([]uint64, 0, len(replies))
	for _, r := range replies {
		replyIDs = append(replyIDs, r.ID)
	}
	reviewerNames, rerr := s.ticketRepo.AdminNameMap(ctx, replyReviewerIDs(replies))
	if rerr != nil {
		reviewerNames = nil
	}
	for _, r := range replies {
		info := buildReplyInfo(r)
		if viewer.manage {
			info.ReviewerName = reviewerNames[r.ReviewerID]
		} else {
			// 用户端只看「这条回复是否还在复核」，复核人、时间与驳回原因属内部过程，不外泄。
			// 复核状态一并抹平：用户能看到的回复必然已通过，回传 approved 只会暴露内部流程存在。
			info.ReviewerID = 0
			info.ReviewedAt = ""
			info.ReviewNote = ""
			info.ReviewStatus = ""
		}
		if items, ok := byReply[r.ID]; ok {
			info.Attachments = items
		} else {
			info.Attachments = []dto.TicketAttachmentInfo{}
		}
		replyInfos = append(replyInfos, info)
	}

	logInfos := make([]dto.TicketLogInfo, 0, len(logs))
	for _, l := range logs {
		logInfos = append(logInfos, buildLogInfo(l))
	}
	userNames, err := s.ticketRepo.UserNamesByIDs(ctx, []uint64{item.UserID})
	if err != nil {
		return nil, err
	}
	categories := s.categoryMap(ctx, []model.Ticket{item})
	departmentName := s.departmentNameMap(ctx, []model.Ticket{item})[item.DepartmentID]

	detail := &dto.TicketDetail{
		TicketInfo:   buildTicketInfo(item, userNames[item.UserID], categories[item.Category], departmentName, int64(len(replies))),
		Description:  item.Description,
		FirstReplyAt: formatTime(item.FirstReplyAt),
		ResolvedAt:   formatTime(item.ResolvedAt),
		ClosedAt:     formatTime(item.ClosedAt),
		Replies:      replyInfos,
		Logs:         logInfos,
		Attachments:  attachmentInfos,
	}
	if viewer.manage {
		detail.ReviewNote = item.ReviewNote
		detail.ReviewedAt = formatTime(item.ReviewedAt)
		// 复核人姓名（管理端展示；用户端 review_status 只作提示，不给复核人）。
		if item.ReviewerID > 0 {
			if name, err := s.ticketRepo.AdminNameByID(ctx, item.ReviewerID); err == nil {
				detail.ReviewerName = name
			}
		}
	} else {
		// 用户端只需知道「有没有回复在内部复核中」：驳回/通过都不该向用户暴露审批过程，
		// 统一收敛为空串，只有 pending 保留给前端做「请稍候」提示。
		if detail.ReviewStatus != model.ReviewStatusPending {
			detail.ReviewStatus = model.ReviewStatusNone
		}
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

// categoryNameMap 分类编码 → 名称。
func (s *ticketService) categoryNameMap(ctx context.Context, tickets map[uint64]model.Ticket) map[string]string {
	items := make([]model.Ticket, 0, len(tickets))
	for _, t := range tickets {
		items = append(items, t)
	}
	categories := s.categoryMap(ctx, items)
	out := make(map[string]string, len(categories))
	for code, c := range categories {
		out[code] = c.Name
	}
	return out
}

// departmentNameMap 批量取工单归属部门名称。
func (s *ticketService) departmentNameMap(ctx context.Context, items []model.Ticket) map[uint64]string {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.DepartmentID > 0 {
			ids = append(ids, item.DepartmentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	names, err := s.ticketRepo.DepartmentNameMap(ctx, ids)
	if err != nil {
		return nil
	}
	return names
}

// departmentNamesByTicket 从 id → 工单的映射里取部门名称（复核队列用）。
func (s *ticketService) departmentNamesByTicket(ctx context.Context, tickets map[uint64]model.Ticket) map[uint64]string {
	items := make([]model.Ticket, 0, len(tickets))
	for _, t := range tickets {
		items = append(items, t)
	}
	return s.departmentNameMap(ctx, items)
}

// —— 构建器与工具 ——

func buildTicketInfo(item model.Ticket, username string, category model.TicketCategory, departmentName string, replyCount int64) dto.TicketInfo {
	return dto.TicketInfo{
		ID:             item.ID,
		TicketNo:       item.TicketNo,
		UserID:         item.UserID,
		Username:       username,
		Title:          item.Title,
		Category:       item.Category,
		CategoryName:   category.Name,
		Priority:       item.Priority,
		Status:         item.Status,
		AssignedTo:     item.AssignedTo,
		AssignedName:   item.AssignedName,
		DepartmentID:   item.DepartmentID,
		DepartmentName: departmentName,
		OrderID:        item.OrderID,
		InstanceID:     item.InstanceID,
		ReplyCount:     replyCount,
		ReviewStatus:   item.ReviewStatus,
		SLAHours:       category.SLAHours,
		SLABreached:    slaBreached(item, category.SLAHours),
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
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
		ID:           r.ID,
		TicketID:     r.TicketID,
		SenderType:   r.SenderType,
		SenderID:     r.SenderID,
		SenderName:   r.SenderName,
		Content:      r.Content,
		IsInternal:   r.IsInternal,
		ReviewStatus: r.ReviewStatus,
		ReviewerID:   r.ReviewerID,
		ReviewedAt:   formatTime(r.ReviewedAt),
		ReviewNote:   r.ReviewNote,
		Attachments:  []dto.TicketAttachmentInfo{},
		CreatedAt:    r.CreatedAt.Format(time.RFC3339),
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

// filterUserVisibleLogs 用户端日志裁剪（S2）：去掉内部备注与复核过程记录。
// 走白名单而非黑名单，新增内部动作时默认不外泄。
func filterUserVisibleLogs(logs []model.TicketLog) []model.TicketLog {
	if len(logs) == 0 {
		return logs
	}
	visible := make(map[string]struct{}, len(model.UserVisibleLogActions))
	for _, action := range model.UserVisibleLogActions {
		visible[action] = struct{}{}
	}
	out := make([]model.TicketLog, 0, len(logs))
	for _, l := range logs {
		if _, ok := visible[l.Action]; ok {
			out = append(out, l)
		}
	}
	return out
}

// replyReviewerIDs 提取回复的复核人 ID（去重，用于批量取名）。
func replyReviewerIDs(replies []model.TicketReply) []uint64 {
	ids := make([]uint64, 0, len(replies))
	seen := make(map[uint64]struct{}, len(replies))
	for _, r := range replies {
		if r.ReviewerID == 0 {
			continue
		}
		if _, ok := seen[r.ReviewerID]; ok {
			continue
		}
		seen[r.ReviewerID] = struct{}{}
		ids = append(ids, r.ReviewerID)
	}
	return ids
}

// containsAny 判断 src 与 want 是否有交集。
func containsAny(src, want []string) bool {
	set := make(map[string]struct{}, len(src))
	for _, s := range src {
		set[s] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
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
