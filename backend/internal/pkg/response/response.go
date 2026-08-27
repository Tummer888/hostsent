package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
)

// Body 统一响应体结构
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// PageData 分页数据载体
type PageData struct {
	Total    int64 `json:"total"`
	List     any   `json:"list"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Success 返回统一成功响应
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data, Timestamp: time.Now().Unix()})
}

// SuccessMessage 返回仅带 message 的成功响应（写操作）
func SuccessMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: message, Timestamp: time.Now().Unix()})
}

// Error 返回统一错误响应（HTTP 状态码固定为 200，业务状态由 code 区分，与现有约定一致）
func Error(c *gin.Context, err *apperrors.AppError) {
	c.JSON(http.StatusOK, Body{Code: err.Code, Message: err.Message, Timestamp: time.Now().Unix()})
}

// ErrorWithStatus 返回指定 HTTP 状态码的错误响应
func ErrorWithStatus(c *gin.Context, status int, err *apperrors.AppError) {
	c.JSON(status, Body{Code: err.Code, Message: err.Message, Timestamp: time.Now().Unix()})
}

// Page 返回统一分页成功响应
func Page(c *gin.Context, total int64, list any, page, pageSize int) {
	Success(c, PageData{Total: total, List: list, Page: page, PageSize: pageSize})
}
