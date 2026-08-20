#!/bin/bash

# 后端服务一键启动脚本
# 功能：检查现有服务，关闭并重新启动

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$( dirname "$SCRIPT_DIR" )"
BACKEND_DIR="$PROJECT_ROOT/backend"
DOCKER_COMPOSE_FILE="$BACKEND_DIR/docker-compose.yml"

echo "=== 🚀 正在启动后端服务编排 ==="

# 检查 docker compose 是否可用
if ! docker compose version &> /dev/null
then
    echo "❌ 错误: 未找到 'docker compose' 命令，请确保已安装 Docker Compose V2。"
    exit 1
fi

# 进入后端目录
cd "$BACKEND_DIR" || { echo "❌ 错误: 无法进入目录 $BACKEND_DIR"; exit 1; }

# 检查并关闭已存在的服务
echo "--- 🛑 正在检查并清理旧服务 ---"
docker compose -f "$DOCKER_COMPOSE_FILE" down --remove-orphans

# 启动服务
echo "--- 🏗️  正在启动容器服务 (后台运行) ---"
docker compose -f "$DOCKER_COMPOSE_FILE" up -d --build

# 等待 API 服务就绪 (最多等待 30 秒)
echo "--- ⏳ 正在等待 API 服务就绪 ---"
for i in {1..30}; do
    if curl -s http://localhost:8080/health > /dev/null; then
        echo "✅ API 服务已在线 (localhost:8080)"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "⚠️  警告: API 服务在 30 秒内未响应 health 检查，请检查日志。"
    fi
    sleep 1
done

# 等待服务就绪并显示状态
echo "--- 📊 服务当前运行状态 ---"
docker compose -f "$DOCKER_COMPOSE_FILE" ps

echo "=== ✅ 后端服务已成功编排并启动 ==="
echo "提示: 可以使用 'docker compose logs -f' 查看实时日志。"
