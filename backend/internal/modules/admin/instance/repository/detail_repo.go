package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hostsent/backend/internal/modules/admin/instance/dto"
	"hostsent/backend/internal/pkg/billingcycle"
)

// 本文件提供实例详情页的附加只读数据：来源订单财务口径、售出商品与对接关系、
// 上游商品价格、实例生效配置项、登录连接信息。
//
// 与 related_repo.go 同样的取舍：跨表只读查询直接在本模块内完成，
// 不为「一屏看全」给每个关联域引入模块依赖（否则 instance 包会依赖 order/product/resource 三个包）。

// InstanceDetailRepository 实例详情附加数据只读仓储。
type InstanceDetailRepository interface {
	// SourceOrder 读来源订单（含按实例当前周期算出的续费价）。
	SourceOrder(ctx context.Context, orderID uint64, billingMode string, quantity int) (*dto.SourceOrderInfo, error)
	// Product 读售出商品与对接关系（含渠道名）。
	Product(ctx context.Context, productID uint64) (*dto.ProductInfo, error)
	// Upstream 读对接商品的上游信息（非对接商品返回 nil）。
	Upstream(ctx context.Context, sourceProductID uint64, billingMode string) (*dto.UpstreamInfo, float64, error)
	// ProductConfigs 读商品配置项并与订单选配/实例取值合并。
	ProductConfigs(ctx context.Context, productID uint64, orderConfigOptions string) ([]dto.ConfigItem, error)
	// Credentials 读实例登录连接信息（不含明文密码）。
	Credentials(ctx context.Context, instanceID uint64) (*dto.CredentialInfo, error)
	// RevealCredential 读明文密码（供 reveal 接口；未存返回空）。
	RevealCredential(ctx context.Context, instanceID uint64) (username, password, port string, err error)
}

type instanceDetailRepository struct {
	db *gorm.DB
}

// NewInstanceDetailRepository 创建实例详情附加数据仓储。
func NewInstanceDetailRepository(db *gorm.DB) InstanceDetailRepository {
	return &instanceDetailRepository{db: db}
}

// ============================ 来源订单 ============================

// orderDetailRow orders 读取行。
type orderDetailRow struct {
	ID             uint64
	OrderNo        string
	Status         string
	ProductID      uint64
	ProductName    string
	Cycle          string
	Quantity       int
	TotalAmount    float64
	PaidAmount     float64
	DiscountAmount float64
	PayMethod      string
	PayTime        *time.Time
	CreatedAt      time.Time
	RenewalID      uint64
}

