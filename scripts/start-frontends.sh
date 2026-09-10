#!/bin/bash

# 前端一键启动脚本
# 功能：一键启动三个前端开发服务
#   frontend-admin (Vue3 + Vite)  -> http://localhost:3000
#   frontend-user  (Vue3 + Vite)  -> http://localhost:3002
#   frontend-site  (Nuxt3)        -> http://localhost:3003
# 用法：
#   ./start-frontends.sh          启动全部（会先释放占用端口）
#   ./start-frontends.sh stop     停止全部
#   ./start-frontends.sh restart  重启全部

set -u

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$( dirname "$SCRIPT_DIR" )"
LOG_DIR="$SCRIPT_DIR/logs"

# 端口与目录的对应关系（保持与各自 vite/nuxt 配置一致）
ADMIN_PORT=3000
USER_PORT=3002
SITE_PORT=3003

ACTION="${1:-start}"

# 释放被占用的端口，避免重复启动导致端口冲突
kill_port() {
    local port="$1"
    local pids
    pids="$(lsof -t -i:"$port" 2>/dev/null)"
    if [ -n "$pids" ]; then
        echo "--- 🛑 端口 $port 已被占用，正在停止旧进程: $pids ---"
        # shellcheck disable=SC2086
        kill $pids 2>/dev/null
        sleep 1
        pids="$(lsof -t -i:"$port" 2>/dev/null)"
        if [ -n "$pids" ]; then
            # shellcheck disable=SC2086
            kill -9 $pids 2>/dev/null
        fi
    fi
}

stop_all() {
    echo "=== 🛑 正在停止三个前端服务 ==="
    kill_port "$ADMIN_PORT"
    kill_port "$USER_PORT"
    kill_port "$SITE_PORT"
    echo "=== ✅ 已停止 ==="
}

start_one() {
    local name="$1" dir="$2" port="$3"
    echo "--- 🚀 启动 $name (端口 $port) ---"

    if [ ! -d "$dir/node_modules" ]; then
        echo "⚠️  $name 未安装依赖，正在执行 npm install ..."
        ( cd "$dir" && npm install ) || { echo "❌ $name 依赖安装失败"; return 1; }
    fi

    # 以当前端口显式覆盖配置，后台运行并输出日志
    ( cd "$dir" && PORT="$port" nohup npm run dev -- --port "$port" --host 0.0.0.0 \
        > "$LOG_DIR/${name}_${port}.log" 2>&1 & echo $! > "$LOG_DIR/${name}_${port}.pid" )

    return 0
}

if [ "$ACTION" = "stop" ]; then
    stop_all
    exit 0
fi

mkdir -p "$LOG_DIR"

if [ "$ACTION" = "restart" ]; then
    stop_all
fi

echo "=== 🚀 正在启动三个前端服务 ==="

# 启动前先释放端口，确保端口与配置一致
kill_port "$ADMIN_PORT"
kill_port "$USER_PORT"
kill_port "$SITE_PORT"

start_one "frontend-admin" "$PROJECT_ROOT/frontend-admin" "$ADMIN_PORT"
start_one "frontend-user"  "$PROJECT_ROOT/frontend-user"  "$USER_PORT"
start_one "frontend-site"  "$PROJECT_ROOT/frontend-site"  "$SITE_PORT"

echo "--- ⏳ 等待服务就绪 ---"
for i in {1..60}; do
    ok=1
    for port in "$ADMIN_PORT" "$USER_PORT" "$SITE_PORT"; do
        curl -s -o /dev/null "http://127.0.0.1:$port" || ok=0
    done
    [ "$ok" = "1" ] && break
    sleep 1
done

echo ""
echo "=== 📊 访问地址 ==="
echo "  管理端 admin : http://localhost:$ADMIN_PORT"
echo "  用户端 user  : http://localhost:$USER_PORT"
echo "  官网   site  : http://localhost:$SITE_PORT"
echo ""
echo "日志目录: $LOG_DIR"
echo "  实时查看: tail -f $LOG_DIR/frontend-admin_${ADMIN_PORT}.log"
echo "停止服务: $0 stop"
