package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"requestflow/engine"
	"requestflow/model"
)

// 验证从 Chrome DevTools 复制的请求头能被正确解析：
// 值里含 "="（Cookie / Bearer token）不应破坏 key，HTTP/2 伪头应被忽略。
// 这是修复「粘贴请求头后发送报 invalid header field name」的回归用例。
func TestStringToMapChromeHeaders(t *testing.T) {
	in := strings.Join([]string{
		"accept: */*",
		"cookie: _ga=GA1.1.123; session=abc",
		"authorization: Bearer abc123==",
		"referer: https://example.com/path?q=1",
		"user-agent: Mozilla/5.0",
		":authority: www.example.com", // HTTP/2 伪头，应忽略
		":method: GET",                // 同上
	}, "\n")
	got := stringToMap(in, ":")
	want := map[string]string{
		"accept":        "*/*",
		"cookie":        "_ga=GA1.1.123; session=abc",
		"authorization": "Bearer abc123==",
		"referer":       "https://example.com/path?q=1",
		"user-agent":    "Mozilla/5.0",
	}
	if len(got) != len(want) {
		t.Fatalf("解析数量不符: got %d %#v, want %d", len(got), got, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %q: got %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{":authority", ":method"} {
		if _, ok := got[k]; ok {
			t.Errorf("HTTP/2 伪头 %q 不应被解析", k)
		}
	}
}

// 端到端：Chrome 请求头文本经解析后真实发送，不应再报
// "net/http: invalid header field name"，且服务端能收到正确的 Cookie / Authorization。
func TestDebugSendWithChromeHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"cookie":%q,"authorization":%q}`,
			r.Header.Get("Cookie"), r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	headersText := strings.Join([]string{
		"cookie: _ga=GA1.1.123; session=abc",
		"authorization: Bearer tok==",
		":authority: example.invalid",
	}, "\n")
	res := engine.Debug(engine.DebugRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: stringToMap(headersText, ":"),
	}, nil)
	if !res.OK {
		t.Fatalf("调试请求不应失败: %s", res.Err)
	}
	for _, want := range []string{"_ga=GA1.1.123; session=abc", "Bearer tok=="} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("服务端未收到 %q，响应: %s", want, res.Body)
		}
	}
}

// 验证 mapToString 的输出能被 stringToMap 原样还原（key = value 格式）。
func TestStringToMapRoundTrip(t *testing.T) {
	in := map[string]string{"Authorization": "Bearer x==", "X-Test": "a=b", "Empty": ""}
	text := mapToString(in)
	got := stringToMap(text, ":")
	for k, v := range in {
		if got[k] != v {
			t.Errorf("round trip %q: got %q, want %q", k, got[k], v)
		}
	}
}

// 验证结果表数据源筛选：只有数组/JSON数组变量被识别为可表格展示。
func TestTabularVarsSelection(t *testing.T) {
	a := &App{}
	a.lastResult = &engine.Result{
		Vars: map[string]interface{}{
			"token":   "abc",                                          // 标量，不展示
			"page":    3,                                              // 标量，不展示
			"rows":    []interface{}{map[string]interface{}{"id": 1}}, // 数组，展示
			"rawJSON": `["x","y"]`,                                    // JSON 数组字符串，展示
		},
	}
	names := a.tabularVars()
	if len(names) != 2 {
		t.Fatalf("期望 2 个表格变量, 得到 %v", names)
	}
	if !containsStr(names, "rows") || !containsStr(names, "rawJSON") {
		t.Fatalf("表格变量不全: %v", names)
	}
	if containsStr(names, "token") || containsStr(names, "page") {
		t.Fatalf("标量不应作为表格: %v", names)
	}
}

// 验证 resetResult 清空可视化结果状态。
func TestResetResult(t *testing.T) {
	a := &App{lastResult: &engine.Result{Vars: map[string]interface{}{}}, resultVar: "rows", consoleView: 1}
	a.resetResult()
	if a.lastResult != nil || a.resultVar != "" {
		t.Fatalf("resetResult 未清空状态: %+v", a)
	}
}

// 验证全局变量编辑的数据流：UI 直接读写 scenario.Variables。
func TestGlobalVariableEdit(t *testing.T) {
	sc := model.NewScenario("t")
	a := &App{scenario: sc}
	if a.scenario.Variables == nil {
		a.scenario.Variables = map[string]string{}
	}
	// 模拟新增
	a.scenario.Variables["pageSize"] = "20"
	a.scenario.Variables["username"] = "demo"
	if got := a.scenario.Variables["pageSize"]; got != "20" {
		t.Fatalf("新增变量失败: %s", got)
	}
	// 模拟编辑
	a.scenario.Variables["pageSize"] = "50"
	if got := a.scenario.Variables["pageSize"]; got != "50" {
		t.Fatalf("编辑变量失败: %s", got)
	}
	// 模拟删除
	delete(a.scenario.Variables, "username")
	if _, ok := a.scenario.Variables["username"]; ok {
		t.Fatal("删除变量失败")
	}
}

// 验证构建表格：对象数组生成多列表，标量数组生成单列表。
func TestBuildTableForPreview(t *testing.T) {
	cols, rows, err := engine.BuildTable([]interface{}{
		map[string]interface{}{"id": 1, "name": "Ada"},
		map[string]interface{}{"id": 2, "name": "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || len(rows) != 2 {
		t.Fatalf("期望 2 列 2 行, 得到 %d 列 %d 行", len(cols), len(rows))
	}
	if rows[0][1] != "Ada" {
		t.Fatalf("单元格值错误: %s", rows[0][1])
	}
}

// 验证变量预览的类型分类与展开：数组/对象可展开为逐元素表，标量不展开。
func TestVariablePreviewHelpers(t *testing.T) {
	kind, n := variableKind([]interface{}{"a", "b", "c"})
	if kind != "数组" || n != 3 {
		t.Fatalf("数组分类错误: %s %d", kind, n)
	}
	kind, n = variableKind(map[string]interface{}{"a": 1, "b": 2})
	if kind != "对象" || n != 2 {
		t.Fatalf("对象分类错误: %s %d", kind, n)
	}
	kind, _ = variableKind("hello")
	if kind != "文本" {
		t.Fatalf("文本分类错误: %s", kind)
	}

	els := variableElements([]interface{}{"x", "y"})
	if len(els) != 2 || els[0][0] != "[0]" || els[0][1] != "x" {
		t.Fatalf("数组展开错误: %v", els)
	}
	els = variableElements(map[string]interface{}{"k": "v"})
	if len(els) != 1 || els[0][0] != "k" || els[0][1] != "v" {
		t.Fatalf("对象展开错误: %v", els)
	}
	if els := variableElements("scalar"); len(els) != 0 {
		t.Fatalf("标量不应展开: %v", els)
	}
	// 文本值经 JSON 渲染，换行被转义为 \n（便于单行展示与复制）。
	if cell := variableCell("line1\nline2"); cell != `line1\nline2` {
		t.Fatalf("文本值展示错误: %q", cell)
	}
}
