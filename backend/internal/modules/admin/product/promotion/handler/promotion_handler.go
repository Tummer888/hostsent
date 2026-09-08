package handler

import (
	"github.com/gin-gonic/gin"

	"hostsent/backend/internal/modules/admin/product/promotion/dto"
	"hostsent/backend/internal/modules/admin/product/promotion/service"
	apperrors "hostsent/backend/internal/pkg/errors"
	"hostsent/backend/internal/pkg/response"
)

// PromotionHandler 促销管理处理入口。
type PromotionHandler struct {
	couponService      service.CouponService
	couponGrantService service.CouponGrantService
	promotionService   service.PromotionService
}

// NewPromotionHandler 创建促销管理处理入口。
func NewPromotionHandler(couponService service.CouponService, couponGrantService service.CouponGrantService, promotionService service.PromotionService) *PromotionHandler {
	return &PromotionHandler{
		couponService:      couponService,
		couponGrantService: couponGrantService,
		promotionService:   promotionService,
	}
}

// ===== 优惠券 =====

// ListCoupons godoc
// @Summary 查询优惠券列表
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param coupon_type query string false "优惠券类型"
// @Param status query int false "状态"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupons [get]
func (h *PromotionHandler) ListCoupons(c *gin.Context) {
	var query dto.CouponQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.couponService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateCoupon godoc
// @Summary 创建优惠券
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param request body dto.CouponRequest true "优惠券参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupons [post]
func (h *PromotionHandler) CreateCoupon(c *gin.Context) {
	var req dto.CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.couponService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetCoupon godoc
// @Summary 查询优惠券详情
// @Tags 产品管理-促销
// @Param id path int true "优惠券 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupons/{id} [get]
func (h *PromotionHandler) GetCoupon(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.couponService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdateCoupon godoc
// @Summary 更新优惠券
// @Tags 产品管理-促销
// @Param id path int true "优惠券 ID"
// @Param request body dto.CouponRequest true "优惠券参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupons/{id} [put]
func (h *PromotionHandler) UpdateCoupon(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.CouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.couponService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeleteCoupon godoc
// @Summary 删除优惠券
// @Tags 产品管理-促销
// @Param id path int true "优惠券 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupons/{id} [delete]
func (h *PromotionHandler) DeleteCoupon(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.couponService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}

// ===== 优惠券发放记录 =====

// ListCouponGrants godoc
// @Summary 查询优惠券发放记录
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param coupon_id query int false "优惠券 ID"
// @Param status query string false "状态"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupon-grants [get]
func (h *PromotionHandler) ListCouponGrants(c *gin.Context) {
	var query dto.CouponGrantQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.couponGrantService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreateCouponGrants godoc
// @Summary 批量发放优惠券
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param request body dto.CouponGrantCreateRequest true "发放参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/coupon-grants [post]
func (h *PromotionHandler) CreateCouponGrants(c *gin.Context) {
	var req dto.CouponGrantCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	count, err := h.couponGrantService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, gin.H{"issued": count})
}

// ===== 促销活动 =====

// ListPromotions godoc
// @Summary 查询促销活动列表
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param keyword query string false "关键字"
// @Param promotion_type query string false "活动类型"
// @Param status query int false "状态"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/promotions [get]
func (h *PromotionHandler) ListPromotions(c *gin.Context) {
	var query dto.PromotionQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.promotionService.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// CreatePromotion godoc
// @Summary 创建促销活动
// @Tags 产品管理-促销
// @Security BearerAuth
// @Param request body dto.PromotionRequest true "活动参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/promotions [post]
func (h *PromotionHandler) CreatePromotion(c *gin.Context) {
	var req dto.PromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.promotionService.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// GetPromotion godoc
// @Summary 查询促销活动详情
// @Tags 产品管理-促销
// @Param id path int true "活动 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/promotions/{id} [get]
func (h *PromotionHandler) GetPromotion(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	resp, err := h.promotionService.FindByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// UpdatePromotion godoc
// @Summary 更新促销活动
// @Tags 产品管理-促销
// @Param id path int true "活动 ID"
// @Param request body dto.PromotionRequest true "活动参数"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/promotions/{id} [put]
func (h *PromotionHandler) UpdatePromotion(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req dto.PromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.New(20001, err.Error()))
		return
	}
	resp, err := h.promotionService.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.Success(c, resp)
}

// DeletePromotion godoc
// @Summary 删除促销活动
// @Tags 产品管理-促销
// @Param id path int true "活动 ID"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/admin/product/promotion/promotions/{id} [delete]
func (h *PromotionHandler) DeletePromotion(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.promotionService.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, apperrors.New(50001, err.Error()))
		return
	}
	response.SuccessMessage(c, "success")
}
