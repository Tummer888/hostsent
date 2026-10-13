package dcimcloud

import (
	"context"
	"net/http"
	"net/url"

	"hostsent/backend/internal/pkg/upstream"
)

// NAT 机型访问地址读取（NatReader）实现。
//
// 协议取自面板源码 app/common/logic/DcimCloud.php 与对应模板：
//   - GET /clouds/{id}/nat_acl?list_type=all → {"data":{"nat_host_ip":"1.2.3.4","data":[{name,ext_port,int_port,protocol,id}]}}
//   - GET /clouds/{id}/nat_web?list_type=all → {"data":{"nat_host_ip":"1.2.3.4","data":[{domain,ext_port,int_port,id}]}}
//
// 关键点：nat_host_ip 在**外层 data 上**，不在列表项里（模板渲染 {nat_host_ip}:{vo.ext_port}
// 就是这个结构）。因此解析必须同时取外层 IP 与内层列表，逐项拼成 HostIP。
//
// protocol 是数字枚举，模板里按 1=tcp / 2=udp / 其它=tcp,udp 渲染（源码同一套判定），
// 这里直接归一成可读文本，避免把数字枚举泄漏给前端。
//
// 面板侧的调用门禁（DcimCloud.php:147-153）：只有 type ∈ {空, host, lightHost} 且
// nat_*_limit >= 0 时才挂 NAT 入口。本实现不做这个门禁——调用方只在确认是 NAT 机型后
// 才会调到这里；面板对无关实例返回空列表也不会报错。

// natProtocolText 协议枚举 → 可读文本（与面板模板的渲染分支一致）。
func natProtocolText(code int) string {
	switch code {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	default:
		return "tcp,udp"
	}
}

// natAclResponse NAT 端口转发列表响应。
type natAclResponse struct {
	Data struct {
		NatHostIP string `json:"nat_host_ip"`
		Data      []struct {
			ID       jsonNumberish `json:"id"`
			Name     string        `json:"name"`
			ExtPort  int           `json:"ext_port"`
			IntPort  int           `json:"int_port"`
			Protocol int           `json:"protocol"`
		} `json:"data"`
	} `json:"data"`
}

// natWebResponse 共享建站列表响应（字段比端口转发少一个 protocol）。
type natWebResponse struct {
	Data struct {
		NatHostIP string `json:"nat_host_ip"`
		Data      []struct {
			ID      jsonNumberish `json:"id"`
			Domain  string        `json:"domain"`
			ExtPort int           `json:"ext_port"`
			IntPort int           `json:"int_port"`
		} `json:"data"`
	} `json:"data"`
}

// jsonNumberish 兼容面板把 id 返回成数字或字符串两种形态。
// 面板其它接口（如创建实例的 id）已实测为字符串，这里两种都收。
type jsonNumberish string

func (n *jsonNumberish) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		*n = jsonNumberish(s[1 : len(s)-1])
		return nil
	}
	*n = jsonNumberish(s)
	return nil
}

func (n jsonNumberish) String() string { return string(n) }

// ListNatForwards 列出实例的 NAT 端口转发规则（无规则时返回空切片，不是错误）。
func (p *DCIMCloudProvider) ListNatForwards(ctx context.Context, providerInstanceID string) ([]upstream.NatForwardInfo, error) {
	if providerInstanceID == "" {
		return nil, nil
	}
	var resp natAclResponse
	if err := p.call(ctx, "ListNatForwards", http.MethodGet,
		"/clouds/"+providerInstanceID+"/nat_acl",
		url.Values{"list_type": {"all"}}, &resp); err != nil {
		return nil, err
	}
	out := make([]upstream.NatForwardInfo, 0, len(resp.Data.Data))
	for _, item := range resp.Data.Data {
		out = append(out, upstream.NatForwardInfo{
			ID:           item.ID.String(),
			Name:         item.Name,
			HostIP:       resp.Data.NatHostIP,
			ExternalPort: item.ExtPort,
			InternalPort: item.IntPort,
			Protocol:     natProtocolText(item.Protocol),
		})
	}
	return out, nil
}

// ListNatWebs 列出实例的共享建站规则（无规则时返回空切片，不是错误）。
func (p *DCIMCloudProvider) ListNatWebs(ctx context.Context, providerInstanceID string) ([]upstream.NatWebInfo, error) {
	if providerInstanceID == "" {
		return nil, nil
	}
	var resp natWebResponse
	if err := p.call(ctx, "ListNatWebs", http.MethodGet,
		"/clouds/"+providerInstanceID+"/nat_web",
		url.Values{"list_type": {"all"}}, &resp); err != nil {
		return nil, err
	}
	out := make([]upstream.NatWebInfo, 0, len(resp.Data.Data))
	for _, item := range resp.Data.Data {
		out = append(out, upstream.NatWebInfo{
			ID:           item.ID.String(),
			Domain:       item.Domain,
			HostIP:       resp.Data.NatHostIP,
			ExternalPort: item.ExtPort,
			InternalPort: item.IntPort,
		})
	}
	return out, nil
}

// FormatNatForward 把一条转发规则拼成客户可直接使用的地址 "IP:端口"。
// 实现放在 upstream 包（FormatNatForwardAddr），因为服务层也要用同一口径；
// 这里保留一个薄封装，便于适配器内部与测试的调用习惯。
func FormatNatForward(hostIP string, externalPort int) string {
	return upstream.FormatNatForwardAddr(hostIP, externalPort)
}
