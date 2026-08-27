// Package mofangyun 为魔方云上游提供商的适配器实现。
//
// 真实魔方云 API 为面板式入口 `index.php?m=api&a=<方法>`，通过 POST + 公共参数
// 调用，响应统一为 `{code, msg, data}`。本适配器持有独立配置（支持同类型多提供商），
// 并把魔方云的「商品/实例/节点/账户」映射为统一标准模型。
//
// 注意：具体方法名（a=xxx）与鉴权字段名以魔方云后台「系统设置-开发文档」为准，
// 需在对真实环境联编时核对；本实现已集中到 call() 与下方 action 常量，便于调整。
package mofangyun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方云提供商类型标识
const ProviderType = "mofangyun"

// 魔方云接口动作（a=xxx）。命名以魔方云「系统设置-开发文档」为准。
const (
	actionHealthCheck     = "ping"
	actionProductList     = "getProductList"
	actionProductInfo     = "getProduct"
	actionCreateInstance  = "createInstance"
	actionInstanceList    = "getInstanceList"
	actionInstanceInfo    = "getInstance"
	actionStartInstance   = "startInstance"
	actionStopInstance    = "stopInstance"
	actionRestartInstance = "restartInstance"
	actionDeleteInstance  = "deleteInstance"
	actionResizeInstance  = "resizeInstance"
	actionNodeList        = "getNodeList"
	actionAccountInfo     = "getAccountInfo"
)

func init() {
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewMoFangYunProvider(cfg)
	})
}

// MoFangYunProvider 魔方云适配器
type MoFangYunProvider struct {
	config *upstream.ProviderConfig
	client *http.Client
}

// NewMoFangYunProvider 按配置创建魔方云适配器实例
func NewMoFangYunProvider(config *upstream.ProviderConfig) *MoFangYunProvider {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MoFangYunProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// GetType 返回提供商类型标识
func (p *MoFangYunProvider) GetType() string { return ProviderType }

// GetName 返回提供商名称
func (p *MoFangYunProvider) GetName() string { return "魔方云" }

// call 统一请求魔方云接口并解析 `{code, msg, data}`。
// result 非空时将 data 反序列化到该对象；code != 0 视为业务失败。
func (p *MoFangYunProvider) call(ctx context.Context, action string, params url.Values, result any) error {
	endpoint := strings.TrimRight(p.config.APIEndpoint, "/") + "/index.php"
	query := "?m=api&a=" + url.QueryEscape(action)

	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	// 公共鉴权参数（字段名以魔方云开发文档为准）
	if p.config.APISecret != "" {
		params.Set("user", p.config.APISecret)
	}
	if p.config.APIKey != "" {
		params.Set("pass", p.config.APIKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+query, strings.NewReader(params.Encode()))
	if err != nil {
		return &upstream.ProviderError{Op: action, Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return &upstream.ProviderError{Op: action, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &upstream.ProviderError{Op: action, StatusCode: resp.StatusCode}
	}

	var outer struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&outer); err != nil {
		return &upstream.ProviderError{Op: action, Err: err}
	}
	if outer.Code != 0 {
		return &upstream.ProviderError{Op: action, Code: outer.Code, Msg: outer.Msg}
	}
	if result != nil && len(outer.Data) > 0 && string(outer.Data) != "null" {
		if err := json.Unmarshal(outer.Data, result); err != nil {
			return &upstream.ProviderError{Op: action, Err: err}
		}
	}
	return nil
}

// cloneValues 返回独立副本，避免污染调用方。
func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func (p *MoFangYunProvider) HealthCheck(ctx context.Context) error {
	return p.call(ctx, actionHealthCheck, nil, nil)
}

func (p *MoFangYunProvider) ListProducts(ctx context.Context) ([]*model.StandardProduct, error) {
	var list []MoFangYunProduct
	if err := p.call(ctx, actionProductList, nil, &list); err != nil {
		return nil, err
	}
	products := make([]*model.StandardProduct, 0, len(list))
	for i := range list {
		products = append(products, convertProduct(&list[i]))
	}
	return products, nil
}

func (p *MoFangYunProvider) GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error) {
	var item MoFangYunProduct
	if err := p.call(ctx, actionProductInfo, url.Values{"id": {upstreamID}}, &item); err != nil {
		return nil, err
	}
	return convertProduct(&item), nil
}

func (p *MoFangYunProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	params := url.Values{}
	if req.Name != "" {
		params.Set("name", req.Name)
	}
	if req.Region != "" {
		params.Set("region", req.Region)
	}
	if req.Zone != "" {
		params.Set("zone", req.Zone)
	}
	if req.Count > 0 {
		params.Set("count", strconv.Itoa(req.Count))
	}
	if req.Password != "" {
		params.Set("password", req.Password)
	}
	for k, v := range req.Extra {
		params.Set(k, fmt.Sprintf("%v", v))
	}
	var item MoFangYunInstance
	if err := p.call(ctx, actionCreateInstance, params, &item); err != nil {
		return nil, err
	}
	return convertInstance(&item), nil
}

func (p *MoFangYunProvider) ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error) {
	params := make(url.Values, len(filters))
	for k, v := range filters {
		params.Set(k, v)
	}
	var list []MoFangYunInstance
	if err := p.call(ctx, actionInstanceList, params, &list); err != nil {
		return nil, err
	}
	instances := make([]*model.StandardInstance, 0, len(list))
	for i := range list {
		instances = append(instances, convertInstance(&list[i]))
	}
	return instances, nil
}

func (p *MoFangYunProvider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
	var item MoFangYunInstance
	if err := p.call(ctx, actionInstanceInfo, url.Values{"id": {instanceID}}, &item); err != nil {
		return nil, err
	}
	return convertInstance(&item), nil
}

func (p *MoFangYunProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, actionStartInstance, url.Values{"id": {instanceID}}, nil)
}

