package middleware

import "github.com/gin-gonic/gin"

// EffectiveUserID 返回数据归属账号 ID（P4-04）：
//   - 子账号 → 主账号 ID（实例、订单、账单、工单列表等资源/资金一律记在主账号名下）
//   - 主账号 → 自身 ID
//
// 这是客户侧取「归属 ID」的唯一入口，各 uc handler 不得再自行判断 IsSub。
func EffectiveUserID(c *gin.Context) uint64 {
	if uc, ok := GetUserClaims(c); ok {
		if uc.IsSub && uc.OwnerUserID > 0 {
			return uc.OwnerUserID
		}
		return uc.UserID
	}
	// 兜底：兼容仅有统一 Claims 的场景（无子账号语义，即主账号）。
	if claims, ok := GetClaims(c); ok {
		return claims.UserID
	}
	return 0
}

// ActorUserID 返回真实操作人 ID（P4-04）：用于审计与「谁提交的工单」。
// 子账号返回自身 ID，主账号亦返回自身 ID。
func ActorUserID(c *gin.Context) uint64 {
	if uc, ok := GetUserClaims(c); ok {
		return uc.UserID
	}
	if claims, ok := GetClaims(c); ok {
		return claims.UserID
	}
	return 0
}

// ActorUsername 返回真实操作人用户名，便于审计与工单提交人展示。
func ActorUsername(c *gin.Context) string {
	if uc, ok := GetUserClaims(c); ok {
		return uc.Username
	}
	if claims, ok := GetClaims(c); ok {
		return claims.Username
	}
	return ""
}

// IsSubAccount 判断当前请求是否来自子账号。
func IsSubAccount(c *gin.Context) bool {
	if uc, ok := GetUserClaims(c); ok {
		return uc.IsSub
	}
	return false
}
