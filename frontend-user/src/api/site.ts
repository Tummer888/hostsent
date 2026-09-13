import request from '@/utils/request'

/**
 * 站点品牌配置（官网 + 控制台共用同一份数据源）。
 *
 * 后端 `GET /api/v1/public/site-content` 是免登录的公开白名单接口，
 * 与管理端「系统配置」页写入的是同一张 `system_configs` 表 —— 运营在后台改官网名称/
 * Logo/版权，官网与控制台同时生效，不需要在两处各维护一份。
 */
export interface SiteContentResponse {
  /** 扁平键值对，形如 `{ "site.name": "...", "site_name": "..." }` */
  items: Record<string, string>
}

export function getSiteContent() {
  return request.get<any, { data: SiteContentResponse }>('/public/site-content')
}
