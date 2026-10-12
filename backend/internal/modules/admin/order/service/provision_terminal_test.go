package service

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"hostsent/backend/internal/pkg/upstream"
)

// TestIsTerminalProvisionError 锁定「重试也不会变好」的判据。
//
// 事故背景（2026-10-12 实测）：上游余额不足时旧逻辑重试 5 次，每次都重新下单，
// 一个订单在上游留下 5 张待支付账单。这类错误上游已经明确答复，重试只会放大损失，
// 必须首次即转人工；而网络超时/5xx 没有明确答复，仍要自动重试。
func TestIsTerminalProvisionError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"普通错误（非上游）", errors.New("boom"), false},
		{"上游业务码（余额不足）", &upstream.ProviderError{Op: "CreateInstance", Code: 200, Msg: "余额不足：账单需18.00，现余额5.08"}, true},
		{"上游业务码（商品下架）", &upstream.ProviderError{Op: "CreateInstance", Code: 400, Msg: "商品不存在"}, true},
		{"HTTP 400", &upstream.ProviderError{Op: "CreateInstance", StatusCode: 400, Msg: "参数错误"}, true},
		{"HTTP 404", &upstream.ProviderError{Op: "CreateInstance", StatusCode: 404}, true},
		{"HTTP 500 可重试", &upstream.ProviderError{Op: "CreateInstance", StatusCode: 500, Msg: "内部错误"}, false},
		{"HTTP 502 可重试", &upstream.ProviderError{Op: "CreateInstance", StatusCode: 502}, false},
		{"网络错误可重试", &upstream.ProviderError{Op: "request", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}, false},
		{"零值 ProviderError 可重试", &upstream.ProviderError{}, false},
		{"包装后的业务错误仍判终止", fmt.Errorf("上游开通失败: %w", &upstream.ProviderError{Code: 400, Msg: "余额不足"}), true},
		{"包装后的网络错误仍可重试", fmt.Errorf("上游开通失败: %w", &upstream.ProviderError{Err: errors.New("timeout")}), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTerminalProvisionError(c.err); got != c.want {
				t.Fatalf("isTerminalProvisionError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
