//go:build !windows

package cmd

// hideConsole 在非 Windows 平台无需操作（控制台由终端决定，不存在双击弹黑框问题）。
func hideConsole() {}
