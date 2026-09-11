// Package specatom 实现规格原子字典的校验（契约①，T2.6）。
//
// 背景（docs/实施计划/16 §3.2）：
//
//	StandardProductSpec 的 9 个固定字段 + Extra map[string]interface{} 是"万能袋"：
//	Extra 里塞什么都能通过，既无法校验、也无法渲染表单、更无法对外输出。
//	本包把 Extra 升级为"受 spec_atoms 约束的命名空间"——写入前校验 key 是否登记、
//	值是否符合值域，同时把 9 个固定字段登记为"良构原子"。
//
// 宽松策略（P2 预埋期）：字典未加载（空）时不校验；字典已加载时未知 key 与越界值
//
//	以 Issue 形式返回，由调用方决定是阻断还是记录告警——存量数据不会被误伤。
package specatom

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"hostsent/backend/internal/pkg/model"
)

// 良构原子 key：与 StandardProductSpec 固定字段一一对应。
const (
	KeyCPU       = "compute.cpu"
	KeyMemory    = "compute.memory"
	KeyDisk      = "storage.system.size"
	KeyDiskType  = "storage.system.type"
	KeyBandwidth = "network.bandwidth"
	KeyOS        = "image.os"
	KeyRegion    = "placement.region"
	KeyZone      = "placement.zone"
)

// wellKnownExtraKeys Extra 中允许的"平台专有参数"命名空间前缀白名单。
// 这些是各上游/平台在 catalog 阶段透传的原始字段，不纳入原子字典强校验。
var wellKnownExtraKeys = map[string]bool{
	"upstream_pid":  true,
	"pid":           true,
	"gid":           true,
	"group_name":    true,
	"configoptions": true,
	"config_groups": true,
}

// Atom 规格原子定义（可来自 spec_atoms 表，也可静态注册）。
type Atom struct {
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Unit        string          `json:"unit"`
	ValueType   string          `json:"value_type"` // int|decimal|enum|bool|string
	EnumValues  json.RawMessage `json:"enum_values"`
	MinValue    *float64        `json:"min_value"`
	MaxValue    *float64        `json:"max_value"`
	AppliesTo   string          `json:"applies_to"`
	Status      int             `json:"status"`
	Description string          `json:"description"`
	// Required 必填原子（spec_atoms.required）：上架门禁据此校验规格完整性（T4.6）。
	Required bool `json:"required"`
	// PlatformFields 各平台的读写字段名映射（spec_atoms.platform_fields 列）：
	// {"mofangyun":{"read":"cpu_num","write":"cpu","source":"实测"},...}
	// SKU 开通时据此把原子取值翻译为目标平台的写参数（T4.1/T4.2）。
	PlatformFields map[string]AtomPlatformField `json:"platform_fields,omitempty"`
}

// AtomPlatformField 单个平台对某原子的读写字段名与来源说明。
type AtomPlatformField struct {
	Read   string `json:"read,omitempty"`
	Write  string `json:"write,omitempty"`
	Source string `json:"source,omitempty"`
}

// Issue 校验发现的问题。
type Issue struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

func (i Issue) Error() string { return fmt.Sprintf("%s: %s", i.Key, i.Reason) }

// Dictionary 原子字典（并发安全）。
type Dictionary struct {
	mu    sync.RWMutex
	atoms map[string]Atom
}

// NewDictionary 创建空字典。
func NewDictionary() *Dictionary {
	return &Dictionary{atoms: map[string]Atom{}}
}

// Replace 全量替换字典内容（启动时加载 spec_atoms 用）。
func (d *Dictionary) Replace(atoms []Atom) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.atoms = make(map[string]Atom, len(atoms))
	for _, a := range atoms {
		if a.Status == 0 {
			continue
		}
		d.atoms[a.Key] = a
	}
}

// Len 返回已加载原子数。
func (d *Dictionary) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.atoms)
}

// Atoms 返回全部原子的副本（按 key 升序）。
func (d *Dictionary) Atoms() []Atom {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Atom, 0, len(d.atoms))
	for _, a := range d.atoms {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Get 查询单个原子。
func (d *Dictionary) Get(key string) (Atom, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	a, ok := d.atoms[key]
	return a, ok
}

var (
	globalMu sync.RWMutex
	global   = NewDictionary()
)

// Global 返回全局字典（由启动期从 spec_atoms 加载）。
func Global() *Dictionary {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// SetGlobal 替换全局字典（测试或多数据源场景用）。
func SetGlobal(d *Dictionary) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = d
}

// Validate 校验标准规格：固定字段按良构原子校验值域，Extra 的每个 key 必须登记
// （或在平台专有参数白名单内）且值符合 value_type/枚举/区间约束。
// 返回全部问题；字典为空时返回 nil（预埋期不阻断）。
func (d *Dictionary) Validate(spec model.StandardProductSpec) []Issue {
	if d == nil || d.Len() == 0 {
		return nil
	}
	var issues []Issue
	issues = append(issues, d.validateValue(KeyCPU, spec.CPU)...)
	issues = append(issues, d.validateValue(KeyMemory, spec.Memory)...)
	issues = append(issues, d.validateValue(KeyDisk, spec.Disk)...)
	issues = append(issues, d.validateValue(KeyBandwidth, spec.Bandwidth)...)
	issues = append(issues, d.validateValue(KeyDiskType, spec.DiskType)...)
	issues = append(issues, d.validateValue(KeyOS, spec.OS)...)
	issues = append(issues, d.validateValue(KeyRegion, spec.Region)...)
	issues = append(issues, d.validateValue(KeyZone, spec.Zone)...)

	keys := make([]string, 0, len(spec.Extra))
	for k := range spec.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if wellKnownExtraKeys[k] {
			continue
		}
		if _, ok := d.Get(k); !ok {
			issues = append(issues, Issue{Key: k, Reason: "Extra 键未在 spec_atoms 字典登记"})
			continue
		}
		issues = append(issues, d.validateValue(k, spec.Extra[k])...)
	}
	return issues
}

