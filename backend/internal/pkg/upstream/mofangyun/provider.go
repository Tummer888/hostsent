// Package mofangyun 为魔方云（DCIMCloud）上游提供商的适配器实现。
//
// 协议严格参考魔方财务源码 `app/common/logic/DcimCloud.php`（财务系统作为下游调用魔方云）：
//
//   - 管理员账号（account_type=admin）：
//     登录 POST {base}/v1/login?a=a（表单 username/password），响应体即 access token；
//     业务调用 {base}/v1{path}，请求头 `access-token: {token}`。
//   - 代理商账号（account_type=agent）：
//     登录 POST {base}/index.php?path=/token，响应 {status:"success", data:{token}}；
//     业务调用 {base}/index.php?path={path}。
//   - 业务请求为 application/x-www-form-urlencoded；GET 时参数拼 query。
//   - 响应为 JSON：含 error 字段即业务失败；2xx 即成功；401 时强制重新登录并重试一次。
//     token 按上游默认缓存 6 小时。
//   - 详情为扁平对象（id/hostname/mainip/ip/ipv6/osuser/rootpassword/port/status 等），
//     列表接口形如 {data:[...], ...}。
//
// 核心动作映射：
//   - 健康检查：登录成功即可达（等价 testLink）。
//   - 开机/关机/重启：POST /clouds/{id}/on | /off | /reboot（强制对应 /hardoff、/hard_reboot）。
//   - 详情/状态：GET /clouds/{id} 与 GET /clouds/{id}/status。
//   - 创建：POST /user 建用户 → GET /user/check 校验 → POST /clouds 下发云主机。
//   - 删除：DELETE /clouds/{id}（404 视为已删除）。
//   - 升配：参考源码 upgrade()——先关机（软关机失败转硬关机）再 PUT /clouds/{id}，完成后恢复原电源状态。
package mofangyun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostsent/backend/internal/pkg/model"
	"hostsent/backend/internal/pkg/upstream"
)

// ProviderType 魔方云提供商类型标识
const ProviderType = "mofangyun"

// tokenTTL 与财务源码一致：token 缓存 6 小时（dcim_cloud_token 缓存 21600 秒）。
const tokenTTL = 6 * time.Hour

func init() {
	upstream.GetProviderManager().RegisterFactory(ProviderType, func(cfg *upstream.ProviderConfig) upstream.Provider {
		return NewMoFangYunProvider(cfg)
	})
}

