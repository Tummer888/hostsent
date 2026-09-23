package sessionguard

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GormStore 用 user_sessions 表实现 Store。
//
// 直接用表名而不是 security 模块的 model.Session：本包在 pkg 层，import 业务模块
// 会造成 pkg → modules 的反向依赖。查询只用到 session_id / status / expired_at
// 三列，语义足够稳定，不值得为它引入模型耦合。
type GormStore struct {
	db *gorm.DB
}

// NewGormStore 创建基于数据库的会话校验数据源。db 为 nil 时 IsActive 恒为 false。
func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// IsActive 查询会话是否仍有效。
//
// 三个条件缺一不可：
//   - session_id 存在（不存在 = 令牌是伪造的，或会话行已被清理）；
//   - status = 'active'（revoked = 管理员踢人/改密/登出；expired = 已过期）；
//   - expired_at 为 NULL 或未到期（NULL 表示「不设过期」，与 seed 数据口径一致）。
//
// 用 COUNT 而不是取整行：本方法每请求都可能被调用，少读几列没有意义，
// 但 COUNT 让「多行匹配」这种不可能的情况（session_id 是唯一索引）不会退化成
// 一次多余的行扫描。
func (s *GormStore) IsActive(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.db == nil || sessionID == "" {
		return false, nil
	}
	var count int64
	err := s.db.WithContext(ctx).Table("user_sessions").
		Where("session_id = ?", sessionID).
		Where("status = ?", "active").
		Where("expired_at IS NULL OR expired_at > ?", time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
