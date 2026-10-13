package service

import (
	"errors"

	"hostsent/backend/internal/modules/admin/user/verification/repository"
)

var (
	ErrDocumentStoreNotConfigured = errors.New("材料存储未配置，请联系管理员")
	// ErrDocumentTypeNotAllowed 材料类型不在白名单内。
	ErrDocumentTypeNotAllowed = errors.New("不支持的材料类型")
	// ErrDocumentExtNotAllowed 文件扩展名不在白名单内。
	ErrDocumentExtNotAllowed = errors.New("仅支持图片（png/jpg/webp/bmp）与 PDF")
	// ErrDocumentTooLarge 文件超过大小上限。
	ErrDocumentTooLarge = errors.New("文件超过 10MB 上限")
	// ErrDocumentLimitExceeded 单申请材料数超限。
	ErrDocumentLimitExceeded = errors.New("材料数量已达上限")
	// ErrDocumentNotFound 材料不存在。
	ErrDocumentNotFound = errors.New("材料不存在")
	ErrConfigNotAllowed = errors.New("当前未开放自助实名认证")
	// ErrTypeNotAllowed 认证类型不在允许列表内。
	ErrTypeNotAllowed = errors.New("不支持该认证类型")
	// ErrAlreadyVerified 已实名且配置禁止重复提交。
	ErrAlreadyVerified = errors.New("账号已完成实名认证，如需变更请先联系客服撤销")
	// ErrPendingExists 已有待审核申请。
	ErrPendingExists = errors.New("已有待审核的实名申请，请耐心等待")
	// ErrCooldown 驳回后的冷却期未满。
	ErrCooldown = errors.New("距离上次驳回未满冷却期，请稍后再试")
	// ErrStatusConflict 状态流转非法（对已审核的申请再审）。
	ErrStatusConflict = errors.New("当前状态不允许该操作")
	// ErrApplicationNotFound 申请不存在，或不属于当前用户（不区分，避免探测）。
	ErrApplicationNotFound = errors.New("实名申请不存在")
	// ErrIDNumberInvalid 证件号格式不合法。
	ErrIDNumberInvalid = errors.New("证件号格式不合法")
	// ErrNotInitializer 该 provider 不支持跳转式核验。
	ErrNotInitializer = errors.New("该核验服务商不支持跳转认证")
	// ErrProviderNotConfigured 核验服务商未配置。
	ErrProviderNotConfigured = errors.New("实名核验服务商未配置")
	// ErrOperatorRequired 审核人缺失。
	ErrOperatorRequired = errors.New("审核人不能为空")
	// ErrRejectReasonRequired 驳回未填理由。
	//
	// 单独立一个哨兵而不是 errors.New 就地返回：驳回理由为空是**参数问题**（400），
	// 不是服务端故障。原先走 default 分支落进 500，前端只能靠 message 文本判断
	// 「这次该不该让运营补理由」，而 5xx 在监控里还会被当成真实故障告警。
	ErrRejectReasonRequired = errors.New("驳回理由不能为空")
	// ErrProviderNotFound 是 repository 层同义哨兵的 service 再出口：
	// handler 只允许依赖本包判断语义（CONTRIBUTING §3）。
	ErrProviderNotFound = repository.ErrProviderNotFound
)
