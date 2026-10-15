// 订单域装配——履约链路：上游开通适配器依赖、开通前校验、实例落库与规格解析。

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	ordermodel "hostsent/backend/internal/modules/admin/order/model"
	orderrepo "hostsent/backend/internal/modules/admin/order/repository"
	orderservice "hostsent/backend/internal/modules/admin/order/service"
	catalogservice "hostsent/backend/internal/modules/admin/product/catalog/service"
	providerservice "hostsent/backend/internal/modules/admin/resource/provider/service"
	syncmodel "hostsent/backend/internal/modules/admin/resource/sync/model"
	syncrepo "hostsent/backend/internal/modules/admin/resource/sync/repository"
	openservice "hostsent/backend/internal/modules/open/service"
	"hostsent/backend/internal/pkg/billingcycle"
	"hostsent/backend/internal/pkg/credentials"
	model "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/observability"
	"hostsent/backend/internal/pkg/specatom"
	"hostsent/backend/internal/pkg/upstream"
)

// buildOrderProvisionDeps 组装订单履约的上游开通适配器依赖（打通订单 paid/provisioning → 上游创建实例）。
// events 为开放平台事件发布器（P6/T6.5，可为 nil）：开通成功落库后发布 instance.created。
func buildOrderProvisionDeps(
	catalog catalogservice.ProductService,
	provider providerservice.ProviderService,
	upmgr *upstream.ProviderManager,
	syncRepo syncrepo.SyncRepository,
	events *openservice.EventPublisher,
	orderRepo orderrepo.OrderRepository,
	configValueReader func(ctx context.Context, key string) (string, bool, error),
	logger *zap.Logger,
	encryptKey string,
) orderservice.ProvisionDeps {
	return orderservice.ProvisionDeps{
		Logger:     logger,
		EncryptKey: encryptKey,
		// 上游账单 id 落库（开通幂等）：账单生成即记，重试复用同一张，不再重复下单。
		RecordUpstreamInvoice: func(ctx context.Context, orderID uint64, invoiceID int64) error {
			if invoiceID <= 0 {
				return nil
			}
			return orderRepo.SetUpstreamInvoiceID(ctx, orderID, uint64(invoiceID))
		},
		// 上游对接令牌落库（首次履约生成后固化）：上游据此反查既有账单/主机。
		RecordDownstreamToken: orderRepo.SetDownstreamToken,
		// 登录密码随单固化（首次履约生成后复用）：上游从不回传密码，不留存就永远查不到。
		// 先原子占位（只在列为空时写入），再读回——并发重试下两个 worker 也只会有一个密码，
		// 不会出现"A 下发密码 X、本地记着密码 Y"的错位。
		EnsureCredentialPassword: func(ctx context.Context, orderID uint64, preferred string) (string, error) {
			if strings.TrimSpace(encryptKey) == "" {
				return "", errors.New("未配置加密密钥（app.encrypt_key），无法留存实例密码")
			}
			plain := strings.TrimSpace(preferred)
			if plain == "" {
				plain = credentials.GeneratePassword()
			}
			enc, err := credentials.EncryptSecret(plain, encryptKey)
			if err != nil {
				return "", err
			}
			if serr := orderRepo.SetCredentialPasswordIfEmpty(ctx, orderID, enc); serr != nil {
				return "", serr
			}
			// 读回权威值：本进程若输给并发 worker，拿到的是对方写入的那一份
			// （上游收到的也一定是那一个，因为它是先写入、再下发）。
			if order, ferr := orderRepo.FindByID(ctx, orderID); ferr == nil && order != nil {
				return firstNonEmpty(order.CredentialPassword, enc), nil
			}
			return enc, nil
		},
		// 本站对外地址（上游握手标识）：运营可在系统配置 site.public_url 填真实地址。
		PublicURL: func(ctx context.Context) string {
			if raw, ok, err := configValueReader(ctx, "site.public_url"); err == nil && ok {
				return raw
			}
			return ""
		},
		BuildProvisionRequest: func(ctx context.Context, productID uint64, name, specSnapshot, specCode string, selections map[string]string) (interface{}, error) {
			return catalog.BuildProvisionRequest(ctx, productID, name, specSnapshot, specCode, selections)
		},
		BuildProviderConfig: provider.BuildProviderConfig,
		CreateInstance: func(ctx context.Context, cfg *upstream.ProviderConfig, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
			provider, err := upmgr.Build(cfg.Type, cfg)
			if err != nil {
				return nil, err
			}
			provisioner, ok := provider.(upstream.InstanceProvisioning)
			if !ok {
				return nil, errors.New("上游不支持开通操作")
			}
			var inst *model.StandardInstance
			err = observability.Timed("upstream_create_instance", func() error {
				var cerr error
				inst, cerr = provisioner.CreateInstance(ctx, req)
				return cerr
			})
			if err != nil {
				return nil, err
			}
			return inst, nil
		},
		RecordInstance: func(ctx context.Context, inst *model.StandardInstance, order *ordermodel.Order) error {
			// 链路判据（D6）：实例归属由商品 source_mode 决定，而非"经订单开通即自营"
			// ——代理商品（upstream）经订单开通后同样是上游链路实例，否则续费会被
			// markRenewalSuccess 误判为链路 B 而静默本地顺延。
			sourceMode := ""
			var upstreamProductID uint64
			var productKind string
			if product, perr := catalog.FindByID(ctx, order.ProductID); perr == nil && product != nil {
				sourceMode = product.SourceMode
				upstreamProductID = product.SourceProductID
				productKind = product.ProductType
			}
			row := buildRecordedInstance(inst, order, sourceMode, upstreamProductID, productKind)
			if row.InstanceID == "" {
				return nil
			}
			// 优先让该订单的「待开通」占位行就地转正（下单时已建）：避免同一台机器
			// 在实例列表里出现"待开通"与真实实例两行；未命中占位才按常规 upsert。
			adopted, aerr := syncRepo.AdoptPendingInstance(ctx, order.ID, &row)
			if aerr != nil {
				return aerr
			}
			if !adopted {
				if err := syncRepo.UpsertInstances(ctx, []syncmodel.Instance{row}); err != nil {
					return err
				}
			}
			// 开放平台 instance.created（P6/T6.5）：仅对代客下单订单发布，事件只落库不阻塞履约。
			if order.OpenAppID != 0 {
				events.PublishToOwner(ctx, order.UserID, openservice.EventInstanceCreated, map[string]any{
					"instance_id":     row.ID,
					"mark":            row.InstanceID,
					"name":            row.Name,
					"status":          row.Status,
					"order_no":        order.OrderNo,
					"customer_ref":    order.ChannelCustomerRef,
					"sell_product_id": order.ProductID,
				})
			}
			return nil
		},
		HasInstance: func(ctx context.Context, userID, productID uint64) (bool, error) {
			n, err := syncRepo.CountInstancesByProductUser(ctx, userID, productID)
			return n > 0, err
		},
	}
}

