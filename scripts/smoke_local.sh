#!/usr/bin/env bash
# 本地 e2e 验收统一入口（批次四 H1，见 scripts/e2e_README.md 的依赖总览）。
#
# 按环境依赖分级，自动跑「当前环境能跑」的组：
#   tier1 纯本地组（仅 backend + postgres）：e2e_referral、e2e_user_closure
#   tier2 mock 上游组（需 p5-mock 容器 + 种子商品/渠道）：password_chain、p5_fulfillment
#   tier3 凭证组（需 openapp 应用凭证，走 --app-id/--app-secret）：e2e_p6_open_api
#
# 用法：
#   scripts/smoke_local.sh              # 探测环境，跑所有可跑的组
#   scripts/smoke_local.sh --tier 1     # 只跑纯本地组
#   scripts/smoke_local.sh --tier 12    # 跑纯本地 + mock 上游组
#
# 每组内部脚本失败立即终止（set -e），组与组之间不互相拖累。
set -euo pipefail

cd "$(dirname "$0")/.."

TIER="${TIER:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tier) TIER="$2"; shift 2 ;;
    *) echo "未知参数: $1" >&2; exit 2 ;;
  esac
done

# 环境探测
backend_up() { curl -sf -o /dev/null -m 3 "http://127.0.0.1:8080/health" 2>/dev/null; }
p5_up() { docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "p5-mock"; }

echo "=== 环境探测 ==="
if backend_up; then
  echo "  backend: 可达"
else
  echo "  backend: 不可达（127.0.0.1:8080/health）——tier1/2 均无法运行"
  exit 1
fi
if p5_up; then
  echo "  p5-mock: 在跑"
else
  echo "  p5-mock: 不在跑（tier2 需要）"
fi

FAILED=0
run_group() {
  local label="$1"; shift
  echo
  echo "=== $label ==="
  for script in "$@"; do
    echo "--- $script ---"
    if ! python3 "scripts/$script"; then
      echo "!!! $script 失败"
      FAILED=1
    fi
  done
}

# tier1 纯本地组（backend 可达即可）
if [[ -z "$TIER" ]] || [[ "$TIER" == *1* ]]; then
  run_group "tier1 纯本地组" e2e_referral.py e2e_user_closure.py
fi

# tier2 mock 上游组（p5-mock 在跑才执行；未探测到则提示跳过）
if { [[ -z "$TIER" ]] || [[ "$TIER" == *2* ]]; } && p5_up; then
  run_group "tier2 mock 上游组" e2e_password_chain.py e2e_p5_fulfillment.py
elif [[ "$TIER" == *2* ]]; then
  echo "!!! 指定了 tier2 但 p5-mock 不在跑，跳过"
  FAILED=1
fi

# tier3 凭证组：需要显式传凭证，从环境变量取
if [[ "$TIER" == *3* ]]; then
  if [[ -z "${OPEN_APP_ID:-}" || -z "${OPEN_APP_SECRET:-}" ]]; then
    echo "!!! tier3 需要 OPEN_APP_ID / OPEN_APP_SECRET 环境变量"
    FAILED=1
  else
    echo
    echo "=== tier3 凭证组 ==="
    echo "--- e2e_p6_open_api.py ---"
    if ! python3 scripts/e2e_p6_open_api.py --app-id "$OPEN_APP_ID" --app-secret "$OPEN_APP_SECRET"; then
      echo "!!! e2e_p6_open_api.py 失败"
      FAILED=1
    fi
  fi
fi

if [[ "$FAILED" -ne 0 ]]; then
  echo
  echo "冒烟结果：存在失败"
  exit 1
fi
echo
echo "冒烟结果：全部通过"
