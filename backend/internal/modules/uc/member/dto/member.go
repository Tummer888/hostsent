// Package dto 定义用户中心成员（子账号）模块的请求与响应结构。
package dto

import "time"

// MemberListQuery 成员列表查询。
type MemberListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

// CreateRequest 新建成员。
// Password 留空时由服务生成一次性密码，并在响应中仅返回一次。
type CreateRequest struct {
	Username    string   `json:"username" binding:"required,min=3,max=64"`
	Email       string   `json:"email" binding:"required,email"`
	Phone       string   `json:"phone"`
	Password    string   `json:"password"`
	Remark      string   `json:"remark"`
	Permissions []string `json:"permissions"`
}

// UpdateRequest 修改成员备注与状态。
type UpdateRequest struct {
	Remark string `json:"remark"`
	Status string `json:"status"`
}

// SetPermissionsRequest 覆盖式设置成员权限。
type SetPermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

// PermissionOption 可选权限项（前端渲染开关）。
type PermissionOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Info 成员信息。
type Info struct {
	ID          uint64     `json:"id"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Remark      string     `json:"remark"`
	Status      string     `json:"status"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// CreateResponse 新建成员响应：一次性密码仅在此返回。
type CreateResponse struct {
	Member   Info   `json:"member"`
	Password string `json:"password"`
}

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListResponse 成员列表响应。
type ListResponse struct {
	Items          []Info             `json:"items"`
	Meta           ListMeta           `json:"meta"`
	MaxSubAccounts int                `json:"max_sub_accounts"` // 主账号等级允许的子账号上限
	Permissions    []PermissionOption `json:"permission_options"`
}

// OperationLogInfo 成员操作日志项。
type OperationLogInfo struct {
	ID        uint64 `json:"id"`
	ActorID   uint64 `json:"actor_user_id"`
	ActorName string `json:"actor_name"`
	Module    string `json:"module"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

// OperationLogListResponse 成员操作日志列表。
type OperationLogListResponse struct {
	Items []OperationLogInfo `json:"items"`
	Meta  ListMeta           `json:"meta"`
}

// APIResponse 通用接口响应结构（仅用于接口文档标注）。
type APIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}