// SourceOrder 读来源订单，并按其「当前付款周期」补出下次续费金额。
//
// 为什么续费金额要现算而不是读订单里的金额：订单金额是**历史成交**（可能带折扣、
// 可能是首月促销价），而运营关心的是"这台机器下次续费要多少钱"。后者由商品当前
// 周期价（product_prices）决定。
func (r *instanceDetailRepository) SourceOrder(ctx context.Context, orderID uint64, billingMode string, quantity int) (*dto.SourceOrderInfo, error) {
	if orderID == 0 {
		return nil, nil
	}
	var row orderDetailRow
	err := r.db.WithContext(ctx).Table("orders").
		Select("id, order_no, status, product_id, product_name, cycle, quantity, total_amount, paid_amount, discount_amount, pay_method, pay_time, created_at, renewal_id").
		Where("id = ?", orderID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := &dto.SourceOrderInfo{
		ID:           row.ID,
		OrderNo:      row.OrderNo,
		Status:       row.Status,
		ProductID:    row.ProductID,
		ProductName:  row.ProductName,
		Cycle:        row.Cycle,
		CycleName:    billingcycle.Name(row.Cycle),
		Quantity:     row.Quantity,
		TotalAmount:  row.TotalAmount,
		PaidAmount:   row.PaidAmount,
		DiscountAmt:  row.DiscountAmount,
		FirstPaidAmt: row.PaidAmount,
		PayMethod:    row.PayMethod,
		PayTime:      fmtTimeOpt(row.PayTime),
		OrderedAt:    row.CreatedAt.Format(time.RFC3339),
		RenewalID:    row.RenewalID,
	}
	// 续费金额：按实例当前付款周期（可能已被调整过）取商品周期价。
	cycle := billingcycle.NormalizeOrDefault(billingMode)
	if cycle == "" {
		cycle = billingcycle.NormalizeOrDefault(row.Cycle)
	}
	if row.ProductID > 0 && cycle != "" {
		sale, cost, ok := r.cyclePrice(ctx, row.ProductID, cycle)
		if ok {
			n := quantity
			if n <= 0 {
				n = 1
			}
			out.RenewUnitCost = cost
			out.RenewCycle = cycle
			out.RenewAmount = money2(sale * float64(n))
			out.RenewCost = money2(cost * float64(n))
		}
	}
	return out, nil
}

// cyclePrice 读商品某周期的售价与成本价（product_prices）。
func (r *instanceDetailRepository) cyclePrice(ctx context.Context, productID uint64, cycle string) (sale, cost float64, ok bool) {
	var row struct {
		Price     float64
		CostPrice float64
	}
	err := r.db.WithContext(ctx).Table("product_prices").
		Select("price, cost_price").
		Where("product_id = ? AND cycle = ? AND product_spec_id = 0 AND status = 1", productID, cycle).
		Take(&row).Error
	if err != nil {
		return 0, 0, false
	}
	return row.Price, row.CostPrice, true
}

// ============================ 售出商品 ============================

// Product 读售出商品（含渠道名/类型，便于判断能否改配置）。
func (r *instanceDetailRepository) Product(ctx context.Context, productID uint64) (*dto.ProductInfo, error) {
	if productID == 0 {
		return nil, nil
	}
	var row struct {
		ID                 uint64
		Code               string
		Name               string
		SourceMode         string
		SourceProviderID   uint64
		SourceProductID    uint64
		PriceModel         string
		Price              float64
		CostPrice          float64
		UpstreamMarkupType string
		UpstreamMarkupVal  float64
		Status             int
		ProviderName       string
		ProviderType       string
	}
	err := r.db.WithContext(ctx).Table("products AS p").
		Select(`p.id, p.code, p.name, p.source_mode, p.source_provider_id, p.source_product_id,
			p.price_model, p.price, p.cost_price, p.upstream_markup_type, p.upstream_markup_value, p.status,
			pr.name AS provider_name, pr.provider_type AS provider_type`).
		Joins("LEFT JOIN resource_providers AS pr ON pr.id = p.source_provider_id").
		Where("p.id = ?", productID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &dto.ProductInfo{
		ID:               row.ID,
		Code:             row.Code,
		Name:             row.Name,
		SourceMode:       row.SourceMode,
		SourceProviderID: row.SourceProviderID,
		SourceProvider:   row.ProviderName,
		ProviderType:     row.ProviderType,
		SourceProductID:  row.SourceProductID,
		PriceModel:       row.PriceModel,
		Price:            row.Price,
		CostPrice:        row.CostPrice,
		MarkupType:       row.UpstreamMarkupType,
		MarkupValue:      row.UpstreamMarkupVal,
		Status:           row.Status,
	}, nil
}

// ============================ 上游商品 ============================

// upstreamRaw 上游镜像快照（resource_products.raw_specs）中本页需要的子集。
type upstreamRaw struct {
	GID          int64                      `json:"gid"`
	GroupName    string                     `json:"group_name"`
	PayType      string                     `json:"pay_type"`
	ListPrice    float64                    `json:"list_price"`
	CostMonthly  float64                    `json:"cost_price_monthly"`
	PricingsCost []map[string]interface{}   `json:"product_pricings"`
	PricingsList []map[string]interface{}   `json:"product_pricings_list"`
	Discount     map[string]interface{}     `json:"upstream_discount"`
	PayTypeRaw   map[string]json.RawMessage `json:"-"`
}

// Upstream 读对接商品的上游信息；同时返回该实例当前周期的上游续费金额。
//
// 第二个返回值是"上游续费金额"（上游标价口径，即我们向上游下单时上游收的价），
// 非对接商品或无该周期价时为 0。
func (r *instanceDetailRepository) Upstream(ctx context.Context, sourceProductID uint64, billingMode string) (*dto.UpstreamInfo, float64, error) {
	if sourceProductID == 0 {
		return nil, 0, nil
	}
	var row struct {
		ID         uint64
		ProviderID uint64
		UpstreamID string
		Name       string
		GroupID    int64
		GroupName  string
		RawSpecs   string
		SalePrice  float64
		CostPrice  float64
	}
	err := r.db.WithContext(ctx).Table("resource_products").
		Select("id, provider_id, upstream_id, name, group_id, group_name, raw_specs, sale_price, cost_price").
		Where("id = ?", sourceProductID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	raw := parseUpstreamRaw(row.RawSpecs)
	out := &dto.UpstreamInfo{
		ResourceProductID: row.ID,
		UpstreamID:        row.UpstreamID,
		Name:              row.Name,
		GroupID:           firstNonZeroInt64(raw.GID, row.GroupID),
		GroupName:         firstNonEmpty(raw.GroupName, row.GroupName),
		ListPrice:         firstNonZeroFloat(raw.ListPrice, row.SalePrice),
		CostPrice:         firstNonZeroFloat(raw.CostMonthly, row.CostPrice),
		PayType:           raw.PayType,
		Renewable:         raw.PayType == "" || raw.PayType == "recurring",
		ListCycles:        cycleMap(raw.PricingsList),
		CostCycles:        cycleMap(raw.PricingsCost),
	}
	if d, ok := raw.Discount["desc"].(string); ok && d != "" {
		out.DiscountDesc = d
	}
	// 上游续费金额：优先取上游标价（上给我们看的价），没有则取采购价。
	cycle := billingcycle.NormalizeOrDefault(billingMode)
	upstreamRenew := out.ListCycles[cycle]
	if upstreamRenew == 0 {
		upstreamRenew = out.CostCycles[cycle]
	}
	return out, money2(upstreamRenew), nil
}

// cycleNameToKey 上游 pricing 行里的周期字段名（与 billingcycle 规范值的映射）。
var cycleKeys = []struct {
	key   string
	cycle string
}{
	{"hour", billingcycle.Hourly},
	{"day", billingcycle.Daily},
	{"monthly", billingcycle.Monthly},
	{"quarterly", billingcycle.Quarterly},
	{"semiannually", billingcycle.Semiannually},
	{"annually", billingcycle.Annually},
	{"biennially", billingcycle.Biennially},
	{"triennially", billingcycle.Triennially},
	{"onetime", billingcycle.Onetime},
}

// cycleMap 把上游 pricing 行（可能多行、多币种）压成「周期 → 金额」。
// 只取第一行（默认币种行），负值（上游用 -1 表示"该周期不卖"）一律丢弃。
func cycleMap(rows []map[string]interface{}) map[string]float64 {
	out := map[string]float64{}
	if len(rows) == 0 {
		return out
	}
	row := rows[0]
	for _, ck := range cycleKeys {
		if v, ok := row[ck.key]; ok {
			if f, ok2 := toFloat(v); ok2 && f >= 0 {
				out[ck.cycle] = f
			}
		}
	}
	return out
}

// parseUpstreamRaw 解析上游镜像快照（失败返回零值，调用方按"无上游信息"降级）。
func parseUpstreamRaw(raw string) upstreamRaw {
	var out upstreamRaw
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// ============================ 配置项 ============================

// ProductConfigs 读商品配置项（子表）并与「订单选配」「实例取值」合并。
//
// 三层优先级（后者覆盖前者）：
//
//	商品默认（product_config_options 首个可见子项）
//	  → 订单选配（orders.config_options，客户实际所选）
//	    → 实例表字段（cpu/memory/disk/bandwidth/os/region/zone，回源刷新过，最准）
//
// 上游锁定项（对接商品里只有一个可见取值的项）标记 Locked=true，提示运营不可改。
func (r *instanceDetailRepository) ProductConfigs(ctx context.Context, productID uint64, orderConfigOptions string) ([]dto.ConfigItem, error) {
	var rows []struct {
		OptionName string
		OptionType int
		Source     string
		SourceKey  string
		OptionKey  string
		Unit       string
		SubName    string
		SubKey     string
		SubHidden  int
	}
	err := r.db.WithContext(ctx).Table("product_config_options AS o").
		Select(`o.option_name, o.option_type, o.source, o.source_key, o.option_key, o.unit,
			s.option_name AS sub_name, s.source_key AS sub_key, s.hidden AS sub_hidden`).
		Joins("LEFT JOIN product_config_options_sub AS s ON s.option_id = o.id").
		Where("o.product_id = ?", productID).
		Order("o.sort_order ASC, o.id ASC, s.sort_order ASC, s.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	selections := parseSelections(orderConfigOptions)
	// 子项取值字典：选项 source_key → { 子项 source_key: 子项展示名 }。
	// 订单选配里存的是子项 source_key（如 os=62），直接展示只是裸数字；
	// 用本字典翻成中文名（CentOS-7.9），运营才看得懂。
	subLabels := map[string]map[string]string{}
	for _, row := range rows {
		if row.SubKey == "" {
			continue
		}
		optKey := row.SourceKey
		if optKey == "" {
			optKey = row.OptionKey
		}
		if subLabels[optKey] == nil {
			subLabels[optKey] = map[string]string{}
		}
		subLabels[optKey][row.SubKey] = trimOptionValue(row.SubName)
	}
	out := make([]dto.ConfigItem, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		key := configCanonicalKey(row.OptionName, row.SourceKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		item := dto.ConfigItem{
			Key:    key,
			Label:  optionLabel(row.OptionName),
			Unit:   row.Unit,
			Source: "product",
			Value:  trimOptionValue(row.SubName),
		}
		// 订单选配覆盖：orders.config_options 用「上游选项 id」或「平台参数名」作键。
		if v, ok := selections[row.SourceKey]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.SourceKey]), "order"
		} else if v, ok := selections[key]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.SourceKey]), "order"
		} else if v, ok := selections[row.OptionKey]; ok && v != "" {
			item.Value, item.Source = resolveSelection(v, subLabels[row.OptionKey]), "order"
		}
		if row.Source == "upstream" {
			item.Locked = true
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// resolveSelection 把订单选配里的子项键翻成展示名；查不到则原样返回。
func resolveSelection(value string, labels map[string]string) string {
	if labels == nil {
		return value
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok && label != "" {
		return label
	}
	return value
}

// configKeyAliases 配置项名/上游键 → 机器键。自营商品的 option_name 是中文短名
// （"CPU"/"内存"/"数据盘"），上游商品是 "cpu|CPU" 形式，两者必须落到同一个机器键，
// 否则「实例字段覆盖取值」与前端去重都匹配不上，配置区会出现重复行。
var configKeyAliases = map[string]string{
	"cpu":              "cpu",
	"内存":               "memory",
	"memory":           "memory",
	"数据盘":              "data_disk_size",
	"data_disk_size":   "data_disk_size",
	"datadisk":         "data_disk_size",
	"操作系统":             "os",
	"os":               "os",
	"带宽":               "bw",
	"bw":               "bw",
	"bandwidth":        "bw",
	"ip 数量":            "ip_num",
	"ip数量":             "ip_num",
	"ip_num":           "ip_num",
	"系统盘":              "system_disk_size",
	"system_disk_size": "system_disk_size",
	"区域":               "area",
	"area":             "area",
	"region":           "area",
	"可用区":              "zone",
	"zone":             "zone",
	"node":             "zone",
	"网络类型":             "network_type",
	"network_type":     "network_type",
}

// configCanonicalKey 归一化配置项键：先看 option_name 的中文/英文名，再看 source_key，
// 都不认识时退回归一化后的 option_name（保证任何配置项都能展示，只是覆盖不到实例值）。
func configCanonicalKey(optionName, sourceKey string) string {
	if c, ok := configKeyAliases[strings.ToLower(strings.TrimSpace(normalizeOptionName(optionName)))]; ok {
		return c
	}
	sk := strings.ToLower(strings.TrimSpace(sourceKey))
	if c, ok := configKeyAliases[sk]; ok {
		return c
	}
	if name := strings.ToLower(strings.TrimSpace(normalizeOptionName(optionName))); name != "" {
		return name
	}
	return sk
}

// parseSelections 解析订单落库的客户选配（参数名 → 取值）。
func parseSelections(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// normalizeOptionName 归一化配置项名：上游形如 "cpu|CPU"/"os|操作系统"，取竖线前的机器键。
func normalizeOptionName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// optionLabel 取配置项展示名：优先竖线后的中文名，没有则用原名。
func optionLabel(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "|"); i >= 0 && i+1 < len(s) {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// trimOptionValue 去掉子项名前缀（上游形如 "16|16核"）并清理版本后缀。
func trimOptionValue(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "^"); i >= 0 {
		s = s[:i] + s[i+1:]
	}
	return strings.TrimSpace(s)
}

// ============================ 登录连接信息 ============================

// Credentials 读实例登录连接信息（从 raw_data 提取，不含明文密码）。
//
// raw_data 是上游详情的原样快照，字段名随平台不同（魔方云 osuser/rootpassword/port，
// 魔方财务 username/port）。这里只判定"有没有密码"，绝不把明文带回详情响应。
func (r *instanceDetailRepository) Credentials(ctx context.Context, instanceID uint64) (*dto.CredentialInfo, error) {
	var row struct {
		RawData      string
		ProviderType string
	}
	err := r.db.WithContext(ctx).Table("instances AS i").
		Select("i.raw_data, pr.provider_type AS provider_type").
		Joins("LEFT JOIN resource_providers AS pr ON pr.id = i.provider_id").
		Where("i.id = ?", instanceID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	var raw map[string]interface{}
	if strings.TrimSpace(row.RawData) != "" {
		_ = json.Unmarshal([]byte(row.RawData), &raw)
	}
	out := &dto.CredentialInfo{
		Username:    firstNonEmpty(rawStr(raw, "osuser"), rawStr(raw, "username")),
		Port:        firstNonEmpty(rawStr(raw, "port"), defaultPort(row.ProviderType)),
		HasPassword: false,
	}
	if p := firstNonEmpty(rawStr(raw, "rootpassword"), rawStr(raw, "password")); p != "" {
		out.HasPassword = true
	}
	if p := rawStr(raw, "panel_pass"); p != "" {
		out.HasPanelPassword = true
	}
	switch {
	case out.HasPassword:
		out.PasswordHint = "上游已下发初始密码，可点「查看密码」获取"
	case row.ProviderType == "zjmf":
		out.PasswordHint = "上游财务不回传密码，请用「重置密码」或到上游面板查看"
	default:
		out.PasswordHint = "上游未返回密码，请用「重置密码」设置"
	}
	return out, nil
}

// RevealCredential 读明文密码（供 reveal 接口）。未存返回空串，由调用方提示改用重置密码。
func (r *instanceDetailRepository) RevealCredential(ctx context.Context, instanceID uint64) (string, string, string, error) {
	var row struct {
		RawData string
	}
	err := r.db.WithContext(ctx).Table("instances").Select("raw_data").Where("id = ?", instanceID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", "", nil
		}
		return "", "", "", err
	}
	var raw map[string]interface{}
	if strings.TrimSpace(row.RawData) != "" {
		_ = json.Unmarshal([]byte(row.RawData), &raw)
	}
	return firstNonEmpty(rawStr(raw, "osuser"), rawStr(raw, "username")),
		firstNonEmpty(rawStr(raw, "rootpassword"), rawStr(raw, "password")),
		rawStr(raw, "port"), nil
}

// defaultPort 平台默认 SSH 端口（仅用于展示兜底）。
func defaultPort(providerType string) string {
	switch providerType {
	case "dcimcloud", "zjmf":
		return "22"
	default:
		return ""
	}
}

// ============================ 小工具 ============================

func rawStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// fmtTimeOpt 时间指针转 RFC3339；空值返回空串。
func fmtTimeOpt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZeroFloat(vals ...float64) float64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func firstNonZeroInt64(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func money2(v float64) float64 {
	return float64(int64(v*100+copySign(0.5, v))) / 100
}

func copySign(mag, sign float64) float64 {
	if sign < 0 {
		return -mag
	}
	return mag
}
