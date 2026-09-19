package service

// 实名材料上传与下载（doc104 §5.7）。
//
// 复用工单附件的既有机制（storage.LocalStore + 扩展名白名单 + 大小上限），
// 不新造上传通道：多一条上传链路就多一处需要单独加固的入口。
// 与工单附件的差别只有两点，都在这里显式处理：
//   1. 材料只能挂到「自己的、且仍在待审核」的申请上 —— 已通过/已驳回的单子
//      不能再补材料，否则审核员看到的材料与审核结论可能不一致；
//   2. 下载必须走鉴权端点。工单附件是「账号家族可见」，实名材料是敏感个人信息，
//      只有申请本人与该单的管理端审核员能取到。

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"hostsent/backend/internal/modules/admin/user/verification/dto"
	"hostsent/backend/internal/modules/admin/user/verification/model"
)

// 材料上传约束。
const (
	// MaxDocumentSize 单份材料大小上限 10MB，与工单附件同口径。
	MaxDocumentSize int64 = 10 << 20
	// MaxDocumentsPerApplication 单申请材料数上限。
	MaxDocumentsPerApplication = 10
)

// allowedDocumentExts 材料扩展名白名单：只收图片与 PDF。
//
// 与工单附件相比刻意更窄：实名材料是证件影像，不需要压缩包/日志这类形态，
// 收窄白名单等于减少一类需要审计的文件类型。
var allowedDocumentExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".bmp": true, ".pdf": true,
}

// allowedDocumentTypes 材料类型白名单（与前端选项、管理端标签一一对应）。
var allowedDocumentTypes = map[string]bool{
	"id_front":         true,
	"id_back":          true,
	"handheld":         true,
	"business_license": true,
	"authorization":    true,
}

// 材料相关哨兵错误。
var (
	// ErrDocumentStoreNotConfigured 存储未装配（配置缺失），属部署问题。
	ErrDocumentStoreNotConfigured = errors.New("材料存储未配置，请联系管理员")
	// ErrDocumentTypeNotAllowed 材料类型不在白名单内。
	ErrDocumentTypeNotAllowed = errors.New("不支持的材料类型")
	// ErrDocumentExtNotAllowed 文件扩展名不在白名单内。
	ErrDocumentExtNotAllowed = errors.New("仅支持图片（png/jpg/webp/bmp）与 PDF")
	// ErrDocumentTooLarge 文件超过大小上限。
	ErrDocumentTooLarge = errors.New("文件超过 10MB 上限")
	// ErrDocumentLimitExceeded 单申请材料数超限。
	ErrDocumentLimitExceeded = errors.New("材料数量已达上限")
	// ErrDocumentNotFound 材料不存在。
	ErrDocumentNotFound = errors.New("材料不存在")
)

// DocumentStore 材料存储端口。
//
// 声明成本包的接口而不是直接依赖 *storage.LocalStore：实名服务不该 import
// 文件系统实现，装配层负责把本地存储适配进来（与 VerifyPort 同款做法）。
type DocumentStore interface {
	Save(dir, filename string, r io.Reader, maxSize int64) (string, int64, error)
	Open(relPath string) (io.ReadCloser, error)
	Remove(relPath string) error
}

// UploadDocument 上传一份实名材料。
//
// 归属与状态双重校验：
//   - 申请必须属于当前用户（越权一律按「不存在」处理，不泄露存在性）；
//   - 申请必须仍是 pending —— 审核结论已下后不允许再改材料。
func (s *verificationService) UploadDocument(
	ctx context.Context, userID, applicationID uint64, docType, fileName string, r io.Reader,
) (*dto.DocumentInfo, error) {
	if s.documents == nil {
		return nil, ErrDocumentStoreNotConfigured
	}
	docType = strings.TrimSpace(docType)
	if !allowedDocumentTypes[docType] {
		return nil, ErrDocumentTypeNotAllowed
	}
	// 文件名只取 basename：上传方可能带路径，拼进存储目录会越权写到别处。
	fileName = strings.TrimSpace(filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	if fileName == "" || fileName == "." || fileName == "/" {
		return nil, ErrDocumentExtNotAllowed
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedDocumentExts[ext] {
		return nil, ErrDocumentExtNotAllowed
	}

	app, err := s.repo.FindByID(ctx, applicationID)
	if err != nil || app.UserID != userID {
		return nil, ErrApplicationNotFound
	}
	if app.Status != model.StatusPending {
		return nil, ErrStatusConflict
	}

	existing, err := s.repo.ListDocuments(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	// 同类型重传是覆盖（不占配额），只有新增类型才计数。
	if len(existing) >= MaxDocumentsPerApplication {
		replacing := false
		for i := range existing {
			if existing[i].DocumentType == docType {
				replacing = true
				break
			}
		}
		if !replacing {
			return nil, ErrDocumentLimitExceeded
		}
	}

	relPath, _, err := s.documents.Save(docDir(applicationID), fileName, r, MaxDocumentSize)
	if err != nil {
		if strings.Contains(err.Error(), "超过大小上限") {
			return nil, ErrDocumentTooLarge
		}
		return nil, err
	}

	row := &model.VerificationDocument{
		ApplicationID: applicationID,
		DocumentType:  docType,
		FileURL:       relPath,
		Sort:          len(existing) + 1,
	}
	if err := s.repo.SaveDocument(ctx, row); err != nil {
		// 落库失败清理已落盘文件，避免孤儿文件堆积（与工单附件同款处理）。
		_ = s.documents.Remove(relPath)
		return nil, err
	}
	return &dto.DocumentInfo{
		ID: row.ID, DocumentType: row.DocumentType, FileURL: row.FileURL, Sort: row.Sort,
	}, nil
}

// OpenDocument 打开材料供下载。
//
// manageView=true 为管理端视角（审核员看任意单子的材料）；false 时要求申请属于该用户。
func (s *verificationService) OpenDocument(
	ctx context.Context, docID uint64, manageView bool, userID uint64,
) (*dto.DocumentInfo, io.ReadCloser, error) {
	if s.documents == nil {
		return nil, nil, ErrDocumentStoreNotConfigured
	}
	doc, err := s.repo.FindDocument(ctx, docID)
	if err != nil {
		return nil, nil, ErrDocumentNotFound
	}
	if !manageView {
		app, err := s.repo.FindByID(ctx, doc.ApplicationID)
		// 不属于本人时同样返回「不存在」，不泄露他人申请的存在性。
		if err != nil || app.UserID != userID {
			return nil, nil, ErrDocumentNotFound
		}
	}
	reader, err := s.documents.Open(doc.FileURL)
	if err != nil {
		return nil, nil, ErrDocumentNotFound
	}
	return &dto.DocumentInfo{
		ID: doc.ID, DocumentType: doc.DocumentType, FileURL: doc.FileURL, Sort: doc.Sort,
	}, reader, nil
}

// DocumentFileName 从相对路径推出下载文件名（落盘名是随机串，下载时要还原成语义名）。
func DocumentFileName(docType, relPath string) string {
	ext := filepath.Ext(relPath)
	if ext == "" {
		ext = ".bin"
	}
	if docType == "" {
		docType = "document"
	}
	return docType + ext
}

// DocumentContentType 按扩展名给 MIME；未知类型一律按二进制流下发，
// 绝不内联渲染用户上传的内容（与工单附件同款加固）。
func DocumentContentType(relPath string) string {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// docDir 材料落盘目录：按申请分桶，便于随申请单一起清理。
func docDir(applicationID uint64) string {
	return "verification/" + strconv.FormatUint(applicationID, 10)
}
