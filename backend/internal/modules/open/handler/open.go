// Package handler 开放平台 HTTP 层（P6）。Bundle 聚合网关与各业务 handler，
// 由 server 装配层以单一字段注入 App，路由注册只经 Bundle 暴露的方法。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"hostsent/backend/internal/modules/open/dto"
	"hostsent/backend/internal/modules/open/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// Bundle 开放平台处理器集合。各业务服务经 Set* 注入。
type Bundle struct {
	gw       *service.Gateway
	catalog  *service.CatalogService
	order    *service.OpenOrderService
	instance *service.OpenInstanceService
	audit    *service.OpenAuditService
	logger   *zap.Logger
}

// NewBundle 构造处理器集合（业务服务可后置注入）。
func NewBundle(gw *service.Gateway, logger *zap.Logger) *Bundle {
	return &Bundle{gw: gw, logger: logger}
}

// SetCatalog 注入目录服务（T6.2）。
func (b *Bundle) SetCatalog(catalog *service.CatalogService) { b.catalog = catalog }

// SetOrder 注入代客下单服务（T6.3）。
func (b *Bundle) SetOrder(order *service.OpenOrderService) { b.order = order }

// SetInstance 注入实例服务（T6.4）。
func (b *Bundle) SetInstance(instance *service.OpenInstanceService) { b.instance = instance }

// SetAudit 注入对账服务（T6.6）。
func (b *Bundle) SetAudit(audit *service.OpenAuditService) { b.audit = audit }

// Audit 请求/响应审计（分组最外层）。
func (b *Bundle) Audit() gin.HandlerFunc { return b.gw.Audit() }

// Gateway 签名与应用鉴权链。
func (b *Bundle) Gateway() gin.HandlerFunc { return b.gw.Middleware() }

// RequireScope 声明路由所需能力位。
func (b *Bundle) RequireScope(scope string) gin.HandlerFunc { return b.gw.RequireScope(scope) }

// Ping 链路自检：签名通过即可访问，不占能力位。返回应用身份供联调核对。
func (b *Bundle) Ping(c *gin.Context) {
	app := service.AppFromContext(c)
	if app == nil {
		// 防御分支：Ping 挂在鉴权链之后，正常请求不会走到这里。
		response.Error(c, apperrors.New(service.CodeOpenMissingAuth, "缺少认证头"))
		return
	}
	response.Success(c, gin.H{
		"app_id":        app.App.AppID,
		"name":          app.App.Name,
		"api_version":   app.App.APIVersion,
		"owner_user_id": app.App.OwnerUserID,
		"scopes":        app.ScopeNames(),
	})
}

// ListSpecAtoms GET /open/v1/spec-atoms
func (b *Bundle) ListSpecAtoms(c *gin.Context) {
	items, err := b.catalog.ListSpecAtoms(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(service.CodeOpenInternal, err.Error()))
		return
	}
	response.Success(c, items)
}

// ListProducts GET /open/v1/products
func (b *Bundle) ListProducts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := b.catalog.ListProducts(c.Request.Context(),
		c.Query("keyword"),
		parseUint64(c.Query("category_id")),
		page, pageSize)
	if err != nil {
		response.Error(c, apperrors.New(service.CodeOpenInternal, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetProduct GET /open/v1/products/:id
func (b *Bundle) GetProduct(c *gin.Context) {
	id := parseUint64(c.Param("id"))
	if id == 0 {
		response.Error(c, apperrors.New(service.CodeOpenParam, "id 非法"))
		return
	}
	detail, err := b.catalog.GetProduct(c.Request.Context(), id)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, detail)
}

// ListRegions GET /open/v1/regions
func (b *Bundle) ListRegions(c *gin.Context) {
	items, err := b.catalog.ListRegions(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(service.CodeOpenInternal, err.Error()))
		return
	}
	response.Success(c, items)
}

// ListImages GET /open/v1/images
func (b *Bundle) ListImages(c *gin.Context) {
	items, err := b.catalog.ListImages(c.Request.Context())
	if err != nil {
		response.Error(c, apperrors.New(service.CodeOpenInternal, err.Error()))
		return
	}
	response.Success(c, items)
}

// Quote POST /open/v1/quote
func (b *Bundle) Quote(c *gin.Context) {
	var req dto.QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(service.CodeOpenParam, err.Error()))
		return
	}
	app := service.AppFromContext(c)
	info, err := b.catalog.Quote(c.Request.Context(), app.App.OwnerUserID, req)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, info)
}

