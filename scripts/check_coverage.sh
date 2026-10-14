#!/usr/bin/env bash
# 核心链路覆盖率门禁（批次四 D1，CONTRIBUTING §10）。
# 只盯交易链路两个核心包：下单/退款/开通是资金与履约主链路，
# 其余包的覆盖率提升由开发者自行保证（go test -cover ./internal/modules/... 全量报告）。
#
# 阈值语义：低于阈值即 fail；阈值上调（棘轮）时同步更新本文件与 CONTRIBUTING。
set -euo pipefail

cd "$(dirname "$0")/../backend"

declare -A TARGETS=(
  ["./internal/modules/admin/order/service/"]=70
  ["./internal/modules/uc/order/service/"]=70
)

FAILED=0
for pkg in "${!TARGETS[@]}"; do
  threshold="${TARGETS[$pkg]}"
  # -coverprofile 落到临时目录，避免把覆盖产物留进工作区（后端根目录有 .gitignore 但保持干净）。
  profile=$(mktemp)
  out=$(go test -count=1 -coverprofile="$profile" "$pkg" 2>&1) || {
    echo "覆盖率门禁：$pkg 测试失败"
    echo "$out" | tail -5
    FAILED=1
    rm -f "$profile"
    continue
  }
  # 取总覆盖率：go tool cover -func 的 total 行形如 "total: (statements) 79.8%"
  pct=$(go tool cover -func="$profile" | awk '/^total:/{gsub("%","",$NF); print $NF}')
  rm -f "$profile"
  echo "覆盖率：$pkg = ${pct}%（阈值 ${threshold}%）"
  if awk -v p="$pct" -v t="$threshold" 'BEGIN{exit !(p >= t)}'; then
    echo "  PASS"
  else
    echo "  FAIL：低于阈值 ${threshold}%"
    FAILED=1
  fi
done

if [ "$FAILED" -ne 0 ]; then
  echo "覆盖率门禁未通过"
  exit 1
fi
echo "覆盖率门禁通过"
