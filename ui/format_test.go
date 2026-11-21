package ui

import (
	"strings"
	"testing"
)

func TestFormatContentJSON(t *testing.T) {
	in := `{"name":"rf","n":1,"arr":[1,2]}`
	out, kind, changed := FormatContent(in)
	if kind != "JSON" || !changed {
		t.Fatalf("JSON 识别错误: kind=%s changed=%v out=%q", kind, changed, out)
	}
	if !strings.Contains(out, "\n  \"name\"") || !strings.Contains(out, "\n  \"arr\"") {
		t.Fatalf("JSON 未缩进: %q", out)
	}
	// 已格式化的 JSON 不应再次变化
	out2, _, changed2 := FormatContent(out)
	if changed2 || out2 != out {
		t.Fatalf("重复格式化应幂等: changed=%v", changed2)
	}
}

func TestFormatContentKeyValue(t *testing.T) {
	in := "authorization: Bearer x\nx-api-key: abc\naccept: application/json"
	out, kind, changed := FormatContent(in)
	if kind != "键值" || !changed {
		t.Fatalf("键值识别错误: kind=%s changed=%v", kind, changed)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("行数错误: %d", len(lines))
	}
	// 分隔符前应对齐：最长 key 是 authorization(13)，其余应补足空格
	if strings.Index(lines[1], ":") < strings.Index(lines[2], ":") {
		t.Fatalf("未对齐: %q", out)
	}
}

func TestFormatContentXML(t *testing.T) {
	in := `<root><item id="1"><name>x</name></item></root>`
	out, kind, changed := FormatContent(in)
	if kind != "XML" || !changed {
		t.Fatalf("XML 识别错误: kind=%s changed=%v out=%q", kind, changed, out)
	}
	if !strings.Contains(out, "\n  <item") || !strings.Contains(out, "\n    <name>") {
		t.Fatalf("XML 未缩进: %q", out)
	}
}

func TestFormatContentUnknown(t *testing.T) {
	for _, s := range []string{
		"hello world",
		"vars.token = last.json.data.token;",
		"",
		"page: 1", // 单行键值不触发对齐
	} {
		out, kind, changed := FormatContent(s)
		if changed || kind != "" || out != s {
			t.Fatalf("未知内容不应变化: in=%q out=%q kind=%s changed=%v", s, out, kind, changed)
		}
	}
}
