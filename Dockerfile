# ==========================================
# 多阶段构建: 编译 + 运行
# ==========================================

# 阶段1: 编译
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# 先复制依赖文件,利用Docker缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /tdx-server \
    ./cmd/server/

# 阶段2: 运行
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从builder阶段复制二进制
COPY --from=builder /tdx-server /app/tdx-server

# 数据目录
RUN mkdir -p /app/data

# 环境变量(可被docker-compose覆盖)
ENV TDX_PORT=8080
ENV TDX_DIR=/app/data
ENV DB_TYPE=mysql

EXPOSE 8080

ENTRYPOINT ["/app/tdx-server"]
CMD ["-port", "8080", "-dir", "/app/data", "-db-type", "mysql"]
