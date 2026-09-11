import { request } from '@/utils/request'

import type {
  SaleProductCategoryCreateRequest,
  SaleProductCategoryInfo,
  SaleProductCategoryListResponse,
  SaleProductBatchCloneRequest,
  SaleProductCategoryUpdateRequest,
  SaleProductCloneRequest,
  SaleProductCreateRequest,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductListQuery,
  SaleProductListResponse,
  SaleProductPriceRequest,
  SaleProductSpecInfo,
  SaleProductUpdateRequest,
  SpecMappingBindRequest,
  SpecMappingInfo,
  SpecMappingListResponse,
  SpecMappingQuery,
  SpecMappingRequest,
  SpecTemplateInfo,
  SpecTemplateListResponse,
  SpecTemplateQuery,
  SpecTemplateRequest,
  PricingInfo,
  PricingListResponse,
  PricingQuery,
  PricingRequest,
  PricePolicyInfo,
  PricePolicyListResponse,
  PricePolicyQuery,
  PricePolicyRequest,
  CouponInfo,
  CouponListResponse,
  CouponQuery,
  CouponRequest,
  CouponGrantCreateRequest,
  CouponGrantListResponse,
  CouponGrantQuery,
  PromotionInfo,
  PromotionListResponse,
  PromotionQuery,
  PromotionRequest,
} from '@/types/interface'

// ===== 产品管理 - 分类 =====

export function getProductCategoryList(status?: number): Promise<SaleProductCategoryListResponse> {
  return request.get<SaleProductCategoryListResponse>({
    url: '/product/categories',
    params: { status },
  })
}

export function getProductCategoryDetail(id: number): Promise<SaleProductCategoryInfo> {
  return request.get<SaleProductCategoryInfo>({
    url: `/product/categories/${id}`,
  })
}

export function createProductCategory(data: SaleProductCategoryCreateRequest): Promise<SaleProductCategoryInfo> {
  return request.post<SaleProductCategoryInfo>({
    url: '/product/categories',
    data,
  })
}

export function updateProductCategory(id: number, data: SaleProductCategoryUpdateRequest): Promise<SaleProductCategoryInfo> {
  return request.put<SaleProductCategoryInfo>({
    url: `/product/categories/${id}`,
    data,
  })
}

export function deleteProductCategory(id: number): Promise<string> {
  return request.delete<string>({
    url: `/product/categories/${id}`,
  })
}

// ===== 产品管理 - 产品 =====

export function getProductList(params: SaleProductListQuery): Promise<SaleProductListResponse> {
  return request.get<SaleProductListResponse>({
    url: '/product/products',
    params: {
      keyword: params.keyword,
      category_id: params.category_id,
      status: params.status,
      provision_mode: params.provision_mode,
      source_mode: params.source_mode,
      page: params.page,
      page_size: params.page_size,
    },
  })
}

export function cloneProductFromUpstream(data: SaleProductCloneRequest): Promise<SaleProductInfo> {
  return request.post<SaleProductInfo>({
    url: '/product/products/clone',
    data,
  })
}

export function batchCloneProductFromUpstream(data: SaleProductBatchCloneRequest): Promise<SaleProductInfo[]> {
  return request.post<SaleProductInfo[]>({
    url: '/product/products/clone/batch',
    data,
  })
}

export function getProductDetail(id: number): Promise<SaleProductInfo> {
  return request.get<SaleProductInfo>({
    url: `/product/products/${id}`,
  })
}

export function createProduct(data: SaleProductCreateRequest): Promise<SaleProductInfo> {
  return request.post<SaleProductInfo>({
    url: '/product/products',
    data,
  })
}

export function updateProduct(id: number, data: SaleProductUpdateRequest): Promise<SaleProductInfo> {
  return request.put<SaleProductInfo>({
    url: `/product/products/${id}`,
    data,
  })
}

export function deleteProduct(id: number): Promise<string> {
  return request.delete<string>({
    url: `/product/products/${id}`,
  })
}

export function publishProduct(id: number): Promise<SaleProductInfo> {
  return request.post<SaleProductInfo>({
    url: `/product/products/${id}/publish`,
  })
}

export function unpublishProduct(id: number): Promise<SaleProductInfo> {
  return request.post<SaleProductInfo>({
    url: `/product/products/${id}/unpublish`,
  })
}

export function updateProductPrice(id: number, data: SaleProductPriceRequest): Promise<SaleProductInfo> {
  return request.put<SaleProductInfo>({
    url: `/product/products/${id}/price`,
    data,
  })
}

export function getProductHistory(id: number): Promise<SaleProductHistoryInfo[]> {
  return request.get<SaleProductHistoryInfo[]>({
    url: `/product/products/${id}/history`,
  })
}

export function getProductSpecs(id: number): Promise<SaleProductSpecInfo[]> {
  return request.get<SaleProductSpecInfo[]>({
    url: `/product/products/${id}/specs`,
  })
}

export function setProductFeatured(id: number, featured: boolean): Promise<SaleProductInfo> {
  return request.post<SaleProductInfo>({
    url: `/product/products/${id}/featured`,
    data: { featured },
  })
}

// ===== 产品管理 - 规格模板 =====

export function getSpecTemplateList(params: SpecTemplateQuery): Promise<SpecTemplateListResponse> {
  return request.get<SpecTemplateListResponse>({
    url: '/product/spec/templates',
    params,
  })
}

export function getSpecTemplateDetail(id: number): Promise<SpecTemplateInfo> {
  return request.get<SpecTemplateInfo>({ url: `/product/spec/templates/${id}` })
}

