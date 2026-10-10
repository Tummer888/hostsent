// Package service 提供财务参数读写（system_configs 的白名单键）。
//
// 设计取舍：财务参数不另建表，直接落 system_configs（与返现/提成/验证码同构），
// 但读写都只允许白名单键 —— 财务角色因此可以调自己的参数，
// 而不需要被授 system:config:update（那会连带放开整站配置）。
package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	costservice "hostsent/backend/internal/modules/admin/finance/cost/service"
	referralmodel "hostsent/backend/internal/modules/admin/finance/referral/model"
	"hostsent/backend/internal/modules/admin/finance/settings/dto"
	settingsmodel "hostsent/backend/internal/modules/admin/finance/settings/model"
)

// ErrInvalidSetting 参数键不在白名单或取值非法。
var ErrInvalidSetting = errors.New("财务参数非法")

// 分组标识（同时是落库的 config_group）。
const (
	GroupFinance  = settingsmodel.ConfigGroup
	GroupReferral = referralmodel.ConfigGroup
)

// 可写财务参数键（键定义见 settings/model 与 referral/model，各有唯一定义处）。
const (
	ConfigKeyAdjustEnabled  = settingsmodel.ConfigKeyAdjustEnabled
	ConfigKeyReconTolerance = settingsmodel.ConfigKeyReconTolerance
	ConfigKeyBalanceWarning = settingsmodel.ConfigKeyBalanceWarning
)

// ConfigStore 配置存取能力（由装配层注入，避免本模块 import 系统配置模块）。
type ConfigStore interface {
	// Get 按键读取原文，键不存在返回 found=false。
	Get(ctx context.Context, key string) (value string, found bool, err error)
	// Save 按键 upsert（不存在则创建）。
	Save(ctx context.Context, key, value, valueType, description, group string) error
}

// spec 参数定义。
type spec struct {
	key         string
	label       string
	description string
	valueType   string
	defaultVal  string
	usage       string
	min         float64
	max         float64
}

type groupSpec struct {
	key   string
	label string
	hint  string
	specs []spec
}

// 参数目录：与 seed（db.go）写入的默认值逐字一致；每个键都必须有运行时消费点。
var groups = []groupSpec{
	{
		key:   GroupFinance,
		label: "财务口径",
		hint:  "调账开关、对账判定与余额预警阈值，分别由账务核心、对账中心与财务统计实时读取。",
		specs: []spec{
			{
				key:         ConfigKeyAdjustEnabled,
				label:       "允许人工资金操作",
				description: "关闭后人工调账与后台代客下单的余额扣款都会被拒绝，存量流水不受影响",
				valueType:   "bool",
				defaultVal:  settingsmodel.DefaultAdjustEnabled,
				usage:       "生效点：POST /api/v1/admin/finance/transactions/adjust 与后台代下单余额支付",
			},
			{
				key:         ConfigKeyReconTolerance,
				label:       "对账差异容差（元）",
				description: "账实差异绝对值超过该值时，对账中心判为「可疑」",
				valueType:   "number",
				defaultVal:  settingsmodel.DefaultReconTolerance,
				usage:       "生效点：对账中心 POST /api/v1/admin/finance/bills/recon 的 status 判定",
				max:         1000,
			},
			{
				key:         costservice.ConfigKeyBalanceWarning,
				label:       "上游余额低水位阈值",
				description: "上游账户余额低于该金额时在成本台账/总览提示充值（元）；0=不启用固定阈值，仅按到期金额判断",
				valueType:   "number",
				defaultVal:  "0",
				usage:       "生效点：成本管理「上游余额台账」与「成本总览」的余额水位告警（余额 < 未来 30 天到期金额恒为 critical）",
				max:         100000000,
			},
			{
				key:         costservice.ConfigKeySnapshotHour,
				label:       "上游余额自动快照小时",
				description: "每天该时点后首次抓取各渠道余额并落快照（0-23 整点）；抓不到的渠道手工录入",
				valueType:   "number",
				defaultVal:  settingsmodel.DefaultSnapshotHour,
				usage:       "生效点：成本管理每日自动快照任务（跨天只执行一次，改到已过时点则次日生效）",
				max:         23,
			},
			{
				key:         ConfigKeyBalanceWarning,
				label:       "余额预警阈值（元）",
				description: "钱包可用余额低于该值时计入总览的预警统计",
				valueType:   "number",
				defaultVal:  settingsmodel.DefaultBalanceWarning,
				usage:       "生效点：财务总览「低余额钱包」卡片与统计接口 wallet 字段",
				max:         1000000,
			},
		},
	},
	{
		key:   GroupReferral,
		label: "返现参数",
		hint:  "推广邀请返现的开关、比率与提现下限，由返现模块计提与提现时实时读取（迁移 073 起编辑入口由系统配置迁入本页）。",
		specs: []spec{
			{
				key:         referralmodel.ConfigKeyEnabled,
				label:       "启用推广邀请返现",
				description: "关闭后停止计提返现，且用户无法提现或转入余额",
				valueType:   "bool",
				defaultVal:  "true",
				usage:       "生效点：返现计提与提现校验",
			},
			{
				key:         referralmodel.ConfigKeyFirstOrder,
				label:       "首单返现比率",
				description: "0-1 小数，基数为被邀请人订单实付金额",
				valueType:   "number",
				defaultVal:  "0.10",
				usage:       "生效点：被邀请人首单支付后计提返现",
				max:         1,
			},
			{
				key:         referralmodel.ConfigKeySubsequent,
				label:       "后续订单返现比率",
				description: "被邀请人首单之后每次成功订单适用",
				valueType:   "number",
				defaultVal:  "0.05",
				usage:       "生效点：被邀请人非首单支付后计提返现",
				max:         1,
			},
			{
				key:         referralmodel.ConfigKeyRenewal,
				label:       "续费返现比率",
				description: "被邀请人续费订单适用",
				valueType:   "number",
				defaultVal:  "0.03",
				usage:       "生效点：被邀请人续费支付后计提返现",
				max:         1,
			},
			{
				key:         referralmodel.ConfigKeyMinWithdraw,
				label:       "返现最低提现金额（元）",
				description: "单笔返现提现申请的最低金额",
				valueType:   "number",
				defaultVal:  "50",
				usage:       "生效点：用户提交返现提现时的下限校验",
				max:         1000000,
			},
		},
	},
}

