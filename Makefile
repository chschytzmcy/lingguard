.PHONY: all build run clean test deps install docker dev fmt lint help

# 项目配置
PROJECT := lingguard
CMD_DIR := cmd/lingguard

# 构建信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 默认目标
all: build

# 下载依赖
deps:
	go mod download
	go mod tidy

# 构建 - 输出到当前目录
build:
	go build $(LDFLAGS) -o $(PROJECT) ./$(CMD_DIR)

# 构建并运行
run: build
	./$(PROJECT)

# 清理
clean:
	go clean
	rm -f $(PROJECT)

# 测试
test:
	go test -v ./...

# 安装到系统
install: build
	cp $(PROJECT) /usr/local/bin/

# Docker 构建
docker:
	docker build -t $(PROJECT):$(VERSION) .

# 开发模式（直接运行，不构建）
dev:
	go run ./$(CMD_DIR)

# 格式化代码
fmt:
	go fmt ./...

# 静态检查
lint:
	golangci-lint run

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build    - 构建项目（输出到当前目录）"
	@echo "  make run      - 构建并运行"
	@echo "  make clean    - 清理构建产物"
	@echo "  make test     - 运行测试"
	@echo "  make deps     - 下载依赖"
	@echo "  make install  - 安装到 /usr/local/bin"
	@echo "  make docker   - 构建 Docker 镜像"
	@echo "  make dev      - 开发模式运行"
	@echo "  make fmt      - 格式化代码"
	@echo "  make lint     - 静态检查"
