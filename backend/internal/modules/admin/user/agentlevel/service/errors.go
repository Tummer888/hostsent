package service

import (
	"errors"

	"hostsent/backend/internal/modules/admin/user/agentlevel/repository"
)

var (
	ErrSchemeNoGroup = errors.New("请先为该折扣组绑定商品分组")
	// ErrSchemeGroupEmpty 绑定的商品分组为空。
	ErrSchemeGroupEmpty = errors.New("绑定的商品分组为空，请先添加分类或商品")
	// ErrSchemeNoRates 折扣组内没有大于 0 的费率。
	ErrSchemeNoRates = errors.New("折扣组内没有大于 0 的折扣率，无需应用")
	// ErrInvalidRate 折扣率非法。
	ErrInvalidRate = errors.New("折扣率必须是 0 到 1 之间的数值")
	// ErrTargetConflict 本折扣组要写的目标已被别的折扣组占用。
	//
	// 为什么必须拦：同一个 (代理分组, 目标) 单元格只有一个值，两个折扣组都写它时后应用的
	// 静默覆盖前面的，矩阵上只看得到一个数字、查不出是谁写的，运营会以为两套折扣都在生效。
	ErrTargetConflict      = errors.New("该折扣组绑定的商品与其它折扣组重叠，无法应用")
	ErrInvalidAssignTarget = errors.New("用户与代理等级 ID 不能为空")
	// ErrLevelDisabled 目标等级已停用，不允许指派。
	ErrLevelDisabled = errors.New("目标代理等级已停用，不能指派")
	// ErrRateBelowCost 折扣率低于成本率（会亏本卖）。
	ErrRateBelowCost = errors.New("折扣率低于成本率，会亏本销售")
	// ErrRateOverOne 折扣率大于 1，比标价还贵。
	ErrRateOverOne = errors.New("折扣率不能大于 1（100%）")
	// ErrNotMonotonic 同一目标下等级越高折扣反而越差。
	ErrNotMonotonic = errors.New("同一目标下，等级越高折扣必须越优（折扣率越小）")
	// 以下四个是 repository 层同义哨兵的 service 再出口：
	// handler 只允许依赖本包判断语义（CONTRIBUTING §3）。
	// ErrNotFound 等级不存在。
	ErrNotFound = repository.ErrNotFound
	// ErrLevelInUse 等级仍被用户占用。
	ErrLevelInUse = repository.ErrLevelInUse
	// ErrTargetNotFound 折扣目标（分类/商品）不存在。
	ErrTargetNotFound = repository.ErrTargetNotFound
	// ErrCodeTaken 等级编码重复。
	ErrCodeTaken = repository.ErrCodeTaken
)
