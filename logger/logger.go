// Package logger 提供 RequestFlow 的默认运行日志：无论 GUI 还是无界面模式，
// 日常运行都会把关键事件追加写入可执行文件同目录的日志文件，便于绿色部署与事后排查。
// 若 exe 同目录不可写（如装在 Program Files），则自动退回用户配置目录。
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	file    *os.File
	logPath string
	enabled = true
)

// Init 初始化日志。若 dir 非空则写入该目录；否则默认写可执行文件同目录
// （绿色部署，日志与 exe 放一起）。若 exe 同目录不可写（如装在 Program Files），
// 自动退回用户配置目录 %APPDATA%/requestflow/logs（Windows）或 ~/.config/requestflow/logs。
// 初始化失败不会致命，仅关闭文件日志，GUI/CLI 仍可正常运行。
func Init(dir string) {
	mu.Lock()
	defer mu.Unlock()

	var candidates []string
	if dir != "" {
		candidates = []string{dir}
	} else {
		candidates = defaultDirs()
	}
	for _, d := range candidates {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			continue
		}
		p := filepath.Join(d, "requestflow.log")
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			continue
		}
		file = f
		logPath = p
		// 记录本次启动，日志跨会话可区分。
		fmt.Fprintf(file, "[%s] ==== 日志开始 (PID %d) ====\n", now(), os.Getpid())
		return
	}
	enabled = false
}

// defaultDirs 返回候选日志目录，按优先级排列：
//  1. 可执行文件同目录（默认，绿色部署）
//  2. 用户配置目录（exe 同目录不可写时的降级）
func defaultDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil && exe != "" {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		dirs = append(dirs, filepath.Join(base, "requestflow", "logs"))
	}
	if len(dirs) == 0 {
		dirs = []string{"logs"}
	}
	return dirs
}

// Path 返回当前日志文件路径，未启用时返回空串。
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return logPath
}

// Printf 追加一条 INFO 日志。
func Printf(format string, args ...interface{}) {
	write("INFO", format, args...)
}

// Error 追加一条 ERROR 日志。
func Error(format string, args ...interface{}) {
	write("ERROR", format, args...)
}

func write(level, format string, args ...interface{}) {
	if !enabled {
		return
	}
	line := fmt.Sprintf("[%s] %-5s %s\n", now(), level, fmt.Sprintf(format, args...))
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return
	}
	_, _ = io.WriteString(file, line)
}

// Flush 将缓冲落盘（当前为直接写文件，保留接口用于确保持久化）。
func Flush() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Sync()
	}
}

// Close 关闭日志文件。
func Close() {
	Flush()
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
		file = nil
	}
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
