// Package version 承载构建期注入的版本信息，供 CLI 与 GUI 统一展示。
//
// 三个变量由构建脚本通过 -ldflags "-X requestflow/version.Version=..." 注入：
//   - Version:   来自 git describe --tags --always --dirty，例如 v1.0.0、v1.0.0-3-gabc1234、v1.0.0-dirty
//   - Commit:    来自 git rev-parse HEAD 的提交哈希
//   - BuildTime: 构建时间（RFC3339）
//
// 未注入时保留默认值，便于 `go run` / IDE 直接开发调试。
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// 以下变量在构建时由 -ldflags -X 覆盖，默认值用于本地 `go run` 开发。
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Short 返回简短版本号（tag 或 dev），适合放在标题栏、状态栏等空间有限处。
func Short() string {
	return Version
}

// String 返回单行完整版本描述。
func String() string {
	return fmt.Sprintf("RequestFlow %s (commit %s, built %s, %s/%s)",
		Version, shortCommit(Commit), BuildTime, runtime.GOOS, runtime.GOARCH)
}

// Detail 返回多行详细信息，供"关于"面板或 -version 展示。
func Detail() string {
	var b strings.Builder
	fmt.Fprintf(&b, "版本:       %s\n", Version)
	fmt.Fprintf(&b, "提交哈希:   %s\n", Commit)
	fmt.Fprintf(&b, "构建时间:   %s\n", BuildTime)
	fmt.Fprintf(&b, "Go 版本:    %s\n", runtime.Version())
	fmt.Fprintf(&b, "目标平台:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return strings.TrimRight(b.String(), "\n")
}

// shortCommit 把长哈希截为 7 位，未注入时原样返回。
func shortCommit(c string) string {
	if len(c) > 7 {
		return c[:7]
	}
	return c
}
