package service

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/admin/order/model"
	pkgcredentials "hostsent/backend/internal/pkg/credentials"
	pkgmodel "hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProvisionDeps 上游开通适配器依赖集合（由装配层注入，避免订单包反向依赖产品/资源包）。
type ProvisionDeps struct {
	// BuildProvisionRequest 按商品解析上游开通请求（product/catalog 服务实现）。
	// specSnapshot 为下单时落库的规格快照（SKU 原子取值 JSON，T4.1），可为空；
	// specCode 为所选 SKU 编码（T4.2），用于回查已确认的平台绑定参数，可为空。
	// selections 为客户选配项选择（T4.5，参数名 → 取值），优先级最高，可为空。
	BuildProvisionRequest func(ctx context.Context, productID uint64, name, specSnapshot, specCode string, selections map[string]string) (interface{}, error)
	// BuildProviderConfig 按提供商 ID 构建适配器配置（含解密密钥与提供商类型）。
	BuildProviderConfig func(ctx context.Context, providerID uint64) (*upstream.ProviderConfig, error)
	// CreateInstance 创建上游实例：由装配层用 cfg 实例化 provider 并调用。
	CreateInstance func(ctx context.Context, cfg *upstream.ProviderConfig, req *pkgmodel.CreateInstanceRequest) (*pkgmodel.StandardInstance, error)
	// RecordInstance 开通成功后记录实例（落库 instances）。
	RecordInstance func(ctx context.Context, inst *pkgmodel.StandardInstance, order *model.Order) error
	// HasInstance 判断某用户是否已开通某商品（幂等，避免重复开通）。
	HasInstance func(ctx context.Context, userID, productID uint64) (bool, error)
	// RecordUpstreamInvoice 落库本次开通的上游账单 id（开通幂等：重试复用同一张账单，
	// 不再生成第二张上游账单）。可选，未装配时退化为旧行为（每次重试新建账单）。
	RecordUpstreamInvoice func(ctx context.Context, orderID uint64, invoiceID int64) error
	// RecordDownstreamToken 落库本订单的上游对接令牌（首次履约生成后固化）。
	// 上游据此反查既有账单/主机，是"重试不重复下单"的前置条件。可选。
	RecordDownstreamToken func(ctx context.Context, orderID uint64, token string) error
	// EnsureCredentialPassword 取本订单的登录密码密文：已有则返回既存值（重试复用），
	// 没有则用 preferred（运营在商品配置里钉死的密码，dcimcloud=rootpass、
	// zjmf=root_password）或本系统生成的强随机密码，加密落库后返回密文。
	// 为空表示未装配（退化为不指定密码，由上游适配器自行生成随机密码）。可选。
	EnsureCredentialPassword func(ctx context.Context, orderID uint64, preferred string) (encrypted string, err error)
	// PublicURL 本站对外可访问地址（upstream 握手与回调标识用）；空则用占位地址。可选。
	PublicURL func(ctx context.Context) string
	// EncryptKey 应用加密密钥：用于解密随单固化的登录密码密文（与 pkg/crypto 同源）。
	// 为空时不解密、不带密码下发（上游自行生成随机密码）。
	EncryptKey string
	// Logger 可选日志（记录账单落库失败等不影响履约主流程的事件）。
	Logger *zap.Logger
}

// UpstreamProvisionAdapter 真实上游履约：订单支付后调用上游创建实例并记录，
// 打通「订单——上游」联动。若商品未绑定上游提供商（如虚拟商品/纯本地），
// 则回退到仅推进订单状态（与 DefaultProvisionAdapter 行为一致）。
type UpstreamProvisionAdapter struct {
	deps ProvisionDeps
}

// NewUpstreamProvisionAdapter 创建上游履约适配器。
func NewUpstreamProvisionAdapter(deps ProvisionDeps) *UpstreamProvisionAdapter {
	return &UpstreamProvisionAdapter{deps: deps}
}

