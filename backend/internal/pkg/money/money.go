// Package money 提供金融金额的精度与格式化工具。
//
// 财务域的金额底层统一使用整数分（int64）参与运算，避免浮点误差；
// 对外（DB/前端）以 decimal(15,2) 元展示。存量 order 等模块的历史字段为 float64，
// 本包提供 float64 与分之间安全转换的适配能力。
package money

import (
	"math"
	"strconv"
)

// Money 表示以「分」为单位的金额。
type Money int64

// ParseYuan 将元（float64）转换为分（向上取整到最近的分）。
func ParseYuan(v float64) Money {
	return Money(int64(math.Round(v * 100)))
}

// ParseFen 将分（数字或字符串）解析为 Money，非法输入返回 0。
func ParseFen(v interface{}) Money {
	switch n := v.(type) {
	case int64:
		return Money(n)
	case int:
		return Money(n)
	case float64:
		return Money(int64(math.Round(n)))
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return 0
		}
		return Money(i)
	default:
		return 0
	}
}

// Yuan 将分转换为元（float64，保留两位）。
func (m Money) Yuan() float64 {
	return float64(m) / 100
}

// String 将分格式化为元字符串，保留两位小数，形如 "0.00"。
func (m Money) String() string {
	return strconv.FormatFloat(m.Yuan(), 'f', 2, 64)
}

// Add 金额相加。
func (m Money) Add(other Money) Money { return m + other }

// Sub 金额相减。
func (m Money) Sub(other Money) Money { return m - other }

// Compare 比较金额：小于返回 -1，等于返回 0，大于返回 1。
func (m Money) Compare(other Money) int {
	switch {
	case m < other:
		return -1
	case m > other:
		return 1
	default:
		return 0
	}
}

// Round2 将浮点金额四舍五入保留两位小数（元）。
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// FormatYuan 将元金额格式化保留两位小数字符串。
func FormatYuan(v float64) string {
	return strconv.FormatFloat(Round2(v), 'f', 2, 64)
}
