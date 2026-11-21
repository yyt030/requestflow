//go:build windows

package cmd

import "syscall"

// hideConsole 在 Windows GUI 模式下隐藏双击 exe 时自动弹出的控制台窗口。
// 只在进入图形界面前调用；-version / -run 等 CLI 模式不调用，保证终端输出可见。
// 程序本身仍是 console 子系统，因此不会出现 windowsgui 子系统下 CLI 输出丢失的问题。
func hideConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")

	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	const swHide = 0 // SW_HIDE
	showWindow.Call(hwnd, uintptr(swHide))
}
