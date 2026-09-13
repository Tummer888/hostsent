package model

import (
	"encoding/json"
	"strings"
	"time"
)

// TicketCategory 工单分类
type TicketCategory struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"size:64;not null"`                     // 显示名称
	Code        string `gorm:"size:64;not null;uniqueIndex"`         // 分类编码（如 presales）
	Description string `gorm:"size:255"`                             // 描述
	SortOrder   int    `gorm:"column:sort_order;not null;default:0"` // 排序值（越小越靠前）
	Status      string `gorm:"size:32;not null;default:active"`      // 状态：active / disabled
	// DefaultRoleCode 自动派单目标角色 code（P2-03）：按此角色挑在岗员工。
	DefaultRoleCode string `gorm:"column:default_role_code;size:64"`
	// DefaultGroupID 历史裸客服组 ID（deprecated）：派单已改为按 department_id，
	// 迁移 040 已把同名组映射为部门；保留列仅作存量读取与回滚兼容。
	DefaultGroupID *uint64 `gorm:"column:default_group_id"`
	// —— S2 工单基础增强 ——
	// DepartmentID 归属部门（departments.id）：决定派单候选与数据范围。
	DepartmentID uint64 `gorm:"column:department_id;index"`
	// RequireRealname 提交前置：要求用户已通过实名认证。
	RequireRealname bool `gorm:"column:require_realname;not null;default:false"`
	// RequireBinding 提交前置：必须关联本人名下的订单或实例。
	RequireBinding bool `gorm:"column:require_binding;not null;default:false"`
	// NeedReview 该分类的管理员回复需双人复核后才对用户可见（S3）。
	NeedReview bool `gorm:"column:need_review;not null;default:false"`
	// VisibleRoleCodes 仅这些用户角色可提交该分类（JSON 数组字符串，空=不限）。
	VisibleRoleCodes string `gorm:"column:visible_role_codes;type:jsonb;not null;default:'[]'"`
	// SLAHours 首次响应时限（小时）；0 表示不启用 SLA 标记（P2-05）。
	SLAHours  int       `gorm:"column:sla_hours;not null;default:0"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (TicketCategory) TableName() string {
	return "ticket_categories"
}

// RoleCodeList 解析可见角色码列表（列存 jsonb，宽松解析：非法内容按空处理，
// 空列表语义为「不限」）。
func (c TicketCategory) RoleCodeList() []string {
	raw := strings.TrimSpace(c.VisibleRoleCodes)
	if raw == "" || raw == "null" {
		return nil
	}
	var codes []string
	if err := json.Unmarshal([]byte(raw), &codes); err != nil {
		return nil
	}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		if code = strings.TrimSpace(code); code != "" {
			out = append(out, code)
		}
	}
	return out
}

// EncodeRoleCodes 把角色码列表编码为存储用 jsonb 字符串（空列表存 "[]"）。
func EncodeRoleCodes(codes []string) string {
	out := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	buf, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(buf)
}
