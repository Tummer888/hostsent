#!/usr/bin/env bash
# 结构规范检查（CONTRIBUTING §1–§4，115 整改方案批次五 + 批次二 B2）。
# grep 级实现：校验目录层级规则、uc 端模块形态约束、跨层依赖方向、
# 响应统一与错误链。
#
# 棘轮模式：存量违例列入下方豁免清单（每条带原因），修复后应从清单移除；
# 清单之外的新增违例直接 fail（CI 门禁）。
#
# --selfcheck：fixture 自测。grep 级检查会静默退化（历史上 admin/menu 单段域
# 盲区漏检数月），因此脚本自带反例断言：构造临时违规目录，检查逻辑必须抓到。
# CI 与本地提交前都应先跑 --selfcheck 再跑正式检查。
set -uo pipefail

SELFCHECK=0
if [ "${1:-}" = "--selfcheck" ]; then
  SELFCHECK=1
fi

cd "$(dirname "$0")/../backend" || exit 1

violations=0

# ── 豁免清单（存量技术债，逐项修复后删除对应条目）─────────────────────────
# pkg/db：迁移中心（AutoMigrate + Go seed 需要全量 model，架构既定；
# CONTRIBUTING §1 例外登记，pkg 面其余包不得 import modules）
ALLOW_PKG_MODULES='internal/pkg/db/'
# uc 薄适配 service 委托 admin 域 service（形态合法，CONTRIBUTING §2；
# 现状清单见 CONTRIBUTING §2 表格）
# coverage_gap_test.go 与 order_service.go 同包：测试复用生产端口所用的
# catalogservice 类型（CustomerOptionGroup/OptionPricing），依赖方向一致。
ALLOW_UC_ADMIN_SERVICE='uc/product/service/product_service.go|uc/order/service/order_service.go|uc/order/service/coverage_gap_test.go'
# handler 里非 JSON 的 c.JSON 豁免（流式下载/回调等场景）
ALLOW_RAW_CJSON='open/handler/open.go'

check() {
  local desc="$1" pattern="$2" scope="$3" allow="${4:-}"
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
    'hostsent/backend/internal/modules/.*repository|"gorm.io/gorm"' \
    "$dir"
done < <(find internal/modules -type d -name handler)

# 3) uc 端依赖 admin 域 service 必须是「薄适配委托」（包注释声明，
#    CONTRIBUTING §2 现状清单），禁止 import admin 域 model。
check "uc → admin service（仅薄适配可）" \
  'hostsent/backend/internal/modules/admin/[a-z]+(/[a-z_]+)*/service' \
  internal/modules/uc \
  "$ALLOW_UC_ADMIN_SERVICE"
check "uc → admin model 依赖禁止" \
  'hostsent/backend/internal/modules/admin/[a-z]+(/[a-z_]+)*/model' \
  internal/modules/uc

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

# ── 模块形态登记（CONTRIBUTING §1/§2，批次二 B4）────────────────────────
# 每个「模块级」目录必须在 ../scripts/module_shapes.tsv 登记形态；
# 新模块未登记即 fail，防止新目录悄悄绕过层级/形态规范。
SHAPES="../scripts/module_shapes.tsv"
if [ -f "$SHAPES" ]; then
  # 模块级目录 = 直接含 .go 文件、或再下一层出现五件套目录的目录。
  # 排除五件套目录本身（dto/model/repository/service/handler 由模板规则覆盖）。
  while IFS= read -r dir; do
    rel="${dir#internal/modules/}"
    case "$rel" in
      *"/dto"|*"/model"|*"/repository"|*"/service"|*"/handler") continue ;;
    esac
    # 是模块级：含 .go 或含五件套子目录
    has_go=$(find "$dir" -maxdepth 1 -name "*.go" -print -quit 2>/dev/null)
    has_suite=$(find "$dir" -mindepth 1 -maxdepth 1 -type d \( -name dto -o -name model -o -name repository -o -name service -o -name handler -o -name adapter -o -name catalog -o -name common -o -name lifecycle -o -name level -o -name recharge -o -name walletwithdraw -o -name providerkind \) -print -quit 2>/dev/null)
    if [ -z "$has_go" ] && [ -z "$has_suite" ]; then
      continue
    fi
    if ! cut -f1 "$SHAPES" | grep -vE '^#' | grep -qxF "$rel"; then
      echo "VIOLATION [模块目录 $rel 未在 module_shapes.tsv 登记形态]: 新建模块必须先判断归属域并登记（CONTRIBUTING §1）"
      violations=$((violations + 1))
    fi
  done < <(find internal/modules -mindepth 1 -type d | sort)
fi

# ── 并发约定（CONTRIBUTING §5）───────────────────────────────────────────

# goroutine panic 防护：精确逐处校验超出 grep 能力，这里输出计数供 review
# 对照——新增 go func() 时必须接入 internal/pkg/safego（Recover/Go）。
goroutines=$(grep -rn --include='*.go' 'go func(' internal/modules internal/server 2>/dev/null | grep -v '_test.go' | wc -l)
recovers=$(grep -rln --include='*.go' 'safego.Recover\|safego.Go' internal/modules internal/server internal/pkg 2>/dev/null | wc -l)
echo "INFO goroutine(生产代码)=$goroutines 处，含 safego 防护的文件=$recovers 个"

# ── 响应统一（CONTRIBUTING §4）────────────────────────────────────────────

