#!/bin/bash
#
# TDX 股票数据 API 服务 - 一键部署脚本
#
# 用法:
#   ./deploy.sh              # 本地部署(编译+运行)
#   ./deploy.sh docker       # Docker部署
#   ./deploy.sh stop         # 停止服务
#   ./deploy.sh status       # 查看状态
#   ./deploy.sh build        # 仅编译
#   ./deploy.sh cross        # 交叉编译
#   ./deploy.sh help         # 帮助
#

set -euo pipefail

# ==========================================
# 加载 .env (不覆盖已有环境变量)
# ==========================================
if [ -f .env ]; then
    while IFS='=' read -r key val; do
        key=$(echo "$key" | xargs)
        val=$(echo "$val" | xargs)
        if [ -z "$key" ] || [[ "$key" == \#* ]]; then
            continue
        fi
        if [ -z "${!key:-}" ]; then
            export "$key=$val"
        fi
    done < .env
fi

# ==========================================
# 配置
# ==========================================
BINARY="tdx-server"
PORT="${TDX_PORT:-8080}"
DIR="${TDX_DIR:-data}"
DB_TYPE="${DB_TYPE:-mysql}"
IMAGE="tdx-api"
VERSION="1.0.0"
PID_FILE="tdx-server.pid"
LOG_FILE="tdx-server.log"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# ==========================================
# 辅助函数
# ==========================================
info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

check_go() {
    if ! command -v go &>/dev/null; then
        err "未安装 Go,请先安装: https://go.dev/dl/"
    fi
    local ver=$(go version | awk '{print $3}')
    info "Go 版本: $ver"
}

check_docker() {
    if ! command -v docker &>/dev/null; then
        err "未安装 Docker,请先安装: https://docs.docker.com/get-docker/"
    fi
    info "Docker 版本: $(docker --version | awk '{print $3}' | tr -d ',')"
}

is_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            return 0
        fi
        rm -f "$PID_FILE"
    fi
    return 1
}

# ==========================================
# 命令: 编译
# ==========================================
cmd_build() {
    info "编译 $BINARY ..."
    check_go
    go build -ldflags="-s -w" -o "$BINARY" ./cmd/server/
    ok "编译完成: $BINARY ($(du -h "$BINARY" | awk '{print $1}'))"
}

# ==========================================
# 命令: 本地部署
# ==========================================
cmd_local() {
    echo ""
    echo "========================================="
    echo "  TDX 股票数据 API 服务 - 本地部署"
    echo "========================================="
    echo ""

    # 检查是否已在运行
    if is_running; then
        warn "服务已在运行 (PID: $(cat $PID_FILE))"
        read -p "是否重启? [y/N] " confirm
        if [[ "$confirm" =~ ^[Yy]$ ]]; then
            cmd_stop
        else
            return 0
        fi
    fi

    # 编译
    cmd_build

    # 创建数据目录
    mkdir -p "$DIR"

    # 启动
    info "启动服务 (端口: $PORT, 数据目录: $DIR, 数据库: $DB_TYPE)..."
    nohup ./"$BINARY" -port "$PORT" -dir "$DIR" -db-type "$DB_TYPE" > "$LOG_FILE" 2>&1 &
    local pid=$!
    echo "$pid" > "$PID_FILE"

    # 等待启动
    info "等待服务就绪..."
    local retries=30
    while [ $retries -gt 0 ]; do
        if curl -s --max-time 2 "http://localhost:$PORT/api/server-status" &>/dev/null; then
            break
        fi
        sleep 1
        retries=$((retries - 1))
    done

    if [ $retries -eq 0 ]; then
        err "服务启动超时,请检查日志: $LOG_FILE"
    fi

    echo ""
    ok "服务已启动!"
    echo ""
    echo "  地址:  http://localhost:$PORT"
    echo "  文档:  http://localhost:$PORT/api/docs"
    echo "  测试:  http://localhost:$PORT/"
    echo "  日志:  tail -f $LOG_FILE"
    echo "  停止:  ./deploy.sh stop"
    echo "  状态:  ./deploy.sh status"
    echo ""
}

# ==========================================
# 命令: Docker部署
# ==========================================
cmd_docker() {
    echo ""
    echo "========================================="
    echo "  TDX 股票数据 API 服务 - Docker部署"
    echo "========================================="
    echo ""

    check_docker

    # 检查docker compose
    if docker compose version &>/dev/null; then
        info "Docker Compose: $(docker compose version --short)"
    elif command -v docker-compose &>/dev/null; then
        warn "使用旧版 docker-compose"
    else
        err "未安装 Docker Compose"
    fi

    # 构建并启动
    info "构建镜像并启动容器..."
    docker compose up -d --build

    # 等待健康检查
    info "等待服务就绪..."
    local retries=30
    while [ $retries -gt 0 ]; do
        local status=$(docker inspect --format='{{.State.Health.Status}}' tdx-api 2>/dev/null || echo "unknown")
        if [ "$status" = "healthy" ]; then
            break
        fi
        sleep 2
        retries=$((retries - 1))
    done

    echo ""
    ok "Docker 服务已启动!"
    echo ""
    echo "  地址:  http://localhost:$PORT"
    echo "  文档:  http://localhost:$PORT/api/docs"
    echo "  测试:  http://localhost:$PORT/"
    echo "  日志:  docker compose logs -f"
    echo "  停止:  ./deploy.sh stop"
    echo ""
}

# ==========================================
# 命令: 停止
# ==========================================
cmd_stop() {
    info "停止服务..."

    # 停止本地进程
    if is_running; then
        local pid=$(cat "$PID_FILE")
        kill "$pid" 2>/dev/null || true
        rm -f "$PID_FILE"
        ok "本地服务已停止 (PID: $pid)"
    fi

    # 停止Docker容器
    if command -v docker &>/dev/null && docker ps -q -f name=tdx-api &>/dev/null; then
        if [ -n "$(docker ps -q -f name=tdx-api)" ]; then
            docker compose down 2>/dev/null || true
            ok "Docker 服务已停止"
        fi
    fi
}

# ==========================================
# 命令: 状态
# ==========================================
cmd_status() {
    echo ""
    echo "========================================="
    echo "  TDX 股票数据 API 服务 - 状态"
    echo "========================================="
    echo ""

    # 本地进程
    if is_running; then
        local pid=$(cat "$PID_FILE")
        ok "本地服务运行中 (PID: $pid)"
        # 尝试获取服务状态
        local resp=$(curl -s --max-time 3 "http://localhost:$PORT/api/server-status" 2>/dev/null || echo "")
        if [ -n "$resp" ]; then
            echo "  API响应: $resp" | python3 -m json.tool 2>/dev/null || echo "  API响应: $resp"
        fi
    else
        warn "本地服务未运行"
    fi

    # Docker容器
    if command -v docker &>/dev/null; then
        local container=$(docker ps -q -f name=tdx-api 2>/dev/null || echo "")
        if [ -n "$container" ]; then
            ok "Docker 容器运行中"
            docker ps -f name=tdx-api --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        else
            warn "Docker 容器未运行"
        fi
    fi
    echo ""
}

# ==========================================
# 命令: 交叉编译
# ==========================================
cmd_cross() {
    info "交叉编译..."
    check_go

    local platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
    )

    for platform in "${platforms[@]}"; do
        local goos=$(echo "$platform" | cut -d/ -f1)
        local goarch=$(echo "$platform" | cut -d/ -f2)
        local output="${BINARY}-${goos}-${goarch}"
        info "编译 $output ..."
        CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
            -ldflags="-s -w" \
            -o "$output" \
            ./cmd/server/
    done

    echo ""
    ok "交叉编译完成:"
    ls -lh ${BINARY}-linux-* ${BINARY}-darwin-* 2>/dev/null
}

