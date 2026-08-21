// Package handler 提供菜单模块的 HTTP 接口。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/menu/dto"
	"hostsent/backend/internal/modules/menu/service"
)

type MenuHandler struct {
	menuService service.MenuService
}

// NewMenuHandler 创建菜单接口处理器。
func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// Tree godoc
// @Summary 菜单树
// @Description 按平台获取管理员后台或用户中心菜单树
// @Tags 菜单管理
// @Produce json
// @Param platform query string false "平台：admin(默认) 或 user"
// @Success 200 {object} dto.APIResponse[[]dto.MenuNode]
// @Router /api/v1/menus/tree [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	platform := c.DefaultQuery("platform", "admin")
	tree, err := h.menuService.Tree(c.Request.Context(), platform)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tree, "timestamp": time.Now().Unix()})
}

// CreateMenu godoc
// @Summary 创建菜单节点
// @Tags 菜单管理
// @Accept json
// @Produce json
// @Param request body dto.MenuCreateRequest true "菜单参数"
// @Success 200 {object} dto.APIResponse[dto.MenuNode]
// @Router /api/v1/menus [post]
func (h *MenuHandler) CreateMenu(c *gin.Context) {
	var req dto.MenuCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	menu, err := h.menuService.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": menu, "timestamp": time.Now().Unix()})
}

// UpdateMenu godoc
// @Summary 更新菜单节点
// @Tags 菜单管理
// @Accept json
// @Produce json
// @Param id path int true "菜单ID"
// @Param request body dto.MenuUpdateRequest true "菜单参数"
// @Success 200 {object} dto.APIResponse[dto.MenuNode]
// @Router /api/v1/menus/{id} [put]
func (h *MenuHandler) UpdateMenu(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.MenuUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	menu, err := h.menuService.Update(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "菜单不存在", "timestamp": time.Now().Unix()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": menu, "timestamp": time.Now().Unix()})
}

// DeleteMenu godoc
// @Summary 删除菜单节点
// @Description 递归删除菜单及其全部子节点
// @Tags 菜单管理
// @Produce json
// @Param id path int true "菜单ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/menus/{id} [delete]
func (h *MenuHandler) DeleteMenu(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.menuService.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "菜单不存在", "timestamp": time.Now().Unix()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": err.Error(), "timestamp": time.Now().Unix()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": "ok", "timestamp": time.Now().Unix()})
}
