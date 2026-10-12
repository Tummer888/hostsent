package server

// 适配器注册触发点（必须保留）。
//
// 各上游/支付/通知/验证码/第三方登录/实名核验适配器一律靠 init() 向各自注册表
// 登记工厂与能力描述符；只要没有任何文件 import 它们，链接器就会把整个包裁掉，
// 表现为「类型列表为空、Build 报 未注册（已接入: []）、同步整体停摆」。
//
// 历史事故：批次四 4b 把 server.go 拆成分域装配时，连带删掉了原先写在 server.go
// 里的这批空导入且没有补位，于是 mofangyun 等适配器全部失效——provider 19 连续
// 报 factory 未注册、熔断暂停，调度扫描 due=0。本文件专为此存在：装配拆分可以
// 继续搬动业务代码，但这批空导入必须始终有且仅有一处。
//
// 新增适配器时在此追加，并同步更新 pkg 侧各注册表。
import (
	// 上游计算/转售适配器：注册 ProviderManager 工厂 + CapabilityDescriptor。
	// dcimcloud=魔方云（自营算力）、zjmf=魔方财务（上游转售）。
	_ "hostsent/backend/internal/pkg/upstream/dcimcloud"
	_ "hostsent/backend/internal/pkg/upstream/zjmf"

	// 支付渠道适配器：登记能力描述符与工厂。
	_ "hostsent/backend/internal/pkg/payment/manual"

	// 通知渠道适配器（doc90 §2.3）：smtp 为真实现，四家短信为占位
	// （Send 返回「待接入」），使渠道在后台 UI 上完整可见。
	_ "hostsent/backend/internal/pkg/notifier/provider/aliyun"
	_ "hostsent/backend/internal/pkg/notifier/provider/duanxinbao"
	_ "hostsent/backend/internal/pkg/notifier/provider/saiyou"
	_ "hostsent/backend/internal/pkg/notifier/provider/smtp"
	_ "hostsent/backend/internal/pkg/notifier/provider/tencent"

	// 第三方验证码适配器（doc91 §7.2）：netease 为真实现，其余四家为占位
	// （Challenge/Verify 返回「待接入」），使类型在管理端完整可选，
	// 选中后自动回落 native（兜底必须存在）。
	_ "hostsent/backend/internal/pkg/captcha/provider/aliyun"
	_ "hostsent/backend/internal/pkg/captcha/provider/dingxiang"
	_ "hostsent/backend/internal/pkg/captcha/provider/geetest"
	_ "hostsent/backend/internal/pkg/captcha/provider/netease"
	_ "hostsent/backend/internal/pkg/captcha/provider/tencent"

	// 第三方登录适配器（doc104 §6.3）：三家都走标准 OAuth2 授权码流程，
	// 凭证由运营在后台配置。
	_ "hostsent/backend/internal/pkg/oauth/provider/alipay"
	_ "hostsent/backend/internal/pkg/oauth/provider/qq"
	_ "hostsent/backend/internal/pkg/oauth/provider/wechat"

	// 实名核验适配器（doc104 §5.5）：manual 为人工审核兜底（恒可用），
	// alipay 为跳转式核验。
	_ "hostsent/backend/internal/pkg/realname/provider/alipay"
	_ "hostsent/backend/internal/pkg/realname/provider/manual"
)