# handler 禁止手写 c.JSON 响应体：统一走 internal/pkg/response
# （Success/Error/ErrorWithStatus）。豁免：测试文件与非 JSON 流式场景。
check "handler 手写 c.JSON 响应体禁止" \
  'c\.JSON\([^)]*gin\.H\{' \
  internal/modules \
  "$ALLOW_RAW_CJSON|_test.go"

# ── 错误链（CONTRIBUTING §4）──────────────────────────────────────────────

# fmt.Errorf 包装错误必须用 %w：%v/%s 后跟错误变量即丢链（已含 %w 的合法
# 包装不受影响——「含 %v 且包 err」排除「含 %w」行）。
check "错误链丢失（fmt.Errorf %v/%s 包 error）禁止" \
  'fmt\.Errorf\("[^"]*%[vsd][^"]*",[^)]*err[,)]' \
  internal \
  '_test.go|%w'

# ── 装配层行数棘轮（批次二 B3）───────────────────────────────────────────
# God File 会在装配层再生（115 方案拆过 server.go，如今 assembly_order.go
# 已 1016 行）。规则：assembly_*/routes_* 超过 800 行告警（不 fail），
# 超过 1200 行 fail——逼着在失控前拆分；新增装配文件同样受此约束。
ASSEMBLY_LIMIT_WARN=800
ASSEMBLY_LIMIT_FAIL=1200
while IFS= read -r f; do
  lines=$(wc -l < "$f")
  if [ "$lines" -gt "$ASSEMBLY_LIMIT_FAIL" ]; then
    echo "VIOLATION [装配文件 $f 达 $lines 行（上限 $ASSEMBLY_LIMIT_FAIL）]: 必须拆分为每域一个装配文件"
    violations=$((violations + 1))
  elif [ "$lines" -gt "$ASSEMBLY_LIMIT_WARN" ]; then
    echo "WARN 装配文件 $f 达 $lines 行（告警线 $ASSEMBLY_LIMIT_WARN）：考虑按业务段拆分"
  fi
done < <(find internal/server -maxdepth 1 -type f \( -name 'assembly_*.go' -o -name 'routes_*.go' \) ! -name '*_test.go' | sort)

if [ "$violations" -gt 0 ]; then
  echo "结构检查失败：$violations 类违例"
  exit 1
fi
echo "结构检查通过"

# ── fixture 自测（--selfcheck）────────────────────────────────────────────
# 反例断言：为每类核心检查构造一个最小违规 fixture，检查逻辑必须能抓到。
# 抓到=通过；抓不到=该检查已退化，自测失败。
if [ "$SELFCHECK" -eq 1 ]; then
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  fails=0

  # fixture 1：handler 直用 repository（应被「handler 依赖 repository/gorm」抓到）
  mkdir -p "$tmp/modules/foo/handler"
  cat > "$tmp/modules/foo/handler/fixture.go" <<'EOF'
package handler

import "hostsent/backend/internal/modules/admin/order/repository"

var _ = repository.ErrScheduleNotFound
EOF
  if grep -rn --include='*.go' -E 'hostsent/backend/internal/modules/.*repository' "$tmp/modules/foo/handler" | grep -q repository; then
    echo "selfcheck: handler→repository 可检出"
  else
    echo "selfcheck FAIL: handler→repository 检查已退化"; fails=$((fails+1))
  fi

  # fixture 2：单段域 uc→admin model（历史盲区回归，必须抓到）
  mkdir -p "$tmp/modules/uc/zzz"
  cat > "$tmp/modules/uc/zzz/fixture.go" <<'EOF'
package zzz

import menumodel "hostsent/backend/internal/modules/admin/menu/model"

var _ = menumodel.Menu{}
EOF
  if grep -rn --include='*.go' -E 'hostsent/backend/internal/modules/admin/[a-z]+(/[a-z_]+)*/model' "$tmp/modules/uc" | grep -q 'admin/menu/model'; then
    echo "selfcheck: uc→admin model（单段域）可检出"
  else
    echo "selfcheck FAIL: uc→admin model 检查对单段域已退化"; fails=$((fails+1))
  fi

  # fixture 3：手写 c.JSON（应被「响应统一」抓到）
  mkdir -p "$tmp/modules/foo/handler"
  cat > "$tmp/modules/foo/handler/fixture2.go" <<'EOF'
package handler

import "github.com/gin-gonic/gin"

func Fixture(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0})
}
EOF
  if grep -rn --include='*.go' -E 'c\.JSON\([^)]*gin\.H\{' "$tmp/modules/foo/handler" | grep -v '_test.go' | grep -q 'fixture2'; then
    echo "selfcheck: 手写 c.JSON 可检出"
  else
    echo "selfcheck FAIL: c.JSON 检查已退化"; fails=$((fails+1))
  fi

  # fixture 4：错误链丢失（%v 包 err 且无 %w，应被抓到）
  cat > "$tmp/modules/foo/errchain.go" <<'EOF'
package foo

import "fmt"

func Fixture(err error) error { return fmt.Errorf("wrap: %v", err) }
EOF
  if grep -rn --include='*.go' -E 'fmt\.Errorf\("[^"]*%[vsd][^"]*",[^)]*err[,)]' "$tmp/modules/foo" | grep -v '_test.go' | grep -v '%w' | grep -q 'errchain'; then
    echo "selfcheck: 错误链丢失可检出"
  else
    echo "selfcheck FAIL: 丢链检查已退化"; fails=$((fails+1))
  fi

  if [ "$fails" -gt 0 ]; then
    echo "fixture 自测失败：$fails 类检查已退化"
    exit 1
  fi
  echo "fixture 自测通过（4 类核心检查均可检出反例）"
fi