// buildEnsureOpenable 返回下单前校验函数：校验商品能否直连开通，
// 财务型上游等不支持单次开通的场景先行拒绝，避免误扣款。

// buildEnsureOpenable 返回下单前校验函数：校验商品能否直连开通，
// 财务型上游等不支持单次开通的场景先行拒绝，避免误扣款。
func buildEnsureOpenable(catalog catalogservice.ProductService, provider providerservice.ProviderService) func(ctx context.Context, productID uint64) error {
	return func(ctx context.Context, productID uint64) error {
		preq, err := catalog.BuildProvisionRequest(ctx, productID, "", "", "", nil)
		if err != nil {
			return err
		}
		if preq == nil || preq.ProviderID == 0 {
			return errors.New("该商品未绑定上游，暂不可在线购买")
		}
		cfg, err := provider.BuildProviderConfig(ctx, preq.ProviderID)
		if err != nil {
			return errors.New("该商品上游配置不可用，暂不可购买")
		}
		// 财务型上游（zjmf）现已支持下游下单开通，不再拦截。
		_ = cfg
		return nil
	}
}

// buildRefundPolicyReader 读取退款后实例处置策略（lifecycle_policies.refund_action）。
// 读取失败或值为空一律回落 none：处置线上实例是不可逆动作，配置不可用时必须选「不动作」。
// buildProductSourceModeResolver 返回商品链路判据解析（T5.1 任务留痕用）：
// 读不到商品时返回空串，不影响开通投递。
func buildProductSourceModeResolver(catalog catalogservice.ProductService) func(ctx context.Context, productID uint64) string {
	return func(ctx context.Context, productID uint64) string {
		product, err := catalog.FindByID(ctx, productID)
		if err != nil || product == nil {
			return ""
		}
		return product.SourceMode
	}
}

// provisionManualNotifier 桥接开通任务连续失败告警到通知中心（T5.1）。
// 复用 system 事件避免为一个提示新增模板（与调价待确认一致）。