// MoFangYunProvider 魔方云适配器
type MoFangYunProvider struct {
	config *upstream.ProviderConfig
	client *http.Client

	tokenMu    sync.Mutex
	token      string
	tokenUntil time.Time
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

// isAgent 账号类型：agent=代理商（/index.php?path= 入口），其余为管理员（/v1 入口）。
func (p *MoFangYunProvider) isAgent() bool { return p.config.AccountType == "agent" }

// baseURL 组合协议、地址与端口，去掉尾部斜杠。
func (p *MoFangYunProvider) baseURL() (string, error) {
	host := strings.TrimRight(p.config.APIEndpoint, "/")
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if host == "" {
		return "", &upstream.ProviderError{Op: "request", Msg: "接口地址为空"}
	}
	scheme := "http"
	if p.config.Secure {
		scheme = "https"
	}
	if p.config.Port != "" {
		host = host + ":" + strings.TrimPrefix(p.config.Port, ":")
	}
	return scheme + "://" + host, nil
}

// actionURL 按账号类型构造业务调用地址，path 形如 "/clouds/1/on"（可含 query）。
func (p *MoFangYunProvider) actionURL(base, path string) string {
	if p.isAgent() {
		return base + "/index.php?path=" + strings.TrimLeft(path, "/")
	}
	return base + "/v1" + path
}

// login 获取 access token，带本地缓存；force 时强制重新登录。
// 参考源码 login()：管理员账号响应体即 token；代理商账号响应 {status:"success",data:{token}}。
func (p *MoFangYunProvider) login(ctx context.Context, force bool) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()
	if !force && p.token != "" && time.Now().Before(p.tokenUntil) {
		return p.token, nil
	}
	base, err := p.baseURL()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("username", p.config.APIKey)
	form.Set("password", p.config.APISecret)

	var loginURL string
	if p.isAgent() {
		loginURL = base + "/index.php?path=" + strings.TrimLeft("/token", "/")
	} else {
		loginURL = base + "/v1/login?a=a"
	}
	body, status, err := p.httpPost(ctx, loginURL, form, "")
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound {
		return "", &upstream.ProviderError{
			Op:         "login",
			StatusCode: status,
			Msg:        "无法连接魔方云管理系统，请检查魔方云访问地址是否填写正确",
		}
	}
	if p.isAgent() {
		var shell struct {
			Status string `json:"status"`
			Msg    string `json:"msg"`
			Data   struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &shell); err != nil || shell.Data.Token == "" {
			return "", &upstream.ProviderError{Op: "login", StatusCode: status, Msg: firstNonEmpty(shell.Msg, "魔方云登录失败：响应缺少 token")}
		}
		p.token = strings.Trim(shell.Data.Token, "\"")
	} else {
		// 管理员登录响应体即 token 字符串（可能带引号）
		p.token = strings.Trim(strings.TrimSpace(string(body)), "\"")
		if p.token == "" {
			return "", &upstream.ProviderError{Op: "login", StatusCode: status, Msg: "魔方云登录失败：响应为空"}
		}
	}
	p.tokenUntil = time.Now().Add(tokenTTL)
	return p.token, nil
}

// httpPost 发送表单 POST；bearer 传入时设置 access-token 请求头。
func (p *MoFangYunProvider) httpPost(ctx context.Context, rawURL string, form url.Values, token string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "WHMCS") // 与源码 basecurl 一致
	if token != "" {
		req.Header.Set("access-token", token)
	}
	return p.httpDo(req)
}

// httpDo 执行请求并返回响应体与状态码。
func (p *MoFangYunProvider) httpDo(req *http.Request) ([]byte, int, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: "request", Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, &upstream.ProviderError{Op: "request", Err: err}
	}
	return body, resp.StatusCode, nil
}

// call 调用魔方云业务接口：自动登录、401 重登录重试一次、解析 error 字段。
// out 非 nil 时将整个响应体 JSON 解析到 out。
func (p *MoFangYunProvider) call(ctx context.Context, op, method, path string, form url.Values, out interface{}) error {
	base, err := p.baseURL()
	if err != nil {
		return err
	}
	token, err := p.login(ctx, false)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	body, status, err := p.doWithToken(ctx, base, token, method, path, form)
	if err != nil {
		return &upstream.ProviderError{Op: op, Err: err}
	}
	// 与源码 curl() 一致：401 强制重新登录后重试一次。
	if status == http.StatusUnauthorized {
		token, err = p.login(ctx, true)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
		body, status, err = p.doWithToken(ctx, base, token, method, path, form)
		if err != nil {
			return &upstream.ProviderError{Op: op, Err: err}
		}
	}
	if status < 200 || status >= 300 {
		return &upstream.ProviderError{Op: op, StatusCode: status, Msg: "请求失败,HTTP状态码:" + strconv.Itoa(status)}
	}
	// 与源码 basecurl 一致：响应含 error 字段即业务失败。
	var probe struct {
		Error interface{} `json:"error"`
	}
	if json.Unmarshal(body, &probe) == nil && probe.Error != nil {
		return &upstream.ProviderError{Op: op, Code: status, Msg: fmt.Sprintf("%v", probe.Error)}
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return &upstream.ProviderError{Op: op, Err: fmt.Errorf("解析魔方云响应失败: %w", err)}
		}
	}
	return nil
}

