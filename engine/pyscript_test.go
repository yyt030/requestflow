package engine

import (
	"strings"
	"testing"

	"requestflow/model"
)

func pyStep(code string) model.Step {
	return model.Step{ID: "py", Name: "python脚本", Type: model.StepScript, Enabled: true, ScriptLang: "python", Code: code}
}

// TestPythonScript_Basic 验证 Python 脚本注入 vars/last、变量读写与写回。
func TestPythonScript_Basic(t *testing.T) {
	ctx := NewContext(map[string]string{"amount": "120"})
	resp := &model.Response{StatusCode: 200, Text: `{"data":{"token":"abc"}}`, JSON: map[string]interface{}{
		"data": map[string]interface{}{"token": "abc"},
	}}

	step := pyStep(`
vars["token"] = last["json"]["data"]["token"]
vars["doubled"] = int(vars["amount"]) * 2
items = [1, 2, 3]
vars["total"] = sum([x for x in items if x > 1])
vars["status"] = last["status"]
log("doubled=", vars["doubled"])
`)
	if err := runPythonScript(ctx, step, resp); err != nil {
		t.Fatalf("Python 脚本执行失败: %v", err)
	}
	checks := map[string]string{
		"token":   "abc",
		"doubled": "240",
		"total":   "5",
		"status":  "200",
	}
	for k, want := range checks {
		if got := toStr(ctx.Get(k)); got != want {
			t.Errorf("vars[%q] = %s, 期望 %s", k, got, want)
		}
	}
}

// TestPythonScript_Functions 验证自定义函数、if、for（函数内）、any/all/abs。
func TestPythonScript_Functions(t *testing.T) {
	ctx := NewContext(nil)
	step := pyStep(`
def classify(n):
    if n > 100:
        return "big"
    return "small"

vars["cmp"] = classify(120)

def fill():
    for i in range(3):
        vars["loop" + str(i)] = i * 10
fill()

nums = [1, -2, 3]
vars["has_neg"] = any([x < 0 for x in nums])
vars["all_pos"] = all([x > 0 for x in nums])
vars["abs_val"] = abs(-42)
`)
	if err := runPythonScript(ctx, step, nil); err != nil {
		t.Fatalf("Python 脚本执行失败: %v", err)
	}
	want := map[string]string{"cmp": "big", "loop0": "0", "loop2": "20", "has_neg": "true", "all_pos": "false", "abs_val": "42"}
	for k, v := range want {
		if got := toStr(ctx.Get(k)); got != v {
			t.Errorf("vars[%q] = %s, 期望 %s", k, got, v)
		}
	}
}

// TestPythonScript_JSON 验证 JSON.stringify / parse。
func TestPythonScript_JSON(t *testing.T) {
	ctx := NewContext(nil)
	step := pyStep(`
d = {"a": 1, "b": [2, 3]}
s = JSON["stringify"](d)
vars["json"] = s
back = JSON["parse"](s)
vars["a"] = back["a"]
`)
	if err := runPythonScript(ctx, step, nil); err != nil {
		t.Fatalf("Python 脚本执行失败: %v", err)
	}
	if got := toStr(ctx.Get("a")); got != "1" {
		t.Errorf("JSON.parse 后 a = %s, 期望 1", got)
	}
	if got := toStr(ctx.Get("json")); !strings.Contains(got, `"a"`) {
		t.Errorf("JSON.stringify 结果异常: %s", got)
	}
}

// TestPythonScript_TopLevelForRejected 顶层 for 在 Starlark 中不允许，验证报错清晰。
func TestPythonScript_TopLevelForRejected(t *testing.T) {
	ctx := NewContext(nil)
	step := pyStep("for i in range(3):\n    vars['x'] = i\n")
	err := runPythonScript(ctx, step, nil)
	if err == nil {
		t.Fatal("顶层 for 应被 Starlark 拒绝")
	}
	if !strings.Contains(err.Error(), "for") || !strings.Contains(err.Error(), "function") {
		t.Errorf("错误信息应提示 for 需在函数内, got: %v", err)
	}
}

// TestScriptDefaultIsJS 验证未设置 scriptLang 时仍按 JS 执行（兼容旧场景）。
func TestScriptDefaultIsJS(t *testing.T) {
	ctx := NewContext(nil)
	step := model.Step{ID: "js", Name: "js脚本", Type: model.StepScript, Enabled: true,
		Code: `vars.jsok = 1 + 2; vars.jsmode = "yes";`}
	if err := runScript(ctx, step, nil); err != nil {
		t.Fatalf("JS 脚本执行失败: %v", err)
	}
	if got := toStr(ctx.Get("jsok")); got != "3" {
		t.Errorf("jsok = %s, 期望 3", got)
	}
}

// TestPythonScript_ ViaEngine 验证通过 Engine 分发能正确选择 Python 引擎。
func TestPythonScript_ViaEngine(t *testing.T) {
	sc := &model.Scenario{Name: "py via engine", Steps: []model.Step{pyStep(`vars["via"] = "engine"`)}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("场景执行失败: %v", res.Err)
	}
	if got := toStr(res.Vars["via"]); got != "engine" {
		t.Errorf("via = %s, 期望 engine", got)
	}
}
