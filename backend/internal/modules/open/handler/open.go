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

// Bundle 开放平台处理器集合。后续任务（T6.4～T6.6）的业务服务挂在同一结构上。
type Bundle struct {
	gw      *service.Gateway
	catalog *service.CatalogService
	order   *service.OpenOrderService
	logger  *zap.Logger
}

// NewBundle 构造处理器集合（catalog 服务可后置注入）。
func NewBundle(gw *service.Gateway, logger *zap.Logger) *Bundle {
	return &Bundle{gw: gw, logger: logger}
}

// SetCatalog 注入目录服务（T6.2）。
func (b *Bundle) SetCatalog(catalog *service.CatalogService) { b.catalog = catalog }

// SetOrder 注入代客下单服务（T6.3）。
func (b *Bundle) SetOrder(order *service.OpenOrderService) { b.order = order }

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