// doWithToken 以给定 token 执行一次业务调用。GET 的表单参数转 query，其余进请求体。
func (p *MoFangYunProvider) doWithToken(ctx context.Context, base, token, method, path string, form url.Values) ([]byte, int, error) {
	rawURL := p.actionURL(base, path)
	if form == nil {
		form = url.Values{}
	}
	method = strings.ToUpper(method)
	var req *http.Request
	var err error
	if method == http.MethodGet {
		if enc := form.Encode(); enc != "" {
			sep := "?"
			if strings.Contains(rawURL, "?") {
				sep = "&"
			}
			rawURL += sep + enc
		}
		req, err = http.NewRequestWithContext(ctx, method, rawURL, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return nil, 0, &upstream.ProviderError{Op: path, Err: err}
	}
	req.Header.Set("User-Agent", "WHMCS")
	req.Header.Set("access-token", token)
	return p.httpDo(req)
}

// HealthCheck 连通性测试：登录成功即视为可达（等价源码 testLink）。
func (p *MoFangYunProvider) HealthCheck(ctx context.Context) error {
	_, err := p.login(ctx, true)
	return err
}

// ListProducts 魔方云无商品目录概念：商品由计费系统定义，
// 规格在开通时通过 CreateInstance 的 Extra（configoptions）映射为 /clouds 参数。
func (p *MoFangYunProvider) ListProducts(ctx context.Context) ([]*model.StandardProduct, error) {
	return nil, p.notSupported("ListProducts", "魔方云无商品目录，商品由计费系统（HostSent 产品模块）定义，规格通过创建实例的 Extra 映射")
}

// GetProduct 同 ListProducts：魔方云不提供商品目录。
func (p *MoFangYunProvider) GetProduct(ctx context.Context, upstreamID string) (*model.StandardProduct, error) {
	return nil, p.notSupported("GetProduct", "魔方云无商品目录，规格通过创建实例的 Extra 映射")
}

// CreateInstance 开通云主机。
// 参考源码 createAccount()：先确保魔方云用户（POST /user + GET /user/check），
// 再 POST /clouds 下发实例。规格参数通过 req.Extra 传递，键名与魔方云一致
// （area/node/os/cpu/memory/system_disk_size/data_disk_size/network_type/bw/flow_limit/
// flow_way/ip_num/backup_num/snap_num/nat_acl_limit/nat_web_limit/type/ipv6_num/port/
// node_group/ip_group/node_priority/resource_package 等），同时支持包一层
// Extra["configoptions"]（与财务 configoptions 同形）。
func (p *MoFangYunProvider) CreateInstance(ctx context.Context, req *model.CreateInstanceRequest) (*model.StandardInstance, error) {
	opts := collectOptions(req.Extra)

	form := url.Values{}
	// 用户：优先使用传入的魔方云用户 ID，否则按源码创建用户。
	clientID := firstNonEmpty(opts["client"], opts["client_id"], opts["user_id"])
	if clientID == "" {
		id, err := p.ensureCloudUser(ctx, opts)
		if err != nil {
			return nil, err
		}
		clientID = id
	}
	form.Set("client", clientID)
	form.Set("hostname", firstNonEmpty(req.Name, opts["hostname"]))
	rootpass := firstNonEmpty(req.Password, opts["rootpass"], randStr(8))
	form.Set("rootpass", rootpass)

	// 规格与可配置项透传（保留创建时必填校验交给上游判断）。
	for _, key := range cloudOptionKeys {
		if v, ok := opts[key]; ok && v != "" {
			form.Set(key, v)
		}
	}
	// 数据盘：other_data_disk 传数组，需按表单数组语法展开。
	expandOtherDataDisk(form, req.Extra)

	var created struct {
		ID int64 `json:"id"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "/clouds", form, &created); err != nil {
		return nil, err
	}
	if created.ID == 0 {
		return nil, &upstream.ProviderError{Op: "CreateInstance", Msg: "魔方云开通请求已提交但未返回云主机 ID"}
	}
	inst, err := p.GetInstance(ctx, strconv.FormatInt(created.ID, 10))
	if err != nil {
		// 详情失败不回滚开通，仅返回最小可用信息。
		return &model.StandardInstance{
			ProviderType: ProviderType,
			UpstreamID:   strconv.FormatInt(created.ID, 10),
			Name:         firstNonEmpty(req.Name, opts["hostname"]),
			Status:       model.InstanceStatusCreating,
		}, nil
	}
	return inst, nil
}

// ListInstances 拉取魔方云云主机列表。filters 支持 per_page/page/status/area/node。
func (p *MoFangYunProvider) ListInstances(ctx context.Context, filters map[string]string) ([]*model.StandardInstance, error) {
	form := url.Values{}
	form.Set("per_page", firstNonEmpty(filters["per_page"], "100"))
	form.Set("page", firstNonEmpty(filters["page"], "1"))
	for _, key := range []string{"status", "area", "node", "keywords"} {
		if v := filters[key]; v != "" {
			form.Set(key, v)
		}
	}
	var rows []map[string]interface{}
	if err := p.call(ctx, "ListInstances", http.MethodGet, "/clouds", form, &rows); err != nil {
		return nil, err
	}
	instances := make([]*model.StandardInstance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, buildInstance(row))
	}
	return instances, nil
}

// GetInstance 拉取单台云主机详情并映射标准实例。
func (p *MoFangYunProvider) GetInstance(ctx context.Context, instanceID string) (*model.StandardInstance, error) {
	var detail map[string]interface{}
	if err := p.call(ctx, "GetInstance", http.MethodGet, "/clouds/"+instanceID, nil, &detail); err != nil {
		return nil, err
	}
	inst := buildInstance(detail)
	// 状态以 /clouds/{id}/status 为准（参考源码 getCloudStatus）。
	var st struct {
		Status   string `json:"status"`
		TaskName string `json:"task_name"`
	}
	if err := p.call(ctx, "GetInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil && st.Status != "" {
		inst.Status = mapCloudStatus(st.Status)
		if st.TaskName != "" {
			if inst.RawData == nil {
				inst.RawData = map[string]interface{}{}
			}
			inst.RawData["task_name"] = st.TaskName
		}
	}
	return inst, nil
}

// StartInstance 开机：POST /clouds/{id}/on
func (p *MoFangYunProvider) StartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "StartInstance", http.MethodPost, "/clouds/"+instanceID+"/on", url.Values{}, nil)
}

// StopInstance 关机：POST /clouds/{id}/off；force 时硬关机 /hardoff。
func (p *MoFangYunProvider) StopInstance(ctx context.Context, instanceID string, force bool) error {
	action := "/off"
	if force {
		action = "/hardoff"
	}
	return p.call(ctx, "StopInstance", http.MethodPost, "/clouds/"+instanceID+action, url.Values{}, nil)
}

// RestartInstance 重启：POST /clouds/{id}/reboot；force 时硬重启 /hard_reboot。
func (p *MoFangYunProvider) RestartInstance(ctx context.Context, instanceID string) error {
	return p.call(ctx, "RestartInstance", http.MethodPost, "/clouds/"+instanceID+"/reboot", url.Values{}, nil)
}

// DeleteInstance 删除云主机：DELETE /clouds/{id}；404 视为已删除（与源码 terminate 一致）。
func (p *MoFangYunProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	base, err := p.baseURL()
	if err != nil {
		return err
	}
	token, err := p.login(ctx, false)
	if err != nil {
		return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
	}
	_, status, err := p.doWithToken(ctx, base, token, http.MethodDelete, "/clouds/"+instanceID, nil)
	if err != nil {
		return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
	}
	if status == http.StatusUnauthorized {
		token, err = p.login(ctx, true)
		if err != nil {
			return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
		}
		_, status, err = p.doWithToken(ctx, base, token, http.MethodDelete, "/clouds/"+instanceID, nil)
		if err != nil {
			return &upstream.ProviderError{Op: "DeleteInstance", Err: err}
		}
	}
	if status == http.StatusNotFound || (status >= 200 && status < 300) {
		return nil
	}
	return &upstream.ProviderError{Op: "DeleteInstance", StatusCode: status, Msg: "删除失败,HTTP状态码:" + strconv.Itoa(status)}
}

// ResizeInstance 升降配：参考源码 upgrade()——先关机（硬关机兜底），
// 再 PUT /clouds/{id} 修改 cpu/memory 等参数，完成后恢复原电源状态。
func (p *MoFangYunProvider) ResizeInstance(ctx context.Context, instanceID string, specs *model.StandardProductSpec) error {
	if specs == nil {
		return &upstream.ProviderError{Op: "ResizeInstance", Msg: "缺少目标规格"}
	}
	// 记录原电源状态
	var st struct {
		Status string `json:"status"`
	}
	wasOn := false
	if err := p.call(ctx, "ResizeInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil {
		wasOn = st.Status == "on" || st.Status == "task" || st.Status == "wait_reboot"
	}
	// 关机并等待（软关机 10×2s，未停则硬关机再等 5×2s）
	if wasOn {
		if err := p.StopInstance(ctx, instanceID, false); err != nil {
			return err
		}
		if !p.waitStatus(ctx, instanceID, "off", 10) {
			_ = p.StopInstance(ctx, instanceID, true)
			if !p.waitStatus(ctx, instanceID, "off", 5) {
				return &upstream.ProviderError{Op: "ResizeInstance", Msg: "等待云主机关机超时，已取消升降配"}
			}
		}
	}
	form := url.Values{}
	if specs.CPU > 0 {
		form.Set("cpu", strconv.Itoa(specs.CPU))
	}
	if specs.Memory > 0 {
		form.Set("memory", strconv.Itoa(specs.Memory))
	}
	for _, key := range cloudOptionKeys {
		if v := specs.Extra[key]; v != nil && fmt.Sprintf("%v", v) != "" {
			form.Set(key, fmt.Sprintf("%v", v))
		}
	}
	if err := p.call(ctx, "ResizeInstance", http.MethodPut, "/clouds/"+instanceID, form, nil); err != nil {
		return err
	}
	if wasOn {
		if err := p.StartInstance(ctx, instanceID); err != nil {
			return &upstream.ProviderError{Op: "ResizeInstance", Msg: "配置已修改，但开机恢复失败：" + err.Error()}
		}
	}
	return nil
}

// ListPools 拉取区域列表：GET /areas?sort=asc&list_type=all（参考源码 getArea）。
func (p *MoFangYunProvider) ListPools(ctx context.Context) ([]*upstream.StandardPool, error) {
	// 列表接口常规为 {data:[...]} 分页包裹；兼容裸数组形态。
	var wrapped struct {
		Data []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	err := p.call(ctx, "ListPools", http.MethodGet, "/areas", url.Values{"sort": {"asc"}, "list_type": {"all"}}, &wrapped)
	if err != nil {
		var areas []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err2 := p.call(ctx, "ListPools", http.MethodGet, "/areas", url.Values{"sort": {"asc"}, "list_type": {"all"}}, &areas); err2 != nil {
			return nil, err
		}
		for _, a := range areas {
			wrapped.Data = append(wrapped.Data, struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			}{a.ID, a.Name})
		}
	}
	pools := make([]*upstream.StandardPool, 0, len(wrapped.Data))
	for _, a := range wrapped.Data {
		pools = append(pools, &upstream.StandardPool{
			ID:     strconv.FormatInt(a.ID, 10),
			Name:   a.Name,
			Type:   "region",
			Status: "active",
		})
	}
	return pools, nil
}

// GetAccountInfo 魔方云为资源管理系统，无账户余额概念。
func (p *MoFangYunProvider) GetAccountInfo(ctx context.Context) (*upstream.AccountInfo, error) {
	return nil, p.notSupported("GetAccountInfo", "魔方云为资源管理系统，不提供账户余额信息")
}

// ensureCloudUser 确保魔方云侧用户存在并返回用户 ID（参考源码 createAccount 的建用户逻辑）。
func (p *MoFangYunProvider) ensureCloudUser(ctx context.Context, opts map[string]string) (string, error) {
	username := firstNonEmpty(opts["cloud_username"], p.config.UserPrefix+firstNonEmpty(opts["username"], fmt.Sprintf("hs%d", time.Now().Unix())))
	password := firstNonEmpty(opts["cloud_password"], randStr(8))
	userForm := url.Values{}
	userForm.Set("username", username)
	userForm.Set("email", firstNonEmpty(opts["email"], username+"@hostsent.local"))
	userForm.Set("status", "1")
	userForm.Set("real_name", firstNonEmpty(opts["real_name"], username))
	userForm.Set("password", password)
	if p.isAgent() {
		// 代理商账号建用户必须指定资源包（与源码一致）。
		rid := opts["resource_package"]
		if rid == "" {
			return "", &upstream.ProviderError{Op: "CreateInstance", Msg: "代理商账号创建用户需要配置资源包（resource_package）"}
		}
		userForm.Set("rid", rid)
	}
	if err := p.call(ctx, "CreateInstance", http.MethodPost, "/user", userForm, nil); err != nil {
		// 用户可能已存在，继续走 /user/check 校验。
		_ = err
	}
	var checked struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := p.call(ctx, "CreateInstance", http.MethodGet, "/user/check", url.Values{"username": {username}}, &checked); err != nil {
		return "", err
	}
	if checked.Data.ID == 0 {
		return "", &upstream.ProviderError{Op: "CreateInstance", Msg: firstNonEmpty(checked.Msg, "魔方云用户创建失败")}
	}
	return strconv.FormatInt(checked.Data.ID, 10), nil
}

// waitStatus 轮询云主机状态直到达到期望值或超时。
func (p *MoFangYunProvider) waitStatus(ctx context.Context, instanceID, want string, tries int) bool {
	for i := 0; i < tries; i++ {
		time.Sleep(2 * time.Second)
		var st struct {
			Status string `json:"status"`
		}
		if err := p.call(ctx, "ResizeInstance", http.MethodGet, "/clouds/"+instanceID+"/status", nil, &st); err == nil && st.Status == want {
			return true
		}
	}
	return false
}

// cloudOptionKeys 与魔方云 /clouds 参数对齐的可配置项（键名与财务 configoptions 一致）。
var cloudOptionKeys = []string{
	"area", "node", "os", "cpu", "memory", "node_group", "ip_group", "node_priority",
	"nat_acl_limit", "nat_web_limit", "advanced_cpu", "advanced_bw",
	"system_disk_size", "network_type", "system_read_bytes_sec", "system_write_bytes_sec",
	"system_read_iops_sec", "system_write_iops_sec", "data_read_bytes_sec", "data_write_bytes_sec",
	"data_read_iops_sec", "data_write_iops_sec", "vpc", "vpc_name", "traffic_type",
	"in_bw", "out_bw", "ip_num", "traffic_quota", "backup_num", "snap_num", "flow_limit",
	"bw", "flow_way", "bind_mac", "cpu_limit", "port", "cpu_model", "rid", "ipv6_num",
	"type", "niccard", "link_clone", "reset_flow_day", "store",
}

// collectOptions 归一化创建参数：优先取 Extra["configoptions"]（与财务 configoptions 同形），
// 再合并 Extra 顶层标量键。
func collectOptions(extra map[string]interface{}) map[string]string {
	opts := map[string]string{}
	set := func(k string, v interface{}) {
		switch t := v.(type) {
		case string:
			if t != "" {
				opts[k] = t
			}
		case float64:
			opts[k] = strconv.FormatFloat(t, 'f', -1, 64)
		case json.Number:
			opts[k] = t.String()
		case bool:
			if t {
				opts[k] = "1"
			} else {
				opts[k] = "0"
			}
		case nil:
		default:
			opts[k] = fmt.Sprintf("%v", t)
		}
	}
	if raw, ok := extra["configoptions"]; ok {
		if m, ok := raw.(map[string]interface{}); ok {
			for k, v := range m {
				set(k, v)
			}
		}
	}
	for k, v := range extra {
		if k == "configoptions" || k == "other_data_disk" {
			continue
		}
		set(k, v)
	}
	// flow_way → traffic_type（1 进 / 2 出 / 3 进出汇总），与源码一致。
	if opts["traffic_type"] == "" && opts["flow_way"] != "" {
		switch opts["flow_way"] {
		case "in":
			opts["traffic_type"] = "1"
		case "out":
			opts["traffic_type"] = "2"
		default:
			opts["traffic_type"] = "3"
		}
	}
	// flow_limit → traffic_quota 别名。
	if opts["traffic_quota"] == "" && opts["flow_limit"] != "" {
		opts["traffic_quota"] = opts["flow_limit"]
	}
	return opts
}

// expandOtherDataDisk 将 other_data_disk（[{size, store}]）展开为表单数组语法。
func expandOtherDataDisk(form url.Values, extra map[string]interface{}) {
	raw, ok := extra["other_data_disk"]
	if !ok {
		if co, ok := extra["configoptions"].(map[string]interface{}); ok {
			raw, ok = co["data_disk_size"]
		}
		if !ok {
			return
		}
		switch t := raw.(type) {
		case string:
			if t != "" {
				form.Set("other_data_disk[0][size]", t)
			}
			return
		case float64:
			form.Set("other_data_disk[0][size]", strconv.FormatFloat(t, 'f', -1, 64))
			return
		default:
			return
		}
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := m["size"]; ok {
			form.Set(fmt.Sprintf("other_data_disk[%d][size]", i), fmt.Sprintf("%v", v))
		}
		if v, ok := m["store"]; ok {
			form.Set(fmt.Sprintf("other_data_disk[%d][store]", i), fmt.Sprintf("%v", v))
		}
	}
}

// buildInstance 将魔方云云主机详情（扁平对象）映射为标准实例。
func buildInstance(detail map[string]interface{}) *model.StandardInstance {
	id := jsonStr(detail, "id")
	inst := &model.StandardInstance{
		ProviderType: ProviderType,
		UpstreamID:   id,
		Name:         jsonStr(detail, "hostname"),
		PublicIP:     jsonStr(detail, "mainip"),
		Status:       model.InstanceStatusRunning,
		RawData:      detail,
	}
	// 附加 IP：ip[] 为公网列表，ipv6[] 为 v6 列表；主 IP 之外的第一个不作为私有 IP 处理，
	// 保留在 RawData 中即可。
	if v := jsonStr(detail, "osuser"); v != "" {
		inst.RawData["osuser"] = v
	}
	if v := jsonStr(detail, "port"); v != "" {
		inst.RawData["port"] = v
	}
	// 状态：详情里的 status 字段（若有）
	if s := jsonStr(detail, "status"); s != "" {
		inst.Status = mapCloudStatus(s)
	}
	return inst
}

// mapCloudStatus 将魔方云状态映射为标准实例状态（参考源码 getCloudStatus）。
func mapCloudStatus(s string) model.StandardInstanceStatus {
	switch s {
	case "on":
		return model.InstanceStatusRunning
	case "off", "suspend", "paused":
		return model.InstanceStatusStopped
	case "wait_reboot", "task":
		return model.InstanceStatusRestarting
	case "cold_migrate", "hot_migrate":
		return model.InstanceStatusRunning
	default:
		return model.InstanceStatusStopped
	}
}

// jsonStr 从 map 取字符串值（兼容 json.Number / float64）。
func jsonStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// notSupported 返回能力不支持错误。
func (p *MoFangYunProvider) notSupported(op, msg string) error {
	return &upstream.ProviderError{Op: op, Msg: msg}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// randStr 生成随机小写字母数字串（用于魔方云用户/密码），与源码 randStr 等价的简化实现。
func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
