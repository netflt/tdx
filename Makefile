# TDX 股票数据 API 服务
#
# 用法:
#   make            - 编译
#   make run        - 编译并运行
#   make clean      - 清理构建产物
#   make docker     - 构建 Docker 镜像
#   make docker-up  - Docker Compose 启动
#   make docker-down- Docker Compose 停止
#   make deploy     - 一键部署(本地)
#   make cross      - 交叉编译 linux/darwin

BINARY   := tdx-server
PORT     ?= 8080
DIR      ?= data
IMAGE    := tdx-api
VERSION  := 1.0.0
GO       := go
LDFLAGS  := -s -w -X main.Version=$(VERSION)

.PHONY: all build run clean test docker docker-up docker-down deploy cross lint

all: build

# 编译
build:
	@echo "编译 $(BINARY)..."
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server/

# 编译并运行
run: build
	@echo "启动服务 (端口: $(PORT), 数据目录: $(DIR))..."
	./$(BINARY) -port $(PORT) -dir $(DIR)

# 清理
clean:
	@echo "清理构建产物..."
	rm -f $(BINARY)
	rm -rf data/database

# 测试
test:
	$(GO) test ./... -count=1

# 代码检查
lint:
	golangci-lint run ./...

# 交叉编译
cross:
	@echo "交叉编译..."
	GOOS=linux   GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64   ./cmd/server/
	GOOS=linux   GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64   ./cmd/server/
	GOOS=darwin  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64  ./cmd/server/
	GOOS=darwin  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64  ./cmd/server/
	@echo "完成: $(BINARY)-linux-amd64, $(BINARY)-linux-arm64, $(BINARY)-darwin-amd64, $(BINARY)-darwin-arm64"

# Docker 构建
docker:
	@echo "构建 Docker 镜像 $(IMAGE):$(VERSION)..."
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

# Docker Compose 启动
docker-up:
	docker compose up -d

# Docker Compose 停止
docker-down:
	docker compose down

# Docker Compose 日志
docker-logs:
	docker compose logs -f

# 一键部署(本地)
deploy: build
	@echo "========================================="
	@echo "  TDX 股票数据 API 服务部署"
	@echo "========================================="
	@mkdir -p $(DIR)
	@echo "启动服务..."
	./$(BINARY) -port $(PORT) -dir $(DIR)
