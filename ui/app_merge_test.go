package ui

import (
	"strings"
	"testing"

	"requestflow/history"
	"requestflow/model"
)

// TestSyncStepToDebugSingleSource 验证 step 是请求配置的唯一数据源：
// 改动 step 后再同步，调试快照随之更新（合并后编排即调试，不再有两份可编辑配置）。
func TestSyncStepToDebugSingleSource(t *testing.T) {
	a := &App{scenario: model.NewScenario("s")}
	step := &model.Step{
		Type: model.StepRequest, Method: "PUT", URL: "/items",
		Headers: map[string]string{"X-Token": "abc"},
		Query:   map[string]string{"page": "3"},
		Body:    `{"a":1}`,
	}
	a.syncStepToDebug(step)
	if a.debugMethod != "PUT" || a.debugURL != "/items" || a.debugBody != `{"a":1}` {
		t.Fatalf("首次同步失败: %q %q %q", a.debugMethod, a.debugURL, a.debugBody)
	}
	if !strings.Contains(a.debugHeaders, "X-Token: abc") || !strings.Contains(a.debugQuery, "page: 3") {
		t.Fatalf("Header/Query 应为冒号格式: %q %q", a.debugHeaders, a.debugQuery)
	}
	// 改动 step 再同步：快照跟着变，证明 step 是唯一数据源。
	step.Method = "DELETE"
	step.Headers = map[string]string{"X-Token": "new"}
	a.syncStepToDebug(step)
	if a.debugMethod != "DELETE" || !strings.Contains(a.debugHeaders, "X-Token: new") {
		t.Fatalf("改 step 后快照未更新: %q %q", a.debugMethod, a.debugHeaders)
	}
}

// TestHistoryLoadAsStep 验证历史请求还原为临时 request 步骤并选中，
// 而非跳到已删除的独立调试面板（mainTab 重映射后 0=编排 1=运行结果）。
func TestHistoryLoadAsStep(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), mainTab: 1, showHistory: true}
	a.loadHistoryToDebug(history.Entry{
		Time: "2026-11-24 12:00:00", Method: "POST", URL: "https://example.com/x",
		Headers:  map[string]string{"Content-Type": "application/json"},
		BodyType: "raw", Body: `{"k":"v"}`,
	})
	if a.mainTab != 0 {
		t.Fatalf("载入历史应回到步骤编排, got %d", a.mainTab)
	}
	if a.showHistory {
		t.Fatal("载入历史后应关闭历史面板")
	}
	if len(a.scenario.Steps) != 1 || a.selectedStep != 0 {
		t.Fatalf("应新增一个临时步骤并选中: steps=%d selected=%d", len(a.scenario.Steps), a.selectedStep)
	}
	step := a.scenario.Steps[0]
	if step.Type != model.StepRequest || step.Method != "POST" || step.URL != "https://example.com/x" {
		t.Fatalf("历史请求还原不符: %+v", step)
	}
	if step.Headers["Content-Type"] != "application/json" {
		t.Fatalf("历史 Header 未还原: %v", step.Headers)
	}
}
