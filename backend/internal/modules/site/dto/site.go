// Package dto 定义官网门户公开接口的出入参（脱敏，仅暴露展示所需字段）。
package dto

// AnnouncementItem 公开公告条目。
//
// 脱敏说明：不返回 operator_id、status、platform、offline_at 等内部字段，
// 仅保留展示所需内容。查询侧已强制 status=published 且 offline_at IS NULL，
// 不会泄漏草稿与已下线公告。
type AnnouncementItem struct {
	ID        uint64 `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"`      // info / warning / critical
	Popup     bool   `json:"popup"`      // 是否弹窗强提醒
	PublishAt string `json:"publish_at"` // 2006-01-02 15:04:05，无则为空串
}

// AnnouncementListResponse 公开公告列表响应。
type AnnouncementListResponse struct {
	Items []AnnouncementItem `json:"items"`
}

// SiteContentResponse 站点品牌配置响应。
//
// Items 是「扁平键 → 值」的白名单映射，键形如 `site.name` / `home.hero_title`（点号命名）
// 或管理端历史扁平键 `site_name`。刻意不做结构化成嵌套对象：键的 schema、默认值与校验
// 是前端 frontend-site/shared/schemas/siteContent.ts 的单一真相，服务端只负责按白名单取值，
// 避免同一份配置定义在前后端各维护一遍而漂移。
type SiteContentResponse struct {
	Items map[string]string `json:"items"`
}
