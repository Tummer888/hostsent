#!/usr/bin/env bash
# ============================================================================
# migrate.sh — 迁移管理 (T5.4)：按序执行 backend/migrations/NNN_*.sql，
# 已应用的记录到 schema_migrations 表，重复执行时跳过（可重复、含回滚说明）。
#
# 用法：  ./scripts/migrate.sh
# 覆盖：  DB_USER=<user> DB_NAME=<db> PGHOST=<host> PGPORT=<port> ./scripts/migrate.sh
# 默认：  docker exec backend-postgres-1 psql -U hostsent -d hostsent
# 注意：  执行破坏性迁移（如 013 DROP 冗余表）前，确保已备份（pg_dump）且新后端已部署。
#         每个迁移文件都自带 BEGIN/COMMIT 与回滚说明，失败时 ON_ERROR_STOP 中止。
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATIONS_DIR="$ROOT/backend/migrations"

# 默认走 docker 里的 postgres；可用环境变量覆盖为远程库。
# 注意：docker exec 需 -i 才能从宿主机 stdin 读入 SQL（见下方 -f - 用法），
#       否则 psql 在容器内找不到宿主机路径的迁移文件。
if [[ -n "${PGHOST:-}" && -n "${PGPORT:-}" ]]; then
  PSQL=(psql -h "$PGHOST" -p "$PGPORT" -U "${DB_USER:-hostsent}" -d "${DB_NAME:-hostsent}")
else
  PSQL=(docker exec -i backend-postgres-1 psql -U "${DB_USER:-hostsent}" -d "${DB_NAME:-hostsent}")
fi

log() { echo "[migrate] $*"; }

"${PSQL[@]}" -v ON_ERROR_STOP=1 -c \
  "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());"

changed=0
for f in "$MIGRATIONS_DIR"/*.sql; do
  [[ -e "$f" ]] || continue
  v="$(basename "$f")"
  applied="$("${PSQL[@]}" -t -A -c "SELECT 1 FROM schema_migrations WHERE version='$v';" | tr -d ' \n')"
  if [[ "$applied" == "1" ]]; then
    log "skip  $v"
    continue
  fi
  log "apply $v"
  # 以 stdin 方式喂入 SQL：迁移文件在宿主机，psql 可能跑在容器内（-i 已开启）。
  "${PSQL[@]}" -v ON_ERROR_STOP=1 -f - < "$f"
  "${PSQL[@]}" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations(version) VALUES ('$v') ON CONFLICT DO NOTHING;"
  changed=1
done

[[ "$changed" == "1" ]] && log "done" || log "no pending migrations"
