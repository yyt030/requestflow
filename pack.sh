#!/usr/bin/env bash
# RequestFlow 打包脚本：默认只打 Windows（windows/amd64），版本号从 git 自动注入。
#
# 用法:
#   ./pack.sh                          # 默认打包 windows/amd64 → tar.gz
#   ./pack.sh linux/amd64              # 打 Linux 平台（tar.gz）
#   FORMAT=zip ./pack.sh               # 改用 zip 格式
#   VERSION=v2.0.0 ./pack.sh           # 手动覆盖版本号（CI 场景）
#   OUT=/path/to/dist ./pack.sh        # 指定输出目录
#
# 产物: <OUT>/requestflow-<版本>-<平台>.tar.gz
#   内含: requestflow(.exe，含应用图标) · README.md · scenarios/ 示例场景
# Windows 图标来自 assets/icon.png，经 go-winres 生成 rsrc_windows_amd64.syso 自动链接。

set -euo pipefail
cd "$(dirname "$0")"

# ---------- 平台（默认 windows/amd64）----------
PLATFORM="${1:-windows/amd64}"
GOOS_T="${PLATFORM%/*}"
GOARCH_T="${PLATFORM#*/}"
OUT="${OUT:-dist}"
FORMAT="${FORMAT:-tar.gz}"   # tar.gz（默认） | zip

# ---------- 版本信息（与 build.sh 一致）----------
if [[ -n "${VERSION:-}" ]]; then
  ver="$VERSION"
elif git rev-parse --git-dir >/dev/null 2>&1; then
  ver="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
else
  ver="dev"
fi
if git rev-parse --git-dir >/dev/null 2>&1; then
  commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
else
  commit="unknown"
fi
build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

BIN="requestflow"
[[ "$GOOS_T" == "windows" ]] && BIN="requestflow.exe"
PKG="requestflow-${ver}-${GOOS_T}-${GOARCH_T}"

echo "平台:     $GOOS_T/$GOARCH_T"
echo "版本:     $ver"
echo "提交:     $commit"
echo "格式:     $FORMAT"
echo "产物:     $OUT/${PKG}.${FORMAT}"

# ---------- 暂存目录 ----------
STAGE="$OUT/$PKG"
rm -rf "$STAGE"
mkdir -p "$STAGE/scenarios"

# ---------- Windows 图标资源（go-winres 生成 .syso，go build 自动链接）----------
# syso 仅对 windows/amd64 生效；其他平台不生成、不链接。
if [[ "$GOOS_T" == "windows" && "$GOARCH_T" == "amd64" ]]; then
  rm -f rsrc_windows_*.syso
  if command -v go-winres >/dev/null 2>&1 || [[ -x "$(go env GOPATH)/bin/go-winres" ]]; then
    WINRES="$(go env GOPATH)/bin/go-winres"
    [[ -x "$WINRES" ]] || WINRES="go-winres"
    "$WINRES" simply --icon assets/icon.png --arch amd64 --out rsrc \
      --manifest gui --product-name "RequestFlow" \
      --file-description "HTTP 流程自动化工具" \
      --file-version "${ver#v}" --product-version "${ver#v}" >/dev/null 2>&1 \
      && echo "图标资源: 已生成 rsrc_windows_amd64.syso" \
      || echo "图标资源: go-winres 生成失败，使用无图标构建"
  else
    echo "图标资源: 未安装 go-winres，使用无图标构建（可 go install github.com/tc-hib/go-winres@latest）"
  fi
fi

# ---------- 交叉编译（注入版本，无 cgo）----------
# 保留控制台子系统，-run/-version 等 CLI 输出可见；图标已通过 .syso 注入。
CGO_ENABLED=0 GOOS="$GOOS_T" GOARCH="$GOARCH_T" go build -trimpath -ldflags "\
  -s -w \
  -X requestflow/version.Version=${ver} \
  -X requestflow/version.Commit=${commit} \
  -X requestflow/version.BuildTime=${build_time}" \
  -o "$STAGE/$BIN" .

# 清理生成的 syso，保持工作区干净
rm -f rsrc_windows_*.syso

# ---------- 附带文档与示例 ----------
cp README.md "$STAGE/"
cp scenarios/*.json "$STAGE/scenarios/"

# 包内使用说明：按平台生成对应的可执行文件名与运行方式
if [[ "$GOOS_T" == "windows" ]]; then
  RUNBIN="requestflow.exe"
  GUI_TIP="双击 $RUNBIN"
  EXTRA=""
else
  RUNBIN="./requestflow"
  GUI_TIP="在图形桌面环境中执行 $RUNBIN；若卡死/白屏，用 $RUNBIN -software 强制软件渲染"
  EXTRA="【Linux GUI 依赖】GTK3 + WebKitGTK
  Debian/Ubuntu: sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
  Fedora:        sudo dnf install gtk3 webkit2gtk4.1
  无图形环境请用下方的 -run 无界面模式。

"
fi
cat > "$STAGE/使用说明.txt" <<EOF
RequestFlow $ver
========================================

【运行 GUI】
  $GUI_TIP

${EXTRA}【无界面运行示例】
  $RUNBIN -run scenarios/demo.json

【查看版本】
  $RUNBIN -version

将 scenarios/ 下的 JSON 场景复制到任意位置，用 GUI 的「打开」加载即可。
场景为纯文本 JSON，可纳入版本管理、跨机器复用。

完整说明见 README.md
项目地址: https://github.com/yyt030/requestflow
EOF

# ---------- 打包（默认 tar.gz，可用 FORMAT=zip 切回）----------
if [[ "$FORMAT" == "zip" ]]; then
  ARCHIVE="${PKG}.zip"
  ( cd "$OUT" && zip -q -r "$ARCHIVE" "$PKG" )
else
  ARCHIVE="${PKG}.tar.gz"
  ( cd "$OUT" && tar -czf "$ARCHIVE" "$PKG" )
fi
rm -rf "$STAGE"

# ---------- 校验和 ----------
( cd "$OUT" && sha256sum "$ARCHIVE" > "${ARCHIVE}.sha256" )

echo "打包完成: $OUT/$ARCHIVE"
echo "校验文件: $OUT/${ARCHIVE}.sha256"