// SettingsService 财务参数能力。
type SettingsService interface {
	// List 返回全部参数（取落库值，缺失回默认值）。
	List(ctx context.Context) (*dto.SettingsListResponse, error)
	// Update 保存参数（白名单 + 类型与范围校验），返回保存后的列表。
	Update(ctx context.Context, req dto.SettingsUpdateRequest) (*dto.SettingsListResponse, error)
}

type settingsService struct {
	store ConfigStore
}

// NewSettingsService 创建财务参数服务。
func NewSettingsService(store ConfigStore) SettingsService {
	return &settingsService{store: store}
}

func (s *settingsService) List(ctx context.Context) (*dto.SettingsListResponse, error) {
	out := make([]dto.SettingGroup, 0, len(groups))
	for _, group := range groups {
		items := make([]dto.SettingItem, 0, len(group.specs))
		for _, item := range group.specs {
			value, err := s.currentValue(ctx, item)
			if err != nil {
				return nil, err
			}
			items = append(items, dto.SettingItem{
				Key:          item.key,
				Label:        item.label,
				Description:  item.description,
				ValueType:    item.valueType,
				Value:        value,
				DefaultValue: item.defaultVal,
				Usage:        item.usage,
				Min:          item.min,
				Max:          item.max,
			})
		}
		out = append(out, dto.SettingGroup{Key: group.key, Label: group.label, Hint: group.hint, Items: items})
	}
	return &dto.SettingsListResponse{Groups: out}, nil
}

func (s *settingsService) Update(ctx context.Context, req dto.SettingsUpdateRequest) (*dto.SettingsListResponse, error) {
	if len(req.Items) == 0 {
		return nil, ErrInvalidSetting
	}
	// 固定顺序写库，便于日志与排查（map 遍历无序）。
	keys := make([]string, 0, len(req.Items))
	for key := range req.Items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		groupKey, item, ok := findSpec(key)
		if !ok {
			return nil, ErrInvalidSetting
		}
		value := strings.TrimSpace(req.Items[key])
		if err := validateValue(item, value); err != nil {
			return nil, err
		}
		if err := s.store.Save(ctx, key, value, item.valueType, item.description, groupKey); err != nil {
			return nil, err
		}
	}
	return s.List(ctx)
}

func (s *settingsService) currentValue(ctx context.Context, item spec) (string, error) {
	value, found, err := s.store.Get(ctx, item.key)
	if err != nil {
		return "", err
	}
	if !found || strings.TrimSpace(value) == "" {
		return item.defaultVal, nil
	}
	return value, nil
}

// findSpec 按 key 定位参数定义（含其所属分组）。
func findSpec(key string) (string, spec, bool) {
	for _, group := range groups {
		for _, item := range group.specs {
			if item.key == key {
				return group.key, item, true
			}
		}
	}
	return "", spec{}, false
}

func validateValue(item spec, value string) error {
	switch item.valueType {
	case "bool":
		if _, err := strconv.ParseBool(value); err != nil {
			return ErrInvalidSetting
		}
	case "number":
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || number < item.min {
			return ErrInvalidSetting
		}
		if item.max > 0 && number > item.max {
			return ErrInvalidSetting
		}
	}
	return nil
}
