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

func TestDefaultDirIsExeDir(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable 失败: %v", err)
	}
	want := filepath.Dir(exe)
	if got := defaultDir(); got != want {
		t.Fatalf("默认日志目录应为 exe 同目录 %q, got %q", want, got)
	}
	// 显式：默认目录不得是用户配置目录。
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		if got := defaultDir(); strings.HasPrefix(got, base) {
			t.Fatalf("默认目录不应落在用户配置目录下: %q", got)
		}
	}
}

func TestInitFailureIsExplicit(t *testing.T) {
	// 指向一个无法创建的路径（父级是普通文件），应显式报错且不降级到配置目录。
	blocked := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(blocked, "sub") // 父级是文件，MkdirAll 必失败

	Init(badDir)
	defer Close()

	if err := Err(); err == nil {
		t.Fatal("写入失败时 Err() 应返回明确错误")
	}
	if got := Path(); got != "" {
		t.Fatalf("写入失败时 Path() 应为空, got %q", got)
	}
	Printf("should not appear")
	if enabled {
		t.Fatal("写入失败后日志应保持禁用，不降级到其它目录")
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
