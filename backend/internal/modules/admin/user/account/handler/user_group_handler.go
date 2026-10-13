package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"

	"hostsent/backend/internal/modules/admin/user/account/dto"
	"hostsent/backend/internal/modules/admin/user/account/service"
)

type UserGroupHandler struct {
	service service.UserGroupService
}

func NewUserGroupHandler(service service.UserGroupService) *UserGroupHandler {
	return &UserGroupHandler{service: service}
}

// List godoc
// @Summary 用户组列表
// @Description 获取用户组列表，支持分页、状态筛选和关键词搜索
// @Tags 用户组管理
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "用户组状态，如 active/disabled"
// @Param keyword query string false "关键词，支持名称/编码模糊搜索"
// @Success 200 {object} dto.APIResponse[dto.UserGroupListResponse]
// @Router /api/v1/admin/user-groups [get]
func (h *UserGroupHandler) List(c *gin.Context) {
	var query dto.UserGroupListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	items, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, items)
}

// Get godoc
// @Summary 用户组详情
// @Description 获取单个用户组详情
// @Tags 用户组管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户组ID"
// @Success 200 {object} dto.APIResponse[dto.UserGroupInfo]
// @Router /api/v1/admin/user-groups/{id} [get]
func (h *UserGroupHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, item)
}

// Create godoc
// @Summary 创建用户组
// @Description 创建用户组并设置基础属性
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UserGroupCreateRequest true "用户组参数"
// @Success 200 {object} dto.APIResponse[dto.UserGroupInfo]
// @Router /api/v1/admin/user-groups [post]
func (h *UserGroupHandler) Create(c *gin.Context) {
	var req dto.UserGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, item)
}

// Update godoc
// @Summary 更新用户组
// @Description 更新用户组基础信息
// @Tags 用户组管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户组ID"
// @Param request body dto.UserGroupUpdateRequest true "用户组参数"
// @Success 200 {object} dto.APIResponse[dto.UserGroupInfo]
// @Router /api/v1/admin/user-groups/{id} [put]
func (h *UserGroupHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UserGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, http.StatusBadRequest, apperrors.New(20001, err.Error()))
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, item)
}

// Delete godoc
// @Summary 删除用户组
// @Description 删除指定用户组
// @Tags 用户组管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户组ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/user-groups/{id} [delete]
func (h *UserGroupHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorWithStatus(c, http.StatusInternalServerError, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, "ok")
}
