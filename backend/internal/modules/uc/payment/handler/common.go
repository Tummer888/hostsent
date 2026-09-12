package handler

import "strconv"

// parseUint64 解析路径参数中的无符号整数 ID。
func parseUint64(raw string) (uint64, error) {
	return strconv.ParseUint(raw, 10, 64)
}
