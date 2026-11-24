package ui

import (
	"strings"
	"testing"
)

// TestMapToColonStringFormat 验证 Header/Query 显示成 "key: value"（Chrome/Postman 格式），
// 而非 "key = value"，让复制到外部工具时不再因格式报错。
func TestMapToColonStringFormat(t *testing.T) {
	in := map[string]string{
		"Accept":        "*/*",
		"Authorization": "Bearer tok==",
		"Cookie":        "_ga=GA1.1.123; session=abc",
		"Referer":       "https://example.com:8080/path?q=a=b",
	}
	text := mapToColonString(in)
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, " = ") {
			t.Fatalf("不应出现等号格式: %q", line)
		}
		if !strings.Contains(line, ": ") {
			t.Fatalf("应为冒号格式: %q", line)
		}
	}
	// 值里即使含 "=" 或 ":"，也只按 key 后第一个冒号切分，往返无损。
	got := stringToMap(text, ":")
	for k, v := range in {
		if got[k] != v {
			t.Errorf("round trip %q: got %q, want %q (text %q)", k, got[k], v, text)
		}
	}
}

// TestMapToColonStringSorted 验证按 key 排序，输出稳定便于肉眼核对。
func TestMapToColonStringSorted(t *testing.T) {
	text := mapToColonString(map[string]string{"Zeta": "1", "Alpha": "2"})
	if text != "Alpha: 2\nZeta: 1" {
		t.Fatalf("排序输出不符: %q", text)
	}
}

// TestProcessArgsStillEqualFormat 确保 process 参数仍用等号格式，
// 避免本次 Header 改动误伤共用函数导致 process 参数显示变化。
func TestProcessArgsStillEqualFormat(t *testing.T) {
	text := mapToString(map[string]string{"from": "books", "path": "data.list"})
	if !strings.Contains(text, "from = books") || !strings.Contains(text, "path = data.list") {
		t.Fatalf("process 参数应保持等号格式: %q", text)
	}
}
