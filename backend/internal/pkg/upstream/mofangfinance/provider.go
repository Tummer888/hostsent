// Package mofangfinance 为魔方财务（JMF Finance）上游提供商的适配器实现。
//
// 魔方财务作为「下游」第三方对接系统时，入口为 `ZjmfFinanceApiController`：
//   - createApi：POST /admin/zjmf_finance_api （注册/创建第三方接口访问凭证）
//   - inputProduct：POST /admin/zjmf_finance_api/inputproduct （导入上游商品）
//
// 与魔方云（资源管理型）不同，魔方财务是账单/结算型系统，不暴露云主机生命周期操作
// （StartInstance/ResizeInstance 等），因此这些能力统一返回 notSupported 错误；
// 真正的对接能力集中在 CreateAPI 与 ImportProducts 两个推送型操作上。
package mofangfinance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方财务提供商类型标识
const ProviderType = "mofangfinance"

func init() {
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewMoFangFinanceProvider(cfg)
	})
}

// MoFangFinanceProvider 魔方财务适配器
type MoFangFinanceProvider struct {
	config *upstream.ProviderConfig
	client *http.Client
}

// NewMoFangFinanceProvider 按配置创建魔方财务适配器实例
func NewMoFangFinanceProvider(config *upstream.ProviderConfig) *MoFangFinanceProvider {
	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &MoFangFinanceProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// GetType 返回提供商类型标识
func (p *MoFangFinanceProvider) GetType() string { return ProviderType }

// GetName 返回提供商名称
func (p *MoFangFinanceProvider) GetName() string { return "魔方财务" }

// baseURL 去掉尾部斜杠的接口地址
func (p *MoFangFinanceProvider) baseURL() (string, error) {
	endpoint := strings.TrimRight(p.config.APIEndpoint, "/")
	if endpoint == "" {
		return "", &upstream.ProviderError{Op: "request", Msg: "接口地址为空"}
	}
	return endpoint, nil
}

// HealthCheck 连通性测试：向魔方财务接口发起一次 POST，确认接口地址可达并验证响应壳。
// 后续联编核对 createApi 的真实返回后，可进一步改为带鉴权的写/读调用。
func (p *MoFangFinanceProvider) HealthCheck(ctx context.Context) error {
	endpoint, err := p.baseURL()
	if err != nil {
		return err
	}
	// 任一鉴权/参数错误都说明目标可达；仅网络不可达/超时/服务端 5xx 视为失败。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &upstream.ProviderError{Op: "HealthCheck", Err: err}
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return &upstream.ProviderError{Op: "HealthCheck", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return &upstream.ProviderError{Op: "HealthCheck", StatusCode: resp.StatusCode}
	}
	return nil
}

// doPost 向魔方财务指定路径发起 JSON POST，解析统一响应壳，返回业务数据。
// path 形如 "/admin/zjmf_finance_api"；data 为请求体；out 为 data 字段的解析目标。
func (p *MoFangFinanceProvider) doPost(ctx context.Context, path string, params map[string]interface{}, out interface{}) error {
	endpoint, err := p.baseURL()
	if err != nil {
		return err
	}
	body, err := json.Marshal(params)
	if err != nil {
		return &upstream.ProviderError{Op: "doPost", Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(body))
	if err != nil {
		return &upstream.ProviderError{Op: "doPost", Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	// 魔方财务 API 鉴权：优先用配置的 API 密钥作为 Bearer 凭证；无则退化为 basic。
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	} else if p.config.APISecret != "" {
		req.SetBasicAuth(p.config.APIKey, p.config.APISecret)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return &upstream.ProviderError{Op: "doPost", Err: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &upstream.ProviderError{Op: "doPost", Err: err}
	}
	if resp.StatusCode >= 400 {
		return &upstream.ProviderError{
			Op:         "doPost",
			StatusCode: resp.StatusCode,
			Msg:        strings.TrimSpace(string(respBody)),
		}
	}
	var shell MoFangFinanceResp[interface{}]
	if err := json.Unmarshal(respBody, &shell); err != nil {
		// 部分接口返回裸 data（未包裹 status），直接尝试解到 out。
		if out != nil {
			_ = json.Unmarshal(respBody, &out)
		}
		return nil
	}
	if shell.Status >= 400 || shell.Code >= 400 {
		return &upstream.ProviderError{Op: "doPost", StatusCode: shell.Status, Msg: shell.Msg}
	}
	if out != nil && shell.Data != nil {
		dataJSON, err := json.Marshal(shell.Data)
		if err == nil {
			_ = json.Unmarshal(dataJSON, &out)
		}
	}
	return nil
}

// CreateAPI 创建魔方财务接口（注册第三方 API 访问凭证），对应 createApi。
// 返回创建成功后的凭证信息（含 finance api id，供后续 ImportProducts 使用）。
func (p *MoFangFinanceProvider) CreateAPI(ctx context.Context, params CreateAPIParams) (*MoFangFinanceApiInfo, error) {
	req := map[string]interface{}{
		"name":               params.Name,
		"hostname":           params.Hostname,
		"username":           params.Username,
		"password":           params.Password,
		"des":                params.Des,
		"type":               params.Type,
		"contact_way":        params.ContactWay,
		"auto_reply_switch":  params.AutoReplySwitch,
		"auto_reply_account": params.AutoReplyAccount,
		"sync_stock":         params.SyncStock,
	}
	var out MoFangFinanceApiInfo
	if err := p.doPost(ctx, "/admin/zjmf_finance_api", req, &out); err != nil {
		return nil, &upstream.ProviderError{Op: "CreateAPI", Err: err}
	}
	return &out, nil
}

// ImportProducts 将上游商品导入魔方财务，对应 inputProduct。
func (p *MoFangFinanceProvider) ImportProducts(ctx context.Context, params InputProductParams) error {
	req := map[string]interface{}{
		"gid":                  params.GroupID,
		"productnames":         params.ProductNames,
		"upstream_price_value": params.UpstreamPriceVal,
		"ptype":                params.Ptype,
		"zjmf_finance_api_id":  params.ZjmfFinanceAPIID,
		"type":                 params.ModuleTypes,
	}
	if params.Rate != 0 {
		req["rate"] = params.Rate
	}
	return p.doPost(ctx, "/admin/zjmf_finance_api/inputproduct", req, nil)
}

// notSupported 返回「账单系统不支持该资源生命周期操作」错误。
func (p *MoFangFinanceProvider) notSupported(op string) error {
	return &upstream.ProviderError{
		Op:  op,
		Msg: fmt.Sprintf("魔方财务为账单/结算系统，不支持资源生命周期操作 %s；对接请使用 CreateAPI/ImportProducts", op),
	}
}

func (p *MoFangFinanceProvider) ListProducts(ctx context.Context) ([]*model.StandardProduct, error) {
	return nil, p.notSupported("ListProducts")
}

func (p *MoFangFinanceProvider) GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error) {
	return nil, p.notSupported("GetProduct")
}

func (p *MoFangFinanceProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	return nil, p.notSupported("CreateInstance")
}

func (p *MoFangFinanceProvider) ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error) {
	return nil, p.notSupported("ListInstances")
}

func (p *MoFangFinanceProvider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
	return nil, p.notSupported("GetInstance")
}

func (p *MoFangFinanceProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.notSupported("StartInstance")
}

func (p *MoFangFinanceProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	return p.notSupported("StopInstance")
}

func (p *MoFangFinanceProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.notSupported("RestartInstance")
}

func (p *MoFangFinanceProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	return p.notSupported("DeleteInstance")
}

func (p *MoFangFinanceProvider) ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error {
	return p.notSupported("ResizeInstance")
}

func (p *MoFangFinanceProvider) ListPools(ctx context.Context) ([]*upstream.StandardPool, error) {
	return nil, p.notSupported("ListPools")
}

func (p *MoFangFinanceProvider) GetAccountInfo(ctx context.Context) (*upstream.AccountInfo, error) {
	return nil, p.notSupported("GetAccountInfo")
}
