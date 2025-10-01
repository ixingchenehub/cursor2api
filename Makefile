.PHONY: build run test clean help lint golangci-lint dev env-setup

# 变量定义
BINARY_NAME=cursor2api
BUILD_DIR=bin
MAIN_FILE=main.go

# 默认目标
all: build

# 编译项目
build:
	@echo "🔨 编译项目..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ 编译完成: $(BUILD_DIR)/$(BINARY_NAME)"

# 编译并运行
dev: build
	@echo "🚀 启动开发服务器..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# 直接运行（不编译）
run:
	@echo "🚀 运行服务..."
	@go run $(MAIN_FILE)

# 运行测试
test:
	@echo "🧪 运行测试..."
	@go test -v ./...

# 清理构建文件
clean:
	@echo "🧹 清理构建文件..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ 清理完成"

# 格式化代码
fmt:
	@echo "📝 格式化代码..."
	@go fmt ./...

# 代码检查 (使用 go vet)
lint:
	@echo "🔍 检查代码 (go vet)..."
	@go vet ./...

# 完整代码检查 (使用 golangci-lint)
golangci-lint:
	@echo "🔍 完整代码检查 (golangci-lint)..."
	@golangci-lint run --config .golangci.yml

# 安装依赖
deps:
	@echo "📦 安装依赖..."
	@go mod download
	@go mod tidy

# 环境配置
env-setup:
	@if [ ! -f .env ]; then \
		echo "📝 创建 .env 文件..."; \
		cp .env.example .env; \
		echo "✅ .env 文件已创建，请根据需要修改配置"; \
	else \
		echo "⚠️  .env 文件已存在"; \
	fi

# 显示帮助信息
help:
	@echo "==========================================="
	@echo "  Cursor2API - Makefile 命令"
	@echo "==========================================="
	@echo ""
	@echo "📦 构建相关:"
	@echo "  make build          - 编译项目"
	@echo "  make clean          - 清理构建文件"
	@echo ""
	@echo "🚀 运行相关:"
	@echo "  make run            - 直接运行（不编译）"
	@echo "  make dev            - 编译并运行（开发模式）"
	@echo ""
	@echo "🧪 测试相关:"
	@echo "  make test           - 运行测试"
	@echo ""
	@echo "🔍 代码质量:"
	@echo "  make fmt            - 格式化代码"
	@echo "  make lint           - 基础代码检查 (go vet)"
	@echo "  make golangci-lint  - 完整代码检查 (golangci-lint)"
	@echo ""
	@echo "⚙️  环境配置:"
	@echo "  make deps           - 安装依赖"
	@echo "  make env-setup      - 创建 .env 配置文件"
	@echo ""
	@echo "❓ 其他:"
	@echo "  make help           - 显示此帮助信息"
	@echo ""
