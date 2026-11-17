package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogWritesToFile(t *testing.T) {
	dir := t.TempDir()
	Init(dir)
	defer Close()

	if got := Path(); got == "" {
		t.Fatal("Path() 不应为空")
	}
	Printf("hello %s", "world")
	Error("bad %d", 42)
	Flush()

	data, err := os.ReadFile(filepath.Join(dir, "requestflow.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	s := string(data)
	for _, want := range []string{"hello world", "bad 42", "日志开始", "INFO", "ERROR"} {
		if !strings.Contains(s, want) {
			t.Fatalf("日志缺少 %q:\n%s", want, s)
		}
	}
}

func TestDefaultDirPrefersExeDir(t *testing.T) {
	dirs := defaultDirs()
	if len(dirs) == 0 {
		t.Fatal("defaultDirs() 不应为空")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable 失败: %v", err)
	}
	want := filepath.Dir(exe)
	if dirs[0] != want {
		t.Fatalf("首选日志目录应为 exe 同目录 %q, got %q", want, dirs[0])
	}
	// 用户配置目录作为降级候选应排在后面（若可取到）。
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		fallback := filepath.Join(base, "requestflow", "logs")
		found := false
		for _, d := range dirs[1:] {
			if d == fallback {
				found = true
			}
		}
		if !found {
			t.Fatalf("缺少降级目录 %q, dirs=%v", fallback, dirs)
		}
	}
}

func TestAppendAcrossSessions(t *testing.T) {
	dir := t.TempDir()
	Init(dir)
	Printf("first")
	Close()
	Init(dir)
	defer Close()
	Printf("second")
	Flush()

	data, _ := os.ReadFile(filepath.Join(dir, "requestflow.log"))
	s := string(data)
	if !strings.Contains(s, "first") || !strings.Contains(s, "second") {
		t.Fatalf("日志未跨会话追加:\n%s", s)
	}
}