// buildRecordedInstance 将标准实例转换为同步实例记录（订单履约开通成功后落库）。
// sourceMode 为商品链路判据（D6），upstreamProductID 为代理商品对应的上游资源商品 ID，
// productKind 为商品产品类型（迁移 108，落到实例的产品线归属）。
func buildRecordedInstance(inst *model.StandardInstance, order *ordermodel.Order, sourceMode string, upstreamProductID uint64, productKind string) syncmodel.Instance {
	rawJSON, _ := json.Marshal(inst.RawData)
	// 规格回落（T4.1）：上游详情拉取失败时适配器只返回最小实例（规格为空），
	// 此时用订单规格快照（SKU 原子取值 JSON）补齐，保证实例列表能显示下单所选配置。
	//
	// 逐字段合并，而不是"整块规格全空才回落"（实测故障）：上游回源常常只给部分字段
	// （zjmf 的 host/header 有 CPU/内存/磁盘，但**不一定有操作系统**），
	// 用"全空"当判据会让 OS 一直取不到值、最终显示成订单快照里的商品默认镜像
	// （客户选 Debian 开通、本地显示 CentOS）。空字段补、非空字段以**上游为准**：
	// 上游是这台机器真实落成的配置，订单快照只是下单时的意图。
	specs := inst.Specs
	if snapshot := specsFromOrder(order); snapshot != nil {
		if specs.CPU == 0 {
			specs.CPU = snapshot.CPU
		}
		if specs.Memory == 0 {
			specs.Memory = snapshot.Memory
		}
		if specs.Disk == 0 {
			specs.Disk = snapshot.Disk
		}
		if specs.DiskType == "" {
			specs.DiskType = snapshot.DiskType
		}
		if specs.Bandwidth == 0 {
			specs.Bandwidth = snapshot.Bandwidth
		}
		if specs.OS == "" {
			specs.OS = snapshot.OS
		}
		if specs.Region == "" {
			specs.Region = snapshot.Region
		}
		if specs.Zone == "" {
			specs.Zone = snapshot.Zone
		}
	}
	if sourceMode == "" {
		sourceMode = syncmodel.SourceModeSelf
	}
	// 商品未登记类型时归 other：与列表筛选口径一致（other 档兜住所有未分类行）。
	if productKind == "" {
		productKind = model.ProductKindOther
	} else {
		productKind = model.NormalizeProductKind(productKind)
	}
	row := syncmodel.Instance{
		InstanceID: inst.UpstreamID,
		ProviderID: uint64(inst.ProviderID),
		UserID:     order.UserID,
		Name:       firstNonEmpty(inst.Name, order.ProductName),
		CPU:        specs.CPU,
		Memory:     specs.Memory,
		Disk:       specs.Disk,
		DiskType:   specs.DiskType,
		Bandwidth:  specs.Bandwidth,
		OS:         specs.OS,
		Region:     firstNonEmpty(inst.Region, specs.Region),
		Zone:       firstNonEmpty(inst.Zone, specs.Zone),
		Status:     string(inst.Status),
		PublicIP:   inst.PublicIP,
		PrivateIP:  inst.PrivateIP,
		RawData:    string(rawJSON),
		// 计费周期优先取订单 cycle（doc25）：实例账期推进与续费按真实周期走。
		BillingMode: firstNonEmpty(order.Cycle, order.PriceModel),
		// 子账号下单时 order.OperatorID 为真实操作人，落到实例「操作人」列（P4-09）。
		ActorUserID: order.OperatorID,
		// 记录来源订单，实例运维台据此展示关联订单（见 61 实施计划 §4.1）。
		OrderID: order.ID,
		// 双链路语义（P1/T1.2，D6）：判据取自商品 source_mode，而非"是否经订单开通"
		//——代理商品（upstream）经订单开通后仍是上游链路实例，续费/暂停/销毁须调上游。
		// sell_product_id 恒为我方售出的 products.id（续费算价与展示的权威来源）；
		// 上游链路另记 upstream_product_id（resource_products.id）便于对账与回溯源。
		SourceMode:         sourceMode,
		SellProductID:      order.ProductID,
		UpstreamProductID:  upstreamProductID,
		ProviderInstanceID: inst.UpstreamID,
		// 续费价绑定随订单带过来（未指定时为 nil）：没有占位行的存量路径
		// （常规 upsert 新建实例）也据此保留运营谈定的续费价。
		RenewalPrice: orderRenewalPrice(order),
		// 登录密码密文随单搬到实例：订单上那份是开通时下发上游的**同一个**密码
		// （重试复用同一份），因此这里必须原样带走，不能重新生成。
		// 原样搬运的前提：order.CredentialPassword 已是 enc:v1: 密文（不透明搬运，不解密）。
		CredentialPassword: order.CredentialPassword,
		// 产品类型（迁移 108）：实例导航按产品线分组，判据随商品带过来。
		// 未登记/未知类型归一为 other，避免实例在任何标签页都找不到。
		InstanceType: productKind,
	}
	if !inst.ExpireAt.IsZero() {
		exp := inst.ExpireAt
		row.ExpireAt = &exp
	} else if base, cycles, ok := orderBillingPeriod(order); ok {
		// 上游未返回账期时按订单周期本地起算（自营平台普遍不返回 expire_at）。
		// 没有这一步，实例的 expire_at 恒为 NULL，生命周期推进器（到期暂停/宽限/销毁）
		// 永远筛不到它——"账单计费→到期暂停"这条链路就是断的。
		exp := billingcycle.Advance(base, cycles, 1)
		row.ExpireAt = &exp
	}
	return row
}

