package cache

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
)

// randomToken 生成 16 字节随机令牌（锁持有者标识）。
func randomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "fallback-token"
	}
	return hex.EncodeToString(buf)
}

// parseInt64 宽松解析十进制字符串，失败返回 0。
func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func formatInt64(v int64) string { return strconv.FormatInt(v, 10) }
