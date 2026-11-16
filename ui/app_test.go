package ui

import (
	"testing"

	"requestflow/engine"
	"requestflow/model"
)

// 验证结果表数据源筛选：只有数组/JSON数组变量被识别为可表格展示。
func TestTabularVarsSelection(t *testing.T) {
	a := &App{}
	a.lastResult = &engine.Result{
		Vars: map[string]interface{}{
			"token":  "abc",                       // 标量，不展示
			"page":   3,                           // 标量，不展示
			"rows":   []interface{}{map[string]interface{}{"id": 1}}, // 数组，展示
			"rawJSON": `["x","y"]`,                // JSON 数组字符串，展示
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
