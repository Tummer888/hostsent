// Package integration 提供跨集成域（上游云渠道、支付渠道）共用的中立构件。
//
// 背景：上游渠道与支付渠道都需要「凭证字段描述 + 字段级加密 + 后台动态表单」，
// 但二者业务语义不同（云资源 vs 资金通道）。把字段描述抽到本包后，
// pkg/credentials 与后台动态表单不再反向依赖 pkg/upstream 的云语义，
// 新增支付渠道时无需把支付概念塞进云上游契约。
package integration

// FieldType 凭证/端点字段的控件类型，用于后台动态表单渲染。
const (
	FieldTypeString   = "string"
	FieldTypePassword = "password"
	FieldTypeNumber   = "number"
	FieldTypeSelect   = "select"
	FieldTypeBool     = "bool"
	FieldTypeTextarea = "textarea"
)

// FieldOption 下拉选项。
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Field 凭证/端点字段描述：驱动后台动态表单 + 字段级加密。
type Field struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        string        `json:"type"` // FieldType*
	Required    bool          `json:"required"`
	Secret      bool          `json:"secret"` // true=落库前字段级加密，回显脱敏
	Placeholder string        `json:"placeholder,omitempty"`
	Help        string        `json:"help,omitempty"`
	Default     string        `json:"default,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
}
