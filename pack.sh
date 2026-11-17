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
#   内含: requestflow(.exe) · README.md · scenarios/ 示例场景

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

# ---------- 交叉编译（注入版本，无 cgo）----------
CGO_ENABLED=0 GOOS="$GOOS_T" GOARCH="$GOARCH_T" go build -trimpath -ldflags "\
  -s -w \
  -X requestflow/version.Version=${ver} \
  -X requestflow/version.Commit=${commit} \
  -X requestflow/version.BuildTime=${build_time}" \
  -o "$STAGE/$BIN" .

# ---------- 附带文档与示例 ----------
cp README.md "$STAGE/"
cp scenarios/*.json "$STAGE/scenarios/"

# Windows 下直接双击即可用 GUI；命令行用法写进包内说明
cat > "$STAGE/使用说明.txt" <<EOF
RequestFlow $ver
========================================

【运行 GUI】
  双击 requestflow.exe

【无界面运行示例】
  requestflow.exe -run scenarios/demo.json

【查看版本】
  requestflow.exe -version

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
