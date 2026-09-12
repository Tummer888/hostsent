package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/payment/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/payment"
	"hostsent/backend/internal/pkg/response"
)

// NotifyHandler 渠道异步通知入口（免登录，以渠道验签为唯一信任来源）。
// 路由为 /api/v1/payment/notify/:channel_code，不挂 adminAuth/userAuth。
type NotifyHandler struct {
	svc service.OrderService
}

// NewNotifyHandler 创建渠道通知入口。
func NewNotifyHandler(svc service.OrderService) *NotifyHandler {
	return &NotifyHandler{svc: svc}
}

// Notify godoc
// @Summary 支付渠道异步回调（验签后入账）
// @Tags 支付中心-回调
// @Param channel_code path string true "渠道编码"
// @Success 200 {string} string "渠道要求的应答"
// @Router /api/v1/payment/notify/{channel_code} [post]
func (h *NotifyHandler) Notify(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, apperrors.New(20001, "读取回调报文失败"))
		return
	}
	event, err := h.svc.HandleNotify(c.Request.Context(), c.Param("channel_code"), c.Request, raw, c.ClientIP())
	if err != nil {
		// 验签失败/金额不符：不回显内部细节，只返回失败标识。
		response.Error(c, paymentNotifyError(err))
		return
	}
	// 渠道侧通常要求纯文本 "success"；这里统一返回成功标识，渠道按自身约定忽略多余字段。
	response.Success(c, gin.H{"handled": true, "status": event.Status})
}

// paymentNotifyError 回调错误 → 统一错误码（对外不暴露验签细节）。
func paymentNotifyError(err error) *apperrors.AppError {
	if err == nil {
		return nil
	}
	switch {
	case err == payment.ErrSignatureInvalid || err == payment.ErrAmountMismatch:
		return apperrors.New(30009, "回调校验失败")
	case err == payment.ErrOrderNotFound || err == service.ErrOrderNotFound:
		return apperrors.New(20002, "支付单不存在")
	default:
		return apperrors.New(50001, "回调处理失败")
	}
}
