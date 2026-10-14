#!/usr/bin/env bash
# 依赖漏洞门禁（批次四 D3，CONTRIBUTING §10）。
# govulncheck 默认 -mode=source：只报「代码实际调用链可达」的漏洞；
# 模块存在但代码未触达的漏洞（如 x/crypto/openpgp 弃用告警）不阻断，
# 避免为修不可达的包盲目升级破坏依赖面。
# 退出码：0 = 无可达漏洞，1 = 工具运行失败，3 = 存在可达漏洞。
set -euo pipefail

cd "$(dirname "$0")/../backend"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck 未安装：go install golang.org/x/vuln/cmd/govulncheck@latest"
  exit 1
fi

govulncheck ./...
