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

func TestFormatContentHTML(t *testing.T) {
	// 真实完整 HTML：DOCTYPE、无引号属性、未闭合 <br>、实体 &nbsp;、script 内含 < 与 &。
	in := `<!DOCTYPE html><html><head><meta charset=utf-8><title>T</title></head><body><div class=box><h1>Hi</h1><p>Hello &nbsp; world<br>line2</p><!-- c --><script>if (a < b) x = "a&b";</script></div></body></html>`
	out, kind, changed := FormatContent(in)
	if kind != "HTML" || !changed {
		t.Fatalf("HTML 识别错误: kind=%s changed=%v out=%q", kind, changed, out)
	}
	for _, want := range []string{
		"<!DOCTYPE html>",
		"\n<html>",
		"\n  <head>",
		"&nbsp;",
		"class=box", // 原始属性写法应保留，不被改写成 class="box"
		"<br>",
		"a < b", // script 内容不应被转义/破坏
		"<!-- c -->",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML 格式化丢失/改写了 %q，实际:\n%s", want, out)
		}
	}
	// 幂等：再次格式化不应变化。
	out2, _, changed2 := FormatContent(out)
	if changed2 || out2 != out {
		t.Fatalf("HTML 重复格式化应幂等: changed=%v\nout2=%q", changed2, out2)
	}
}

func TestFormatContentHTMLImplicitClose(t *testing.T) {
	// <li> 省略结束标签时应并列，不能像 XML 那样层层嵌套。
	in := `<ul><li>a<li>b<li>c</ul>`
	out, kind, _ := FormatContent(in)
	if kind != "HTML" {
		t.Fatalf("应识别为 HTML，实际 %s", kind)
	}
	if strings.Contains(out, "<li>\n    a\n    <li>") {
		t.Fatalf("<li> 不应错误嵌套: %s", out)
	}
	closes := strings.Count(out, "</li>")
	if closes != 3 {
		t.Fatalf("应有 3 个并列 </li>，实际 %d:\n%s", closes, out)
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