// orderBillingPeriod 由订单推出计费起算时间与周期编码：
// 起算时间取支付时间（缺失回退下单时间），周期取 cycle（缺失回退 price_model）。
// 识别不出周期或一次性买断（onetime）时不产生账期，交由上游/人工处理。

// orderBillingPeriod 由订单推出计费起算时间与周期编码：
// 起算时间取支付时间（缺失回退下单时间），周期取 cycle（缺失回退 price_model）。
// 识别不出周期或一次性买断（onetime）时不产生账期，交由上游/人工处理。
func orderBillingPeriod(order *ordermodel.Order) (time.Time, string, bool) {
	cycle := order.Cycle
	if cycle == "" {
		cycle = order.PriceModel
	}
	code := billingcycle.Normalize(cycle)
	if !billingcycle.IsValid(code) || code == billingcycle.Onetime {
		return time.Time{}, "", false
	}
	base := order.CreatedAt
	if order.PayTime != nil {
		base = *order.PayTime
	}
	if base.IsZero() {
		return time.Time{}, "", false
	}
	return base, code, true
}

// specsFromOrder 解析订单的规格快照为标准规格；解析失败返回 nil。
//
// 兼容两种落库形态（实测两套并存）：
//   - SKU 原子取值 JSON：{"compute.cpu":4,"compute.memory":4096,...}（specatom 字典）；
//   - 上游透传快照：{"cpu":4,"memory":4096,"system_disk_size":40,"bw":30,...}
//     （对接商品从 resource_products.raw_specs 克隆而来）。
//
// 只认原子 key 时，第二种会被解出全 0 规格 —— 实例列表/详情显示"0 核/0MB"，
// 用户看到的就是"配置是空的"。
func specsFromOrder(order *ordermodel.Order) *model.StandardProductSpec {
	raw := strings.TrimSpace(order.Specs)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	spec := specatom.FromMap(m)
	// 上游透传 key 兜底（仅补原子 key 未给出的字段）。
	if spec.CPU == 0 {
		spec.CPU = looseInt(m, "cpu", "cpu_num", "cores", "vcpu")
	}
	if spec.Memory == 0 {
		spec.Memory = looseInt(m, "memory", "memory_mb", "ram", "mem")
	}
	if spec.Disk == 0 {
		spec.Disk = looseInt(m, "system_disk_size", "disk", "disk_size", "system_disk")
	}
	if spec.Bandwidth == 0 {
		spec.Bandwidth = looseInt(m, "bandwidth", "bw", "net_bandwidth")
	}
	if spec.DiskType == "" {
		spec.DiskType = looseString(m, "disk_type")
	}
	if spec.OS == "" {
		spec.OS = looseString(m, "os", "image", "system")
	}
	if spec.Region == "" {
		spec.Region = looseString(m, "region", "area", "location")
	}
	if spec.Zone == "" {
		spec.Zone = looseString(m, "zone", "node")
	}
	return &spec
}

// looseInt 依序取第一个能解析为正整数的键（数字/数字字符串皆可）。
func looseInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			if t > 0 {
				return int(t)
			}
		case int:
			if t > 0 {
				return t
			}
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil && n > 0 {
				return n
			}
		case json.Number:
			if n, err := t.Int64(); err == nil && n > 0 {
				return int(n)
			}
		}
	}
	return 0
}

// looseString 依序取第一个非空字符串键。
func looseString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