// Activate 推进订单开通并触发上游创建实例（幂等）。
// 状态机（遵守 transitionTable：paid → provisioning → active）：
//   - 已支付(paid)：触发上游创建实例并记录 → 置 provisioning；失败返回错误保持 paid 可重试；
//   - 开通中(provisioning)：若实例已记录 → 置 active；否则视为重试上报一次 → active；
//   - 未绑定上游商品 → 回退仅推进状态。
func (a *UpstreamProvisionAdapter) Activate(ctx context.Context, order *model.Order) error {
	switch order.Status {
	case model.OrderStatusActive:
		return nil // 已是服务中，幂等返回
	case model.OrderStatusPaid:
		if order.ProductID == 0 {
			return a.advanceLegacy(order)
		}
		inst, err := a.provision(ctx, order)
		if err != nil {
			return err // 保持 paid，交由重试
		}
		if inst == nil {
			return a.advanceLegacy(order)
		}
		_ = a.deps.RecordInstance(ctx, inst, order)
		if err := EnsureStatus(order.Status, model.OrderStatusProvisioning); err != nil {
			return err
		}
		order.Status = model.OrderStatusProvisioning
		// 上游主机已就绪（同步开通）：直接推进到服务中，避免订单停留在 provisioning 等待人工重试。
		if inst.Status == pkgmodel.InstanceStatusRunning {
			return a.markActive(order)
		}
		return nil
	case model.OrderStatusProvisioning:
		if order.ProductID == 0 {
			return a.advanceLegacy(order)
		}
		// 幂等：已在上游开通（存在非占位实例）则回源确认主机真实状态，
		// 只有确认 running 才置服务中，仍创建中则保持 provisioning。
		// HasInstance 已排除待开通占位行（占位行不是"已开通"）。
		if a.deps.HasInstance != nil {
			exists, err := a.deps.HasInstance(ctx, order.UserID, order.ProductID)
			if err == nil && exists {
				inst, perr := a.provision(ctx, order)
				if perr != nil {
					return perr
				}
				if inst == nil {
					return a.markActive(order)
				}
				// 先落库再判状态：否则"上游已就绪"这条路径只改了订单状态，
				// 本地实例仍是占位行/旧状态，用户看到的还是"待开通"。
				_ = a.deps.RecordInstance(ctx, inst, order)
				if inst.Status == pkgmodel.InstanceStatusRunning {
					return a.markActive(order)
				}
				return nil
			}
		}
		inst, err := a.provision(ctx, order)
		if err != nil {
			return err
		}
		if inst == nil {
			return a.markActive(order)
		}
		_ = a.deps.RecordInstance(ctx, inst, order)
		// 上游主机已就绪才可直接置服务中；仍创建中保持 provisioning，
		// 由滞留推进器（ProvisioningPromoter）在主机就绪后跟进。
		if inst.Status == pkgmodel.InstanceStatusRunning {
			return a.markActive(order)
		}
		return nil
	default:
		return ErrStatusConflict
	}
}

// markActive 校验并置订单为服务中，同时记录开通成功时间。
//
// activated_at 是退款时限策略的起算点（见 model.Order.ActivatedAt）：
// 只在首次转入服务中时写，重试/回源推进不覆盖。
func (a *UpstreamProvisionAdapter) markActive(order *model.Order) error {
	if err := EnsureStatus(order.Status, model.OrderStatusActive); err != nil {
		return err
	}
	order.Status = model.OrderStatusActive
	if order.ActivatedAt == nil {
		now := time.Now()
		order.ActivatedAt = &now
	}
	return nil
}

