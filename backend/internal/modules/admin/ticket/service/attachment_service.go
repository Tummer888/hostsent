package service

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/ticket/dto"
	"hostsent/backend/internal/modules/admin/ticket/model"
	"hostsent/backend/internal/modules/admin/ticket/repository"
	"hostsent/backend/internal/pkg/storage"
)

// 附件上传约束（S2）。集中在此便于前后端对齐与后续配置化。
const (
	// MaxAttachmentSize 单附件大小上限 10MB。
	MaxAttachmentSize int64 = 10 << 20
	// MaxAttachmentsPerTicket 单工单附件数上限，防止刷量。
	MaxAttachmentsPerTicket = 20
)

// allowedAttachmentExts 附件扩展名白名单：图片、压缩包、文本日志。
// 不放行可执行/脚本类后缀，避免把服务器变成恶意文件分发点。
var allowedAttachmentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
	".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
	".txt": true, ".log": true, ".json": true, ".csv": true, ".pdf": true, ".md": true,
}

// AttachmentService 工单附件业务能力（S2）。
type AttachmentService interface {
	// Upload 保存附件并落库。ticketID 为 0 时先落为待挂载附件（建单/回复时再绑定）。
	Upload(ctx context.Context, ticketID uint64, uploaderID uint64, fileName string, r io.Reader, isInternal bool) (*dto.TicketAttachmentInfo, error)
	// FindForDownload 校验下载权限并返回附件与文件句柄。
	// manageView=true 表示管理端视角（可见内部附件）；userIDs 为用户端账号家族校验用。
	Open(ctx context.Context, id uint64, manageView bool, userIDs []uint64) (*model.TicketAttachment, io.ReadCloser, error)
	// BindTicket 把待挂载附件关联到工单（建单成功后调用）。
	BindTicket(ctx context.Context, ticketID uint64, attachmentIDs []uint64) error
	// ValidateBindable 校验附件是否可挂到该工单（归属与数量）。
	ValidateBindable(ctx context.Context, ticketID uint64, attachmentIDs []uint64) error
	// ListByTicket 工单下全部附件（含内部附件，管理端视角）。
	ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error)
	// ListByTicketVisible 用户端可见附件（排除内部附件与待复核回复附件）。
	ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error)
}

type attachmentService struct {
	repo       repository.AttachmentRepository
	ticketRepo repository.TicketRepository
	store      *storage.LocalStore
}

// NewAttachmentService 创建附件业务服务。store 为 nil 时相关接口返回错误（装配缺失可被发现）。
func NewAttachmentService(repo repository.AttachmentRepository, ticketRepo repository.TicketRepository, store *storage.LocalStore) AttachmentService {
	return &attachmentService{repo: repo, ticketRepo: ticketRepo, store: store}
}

func (s *attachmentService) Upload(
	ctx context.Context, ticketID uint64, uploaderID uint64, fileName string, r io.Reader, isInternal bool,
) (*dto.TicketAttachmentInfo, error) {
	if s.store == nil {
		return nil, errors.New("附件存储未配置")
	}
	fileName = strings.TrimSpace(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	if fileName == "" || fileName == "." || fileName == "/" {
		return nil, ErrAttachmentNotFound
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedAttachmentExts[ext] {
		return nil, ErrAttachmentTypeNotAllowed
	}
	if ticketID > 0 {
		if _, err := s.ticketRepo.FindByID(ctx, ticketID); err != nil {
			return nil, mapTicketErr(err)
		}
		count, err := s.countByTicket(ctx, ticketID)
		if err != nil {
			return nil, err
		}
		if count >= MaxAttachmentsPerTicket {
			return nil, ErrAttachmentTooLarge
		}
	}

	// 目录按工单号（或待挂载）分桶，便于运维按工单清理。
	dir := "pending"
	if ticketID > 0 {
		if ticket, err := s.ticketRepo.FindByID(ctx, ticketID); err == nil {
			dir = filepath.ToSlash(filepath.Join("tickets", ticket.TicketNo))
		}
	}
	relPath, size, err := s.store.Save(dir, fileName, r, MaxAttachmentSize)
	if err != nil {
		if strings.Contains(err.Error(), "超过大小上限") {
			return nil, ErrAttachmentTooLarge
		}
		return nil, err
	}
	item := &model.TicketAttachment{
		TicketID:   ticketID,
		FileName:   fileName,
		FileURL:    relPath,
		FileSize:   size,
		FileType:   ext,
		UploaderID: uploaderID,
		IsInternal: isInternal,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		// 落库失败时清理已落盘文件，避免孤儿文件堆积。
		_ = s.store.Remove(relPath)
		return nil, err
	}
	info := buildAttachmentInfo(*item)
	return &info, nil
}

// Open 下载鉴权（S2）：
//   - 管理端：可下载工单下全部附件（含内部附件）；
//   - 用户端：仅本人账号家族的工单附件，且内部附件与待复核回复附件不可见。
func (s *attachmentService) Open(
	ctx context.Context, id uint64, manageView bool, userIDs []uint64,
) (*model.TicketAttachment, io.ReadCloser, error) {
	if s.store == nil {
		return nil, nil, errors.New("附件存储未配置")
	}
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, ErrAttachmentNotFound
	}
	if !manageView {
		if item.IsInternal {
			return nil, nil, ErrAttachmentNotFound
		}
		ticket, terr := s.ticketRepo.FindByID(ctx, item.TicketID)
		if terr != nil {
			return nil, nil, ErrAttachmentNotFound
		}
		owned := false
		for _, uid := range userIDs {
			if uid == ticket.UserID {
				owned = true
				break
			}
		}
		if !owned {
			return nil, nil, ErrAttachmentNotFound
		}
		// 挂在待复核/已驳回回复上的附件同样不可下载，避免绕过复核泄露内容。
		if item.ReplyID > 0 {
			visible, verr := isReplyVisible(ctx, s.repo, item.TicketID, item.ReplyID)
			if verr != nil || !visible {
				return nil, nil, ErrAttachmentNotFound
			}
		}
	}
	f, err := s.store.Open(item.FileURL)
	if err != nil {
		if errors.Is(err, storage.ErrPathEscape) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrAttachmentNotFound
		}
		return nil, nil, err
	}
	return item, f, nil
}

