package engine

import (
	"strings"
	"testing"

	"requestflow/model"
)

func TestPreviewResponseFromDebug(t *testing.T) {
	dr := &DebugResult{
		StatusCode: 200,
		Body:       `{"data":{"token":"abc"}}`,
		IsJSON:     true,
		Headers:    []KV{{Key: "Content-Type", Value: "application/json"}},
		Cookies:    []KV{{Key: "sid", Value: "xyz"}},
	}
	resp := PreviewResponseFromDebug(dr)
	if resp == nil {
		t.Fatal("响应不应为空")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d", resp.StatusCode)
	}
	if resp.Text != dr.Body {
		t.Fatalf("文本不匹配: %q", resp.Text)
	}
	if v := resp.Headers["Content-Type"]; len(v) != 1 || v[0] != "application/json" {
		t.Fatalf("响应头解析失败: %v", v)
	}
	if len(resp.Cookies) != 1 || resp.Cookies[0].Name != "sid" {
		t.Fatalf("Cookie 解析失败: %v", resp.Cookies)
	}
	if jm, ok := resp.JSON.(map[string]interface{}); !ok || jm["data"] == nil {
		t.Fatalf("JSON 解析失败: %v", resp.JSON)
	}
	if PreviewResponseFromDebug(nil) != nil {
		t.Fatal("nil 输入应返回 nil")
	}
}

func TestPreviewExtractsJSONPath(t *testing.T) {
	dr := &DebugResult{StatusCode: 200, Body: `{"data":{"token":"abc123","list":[{"id":1},{"id":2}]}}`, IsJSON: true}
	resp := PreviewResponseFromDebug(dr)
	items := PreviewExtracts([]model.Extract{
		{Name: "token", Source: "body", Kind: "jsonpath", Path: "data.token"},
		{Name: "ids", Source: "body", Kind: "jsonpath", Path: "data.list"},
		{Name: "miss", Source: "body", Kind: "jsonpath", Path: "data.nope", Default: "fallback"},
	}, map[string]string{}, resp)
	if len(items) != 3 {
		t.Fatalf("条目数 = %d", len(items))
	}
	if items[0].Text != "abc123" || items[0].Count != 1 || items[0].Err != "" {
		t.Fatalf("token 试算失败: %+v", items[0])
	}
	if items[1].Count != 2 {
		t.Fatalf("ids 条数 = %d, 文本=%s", items[1].Count, items[1].Text)
	}
	if items[2].Text != "fallback" || !items[2].Default {
		t.Fatalf("默认值试算失败: %+v", items[2])
	}
}

func TestPreviewExtractsCSSRows(t *testing.T) {
	html := `<div class="item"><h3>书A</h3><a href="/a">链接A</a></div><div class="item"><h3>书B</h3><a href="/b">链接B</a></div>`
	resp := &model.Response{StatusCode: 200, Text: html, Body: []byte(html)}
	ex := model.Extract{
		Name: "books", Source: "body", Kind: "cssRows", Path: "div.item",
		CssFields: []model.CSSField{
			{Name: "title", Path: "h3", CssField: "text"},
			{Name: "link", Path: "a", CssField: "attr", CssAttr: "href"},
		},
	}
	items := PreviewExtracts([]model.Extract{ex}, map[string]string{}, resp)
	if len(items) != 1 || items[0].Err != "" {
		t.Fatalf("cssRows 试算失败: %+v", items)
	}
	if items[0].Count != 2 || !strings.Contains(items[0].Text, "书A") || !strings.Contains(items[0].Text, "/a") {
		t.Fatalf("cssRows 结果不符: %+v", items[0])
	}
}

func TestPreviewExtractsNoResponse(t *testing.T) {
	items := PreviewExtracts([]model.Extract{{Name: "x", Source: "body", Kind: "whole"}}, nil, nil)
	if len(items) != 1 || !strings.Contains(items[0].Err, "暂无调试响应") {
		t.Fatalf("无响应提示失败: %+v", items)
	}
}

func TestPreviewProcessFilter(t *testing.T) {
	step := model.Step{
		Type: model.StepProcess,
		Op:   "filter",
		Args: map[string]string{"from": "books", "field": "rating", "op": "gte", "value": "8", "to": "top"},
	}
	vars := map[string]string{
		"books": `[{"title":"A","rating":9},{"title":"B","rating":7},{"title":"C","rating":8.5}]`,
	}
	item := PreviewProcess(step, vars)
	if item.Err != "" {
		t.Fatalf("filter 试算失败: %s", item.Err)
	}
	if item.OutName != "top" || item.AfterT == "" {
		t.Fatalf("输出变量不符: %+v", item)
	}
	if !strings.Contains(item.AfterT, "9") || strings.Contains(item.AfterT, "B") {
		t.Fatalf("过滤结果不符: %s", item.AfterT)
	}
	if !strings.Contains(item.Delta, "→") {
		t.Fatalf("变化摘要缺失: %s", item.Delta)
	}
}

func TestPreviewProcessSetAndDefaultOut(t *testing.T) {
	step := model.Step{Type: model.StepProcess, Op: "set", Args: map[string]string{"name": "hi", "value": "world"}}
	item := PreviewProcess(step, map[string]string{})
	if item.Err != "" || item.AfterT != "world" || item.OutName != "hi" {
		t.Fatalf("set 试算失败: %+v", item)
	}
	if defaultOutName("limit") != "limited" || defaultOutName("bogus") != "result" {
		t.Fatal("默认输出名映射失败")
	}
}

func TestPreviewCompactText(t *testing.T) {
	if previewCompactText(nil) != "∅（无结果）" {
		t.Fatal("nil 展示失败")
	}
	s := previewCompactText([]interface{}{1, 2, 3, 4})
	if !strings.Contains(s, "4 条") {
		t.Fatalf("数组展示失败: %s", s)
	}
}

// TestPreviewJSONListToTable 验证完整链路：从响应 JSON 用 jsonpath 取出 list，
// 再把提取到的数组变量交给 json_to_table 解析成表格。这是"解析返回报文里的 list"案例的核心。
func TestPreviewJSONListToTable(t *testing.T) {
	dr := &DebugResult{
		StatusCode: 200,
		Body:       `{"code":0,"data":{"books":[{"title":"活着","rating":9.4},{"title":"平凡的世界","rating":8.8}]}}`,
		IsJSON:     true,
	}
	resp := PreviewResponseFromDebug(dr)

	// 1) 提取 data.books 这个 list。
	extracts := []model.Extract{{Name: "books", Source: "body", Kind: "jsonpath", Path: "data.books"}}
	items := PreviewExtracts(extracts, map[string]string{}, resp)
	if len(items) != 1 || items[0].Err != "" || items[0].Count != 2 {
		t.Fatalf("提取 data.books 失败: %+v", items)
	}

	// 2) 把提取结果回填为变量（与 UI previewVars 一致：PreviewValueString）。
	vars := map[string]string{"books": PreviewValueString(items[0].Value)}

	// 3) json_to_table 解析成表格。
	r := PreviewProcess(model.Step{Op: "json_to_table", Args: map[string]string{"from": "books", "to": "table"}}, vars)
	if r.Err != "" {
		t.Fatalf("json_to_table 解析失败: %s", r.Err)
	}
	if !strings.Contains(r.AfterT, "[2 条]") {
		t.Fatalf("表格应有 2 条，实际: %s", r.AfterT)
	}
	if r.OutName != "table" {
		t.Fatalf("输出变量名 = %q", r.OutName)
	}
}
