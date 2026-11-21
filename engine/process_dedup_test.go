package engine

import (
	"testing"

	"requestflow/model"
)

// runProcessWith 用初始变量构造 Context 执行一个 process 步骤，返回上下文。
func runProcessWith(t *testing.T, vars map[string]string, op string, args map[string]string) *Context {
	t.Helper()
	ctx := NewContext(vars)
	step := model.Step{Op: op, Args: args}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("runProcess(%s) 失败: %v", op, err)
	}
	return ctx
}

func TestProcessDedupScalars(t *testing.T) {
	ctx := runProcessWith(t, map[string]string{}, "dedup", map[string]string{
		"from": `["a","b","a","c","b"]`,
		"to":   "out",
		"stat": "dupCount",
	})
	got, ok := ctx.Get("out").([]interface{})
	if !ok {
		t.Fatalf("out 不是数组: %T", ctx.Get("out"))
	}
	if len(got) != 3 || toStr(got[0]) != "a" || toStr(got[1]) != "b" || toStr(got[2]) != "c" {
		t.Fatalf("dedup 结果错误: %v", got)
	}
	if n, _ := toFloat(ctx.Get("dupCount")); n != 2 {
		t.Fatalf("dupCount = %v, 期望 2", ctx.Get("dupCount"))
	}
}

func TestProcessDedupByField(t *testing.T) {
	ctx := NewContext(nil)
	ctx.Set("rows", []interface{}{
		map[string]interface{}{"link": "https://a", "title": "A1"},
		map[string]interface{}{"link": "https://b", "title": "B"},
		map[string]interface{}{"link": "https://a", "title": "A2"},
	})
	step := model.Step{Op: "dedup", Args: map[string]string{
		"from": "rows", "field": "link", "to": "uniq", "stat": "removed",
	}}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("dedup 失败: %v", err)
	}
	got := ctx.Get("uniq").([]interface{})
	if len(got) != 2 {
		t.Fatalf("按字段去重后应有 2 行, 得到 %d: %v", len(got), got)
	}
	if toStr(fieldValue(got[0], "title")) != "A1" {
		t.Fatalf("应保留首次出现的 A1, 得到 %v", got[0])
	}
	if n, _ := toFloat(ctx.Get("removed")); n != 1 {
		t.Fatalf("removed = %v, 期望 1", ctx.Get("removed"))
	}
}

func TestProcessBase64DecodeArray(t *testing.T) {
	// "aHR0cHM6Ly9h" = https://a, "aHR0cHM6Ly9i" = https://b
	ctx := NewContext(nil)
	ctx.Set("links", []interface{}{"aHR0cHM6Ly9h", "aHR0cHM6Ly9i"})
	step := model.Step{Op: "base64decode", Args: map[string]string{"from": "links", "to": "decoded"}}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("base64decode 数组失败: %v", err)
	}
	got, ok := ctx.Get("decoded").([]interface{})
	if !ok || len(got) != 2 {
		t.Fatalf("decoded 应是 2 元素数组: %T %v", ctx.Get("decoded"), got)
	}
	if toStr(got[0]) != "https://a" || toStr(got[1]) != "https://b" {
		t.Fatalf("解码结果错误: %v", got)
	}
}

func TestProcessBase64DecodeScalar(t *testing.T) {
	ctx := runProcessWith(t, map[string]string{}, "base64decode", map[string]string{
		"from": "aHR0cHM6Ly9h", "to": "decoded",
	})
	if got := ctx.Get("decoded"); toStr(got) != "https://a" {
		t.Fatalf("标量解码 = %v, 期望 https://a", got)
	}
}

func TestProcessBase64DecodeURLSafeNoPadding(t *testing.T) {
	// base64url 无 padding 的跳转链接片段
	ctx := runProcessWith(t, map[string]string{}, "base64decode", map[string]string{
		"from": "aHR0cHM6Ly9leGFtcGxlLmNvbS9hP2I9Yy1kX2U", "to": "decoded",
	})
	got := toStr(ctx.Get("decoded"))
	if got == "" {
		t.Fatalf("无 padding base64url 解码为空")
	}
}

func TestScriptAtobBtoaJS(t *testing.T) {
	ctx := NewContext(nil)
	step := model.Step{Code: `
		var enc = btoa("hello");
		vars.enc = enc;
		vars.dec = atob(enc);
	`}
	if err := runScript(ctx, step, nil); err != nil {
		t.Fatalf("JS atob/btoa 脚本失败: %v", err)
	}
	if toStr(ctx.Get("dec")) != "hello" {
		t.Fatalf("JS atob(btoa(hello)) = %v", ctx.Get("dec"))
	}
}

func TestScriptAtobBtoaPython(t *testing.T) {
	ctx := NewContext(nil)
	step := model.Step{ScriptLang: "python", Code: `
enc = btoa("hello")
vars["enc"] = enc
vars["dec"] = atob(enc)
`}
	if err := runPythonScript(ctx, step, nil); err != nil {
		t.Fatalf("Python atob/btoa 脚本失败: %v", err)
	}
	if toStr(ctx.Get("dec")) != "hello" {
		t.Fatalf("Python atob(btoa(hello)) = %v", ctx.Get("dec"))
	}
}
