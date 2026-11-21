# RequestFlow Makefile
# 配置驱动的 HTTP/TCP request 流程自动化工具（纯 Go，无 cgo）。
#
# 常用:
#   make            等价于 make build
#   make build      构建当前平台二进制到 ./requestflow（注入版本信息）
#   make test       运行全部单元测试
#   make check      格式/静态检查/测试/行数约束 一站式校验
#   make run ARGS="-run scenarios/demo.json"   构建并运行
#   make pack       交叉编译打 windows+linux amd64 包（调用 pack.sh）
#   make clean      清理构建与测试产物

# ---------- 基本变量 ----------
MODULE      := requestflow
BIN         := requestflow
VERSION_PKG := $(MODULE)/version

GO          ?= go
GOFLAGS     ?=
CGO_ENABLED ?= 0

# 版本信息：与 build.sh / pack.sh 保持一致（git describe + 提交 + UTC 构建时间）
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILTTIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).BuildTime=$(BUILTTIME)

# 包列表
PKGS := $(shell $(GO) list ./... 2>/dev/null)

# goimports：优先用已安装的，缺失时给出安装提示（不强制联网）
GOIMPORTS ?= $(shell command -v goimports 2>/dev/null || echo $(shell $(GO) env GOPATH)/bin/goimports)

# 单文件行数上限约束
MAX_LINES := 300

.DEFAULT_GOAL := build
.PHONY: all build test vet fmt fmt-check lint check line-check run clean install pack help

# ---------- 构建 ----------
all: build

build:
	@echo ">> 构建 $(BIN)  version=$(VERSION) commit=$(COMMIT)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	@echo ">> 安装到 GOBIN"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) install $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" .

run: build
	@./$(BIN) $(ARGS)

# ---------- 质量检查 ----------
test:
	@echo ">> 单元测试"
	$(GO) test ./...

test-race:
	@echo ">> 竞态检测"
	$(GO) test -race ./engine/

vet:
	@echo ">> go vet"
	$(GO) vet ./...

fmt:
	@echo ">> 格式化"
	$(GO) fmt ./...
	@if [ -x "$(GOIMPORTS)" ]; then $(GOIMPORTS) -w .; else echo "提示: 未找到 goimports，跳过 import 整理（go install golang.org/x/tools/cmd/goimports@latest）"; fi

fmt-check:
	@echo ">> 格式检查（gofmt -l 应为空）"
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "以下文件未格式化:"; echo "$$out"; exit 1; fi

lint: vet fmt-check

# line-check 校验所有 .go 单文件行数低于 MAX_LINES（排除 .git）。
line-check:
	@echo ">> 单文件行数约束 (< $(MAX_LINES) 行)"
	@bad=$$(find . -name '*.go' -not -path './.git/*' -exec wc -l {} + \
		| awk -v m=$(MAX_LINES) '$$2 != "total" && $$1 >= m {print $$2 " (" $$1 " 行)"}'); \
	if [ -n "$$bad" ]; then echo "以下文件超过 $(MAX_LINES) 行:"; echo "$$bad"; exit 1; else echo "OK: 所有 .go 文件 < $(MAX_LINES) 行"; fi

# check 一站式校验：格式 + 静态 + 行数 + 测试 + 构建
check: fmt-check vet line-check test build
	@echo ">> 全部检查通过"

# ---------- 打包 ----------
pack:
	@echo ">> 交叉编译打包（windows/amd64 + linux/amd64）"
	./pack.sh

# ---------- 清理 ----------
clean:
	@echo ">> 清理"
	rm -f $(BIN) requestflow.exe
	rm -rf dist
	rm -f rsrc_windows_*.syso
	$(GO) clean ./... 2>/dev/null || true

# ---------- 帮助 ----------
help:
	@sed -n '2,20p' Makefile
