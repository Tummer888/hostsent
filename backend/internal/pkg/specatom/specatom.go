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
