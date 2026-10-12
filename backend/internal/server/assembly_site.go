// 门户站点域装配（批次四 4b）。

package server

import (
	sitehandler "hostsent/backend/internal/modules/site/handler"
	siteservice "hostsent/backend/internal/modules/site/service"
)

// buildSite 构造门户公开只读服务与处理器。
func (a *asm) buildSite() {
	// 官网门户公开只读数据（公告 + 内容中心），无需登录；site-content 也落在该模块。
	// 第二个参数是公开读取适配器：site 只声明需要的读方法，装配层注入实现。
	a.siteSvc = siteservice.NewSiteService(a.announceSvc, a.contentBundle.reader, a.configRepo)
	a.siteHandler = sitehandler.NewSiteHandler(a.siteSvc)
}
