#!/usr/bin/env bash
# 结构规范检查（CONTRIBUTING §1–§3，115 整改方案批次五）。
# grep 级实现：校验目录层级规则、uc 端模块形态约束、跨层依赖方向。
#
# 棘轮模式：存量违例列入下方豁免清单（每条带原因），修复后应从清单移除；
# 清单之外的新增违例直接 fail（CI 门禁）。
set -uo pipefail

cd "$(dirname "$0")/../backend" || exit 1

violations=0

# ── 豁免清单（存量技术债，逐项修复后删除对应条目）─────────────────────────
# pkg/db：迁移中心（AutoMigrate + Go seed 需要全量 model，架构既定）
ALLOW_PKG_MODULES='internal/pkg/db/'
# 审计中间件直读 model，待抽最小接口
ALLOW_PKG_MODULES+='|internal/pkg/middleware/admin_audit.go'
ALLOW_PKG_MODULES+='|internal/pkg/middleware/user_audit.go'
# handler 直用 repo/gorm（存量 9 文件，待把依赖下沉 service）
ALLOW_HANDLER_REPO='flashdiscount/handler/flash_discount_handler.go|manager/handler/department_handler.go|system/config/handler/config_handler.go|logcenter/handler/handler.go|agentlevel/handler/agent_level_handler.go|account/level/handler/level_handler.go|user_deletion_handler.go|user_error.go|verification_handler.go|uc/oauth/handler/oauth_admin_handler.go'
# uc 薄适配 service 委托 admin 域 service（形态合法，CONTRIBUTING §2）
ALLOW_UC_ADMIN_SERVICE='uc/product/service/product_service.go|uc/order/service/order_service.go'
# uc service 引用 admin model 做请求/响应转换（存量，待收敛为 DTO 映射）
ALLOW_UC_ADMIN_MODEL='uc/order/service/order_service_test.go|uc/order/service/order_service.go|uc/payment/service/cashier.go'

check() {
  local desc="$1" pattern="$2" scope="$3" allow="$4"
  local hits
  hits=$(grep -rn --include='*.go' -E "$pattern" $scope 2>/dev/null || true)
  if [ -n "$allow" ]; then
    hits=$(echo "$hits" | grep -vE "$allow" || true)
  fi
  if [ -n "$hits" ]; then
    echo "VIOLATION [$desc]"
    echo "$hits"
    violations=$((violations + 1))
  fi
}

# ── 依赖方向（CONTRIBUTING §3）───────────────────────────────────────────

# 1) pkg 禁止 import modules（pkg 只能被 modules/cmd 引用）
check "pkg → modules 依赖禁止" \
  'hostsent/backend/internal/modules' \
  internal/pkg \
  "$ALLOW_PKG_MODULES"

# 2) handler 禁止直接使用 repository / gorm（HTTP 层只调 service）
#    逐 handler 目录检查，含分组式嵌套（域/子模块/handler）。
while IFS= read -r dir; do
  check "handler 依赖 repository/gorm 禁止 (${dir})" \
    'modules/[a-z]+/.*repository|"gorm.io/gorm"' \
    "$dir" \
    "$ALLOW_HANDLER_REPO"
done < <(find internal/modules -type d -name handler)

# 3) uc 端依赖 admin 域 service 必须是「薄适配委托」（包注释声明），
#    禁止 import admin 域 model（转换依赖已列入豁免清单收敛中）。
check "uc → admin service（仅薄适配可）" \
  'hostsent/backend/internal/modules/admin/[a-z]+/[a-z/]+/service' \
  internal/modules/uc \
  "$ALLOW_UC_ADMIN_SERVICE"
check "uc → admin model 依赖禁止" \
  'hostsent/backend/internal/modules/admin/[a-z]+/[a-z/]+/model' \
  internal/modules/uc \
  "$ALLOW_UC_ADMIN_MODEL"

# ── uc 端形态约束（CONTRIBUTING §2）──────────────────────────────────────

# 纯透传模块（uc/{finance,point,referral}）只允许 handler 目录存在；
# 若新增了 service/repository 目录，说明编排已超 3 步，必须按规范
# 升级为「薄适配」并在包注释中声明，而不是继续堆 handler。
for m in finance point referral; do
  extra=$(find "internal/modules/uc/$m" -mindepth 1 -maxdepth 1 -type d ! -name handler 2>/dev/null || true)
  if [ -n "$extra" ]; then
    echo "VIOLATION [纯透传模块 uc/$m 出现非 handler 目录，需按 CONTRIBUTING §2 升级形态]: $extra"
    violations=$((violations + 1))
  fi
done

# ── 并发约定（CONTRIBUTING §5）───────────────────────────────────────────

# goroutine panic 防护：精确逐处校验超出 grep 能力，这里输出计数供 review
# 对照——新增 go func() 时必须接入 internal/pkg/safego（Recover/Go）。
goroutines=$(grep -rn --include='*.go' 'go func(' internal/modules internal/server 2>/dev/null | grep -v '_test.go' | wc -l)
recovers=$(grep -rln --include='*.go' 'safego.Recover\|safego.Go' internal/modules internal/server internal/pkg 2>/dev/null | wc -l)
echo "INFO goroutine(生产代码)=$goroutines 处，含 safego 防护的文件=$recovers 个"

if [ "$violations" -gt 0 ]; then
  echo "结构检查失败：$violations 类违例"
  exit 1
fi
echo "结构检查通过"