# ==========================================
# 命令: 帮助
# ==========================================
cmd_help() {
    echo ""
    echo "TDX 股票数据 API 服务 - 一键部署脚本"
    echo ""
    echo "用法: ./deploy.sh <命令>"
    echo ""
    echo "命令:"
    echo "  (无参数)    本地部署(编译+运行)"
    echo "  build       仅编译"
    echo "  docker      Docker部署"
    echo "  stop        停止服务"
    echo "  status      查看状态"
    echo "  cross       交叉编译(linux/darwin amd64/arm64)"
    echo "  help        显示帮助"
    echo ""
    echo "环境变量:"
    echo "  TDX_PORT    服务端口(默认: 8080)"
    echo "  TDX_DIR     数据目录(默认: data)"
    echo "  DB_TYPE     数据库类型 mysql/sqlite(默认: mysql)"
    echo "  DB_HOST     MySQL主机(默认: mysql)"
    echo "  DB_PORT     MySQL端口(默认: 3306)"
    echo "  DB_USER     MySQL用户(默认: tdx)"
    echo "  DB_PASSWORD MySQL密码(默认: tdx123456)"
    echo "  DB_NAME     MySQL数据库(默认: tdx)"
    echo ""
    echo "示例:"
    echo "  ./deploy.sh                # 本地部署,MySQL,端口8080"
    echo "  DB_TYPE=sqlite ./deploy.sh # 本地部署,SQLite"
    echo "  TDX_PORT=9090 ./deploy.sh # 本地部署,端口9090"
    echo "  ./deploy.sh docker        # Docker部署(含MySQL)"
    echo "  ./deploy.sh stop          # 停止服务"
    echo "  ./deploy.sh status        # 查看状态"
    echo ""
}

# ==========================================
# 主入口
# ==========================================
case "${1:-}" in
    build)   cmd_build   ;;
    docker)  cmd_docker  ;;
    stop)    cmd_stop    ;;
    status)  cmd_status  ;;
    cross)   cmd_cross   ;;
    help|-h) cmd_help    ;;
    "")      cmd_local   ;;
    *)
        err "未知命令: $1\n运行 './deploy.sh help' 查看帮助"
        ;;
esac