// ValidateBindable 校验附件可挂载：必须存在、未挂到别的工单、且数量不超额。
func (s *attachmentService) ValidateBindable(ctx context.Context, ticketID uint64, attachmentIDs []uint64) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	items, err := s.repo.ListByIDs(ctx, attachmentIDs)
	if err != nil {
		return err
	}
	if len(items) != len(dedupeIDs(attachmentIDs)) {
		return ErrAttachmentNotFound
	}
	for _, item := range items {
		// ticket_id=0 表示待挂载；等于本工单表示重复提交同一批附件（幂等放行）。
		if item.TicketID != 0 && item.TicketID != ticketID {
			return ErrAttachmentNotFound
		}
	}
	return nil
}

// BindTicket 把 ticket_id=0 的待挂载附件挂到工单。
func (s *attachmentService) BindTicket(ctx context.Context, ticketID uint64, attachmentIDs []uint64) error {
	if len(attachmentIDs) == 0 {
		return nil
	}
	return s.repoBindTicket(ctx, ticketID, attachmentIDs)
}

// repoBindTicket 通过仓储批量更新 ticket_id（仅处理未挂载的，防止跨工单搬附件）。
func (s *attachmentService) repoBindTicket(ctx context.Context, ticketID uint64, attachmentIDs []uint64) error {
	items, err := s.repo.ListByIDs(ctx, attachmentIDs)
	if err != nil {
		return err
	}
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.TicketID == 0 || item.TicketID == ticketID {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.repo.BindTicket(ctx, ticketID, ids)
}

// ListByTicket 工单下全部附件（管理端视角）。
func (s *attachmentService) ListByTicket(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error) {
	return s.repo.ListByTicket(ctx, ticketID)
}

// ListByTicketVisible 用户端可见附件（内部附件与待复核回复的附件都不可见）。
func (s *attachmentService) ListByTicketVisible(ctx context.Context, ticketID uint64) ([]model.TicketAttachment, error) {
	return s.repo.ListByTicketVisible(ctx, ticketID)
}

func (s *attachmentService) countByTicket(ctx context.Context, ticketID uint64) (int64, error) {
	items, err := s.repo.ListByTicket(ctx, ticketID)
	if err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

// isReplyVisible 判断某回复对用户是否可见（非内部备注且非待复核/已驳回）。
func isReplyVisible(ctx context.Context, repo repository.AttachmentRepository, ticketID, replyID uint64) (bool, error) {
	visible, err := repo.ListByTicketVisible(ctx, ticketID)
	if err != nil {
		return false, err
	}
	for _, item := range visible {
		if item.ReplyID == replyID {
			return true, nil
		}
	}
	return false, nil
}

func buildAttachmentInfo(item model.TicketAttachment) dto.TicketAttachmentInfo {
	return dto.TicketAttachmentInfo{
		ID:         item.ID,
		TicketID:   item.TicketID,
		ReplyID:    item.ReplyID,
		FileName:   item.FileName,
		FileURL:    item.FileURL,
		FileSize:   item.FileSize,
		FileType:   item.FileType,
		UploaderID: item.UploaderID,
		IsInternal: item.IsInternal,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
	}
}

func dedupeIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
