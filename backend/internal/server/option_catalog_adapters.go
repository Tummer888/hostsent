package server

import (
	"context"

	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	specservice "hostsent/backend/internal/modules/admin/product/spec/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	"hostsent/backend/internal/pkg/upstream"
)

// 平台配置项目录（T4.5）的装配适配层。
//
// spec 子域需要"渠道 ID → 平台类型"与"平台实时资源目录"两项能力，而这两项都在
// resource/provider 子域。按本项目既有的适配器隔离法（同 price_matrix_adapters、
// spec_template_adapters），这里把 ProviderService 适配成 spec 侧声明的最小接口，
// 避免 spec 反向依赖 resource。

// providerTypeReaderAdapter 把渠道服务适配为 spec 的 ProviderTypeByID。
type providerTypeReaderAdapter struct {
	providers providerservice.ProviderService
}

// NewProviderTypeReader 构造渠道类型读取器（装配层调用）。
func NewProviderTypeReader(providers providerservice.ProviderService) *providerTypeReaderAdapter {
	return &providerTypeReaderAdapter{providers: providers}
}

func (a *providerTypeReaderAdapter) ProviderTypeByID(ctx context.Context, id uint64) (string, error) {
	item, err := a.providers.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", nil
	}
	return item.ProviderType, nil
}

// platformResourceReaderAdapter 把渠道服务适配为 spec 的平台资源目录读取器。
type platformResourceReaderAdapter struct {
	providers providerservice.ProviderService
}

// NewPlatformResourceReader 构造平台资源读取器（装配层调用）。
func NewPlatformResourceReader(providers providerservice.ProviderService) *platformResourceReaderAdapter {
	return &platformResourceReaderAdapter{providers: providers}
}

func (a *platformResourceReaderAdapter) PlatformResources(ctx context.Context, providerID uint64) (*upstream.PlatformResources, error) {
	return a.providers.PlatformResources(ctx, providerID)
}

// specOptionCatalogAdapter 把 spec 的配置项目录服务适配为 catalog 的 SpecOptionCatalogReader。
// 两个子域各自声明最小接口，中间靠本适配器翻译快照结构。
type specOptionCatalogAdapter struct {
	options specservice.SpecOptionService
}

// NewSpecOptionCatalogReader 构造配置项目录读取器（装配层调用）。
func NewSpecOptionCatalogReader(options specservice.SpecOptionService) catalogservice.SpecOptionCatalogReader {
	return &specOptionCatalogAdapter{options: options}
}

func (a *specOptionCatalogAdapter) OptionCatalogByProvider(ctx context.Context, providerType string) ([]catalogservice.SpecOptionMeta, error) {
	infos, err := a.options.OptionCatalogByProvider(ctx, providerType)
	if err != nil {
		return nil, err
	}
	out := make([]catalogservice.SpecOptionMeta, 0, len(infos))
	for _, info := range infos {
		meta := catalogservice.SpecOptionMeta{
			OptionKey:    info.OptionKey,
			Label:        info.Label,
			Widget:       info.Widget,
			Required:     info.Required,
			Default:      info.DefaultValue,
			Unit:         info.Unit,
			GroupName:    info.GroupName,
			Help:         info.Help,
			ProviderType: info.ProviderType,
			MinValue:     info.MinValue,
			MaxValue:     info.MaxValue,
			SortOrder:    info.SortOrder,
		}
		for _, v := range info.Values {
			meta.Values = append(meta.Values, catalogservice.SpecOptionValueMeta{
				Value: v.Value, Label: v.Label, GroupLabel: v.GroupLabel,
			})
		}
		out = append(out, meta)
	}
	return out, nil
}
