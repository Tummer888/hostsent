// Package handler：商品分组与折扣组的 HTTP 接口（doc108 §8I）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/user/agentlevel/dto"
	"hostsent/backend/internal/modules/admin/user/agentlevel/service"
)

// SchemeHandler 商品分组/折扣组处理器。
type SchemeHandler struct {
	groups  service.ProductGroupService
	schemes service.DiscountSchemeService
}

// NewSchemeHandler 创建商品分组/折扣组处理器。
func NewSchemeHandler(groups service.ProductGroupService, schemes service.DiscountSchemeService) *SchemeHandler {
	return &SchemeHandler{groups: groups, schemes: schemes}
}

// ListGroups godoc
// @Summary 商品分组列表
// @Tags 商品分组
// @Produce json
// @Success 200 {object} dto.APIResponse[dto.ProductGroupListResponse]
// @Router /api/v1/admin/agent-levels/product-groups [get]
func (h *SchemeHandler) ListGroups(c *gin.Context) {
	data, err := h.groups.List(c.Request.Context())
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// CreateGroup godoc
// @Summary 创建商品分组
// @Tags 商品分组
// @Accept json
// @Produce json
// @Param request body dto.ProductGroupRequest true "商品分组参数"
// @Success 200 {object} dto.APIResponse[dto.ProductGroupInfo]
// @Router /api/v1/admin/agent-levels/product-groups [post]
func (h *SchemeHandler) CreateGroup(c *gin.Context) {
	var req dto.ProductGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.groups.Create(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// UpdateGroup godoc
// @Summary 更新商品分组（items 整体覆盖语义）
// @Tags 商品分组
// @Accept json
// @Produce json
// @Param id path int true "商品分组ID"
// @Param request body dto.ProductGroupRequest true "商品分组参数"
// @Success 200 {object} dto.APIResponse[dto.ProductGroupInfo]
// @Router /api/v1/admin/agent-levels/product-groups/{id} [put]
func (h *SchemeHandler) UpdateGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ProductGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.groups.Update(c.Request.Context(), id, req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// DeleteGroup godoc
// @Summary 删除商品分组（被折扣组绑定时拒绝）
// @Tags 商品分组
// @Param id path int true "商品分组ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/agent-levels/product-groups/{id} [delete]
func (h *SchemeHandler) DeleteGroup(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.groups.Delete(c.Request.Context(), id); err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, "ok")
}

// ListSchemes godoc
// @Summary 折扣组列表
// @Tags 折扣组
// @Produce json
// @Success 200 {object} dto.APIResponse[dto.SchemeListResponse]
// @Router /api/v1/admin/agent-levels/discount-schemes [get]
func (h *SchemeHandler) ListSchemes(c *gin.Context) {
	data, err := h.schemes.List(c.Request.Context())
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// CreateScheme godoc
// @Summary 创建折扣组
// @Tags 折扣组
// @Accept json
// @Produce json
// @Param request body dto.SchemeRequest true "折扣组参数"
// @Success 200 {object} dto.APIResponse[dto.SchemeInfo]
// @Router /api/v1/admin/agent-levels/discount-schemes [post]
func (h *SchemeHandler) CreateScheme(c *gin.Context) {
	var req dto.SchemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.schemes.Create(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// UpdateScheme godoc
// @Summary 更新折扣组（items 整体覆盖语义）
// @Tags 折扣组
// @Accept json
// @Produce json
// @Param id path int true "折扣组ID"
// @Param request body dto.SchemeRequest true "折扣组参数"
// @Success 200 {object} dto.APIResponse[dto.SchemeInfo]
// @Router /api/v1/admin/agent-levels/discount-schemes/{id} [put]
func (h *SchemeHandler) UpdateScheme(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.SchemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.schemes.Update(c.Request.Context(), id, req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// DeleteScheme godoc
// @Summary 删除折扣组
// @Tags 折扣组
// @Param id path int true "折扣组ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/agent-levels/discount-schemes/{id} [delete]
func (h *SchemeHandler) DeleteScheme(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.schemes.Delete(c.Request.Context(), id); err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, "ok")
}

// ApplyScheme godoc
// @Summary 应用折扣组：展开写入绑定商品分组下的全部目标
// @Tags 折扣组
// @Produce json
// @Param id path int true "折扣组ID"
// @Success 200 {object} dto.APIResponse[dto.MatrixResponse]
// @Router /api/v1/admin/agent-levels/discount-schemes/{id}/apply [post]
func (h *SchemeHandler) ApplyScheme(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.schemes.Apply(c.Request.Context(), id)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}