// ValidateMap 直接校验"原子 key → 取值"映射（SKU 的 Specs JSON）。
// 与 Validate 的区别：不做 int 截断，按原始取值判值域，因此 0.5 核 CPU 这类
// 非整数/越界取值会被拦下，而不是先被转成 0 后当作"未提供"跳过。
// 每个 key 必须登记（平台专有参数白名单除外）；字典为空时返回 nil（预埋期不阻断）。
func (d *Dictionary) ValidateMap(m map[string]any) []Issue {
	if d == nil || d.Len() == 0 || len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var issues []Issue
	for _, k := range keys {
		if wellKnownExtraKeys[k] {
			continue
		}
		if _, ok := d.Get(k); !ok {
			issues = append(issues, Issue{Key: k, Reason: "规格键未在 spec_atoms 字典登记"})
			continue
		}
		issues = append(issues, d.validateValue(k, m[k])...)
	}
	return issues
}

// ValidateMap 用全局字典校验（便捷入口）。
func ValidateMap(m map[string]any) []Issue {
	return Global().ValidateMap(m)
}

// RequiredMissing 返回字典中标记 required 但映射未提供（缺失或零值）的原子 key。
// 用于上架/入库前的完整性门禁（T4.6）。不带平台时按全部 required 原子校验。
func (d *Dictionary) RequiredMissing(m map[string]any) []Issue {
	return d.RequiredMissingFor(m, "", "")
}

// RequiredMissingFor 按链路与目标平台过滤必填原子（上架门禁，T4.6）：
//   - appliesTo 非空时只校验 applies_to 为该链路或 both 的原子（self/upstream 互不误伤）；
//   - platform 非空时只校验"在该平台确有写参数"的原子——billing.cycle 这类
//     由订单账期决定、platform_fields 写侧为 "-" 的原子不参与 SKU 规格完整性校验；
//   - platform 为空时保守回退为校验全部适用的 required 原子。
func (d *Dictionary) RequiredMissingFor(m map[string]any, appliesTo, platform string) []Issue {
	if d == nil || d.Len() == 0 {
		return nil
	}
	var issues []Issue
	for _, atom := range d.Atoms() {
		if !atom.Required || !atomAppliesTo(atom, appliesTo) {
			continue
		}
		if platform != "" && !atomProvisionableOn(atom, platform) {
			continue
		}
		if v, ok := m[atom.Key]; !ok || isZeroValue(v) {
			issues = append(issues, Issue{Key: atom.Key, Reason: "必填规格缺失"})
		}
	}
	return issues
}

// atomAppliesTo 判断原子是否适用于指定链路；appliesTo 或 atom.AppliesTo 为空视为适用。
func atomAppliesTo(atom Atom, appliesTo string) bool {
	if appliesTo == "" || atom.AppliesTo == "" {
		return true
	}
	return atom.AppliesTo == appliesTo || atom.AppliesTo == "both"
}

// atomProvisionableOn 判断原子在目标平台是否存在有效的写字段（"-" 表示该平台无此参数）。
func atomProvisionableOn(atom Atom, platform string) bool {
	field, ok := atom.PlatformFields[platform]
	if !ok {
		return false
	}
	write := strings.TrimSpace(field.Write)
	return write != "" && write != "-"
}

// RequiredMissing 用全局字典检查（便捷入口）。
func RequiredMissing(m map[string]any) []Issue {
	return Global().RequiredMissing(m)
}

// RequiredMissingFor 用全局字典按链路 + 平台检查（便捷入口，T4.6 上架门禁）。
func RequiredMissingFor(m map[string]any, appliesTo, platform string) []Issue {
	return Global().RequiredMissingFor(m, appliesTo, platform)
}

