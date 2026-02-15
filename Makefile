.PHONY: build run clean test

# 项目名称
PROJECT_NAME := lingguard
BUILD_DIR := .
CMD_DIR := cmd/lingguard

# Go 参数
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# 主程序
MAIN := ./

# 构建参数
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 默认目标
all: clean deps build

# 下载依赖
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# 构建
build:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME) ./$(CMD_DIR)

# 运行
run: build
	./$(BUILD_DIR)/$(PROJECT_NAME)

# 清理
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# 测试
test:
	$(GOTEST) -v ./...

# 安装
install: build
	cp $(BUILD_DIR)/$(PROJECT_NAME) /usr/local/bin/

# Docker 构建
docker:
	docker build -t $(PROJECT_NAME):$(VERSION) .

# 开发模式
dev:
	$(GOCMD) run ./$(CMD_DIR)

# 格式化代码
fmt:
	$(GOCMD) fmt ./...

# 静态检查
lint:
	golangci-lint run

# 帮助
help:
	@echo "可用目标:"
	@echo "  make build    - 构建项目"
	@echo "  make run      - 构建并运行"
	@echo "  make clean    - 清理构建产物"
	@echo "  make test     - 运行测试"
	@echo "  make deps     - 下载依赖"
	@echo "  make install  - 安装到系统"
	@echo "  make docker   - 构建 Docker 镜像"
	@echo "  make dev      - 开发模式运行"
	@echo "  make fmt      - 格式化代码"
	@echo "  make lint     - 静态检查"
