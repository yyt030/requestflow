# RequestFlow Makefile
# 配置驱动的 HTTP/TCP request 流程自动化工具（纯 Go，无 cgo）。
#
# 常用:
#   make            等价于 make build
#   make build      构建当前平台二进制到 ./requestflow（注入版本信息）
#   make test       运行全部单元测试
#   make check      格式/静态检查/测试/行数约束 一站式校验
#   make run ARGS="-run scenarios/demo.json"   构建并运行
#   make pack       交叉编译打 windows+linux amd64 包（逻辑内联在本文件）
#   make pack PLATFORMS=linux/arm64 FORMAT=zip OUT=dist  自定义平台/格式/输出
#   make clean      清理构建与测试产物

# ---------- 基本变量 ----------
MODULE      := requestflow
BIN         := requestflow
VERSION_PKG := $(MODULE)/version

GO          ?= go
GOFLAGS     ?=
CGO_ENABLED ?= 0

# 版本信息：git describe + 提交 + UTC 构建时间（build 与 pack 共用，保持一致）
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILTTIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# 打包参数（可用 make pack PLATFORMS=... FORMAT=... OUT=... 覆盖）
PLATFORMS ?= windows/amd64 linux/amd64
FORMAT    ?= tar.gz
OUT       ?= dist

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
# 打包逻辑以内联 bash 脚本实现：define 存脚本体，$(file >...) 写出临时脚本后执行，
# 全部集中在本文件、无外部脚本依赖。配置由上方 make 变量注入。
define PACK_SCRIPT
#!/usr/bin/env bash
set -euo pipefail

platforms="$(PLATFORMS)"
OUT="$(OUT)"
FORMAT="$(FORMAT)"
ver="$(VERSION)"
commit="$(COMMIT)"
build_time="$(BUILTTIME)"

echo "版本:     $$ver"
echo "提交:     $$commit"
echo "格式:     $$FORMAT"
echo "输出目录: $$OUT"
echo "平台:     $$platforms"

mkdir -p "$$OUT"
rm -f "$$OUT"/requestflow-*."$$FORMAT" "$$OUT"/requestflow-*."$$FORMAT".sha256 \
      "$$OUT"/requestflow-*.tar.gz "$$OUT"/requestflow-*.tar.gz.sha256 \
      "$$OUT"/requestflow-*.zip "$$OUT"/requestflow-*.zip.sha256

pack_one() {
  local platform="$$1"
  local GOOS_T="$${platform%/*}"
  local GOARCH_T="$${platform#*/}"
  local BIN="requestflow"
  [[ "$$GOOS_T" == "windows" ]] && BIN="requestflow.exe"
  local PKG="requestflow-$${ver}-$${GOOS_T}-$${GOARCH_T}"
  local STAGE="$$OUT/$$PKG"

  echo ""
  echo "---- $$GOOS_T/$$GOARCH_T ----"
  echo "产物: $$OUT/$${PKG}.$$FORMAT"

  rm -rf "$$STAGE"
  mkdir -p "$$STAGE/scenarios"

  if [[ "$$GOOS_T" == "windows" && "$$GOARCH_T" == "amd64" ]]; then
    rm -f rsrc_windows_*.syso
    if command -v go-winres >/dev/null 2>&1 || [[ -x "$$(go env GOPATH)/bin/go-winres" ]]; then
      WINRES="$$(go env GOPATH)/bin/go-winres"
      [[ -x "$$WINRES" ]] || WINRES="go-winres"
      "$$WINRES" simply --icon assets/icon.png --arch amd64 --out rsrc \
        --manifest gui --product-name "RequestFlow" \
        --file-description "request流程自动化工具" \
        --file-version "$${ver#v}" --product-version "$${ver#v}" >/dev/null 2>&1 \
        && echo "图标资源: 已生成 rsrc_windows_amd64.syso" \
        || echo "图标资源: go-winres 生成失败，使用无图标构建"
    else
      echo "图标资源: 未安装 go-winres，使用无图标构建（可 go install github.com/tc-hib/go-winres@latest）"
    fi
  fi

  CGO_ENABLED=0 GOOS="$$GOOS_T" GOARCH="$$GOARCH_T" go build -trimpath -ldflags "\
    -s -w \
    -X requestflow/version.Version=$${ver} \
    -X requestflow/version.Commit=$${commit} \
    -X requestflow/version.BuildTime=$${build_time}" \
    -o "$$STAGE/$$BIN" .

  rm -f rsrc_windows_*.syso
  cp README.md "$$STAGE/"
  cp scenarios/*.json "$$STAGE/scenarios/"

  if [[ "$$GOOS_T" == "windows" ]]; then
    RUNBIN="requestflow.exe"; GUI_TIP="双击 $$RUNBIN"; EXTRA=""
  else
    RUNBIN="./requestflow"
    GUI_TIP="在图形桌面环境中执行 $$RUNBIN；若卡死/白屏，用 $$RUNBIN -software 强制软件渲染"
    EXTRA="【Linux GUI 依赖】GTK3 + WebKitGTK
  Debian/Ubuntu: sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
  Fedora:        sudo dnf install gtk3 webkit2gtk4.1
  无图形环境请用下方的 -run 无界面模式。

"
  fi
  cat > "$$STAGE/使用说明.txt" <<EOF
RequestFlow $$ver
========================================

【运行 GUI】
  $$GUI_TIP

$${EXTRA}【无界面运行示例】
  $$RUNBIN -run scenarios/demo.json

【查看版本】
  $$RUNBIN -version

将 scenarios/ 下的 JSON 场景复制到任意位置，用 GUI 的「打开」加载即可。
场景为纯文本 JSON，可纳入版本管理、跨机器复用。

完整说明见 README.md
项目地址: https://github.com/yyt030/requestflow
EOF

  if [[ "$$FORMAT" == "zip" ]]; then
    ARCHIVE="$${PKG}.zip"
    ( cd "$$OUT" && zip -q -r "$$ARCHIVE" "$$PKG" )
  else
    ARCHIVE="$${PKG}.tar.gz"
    ( cd "$$OUT" && tar -czf "$$ARCHIVE" "$$PKG" )
  fi
  rm -rf "$$STAGE"
  ( cd "$$OUT" && sha256sum "$$ARCHIVE" > "$${ARCHIVE}.sha256" )

  echo "打包完成: $$OUT/$$ARCHIVE"
  echo "校验文件: $$OUT/$${ARCHIVE}.sha256"
}

for platform in $$platforms; do
  pack_one "$$platform"
done

echo ""
echo "全部平台打包完成。"
endef

PACK_TMP := /tmp/requestflow-make-pack.sh
pack:
	@echo ">> 交叉编译打包（$(PLATFORMS)）"
	$(file >$(PACK_TMP),$(PACK_SCRIPT))
	@bash "$(PACK_TMP)"

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