func (p *MoFangYunProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	status := "stop"
	if force {
		status = "force_stop"
	}
	return p.call(ctx, actionStopInstance, url.Values{"id": {instanceID}, "status": {status}}, nil)
}

func (p *MoFangYunProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, actionRestartInstance, url.Values{"id": {instanceID}}, nil)
}

func (p *MoFangYunProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, actionDeleteInstance, url.Values{"id": {instanceID}}, nil)
}

func (p *MoFangYunProvider) ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error {
	params := url.Values{"id": {instanceID}}
	if specs == nil {
		return &upstream.ProviderError{Op: actionResizeInstance, Msg: "resize specs is nil"}
	}
	if specs.CPU > 0 {
		params.Set("cpu", strconv.Itoa(specs.CPU))
	}
	if specs.Memory > 0 {
		params.Set("memory", strconv.Itoa(specs.Memory))
	}
	if specs.Disk > 0 {
		params.Set("system_disk_size", strconv.Itoa(specs.Disk))
	}
	return p.call(ctx, actionResizeInstance, params, nil)
}

func (p *MoFangYunProvider) ListPools(ctx context.Context) ([]*upstream.StandardPool, error) {
	var nodes []MoFangYunNode
	if err := p.call(ctx, actionNodeList, nil, &nodes); err != nil {
		return nil, err
	}
	pools := make([]*upstream.StandardPool, 0, len(nodes))
	for i := range nodes {
		pools = append(pools, convertNode(&nodes[i]))
	}
	return pools, nil
}

func (p *MoFangYunProvider) GetAccountInfo(ctx context.Context) (*upstream.AccountInfo, error) {
	var acc MoFangYunAccount
	if err := p.call(ctx, actionAccountInfo, nil, &acc); err != nil {
		return nil, err
	}
	return &upstream.AccountInfo{
		TotalCPU:    acc.TotalCPU,
		TotalMemory: acc.TotalMemory,
		TotalDisk:   acc.TotalDisk,
		UsedCPU:     acc.UsedCPU,
		UsedMemory:  acc.UsedMemory,
		UsedDisk:    acc.UsedDisk,
		Balance:     acc.Balance,
		Currency:    acc.Currency,
	}, nil
}

// ===== 字段 / 状态映射 =====

// convertStatus 将魔方云实例状态映射为统一状态
func convertStatus(s string) model.StandardInstanceStatus {
	switch strings.ToLower(s) {
	case "running":
		return model.InstanceStatusRunning
	case "stopped":
		return model.InstanceStatusStopped
	case "creating":
		return model.InstanceStatusCreating
	case "restarting":
		return model.InstanceStatusRestarting
	case "deleting":
		return model.InstanceStatusDeleting
	case "deleted":
		return model.InstanceStatusDeleted
	default:
		return model.InstanceStatusError
	}
}

func convertProduct(m *MoFangYunProduct) *model.StandardProduct {
	return &model.StandardProduct{
		ProviderType: ProviderType,
		UpstreamID:   m.ID,
		Name:         m.Name,
		Specs: model.StandardProductSpec{
			CPU:       m.CPU,
			Memory:    m.Memory,
			Disk:      m.Disk,
			DiskType:  m.DiskType,
			Bandwidth: m.Bandwidth,
			OS:        m.OSType,
			Region:    m.Region,
			Zone:      m.Zone,
		},
		RawSpecs: map[string]interface{}{
			"cpu_num":         m.CPU,
			"memory_size":     m.Memory,
			"disk_size":       m.Disk,
			"disk_type":       m.DiskType,
			"bandwidth":       m.Bandwidth,
			"os_type":         m.OSType,
			"region":          m.Region,
			"zone":            m.Zone,
			"max_instances":   m.MaxInstances,
			"suggested_price": m.SuggestedPrice,
			"cost_price":      m.CostPrice,
			"status":          m.Status,
		},
		CostPrice: m.CostPrice,
		SalePrice: m.SuggestedPrice,
		Status:    m.Status,
	}
}

func convertInstance(m *MoFangYunInstance) *model.StandardInstance {
	return &model.StandardInstance{
		ProviderType: ProviderType,
		UpstreamID:   m.ID,
		Name:         m.Name,
		Specs: model.StandardProductSpec{
			CPU:       m.CPU,
			Memory:    m.Memory,
			Disk:      m.Disk,
			DiskType:  m.DiskType,
			Bandwidth: m.Bandwidth,
			Region:    m.Region,
			Zone:      m.Zone,
		},
		Status:    convertStatus(m.State),
		PrivateIP: m.PrivateIP,
		PublicIP:  m.PublicIP,
		Region:    m.Region,
		Zone:      m.Zone,
		CreatedAt: m.CreatedAt,
		ExpireAt:  m.ExpireAt,
		RawData: map[string]interface{}{
			"state":    m.State,
			"host":     m.Host,
			"os_image": m.OsImage,
		},
	}
}

func convertNode(n *MoFangYunNode) *upstream.StandardPool {
	return &upstream.StandardPool{
		ID:          n.ID,
		Name:        n.Name,
		Type:        "node",
		TotalCPU:    n.TotalCPU,
		TotalMemory: n.TotalMem,
		TotalDisk:   n.TotalDisk,
		UsedCPU:     n.UsedCPU,
		UsedMemory:  n.UsedMem,
		UsedDisk:    n.UsedDisk,
		Status:      n.Status,
	}
}
