//go:build !linux

package cmd

// prepareGUI 在非 Linux 平台无需图形环境预处理（Windows/macOS 由系统处理）。
func prepareGUI(software bool) {}

// guiAvailable 在非 Linux 平台由 mygo 自身处理，不做额外检测。
func guiAvailable() error { return nil }
