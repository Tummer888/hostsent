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
// 除会话自身状态外，还要校验**账号当前是否可用**：
//   - 账号存在、status='active'、未注销（deleted_at IS NULL）；
//   - 子账号还要连带校验其主账号（owner_user_id）同样可用。
//
// 为什么必须查账号状态：令牌只带 sid，账号被禁用/注销后旧令牌本身没有任何变化，
// 而这里此前只看 user_sessions 状态位 —— 于是「后台禁用主账号」不会影响其子账号，
// 子账号拿着旧令牌最长还能用满整个会话有效期继续操作主账号资源（实测确认）。
// 主账号被禁用 = 该客户主体停用，其下属成员自然一并失效（设计 §4.6.2 从属关系）。
//
// 用一条 JOIN 而不是先查账号再判会话：本方法每请求都可能被调用，单独两条查询
// 会让禁用态账号的每个请求打两次库（缓存只缓存结论，禁用后结论恒为 false，
// 负缓存 1 分钟即可挡住噪音）。
func (s *GormStore) IsActive(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.db == nil || sessionID == "" {
		return false, nil
	}
	var count int64
	err := s.db.WithContext(ctx).Table("user_sessions AS us").
		Joins("JOIN users AS u ON u.id = us.user_id").
		Joins("LEFT JOIN users AS owner ON owner.id = u.owner_user_id").
		Where("us.session_id = ?", sessionID).
		Where("us.status = ?", "active").
		Where("us.expired_at IS NULL OR us.expired_at > ?", time.Now()).
		Where("u.status = ?", "active").
		Where("u.deleted_at IS NULL").
		// 子账号：主账号也必须可用（未禁用、未注销）。
		Where("u.owner_user_id IS NULL OR (owner.status = ? AND owner.deleted_at IS NULL)", "active").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
