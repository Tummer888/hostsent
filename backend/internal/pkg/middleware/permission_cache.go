package middleware

import (
	"context"
	"sync"

	appauth "hostsent/backend/internal/pkg/auth"
)

// PermissionResolver 由员工 RBAC 仓储实现（结构调整由 pkg 定义接口，避免 middleware → modules 反向依赖）。
type PermissionResolver interface {
	FindRoleCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error)
	FindPermissionCodesByAdminID(ctx context.Context, adminID uint64) ([]string, error)
	IsAdminActive(ctx context.Context, adminID uint64) (bool, error)
	// FindDepartmentIDByAdminID 返回员工所属部门（0 表示未归属部门）。
	// 部门随鉴权快照一起缓存，改部门后靠 InvalidateAdmin 刷新，避免每请求多查一次库。
	FindDepartmentIDByAdminID(ctx context.Context, adminID uint64) (uint64, error)
}

// AdminGrant 一名管理员的鉴权快照：角色码 + 生效权限集合 + 启用状态 + 所属部门。
type AdminGrant struct {
	Roles        []string
	Perms        appauth.PermissionSet
	Active       bool
	DepartmentID uint64
}

// IsSuper 是否为超管（权限集合含通配 "*"）。
func (g *AdminGrant) IsSuper() bool {
	return g != nil && g.Perms.IsSuper()
}

// HasAny 是否持有任一权限码。
func (g *AdminGrant) HasAny(codes ...string) bool {
	return g != nil && g.Perms.HasAny(codes...)
}

// PermissionCache 缓存管理员鉴权快照，避免每次请求打 DB。
//
// 失效策略用全局版本号实现（单机部署足够）：
// 任何角色/绑定变更都整体失效，宁可多查一次库，也不留下"改了没生效"的疑难杂症。
type PermissionCache interface {
	Get(adminID uint64) (*AdminGrant, bool)
	Set(adminID uint64, grant *AdminGrant)
	InvalidateAdmin(adminID uint64)
	InvalidateRole(roleID uint64)
	InvalidateAll()
}

type cacheEntry struct {
	version uint64
	grant   *AdminGrant
}

// MemoryPermissionCache 进程内版本号缓存（无 Redis 亦可运行）。
type MemoryPermissionCache struct {
	mu      sync.RWMutex
	version uint64
	entries map[uint64]cacheEntry
}

// NewMemoryPermissionCache 创建进程内权限缓存。
func NewMemoryPermissionCache() *MemoryPermissionCache {
	return &MemoryPermissionCache{entries: make(map[uint64]cacheEntry)}
}

func (c *MemoryPermissionCache) Get(adminID uint64) (*AdminGrant, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[adminID]
	if !ok || entry.version != c.version {
		return nil, false
	}
	return entry.grant, true
}

func (c *MemoryPermissionCache) Set(adminID uint64, grant *AdminGrant) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[adminID] = cacheEntry{version: c.version, grant: grant}
}

func (c *MemoryPermissionCache) InvalidateAdmin(adminID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, adminID)
}

// InvalidateRole 使角色下所有管理员的缓存失效。
// 进程内实现不知道角色→管理员映射，统一按整体失效处理（正确性优先）。
func (c *MemoryPermissionCache) InvalidateRole(_ uint64) {
	c.InvalidateAll()
}

func (c *MemoryPermissionCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.version++
	c.entries = make(map[uint64]cacheEntry)
}

// ResolveAdminGrant 解析并构建鉴权快照（未命中缓存时调用），super_admin 归一为通配权限。
func ResolveAdminGrant(ctx context.Context, resolver PermissionResolver, adminID uint64) (*AdminGrant, error) {
	active, err := resolver.IsAdminActive(ctx, adminID)
	if err != nil {
		return nil, err
	}
	roles, err := resolver.FindRoleCodesByAdminID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	perms, err := resolver.FindPermissionCodesByAdminID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	// 部门查询失败不应阻断鉴权（老库或异常数据下 department_id 缺失），降级为「无部门」。
	deptID, err := resolver.FindDepartmentIDByAdminID(ctx, adminID)
	if err != nil {
		deptID = 0
	}
	effective := appauth.EffectivePermissions(roles, perms)
	return &AdminGrant{Roles: roles, Perms: appauth.NewPermissionSet(effective), Active: active, DepartmentID: deptID}, nil
}

// LoadAdminGrant 优先读缓存，未命中则查库回填。
func LoadAdminGrant(ctx context.Context, cache PermissionCache, resolver PermissionResolver, adminID uint64) (*AdminGrant, error) {
	if cache != nil {
		if grant, ok := cache.Get(adminID); ok {
			return grant, nil
		}
	}
	grant, err := ResolveAdminGrant(ctx, resolver, adminID)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cache.Set(adminID, grant)
	}
	return grant, nil
}
