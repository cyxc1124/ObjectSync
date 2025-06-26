# Ceph Backup Tool Makefile
# 用于管理构建、开发和发布任务

# 项目信息
PROJECT_NAME = objectsync
VERSION = 2.0.0
BUILD_TIME = $(shell date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go相关变量
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOFMT = $(GOCMD) fmt

# 目录定义
BIN_DIR = bin
RELEASE_DIR = releases
CMD_DIR = cmd
INTERNAL_DIR = internal
SCRIPTS_DIR = scripts
CONFIGS_DIR = configs
DOCS_DIR = docs

# 构建标志
LDFLAGS = -ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

# 默认目标
.PHONY: all
all: clean deps test build

# 帮助信息
.PHONY: help
help:
	@echo "Ceph Backup Tool - 构建工具"
	@echo ""
	@echo "可用的命令:"
	@echo "  make all          - 完整构建流程 (清理+依赖+测试+构建)"
	@echo "  make build        - 构建当前平台版本"
	@echo "  make build-all    - 构建所有平台版本"
	@echo "  make clean        - 清理构建文件"
	@echo "  make deps         - 下载依赖"
	@echo "  make test         - 运行测试"
	@echo "  make fmt          - 格式化代码"
	@echo "  make lint         - 运行代码检查"
	@echo "  make install      - 安装到本地"
	@echo "  make uninstall    - 从本地卸载"
	@echo "  make release      - 创建发布包"
	@echo "  make dev          - 开发模式运行"
	@echo "  make docker       - 构建Docker镜像"

# 清理构建文件
.PHONY: clean
clean:
	@echo "🧹 清理构建文件..."
	$(GOCLEAN)
	@rm -rf $(BIN_DIR)/
	@rm -rf $(RELEASE_DIR)/
	@echo "✅ 清理完成"

# 下载依赖
.PHONY: deps
deps:
	@echo "📦 下载依赖..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ 依赖下载完成"

# 格式化代码
.PHONY: fmt
fmt:
	@echo "🎨 格式化代码..."
	$(GOFMT) ./...
	@echo "✅ 代码格式化完成"

# 代码检查
.PHONY: lint
lint:
	@echo "🔍 运行代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint 未安装，跳过检查"; \
		echo "   安装命令: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 运行测试
.PHONY: test
test:
	@echo "🧪 运行测试..."
	$(GOTEST) -v ./...
	@echo "✅ 测试完成"

# 构建当前平台版本
.PHONY: build
build:
	@echo "🔨 构建 $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME) $(CMD_DIR)/main.go
	@echo "✅ 构建完成: $(BIN_DIR)/$(PROJECT_NAME)"

# 构建所有平台版本
.PHONY: build-all
build-all:
	@echo "🌍 构建多平台版本..."
	@mkdir -p $(BIN_DIR)
	
	@echo "  🔨 Windows AMD64..."
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-windows-amd64.exe $(CMD_DIR)/main.go
	
	@echo "  🔨 Windows ARM64..."
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-windows-arm64.exe $(CMD_DIR)/main.go
	
	@echo "  🔨 Linux AMD64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-linux-amd64 $(CMD_DIR)/main.go
	
	@echo "  🔨 Linux ARM64..."
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-linux-arm64 $(CMD_DIR)/main.go
	
	@echo "  🔨 macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-darwin-amd64 $(CMD_DIR)/main.go
	
	@echo "  🔨 macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-darwin-arm64 $(CMD_DIR)/main.go
	
	@echo "✅ 多平台构建完成"

# 安装到本地
.PHONY: install
install: build
	@echo "📦 安装到本地..."
	@if [ "$(shell uname)" = "Darwin" ]; then \
		INSTALL_DIR="/usr/local/bin"; \
	elif [ "$(shell uname)" = "Linux" ]; then \
		if [ "$(shell id -u)" = "0" ]; then \
			INSTALL_DIR="/usr/local/bin"; \
		else \
			INSTALL_DIR="$(HOME)/.local/bin"; \
			mkdir -p $$INSTALL_DIR; \
		fi; \
	else \
		echo "❌ 不支持的操作系统"; \
		exit 1; \
	fi; \
	cp $(BIN_DIR)/$(PROJECT_NAME) $$INSTALL_DIR/; \
	chmod +x $$INSTALL_DIR/$(PROJECT_NAME); \
	echo "✅ 已安装到: $$INSTALL_DIR/$(PROJECT_NAME)"

# 从本地卸载
.PHONY: uninstall
uninstall:
	@echo "🗑️  从本地卸载..."
	@for dir in /usr/local/bin $(HOME)/.local/bin; do \
		if [ -f $$dir/$(PROJECT_NAME) ]; then \
			rm $$dir/$(PROJECT_NAME); \
			echo "✅ 已从 $$dir 删除"; \
		fi; \
	done

# 创建发布包
.PHONY: release
release: build-all
	@echo "📦 创建发布包..."
	@mkdir -p $(RELEASE_DIR)
	
	@platforms="windows-amd64 windows-arm64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64"; \
	for platform in $$platforms; do \
		echo "  📦 创建 $$platform 发布包..."; \
		temp_dir="$(RELEASE_DIR)/temp-$$platform"; \
		release_name="$(PROJECT_NAME)-v$(VERSION)-$$platform"; \
		\
		rm -rf $$temp_dir; \
		mkdir -p $$temp_dir; \
		\
		if echo $$platform | grep -q windows; then \
			cp $(BIN_DIR)/$(PROJECT_NAME)-$$platform.exe $$temp_dir/$(PROJECT_NAME).exe; \
		else \
			cp $(BIN_DIR)/$(PROJECT_NAME)-$$platform $$temp_dir/$(PROJECT_NAME); \
			chmod +x $$temp_dir/$(PROJECT_NAME); \
		fi; \
		\
		mkdir -p $$temp_dir/scripts $$temp_dir/docs; \
		[ -f config.example.yaml ] && cp config.example.yaml $$temp_dir/ 2>/dev/null || true; \
		[ -d $(DOCS_DIR) ] && cp -r $(DOCS_DIR)/* $$temp_dir/docs/ 2>/dev/null || true; \
		\
		if echo $$platform | grep -q windows; then \
			cp $(SCRIPTS_DIR)/*.bat $$temp_dir/scripts/ 2>/dev/null || true; \
		else \
			cp $(SCRIPTS_DIR)/*.sh $$temp_dir/scripts/ 2>/dev/null || true; \
			chmod +x $$temp_dir/scripts/*.sh 2>/dev/null || true; \
		fi; \
		\
		if command -v zip >/dev/null 2>&1; then \
			(cd $$temp_dir && zip -r ../$$release_name.zip . >/dev/null); \
		else \
			tar -czf $(RELEASE_DIR)/$$release_name.tar.gz -C $$temp_dir .; \
		fi; \
		\
		rm -rf $$temp_dir; \
	done
	
	@echo "✅ 发布包创建完成: $(RELEASE_DIR)/"

# 开发模式运行
.PHONY: dev
dev:
	@echo "🚀 开发模式运行..."
	@$(GOCMD) run $(CMD_DIR)/main.go $(ARGS)

# 构建Docker镜像
.PHONY: docker
docker:
	@echo "🐳 构建Docker镜像..."
	@if [ -f Dockerfile ]; then \
		docker build -t $(PROJECT_NAME):$(VERSION) .; \
		docker tag $(PROJECT_NAME):$(VERSION) $(PROJECT_NAME):latest; \
		echo "✅ Docker镜像构建完成"; \
		echo "  $(PROJECT_NAME):$(VERSION)"; \
		echo "  $(PROJECT_NAME):latest"; \
	else \
		echo "❌ 未找到Dockerfile"; \
	fi

# 显示项目信息
.PHONY: info
info:
	@echo "📋 项目信息"
	@echo "  项目名称: $(PROJECT_NAME)"
	@echo "  版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Git提交: $(GIT_COMMIT)"
	@echo "  Go版本: $(shell go version)"

# 快速构建脚本
.PHONY: quick
quick:
	@echo "⚡ 快速构建..."
	@make build
	@echo "🎯 运行测试命令: ./$(BIN_DIR)/$(PROJECT_NAME) --help" 