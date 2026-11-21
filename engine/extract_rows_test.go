package engine

import (
	"encoding/base64"
	"strings"
	"testing"

	"requestflow/model"
)

func newTestCtx(vars map[string]string) *Context {
	return NewContext(vars)
}

func htmlResp(body string) *model.Response {
	return &model.Response{StatusCode: 200, Text: body, Body: []byte(body)}
}

// 验证 cssRows：同一行的多个子字段配对正确，不依赖多选择器 zip。
func TestExtractCSSRows(t *testing.T) {
	html := `<div class="item"><h3>Go 官网</h3><a href="https://go.dev/">link</a></div>
	<div class="item"><h3>GitHub</h3><a href="https://github.com/golang/go">link</a></div>
	<div class="item"><h3>无链接</h3><span>text</span></div>`
	ex := model.Extract{
		Name: "rows", Kind: "cssrows", Path: "div.item",
		CssFields: []model.CSSField{
			{Name: "title", Path: "h3", CssField: "text"},
			{Name: "url", Path: "a", CssField: "attr", CssAttr: "href"},
		},
	}
	v, err := extractOne(newTestCtx(nil), htmlResp(html), ex)
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := v.([]interface{})
	if !ok || len(rows) != 3 {
		t.Fatalf("期望 3 行, got %v (%T)", v, v)
	}
	r0 := rows[0].(map[string]interface{})
	if r0["title"] != "Go 官网" || r0["url"] != "https://go.dev/" {
		t.Fatalf("第1行配对错误: %v", r0)
	}
	r1 := rows[1].(map[string]interface{})
	if r1["title"] != "GitHub" || r1["url"] != "https://github.com/golang/go" {
		t.Fatalf("第2行配对错误: %v", r1)
	}
	r2 := rows[2].(map[string]interface{})
	if r2["url"] != "" {
		t.Fatalf("第3行缺链接应为空串: %v", r2)
	}
}

// 验证 expectMin：提取条数不足即报错。
func TestExpectMin(t *testing.T) {
	html := `<div class="item">A</div><div class="item">B</div>`
	ctx := newTestCtx(nil)
	ex := model.Extract{Name: "items", Kind: "css", Path: "div.item", CssField: "text", ExpectMin: 3}
	if err := applyExtracts(ctx, htmlResp(html), []model.Extract{ex}); err == nil {
		t.Fatal("expectMin=3 但只有 2 条，应报错")
	}
	ex.ExpectMin = 2
	if err := applyExtracts(ctx, htmlResp(html), []model.Extract{ex}); err != nil {
		t.Fatalf("expectMin=2 应通过: %v", err)
	}
}

// 验证 resolveURL：相对地址绝对化 + 百度/Bing 跳转链接解码。
func TestResolveURL(t *testing.T) {
	ctx := newTestCtx(map[string]string{"baseUrl": "https://example.com/dir/"})
	// 相对地址绝对化
	if got := resolveURLOne("page.html", "https://example.com/dir/"); got != "https://example.com/dir/page.html" {
		t.Fatalf("相对地址绝对化错误: %s", got)
	}
	// 百度跳转链接解码
	raw := "aHR0cHM6Ly9naXRodWIuY29tL2dvbGFuZy9nbw" // base64url(https://github.com/golang/go) 无padding
	if got := resolveURLOne("http://www.baidu.com/link?url="+raw, ""); got != "https://github.com/golang/go" {
		t.Fatalf("百度跳转解码错误: %s", got)
	}
	// Bing 跳转链接解码
	bingRaw := base64.RawURLEncoding.EncodeToString([]byte("\x00https://go.dev/"))
	if got := resolveURLOne("https://www.bing.com/ck/a?u=a1"+bingRaw, ""); got != "https://go.dev/" {
		t.Fatalf("Bing 跳转解码错误: %s", got)
	}
	// HTML 实体兜底
	if got := htmlUnescapeBasic("https://x.com/?a=1&amp;b=2"); got != "https://x.com/?a=1&b=2" {
		t.Fatalf("实体解码错误: %s", got)
	}
	_ = ctx
}

// 验证 process zip：多数组按索引合成对象数组。
func TestProcessZip(t *testing.T) {
	ctx := newTestCtx(nil)
	ctx.Set("titles", []interface{}{"A", "B", "C"})
	ctx.Set("links", []interface{}{"u1", "u2"})
	err := runProcess(ctx, model.Step{Op: "zip", Args: map[string]string{
		"from": "titles,links", "fields": "title,url", "to": "rows",
	}})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := toSlice(ctx.Get("rows"))
	if len(rows) != 3 {
		t.Fatalf("期望 3 行（以最长为准）, got %d", len(rows))
	}
	r0 := rows[0].(map[string]interface{})
	if r0["title"] != "A" || r0["url"] != "u1" {
		t.Fatalf("第1行错误: %v", r0)
	}
	r2 := rows[2].(map[string]interface{})
	if r2["url"] != "" {
		t.Fatalf("第3行缺链接应为空: %v", r2)
	}
}

// 验证 process limit：区间截取。
func TestProcessLimit(t *testing.T) {
	ctx := newTestCtx(nil)
	ctx.Set("all", []interface{}{1, 2, 3, 4, 5})
	if err := runProcess(ctx, model.Step{Op: "limit", Args: map[string]string{
		"from": "all", "count": "3", "to": "top3",
	}}); err != nil {
		t.Fatal(err)
	}
	top, _ := toSlice(ctx.Get("top3"))
	if len(top) != 3 || top[2] != 3 {
		t.Fatalf("count 截取错误: %v", top)
	}
	if err := runProcess(ctx, model.Step{Op: "slice", Args: map[string]string{
		"from": "all", "start": "1", "end": "4", "to": "mid",
	}}); err != nil {
		t.Fatal(err)
	}
	mid, _ := toSlice(ctx.Get("mid"))
	if len(mid) != 3 || mid[0] != 2 || mid[2] != 4 {
		t.Fatalf("区间截取错误: %v", mid)
	}
}

// 验证 ccssRows + resolve 组合：行内 attr 直接解码成目标地址。
func TestCSSRowsResolveURL(t *testing.T) {
	html := `<li class="r"><h2>Go</h2><a href="/link?url=aHR0cHM6Ly9naXRodWIuY29t">x</a></li>`
	ex := model.Extract{
		Name: "rows", Kind: "cssrows", Path: "li.r",
		CssFields: []model.CSSField{
			{Name: "title", Path: "h2", CssField: "text"},
			{Name: "url", Path: "a", CssField: "attr", CssAttr: "href", Resolve: true},
		},
	}
	v, err := extractOne(newTestCtx(nil), htmlResp(html), ex)
	if err != nil {
		t.Fatal(err)
	}
	row := v.([]interface{})[0].(map[string]interface{})
	if !strings.HasPrefix(toStr(row["url"]), "https://") {
		t.Fatalf("resolve 后的 URL 应为绝对/解码地址: %v", row["url"])
	}
}