export function createSpecTemplate(data: SpecTemplateRequest): Promise<SpecTemplateInfo> {
  return request.post<SpecTemplateInfo>({ url: '/product/spec/templates', data })
}

export function updateSpecTemplate(id: number, data: SpecTemplateRequest): Promise<SpecTemplateInfo> {
  return request.put<SpecTemplateInfo>({ url: `/product/spec/templates/${id}`, data })
}

export function deleteSpecTemplate(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/spec/templates/${id}` })
}

// ===== 产品管理 - 规格映射 =====

export function getSpecMappingList(params: SpecMappingQuery): Promise<SpecMappingListResponse> {
  return request.get<SpecMappingListResponse>({
    url: '/product/spec/mappings',
    params,
  })
}

export function getSpecMappingDetail(id: number): Promise<SpecMappingInfo> {
  return request.get<SpecMappingInfo>({ url: `/product/spec/mappings/${id}` })
}

export function createSpecMapping(data: SpecMappingRequest): Promise<SpecMappingInfo> {
  return request.post<SpecMappingInfo>({ url: '/product/spec/mappings', data })
}

export function updateSpecMapping(id: number, data: SpecMappingRequest): Promise<SpecMappingInfo> {
  return request.put<SpecMappingInfo>({ url: `/product/spec/mappings/${id}`, data })
}

export function bindSpecMapping(id: number, data: SpecMappingBindRequest): Promise<SpecMappingInfo> {
  return request.post<SpecMappingInfo>({ url: `/product/spec/mappings/${id}/bind`, data })
}

export function deleteSpecMapping(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/spec/mappings/${id}` })
}

// ===== 产品管理 - 价格策略 =====

export function getPricingList(params: PricingQuery): Promise<PricingListResponse> {
  return request.get<PricingListResponse>({
    url: '/product/pricing',
    params,
  })
}

export function getPricingDetail(id: number): Promise<PricingInfo> {
  return request.get<PricingInfo>({ url: `/product/pricing/${id}` })
}

export function createPricing(data: PricingRequest): Promise<PricingInfo> {
  return request.post<PricingInfo>({ url: '/product/pricing', data })
}

export function updatePricing(id: number, data: PricingRequest): Promise<PricingInfo> {
  return request.put<PricingInfo>({ url: `/product/pricing/${id}`, data })
}

export function deletePricing(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/pricing/${id}` })
}

// ===== 产品管理 - 折扣策略（P5 统一算价管线）=====

export function getPricePolicyList(params: PricePolicyQuery): Promise<PricePolicyListResponse> {
  return request.get<PricePolicyListResponse>({ url: '/product/discount-policies', params })
}

export function getPricePolicyDetail(id: number): Promise<PricePolicyInfo> {
  return request.get<PricePolicyInfo>({ url: `/product/discount-policies/${id}` })
}

export function createPricePolicy(data: PricePolicyRequest): Promise<PricePolicyInfo> {
  return request.post<PricePolicyInfo>({ url: '/product/discount-policies', data })
}

export function updatePricePolicy(id: number, data: PricePolicyRequest): Promise<PricePolicyInfo> {
  return request.put<PricePolicyInfo>({ url: `/product/discount-policies/${id}`, data })
}

export function deletePricePolicy(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/discount-policies/${id}` })
}

// ===== 产品管理 - 优惠券 =====

export function getCouponList(params: CouponQuery): Promise<CouponListResponse> {
  return request.get<CouponListResponse>({
    url: '/product/promotion/coupons',
    params,
  })
}

export function getCouponDetail(id: number): Promise<CouponInfo> {
  return request.get<CouponInfo>({ url: `/product/promotion/coupons/${id}` })
}

export function createCoupon(data: CouponRequest): Promise<CouponInfo> {
  return request.post<CouponInfo>({ url: '/product/promotion/coupons', data })
}

export function updateCoupon(id: number, data: CouponRequest): Promise<CouponInfo> {
  return request.put<CouponInfo>({ url: `/product/promotion/coupons/${id}`, data })
}

export function deleteCoupon(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/promotion/coupons/${id}` })
}

// ===== 产品管理 - 优惠券发放记录 =====

export function getCouponGrantList(params: CouponGrantQuery): Promise<CouponGrantListResponse> {
  return request.get<CouponGrantListResponse>({
    url: '/product/promotion/coupon-grants',
    params,
  })
}

export function createCouponGrants(data: CouponGrantCreateRequest): Promise<{ issued: number }> {
  return request.post<{ issued: number }>({ url: '/product/promotion/coupon-grants', data })
}

// ===== 产品管理 - 促销活动 =====

export function getPromotionList(params: PromotionQuery): Promise<PromotionListResponse> {
  return request.get<PromotionListResponse>({
    url: '/product/promotion/promotions',
    params,
  })
}

export function getPromotionDetail(id: number): Promise<PromotionInfo> {
  return request.get<PromotionInfo>({ url: `/product/promotion/promotions/${id}` })
}

export function createPromotion(data: PromotionRequest): Promise<PromotionInfo> {
  return request.post<PromotionInfo>({ url: '/product/promotion/promotions', data })
}

export function updatePromotion(id: number, data: PromotionRequest): Promise<PromotionInfo> {
  return request.put<PromotionInfo>({ url: `/product/promotion/promotions/${id}`, data })
}

export function deletePromotion(id: number): Promise<string> {
  return request.delete<string>({ url: `/product/promotion/promotions/${id}` })
}
