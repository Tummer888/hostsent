// Package handler 提供代理等级模块的 HTTP 接口。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/user/agentlevel/dto"
	agentlevelrepo "hostsent/backend/internal/modules/admin/user/agentlevel/repository"
	"hostsent/backend/internal/modules/admin/user/agentlevel/service"
)

// AgentLevelHandler 提供代理等级相关的 HTTP 接口。
type AgentLevelHandler struct{ service service.AgentLevelService }

// NewAgentLevelHandler 创建代理等级处理器。
func NewAgentLevelHandler(svc service.AgentLevelService) *AgentLevelHandler {
	return &AgentLevelHandler{service: svc}
}

// List godoc
// @Summary 代理等级列表
// @Tags 代理等级
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "状态"
// @Param keyword query string false "关键词"
// @Success 200 {object} dto.APIResponse[dto.ListResponse]
// @Router /api/v1/admin/agent-levels [get]
func (h *AgentLevelHandler) List(c *gin.Context) {
	var query dto.ListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Get godoc
// @Summary 代理等级详情（含折扣矩阵明细）
// @Tags 代理等级
// @Produce json
// @Param id path int true "代理等级ID"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/agent-levels/{id} [get]
func (h *AgentLevelHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	data, err := h.service.FindByID(c.Request.Context(), id)
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// Create godoc
// @Summary 创建代理等级
// @Tags 代理等级
// @Accept json
// @Produce json
// @Param request body dto.CreateRequest true "代理等级参数"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/agent-levels [post]
func (h *AgentLevelHandler) Create(c *gin.Context) {
	var req dto.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// Update godoc
// @Summary 更新代理等级
// @Tags 代理等级
// @Accept json
// @Produce json
// @Param id path int true "代理等级ID"
// @Param request body dto.UpdateRequest true "代理等级参数"
// @Success 200 {object} dto.APIResponse[dto.Info]
// @Router /api/v1/admin/agent-levels/{id} [put]
func (h *AgentLevelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// Delete godoc
// @Summary 删除代理等级
// @Tags 代理等级
// @Produce json
// @Param id path int true "代理等级ID"
// @Success 200 {object} dto.APIResponse[string]
// @Router /api/v1/admin/agent-levels/{id} [delete]
func (h *AgentLevelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, "ok")
}

// UpdateCell godoc
// @Summary 更新矩阵单格折扣
// @Tags 代理等级
// @Accept json
// @Produce json
// @Param request body dto.CellUpdateRequest true "单格参数（折扣率 0 = 清除）"
// @Success 200 {object} dto.APIResponse[dto.MatrixResponse]
// @Router /api/v1/admin/agent-levels/discount [put]
func (h *AgentLevelHandler) UpdateCell(c *gin.Context) {
	var req dto.CellUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.UpdateCell(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// Matrix godoc
// @Summary 代理折扣矩阵（列为等级、行为目标）
// @Tags 代理等级
// @Produce json
// @Success 200 {object} dto.APIResponse[dto.MatrixResponse]
// @Router /api/v1/admin/agent-levels/matrix [get]
func (h *AgentLevelHandler) Matrix(c *gin.Context) {
	data, err := h.service.Matrix(c.Request.Context())
	if err != nil {
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// PreviewLadder godoc
// @Summary 阶梯填充预览（锚点 + 步长 → 各等级折扣率）
// @Tags 代理等级
// @Accept json
// @Produce json
// @Param request body dto.LadderPreviewRequest true "阶梯参数"
// @Success 200 {object} dto.APIResponse[dto.LadderPreviewResponse]
// @Router /api/v1/admin/agent-levels/ladder/preview [post]
func (h *AgentLevelHandler) PreviewLadder(c *gin.Context) {
	var req dto.LadderPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.PreviewLadder(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// ApplyLadder godoc
// @Summary 应用阶梯填充（写入某目标下的所有等级）
// @Tags 代理等级
// @Accept json
// @Produce json
// @Param request body dto.ApplyLadderRequest true "阶梯参数"
// @Success 200 {object} dto.APIResponse[dto.MatrixResponse]
// @Router /api/v1/admin/agent-levels/ladder/apply [post]
func (h *AgentLevelHandler) ApplyLadder(c *gin.Context) {
	var req dto.ApplyLadderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.ApplyLadder(c.Request.Context(), req)
	if err != nil {
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// —— 代理分组成员（归属管理）——
//
// 「代理分组」= 代理等级（doc108）；归属即 users.agent_level_id。这一组接口
// 让运营在同一个面板里显式管理「哪个代理属于哪个分组」，而不是逐个用户去详情页改。

// ListMembers godoc
// @Summary 代理分组成员列表（?unassigned=true 列未归属账号）
// @Tags 代理分组
// @Produce json
// @Param id path int true "代理分组ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param keyword query string false "关键词"
// @Param unassigned query bool false "仅列未归属任何分组的账号（忽略 id）"
// @Success 200 {object} dto.APIResponse[dto.MemberListResponse]
// @Router /api/v1/admin/agent-levels/{id}/members [get]
func (h *AgentLevelHandler) ListMembers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var query dto.MemberListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		badRequest(c, err.Error())
		return
	}
	data, err := h.service.ListMembers(c.Request.Context(), id, query)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "代理分组不存在", "timestamp": time.Now().Unix()})
			return
		}
		serverError(c, err.Error())
		return
	}
	success(c, data)
}

// AssignMembers godoc
// @Summary 批量纳入本代理分组（可含从其它分组转入）
// @Tags 代理分组
// @Accept json
// @Produce json
// @Param id path int true "代理分组ID"
// @Param request body dto.MemberAssignRequest true "成员参数"
// @Success 200 {object} dto.APIResponse[dto.MemberAssignResponse]
// @Router /api/v1/admin/agent-levels/{id}/members [post]
func (h *AgentLevelHandler) AssignMembers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.MemberAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	// action=remove 复用同一个入口：调用方按语义传参，服务层按语义落到不同的
	// 仓储动作（置为 NULL 而不是再建一条"移出"路径）。
	var (
		data *dto.MemberAssignResponse
		err  error
	)
	if req.Action == dto.MemberActionRemove {
		data, err = h.service.RemoveMembers(c.Request.Context(), id, req.UserIDs)
	} else {
		data, err = h.service.AssignMembers(c.Request.Context(), id, req.UserIDs)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "代理分组不存在", "timestamp": time.Now().Unix()})
			return
		}
		writeBusinessError(c, err)
		return
	}
	success(c, data)
}

// writeBusinessError 把业务错误映射成 409（可自解状态），其余回落 500。
//
// 折扣率低于成本、单调性冲突这类都是"运营能自己改对"的状态冲突，回落 500
// 只会让前端提示"服务器错误"，运营不知道下一步做什么。
func writeBusinessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 等级不存在（单格更新时传了已删除的等级 ID）：404 而不是 500。
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "代理等级不存在", "timestamp": time.Now().Unix()})
	case errors.Is(err, service.ErrRateBelowCost),
		errors.Is(err, service.ErrRateOverOne),
		errors.Is(err, service.ErrNotMonotonic),
		errors.Is(err, agentlevelrepo.ErrLevelInUse),
		errors.Is(err, agentlevelrepo.ErrTargetNotFound),
		errors.Is(err, agentlevelrepo.ErrCodeTaken):
		c.JSON(http.StatusConflict, gin.H{"code": 40901, "message": err.Error(), "timestamp": time.Now().Unix()})
	default:
		serverError(c, err.Error())
	}
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": 20001, "message": message, "timestamp": time.Now().Unix()})
}

func serverError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": message, "timestamp": time.Now().Unix()})
}

func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data, "timestamp": time.Now().Unix()})
}
