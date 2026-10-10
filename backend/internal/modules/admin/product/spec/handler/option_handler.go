package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/spec/dto"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// ===== 平台配置项目录与取值库（T4.5 规格配置化）=====

// ListOptionCatalog godoc
// @Summary 查询平台配置项目录（含取值库中的可选值）
// @Description 目录 = 适配器声明 ∪ 数据库覆盖（DB 优先）。未同步过时直接回适配器声明。
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param provider_type query string false "平台类型（mofangyun），与 provider_id 二选一"
// @Param provider_id query int false "渠道 ID（用于反查平台类型，并补拉平台实时取值）"
// @Param include_hidden query bool false "是否包含已隐藏的配置项"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-catalog [get]
func (h *SpecHandler) ListOptionCatalog(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var query dto.OptionCatalogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	includeHidden := c.Query("include_hidden") == "true"
	resp, err := h.optionService.ListCatalog(c.Request.Context(), query, includeHidden)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// SyncOptionCatalog godoc
// @Summary 把适配器声明的配置项目录幂等导入数据库
// @Description 只补缺失项，不覆盖运营改过的标签/默认值/必选标记。
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.OptionCatalogSyncRequest true "平台类型"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-catalog/sync [post]
func (h *SpecHandler) SyncOptionCatalog(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var req dto.OptionCatalogSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.SyncCatalog(c.Request.Context(), req.ProviderType)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateOptionSpec godoc
// @Summary 新增自定义平台配置项
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.OptionSpecRequest true "配置项"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-catalog [post]
func (h *SpecHandler) CreateOptionSpec(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var req dto.OptionSpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.CreateSpec(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateOptionSpec godoc
// @Summary 修改平台配置项（标签/默认值/必选/控件/枚举/排序/隐藏）
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param id path int true "配置项 ID"
// @Param request body dto.OptionSpecRequest true "配置项"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-catalog/{id} [put]
func (h *SpecHandler) UpdateOptionSpec(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.OptionSpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.UpdateSpec(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeleteOptionSpec godoc
// @Summary 删除平台配置项（连同其取值）
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param id path int true "配置项 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-catalog/{id} [delete]
func (h *SpecHandler) DeleteOptionSpec(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.optionService.DeleteSpec(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// ListOptionValues godoc
// @Summary 查询平台取值库（某配置项允许的取值）
// @Description 库为空且给了 provider_id 时，会自动从平台实时拉一次该配置项的取值。
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param provider_type query string false "平台类型"
// @Param provider_id query int false "渠道 ID（拉取平台实时取值用）"
// @Param option_key query string false "配置项参数名（os/area/cpu…）"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-values [get]
func (h *SpecHandler) ListOptionValues(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var query dto.OptionValueQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.ListValues(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpsertOptionValue godoc
// @Summary 新增/更新一条平台取值
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.OptionValueUpsertRequest true "取值"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-values [post]
func (h *SpecHandler) UpsertOptionValue(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var req dto.OptionValueUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.UpsertValue(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// ImportOptionValues godoc
// @Summary 批量导入平台取值（镜像类型直接入库入口）
// @Description replace=true 时先停用该配置项下平台来源的旧取值，再灌入新的一批。
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.OptionValueImportRequest true "取值列表"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-values/import [post]
func (h *SpecHandler) ImportOptionValues(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var req dto.OptionValueImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.ImportValues(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// RefreshOptionValues godoc
// @Summary 从平台实时刷新取值库（镜像/区域/节点/存储）
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param request body dto.OptionValueRefreshRequest true "刷新参数"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-values/refresh [post]
func (h *SpecHandler) RefreshOptionValues(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	var req dto.OptionValueRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.optionService.RefreshFromPlatform(c.Request.Context(), req.ProviderType, req.ProviderID, req.OptionKeys)
	if err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeleteOptionValue godoc
// @Summary 删除一条平台取值
// @Tags 产品管理-规格
// @Security BearerAuth
// @Param id path int true "取值 ID"
// @Success 200 {object} response.Body
// @Router /api/v1/admin/product/spec/option-values/{id} [delete]
func (h *SpecHandler) DeleteOptionValue(c *gin.Context) {
	if h.optionService == nil {
		response.Error(c, apperrors.New(50001, "平台配置项服务未装配"))
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.optionService.DeleteValue(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}
