# Go-Backend-Core Makefile
# 用法: make build        # debug 构建
#       make release      # release 构建
#       make run          # 本地启动
#       make test         # 运行测试
#       make lint         # 代码检查
#       make clean        # 清理

BINARY ?= server
SCRIPTS := scripts

.PHONY: build release run test lint clean docker-build coverage coverage-html

build: ## debug 构建（保留调试符号）
	@bash $(SCRIPTS)/build.sh debug $(BINARY)

release: ## release 构建（剥离调试信息）
	@bash $(SCRIPTS)/build.sh release $(BINARY)

run: build ## 构建并本地启动
	@./$(BINARY)

test: ## 运行全部单元测试
	@bash $(SCRIPTS)/test.sh

test-race: ## 运行竞态检测（需 CGO_ENABLED=1，Linux/macOS）
	CGO_ENABLED=1 go test -race -count=1 -short ./... 2>&1

test-bench: ## 运行所有基准测试
	go test -bench=. -benchmem -count=1 ./... 2>&1

coverage: ## 生成覆盖率报告（终端文本）
	@bash $(SCRIPTS)/coverage.sh

coverage-html: ## 生成并打开覆盖率 HTML 报告
	@bash $(SCRIPTS)/coverage.sh html

lint: ## 运行 golangci-lint
	@bash $(SCRIPTS)/lint.sh

clean: ## 清理构建产物
	@bash $(SCRIPTS)/clean.sh

docker-build: ## 构建 Docker 镜像
	docker compose build

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
