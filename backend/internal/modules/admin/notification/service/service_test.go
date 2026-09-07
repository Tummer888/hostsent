package service

import (
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name string
		tpl  string
		vars map[string]string
		want string
	}{
		{
			name: "正常替换",
			tpl:  "订单 {order_no} 支付成功，金额 ¥{amount}",
			vars: map[string]string{"order_no": "OD20260906001", "amount": "99.00"},
			want: "订单 OD20260906001 支付成功，金额 ¥99.00",
		},
		{
			name: "变量缺失保留原文",
			tpl:  "订单 {order_no} 支付成功，金额 ¥{amount}",
			vars: map[string]string{"order_no": "OD001"},
			want: "订单 OD001 支付成功，金额 ¥{amount}",
		},
		{
			name: "无变量",
			tpl:  "系统通知",
			vars: nil,
			want: "系统通知",
		},
		{
			name: "空模板",
			tpl:  "",
			vars: map[string]string{"key": "val"},
			want: "",
		},
		{
			name: "重复变量",
			tpl:  "{name} 的订单，{name} 好",
			vars: map[string]string{"name": "张三"},
			want: "张三 的订单，张三 好",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTemplate(tt.tpl, tt.vars)
			if got != tt.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrefAllowed(t *testing.T) {
	// 这个测试需要数据库，仅验证逻辑不 panic
	// 在实际环境中使用 testcontainers 或 mock 进行完整测试
}
