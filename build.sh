#!/usr/bin/env bash
# RequestFlow 构建脚本：版本号与提交哈希自动从 git 生成并注入二进制。
#
# 用法:
#   ./build.sh                      # 构建，输出到 ./requestflow
#   ./build.sh -o /path/to/bin      # 指定输出路径
#   VERSION=v2.0.0 ./build.sh       # 手动覆盖版本号（CI 场景）
#
# 注入规则（-ldflags -X）:
#   版本    = git describe --tags --always --dirty   （如 v1.0.0 / v1.0.0-3-gabc1234 / abc1234-dirty）
#   提交哈希 = git rev-parse HEAD
#   构建时间 = UTC RFC3339

set -euo pipefail
cd "$(dirname "$0")"

# ---------- 参数解析 ----------
OUT="requestflow"
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output) OUT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *) echo "未知参数: $1" >&2; exit 1 ;;
  esac
done

# ---------- 自动推导版本信息 ----------
# 版本号：优先用环境变量覆盖，否则从 git tag 推导
if [[ -n "${VERSION:-}" ]]; then
  ver="$VERSION"
elif git rev-parse --git-dir >/dev/null 2>&1; then
  ver="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
else
  ver="dev"
fi

# 提交哈希
if git rev-parse --git-dir >/dev/null 2>&1; then
  commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
else
  commit="unknown"
fi

# 构建时间（UTC）
build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "版本:     $ver"
echo "提交:     $commit"
echo "构建时间: $build_time"
echo "输出:     $OUT"

# ---------- 注入并构建 ----------
PKG="requestflow/version"
go build -trimpath -ldflags "\
  -s -w \
  -X ${PKG}.Version=${ver} \
  -X ${PKG}.Commit=${commit} \
  -X ${PKG}.BuildTime=${build_time}" \
  -o "$OUT" .

echo "构建完成: $OUT"