// validateValue 校验单个值；零值（空串/0）视为未提供，跳过 required 判定。
func (d *Dictionary) validateValue(key string, value any) []Issue {
	atom, ok := d.Get(key)
	if !ok {
		return nil // 字典未登记的固定字段不报错（字典为增量演进）
	}
	if isZeroValue(value) {
		return nil
	}
	switch atom.ValueType {
	case "enum":
		var allowed []string
		if len(atom.EnumValues) > 0 {
			_ = json.Unmarshal(atom.EnumValues, &allowed)
		}
		s := fmt.Sprintf("%v", value)
		for _, a := range allowed {
			if a == s {
				return nil
			}
		}
		return []Issue{{Key: key, Reason: fmt.Sprintf("取值 %q 不在枚举 %v 内", s, allowed)}}
	case "int", "decimal":
		f, ok := toFloat(value)
		if !ok {
			return []Issue{{Key: key, Reason: fmt.Sprintf("取值 %v 不是数值", value)}}
		}
		if atom.MinValue != nil && f < *atom.MinValue {
			return []Issue{{Key: key, Reason: fmt.Sprintf("取值 %v 小于下限 %v", f, *atom.MinValue)}}
		}
		if atom.MaxValue != nil && f > *atom.MaxValue {
			return []Issue{{Key: key, Reason: fmt.Sprintf("取值 %v 超过上限 %v", f, *atom.MaxValue)}}
		}
		return nil
	case "bool":
		switch value.(type) {
		case bool, string:
			return nil
		}
		return []Issue{{Key: key, Reason: fmt.Sprintf("取值 %v 不是布尔", value)}}
	default: // string
		return nil
	}
}

// isZeroValue 判定零值（未提供）。
func isZeroValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	}
	return false
}

// toFloat 把任意数值/数字字符串转为 float64。
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// Validate 用全局字典校验（便捷入口）。
func Validate(spec model.StandardProductSpec) []Issue {
	return Global().Validate(spec)
}

// FromMap 把"原子 key → 取值"的映射还原为标准规格（SKU 的 Specs JSON → StandardProductSpec）。
// 字典中登记的额外原子（非 9 个固定字段）落入 Extra，供开通时按平台字段名翻译下发。
func FromMap(m map[string]any) model.StandardProductSpec {
	var spec model.StandardProductSpec
	if m == nil {
		return spec
	}
	spec.CPU = intValue(m[KeyCPU])
	spec.Memory = intValue(m[KeyMemory])
	spec.Disk = intValue(m[KeyDisk])
	spec.Bandwidth = intValue(m[KeyBandwidth])
	spec.DiskType = stringValue(m[KeyDiskType])
	spec.OS = stringValue(m[KeyOS])
	spec.Region = stringValue(m[KeyRegion])
	spec.Zone = stringValue(m[KeyZone])
	for k, v := range m {
		switch k {
		case KeyCPU, KeyMemory, KeyDisk, KeyBandwidth, KeyDiskType, KeyOS, KeyRegion, KeyZone:
			continue
		}
		if isZeroValue(v) {
			continue
		}
		if spec.Extra == nil {
			spec.Extra = map[string]interface{}{}
		}
		spec.Extra[k] = v
	}
	return spec
}

// ToMap 把标准规格拍平为"原子 key → 取值"，仅保留非零值（SKU Specs JSON 的写入口径）。
func ToMap(spec model.StandardProductSpec) map[string]any {
	m := map[string]any{}
	if spec.CPU != 0 {
		m[KeyCPU] = spec.CPU
	}
	if spec.Memory != 0 {
		m[KeyMemory] = spec.Memory
	}
	if spec.Disk != 0 {
		m[KeyDisk] = spec.Disk
	}
	if spec.Bandwidth != 0 {
		m[KeyBandwidth] = spec.Bandwidth
	}
	if spec.DiskType != "" {
		m[KeyDiskType] = spec.DiskType
	}
	if spec.OS != "" {
		m[KeyOS] = spec.OS
	}
	if spec.Region != "" {
		m[KeyRegion] = spec.Region
	}
	if spec.Zone != "" {
		m[KeyZone] = spec.Zone
	}
	for k, v := range spec.Extra {
		if isZeroValue(v) {
			continue
		}
		m[k] = v
	}
	return m
}

// WriteParams 把原子取值翻译为目标平台的写参数（键名为该平台约定字段）。
// 平台未登记写字段或取值缺失时跳过该原子；原样返回键名相同的通用参数。
// 返回空 map（非 nil）便于调用方直接合并进上游 Extra。
func (d *Dictionary) WriteParams(platform string, spec model.StandardProductSpec) map[string]any {
	out := map[string]any{}
	if d == nil || platform == "" {
		return out
	}
	for key, value := range ToMap(spec) {
		atom, ok := d.Get(key)
		if !ok {
			continue
		}
		field, ok := atom.PlatformFields[platform]
		if !ok || field.Write == "" {
			continue
		}
		out[field.Write] = value
	}
	return out
}

// WriteParams 用全局字典翻译（便捷入口）。
func WriteParams(platform string, spec model.StandardProductSpec) map[string]any {
	return Global().WriteParams(platform, spec)
}

// intValue 把字典取值转为 int（数字/数字字符串/浮点）。
func intValue(v any) int {
	f, ok := toFloat(v)
	if !ok {
		return 0
	}
	return int(f)
}

// stringValue 把字典取值转为 string。
func stringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
