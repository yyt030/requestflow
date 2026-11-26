package ui

import (
	"strings"
	"testing"

	"requestflow/engine"
)

// TestResponseBodyTextRawVsFormatted 验证「原始/格式化」切换的核心语义：
// 原始返回服务端原文；格式化复用通用 FormatContent，JSON 与 XML 都能美化。
func TestResponseBodyTextRawVsFormatted(t *testing.T) {
	compactJSON := `{"name":"rf","n":1}`
	xmlBody := `<root><item id="1"><name>x</name></item></root>`

	cases := []struct {
		name     string
		body     string
		mode     int
		wantRaw  bool
		contains string
	}{
		{"原始-JSON 保原文", compactJSON, 0, true, compactJSON},
		{"格式化-JSON 缩进", compactJSON, 1, false, "\n  \"name\""},
		{"原始-XML 保原文", xmlBody, 0, true, xmlBody},
		{"格式化-XML 缩进", xmlBody, 1, false, "\n  <item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{respBodyMode: tc.mode}
			res := &engine.DebugResult{OK: true, Body: tc.body, Formatted: tc.body}
			got, errMsg := a.responseBodyText(res)
			if errMsg != "" {
				t.Fatalf("不应出错: %s", errMsg)
			}
			if !strings.Contains(got, tc.contains) {
				t.Fatalf("结果缺少 %q，实际: %q", tc.contains, got)
			}
			if tc.wantRaw && got != tc.body {
				t.Fatalf("原始模式应逐字返回原文: got=%q want=%q", got, tc.body)
			}
		})
	}
}

// TestResponseBodyTextHex 验证 Hex 模式：输出经典 hexdump（偏移 + 十六进制 + ASCII），
// 不可打印字节显示为 '.'，且忽略 JSONPath（字节级视图）。
func TestResponseBodyTextHex(t *testing.T) {
	body := "AB\x00\xffZ" // 可打印 A B，不可打印 NUL / 0xff，可打印 Z
	a := &App{respBodyMode: 2, respJSONPath: "ignored.path"}
	got, errMsg := a.responseBodyText(&engine.DebugResult{Body: body})
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	// 偏移 00000000；A=41 B=42 NUL=00 ff=FF Z=5A；ASCII 区 AB..Z
	for _, want := range []string{"00000000", "41 42 00 ff 5a", "|AB..Z|"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Hex 输出缺少 %q，实际:\n%s", want, got)
		}
	}
}

// TestHexDumpEmptyAndLimit 验证空体提示与超限截断说明。
func TestHexDumpEmptyAndLimit(t *testing.T) {
	if got := hexDump(nil); !strings.Contains(got, "空响应体") {
		t.Fatalf("空体应给出提示，实际: %q", got)
	}
	big := make([]byte, hexDumpLimit+16)
	got := hexDump(big)
	if !strings.Contains(got, "上限 64KB") || !strings.Contains(got, "共 65552 字节") {
		t.Fatalf("超限应注明截断，实际结尾: %q", got[len(got)-80:])
	}
}

// TestResponseBodyTextNotAffectedByJSONPath 验证核心需求：JSONPath 分屏展示时，
// 完整响应体（responseBodyText）绝不被 JSONPath 改动，仍可对照原文调试。
func TestResponseBodyTextNotAffectedByJSONPath(t *testing.T) {
	body := `{"code":0,"data":{"token":"abc","name":"rf"}}`
	for _, mode := range []int{0, 1} {
		a := &App{respBodyMode: mode, respJSONPath: "data.token"}
		got, errMsg := a.responseBodyText(&engine.DebugResult{Body: body})
		if errMsg != "" {
			t.Fatal(errMsg)
		}
		if !strings.Contains(got, "code") || !strings.Contains(got, "data") {
			t.Fatalf("mode=%d 完整响应体不应被 JSONPath 过滤: %q", mode, got)
		}
	}
}

// TestResponseJSONPathText 验证分屏右侧的 JSONPath 提取结果：
// 原始模式返回节点原文，格式化模式返回美化节点，且都支持关键字过滤。
func TestResponseJSONPathText(t *testing.T) {
	body := `{"code":0,"data":{"token":"abc","name":"rf"}}`

	a := &App{respBodyMode: 0, respJSONPath: "data"}
	got, errMsg := a.responseJSONPathText(&engine.DebugResult{Body: body})
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if got != `{"token":"abc","name":"rf"}` {
		t.Fatalf("原始模式 JSONPath 应返回节点原文，实际: %q", got)
	}

	a.respBodyMode = 1
	got, errMsg = a.responseJSONPathText(&engine.DebugResult{Body: body})
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if !strings.Contains(got, "\n  \"token\"") {
		t.Fatalf("格式化模式 JSONPath 应美化节点，实际: %q", got)
	}
}

// TestResponseJSONPathTextErrors 验证非法 JSON / 未匹配路径返回错误提示而非正文。
func TestResponseJSONPathTextErrors(t *testing.T) {
	a := &App{respBodyMode: 1, respJSONPath: "data.token"}
	if _, errMsg := a.responseJSONPathText(&engine.DebugResult{Body: "not json"}); errMsg == "" {
		t.Fatal("非 JSON 响应 + JSONPath 应返回错误提示")
	}
	if _, errMsg := a.responseJSONPathText(&engine.DebugResult{Body: `{"data":{}}`}); errMsg == "" {
		t.Fatal("JSONPath 未匹配应返回错误提示")
	}
}

// TestResponseBodyTextFilter 验证关键字过滤在两种模式下均生效。
func TestResponseBodyTextFilter(t *testing.T) {
	// 多行输入：按行过滤后可剔除不含关键字的行（单行整体保留是 filterLines 的固有语义）。
	body := "{\n  \"token\": \"abc\",\n  \"name\": \"rf\"\n}"
	for _, mode := range []int{0, 1} {
		a := &App{respBodyMode: mode, respFilter: "token"}
		got, errMsg := a.responseBodyText(&engine.DebugResult{Body: body})
		if errMsg != "" {
			t.Fatal(errMsg)
		}
		if !strings.Contains(got, "abc") || strings.Contains(got, "name") {
			t.Fatalf("mode=%d 过滤未生效: %q", mode, got)
		}
	}
}
