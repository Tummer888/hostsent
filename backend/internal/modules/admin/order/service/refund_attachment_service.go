package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"hostsent/backend/internal/modules/admin/order/dto"
	"hostsent/backend/internal/modules/admin/order/model"
	"hostsent/backend/internal/modules/admin/order/repository"
	"hostsent/backend/internal/pkg/storage"
)

// 凭据附件约束（与工单附件同口径：资金审计材料要求文件名可读、类型可控）。
const (
	// MaxRefundAttachmentSize 单文件上限 10MB。
	MaxRefundAttachmentSize = 10 * 1024 * 1024
	// MaxRefundAttachments 单张退款单附件上限。
	MaxRefundAttachments = 10
)

// refundAttachmentExtWhitelist 允许的扩展名：转账截图、回单 PDF、渠道导出的对账单等。
var refundAttachmentExtWhitelist = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
	".pdf": true, ".csv": true, ".xlsx": true, ".txt": true, ".zip": true,
}

var (
	// ErrRefundAttachmentStoreUnavailable 附件存储未启用。
	ErrRefundAttachmentStoreUnavailable = errors.New("退款凭据附件服务未启用")
	// ErrRefundAttachmentTooLarge 附件超过单文件上限。
	ErrRefundAttachmentTooLarge = errors.New("附件超过 10MB 上限")
	// ErrRefundAttachmentType 附件类型不在白名单内。
	ErrRefundAttachmentType = errors.New("附件类型不允许（仅支持图片/PDF/表格/压缩包）")
	// ErrRefundAttachmentNotFound 附件不存在。
	ErrRefundAttachmentNotFound = errors.New("附件不存在")
	// ErrRefundAttachmentLimit 附件数量超限。
	ErrRefundAttachmentLimit = errors.New("该退款单附件数量已达上限")
)

// RefundAttachmentService 退款审核凭据附件能力。
type RefundAttachmentService interface {
	// Upload 上传凭据附件并绑定到退款单。
	Upload(ctx context.Context, refundID, uploaderID uint64, fileName string, r io.Reader, size int64) (*dto.RefundAttachmentInfo, error)
	// ListByRefund 列出某退款单的全部凭据附件。
	ListByRefund(ctx context.Context, refundID uint64) ([]dto.RefundAttachmentInfo, error)
	// Open 打开附件用于鉴权下载。
	Open(ctx context.Context, id uint64) (*model.RefundAttachment, io.ReadCloser, error)
}

type refundAttachmentService struct {
	repo  repository.RefundAttachmentRepository
	store *storage.LocalStore
}

// NewRefundAttachmentService 创建退款凭据附件服务。
// store 为 nil 时表示附件功能未启用（上传/下载返回明确错误，不影响退款主流程）。
func NewRefundAttachmentService(repo repository.RefundAttachmentRepository, store *storage.LocalStore) RefundAttachmentService {
	return &refundAttachmentService{repo: repo, store: store}
}

func (s *refundAttachmentService) Upload(ctx context.Context, refundID, uploaderID uint64, fileName string, r io.Reader, size int64) (*dto.RefundAttachmentInfo, error) {
	if s.store == nil {
		return nil, ErrRefundAttachmentStoreUnavailable
	}
	if size > MaxRefundAttachmentSize {
		return nil, ErrRefundAttachmentTooLarge
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !refundAttachmentExtWhitelist[ext] {
		return nil, ErrRefundAttachmentType
	}
	// 数量上限：先查后传（并发下的轻微超限可接受，不为此加锁）。
	existing, err := s.repo.ListByRefund(ctx, refundID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= MaxRefundAttachments {
		return nil, ErrRefundAttachmentLimit
	}

	dir := fmt.Sprintf("refunds/%d", refundID)
	relPath, written, err := s.store.Save(dir, fileName, r, MaxRefundAttachmentSize)
	if err != nil {
		return nil, err
	}
	item := &model.RefundAttachment{
		RefundID:   refundID,
		FileName:   sanitizeAttachmentName(fileName),
		FileURL:    relPath,
		FileSize:   written,
		FileType:   ext,
		UploaderID: uploaderID,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		// 落库失败清理已写入的文件，避免孤儿文件占空间。
		_ = s.store.Remove(relPath)
		return nil, err
	}
	info := buildRefundAttachmentInfo(*item)
	return &info, nil
}

func (s *refundAttachmentService) ListByRefund(ctx context.Context, refundID uint64) ([]dto.RefundAttachmentInfo, error) {
	items, err := s.repo.ListByRefund(ctx, refundID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RefundAttachmentInfo, 0, len(items))
	for _, it := range items {
		out = append(out, buildRefundAttachmentInfo(it))
	}
	return out, nil
}

func (s *refundAttachmentService) Open(ctx context.Context, id uint64) (*model.RefundAttachment, io.ReadCloser, error) {
	if s.store == nil {
		return nil, nil, ErrRefundAttachmentStoreUnavailable
	}
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, ErrRefundAttachmentNotFound
	}
	rc, err := s.store.Open(item.FileURL)
	if err != nil {
		return nil, nil, ErrRefundAttachmentNotFound
	}
	return item, rc, nil
}

// sanitizeAttachmentName 只保留文件名本身（去掉客户端可能带上的路径分隔符）。
func sanitizeAttachmentName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return "attachment"
	}
	return name
}

func buildRefundAttachmentInfo(item model.RefundAttachment) dto.RefundAttachmentInfo {
	return dto.RefundAttachmentInfo{
		ID:         item.ID,
		RefundID:   item.RefundID,
		FileName:   item.FileName,
		FileSize:   item.FileSize,
		FileType:   item.FileType,
		UploaderID: item.UploaderID,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
	}
}
