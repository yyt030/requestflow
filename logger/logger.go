// Package logger 提供 RequestFlow 的默认运行日志：无论 GUI 还是无界面模式，
// 日常运行都会把关键事件追加写入可执行文件同目录的 requestflow.log，
// 便于绿色部署与事后排查。日志位置是显式的——只写 exe 同目录（或 -logdir 指定），
// 不降级到用户配置目录；若写入失败会明确报错，而非静默更换位置。
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
	lastErr error
)

// Init 初始化日志。dir 非空时写入该目录，否则写可执行文件同目录（绿色部署）。
// 只使用显式指定的位置，不会降级到用户配置目录；写入失败会记录错误并通过
// stderr 明确提示，同时 Err() 可供界面展示。初始化失败不致命，GUI/CLI 仍可运行。
func Init(dir string) {
	mu.Lock()
	defer mu.Unlock()

	target := dir
	if target == "" {
		target = defaultDir()
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		failLocked(target, err)
		return
	}
	p := filepath.Join(target, "requestflow.log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		failLocked(target, err)
		return
	}
	file = f
	logPath = p
	lastErr = nil
	enabled = true
	// 记录本次启动，日志跨会话可区分。
	fmt.Fprintf(file, "[%s] ==== 日志开始 (PID %d) 路径 %s ====\n", now(), os.Getpid(), p)
}

// failLocked 记录初始化失败并显式向 stderr 报错（调用方需持有 mu）。
func failLocked(target string, err error) {
	enabled = false
	logPath = ""
	lastErr = fmt.Errorf("无法写入日志目录 %q: %w", target, err)
	fmt.Fprintf(os.Stderr, "[logger] %v\n", lastErr)
}

// defaultDir 返回默认日志目录：可执行文件同目录。
// 这是唯一默认位置，不使用用户配置目录；拿不到 exe 路径时退回当前工作目录。
func defaultDir() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		if d := filepath.Dir(exe); d != "" {
			return d
		}
	}
	return "."
}

// Err 返回日志初始化/写入的最后一个错误，未出错时为 nil。
// 供界面在「关于」等处显式告知用户日志未生效。
func Err() error {
	mu.Lock()
	defer mu.Unlock()
	return lastErr
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
	if _, err := io.WriteString(file, line); err != nil {
		lastErr = err
		fmt.Fprintf(os.Stderr, "[logger] 写日志失败: %v\n", err)
	}
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