// provision 调用上游创建实例。
func (a *UpstreamProvisionAdapter) provision(ctx context.Context, order *model.Order) (*pkgmodel.StandardInstance, error) {
	if a.deps.BuildProvisionRequest == nil || a.deps.CreateInstance == nil || a.deps.BuildProviderConfig == nil {
		return nil, nil
	}
	// 客户选配项（T4.5）：下单时的选择落库在 orders.config_options，
	// 优先级高于 SKU 平台绑定与商品默认参数（客户实际购买决定）。
	raw, err := a.deps.BuildProvisionRequest(ctx, order.ProductID, order.ProductName, order.Specs, order.SpecCode, orderConfigSelections(order))
	if err != nil {
		return nil, fmt.Errorf("构建开通请求失败: %w", err)
	}
	pid := providerID(raw)
	if pid == 0 {
		// 商品未绑定上游提供商（非云商品），回退为本地推进。
		return nil, nil
	}
	cfg, err := a.deps.BuildProviderConfig(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("构建提供商配置失败: %w", err)
	}
	req := &pkgmodel.CreateInstanceRequest{
		ProviderType: cfg.Type,
		ProductID:    uint(order.ProductID),
		Name:         order.ProductName,
		Extra:        extractConfigOptions(raw),
		// 计费周期优先取订单 cycle（doc25），空则回落 price_model（存量订单）。
		BillingMode: orderBillingCycle(order),
		// 幂等：带上次已生成的上游账单 id；适配器据此跳过"下单+结算"直接支付该账单。
		UpstreamInvoiceID: order.UpstreamInvoiceID,
	}
	// 登录密码由本系统生成并随订单固化：上游从不回传密码（dcimcloud 详情该字段实测为空、
	// zjmf 无读密码接口），不在这里留存，实例详情页的「查看密码」就永远取不到值。
	// 明文只在本次下发上游时存在于内存；落库的是密文，供后续「查看密码」解密回显。
	enc := strings.TrimSpace(order.CredentialPassword)
	if enc == "" && a.deps.EnsureCredentialPassword != nil {
		// 首次履约：优先用运营在商品配置里钉死的密码（适配器自身也认这两个键：
		// dcimcloud=rootpass、zjmf=root_password），否则本系统生成强随机密码。
		// 失败只告警不阻断（上游会自行生成随机密码，开通照常成功，
		// 代价是这台机器事后无法回看初始密码）。
		if generated, perr := a.deps.EnsureCredentialPassword(ctx, order.ID, preferredPassword(raw)); perr != nil {
			if a.deps.Logger != nil {
				a.deps.Logger.Warn("生成/固化开通密码失败（本次交由上游生成随机密码，开通后无法回看）",
					zap.Uint64("order_id", order.ID), zap.Error(perr))
			}
		} else {
			enc = generated
			order.CredentialPassword = generated
		}
	}
	if enc != "" {
		if plain, derr := pkgcredentials.DecryptSecret(enc, a.deps.EncryptKey); derr == nil {
			req.Password = plain
		} else if a.deps.Logger != nil {
			// 解密失败（密钥轮换/数据损坏）：不带密码下发，绝不把密文当明文传给上游。
			a.deps.Logger.Warn("开通密码解密失败，本次交由上游生成随机密码",
				zap.Uint64("order_id", order.ID), zap.Error(derr))
		}
	}
	// 下游身份三件套（上游幂等握手用）：token 首次生成后落库，重试必须复用同一个。
	// 生成失败不阻断履约：本次仍带 token 走（握手在本进程内有效），仅日志告警。
	if token, err := a.downstreamToken(ctx, order); err != nil {
		if a.deps.Logger != nil {
			a.deps.Logger.Warn("生成上游对接令牌失败（重试可能重复下单）",
				zap.Uint64("order_id", order.ID), zap.Error(err))
		}
	} else {
		req.DownstreamToken = token
		req.DownstreamID = order.ID
		req.DownstreamURL = a.downstreamURL(ctx)
	}
	if a.deps.RecordUpstreamInvoice != nil {
		orderID := order.ID
		req.OnInvoiceCreated = func(invoiceID int64) {
			// 账单生成即落库（在支付之前）：支付失败（余额不足）后的重试才有账单可复用。
			if err := a.deps.RecordUpstreamInvoice(ctx, orderID, invoiceID); err != nil {
				if a.deps.Logger != nil {
					a.deps.Logger.Warn("记录上游账单ID失败（重试将可能重复下单）",
						zap.Uint64("order_id", orderID), zap.Int64("invoice_id", invoiceID), zap.Error(err))
				}
				return
			}
			order.UpstreamInvoiceID = invoiceID
		}
	}
	return a.deps.CreateInstance(ctx, cfg, req)
}

// downstreamToken 取本订单的上游对接令牌：已有则复用，没有则生成并落库。
// 令牌必须按订单稳定 —— 上游据此反查既有账单/主机，换了令牌等于换了订单身份，
// 重试就会在上游新建一份（实测事故的直接原因）。
func (a *UpstreamProvisionAdapter) downstreamToken(ctx context.Context, order *model.Order) (string, error) {
	if t := strings.TrimSpace(order.DownstreamToken); len(t) == 32 {
		return t, nil
	}
	token := newDownstreamToken()
	if a.deps.RecordDownstreamToken == nil {
		return token, nil
	}
	if err := a.deps.RecordDownstreamToken(ctx, order.ID, token); err != nil {
		return token, err
	}
	order.DownstreamToken = token
	return token, nil
}

