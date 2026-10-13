package middleware

import (
	"context"
)

// AdminAuditEntry 管理端操作审计落库条目（纯 DTO，不含 GORM 标签）。
//
// pkg 禁止 import modules（CONTRIBUTING §3）：本结构是 admin/manager/model.
// AdminAuditLog 的 pkg 面投影，由装配层的适配器把条目翻译成对应模型落库。
// 字段与 model 同名同型，映射零歧义。
type AdminAuditEntry struct {
	AdminID       uint64
	AdminName     string
	Action        string
	ResourceType  string
	ResourceID    string
	Detail        *string
	IP            string
	UserAgent     string
	Module        string
	RequestMethod string
	RequestPath   string
	ResponseCode  int
	TraceID       string
}

// AdminAuditWriter 管理端操作审计落库能力（由装配层注入适配器实现，
// 避免 middleware 直接持有模块 model 类型）。
type AdminAuditWriter interface {
	CreateAuditLog(ctx context.Context, entry *AdminAuditEntry) error
}

// UserOperationLogEntry 用户侧操作日志落库条目（纯 DTO，不含 GORM 标签）。
//
// 是 uc/member/model.OperationLog 的 pkg 面投影，字段同名同型；
// 由装配层的适配器翻译成模型落库。
type UserOperationLogEntry struct {
	ActorUserID   uint64
	ActorName     string
	AccountUserID uint64
	Module        string
	Action        string
	Target        string
	Detail        string
	IP            string
}

// UserOperationLogWriter 用户侧操作日志落库能力（由装配层注入适配器实现）。
type UserOperationLogWriter interface {
	CreateOperationLog(ctx context.Context, entry *UserOperationLogEntry) error
}