// CreateOrder POST /open/v1/orders（代客下单，T6.3）
func (b *Bundle) CreateOrder(c *gin.Context) {
	var req dto.OpenOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(service.CodeOpenParam, err.Error()))
		return
	}
	// 幂等键从请求头注入（doc16 §8.5）。
	req.ClientRequestID = c.GetHeader(service.HeaderClientRequestID)
	app := service.AppFromContext(c)
	result, err := b.order.Create(c.Request.Context(), app, req)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, result)
}

// ListInstances GET /open/v1/instances（T6.4）
func (b *Bundle) ListInstances(c *gin.Context) {
	app := service.AppFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := b.instance.List(c.Request.Context(), app.App.OwnerUserID, c.Query("status"), page, pageSize)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, resp)
}

// GetInstance GET /open/v1/instances/:id
func (b *Bundle) GetInstance(c *gin.Context) {
	app := service.AppFromContext(c)
	id := parseUint64(c.Param("id"))
	if id == 0 {
		response.Error(c, apperrors.New(service.CodeOpenParam, "id 非法"))
		return
	}
	item, err := b.instance.Get(c.Request.Context(), app.App.OwnerUserID, id)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, item)
}

// RenewInstance POST /open/v1/instances/:id/renew
func (b *Bundle) RenewInstance(c *gin.Context) {
	var req dto.RenewInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(service.CodeOpenParam, err.Error()))
		return
	}
	app := service.AppFromContext(c)
	resp, err := b.instance.Renew(c.Request.Context(), app, parseUint64(c.Param("id")), req)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, resp)
}

// PowerInstance POST /open/v1/instances/:id/power
func (b *Bundle) PowerInstance(c *gin.Context) {
	var req dto.PowerInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(service.CodeOpenParam, err.Error()))
		return
	}
	app := service.AppFromContext(c)
	if err := b.instance.Power(c.Request.Context(), app, parseUint64(c.Param("id")), req.Action); err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.SuccessMessage(c, "电源操作已执行")
}

// SuspendInstance POST /open/v1/instances/:id/suspend
func (b *Bundle) SuspendInstance(c *gin.Context) {
	var req dto.SuspendInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = dto.SuspendInstanceRequest{}
	}
	app := service.AppFromContext(c)
	if err := b.instance.Suspend(c.Request.Context(), app, parseUint64(c.Param("id")), req.Reason); err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.SuccessMessage(c, "实例已暂停")
}

// UnsuspendInstance POST /open/v1/instances/:id/unsuspend
func (b *Bundle) UnsuspendInstance(c *gin.Context) {
	app := service.AppFromContext(c)
	if err := b.instance.Unsuspend(c.Request.Context(), app, parseUint64(c.Param("id"))); err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.SuccessMessage(c, "实例已恢复")
}

// DeleteInstance DELETE /open/v1/instances/:id —— D2 安全边界：显式注册并固定拒绝，
// 让下游得到明确的「不支持」而不是 404/权限错误（验收专门断言本响应）。
func (b *Bundle) DeleteInstance(c *gin.Context) {
	_ = b.instance // 网关鉴权后到达；响应与实例服务无关，恒为不支持
	response.Error(c, apperrors.New(service.CodeOpenUnsupported,
		"销毁不支持：实例销毁仅限平台侧人工操作"))
}

// ListAuditRequests GET /open/v1/audit/requests（T6.6）
func (b *Bundle) ListAuditRequests(c *gin.Context) {
	app := service.AppFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pageResp, items, err := b.audit.ListRequests(c.Request.Context(), app.App.ID, page, pageSize)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, gin.H{"total": pageResp.Total, "page": pageResp.Page, "page_size": pageResp.PageSize, "items": items})
}

// ListAuditOrders GET /open/v1/audit/orders（T6.6）
func (b *Bundle) ListAuditOrders(c *gin.Context) {
	app := service.AppFromContext(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pageResp, items, err := b.audit.ListOrders(c.Request.Context(), app.App.ID, page, pageSize)
	if err != nil {
		response.Error(c, toAppError(err))
		return
	}
	response.Success(c, gin.H{"total": pageResp.Total, "page": pageResp.Page, "page_size": pageResp.PageSize, "items": items})
}

// toAppError 业务错误透传 AppError，其余按内部错误。
func toAppError(err error) *apperrors.AppError {
	if ae, ok := err.(*apperrors.AppError); ok {
		return ae
	}
	return apperrors.New(service.CodeOpenInternal, err.Error())
}

func parseUint64(raw string) uint64 {
	v, _ := strconv.ParseUint(raw, 10, 64)
	return v
}
