// Package service 提供官网门户（frontend-site）公开只读数据的业务编排。
//
// 与 uc / admin 的分界：
//   - uc 服务「已登录用户的控制台」，接口一律带用户态；
//   - admin 服务「运营管理台」；
//   - 本模块只服务「未登录的公网访客与搜索引擎」，输出必须可共享缓存、字段必须脱敏。
//
// 因此它不挂在 uc 之下：三端后端归属清晰，避免把公开接口混进用户态模块。
package service

import (
	"context"
	"sort"
	"strings"

	notifydto "hostsent/backend/internal/modules/admin/notification/dto"
	notifymodel "hostsent/backend/internal/modules/admin/notification/model"
	systemmodel "hostsent/backend/internal/modules/admin/system/model"

	"hostsent/backend/internal/modules/site/dto"
)

const (
	// announcementDefaultLimit 未指定 limit 时返回的公告条数。
	announcementDefaultLimit = 5
	// announcementMaxLimit 单次请求上限，避免公开接口被当成翻全量数据的入口。
	announcementMaxLimit = 50
)

// announcementReader 公告读取能力的最小暴露接口（由装配层注入，避免 site 依赖 admin service 实现）。
type announcementReader interface {
	ListPublished(ctx context.Context, platform string) ([]notifydto.AnnouncementInfo, error)
}

// configReader 站点配置读取能力（system_configs 表，按分组取）。
type configReader interface {
	ListByGroup(ctx context.Context, group string) ([]systemmodel.SystemConfig, error)
}

// SiteService 官网门户公开数据能力。
type SiteService interface {
	// ListAnnouncements 已发布公告，按发布时间倒序，按 limit 截断。
	ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error)
	// SiteContent 站点品牌配置（白名单键，扁平 map，仅返回已启用项且非空值）。
	SiteContent(ctx context.Context) (dto.SiteContentResponse, error)
}

type siteService struct {
	announcements announcementReader
	configs       configReader
}

// NewSiteService 创建官网门户公开数据服务；configs 为 nil 时站点配置返回空 map（前端回落默认值）。
func NewSiteService(announcements announcementReader, configs configReader) SiteService {
	return &siteService{announcements: announcements, configs: configs}
}

// publicSiteKeys 站点配置对外白名单。
//
// 硬规则（doc80 §5.3）：公开接口逐键返回，禁止整表序列化；带 internal. 前缀的键永不外露。
// 同时覆盖两套键名：管理端系统配置页写入的是历史扁平键（site_name，group=base），
// doc80 规范键是点号命名（site.name，group=site）。前端 BFF 的 fromFlatConfig 只认点号键，
// 因此公开接口把两套键都原样返回，由前端按 schema 取用，服务端不做语义改名。
var publicSiteKeys = []string{
	// doc80 §5.3 规范键（site 分组，点号命名）
	"site.name",
	"site.slogan",
	"site.logo",
	"site.favicon",
	"site.icp",
	"site.contact_phone",
	"site.contact_email",
	"site.copyright",
	"theme.primary_color",
	"theme.radius",
	"home.hero_title",
	"home.hero_subtitle",
	"home.hero_image",
	"home.hero_primary_cta",
	"home.hero_primary_link",
	"home.hero_secondary_cta",
	"home.hero_secondary_link",
	"home.features_title",
	"home.features",
	"home.cta_title",
	"home.cta_desc",
	"home.featured_title",
	"home.featured_limit",
	"home.announce_title",
	"home.announce_limit",
	// 站点法务与联系方式（页脚使用，doc80 未列出但同属公开品牌信息）
	"site.license_no",      // 增值电信业务经营许可证号
	"site.license_org",     // 代理域名注册服务机构
	"site.public_security", // 公网安备号
	"site.contact_address", // 联系地址
	"site.wechat",          // 公众号名称
	// 管理端系统配置页当前写入的历史扁平键（group=base）
	"site_name",
	"site_slogan",
	"site_logo",
	"site_favicon",
	"site_icp",
	"site_copyright",
	"site_license_no",
	"site_license_org",
	"site_public_security",
	"contact_phone",
	"contact_email",
	"contact_address",
	"contact_wechat",
}

// publicSiteKeySet 白名单集合，供 O(1) 判定。
var publicSiteKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(publicSiteKeys))
	for _, key := range publicSiteKeys {
		set[key] = struct{}{}
	}
	return set
}()

// SiteContent 返回站点品牌配置。
//
// 读取合并两个分组（config_group='site' 与历史 'base'），按白名单逐键过滤：
//   - 只返回 status=active 且值非空的项（前端对缺失键回落代码默认值）；
//   - 同义键（site.name 与 site_name）同时存在时都返回，由前端 fromFlatConfig 按 schema 取用；
//   - 任何读取失败都返回空 map 而不是报错，避免官网白屏。
func (s *siteService) SiteContent(ctx context.Context) (dto.SiteContentResponse, error) {
	out := dto.SiteContentResponse{}
	if s.configs == nil {
		return out, nil
	}

	// 分组不存在时 ListByGroup 返回空切片而非错误，两个分组都读以防只配了一处。
	configs := make([]systemmodel.SystemConfig, 0, 64)
	for _, group := range []string{systemmodel.ConfigGroupSite, systemmodel.ConfigGroupBase} {
		items, err := s.configs.ListByGroup(ctx, group)
		if err != nil {
			continue
		}
		configs = append(configs, items...)
	}

	// 规范化键名：去空白、去首尾点，便于兼容运营手工录入的 " site.name " 之类脏值。
	picked := make(map[string]string, len(configs))
	for _, cfg := range configs {
		if cfg.Status != "" && cfg.Status != systemmodel.StatusActive {
			continue
		}
		key := strings.Trim(strings.TrimSpace(cfg.ConfigKey), ".")
		if key == "" {
			continue
		}
		if _, ok := publicSiteKeySet[key]; !ok {
			continue
		}
		if strings.TrimSpace(cfg.ConfigValue) == "" {
			continue
		}
		picked[key] = cfg.ConfigValue
	}
	if len(picked) == 0 {
		return out, nil
	}

	// 按白名单顺序稳定输出，便于人工比对与缓存。
	keys := make([]string, 0, len(picked))
	for key := range picked {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	out.Items = picked
	return out, nil
}

func (s *siteService) ListAnnouncements(ctx context.Context, limit int) ([]dto.AnnouncementItem, error) {
	if limit <= 0 {
		limit = announcementDefaultLimit
	}
	if limit > announcementMaxLimit {
		limit = announcementMaxLimit
	}

	items, err := s.announcements.ListPublished(ctx, notifymodel.AnnouncementPlatformUser)
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]dto.AnnouncementItem, 0, len(items))
	for _, it := range items {
		out = append(out, dto.AnnouncementItem{
			ID:        it.ID,
			Title:     it.Title,
			Content:   it.Content,
			Level:     it.Level,
			Popup:     it.Popup,
			PublishAt: derefString(it.PublishAt),
		})
	}
	return out, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
