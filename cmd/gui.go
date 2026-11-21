package cmd

import (
	"fmt"
	"os"

	"requestflow/logger"
	"requestflow/ui"
)

// launchGUI 进入图形界面：Linux 下先做图形环境预检 + 软件渲染兜底，
// 避免无 display 时静默卡住，或 WebKitGPU 在特定驱动上拖死桌面。
func launchGUI(software bool) {
	hideConsole()
	prepareGUI(software)
	if err := guiAvailable(); err != nil {
		fmt.Fprintln(os.Stderr, "无法启动图形界面:", err)
		logger.Error("图形环境检测失败: %v", err)
		os.Exit(1)
	}
	ui.Run()
}
