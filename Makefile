# ==========================================================================
# go-backend 常用命令
# 用法：make help 查看全部目标
# ==========================================================================

# ---- 基础变量 ----
APP_NAME    := go-backend
BIN_DIR     := bin
SERVER_BIN  := $(BIN_DIR)/server
MAIN_PKG    := ./cmd/server

# 构建期注入的版本信息，通过 -ldflags 写入 internal/version
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X 'go-backend/internal/version.Version=$(VERSION)' \
	-X 'go-backend/internal/version.Commit=$(COMMIT)' \
	-X 'go-backend/internal/version.BuildTime=$(BUILD_TIME)'

# 交叉编译目标（部署到 Linux 云服务器）
GOOS        ?= linux
GOARCH      ?= amd64

# 静态链接，产出单文件可执行程序，容器镜像可做到极小
BUILD_ENV   := CGO_ENABLED=0

.DEFAULT_GOAL := help

# --------------------------------------------------------------------------
# 开发
# --------------------------------------------------------------------------

.PHONY: help
help: ## 显示所有可用命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## 本地启动服务（读取 config/config.yaml）
	go run $(MAIN_PKG) -c config/config.yaml

.PHONY: dev
dev: ## 开发模式启动（debug 日志）
	LOG_LEVEL=debug APP_MODE=debug go run $(MAIN_PKG)

.PHONY: build
build: ## 编译当前平台可执行文件到 bin/
	@mkdir -p $(BIN_DIR)
	$(BUILD_ENV) go build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) $(MAIN_PKG)
	@echo "构建完成: $(SERVER_BIN)"

.PHONY: build-linux
build-linux: ## 交叉编译 Linux amd64 版本（部署用）
	@mkdir -p $(BIN_DIR)
	$(BUILD_ENV) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN)-$(GOOS)-$(GOARCH) $(MAIN_PKG)
	@echo "构建完成: $(SERVER_BIN)-$(GOOS)-$(GOARCH)"

.PHONY: clean
clean: ## 清理构建产物与运行期数据
	rm -rf $(BIN_DIR) data logs

.PHONY: tidy
tidy: ## 整理依赖
	go mod tidy

# --------------------------------------------------------------------------
# 质量
# --------------------------------------------------------------------------

.PHONY: test
test: ## 运行全部单元测试（带覆盖率）
	# 说明：不能用 -race。本机是 windows/386，实测报
	# " -race is not supported on windows/386"；且 -race 需要 CGO，
	# 而本项目为纯 Go sqlite、全局 CGO_ENABLED=0。
	# 需要竞态检测时，只在 linux/amd64 且 CGO_ENABLED=1 的环境单独跑。
	go test -cover -coverprofile=coverage.out ./...

.PHONY: test-short
test-short: ## 运行单元测试（跳过耗时较长的用例）
	go test -short ./...

.PHONY: cover
cover: test ## 生成并打开覆盖率报告
	go tool cover -html=coverage.out

.PHONY: vet
vet: ## 静态检查
	go vet ./...

.PHONY: fmt
fmt: ## 格式化代码
	gofmt -w -s ./cmd ./internal
	@echo "格式化完成"

.PHONY: lint
lint: fmt vet ## 格式化 + 静态检查

.PHONY: check
check: lint test ## 提交前的完整检查

# --------------------------------------------------------------------------
# 文档
# --------------------------------------------------------------------------

.PHONY: docs
docs: ## 打印 API 文档地址（spec 由 docs 包运行时构造，无需生成）
	@echo "启动服务后访问："
	@echo "  UI:   http://localhost:8080/docs"
	@echo "  spec: http://localhost:8080/openapi.json"
	@echo "APP_MODE=release 时以上路由不注册。"

# --------------------------------------------------------------------------
# 容器
# --------------------------------------------------------------------------

.PHONY: docker-build
docker-build: ## 构建 Docker 镜像
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

.PHONY: up
up: ## 启动开发环境（Go + Postgres + Redis）
	docker compose up -d --build

.PHONY: down
down: ## 停止开发环境
	docker compose down

.PHONY: logs
logs: ## 查看容器日志
	docker compose logs -f app

.PHONY: ps
ps: ## 查看容器状态
	docker compose ps
