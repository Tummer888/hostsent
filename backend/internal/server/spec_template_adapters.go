package server

import (
	"context"

	"hostsent/backend/internal/modules/admin/product/catalog/service"
	specdto "hostsent/backend/internal/modules/admin/product/spec/dto"
	specservice "hostsent/backend/internal/modules/admin/product/spec/service"
)

// 自营链路打通（规格模板 → SKU → 平台绑定）的装配适配层。
//
// catalog 服务不认识 spec 子域的模型：这里把 spec 模板服务与规格契约服务
// 适配成 catalog 侧声明的最小接口，避免两个子域互相依赖（与 price_matrix_adapters 同法）。

// specTemplateReaderAdapter 把 spec 的模板服务适配为 catalog 的 SpecTemplateReader。
type specTemplateReaderAdapter struct {
	templates specservice.SpecTemplateService
}

// NewSpecTemplateReader 构造 SpecTemplateReader（装配层调用）。
func NewSpecTemplateReader(templates specservice.SpecTemplateService) service.SpecTemplateReader {
	return &specTemplateReaderAdapter{templates: templates}
}

func (a *specTemplateReaderAdapter) SpecTemplateByID(ctx context.Context, id uint64) (*service.SpecTemplateSnapshot, error) {
	item, err := a.templates.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return &service.SpecTemplateSnapshot{
		ID:             item.ID,
		Name:           item.Name,
		SpecFamily:     item.SpecFamily,
		CPU:            item.CPU,
		Memory:         item.Memory,
		Disk:           item.Disk,
		Bandwidth:      item.Bandwidth,
		DiskType:       item.DiskType,
		OS:             item.OS,
		Price:          item.Price,
		SpecValues:     rawJSONString(item.SpecValues),
		PlatformParams: rawJSONString(item.PlatformParams),
	}, nil
}

// specBindingWriterAdapter 把规格契约服务适配为 catalog 的 SpecBindingWriter。
type specBindingWriterAdapter struct {
	contracts specservice.SpecContractService
}

// NewSpecBindingWriter 构造 SpecBindingWriter（装配层调用）。
func NewSpecBindingWriter(contracts specservice.SpecContractService) service.SpecBindingWriter {
	return &specBindingWriterAdapter{contracts: contracts}
}

// UpsertConfirmedProductSpecBinding 为 SKU 幂等写入出站绑定并置 confirmed（模板即人工确认过的映射）。
func (a *specBindingWriterAdapter) UpsertConfirmedProductSpecBinding(
	ctx context.Context, productSpecID, specTemplateID uint64, platformParams string, operatorID uint64, remark string,
) error {
	info, err := a.contracts.UpsertBinding(ctx, specdto.SpecBindingUpsertRequest{
		ProductSpecID:  productSpecID,
		SpecTemplateID: specTemplateID,
		Direction:      "outbound",
		PlatformParams: []byte(platformParams),
		MatchType:      "manual",
		Status:         "confirmed",
		Confidence:     100,
		Remark:         remark,
	})
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}
	// 复核确认，确保确认人/时间留痕（UpsertBinding 已置 confirmed，这里补齐审计字段）。
	_, err = a.contracts.ConfirmBinding(ctx, info.ID, specdto.SpecBindingConfirmRequest{Remark: remark}, operatorID)
	return err
}

// rawJSONString 把 DTO 的 json.RawMessage 转成字符串；空/null 返回空串。
func rawJSONString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if s == "null" {
		return ""
	}
	return s
}