// newDownstreamToken 生成 32 位十六进制随机令牌（与上游协议长度要求一致）。
func newDownstreamToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// 随机源不可用时退回按时间派生：位数与格式仍满足上游校验。
		sum := md5.Sum([]byte(fmt.Sprintf("hostsent-downstream-%d", time.Now().UnixNano())))
		return hex.EncodeToString(sum[:])
	}
	return hex.EncodeToString(buf)
}

// downstreamURL 告知上游"本站在哪"（downstream_url）。上游只做 http(s://) 格式校验，
// 并在主机事件时向该地址推送通知；本站未实现接收端时填占位地址不影响开通。
// 运营可在系统配置 site.public_url 里填真实对外地址。
func (a *UpstreamProvisionAdapter) downstreamURL(ctx context.Context) string {
	if a.deps.PublicURL != nil {
		if u := strings.TrimSpace(a.deps.PublicURL(ctx)); u != "" {
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				return u
			}
		}
	}
	return defaultDownstreamURL
}

// defaultDownstreamURL 未配置公开地址时的占位地址（仅需是合法 http(s) URL）。
const defaultDownstreamURL = "https://hostsent.local"

// orderBillingCycle 取订单计费周期：cycle 权威，price_model 兼容兜底。
func orderBillingCycle(order *model.Order) string {
	if order.Cycle != "" {
		return order.Cycle
	}
	return order.PriceModel
}

// preferredPassword 取运营在商品配置里钉死的登录密码（可为空）。
//
// 两家适配器的取值键名不同且都只认自己的（dcimcloud provider.go 的 opts["rootpass"]、
// zjmf provider.go 的 extra["root_password"]），这里两个都认：key 与适配器的实际读取
// 键名一致，取到谁就用谁，不影响另一家的语义。
func preferredPassword(raw interface{}) string {
	return firstNonEmptyOption(raw, "rootpass", "root_password")
}

// firstNonEmptyOption 从开通请求的 ConfigOptions 里按顺序取第一个非空字符串值。
func firstNonEmptyOption(raw interface{}, keys ...string) string {
	opts := extractConfigOptions(raw)
	for _, k := range keys {
		v, ok := opts[k]
		if !ok || v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// orderConfigSelections 解析订单落库的客户选配项选择（T4.5）。
// 解析失败返回 nil：履约不应因这个可选字段的脏数据而中断，
// 后续 BuildProvisionRequest 会退回档位基线参数。
func orderConfigSelections(order *model.Order) map[string]string {
	raw := strings.TrimSpace(order.ConfigOptions)
	if raw == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// advanceLegacy 无上游商品的订单：直接推进状态（与 DefaultProvisionAdapter 一致）。
func (a *UpstreamProvisionAdapter) advanceLegacy(order *model.Order) error {
	switch order.Status {
	case model.OrderStatusPaid:
		if err := EnsureStatus(order.Status, model.OrderStatusProvisioning); err != nil {
			return err
		}
		order.Status = model.OrderStatusProvisioning
		return nil
	case model.OrderStatusProvisioning:
		if err := EnsureStatus(order.Status, model.OrderStatusActive); err != nil {
			return err
		}
		order.Status = model.OrderStatusActive
		if order.ActivatedAt == nil {
			now := time.Now()
			order.ActivatedAt = &now
		}
		return nil
	default:
		return ErrStatusConflict
	}
}

// providerID 从订单履约请求中提取提供商 ID（反射读取 ProviderID 字段）。
func providerID(raw interface{}) uint64 {
	if raw == nil {
		return 0
	}
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0
	}
	f := v.FieldByName("ProviderID")
	if !f.IsValid() {
		return 0
	}
	return uint64(toUint(f))
}

// extractConfigOptions 从订单履约请求中提取可配置项（反射读取 ConfigOptions 字段）。
func extractConfigOptions(raw interface{}) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return map[string]interface{}{}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return map[string]interface{}{}
	}
	f := v.FieldByName("ConfigOptions")
	if !f.IsValid() {
		return map[string]interface{}{}
	}
	if f.Kind() == reflect.Map && f.IsNil() {
		return map[string]interface{}{}
	}
	if m, ok := f.Interface().(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// toUint 将任意整型/浮点值转为 uint64（反射辅助）。
func toUint(v reflect.Value) uint64 {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	}
	return 0
}
