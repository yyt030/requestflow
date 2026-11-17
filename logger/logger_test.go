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
