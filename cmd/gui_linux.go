//go:build linux

package cmd

import (
	"errors"
	"os"
)

// prepareGUI 在启动 GUI 前准备图形环境。
//
// RequestFlow 的 UI 基于 mygo，其在 Linux 上内嵌 WebKitGTK。WebKitGTK 的
// DMA-BUF/GPU 渲染在部分 NVIDIA 驱动、虚拟机、远程桌面和老显卡上会把整个
// X/Wayland 合成器拖死，表现为"运行后完全没反应、连鼠标都卡住"。为此在
// Linux 下默认强制走 CPU 软件渲染（RequestFlow 只是表单 UI，不需要 GPU），
// 并设置 GDK_BACKEND=x11 规避个别 Wayland 合成器问题。用户已显式设置的
// 环境变量优先，不覆盖。
func prepareGUI(software bool) {
	// 用户已显式指定的值优先。
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
	// -software 或用户未指定 LIBGL_ALWAYS_SOFTWARE 时，强制软件 OpenGL。
	if software || os.Getenv("LIBGL_ALWAYS_SOFTWARE") == "" {
		_ = os.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")
	}
	// 优先 X11：Wayland 下某些合成器与 WebKit 子进程冲突会冻结桌面。
	// 仅当用户未显式指定 GDK_BACKEND 时设置。
	if os.Getenv("GDK_BACKEND") == "" {
		_ = os.Setenv("GDK_BACKEND", "x11")
	}
}

// guiAvailable 在启动 GUI 前检测是否存在图形显示环境。
// mygo 依赖 GTK，缺 DISPLAY/WAYLAND_DISPLAY 时其内部 gtk_init_check 会失败；
// 这里提前给出明确错误，避免双击运行时窗口一闪而过、被误认为"卡死"。
func guiAvailable() error {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return errors.New("未检测到图形环境（DISPLAY 与 WAYLAND_DISPLAY 均为空）。" +
			"请在桌面会话中运行，或通过 SSH X11 转发；无图形环境请用 -run 无界面模式。")
	}
	return nil
}
