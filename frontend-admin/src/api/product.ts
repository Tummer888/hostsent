import { request } from '@/utils/request'

import type {
  SaleProductCategoryCreateRequest,
  SaleProductCategoryInfo,
  SaleProductCategoryListResponse,
  SaleProductCategoryUpdateRequest,
  SaleProductCreateRequest,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductListQuery,
  SaleProductListResponse,
  SaleProductPriceRequest,
  SaleProductSpecInfo,
  SaleProductUpdateRequest,
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
      page: params.page,
      page_size: params.page_size,
    },
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
