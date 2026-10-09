package service

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// isDuplicateKey 判断是否为 PostgreSQL 唯一约束冲突（23505）。
//
// 用于把「重复发布」从错误降级为幂等成功：uk_notifications_source 命中即视为
// 同一事件已发布过，第二次调用返回成功而不是报错（bug ③ 回归断言）。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	// 找不到 pgx 错误类型时退回文本匹配（驱动被替换时不至于失效）。
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}
