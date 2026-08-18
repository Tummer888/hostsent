#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose not available"
  exit 1
fi

mkdir -p logs

docker compose up -d postgres

echo "waiting for postgres..."
for i in {1..30}; do
  if docker compose exec -T postgres pg_isready -U hostsent -d hostsent >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

docker compose up -d --build backend

echo "backend started on http://127.0.0.1:8080"
