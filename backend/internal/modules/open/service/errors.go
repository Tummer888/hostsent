package service

// 开放平台错误码（固定段，供下游程序化处理；响应信封与全站一致 {code,message,data}）。
const (
	CodeOpenMissingAuth = 40100 // 缺少认证头
	CodeOpenAppInvalid  = 40101 // 应用不存在或已停用
	CodeOpenSignInvalid = 40102 // 签名不正确
	CodeOpenTimestamp   = 40103 // 时间戳超出窗口
	CodeOpenNonceReplay = 40104 // nonce 重放
	CodeOpenIPDenied    = 40105 // 来源 IP 不在白名单
	CodeOpenScopeDenied = 40301 // 能力位不足
	CodeOpenRateLimited = 42901 // 触发限流
	CodeOpenParam       = 40001 // 参数错误
	CodeOpenUnsupported = 40009 // 明确不支持（如实例销毁，D2）
	CodeOpenNotFound    = 40404 // 资源不存在
	CodeOpenConflict    = 40003 // 幂等请求处理中/冲突
	CodeOpenInternal    = 50000 // 服务内部错误
)
